package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*ga4AdminResource)(nil)
var _ resource.ResourceWithConfigure = (*ga4AdminResource)(nil)
var _ resource.ResourceWithImportState = (*ga4AdminResource)(nil)

func NewGA4AdminResource() resource.Resource {
	return &ga4AdminResource{}
}

type ga4AdminResource struct {
	client *marketingClient
}

type ga4AdminModel struct {
	ID          types.String `tfsdk:"id"`
	Parent      types.String `tfsdk:"parent"`
	Collection  types.String `tfsdk:"collection"`
	PayloadJSON types.String `tfsdk:"payload_json"`
	Name        types.String `tfsdk:"name"`
}

func (r *ga4AdminResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ga4_admin_resource"
}

func (r *ga4AdminResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Legacy generic GA4 Admin API child resource. Prefer typed GA4 resources such as googlemarketing_ga4_custom_dimension, googlemarketing_ga4_custom_metric, and googlemarketing_ga4_key_event.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
			"parent": schema.StringAttribute{
				Required:    true,
				Description: "GA4 Admin parent path, for example properties/1234.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"collection": schema.StringAttribute{
				Required:    true,
				Description: "GA4 Admin collection under parent, for example customDimensions, customMetrics, keyEvents, eventCreateRules, eventEditRules, or audiences.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"payload_json": schema.StringAttribute{
				Required:    true,
				Description: "JSON object sent to the GA4 Admin API.",
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "GA4 Admin resource name returned by Google.",
			},
		},
	}
}

func (r *ga4AdminResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ga4AdminResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ga4AdminModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, err := decodeJSONObject(plan.PayloadJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("payload_json"), "Invalid JSON payload", err.Error())
		return
	}

	apiPath := fmt.Sprintf("%s/%s", plan.Parent.ValueString(), plan.Collection.ValueString())
	var out map[string]any
	if err := r.client.doJSON(ctx, http.MethodPost, gaURL(apiPath), payload, &out, nil); err != nil {
		resp.Diagnostics.AddError("Unable to create GA4 Admin resource", err.Error())
		return
	}

	name, _ := out["name"].(string)
	if name == "" {
		resp.Diagnostics.AddError("GA4 response missing name", "Google did not return a resource name.")
		return
	}

	plan.ID = types.StringValue(name)
	plan.Name = types.StringValue(name)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ga4AdminResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ga4AdminModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var out map[string]any
	err := r.client.doJSON(ctx, http.MethodGet, gaURL(state.Name.ValueString()), nil, &out, nil)
	if errors.Is(err, errNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read GA4 Admin resource", err.Error())
		return
	}
	raw, err := normalizeJSONValue(out)
	if err != nil {
		resp.Diagnostics.AddError("Unable to encode GA4 Admin resource", err.Error())
		return
	}
	state.PayloadJSON = types.StringValue(raw)
	if name, _ := out["name"].(string); name != "" {
		state.ID = types.StringValue(name)
		state.Name = types.StringValue(name)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ga4AdminResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ga4AdminModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, err := decodeJSONObject(plan.PayloadJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("payload_json"), "Invalid JSON payload", err.Error())
		return
	}

	mask := updateMaskFromPayload(plan.PayloadJSON.ValueString())
	apiURL := gaURL(plan.Name.ValueString())
	if mask != "" {
		apiURL += "?updateMask=" + url.QueryEscape(mask)
	}
	if err := r.client.doJSON(ctx, http.MethodPatch, apiURL, payload, nil, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update GA4 Admin resource", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ga4AdminResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ga4AdminModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.doJSON(ctx, http.MethodDelete, gaURL(state.Name.ValueString()), nil, nil, nil)
	if err != nil && !errors.Is(err, errNotFound) {
		resp.Diagnostics.AddError("Unable to delete GA4 Admin resource", err.Error())
	}
}

func (r *ga4AdminResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parsed, err := parseGA4AdminPath(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid GA4 Admin resource import ID", err.Error())
		return
	}
	var out map[string]any
	if err := r.client.doJSON(ctx, http.MethodGet, gaURL(parsed.Name), nil, &out, nil); err != nil {
		resp.Diagnostics.AddError("Unable to read imported GA4 Admin resource", err.Error())
		return
	}
	raw, err := normalizeJSONValue(out)
	if err != nil {
		resp.Diagnostics.AddError("Unable to encode imported GA4 Admin resource", err.Error())
		return
	}
	state := ga4AdminModel{
		ID:          types.StringValue(parsed.Name),
		Parent:      types.StringValue(parsed.Parent),
		Collection:  types.StringValue(parsed.Collection),
		PayloadJSON: types.StringValue(raw),
		Name:        types.StringValue(parsed.Name),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
