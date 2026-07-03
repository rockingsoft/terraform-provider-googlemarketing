package provider

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestGTMEntitySchemaOnlyReplacesOnAccountAndContainer(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []string{"variable", "trigger", "tag"} {
		t.Run(kind, func(t *testing.T) {
			resourceSchema := gtmEntitySchema(t, ctx, kind)

			for _, name := range []string{"account_id", "container_id"} {
				attribute, ok := resourceSchema.Attributes[name].(schema.StringAttribute)
				if !ok {
					t.Fatalf("%s is %T, want schema.StringAttribute", name, resourceSchema.Attributes[name])
				}
				assertStringRequiresReplace(t, name, attribute)
			}

			for _, name := range []string{"workspace_name", "name", "notes", "type", "measurement_id"} {
				attribute, ok := resourceSchema.Attributes[name].(schema.StringAttribute)
				if !ok {
					t.Fatalf("%s is %T, want schema.StringAttribute", name, resourceSchema.Attributes[name])
				}
				if len(attribute.PlanModifiers) != 0 {
					t.Fatalf("%s has plan modifiers %#v, want none so changing it alone does not force a replace", name, attribute.PlanModifiers)
				}
			}
		})
	}
}

func TestGTMEntitySchemaComputedAttributesUseStateForUnknown(t *testing.T) {
	ctx := context.Background()
	resourceSchema := gtmEntitySchema(t, ctx, "trigger")
	want := reflect.TypeOf(stringplanmodifier.UseStateForUnknown())

	for _, name := range []string{"id", "entity_id", "path", "workspace_id"} {
		attribute, ok := resourceSchema.Attributes[name].(schema.StringAttribute)
		if !ok {
			t.Fatalf("%s is %T, want schema.StringAttribute", name, resourceSchema.Attributes[name])
		}
		found := false
		for _, modifier := range attribute.PlanModifiers {
			if reflect.TypeOf(modifier) == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s does not use UseStateForUnknown, so it would show as unknown on every plan even without a real change", name)
		}
	}
}

func gtmEntitySchema(t *testing.T, ctx context.Context, kind string) schema.Schema {
	t.Helper()
	var resp resource.SchemaResponse
	(&gtmTypedEntityResource{kind: kind, typeSuffix: "_gtm_" + kind}).Schema(ctx, resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %s", resp.Diagnostics)
	}
	return resp.Schema
}

