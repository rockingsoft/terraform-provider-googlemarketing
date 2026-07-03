package provider

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestGTMPublishSchemaOnlyReplacesOnAccountAndContainer(t *testing.T) {
	ctx := context.Background()
	resourceSchema := gtmPublishSchema(t, ctx)

	for _, name := range []string{"account_id", "container_id"} {
		attribute, ok := resourceSchema.Attributes[name].(schema.StringAttribute)
		if !ok {
			t.Fatalf("%s is %T, want schema.StringAttribute", name, resourceSchema.Attributes[name])
		}
		assertStringRequiresReplace(t, name, attribute)
	}

	for _, name := range []string{"workspace_name", "version_name", "notes"} {
		attribute, ok := resourceSchema.Attributes[name].(schema.StringAttribute)
		if !ok {
			t.Fatalf("%s is %T, want schema.StringAttribute", name, resourceSchema.Attributes[name])
		}
		if len(attribute.PlanModifiers) != 0 {
			t.Fatalf("%s has plan modifiers %#v, want none so changing it alone does not force a republish", name, attribute.PlanModifiers)
		}
	}

	publish, ok := resourceSchema.Attributes["publish"].(schema.BoolAttribute)
	if !ok {
		t.Fatalf("publish is %T, want schema.BoolAttribute", resourceSchema.Attributes["publish"])
	}
	if len(publish.PlanModifiers) != 0 {
		t.Fatalf("publish has plan modifiers %#v, want none so changing it alone does not force a republish", publish.PlanModifiers)
	}
}

func gtmPublishSchema(t *testing.T, ctx context.Context) schema.Schema {
	t.Helper()
	var resp resource.SchemaResponse
	NewGTMPublishResource().Schema(ctx, resource.SchemaRequest{}, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %s", resp.Diagnostics)
	}
	return resp.Schema
}

func TestGTMPublishCreateVersionAndPublishCreatesAndPublishesVersion(t *testing.T) {
	ctx := context.Background()
	var createVersionCalled, publishCalled bool
	client := &marketingClient{
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case strings.HasSuffix(req.URL.Path, "/workspaces"):
				return gtmReleaseTestResponse(`{"workspace":[{"workspaceId":"3","name":"Default Workspace"}]}`), nil
			case strings.HasSuffix(req.URL.Path, ":create_version"):
				createVersionCalled = true
				return gtmReleaseTestResponse(`{"containerVersion":{"path":"accounts/1/containers/2/versions/9","containerVersionId":"9"},"newWorkspacePath":"accounts/1/containers/2/workspaces/11"}`), nil
			case strings.HasSuffix(req.URL.Path, ":publish"):
				publishCalled = true
				return gtmReleaseTestResponse(`{}`), nil
			default:
				t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
			}
			return nil, nil
		})},
		gtmLimiter: newGTMRateLimiter(0),
		gtmCache:   newGTMCollectionCache(),
	}
	r := &gtmPublishResource{client: client}
	plan := gtmPublishModel{
		AccountID:   types.StringValue("1"),
		ContainerID: types.StringValue("2"),
		VersionName: types.StringValue("Terraform publish"),
		Publish:     types.BoolValue(true),
	}
	if err := r.createVersionAndPublish(ctx, &plan); err != nil {
		t.Fatalf("createVersionAndPublish returned error: %v", err)
	}
	if !createVersionCalled || !publishCalled {
		t.Fatalf("createVersionCalled=%t publishCalled=%t, want both true", createVersionCalled, publishCalled)
	}
	if plan.VersionPath.ValueString() != "accounts/1/containers/2/versions/9" {
		t.Fatalf("version_path = %q, want accounts/1/containers/2/versions/9", plan.VersionPath.ValueString())
	}
	if plan.NewWorkspacePath.ValueString() != "accounts/1/containers/2/workspaces/11" {
		t.Fatalf("new_workspace_path = %q, want accounts/1/containers/2/workspaces/11", plan.NewWorkspacePath.ValueString())
	}
	if !plan.Published.ValueBool() {
		t.Fatal("published = false, want true")
	}

	// The workspace create_version consumed no longer exists; the cached
	// workspace ID must be dropped so entity resources re-resolve.
	if id, ok := client.gtmCache.getWorkspaceID("accounts/1/containers/2|Default Workspace"); ok {
		t.Fatalf("workspace ID cache still has %q after publish, want invalidated", id)
	}
}

