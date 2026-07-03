package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*gtmTypedEntityResource)(nil)
var _ resource.ResourceWithConfigure = (*gtmTypedEntityResource)(nil)
var _ resource.ResourceWithImportState = (*gtmTypedEntityResource)(nil)
var _ resource.ResourceWithModifyPlan = (*gtmTypedEntityResource)(nil)

func NewGTMVariableResource() resource.Resource {
	return &gtmTypedEntityResource{kind: "variable", typeSuffix: "_gtm_variable"}
}

func NewGTMTriggerResource() resource.Resource {
	return &gtmTypedEntityResource{kind: "trigger", typeSuffix: "_gtm_trigger"}
}

func NewGTMTagResource() resource.Resource {
	return &gtmTypedEntityResource{kind: "tag", typeSuffix: "_gtm_tag"}
}

// gtmTypedEntityResource implements one GTM variable, trigger, or tag as an
// independently plannable resource, replacing the old monolithic
// googlemarketing_gtm_container_release. Identity is anchored to the
// account/container/entity ID (which GTM keeps stable across publishes),
// while the workspace it edits is re-resolved by name on every operation
// because create_version deletes the workspace it published and replaces it
// with a new one.
type gtmTypedEntityResource struct {
	client     *marketingClient
	kind       string
	typeSuffix string
}

func (r *gtmTypedEntityResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + r.typeSuffix
}

func (r *gtmTypedEntityResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:      true,
			Description:   "Stable identifier (accounts/{account_id}/containers/{container_id}/{collection}/{entity_id}) that survives GTM publishes.",
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		"entity_id": schema.StringAttribute{
			Computed:      true,
			Description:   "Short GTM entity ID, stable across publishes. Reference it from other GTM entity resources, for example firing_trigger_ids.",
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		"path": schema.StringAttribute{
			Computed:      true,
			Description:   "Current GTM workspace-relative API path. Rotates on every publish and is refreshed on every read.",
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		"workspace_id": schema.StringAttribute{
			Computed:      true,
			Description:   "Current GTM workspace ID. Rotates on every publish and is refreshed on every read.",
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		"account_id":   entityReplaceStringAttribute("GTM account ID."),
		"container_id": entityReplaceStringAttribute("GTM container ID."),
		"workspace_name": schema.StringAttribute{
			Optional:    true,
			Description: "Workspace name to edit. Defaults to \"Default Workspace\". Re-resolved to the current workspace ID on every operation.",
		},
		"name": schema.StringAttribute{
			Required:    true,
			Description: "Display name. If an entity with this name already exists in the workspace, it is adopted (updated in place) instead of creating a duplicate.",
		},
		"notes":                schema.StringAttribute{Optional: true},
		"measurement_id":       schema.StringAttribute{Optional: true, Description: "GA4 measurement ID used by gaawe/googtag tag types. Required when type is \"gaawe\"."},
		"event_name":           schema.StringAttribute{Optional: true, Description: "GA4 event name."},
		"html":                 schema.StringAttribute{Optional: true, Sensitive: true, Description: "Custom HTML body for html tags."},
		"conversion_id":        schema.StringAttribute{Optional: true, Description: "Google Ads conversion ID."},
		"conversion_label":     schema.StringAttribute{Optional: true, Sensitive: true, Description: "Google Ads conversion label."},
		"custom_event_name":    schema.StringAttribute{Optional: true, Description: "Event name for CUSTOM_EVENT triggers."},
		"filter_variable":      schema.StringAttribute{Optional: true, Description: "Variable used by the optional trigger filter."},
		"filter_operator":      schema.StringAttribute{Optional: true, Description: "Filter operator, for example EQUALS, CONTAINS, or MATCH_REGEX."},
		"filter_value":         schema.StringAttribute{Optional: true, Description: "Filter comparison value."},
		"value":                schema.StringAttribute{Optional: true, Sensitive: true, Description: "Constant or lookup value."},
		"data_layer_name":      schema.StringAttribute{Optional: true, Description: "Data layer variable name."},
		"cookie_name":          schema.StringAttribute{Optional: true, Sensitive: true, Description: "First-party cookie name."},
		"javascript":           schema.StringAttribute{Optional: true, Sensitive: true, Description: "Custom JavaScript body."},
		"firing_trigger_ids":   schema.ListAttribute{Optional: true, ElementType: types.StringType, Description: "entity_id values of triggers that fire this tag, e.g. [googlemarketing_gtm_trigger.x.entity_id]."},
		"blocking_trigger_ids": schema.ListAttribute{Optional: true, ElementType: types.StringType, Description: "entity_id values of triggers that block this tag."},
		"additional_params": schema.MapAttribute{
			Optional:    true,
			ElementType: types.StringType,
			Description: "Escape hatch for GTM template parameters not covered by a typed attribute above.",
		},
	}

	switch r.kind {
	case "tag":
		attrs["type"] = schema.StringAttribute{Required: true, Description: "GTM tag type, for example gaawe, googtag, html, or awct."}
	case "trigger":
		attrs["type"] = schema.StringAttribute{Required: true, Description: "GTM trigger type, for example CUSTOM_EVENT, PAGEVIEW, CLICK, FORM_SUBMISSION, TIMER, or HISTORY_CHANGE."}
	case "variable":
		attrs["type"] = schema.StringAttribute{Required: true, Description: "GTM variable type, for example c, v, k, or jsm."}
	}

	resp.Schema = schema.Schema{
		Description: fmt.Sprintf("Google Tag Manager workspace %s. Writes go straight to the resolved workspace; publish them with googlemarketing_gtm_publish.", r.kind),
		Attributes:  attrs,
	}
}

func entityReplaceStringAttribute(description string) schema.StringAttribute {
	return schema.StringAttribute{
		Required:    true,
		Description: description,
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.RequiresReplace(),
		},
	}
}

func (r *gtmTypedEntityResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*marketingClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("Expected *marketingClient, got %T", req.ProviderData))
		return
	}
	r.client = client
}