func TestGTMEntityCreateAdoptsExistingEntityByName(t *testing.T) {
	ctx := context.Background()
	collectionPath := "/tagmanager/v2/accounts/1/containers/2/workspaces/3/triggers"
	gets, posts, puts := 0, 0, 0
	client := &marketingClient{
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case strings.HasSuffix(req.URL.Path, "/workspaces"):
				return gtmReleaseTestResponse(`{"workspace":[{"workspaceId":"3","name":"Default Workspace"}]}`), nil
			case req.URL.Path == collectionPath && req.Method == http.MethodGet:
				gets++
				return gtmReleaseTestResponse(`{"trigger":[{"path":"accounts/1/containers/2/workspaces/3/triggers/9","triggerId":"9","name":"Existing"}]}`), nil
			case req.URL.Path == collectionPath && req.Method == http.MethodPost:
				posts++
				return gtmReleaseTestResponse(`{"path":"accounts/1/containers/2/workspaces/3/triggers/10","triggerId":"10","name":"New"}`), nil
			case req.URL.Path == collectionPath+"/9" && req.Method == http.MethodPut:
				puts++
				return gtmReleaseTestResponse(`{"path":"accounts/1/containers/2/workspaces/3/triggers/9","triggerId":"9","name":"Existing"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
			}
			return nil, nil
		})},
		gtmLimiter: newGTMRateLimiter(0),
		gtmCache:   newGTMCollectionCache(),
	}
	r := &gtmTypedEntityResource{kind: "trigger", client: client}

	adopted := gtmTypedWorkspaceEntityModel{
		AccountID:   types.StringValue("1"),
		ContainerID: types.StringValue("2"),
		Name:        types.StringValue("Existing"),
		Type:        types.StringValue("CUSTOM_EVENT"),
	}
	if err := r.createEntity(ctx, &adopted); err != nil {
		t.Fatalf("createEntity (adopt) returned error: %v", err)
	}
	if puts != 1 || posts != 0 {
		t.Fatalf("adopting an existing name did puts=%d posts=%d, want puts=1 posts=0", puts, posts)
	}
	if adopted.EntityID.ValueString() != "9" {
		t.Fatalf("entity_id = %q, want 9", adopted.EntityID.ValueString())
	}

	created := gtmTypedWorkspaceEntityModel{
		AccountID:   types.StringValue("1"),
		ContainerID: types.StringValue("2"),
		Name:        types.StringValue("New"),
		Type:        types.StringValue("CUSTOM_EVENT"),
	}
	if err := r.createEntity(ctx, &created); err != nil {
		t.Fatalf("createEntity (new) returned error: %v", err)
	}
	if posts != 1 {
		t.Fatalf("creating a new name did posts=%d, want 1", posts)
	}
	if created.EntityID.ValueString() != "10" {
		t.Fatalf("entity_id = %q, want 10", created.EntityID.ValueString())
	}
	if gets != 1 {
		t.Fatalf("collection GETs = %d, want 1 (writes update the cache in place instead of invalidating it, so the second createEntity call reuses the first GET)", gets)
	}
}

func TestGTMEntityCreateRejectsGaaweTagWithoutMeasurementID(t *testing.T) {
	r := &gtmTypedEntityResource{kind: "tag", client: &marketingClient{}}
	plan := gtmTypedWorkspaceEntityModel{
		AccountID:   types.StringValue("1"),
		ContainerID: types.StringValue("2"),
		Name:        types.StringValue("GA4 purchase"),
		Type:        types.StringValue("gaawe"),
	}
	if err := r.createEntity(context.Background(), &plan); err == nil {
		t.Fatal("createEntity() error = nil, want error for gaawe tag missing measurement_id")
	}
}

func TestGTMEntityReadAfterWorkspaceRotationRefreshesWorkspaceID(t *testing.T) {
	ctx := context.Background()
	workspacesPath := "/tagmanager/v2/accounts/1/containers/2/workspaces"
	newCollectionPath := "/tagmanager/v2/accounts/1/containers/2/workspaces/7/triggers"
	client := &marketingClient{
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch req.URL.Path {
			case workspacesPath:
				return gtmReleaseTestResponse(`{"workspace":[{"workspaceId":"7","name":"Default Workspace"}]}`), nil
			case newCollectionPath:
				return gtmReleaseTestResponse(`{"trigger":[{"path":"accounts/1/containers/2/workspaces/7/triggers/9","triggerId":"9","name":"Purchase"}]}`), nil
			default:
				t.Fatalf("unexpected request path %s", req.URL.Path)
			}
			return nil, nil
		})},
		gtmLimiter: newGTMRateLimiter(0),
		gtmCache:   newGTMCollectionCache(),
	}
	r := &gtmTypedEntityResource{kind: "trigger", client: client}

	// State was written before a publish rotated the workspace from 3 to 7.
	state := gtmTypedWorkspaceEntityModel{
		AccountID:   types.StringValue("1"),
		ContainerID: types.StringValue("2"),
		EntityID:    types.StringValue("9"),
		WorkspaceID: types.StringValue("3"),
	}
	found, err := r.readEntity(ctx, &state)
	if err != nil {
		t.Fatalf("readEntity returned error: %v", err)
	}
	if !found {
		t.Fatal("readEntity() found = false, want true")
	}
	if state.WorkspaceID.ValueString() != "7" {
		t.Fatalf("workspace_id = %q, want 7 (refreshed to the current workspace)", state.WorkspaceID.ValueString())
	}
	if state.Name.ValueString() != "Purchase" {
		t.Fatalf("name = %q, want Purchase", state.Name.ValueString())
	}
}

func TestGTMEntityReadRemovesResourceWhenEntityIDMissing(t *testing.T) {
	ctx := context.Background()
	client := &marketingClient{
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case strings.HasSuffix(req.URL.Path, "/workspaces"):
				return gtmReleaseTestResponse(`{"workspace":[{"workspaceId":"3","name":"Default Workspace"}]}`), nil
			default:
				return gtmReleaseTestResponse(`{"trigger":[]}`), nil
			}
		})},
		gtmLimiter: newGTMRateLimiter(0),
		gtmCache:   newGTMCollectionCache(),
	}
	r := &gtmTypedEntityResource{kind: "trigger", client: client}
	state := gtmTypedWorkspaceEntityModel{
		AccountID:   types.StringValue("1"),
		ContainerID: types.StringValue("2"),
		EntityID:    types.StringValue("9"),
	}
	found, err := r.readEntity(ctx, &state)
	if err != nil {
		t.Fatalf("readEntity returned error: %v", err)
	}
	if found {
		t.Fatal("readEntity() found = true, want false")
	}
}

func TestGTMEntityUpdatePutsToResolvedWorkspacePath(t *testing.T) {
	ctx := context.Background()
	itemPath := "/tagmanager/v2/accounts/1/containers/2/workspaces/3/variables/9"
	var gotBody string
	client := &marketingClient{
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case strings.HasSuffix(req.URL.Path, "/workspaces"):
				return gtmReleaseTestResponse(`{"workspace":[{"workspaceId":"3","name":"Default Workspace"}]}`), nil
			case req.URL.Path == itemPath && req.Method == http.MethodPut:
				raw, _ := io.ReadAll(req.Body)
				gotBody = string(raw)
				return gtmReleaseTestResponse(`{"path":"accounts/1/containers/2/workspaces/3/variables/9","variableId":"9","name":"Page path"}`), nil
			default:
				t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
			}
			return nil, nil
		})},
		gtmLimiter: newGTMRateLimiter(0),
		gtmCache:   newGTMCollectionCache(),
	}
	r := &gtmTypedEntityResource{kind: "variable", client: client}
	plan := gtmTypedWorkspaceEntityModel{
		AccountID:     types.StringValue("1"),
		ContainerID:   types.StringValue("2"),
		EntityID:      types.StringValue("9"),
		Name:          types.StringValue("Page path"),
		Type:          types.StringValue("v"),
		DataLayerName: types.StringValue("page.path"),
	}
	if err := r.updateEntity(ctx, &plan); err != nil {
		t.Fatalf("updateEntity returned error: %v", err)
	}
	if !strings.Contains(gotBody, "page.path") {
		t.Fatalf("PUT body = %s, want it to contain the data_layer_name parameter", gotBody)
	}
	if plan.WorkspaceID.ValueString() != "3" {
		t.Fatalf("workspace_id = %q, want 3", plan.WorkspaceID.ValueString())
	}
}

func TestGTMEntityDeleteToleratesNotFound(t *testing.T) {
	ctx := context.Background()
	client := &marketingClient{
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case strings.HasSuffix(req.URL.Path, "/workspaces"):
				return gtmReleaseTestResponse(`{"workspace":[{"workspaceId":"3","name":"Default Workspace"}]}`), nil
			case req.Method == http.MethodDelete:
				return &http.Response{StatusCode: http.StatusNotFound, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}, nil
			default:
				t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
			}
			return nil, nil
		})},
		gtmLimiter: newGTMRateLimiter(0),
		gtmCache:   newGTMCollectionCache(),
	}
	r := &gtmTypedEntityResource{kind: "tag", client: client}
	state := gtmTypedWorkspaceEntityModel{
		AccountID:   types.StringValue("1"),
		ContainerID: types.StringValue("2"),
		EntityID:    types.StringValue("9"),
	}
	if err := r.deleteEntity(ctx, &state); err != nil {
		t.Fatalf("deleteEntity returned error: %v", err)
	}
}

func TestResolveGTMWorkspaceIDCachesUntilInvalidated(t *testing.T) {
	ctx := context.Background()
	requests := 0
	client := &marketingClient{
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests++
			return gtmReleaseTestResponse(`{"workspace":[{"workspaceId":"3","name":"Default Workspace"}]}`), nil
		})},
		gtmLimiter: newGTMRateLimiter(0),
		gtmCache:   newGTMCollectionCache(),
	}

	for i := 0; i < 3; i++ {
		id, err := client.resolveGTMWorkspaceID(ctx, "1", "2", "")
		if err != nil {
			t.Fatalf("resolveGTMWorkspaceID returned error: %v", err)
		}
		if id != "3" {
			t.Fatalf("workspace id = %q, want 3", id)
		}
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1 (cached across calls)", requests)
	}

	client.invalidateGTMWorkspaces("1", "2")
	if _, err := client.resolveGTMWorkspaceID(ctx, "1", "2", ""); err != nil {
		t.Fatalf("resolveGTMWorkspaceID returned error: %v", err)
	}
	if requests != 2 {
		t.Fatalf("requests after invalidation = %d, want 2", requests)
	}
}

func TestResolveGTMWorkspaceIDDefaultsToDefaultWorkspace(t *testing.T) {
	ctx := context.Background()
	client := &marketingClient{
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return gtmReleaseTestResponse(`{"workspace":[{"workspaceId":"3","name":"Default Workspace"},{"workspaceId":"5","name":"Staging"}]}`), nil
		})},
		gtmLimiter: newGTMRateLimiter(0),
		gtmCache:   newGTMCollectionCache(),
	}
	id, err := client.resolveGTMWorkspaceID(ctx, "1", "2", "")
	if err != nil {
		t.Fatalf("resolveGTMWorkspaceID returned error: %v", err)
	}
	if id != "3" {
		t.Fatalf("workspace id = %q, want 3 (Default Workspace)", id)
	}
	stagingID, err := client.resolveGTMWorkspaceID(ctx, "1", "2", "Staging")
	if err != nil {
		t.Fatalf("resolveGTMWorkspaceID returned error: %v", err)
	}
	if stagingID != "5" {
		t.Fatalf("workspace id = %q, want 5 (Staging)", stagingID)
	}
}

func TestGTMDirtyRegistryTracksPerContainer(t *testing.T) {
	client := &marketingClient{}
	if client.isGTMContainerDirty("1", "2") {
		t.Fatal("isGTMContainerDirty() = true before anything was marked, want false")
	}
	client.markGTMContainerDirty("1", "2")
	if !client.isGTMContainerDirty("1", "2") {
		t.Fatal("isGTMContainerDirty() = false after marking, want true")
	}
	if client.isGTMContainerDirty("1", "3") {
		t.Fatal("isGTMContainerDirty() = true for an unrelated container, want false")
	}
	client.clearGTMContainerDirty("1", "2")
	if client.isGTMContainerDirty("1", "2") {
		t.Fatal("isGTMContainerDirty() = true after clearing, want false")
	}
}

func TestParseGTMEntityImportIDStableForm(t *testing.T) {
	accountID, containerID, entityID, workspaceHint, err := parseGTMEntityImportID("trigger", "accounts/1/containers/2/triggers/9")
	if err != nil {
		t.Fatalf("parseGTMEntityImportID returned error: %v", err)
	}
	if accountID != "1" || containerID != "2" || entityID != "9" || workspaceHint != "" {
		t.Fatalf("parsed = %q %q %q %q, want 1 2 9 \"\"", accountID, containerID, entityID, workspaceHint)
	}
}

func TestParseGTMEntityImportIDWorkspaceForm(t *testing.T) {
	accountID, containerID, entityID, workspaceHint, err := parseGTMEntityImportID("trigger", "accounts/1/containers/2/workspaces/3/triggers/9")
	if err != nil {
		t.Fatalf("parseGTMEntityImportID returned error: %v", err)
	}
	if accountID != "1" || containerID != "2" || entityID != "9" || workspaceHint != "3" {
		t.Fatalf("parsed = %q %q %q %q, want 1 2 9 3", accountID, containerID, entityID, workspaceHint)
	}
}

func TestParseGTMEntityImportIDRejectsMismatchedKind(t *testing.T) {
	if _, _, _, _, err := parseGTMEntityImportID("trigger", "accounts/1/containers/2/tags/9"); err == nil {
		t.Fatal("parseGTMEntityImportID() error = nil, want error for mismatched kind")
	}
}
