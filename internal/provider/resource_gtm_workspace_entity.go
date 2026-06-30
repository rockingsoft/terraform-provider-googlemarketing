package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*gtmWorkspaceEntityResource)(nil)
var _ resource.ResourceWithConfigure = (*gtmWorkspaceEntityResource)(nil)
var _ resource.ResourceWithImportState = (*gtmWorkspaceEntityResource)(nil)

func NewGTMWorkspaceEntityResource() resource.Resource {
	return &gtmWorkspaceEntityResource{}
}

type gtmWorkspaceEntityResource struct {
	client *marketingClient
}

type gtmWorkspaceEntityModel struct {
	ID          types.String `tfsdk:"id"`
	AccountID   types.String `tfsdk:"account_id"`
	ContainerID types.String `tfsdk:"container_id"`
	WorkspaceID types.String `tfsdk:"workspace_id"`
	Kind        types.String `tfsdk:"kind"`
	PayloadJSON types.String `tfsdk:"payload_json"`
	Path        types.String `tfsdk:"path"`
}

func (r *gtmWorkspaceEntityResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gtm_workspace_entity"
}

func (r *gtmWorkspaceEntityResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Legacy generic Google Tag Manager workspace entity. Prefer the typed googlemarketing_gtm_tag, googlemarketing_gtm_trigger, googlemarketing_gtm_variable, and googlemarketing_gtm_folder resources.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"account_id": schema.StringAttribute{
				Required:    true,
				Description: "GTM account ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"container_id": schema.StringAttribute{
				Required:    true,
				Description: "GTM container ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"workspace_id": schema.StringAttribute{
				Required:    true,
				Description: "GTM workspace ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"kind": schema.StringAttribute{
				Required:    true,
				Description: "Entity kind: tag, trigger, variable, or folder.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"payload_json": schema.StringAttribute{
				Required:    true,
				Description: "JSON object sent to the GTM API. The shape depends on kind.",
			},
			"path": schema.StringAttribute{
				Computed:    true,
				Description: "GTM API resource path returned by Google.",
			},
		},
	}
}

func (r *gtmWorkspaceEntityResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *gtmWorkspaceEntityResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan gtmWorkspaceEntityModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, err := decodeJSONObject(plan.PayloadJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("payload_json"), "Invalid JSON payload", err.Error())
		return
	}

	collection, err := gtmEntityCollection(plan.Kind.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("kind"), "Invalid GTM entity kind", err.Error())
		return
	}

	apiPath := fmt.Sprintf("accounts/%s/containers/%s/workspaces/%s/%s", plan.AccountID.ValueString(), plan.ContainerID.ValueString(), plan.WorkspaceID.ValueString(), collection)
	var out map[string]any
	if err := r.client.doJSON(ctx, http.MethodPost, gtmURL(apiPath), payload, &out, nil); err != nil {
		resp.Diagnostics.AddError("Unable to create GTM workspace entity", err.Error())
		return
	}

	resourcePath, _ := out["path"].(string)
	if resourcePath == "" {
		resp.Diagnostics.AddError("GTM response missing path", "Google did not return a resource path for the created entity.")
		return
	}

	plan.ID = types.StringValue(resourcePath)
	plan.Path = types.StringValue(resourcePath)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *gtmWorkspaceEntityResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state gtmWorkspaceEntityModel
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
		resp.Diagnostics.AddError("Unable to read GTM workspace entity", err.Error())
		return
	}
	raw, err := normalizeJSONValue(out)
	if err != nil {
		resp.Diagnostics.AddError("Unable to encode GTM workspace entity", err.Error())
		return
	}
	state.PayloadJSON = types.StringValue(raw)
	if resourcePath, _ := out["path"].(string); resourcePath != "" {
		state.ID = types.StringValue(resourcePath)
		state.Path = types.StringValue(resourcePath)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *gtmWorkspaceEntityResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan gtmWorkspaceEntityModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, err := decodeJSONObject(plan.PayloadJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("payload_json"), "Invalid JSON payload", err.Error())
		return
	}

	if err := r.client.doJSON(ctx, http.MethodPut, gtmURL(plan.Path.ValueString()), payload, nil, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update GTM workspace entity", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *gtmWorkspaceEntityResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state gtmWorkspaceEntityModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.doJSON(ctx, http.MethodDelete, gtmURL(state.Path.ValueString()), nil, nil, nil)
	if err != nil && !errors.Is(err, errNotFound) {
		resp.Diagnostics.AddError("Unable to delete GTM workspace entity", err.Error())
	}
}

func (r *gtmWorkspaceEntityResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parsed, err := parseGTMWorkspaceEntityPath(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid GTM workspace entity import ID", err.Error())
		return
	}
	var out map[string]any
	if err := r.client.doJSON(ctx, http.MethodGet, gtmURL(parsed.Path), nil, &out, nil); err != nil {
		resp.Diagnostics.AddError("Unable to read imported GTM workspace entity", err.Error())
		return
	}
	raw, err := normalizeJSONValue(out)
	if err != nil {
		resp.Diagnostics.AddError("Unable to encode imported GTM workspace entity", err.Error())
		return
	}
	state := gtmWorkspaceEntityModel{
		ID:          types.StringValue(parsed.Path),
		AccountID:   types.StringValue(parsed.AccountID),
		ContainerID: types.StringValue(parsed.ContainerID),
		WorkspaceID: types.StringValue(parsed.WorkspaceID),
		Kind:        types.StringValue(parsed.Kind),
		PayloadJSON: types.StringValue(raw),
		Path:        types.StringValue(parsed.Path),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func gtmEntityCollection(kind string) (string, error) {
	switch kind {
	case "tag":
		return "tags", nil
	case "trigger":
		return "triggers", nil
	case "variable":
		return "variables", nil
	case "folder":
		return "folders", nil
	default:
		return "", fmt.Errorf("supported values are tag, trigger, variable, folder")
	}
}
