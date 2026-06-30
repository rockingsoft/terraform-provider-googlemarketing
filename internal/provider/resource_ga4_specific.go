package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewGA4PropertyResource() resource.Resource {
	return &ga4TypedResource{typeSuffix: "_ga4_property", kind: "property"}
}

func NewGA4WebDataStreamResource() resource.Resource {
	return &ga4TypedResource{typeSuffix: "_ga4_web_data_stream", kind: "web_data_stream"}
}

func NewGA4KeyEventResource() resource.Resource {
	return &ga4TypedResource{typeSuffix: "_ga4_key_event", kind: "key_event"}
}

func NewGA4DataRetentionSettingsResource() resource.Resource {
	return &ga4TypedResource{typeSuffix: "_ga4_data_retention_settings", kind: "data_retention_settings", singleton: true}
}

func NewGA4CustomDimensionResource() resource.Resource {
	return &ga4TypedResource{typeSuffix: "_ga4_custom_dimension", kind: "custom_dimension"}
}

func NewGA4CustomMetricResource() resource.Resource {
	return &ga4TypedResource{typeSuffix: "_ga4_custom_metric", kind: "custom_metric"}
}

var _ resource.Resource = (*ga4TypedResource)(nil)
var _ resource.ResourceWithConfigure = (*ga4TypedResource)(nil)
var _ resource.ResourceWithImportState = (*ga4TypedResource)(nil)

type ga4TypedResource struct {
	client     *marketingClient
	typeSuffix string
	kind       string
	singleton  bool
}

type ga4TypedModel struct {
	ID                         types.String `tfsdk:"id"`
	PropertyID                 types.String `tfsdk:"property_id"`
	ParentID                   types.String `tfsdk:"parent_id"`
	Name                       types.String `tfsdk:"name"`
	DisplayName                types.String `tfsdk:"display_name"`
	TimeZone                   types.String `tfsdk:"time_zone"`
	CurrencyCode               types.String `tfsdk:"currency_code"`
	DefaultURI                 types.String `tfsdk:"default_uri"`
	MeasurementID              types.String `tfsdk:"measurement_id"`
	EventName                  types.String `tfsdk:"event_name"`
	ParameterName              types.String `tfsdk:"parameter_name"`
	Scope                      types.String `tfsdk:"scope"`
	Description                types.String `tfsdk:"description"`
	MeasurementUnit            types.String `tfsdk:"measurement_unit"`
	RestrictedMetricType       types.String `tfsdk:"restricted_metric_type"`
	EventDataRetention         types.String `tfsdk:"event_data_retention"`
	ResetUserDataOnNewActivity types.Bool   `tfsdk:"reset_user_data_on_new_activity"`
}

func (r *ga4TypedResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + r.typeSuffix
}

func (r *ga4TypedResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"id":                              schema.StringAttribute{Computed: true},
		"property_id":                     schema.StringAttribute{Computed: true, Description: "Numeric GA4 property ID. Only set for googlemarketing_ga4_property."},
		"name":                            schema.StringAttribute{Computed: true, Description: "GA4 Admin resource name returned by Google."},
		"parent_id":                       ga4ParentAttribute(r.kind),
		"display_name":                    schema.StringAttribute{Optional: true},
		"time_zone":                       schema.StringAttribute{Optional: true},
		"currency_code":                   schema.StringAttribute{Optional: true},
		"default_uri":                     schema.StringAttribute{Optional: true},
		"measurement_id":                  schema.StringAttribute{Computed: true, Description: "GA4 web stream measurement ID."},
		"event_name":                      schema.StringAttribute{Optional: true},
		"parameter_name":                  schema.StringAttribute{Optional: true},
		"scope":                           schema.StringAttribute{Optional: true},
		"description":                     schema.StringAttribute{Optional: true},
		"measurement_unit":                schema.StringAttribute{Optional: true},
		"restricted_metric_type":          schema.StringAttribute{Optional: true},
		"event_data_retention":            schema.StringAttribute{Optional: true},
		"reset_user_data_on_new_activity": schema.BoolAttribute{Optional: true},
	}
	switch r.kind {
	case "property":
		attrs["display_name"] = schema.StringAttribute{Required: true}
	case "web_data_stream":
		attrs["display_name"] = schema.StringAttribute{Required: true}
	case "key_event":
		attrs["event_name"] = schema.StringAttribute{Required: true}
	case "custom_dimension":
		attrs["parameter_name"] = schema.StringAttribute{Required: true}
		attrs["display_name"] = schema.StringAttribute{Required: true}
		attrs["scope"] = schema.StringAttribute{Required: true}
	case "custom_metric":
		attrs["parameter_name"] = schema.StringAttribute{Required: true}
		attrs["display_name"] = schema.StringAttribute{Required: true}
		attrs["measurement_unit"] = schema.StringAttribute{Required: true}
	case "data_retention_settings":
		attrs["event_data_retention"] = schema.StringAttribute{Required: true}
	}
	resp.Schema = schema.Schema{
		Description: "Typed GA4 Admin API resource.",
		Attributes:  attrs,
	}
}