// ModifyPlan records that this container has a pending entity change so
// googlemarketing_gtm_publish can show an automatic update in the same plan,
// without the caller having to list every entity resource in a manual
// depends_on. Attribute-level plan modifiers (RequiresReplace,
// UseStateForUnknown) have already run by the time this is called, so
// comparing the raw plan and state values here reliably detects "nothing
// meaningful changed" without hand-rolling a field-by-field diff.
func (r *gtmTypedEntityResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if r.client == nil {
		return
	}
	if req.Plan.Raw.IsNull() {
		if req.State.Raw.IsNull() {
			return
		}
		var state gtmTypedWorkspaceEntityModel
		if diags := req.State.Get(ctx, &state); !diags.HasError() {
			r.client.markGTMContainerDirty(state.AccountID.ValueString(), state.ContainerID.ValueString())
		}
		return
	}
	if !req.State.Raw.IsNull() && req.Plan.Raw.Equal(req.State.Raw) {
		return
	}
	var plan gtmTypedWorkspaceEntityModel
	if diags := req.Plan.Get(ctx, &plan); !diags.HasError() {
		r.client.markGTMContainerDirty(plan.AccountID.ValueString(), plan.ContainerID.ValueString())
	}
}

func (r *gtmTypedEntityResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan gtmTypedWorkspaceEntityModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.createEntity(ctx, &plan); err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Unable to create GTM %s", r.kind), err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *gtmTypedEntityResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state gtmTypedWorkspaceEntityModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	found, err := r.readEntity(ctx, &state)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Unable to read GTM %s", r.kind), err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *gtmTypedEntityResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan gtmTypedWorkspaceEntityModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.updateEntity(ctx, &plan); err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Unable to update GTM %s", r.kind), err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *gtmTypedEntityResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state gtmTypedWorkspaceEntityModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.deleteEntity(ctx, &state); err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Unable to delete GTM %s", r.kind), err.Error())
	}
}

