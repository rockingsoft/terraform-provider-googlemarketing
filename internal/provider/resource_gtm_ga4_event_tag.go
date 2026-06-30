package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*gtmGA4EventTagResource)(nil)
var _ resource.ResourceWithConfigure = (*gtmGA4EventTagResource)(nil)
var _ resource.ResourceWithImportState = (*gtmGA4EventTagResource)(nil)

func NewGTMGA4EventTagResource() resource.Resource {
	return &gtmGA4EventTagResource{}
}

type gtmGA4EventTagResource struct {
	client *marketingClient
}

type gtmGA4EventTagModel struct {
	ID                    types.String `tfsdk:"id"`
	AccountID             types.String `tfsdk:"account_id"`
	ContainerID           types.String `tfsdk:"container_id"`
	WorkspaceID           types.String `tfsdk:"workspace_id"`
	Name                  types.String `tfsdk:"name"`
	EventName             types.String `tfsdk:"event_name"`
	MeasurementIDOverride types.String `tfsdk:"measurement_id_override"`
	Notes                 types.String `tfsdk:"notes"`
	EntityID              types.String `tfsdk:"entity_id"`
	Path                  types.String `tfsdk:"path"`
	TriggerIDs            types.List   `tfsdk:"trigger_ids"`
	BlockingTriggerIDs    types.List   `tfsdk:"blocking_trigger_ids"`
}

func (r *gtmGA4EventTagResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gtm_ga4_event_tag"
}

func (r *gtmGA4EventTagResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Google Tag Manager GA4 event tag. The base Google tag configuration should be managed with googlemarketing_gtm_google_tag_config.",
		Attributes: map[string]schema.Attribute{
			"id":                      schema.StringAttribute{Computed: true},
			"entity_id":               schema.StringAttribute{Computed: true, Description: "Short GTM tag ID returned by Google."},
			"path":                    schema.StringAttribute{Computed: true, Description: "GTM tag API resource path returned by Google."},
			"account_id":              replaceStringAttribute(),
			"container_id":            replaceStringAttribute(),
			"workspace_id":            replaceStringAttribute(),
			"name":                    schema.StringAttribute{Required: true},
			"event_name":              schema.StringAttribute{Required: true, Description: "GA4 event name."},
			"measurement_id_override": schema.StringAttribute{Optional: true, Description: "Transitional override for legacy gaawe tags. Prefer googlemarketing_gtm_google_tag_config for the base Measurement ID."},
			"notes":                   schema.StringAttribute{Optional: true},
			"trigger_ids":             schema.ListAttribute{Required: true, ElementType: types.StringType},
			"blocking_trigger_ids":    schema.ListAttribute{Optional: true, ElementType: types.StringType},
		},
	}
}

