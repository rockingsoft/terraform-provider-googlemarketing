package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*adsMutateResource)(nil)
var _ resource.ResourceWithConfigure = (*adsMutateResource)(nil)

func NewAdsMutateResource() resource.Resource {
	return &adsMutateResource{}
}

type adsMutateResource struct {
	client *marketingClient
}

type adsMutateModel struct {
	ID                   types.String `tfsdk:"id"`
	CustomerID           types.String `tfsdk:"customer_id"`
	OperationsJSON       types.String `tfsdk:"operations_json"`
	RemoveOperationsJSON types.String `tfsdk:"remove_operations_json"`
	ResponseJSON         types.String `tfsdk:"response_json"`
}

func (r *adsMutateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ads_mutate"
}

func (r *adsMutateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Generic Google Ads mutate resource. It can create or update any campaign, asset, criterion, or related entity supported by googleAds:mutate.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
			"customer_id": schema.StringAttribute{
				Required:    true,
				Description: "Google Ads customer ID without dashes.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"operations_json": schema.StringAttribute{
				Required:    true,
				Description: "JSON array used as mutateOperations in customers.googleAds:mutate.",
			},
			"remove_operations_json": schema.StringAttribute{
				Optional:    true,
				Description: "Optional JSON array sent on Terraform destroy, typically remove operations.",
			},
			"response_json": schema.StringAttribute{
				Computed:    true,
				Description: "Raw Google Ads mutate response JSON.",
			},
		},
	}
}

func (r *adsMutateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *adsMutateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan adsMutateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	responseJSON, err := r.mutate(ctx, plan.CustomerID.ValueString(), plan.OperationsJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to mutate Google Ads resources", err.Error())
		return
	}

	plan.ID = types.StringValue(plan.CustomerID.ValueString() + ":" + hashString(plan.OperationsJSON.ValueString()))
	plan.ResponseJSON = types.StringValue(responseJSON)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *adsMutateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state adsMutateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *adsMutateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan adsMutateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	responseJSON, err := r.mutate(ctx, plan.CustomerID.ValueString(), plan.OperationsJSON.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to update Google Ads resources", err.Error())
		return
	}

	plan.ID = types.StringValue(plan.CustomerID.ValueString() + ":" + hashString(plan.OperationsJSON.ValueString()))
	plan.ResponseJSON = types.StringValue(responseJSON)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *adsMutateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state adsMutateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if state.RemoveOperationsJSON.IsNull() || state.RemoveOperationsJSON.ValueString() == "" {
		return
	}
	if _, err := r.mutate(ctx, state.CustomerID.ValueString(), state.RemoveOperationsJSON.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to remove Google Ads resources", err.Error())
	}
}

func (r *adsMutateResource) mutate(ctx context.Context, customerID string, operationsJSON string) (string, error) {
	var operations []any
	if err := jsonUnmarshalString(operationsJSON, &operations); err != nil {
		return "", fmt.Errorf("operations_json must be a JSON array: %w", err)
	}

	body := map[string]any{
		"mutateOperations": operations,
	}
	var out map[string]any
	if err := r.client.doJSON(ctx, http.MethodPost, r.client.adsURL(customerID, "googleAds:mutate"), body, &out, r.client.adsHeaders()); err != nil {
		return "", err
	}
	return encodeJSON(out)
}

func hashString(s string) string {
	h := uint32(2166136261)
	for _, c := range []byte(s) {
		h ^= uint32(c)
		h *= 16777619
	}
	return fmt.Sprintf("%08x", h)
}

func jsonUnmarshalString(raw string, dst any) error {
	return json.Unmarshal([]byte(raw), dst)
}