// createEntity resolves the current workspace and either adopts (PUT) an
// entity that already has the same name, or creates (POST) a new one. GTM
// rejects duplicate names, so adopting by name is what lets a first apply of
// these resources take over entities that already exist (hand-created, or
// left behind by the old release resource) without an import block each.
func (r *gtmTypedEntityResource) createEntity(ctx context.Context, plan *gtmTypedWorkspaceEntityModel) error {
	if r.kind == "tag" {
		if err := validateGTMTagAdditionalRequirements(*plan); err != nil {
			return err
		}
	}

	accountID, containerID := plan.AccountID.ValueString(), plan.ContainerID.ValueString()
	workspaceID, err := r.client.resolveGTMWorkspaceID(ctx, accountID, containerID, plan.WorkspaceName.ValueString())
	if err != nil {
		return err
	}
	collection, _ := gtmEntityCollection(r.kind)
	collectionPath := gtmWorkspaceCollectionPath(accountID, containerID, workspaceID, collection)
	payload := buildGTMPayload(ctx, r.kind, *plan)

	existing, found, err := r.client.getGTMCollectionItemByName(ctx, collectionPath, plan.Name.ValueString())
	if err != nil {
		return err
	}

	var out map[string]any
	if found {
		itemPath := stringFromMap(existing, "path")
		if err := r.client.doJSON(ctx, http.MethodPut, gtmURL(itemPath), payload, &out, nil); err != nil {
			return err
		}
		if len(out) == 0 {
			out = existing
		}
	} else if err := r.client.doJSON(ctx, http.MethodPost, gtmURL(collectionPath), payload, &out, nil); err != nil {
		return err
	}
	if itemPath := stringFromMap(out, "path"); itemPath != "" {
		r.client.updateGTMCollectionItem(collectionPath, itemPath, out)
	}

	plan.WorkspaceID = types.StringValue(workspaceID)
	applyGTMRemoteTypedFields(plan, r.kind, out)
	if plan.Path.ValueString() == "" {
		return fmt.Errorf("Google did not return a resource path for the created entity")
	}
	return nil
}

func (r *gtmTypedEntityResource) readEntity(ctx context.Context, state *gtmTypedWorkspaceEntityModel) (bool, error) {
	accountID, containerID := state.AccountID.ValueString(), state.ContainerID.ValueString()
	workspaceID, err := r.client.resolveGTMWorkspaceID(ctx, accountID, containerID, state.WorkspaceName.ValueString())
	if err != nil {
		return false, err
	}
	collection, _ := gtmEntityCollection(r.kind)
	collectionPath := gtmWorkspaceCollectionPath(accountID, containerID, workspaceID, collection)
	out, found, err := r.client.getGTMCollectionItemByID(ctx, collectionPath, r.kind+"Id", state.EntityID.ValueString())
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	state.WorkspaceID = types.StringValue(workspaceID)
	applyGTMRemoteTypedFields(state, r.kind, out)
	return true, nil
}

func (r *gtmTypedEntityResource) updateEntity(ctx context.Context, plan *gtmTypedWorkspaceEntityModel) error {
	if r.kind == "tag" {
		if err := validateGTMTagAdditionalRequirements(*plan); err != nil {
			return err
		}
	}

	accountID, containerID := plan.AccountID.ValueString(), plan.ContainerID.ValueString()
	workspaceID, err := r.client.resolveGTMWorkspaceID(ctx, accountID, containerID, plan.WorkspaceName.ValueString())
	if err != nil {
		return err
	}
	collection, _ := gtmEntityCollection(r.kind)
	collectionPath := gtmWorkspaceCollectionPath(accountID, containerID, workspaceID, collection)
	itemPath := gtmWorkspaceEntityItemPath(collectionPath, plan.EntityID.ValueString())

	payload := buildGTMPayload(ctx, r.kind, *plan)
	var out map[string]any
	if err := r.client.doJSON(ctx, http.MethodPut, gtmURL(itemPath), payload, &out, nil); err != nil {
		return err
	}
	if len(out) == 0 {
		if err := r.client.doJSON(ctx, http.MethodGet, gtmURL(itemPath), nil, &out, nil); err != nil {
			return err
		}
	}
	r.client.updateGTMCollectionItem(collectionPath, itemPath, out)

	plan.WorkspaceID = types.StringValue(workspaceID)
	applyGTMRemoteTypedFields(plan, r.kind, out)
	return nil
}

