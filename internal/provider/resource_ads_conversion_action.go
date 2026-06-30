package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*adsConversionActionResource)(nil)
var _ resource.ResourceWithConfigure = (*adsConversionActionResource)(nil)
var _ resource.ResourceWithImportState = (*adsConversionActionResource)(nil)

func NewAdsConversionActionResource() resource.Resource {
	return &adsConversionActionResource{}
}

type adsConversionActionResource struct {
	client *marketingClient
}

type adsConversionActionModel struct {
	ID           types.String `tfsdk:"id"`
	CustomerID   types.String `tfsdk:"customer_id"`
	PayloadJSON  types.String `tfsdk:"payload_json"`
	ResourceName types.String `tfsdk:"resource_name"`
}

func (r *adsConversionActionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ads_conversion_action"
}

func (r *adsConversionActionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Declarative Google Ads conversion action resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
			"customer_id": schema.StringAttribute{
				Required:    true,
				Description: "Google Ads customer ID without dashes.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"payload_json": schema.StringAttribute{
				Required:    true,
				Description: "Google Ads ConversionAction JSON object.",
			},
			"resource_name": schema.StringAttribute{
				Computed:    true,
				Description: "Google Ads conversion action resource name.",
			},
		},
	}
}

func (r *adsConversionActionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *adsConversionActionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan adsConversionActionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload, err := decodeJSONObject(plan.PayloadJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("payload_json"), "Invalid JSON payload", err.Error())
		return
	}
	out, err := r.mutate(ctx, plan.CustomerID.ValueString(), map[string]any{"create": payload})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Google Ads conversion action", err.Error())
		return
	}
	resourceName := adsMutateResourceName(out)
	if resourceName == "" {
		resp.Diagnostics.AddError("Google Ads response missing resourceName", "Google did not return a conversion action resource name.")
		return
	}
	plan.ID = types.StringValue(resourceName)
	plan.ResourceName = types.StringValue(resourceName)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *adsConversionActionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state adsConversionActionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	remote, err := r.readRemote(ctx, state.CustomerID.ValueString(), state.ResourceName.ValueString())
	if errors.Is(err, errNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Google Ads conversion action", err.Error())
		return
	}
	raw, err := normalizeJSONValue(remote)
	if err != nil {
		resp.Diagnostics.AddError("Unable to encode Google Ads conversion action", err.Error())
		return
	}
	state.PayloadJSON = types.StringValue(raw)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *adsConversionActionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan adsConversionActionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload, err := decodeJSONObject(plan.PayloadJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("payload_json"), "Invalid JSON payload", err.Error())
		return
	}
	payload["resourceName"] = plan.ResourceName.ValueString()
	op := map[string]any{
		"update":     payload,
		"updateMask": strings.ReplaceAll(updateMaskFromPayload(plan.PayloadJSON.ValueString()), ",", ","),
	}
	if _, err := r.mutate(ctx, plan.CustomerID.ValueString(), op); err != nil {
		resp.Diagnostics.AddError("Unable to update Google Ads conversion action", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *adsConversionActionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state adsConversionActionModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, err := r.mutate(ctx, state.CustomerID.ValueString(), map[string]any{"remove": state.ResourceName.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Unable to remove Google Ads conversion action", err.Error())
	}
}

func (r *adsConversionActionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid Google Ads conversion action import ID", "Expected {customer_id}/{conversion_action_resource_name}.")
		return
	}
	customerID := parts[0]
	resourceName := parts[1]
	remote, err := r.readRemote(ctx, customerID, resourceName)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read imported Google Ads conversion action", err.Error())
		return
	}
	raw, err := normalizeJSONValue(remote)
	if err != nil {
		resp.Diagnostics.AddError("Unable to encode imported Google Ads conversion action", err.Error())
		return
	}
	state := adsConversionActionModel{
		ID:           types.StringValue(resourceName),
		CustomerID:   types.StringValue(customerID),
		PayloadJSON:  types.StringValue(raw),
		ResourceName: types.StringValue(resourceName),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *adsConversionActionResource) mutate(ctx context.Context, customerID string, operation map[string]any) (map[string]any, error) {
	body := map[string]any{
		"operations": []any{
			map[string]any{"conversionActionOperation": operation},
		},
	}
	var out map[string]any
	if err := r.client.doJSON(ctx, http.MethodPost, r.client.adsURL(customerID, "conversionActions:mutate"), body, &out, r.client.adsHeaders()); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *adsConversionActionResource) readRemote(ctx context.Context, customerID, resourceName string) (map[string]any, error) {
	body := map[string]any{
		"query": fmt.Sprintf("SELECT conversion_action.resource_name, conversion_action.name, conversion_action.status, conversion_action.type, conversion_action.category, conversion_action.value_settings, conversion_action.counting_type, conversion_action.click_through_lookback_window_days, conversion_action.view_through_lookback_window_days FROM conversion_action WHERE conversion_action.resource_name = '%s'", resourceName),
	}
	var out struct {
		Results []struct {
			ConversionAction map[string]any `json:"conversionAction"`
		} `json:"results"`
	}
	if err := r.client.doJSON(ctx, http.MethodPost, r.client.adsURL(customerID, "googleAds:search"), body, &out, r.client.adsHeaders()); err != nil {
		return nil, err
	}
	if len(out.Results) == 0 {
		return nil, errNotFound
	}
	return out.Results[0].ConversionAction, nil
}

func adsMutateResourceName(out map[string]any) string {
	results, _ := out["results"].([]any)
	if len(results) == 0 {
		return ""
	}
	first, _ := results[0].(map[string]any)
	resourceName, _ := first["resourceName"].(string)
	return resourceName
}
