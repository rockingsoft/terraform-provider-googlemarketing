package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
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
	ID                             types.String `tfsdk:"id"`
	CustomerID                     types.String `tfsdk:"customer_id"`
	Name                           types.String `tfsdk:"name"`
	Type                           types.String `tfsdk:"type"`
	Category                       types.String `tfsdk:"category"`
	Status                         types.String `tfsdk:"status"`
	CountingType                   types.String `tfsdk:"counting_type"`
	ClickThroughLookbackWindowDays types.Int64  `tfsdk:"click_through_lookback_window_days"`
	ViewThroughLookbackWindowDays  types.Int64  `tfsdk:"view_through_lookback_window_days"`
	ResourceName                   types.String `tfsdk:"resource_name"`
	TagSnippets                    types.List   `tfsdk:"tag_snippets"`
	SendTo                         types.String `tfsdk:"send_to"`
	ConversionID                   types.String `tfsdk:"conversion_id"`
	ConversionLabel                types.String `tfsdk:"conversion_label"`
}

type adsTagSnippetModel struct {
	Type          types.String `tfsdk:"type"`
	PageFormat    types.String `tfsdk:"page_format"`
	GlobalSiteTag types.String `tfsdk:"global_site_tag"`
	EventSnippet  types.String `tfsdk:"event_snippet"`
}

var adsTagSnippetAttrTypes = map[string]attr.Type{
	"type":            types.StringType,
	"page_format":     types.StringType,
	"global_site_tag": types.StringType,
	"event_snippet":   types.StringType,
}