func (r *gtmTypedEntityResource) deleteEntity(ctx context.Context, state *gtmTypedWorkspaceEntityModel) error {
	accountID, containerID := state.AccountID.ValueString(), state.ContainerID.ValueString()
	workspaceID, err := r.client.resolveGTMWorkspaceID(ctx, accountID, containerID, state.WorkspaceName.ValueString())
	if err != nil {
		return err
	}
	collection, _ := gtmEntityCollection(r.kind)
	collectionPath := gtmWorkspaceCollectionPath(accountID, containerID, workspaceID, collection)
	itemPath := gtmWorkspaceEntityItemPath(collectionPath, state.EntityID.ValueString())

	if err := r.client.doJSON(ctx, http.MethodDelete, gtmURL(itemPath), nil, nil, nil); err != nil && !errors.Is(err, errNotFound) {
		return err
	}
	r.client.removeGTMCollectionItem(collectionPath, itemPath)
	return nil
}

func (r *gtmTypedEntityResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	accountID, containerID, entityID, workspaceIDHint, err := parseGTMEntityImportID(r.kind, req.ID)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Invalid GTM %s import ID", r.kind), err.Error())
		return
	}
	workspaceID := workspaceIDHint
	if workspaceID == "" {
		workspaceID, err = r.client.resolveGTMWorkspaceID(ctx, accountID, containerID, "")
		if err != nil {
			resp.Diagnostics.AddError("Unable to resolve GTM workspace", err.Error())
			return
		}
	}
	collection, _ := gtmEntityCollection(r.kind)
	collectionPath := gtmWorkspaceCollectionPath(accountID, containerID, workspaceID, collection)
	out, found, err := r.client.getGTMCollectionItemByID(ctx, collectionPath, r.kind+"Id", entityID)
	if err != nil {
		resp.Diagnostics.AddError(fmt.Sprintf("Unable to read imported GTM %s", r.kind), err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError(fmt.Sprintf("GTM %s not found", r.kind), fmt.Sprintf("no %s with ID %s was found in workspace %s", r.kind, entityID, workspaceID))
		return
	}
	state := gtmTypedWorkspaceEntityModel{
		AccountID:   types.StringValue(accountID),
		ContainerID: types.StringValue(containerID),
	}
	applyGTMRemoteTypedFields(&state, r.kind, out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func gtmWorkspaceEntityItemPath(collectionPath, entityID string) string {
	return strings.Trim(collectionPath, "/") + "/" + entityID
}

// parseGTMEntityImportID accepts either the stable form
// accounts/{account}/containers/{container}/{collection}/{id} or a
// workspace-scoped path copied from the GTM UI/API
// (accounts/{account}/containers/{container}/workspaces/{workspace}/{collection}/{id}).
// The latter yields a workspace ID hint so import doesn't need an extra
// lookup call.
func parseGTMEntityImportID(kind, raw string) (accountID, containerID, entityID, workspaceIDHint string, err error) {
	if parsed, parseErr := parseGTMWorkspaceEntityPath(raw); parseErr == nil {
		if parsed.Kind != kind {
			return "", "", "", "", fmt.Errorf("expected a %s path, got %s", kind, parsed.Kind)
		}
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		return parsed.AccountID, parsed.ContainerID, parts[len(parts)-1], parsed.WorkspaceID, nil
	}
	if parsed, parseErr := parseGTMContainerEntityID(raw); parseErr == nil {
		if parsed.Kind != kind {
			return "", "", "", "", fmt.Errorf("expected a %s path, got %s", kind, parsed.Kind)
		}
		return parsed.AccountID, parsed.ContainerID, parsed.EntityID, "", nil
	}
	return "", "", "", "", fmt.Errorf("expected accounts/{account_id}/containers/{container_id}/{collection}/{id} or accounts/{account_id}/containers/{container_id}/workspaces/{workspace_id}/{collection}/{id}")
}
