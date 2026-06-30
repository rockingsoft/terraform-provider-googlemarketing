package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*gtmGoogleTagConfigResource)(nil)
var _ resource.ResourceWithConfigure = (*gtmGoogleTagConfigResource)(nil)
var _ resource.ResourceWithImportState = (*gtmGoogleTagConfigResource)(nil)

func NewGTMGoogleTagConfigResource() resource.Resource {
	return &gtmGoogleTagConfigResource{}
}

type gtmGoogleTagConfigResource struct {
	client *marketingClient
}

type gtmGoogleTagConfigModel struct {
	ID           types.String `tfsdk:"id"`
	AccountID    types.String `tfsdk:"account_id"`
	ContainerID  types.String `tfsdk:"container_id"`
	WorkspaceID  types.String `tfsdk:"workspace_id"`
	Path         types.String `tfsdk:"path"`
	GtagConfigID types.String `tfsdk:"gtag_config_id"`
	Type         types.String `tfsdk:"type"`
	TagID        types.String `tfsdk:"tag_id"`
}

func (r *gtmGoogleTagConfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gtm_google_tag_config"
}

func (r *gtmGoogleTagConfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Google Tag Manager Google tag configuration.",
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Computed: true},
			"path":           schema.StringAttribute{Computed: true, Description: "GTM Google tag config API resource path returned by Google."},
			"gtag_config_id": schema.StringAttribute{Computed: true, Description: "Short Google tag config ID returned by Google."},
			"account_id":     replaceStringAttribute(),
			"container_id":   replaceStringAttribute(),
			"workspace_id":   replaceStringAttribute(),
			"tag_id":         schema.StringAttribute{Required: true, Description: "Google tag ID, for example a GA4 web stream measurement ID."},
			"type":           schema.StringAttribute{Optional: true, Computed: true, Default: stringdefault.StaticString("google"), Description: "Google tag config type. Defaults to google."},
		},
	}
}

func (r *gtmGoogleTagConfigResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *gtmGoogleTagConfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan gtmGoogleTagConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	apiPath := fmt.Sprintf("accounts/%s/containers/%s/workspaces/%s/gtag_config", plan.AccountID.ValueString(), plan.ContainerID.ValueString(), plan.WorkspaceID.ValueString())
	var out map[string]any
	if err := r.client.doJSON(ctx, http.MethodPost, gtmURL(apiPath), buildGTMGoogleTagConfigPayload(plan), &out, nil); err != nil {
		resp.Diagnostics.AddError("Unable to create GTM Google tag config", err.Error())
		return
	}
	applyGTMGoogleTagConfigRemote(&plan, out)
	if plan.Path.ValueString() == "" {
		resp.Diagnostics.AddError("GTM response missing path", "Google did not return a resource path for the created Google tag config.")
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *gtmGoogleTagConfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state gtmGoogleTagConfigModel
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
		resp.Diagnostics.AddError("Unable to read GTM Google tag config", err.Error())
		return
	}
	applyGTMGoogleTagConfigRemote(&state, out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *gtmGoogleTagConfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan gtmGoogleTagConfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var out map[string]any
	if err := r.client.doJSON(ctx, http.MethodPut, gtmURL(plan.Path.ValueString()), buildGTMGoogleTagConfigPayload(plan), &out, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update GTM Google tag config", err.Error())
		return
	}
	applyGTMGoogleTagConfigRemote(&plan, out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *gtmGoogleTagConfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state gtmGoogleTagConfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.doJSON(ctx, http.MethodDelete, gtmURL(state.Path.ValueString()), nil, nil, nil)
	if err != nil && !errors.Is(err, errNotFound) {
		resp.Diagnostics.AddError("Unable to delete GTM Google tag config", err.Error())
	}
}

func (r *gtmGoogleTagConfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parsed, err := parseGTMGoogleTagConfigPath(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid GTM Google tag config import ID", err.Error())
		return
	}
	var out map[string]any
	if err := r.client.doJSON(ctx, http.MethodGet, gtmURL(parsed.Path), nil, &out, nil); err != nil {
		resp.Diagnostics.AddError("Unable to read imported GTM Google tag config", err.Error())
		return
	}
	state := gtmGoogleTagConfigModel{
		ID:           types.StringValue(parsed.Path),
		AccountID:    types.StringValue(parsed.AccountID),
		ContainerID:  types.StringValue(parsed.ContainerID),
		WorkspaceID:  types.StringValue(parsed.WorkspaceID),
		Path:         types.StringValue(parsed.Path),
		GtagConfigID: types.StringValue(parsed.GtagConfigID),
	}
	applyGTMGoogleTagConfigRemote(&state, out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func buildGTMGoogleTagConfigPayload(m gtmGoogleTagConfigModel) map[string]any {
	typeName := m.Type.ValueString()
	if typeName == "" {
		typeName = "google"
	}
	return map[string]any{
		"type": typeName,
		"parameter": []any{
			map[string]any{"type": "template", "key": "tagId", "value": m.TagID.ValueString()},
		},
	}
}

func applyGTMGoogleTagConfigRemote(m *gtmGoogleTagConfigModel, out map[string]any) {
	if pathValue := stringFromMap(out, "path"); pathValue != "" {
		m.ID = types.StringValue(pathValue)
		m.Path = types.StringValue(pathValue)
	}
	if id := stringFromMap(out, "gtagConfigId"); id != "" {
		m.GtagConfigID = types.StringValue(id)
	}
	if typeName := stringFromMap(out, "type"); typeName != "" {
		m.Type = types.StringValue(typeName)
	} else if m.Type.IsNull() || m.Type.IsUnknown() {
		m.Type = types.StringValue("google")
	}
	if tagID := gtmParamMap(out["parameter"])["tagId"]; tagID != "" {
		m.TagID = types.StringValue(tagID)
	}
}