func (r *gtmGA4EventTagResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *gtmGA4EventTagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan gtmGA4EventTagModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiPath := fmt.Sprintf("accounts/%s/containers/%s/workspaces/%s/tags", plan.AccountID.ValueString(), plan.ContainerID.ValueString(), plan.WorkspaceID.ValueString())
	var out map[string]any
	if err := r.client.doJSON(ctx, http.MethodPost, gtmURL(apiPath), buildGTMGA4EventTagPayload(ctx, plan), &out, nil); err != nil {
		resp.Diagnostics.AddError("Unable to create GTM GA4 event tag", err.Error())
		return
	}
	applyGTMGA4EventTagRemote(&plan, out)
	if plan.Path.ValueString() == "" {
		resp.Diagnostics.AddError("GTM response missing path", "Google did not return a resource path for the created GA4 event tag.")
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *gtmGA4EventTagResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state gtmGA4EventTagModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out map[string]any
	err := r.client.doJSON(ctx, http.MethodGet, gtmURL(state.Path.ValueString()), nil, &out, nil)
	if errors.Is(err, errNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read GTM GA4 event tag", err.Error())
		return
	}
	applyGTMGA4EventTagRemote(&state, out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *gtmGA4EventTagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan gtmGA4EventTagModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out map[string]any
	if err := r.client.doJSON(ctx, http.MethodPut, gtmURL(plan.Path.ValueString()), buildGTMGA4EventTagPayload(ctx, plan), &out, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update GTM GA4 event tag", err.Error())
		return
	}
	if len(out) == 0 {
		if err := r.client.doJSON(ctx, http.MethodGet, gtmURL(plan.Path.ValueString()), nil, &out, nil); err != nil {
			resp.Diagnostics.AddError("Unable to read updated GTM GA4 event tag", err.Error())
			return
		}
	}
	applyGTMGA4EventTagRemote(&plan, out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *gtmGA4EventTagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state gtmGA4EventTagModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.doJSON(ctx, http.MethodDelete, gtmURL(state.Path.ValueString()), nil, nil, nil)
	if err != nil && !errors.Is(err, errNotFound) {
		resp.Diagnostics.AddError("Unable to delete GTM GA4 event tag", err.Error())
	}
}

func (r *gtmGA4EventTagResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parsed, err := parseGTMWorkspaceEntityPath(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid GTM GA4 event tag import ID", err.Error())
		return
	}
	if parsed.Kind != "tag" {
		resp.Diagnostics.AddError("Invalid GTM GA4 event tag import ID", fmt.Sprintf("Expected tag path, got %s.", parsed.Kind))
		return
	}
	var out map[string]any
	if err := r.client.doJSON(ctx, http.MethodGet, gtmURL(parsed.Path), nil, &out, nil); err != nil {
		resp.Diagnostics.AddError("Unable to read imported GTM GA4 event tag", err.Error())
		return
	}
	if typeName := stringFromMap(out, "type"); typeName != "gaawe" {
		resp.Diagnostics.AddError("Invalid GTM GA4 event tag import ID", fmt.Sprintf("Expected tag type gaawe, got %q.", typeName))
		return
	}
	state := gtmGA4EventTagModel{
		ID:          types.StringValue(parsed.Path),
		AccountID:   types.StringValue(parsed.AccountID),
		ContainerID: types.StringValue(parsed.ContainerID),
		WorkspaceID: types.StringValue(parsed.WorkspaceID),
		Path:        types.StringValue(parsed.Path),
	}
	applyGTMGA4EventTagRemote(&state, out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func buildGTMGA4EventTagPayload(ctx context.Context, m gtmGA4EventTagModel) map[string]any {
	payload := map[string]any{
		"name":            m.Name.ValueString(),
		"type":            "gaawe",
		"firingTriggerId": stringList(ctx, m.TriggerIDs),
	}
	if ids := stringList(ctx, m.BlockingTriggerIDs); len(ids) > 0 {
		payload["blockingTriggerId"] = ids
	}
	if !m.Notes.IsNull() && !m.Notes.IsUnknown() && m.Notes.ValueString() != "" {
		payload["notes"] = m.Notes.ValueString()
	}
	params := []any{}
	params = appendGTMParam(params, "eventName", m.EventName)
	params = appendGTMParam(params, "measurementIdOverride", m.MeasurementIDOverride)
	if len(params) > 0 {
		payload["parameter"] = params
	}
	return payload
}

func applyGTMGA4EventTagRemote(m *gtmGA4EventTagModel, out map[string]any) {
	if pathValue := stringFromMap(out, "path"); pathValue != "" {
		m.ID = types.StringValue(pathValue)
		m.Path = types.StringValue(pathValue)
	}
	if id := stringFromMap(out, "tagId"); id != "" {
		m.EntityID = types.StringValue(id)
	}
	if name := stringFromMap(out, "name"); name != "" {
		m.Name = types.StringValue(name)
	}
	if notes := stringFromMap(out, "notes"); notes != "" {
		m.Notes = types.StringValue(notes)
	} else {
		m.Notes = types.StringNull()
	}
	m.TriggerIDs = stringListValueAllowEmpty(stringsFromAny(out["firingTriggerId"]))
	m.BlockingTriggerIDs = stringListValue(stringsFromAny(out["blockingTriggerId"]))
	params := gtmParamMap(out["parameter"])
	if value := params["eventName"]; value != "" {
		m.EventName = types.StringValue(value)
	}
	if value := params["measurementIdOverride"]; value != "" {
		m.MeasurementIDOverride = types.StringValue(value)
	} else {
		m.MeasurementIDOverride = types.StringNull()
	}
}