var adsTagSnippetObjectType = types.ObjectType{AttrTypes: adsTagSnippetAttrTypes}

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
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Google Ads conversion action name.",
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "Google Ads conversion action type, for example WEBPAGE.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"category": schema.StringAttribute{
				Required:    true,
				Description: "Google Ads conversion action category, for example SUBMIT_LEAD_FORM or SIGNUP.",
			},
			"status": schema.StringAttribute{
				Required:    true,
				Description: "Google Ads conversion action status, for example ENABLED.",
			},
			"counting_type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Google Ads conversion action counting type.",
			},
			"click_through_lookback_window_days": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Click-through lookback window in days.",
			},
			"view_through_lookback_window_days": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "View-through lookback window in days.",
			},
			"resource_name": schema.StringAttribute{
				Computed:    true,
				Description: "Google Ads conversion action resource name.",
			},
			"tag_snippets": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Google Ads tag snippets returned for this conversion action.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Computed:    true,
							Description: "Tracking code type, for example WEBPAGE or WEBPAGE_ONCLICK.",
						},
						"page_format": schema.StringAttribute{
							Computed:    true,
							Description: "Tracking code page format, for example HTML or AMP.",
						},
						"global_site_tag": schema.StringAttribute{
							Computed:    true,
							Description: "Global site tag snippet returned by Google Ads.",
						},
						"event_snippet": schema.StringAttribute{
							Computed:    true,
							Description: "Event snippet returned by Google Ads.",
						},
					},
				},
			},
			"send_to": schema.StringAttribute{
				Computed:    true,
				Description: "Google Ads GTM send_to value in AW-.../<label> format.",
			},
			"conversion_id": schema.StringAttribute{
				Computed:    true,
				Description: "Google Ads conversion ID in AW-... format.",
			},
			"conversion_label": schema.StringAttribute{
				Computed:    true,
				Description: "Google Ads conversion label.",
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
	out, err := r.mutate(ctx, plan.CustomerID.ValueString(), map[string]any{"create": buildAdsConversionActionPayload(plan, false)})
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
	remote, err := r.readRemote(ctx, plan.CustomerID.ValueString(), resourceName)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read created Google Ads conversion action", err.Error())
		return
	}
	applyAdsConversionActionRemote(&plan, remote)
	addAdsSnippetWarning(&resp.Diagnostics, plan)
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
	applyAdsConversionActionRemote(&state, remote)
	addAdsSnippetWarning(&resp.Diagnostics, state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *adsConversionActionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan adsConversionActionModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	op := map[string]any{
		"update":     buildAdsConversionActionPayload(plan, true),
		"updateMask": strings.Join(adsConversionActionUpdateFields(plan), ","),
	}
	if _, err := r.mutate(ctx, plan.CustomerID.ValueString(), op); err != nil {
		resp.Diagnostics.AddError("Unable to update Google Ads conversion action", err.Error())
		return
	}
	remote, err := r.readRemote(ctx, plan.CustomerID.ValueString(), plan.ResourceName.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read updated Google Ads conversion action", err.Error())
		return
	}
	applyAdsConversionActionRemote(&plan, remote)
	addAdsSnippetWarning(&resp.Diagnostics, plan)
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
	customerID, resourceName, err := parseAdsConversionActionImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Google Ads conversion action import ID", err.Error())
		return
	}
	remote, err := r.readRemote(ctx, customerID, resourceName)
	if err != nil {
		resp.Diagnostics.AddError("Unable to read imported Google Ads conversion action", err.Error())
		return
	}
	state := adsConversionActionModel{
		ID:           types.StringValue(resourceName),
		CustomerID:   types.StringValue(customerID),
		ResourceName: types.StringValue(resourceName),
	}
	applyAdsConversionActionRemote(&state, remote)
	addAdsSnippetWarning(&resp.Diagnostics, state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func parseAdsConversionActionImportID(raw string) (string, string, error) {
	if customerID, conversionActionID, ok := strings.Cut(raw, "."); ok {
		if customerID == "" || conversionActionID == "" || strings.Contains(conversionActionID, ".") {
			return "", "", fmt.Errorf("expected {customer_id}.{conversion_action_id} or {customer_id}/{conversion_action_resource_name}")
		}
		return customerID, fmt.Sprintf("customers/%s/conversionActions/%s", customerID, conversionActionID), nil
	}

	customerID, resourceName, ok := strings.Cut(raw, "/")
	if !ok || customerID == "" || resourceName == "" {
		return "", "", fmt.Errorf("expected {customer_id}.{conversion_action_id} or {customer_id}/{conversion_action_resource_name}")
	}
	return customerID, resourceName, nil
}

func (r *adsConversionActionResource) mutate(ctx context.Context, customerID string, operation map[string]any) (map[string]any, error) {
	unlock := r.client.adsMutationLocks.lock(customerID)
	defer unlock()

	var out map[string]any
	if err := r.client.doJSON(ctx, http.MethodPost, r.client.adsURL(customerID, "conversionActions:mutate"), adsConversionActionMutateBody(operation), &out, r.client.adsHeaders()); err != nil {
		return nil, err
	}
	return out, nil
}

func adsConversionActionMutateBody(operation map[string]any) map[string]any {
	return map[string]any{
		"operations": []any{operation},
	}
}

func (r *adsConversionActionResource) readRemote(ctx context.Context, customerID, resourceName string) (map[string]any, error) {
	body := map[string]any{
		"query": adsConversionActionReadQuery(resourceName),
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

func adsConversionActionReadQuery(resourceName string) string {
	return fmt.Sprintf("SELECT conversion_action.resource_name, conversion_action.name, conversion_action.status, conversion_action.type, conversion_action.category, conversion_action.counting_type, conversion_action.click_through_lookback_window_days, conversion_action.view_through_lookback_window_days, conversion_action.tag_snippets FROM conversion_action WHERE conversion_action.resource_name = '%s'", resourceName)
}

func buildAdsConversionActionPayload(m adsConversionActionModel, includeResourceName bool) map[string]any {
	payload := map[string]any{}
	if includeResourceName {
		payload["resourceName"] = m.ResourceName.ValueString()
	}
	putString(payload, "name", m.Name)
	putString(payload, "type", m.Type)
	putString(payload, "category", m.Category)
	putString(payload, "status", m.Status)
	putString(payload, "countingType", m.CountingType)
	if !m.ClickThroughLookbackWindowDays.IsNull() && !m.ClickThroughLookbackWindowDays.IsUnknown() {
		payload["clickThroughLookbackWindowDays"] = m.ClickThroughLookbackWindowDays.ValueInt64()
	}
	if !m.ViewThroughLookbackWindowDays.IsNull() && !m.ViewThroughLookbackWindowDays.IsUnknown() {
		payload["viewThroughLookbackWindowDays"] = m.ViewThroughLookbackWindowDays.ValueInt64()
	}
	return payload
}

func adsConversionActionUpdateFields(m adsConversionActionModel) []string {
	fields := presentFields([]fieldValue{
		{"name", m.Name},
		{"category", m.Category},
		{"status", m.Status},
		{"countingType", m.CountingType},
	})
	if !m.ClickThroughLookbackWindowDays.IsNull() && !m.ClickThroughLookbackWindowDays.IsUnknown() {
		fields = append(fields, "clickThroughLookbackWindowDays")
	}
	if !m.ViewThroughLookbackWindowDays.IsNull() && !m.ViewThroughLookbackWindowDays.IsUnknown() {
		fields = append(fields, "viewThroughLookbackWindowDays")
	}
	return fields
}

func applyAdsConversionActionRemote(m *adsConversionActionModel, remote map[string]any) {
	applyStringRemote(&m.ResourceName, remote, "resourceName")
	if m.ResourceName.ValueString() != "" {
		m.ID = m.ResourceName
	}
	applyStringRemote(&m.Name, remote, "name")
	applyStringRemote(&m.Type, remote, "type")
	applyStringRemote(&m.Category, remote, "category")
	applyStringRemote(&m.Status, remote, "status")
	applyStringRemote(&m.CountingType, remote, "countingType")
	applyInt64Remote(&m.ClickThroughLookbackWindowDays, remote, "clickThroughLookbackWindowDays")
	applyInt64Remote(&m.ViewThroughLookbackWindowDays, remote, "viewThroughLookbackWindowDays")
	tagSnippets := adsTagSnippetsFromRemote(remote["tagSnippets"])
	m.TagSnippets = adsTagSnippetListValue(tagSnippets)
	applyAdsTrackingOutputs(m, tagSnippets)
}

func applyInt64Remote(target *types.Int64, out map[string]any, key string) {
	switch value := out[key].(type) {
	case float64:
		*target = types.Int64Value(int64(value))
	case int:
		*target = types.Int64Value(int64(value))
	case int64:
		*target = types.Int64Value(value)
	case string:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err == nil {
			*target = types.Int64Value(parsed)
			return
		}
		*target = types.Int64Null()
	default:
		*target = types.Int64Null()
	}
}

func adsTagSnippetsFromRemote(raw any) []adsTagSnippetModel {
	items, _ := raw.([]any)
	out := make([]adsTagSnippetModel, 0, len(items))
	for _, item := range items {
		snippet, _ := item.(map[string]any)
		if snippet == nil {
			continue
		}
		out = append(out, adsTagSnippetModel{
			Type:          stringValueOrNull(snippet, "type"),
			PageFormat:    stringValueOrNull(snippet, "pageFormat"),
			GlobalSiteTag: stringValueOrNull(snippet, "globalSiteTag"),
			EventSnippet:  stringValueOrNull(snippet, "eventSnippet"),
		})
	}
	return out
}

func adsTagSnippetListValue(snippets []adsTagSnippetModel) types.List {
	elements := make([]attr.Value, 0, len(snippets))
	for _, snippet := range snippets {
		elements = append(elements, types.ObjectValueMust(adsTagSnippetAttrTypes, map[string]attr.Value{
			"type":            snippet.Type,
			"page_format":     snippet.PageFormat,
			"global_site_tag": snippet.GlobalSiteTag,
			"event_snippet":   snippet.EventSnippet,
		}))
	}
	return types.ListValueMust(adsTagSnippetObjectType, elements)
}

func stringValueOrNull(out map[string]any, key string) types.String {
	value, _ := out[key].(string)
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}

func applyAdsTrackingOutputs(m *adsConversionActionModel, snippets []adsTagSnippetModel) {
	sendTo := adsSendToFromSnippets(snippets)
	if sendTo == "" {
		m.SendTo = types.StringNull()
		m.ConversionID = types.StringNull()
		m.ConversionLabel = types.StringNull()
		return
	}
	m.SendTo = types.StringValue(sendTo)
	parts := strings.SplitN(sendTo, "/", 2)
	m.ConversionID = types.StringValue(parts[0])
	if len(parts) == 2 && parts[1] != "" {
		m.ConversionLabel = types.StringValue(parts[1])
	} else {
		m.ConversionLabel = types.StringNull()
	}
}

func adsSendToFromSnippets(snippets []adsTagSnippetModel) string {
	for _, prefer := range []func(adsTagSnippetModel) bool{
		func(s adsTagSnippetModel) bool {
			return s.Type.ValueString() == "WEBPAGE" && s.PageFormat.ValueString() == "HTML"
		},
		func(s adsTagSnippetModel) bool { return s.Type.ValueString() == "WEBPAGE" },
		func(s adsTagSnippetModel) bool { return s.Type.ValueString() == "WEBPAGE_ONCLICK" },
	} {
		for _, snippet := range snippets {
			if prefer(snippet) {
				if sendTo := adsSendToFromEventSnippet(snippet.EventSnippet.ValueString()); sendTo != "" {
					return sendTo
				}
			}
		}
	}
	return ""
}

var adsSendToRE = regexp.MustCompile(`(?i)['"]?send_to['"]?\s*:\s*['"](AW-[^/'"\s]+/[^'"\s,}]+)['"]`)

func adsSendToFromEventSnippet(snippet string) string {
	matches := adsSendToRE.FindStringSubmatch(snippet)
	if len(matches) != 2 {
		return ""
	}
	return matches[1]
}

type adsDiagnostic interface {
	AddWarning(summary string, detail string)
}

func addAdsSnippetWarning(diags adsDiagnostic, m adsConversionActionModel) {
	if !m.SendTo.IsNull() && !m.SendTo.IsUnknown() {
		return
	}
	diags.AddWarning(
		"Google Ads conversion action has no usable web tag snippet",
		"Google Ads did not return a WEBPAGE/HTML event snippet with a send_to value for this conversion action. The tag_snippets attribute contains the raw typed snippets returned by Google Ads, but send_to, conversion_id, and conversion_label are null.",
	)
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