func ga4ParentAttribute(kind string) schema.StringAttribute {
	desc := "GA4 property ID."
	if kind == "property" {
		desc = "GA4 account ID."
	}
	return schema.StringAttribute{
		Required:    true,
		Description: desc,
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.RequiresReplace(),
		},
	}
}

func (r *ga4TypedResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ga4TypedResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ga4TypedModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload := buildGA4Payload(r.kind, plan)
	apiURL := gaURL(r.createPath(plan))
	method := http.MethodPost
	if r.singleton {
		method = http.MethodPatch
		apiURL = withUpdateMaskFields(apiURL, ga4UpdateFields(r.kind, plan))
	}
	var out map[string]any
	if err := r.client.doJSON(ctx, method, apiURL, payload, &out, nil); err != nil {
		resp.Diagnostics.AddError("Unable to create GA4 resource", err.Error())
		return
	}
	applyGA4Remote(&plan, r.kind, out)
	if plan.Name.ValueString() == "" {
		plan.Name = types.StringValue(r.readPath(plan))
		plan.ID = plan.Name
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ga4TypedResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ga4TypedModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	readPath := state.Name.ValueString()
	if readPath == "" || r.singleton {
		readPath = r.readPath(state)
	}
	var out map[string]any
	err := r.client.doJSON(ctx, http.MethodGet, gaURL(readPath), nil, &out, nil)
	if errors.Is(err, errNotFound) {
		resp.State.RemoveResource(ctx)
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Unable to read GA4 resource", err.Error())
		return
	}
	applyGA4Remote(&state, r.kind, out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ga4TypedResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ga4TypedModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	updatePath := plan.Name.ValueString()
	if updatePath == "" || r.singleton {
		updatePath = r.readPath(plan)
	}
	apiURL := withUpdateMaskFields(gaURL(updatePath), ga4UpdateFields(r.kind, plan))
	if err := r.client.doJSON(ctx, http.MethodPatch, apiURL, buildGA4Payload(r.kind, plan), nil, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update GA4 resource", err.Error())
		return
	}
	var out map[string]any
	if err := r.client.doJSON(ctx, http.MethodGet, gaURL(updatePath), nil, &out, nil); err != nil {
		resp.Diagnostics.AddError("Unable to read updated GA4 resource", err.Error())
		return
	}
	applyGA4Remote(&plan, r.kind, out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ga4TypedResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.singleton {
		return
	}
	var state ga4TypedModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.doJSON(ctx, http.MethodDelete, gaURL(state.Name.ValueString()), nil, nil, nil)
	if err != nil && !errors.Is(err, errNotFound) {
		resp.Diagnostics.AddError("Unable to delete GA4 resource", err.Error())
	}
}

func (r *ga4TypedResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	name := strings.Trim(req.ID, "/")
	var out map[string]any
	if err := r.client.doJSON(ctx, http.MethodGet, gaURL(name), nil, &out, nil); err != nil {
		resp.Diagnostics.AddError("Unable to read imported GA4 resource", err.Error())
		return
	}
	parentID, err := ga4ParentIDFromImport(r.typeSuffix, name, out)
	if err != nil {
		resp.Diagnostics.AddError("Invalid GA4 import ID", err.Error())
		return
	}
	state := ga4TypedModel{ID: types.StringValue(name), ParentID: types.StringValue(parentID), Name: types.StringValue(name)}
	applyGA4Remote(&state, r.kind, out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ga4TypedResource) createPath(m ga4TypedModel) string {
	switch r.kind {
	case "property":
		return "properties"
	case "web_data_stream":
		return fmt.Sprintf("properties/%s/dataStreams", m.ParentID.ValueString())
	case "key_event":
		return fmt.Sprintf("properties/%s/keyEvents", m.ParentID.ValueString())
	case "custom_dimension":
		return fmt.Sprintf("properties/%s/customDimensions", m.ParentID.ValueString())
	case "custom_metric":
		return fmt.Sprintf("properties/%s/customMetrics", m.ParentID.ValueString())
	case "data_retention_settings":
		return r.readPath(m)
	default:
		return ""
	}
}

func (r *ga4TypedResource) readPath(m ga4TypedModel) string {
	if r.kind == "data_retention_settings" {
		return fmt.Sprintf("properties/%s/dataRetentionSettings", m.ParentID.ValueString())
	}
	return m.Name.ValueString()
}

func buildGA4Payload(kind string, m ga4TypedModel) map[string]any {
	payload := map[string]any{}
	switch kind {
	case "property":
		payload["parent"] = "accounts/" + m.ParentID.ValueString()
		putString(payload, "displayName", m.DisplayName)
		putString(payload, "timeZone", m.TimeZone)
		putString(payload, "currencyCode", m.CurrencyCode)
	case "web_data_stream":
		payload["type"] = "WEB_DATA_STREAM"
		putString(payload, "displayName", m.DisplayName)
		if !m.DefaultURI.IsNull() && !m.DefaultURI.IsUnknown() && m.DefaultURI.ValueString() != "" {
			payload["webStreamData"] = map[string]any{"defaultUri": m.DefaultURI.ValueString()}
		}
	case "key_event":
		putString(payload, "eventName", m.EventName)
	case "custom_dimension":
		putString(payload, "parameterName", m.ParameterName)
		putString(payload, "displayName", m.DisplayName)
		putString(payload, "scope", m.Scope)
		putString(payload, "description", m.Description)
	case "custom_metric":
		putString(payload, "parameterName", m.ParameterName)
		putString(payload, "displayName", m.DisplayName)
		putString(payload, "measurementUnit", m.MeasurementUnit)
		putString(payload, "scope", m.Scope)
		putString(payload, "description", m.Description)
		putString(payload, "restrictedMetricType", m.RestrictedMetricType)
	case "data_retention_settings":
		putString(payload, "eventDataRetention", m.EventDataRetention)
		if !m.ResetUserDataOnNewActivity.IsNull() && !m.ResetUserDataOnNewActivity.IsUnknown() {
			payload["resetUserDataOnNewActivity"] = m.ResetUserDataOnNewActivity.ValueBool()
		}
	}
	return payload
}

func ga4UpdateFields(kind string, m ga4TypedModel) []string {
	switch kind {
	case "property":
		return presentFields([]fieldValue{{"displayName", m.DisplayName}, {"timeZone", m.TimeZone}, {"currencyCode", m.CurrencyCode}})
	case "web_data_stream":
		fields := presentFields([]fieldValue{{"displayName", m.DisplayName}})
		if !m.DefaultURI.IsNull() && !m.DefaultURI.IsUnknown() {
			fields = append(fields, "webStreamData.defaultUri")
		}
		return fields
	case "key_event":
		return []string{"eventName"}
	case "custom_dimension":
		return presentFields([]fieldValue{{"parameterName", m.ParameterName}, {"displayName", m.DisplayName}, {"scope", m.Scope}, {"description", m.Description}})
	case "custom_metric":
		return presentFields([]fieldValue{{"parameterName", m.ParameterName}, {"displayName", m.DisplayName}, {"measurementUnit", m.MeasurementUnit}, {"scope", m.Scope}, {"description", m.Description}, {"restrictedMetricType", m.RestrictedMetricType}})
	case "data_retention_settings":
		fields := presentFields([]fieldValue{{"eventDataRetention", m.EventDataRetention}})
		if !m.ResetUserDataOnNewActivity.IsNull() && !m.ResetUserDataOnNewActivity.IsUnknown() {
			fields = append(fields, "resetUserDataOnNewActivity")
		}
		return fields
	default:
		return nil
	}
}

type fieldValue struct {
	name  string
	value types.String
}

func presentFields(fields []fieldValue) []string {
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if !field.value.IsNull() && !field.value.IsUnknown() {
			out = append(out, field.name)
		}
	}
	return out
}

func putString(payload map[string]any, key string, value types.String) {
	if value.IsNull() || value.IsUnknown() || value.ValueString() == "" {
		return
	}
	payload[key] = value.ValueString()
}

func applyGA4Remote(m *ga4TypedModel, kind string, out map[string]any) {
	if name, _ := out["name"].(string); name != "" {
		m.ID = types.StringValue(name)
		m.Name = types.StringValue(name)
	}
	if kind == "property" {
		m.PropertyID = types.StringValue(ga4PropertyIDFromName(m.Name.ValueString()))
	} else if m.PropertyID.IsUnknown() {
		m.PropertyID = types.StringNull()
	}
	switch kind {
	case "property":
		applyStringRemote(&m.DisplayName, out, "displayName")
		applyStringRemote(&m.TimeZone, out, "timeZone")
		applyStringRemote(&m.CurrencyCode, out, "currencyCode")
	case "web_data_stream":
		applyStringRemote(&m.DisplayName, out, "displayName")
		if web, _ := out["webStreamData"].(map[string]any); web != nil {
			if measurementID, _ := web["measurementId"].(string); measurementID != "" {
				m.MeasurementID = types.StringValue(measurementID)
			}
			applyStringRemote(&m.DefaultURI, web, "defaultUri")
		}
		if m.MeasurementID.IsUnknown() {
			m.MeasurementID = types.StringNull()
		}
	case "key_event":
		applyStringRemote(&m.EventName, out, "eventName")
	case "custom_dimension":
		applyStringRemote(&m.ParameterName, out, "parameterName")
		applyStringRemote(&m.DisplayName, out, "displayName")
		applyStringRemote(&m.Scope, out, "scope")
		applyStringRemote(&m.Description, out, "description")
	case "custom_metric":
		applyStringRemote(&m.ParameterName, out, "parameterName")
		applyStringRemote(&m.DisplayName, out, "displayName")
		applyStringRemote(&m.Scope, out, "scope")
		applyStringRemote(&m.Description, out, "description")
		applyStringRemote(&m.MeasurementUnit, out, "measurementUnit")
		applyStringRemote(&m.RestrictedMetricType, out, "restrictedMetricType")
	case "data_retention_settings":
		applyStringRemote(&m.EventDataRetention, out, "eventDataRetention")
		if value, ok := out["resetUserDataOnNewActivity"].(bool); ok {
			m.ResetUserDataOnNewActivity = types.BoolValue(value)
		} else {
			m.ResetUserDataOnNewActivity = types.BoolNull()
		}
	}
	if m.MeasurementID.IsUnknown() {
		m.MeasurementID = types.StringNull()
	}
}

func applyStringRemote(target *types.String, out map[string]any, key string) {
	if value := stringFromMap(out, key); value != "" {
		*target = types.StringValue(value)
		return
	}
	*target = types.StringNull()
}

func ga4PropertyIDFromName(name string) string {
	return strings.TrimPrefix(strings.Trim(name, "/"), "properties/")
}

func withUpdateMaskFields(apiURL string, fields []string) string {
	if len(fields) == 0 {
		return apiURL
	}
	sep := "?"
	if strings.Contains(apiURL, "?") {
		sep = "&"
	}
	return apiURL + sep + "updateMask=" + url.QueryEscape(strings.Join(fields, ","))
}

func withUpdateMask(apiURL, payloadJSON string) string {
	mask := updateMaskFromPayload(payloadJSON)
	if mask == "" {
		return apiURL
	}
	return withUpdateMaskFields(apiURL, strings.Split(mask, ","))
}

func ga4ParentIDFromImport(typeSuffix, name string, remote map[string]any) (string, error) {
	parts := strings.Split(strings.Trim(name, "/"), "/")
	if typeSuffix == "_ga4_property" {
		if len(parts) == 2 && parts[0] == "properties" {
			parent, _ := remote["parent"].(string)
			return strings.TrimPrefix(parent, "accounts/"), nil
		}
		return "", fmt.Errorf("expected properties/{property_id}")
	}
	if len(parts) < 2 || parts[0] != "properties" {
		return "", fmt.Errorf("expected properties/{property_id}/...")
	}
	return parts[1], nil
}