func TestGTMPublishCreateVersionAndPublishSkipsPublishWhenDisabled(t *testing.T) {
	ctx := context.Background()
	publishCalled := false
	client := &marketingClient{
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case strings.HasSuffix(req.URL.Path, "/workspaces"):
				return gtmReleaseTestResponse(`{"workspace":[{"workspaceId":"3","name":"Default Workspace"}]}`), nil
			case strings.HasSuffix(req.URL.Path, ":create_version"):
				return gtmReleaseTestResponse(`{"containerVersion":{"path":"accounts/1/containers/2/versions/9","containerVersionId":"9"}}`), nil
			case strings.HasSuffix(req.URL.Path, ":publish"):
				publishCalled = true
				return gtmReleaseTestResponse(`{}`), nil
			default:
				t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
			}
			return nil, nil
		})},
		gtmLimiter: newGTMRateLimiter(0),
		gtmCache:   newGTMCollectionCache(),
	}
	r := &gtmPublishResource{client: client}
	plan := gtmPublishModel{
		AccountID:   types.StringValue("1"),
		ContainerID: types.StringValue("2"),
		VersionName: types.StringValue("Terraform publish"),
		Publish:     types.BoolValue(false),
	}
	if err := r.createVersionAndPublish(ctx, &plan); err != nil {
		t.Fatalf("createVersionAndPublish returned error: %v", err)
	}
	if publishCalled {
		t.Fatal("publish endpoint was called even though publish = false")
	}
	if plan.Published.ValueBool() {
		t.Fatal("published = true, want false")
	}
}

func TestGTMPublishCreateVersionAndPublishReturnsErrorOnCompilerError(t *testing.T) {
	ctx := context.Background()
	client := &marketingClient{
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case strings.HasSuffix(req.URL.Path, "/workspaces"):
				return gtmReleaseTestResponse(`{"workspace":[{"workspaceId":"3","name":"Default Workspace"}]}`), nil
			case strings.HasSuffix(req.URL.Path, ":create_version"):
				return gtmReleaseTestResponse(`{"compilerError":true}`), nil
			default:
				t.Fatalf("unexpected request %s %s", req.Method, req.URL.Path)
			}
			return nil, nil
		})},
		gtmLimiter: newGTMRateLimiter(0),
		gtmCache:   newGTMCollectionCache(),
	}
	r := &gtmPublishResource{client: client}
	plan := gtmPublishModel{
		AccountID:   types.StringValue("1"),
		ContainerID: types.StringValue("2"),
		VersionName: types.StringValue("Terraform publish"),
	}
	if err := r.createVersionAndPublish(ctx, &plan); err == nil {
		t.Fatal("createVersionAndPublish() error = nil, want error on compiler error")
	}
}

func TestGTMWorkspaceHasPendingChanges(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name string
		body string
		want bool
	}{
		{"no changes", `{}`, false},
		{"pending changes", `{"workspaceChange":[{"tag":{"tagId":"1"}}]}`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &marketingClient{
				httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if !strings.HasSuffix(req.URL.Path, "/status") {
						t.Fatalf("unexpected request path %s", req.URL.Path)
					}
					return gtmReleaseTestResponse(tt.body), nil
				})},
				gtmLimiter: newGTMRateLimiter(0),
			}
			got, err := client.gtmWorkspaceHasPendingChanges(ctx, "1", "2", "3")
			if err != nil {
				t.Fatalf("gtmWorkspaceHasPendingChanges returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("gtmWorkspaceHasPendingChanges() = %t, want %t", got, tt.want)
			}
		})
	}
}
