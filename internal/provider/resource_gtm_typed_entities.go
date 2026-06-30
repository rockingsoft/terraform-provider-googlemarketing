package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*gtmTypedWorkspaceEntityResource)(nil)
var _ resource.ResourceWithConfigure = (*gtmTypedWorkspaceEntityResource)(nil)
var _ resource.ResourceWithImportState = (*gtmTypedWorkspaceEntityResource)(nil)

func NewGTMVariableResource() resource.Resource {
	return &gtmTypedWorkspaceEntityResource{kind: "variable", typeSuffix: "_gtm_variable"}
}

func NewGTMTriggerResource() resource.Resource {
	return &gtmTypedWorkspaceEntityResource{kind: "trigger", typeSuffix: "_gtm_trigger"}
}

func NewGTMTagResource() resource.Resource {
	return &gtmTypedWorkspaceEntityResource{kind: "tag", typeSuffix: "_gtm_tag"}
}

func NewGTMFolderResource() resource.Resource {
	return &gtmTypedWorkspaceEntityResource{kind: "folder", typeSuffix: "_gtm_folder"}
}

type gtmTypedWorkspaceEntityResource struct {
	client     *marketingClient
	kind       string
	typeSuffix string
}

type gtmTypedWorkspaceEntityModel struct {
	ID                 types.String `tfsdk:"id"`
	AccountID          types.String `tfsdk:"account_id"`
	ContainerID        types.String `tfsdk:"container_id"`
	WorkspaceID        types.String `tfsdk:"workspace_id"`
	Name               types.String `tfsdk:"name"`
	Type               types.String `tfsdk:"type"`
	EntityID           types.String `tfsdk:"entity_id"`
	Path               types.String `tfsdk:"path"`
	Notes              types.String `tfsdk:"notes"`
	MeasurementID      types.String `tfsdk:"measurement_id"`
	EventName          types.String `tfsdk:"event_name"`
	HTML               types.String `tfsdk:"html"`
	ConversionID       types.String `tfsdk:"conversion_id"`
	ConversionLabel    types.String `tfsdk:"conversion_label"`
	CustomEventName    types.String `tfsdk:"custom_event_name"`
	FilterVariable     types.String `tfsdk:"filter_variable"`
	FilterOperator     types.String `tfsdk:"filter_operator"`
	FilterValue        types.String `tfsdk:"filter_value"`
	Value              types.String `tfsdk:"value"`
	DataLayerName      types.String `tfsdk:"data_layer_name"`
	CookieName         types.String `tfsdk:"cookie_name"`
	JavaScript         types.String `tfsdk:"javascript"`
	FiringTriggerIDs   types.List   `tfsdk:"firing_trigger_ids"`
	BlockingTriggerIDs types.List   `tfsdk:"blocking_trigger_ids"`
}

func (r *gtmTypedWorkspaceEntityResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + r.typeSuffix
}

func (r *gtmTypedWorkspaceEntityResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"id":           schema.StringAttribute{Computed: true},
		"entity_id":    schema.StringAttribute{Computed: true, Description: "Short GTM entity ID returned by Google."},
		"path":         schema.StringAttribute{Computed: true, Description: "GTM API resource path returned by Google."},
		"account_id":   replaceStringAttribute(),
		"container_id": replaceStringAttribute(),
		"workspace_id": replaceStringAttribute(),
		"name":         schema.StringAttribute{Required: true},
		"type":         schema.StringAttribute{Optional: true, Description: "GTM entity type where applicable."},
		"notes":        schema.StringAttribute{Optional: true},
		"measurement_id": schema.StringAttribute{
			Optional:    true,
			Description: "GA4 measurement ID used by GA4 tag types.",
		},
		"event_name":         schema.StringAttribute{Optional: true, Description: "GA4 event name."},
		"html":               schema.StringAttribute{Optional: true, Sensitive: true, Description: "Custom HTML body for html tags."},
		"conversion_id":      schema.StringAttribute{Optional: true, Description: "Google Ads conversion ID."},
		"conversion_label":   schema.StringAttribute{Optional: true, Sensitive: true, Description: "Google Ads conversion label."},
		"custom_event_name":  schema.StringAttribute{Optional: true, Description: "Event name for CUSTOM_EVENT triggers."},
		"filter_variable":    schema.StringAttribute{Optional: true, Description: "Variable used by the optional trigger filter."},
		"filter_operator":    schema.StringAttribute{Optional: true, Description: "Filter operator, for example EQUALS, CONTAINS, or MATCH_REGEX."},
		"filter_value":       schema.StringAttribute{Optional: true, Description: "Filter comparison value."},
		"value":              schema.StringAttribute{Optional: true, Sensitive: true, Description: "Constant or lookup value."},
		"data_layer_name":    schema.StringAttribute{Optional: true, Description: "Data layer variable name."},
		"cookie_name":        schema.StringAttribute{Optional: true, Sensitive: true, Description: "First-party cookie name."},
		"javascript":         schema.StringAttribute{Optional: true, Sensitive: true, Description: "Custom JavaScript body."},
		"firing_trigger_ids": schema.ListAttribute{Optional: true, ElementType: types.StringType},
		"blocking_trigger_ids": schema.ListAttribute{
			Optional:    true,
			ElementType: types.StringType,
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
		Description: fmt.Sprintf("Typed Google Tag Manager workspace %s resource.", r.kind),
		Attributes:  attrs,
	}
}

func (r *gtmTypedWorkspaceEntityResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *gtmTypedWorkspaceEntityResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan gtmTypedWorkspaceEntityModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload := buildGTMPayload(ctx, r.kind, plan)
	collection, _ := gtmEntityCollection(r.kind)
	apiPath := fmt.Sprintf("accounts/%s/containers/%s/workspaces/%s/%s", plan.AccountID.ValueString(), plan.ContainerID.ValueString(), plan.WorkspaceID.ValueString(), collection)
	var out map[string]any
	if err := r.client.doJSON(ctx, http.MethodPost, gtmURL(apiPath), payload, &out, nil); err != nil {
		resp.Diagnostics.AddError("Unable to create GTM workspace entity", err.Error())
		return
	}
	applyGTMRemoteIDs(&plan, r.kind, out)
	if plan.Path.ValueString() == "" {
		resp.Diagnostics.AddError("GTM response missing path", "Google did not return a resource path for the created entity.")
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *gtmTypedWorkspaceEntityResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state gtmTypedWorkspaceEntityModel
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
	applyGTMRemoteTypedFields(&state, r.kind, out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *gtmTypedWorkspaceEntityResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan gtmTypedWorkspaceEntityModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload := buildGTMPayload(ctx, r.kind, plan)
	if err := r.client.doJSON(ctx, http.MethodPut, gtmURL(plan.Path.ValueString()), payload, nil, nil); err != nil {
		resp.Diagnostics.AddError("Unable to update GTM workspace entity", err.Error())
		return
	}
	var out map[string]any
	if err := r.client.doJSON(ctx, http.MethodGet, gtmURL(plan.Path.ValueString()), nil, &out, nil); err != nil {
		resp.Diagnostics.AddError("Unable to read updated GTM workspace entity", err.Error())
		return
	}
	applyGTMRemoteTypedFields(&plan, r.kind, out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *gtmTypedWorkspaceEntityResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state gtmTypedWorkspaceEntityModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.client.doJSON(ctx, http.MethodDelete, gtmURL(state.Path.ValueString()), nil, nil, nil)
	if err != nil && !errors.Is(err, errNotFound) {
		resp.Diagnostics.AddError("Unable to delete GTM workspace entity", err.Error())
	}
}

func (r *gtmTypedWorkspaceEntityResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parsed, err := parseGTMWorkspaceEntityPath(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid GTM workspace entity import ID", err.Error())
		return
	}
	if parsed.Kind != r.kind {
		resp.Diagnostics.AddError("Invalid GTM workspace entity import ID", fmt.Sprintf("Expected %s path, got %s.", r.kind, parsed.Kind))
		return
	}
	var out map[string]any
	if err := r.client.doJSON(ctx, http.MethodGet, gtmURL(parsed.Path), nil, &out, nil); err != nil {
		resp.Diagnostics.AddError("Unable to read imported GTM workspace entity", err.Error())
		return
	}
	state := gtmTypedWorkspaceEntityModel{
		ID:          types.StringValue(parsed.Path),
		AccountID:   types.StringValue(parsed.AccountID),
		ContainerID: types.StringValue(parsed.ContainerID),
		WorkspaceID: types.StringValue(parsed.WorkspaceID),
		Name:        types.StringValue(stringFromMap(out, "name")),
		Type:        types.StringValue(stringFromMap(out, "type")),
		Path:        types.StringValue(parsed.Path),
	}
	applyGTMRemoteTypedFields(&state, r.kind, out)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func buildGTMPayload(ctx context.Context, kind string, m gtmTypedWorkspaceEntityModel) map[string]any {
	payload := map[string]any{"name": m.Name.ValueString()}
	if !m.Type.IsNull() && !m.Type.IsUnknown() && m.Type.ValueString() != "" {
		payload["type"] = m.Type.ValueString()
	}
	if !m.Notes.IsNull() && !m.Notes.IsUnknown() && m.Notes.ValueString() != "" {
		payload["notes"] = m.Notes.ValueString()
	}
	switch kind {
	case "tag":
		if ids := stringList(ctx, m.FiringTriggerIDs); len(ids) > 0 {
			payload["firingTriggerId"] = ids
		}
		if ids := stringList(ctx, m.BlockingTriggerIDs); len(ids) > 0 {
			payload["blockingTriggerId"] = ids
		}
		params := []any{}
		params = appendGTMParam(params, gtmMeasurementIDParamKey(m.Type.ValueString()), m.MeasurementID)
		params = appendGTMParam(params, "eventName", m.EventName)
		params = appendGTMParam(params, "html", m.HTML)
		params = appendGTMParam(params, "conversionId", m.ConversionID)
		params = appendGTMParam(params, "conversionLabel", m.ConversionLabel)
		if len(params) > 0 {
			payload["parameter"] = params
		}
	case "trigger":
		if !m.CustomEventName.IsNull() && !m.CustomEventName.IsUnknown() && m.CustomEventName.ValueString() != "" {
			payload["customEventFilter"] = []any{gtmCondition("EQUALS", "{{_event}}", m.CustomEventName.ValueString())}
		}
		if !m.FilterVariable.IsNull() && !m.FilterVariable.IsUnknown() && m.FilterVariable.ValueString() != "" {
			op := "EQUALS"
			if !m.FilterOperator.IsNull() && !m.FilterOperator.IsUnknown() && m.FilterOperator.ValueString() != "" {
				op = m.FilterOperator.ValueString()
			}
			payload["filter"] = []any{gtmCondition(op, m.FilterVariable.ValueString(), m.FilterValue.ValueString())}
		}
	case "variable":
		params := []any{}
		params = appendGTMParam(params, "value", m.Value)
		params = appendGTMParam(params, "name", m.DataLayerName)
		params = appendGTMParam(params, "cookieName", m.CookieName)
		params = appendGTMParam(params, "javascript", m.JavaScript)
		if len(params) > 0 {
			payload["parameter"] = params
		}
	}
	return payload
}

func gtmMeasurementIDParamKey(tagType string) string {
	if tagType == "gaawe" {
		return "measurementIdOverride"
	}
	return "measurementId"
}

func appendGTMParam(params []any, key string, value types.String) []any {
	if value.IsNull() || value.IsUnknown() || value.ValueString() == "" {
		return params
	}
	return append(params, map[string]any{
		"type":  "template",
		"key":   key,
		"value": value.ValueString(),
	})
}

func gtmCondition(operator, left, right string) map[string]any {
	return map[string]any{
		"type": operator,
		"parameter": []any{
			map[string]any{"type": "template", "key": "arg0", "value": left},
			map[string]any{"type": "template", "key": "arg1", "value": right},
		},
	}
}

func applyGTMRemoteIDs(m *gtmTypedWorkspaceEntityModel, kind string, out map[string]any) {
	if pathValue, _ := out["path"].(string); pathValue != "" {
		m.ID = types.StringValue(pathValue)
		m.Path = types.StringValue(pathValue)
	}
	idKey := kind + "Id"
	if kind == "folder" {
		idKey = "folderId"
	}
	if id, _ := out[idKey].(string); id != "" {
		m.EntityID = types.StringValue(id)
		return
	}
	if pathValue := m.Path.ValueString(); pathValue != "" {
		parts := strings.Split(strings.Trim(pathValue, "/"), "/")
		m.EntityID = types.StringValue(parts[len(parts)-1])
	}
}

func applyGTMRemoteTypedFields(m *gtmTypedWorkspaceEntityModel, kind string, out map[string]any) {
	applyGTMRemoteIDs(m, kind, out)
	if name := stringFromMap(out, "name"); name != "" {
		m.Name = types.StringValue(name)
	}
	if typeName := stringFromMap(out, "type"); typeName != "" {
		m.Type = types.StringValue(typeName)
	}
	if notes := stringFromMap(out, "notes"); notes != "" {
		m.Notes = types.StringValue(notes)
	} else {
		m.Notes = types.StringNull()
	}
	switch kind {
	case "tag":
		m.FiringTriggerIDs = stringListValue(stringsFromAny(out["firingTriggerId"]))
		m.BlockingTriggerIDs = stringListValue(stringsFromAny(out["blockingTriggerId"]))
		params := gtmParamMap(out["parameter"])
		if value := firstNonEmpty(params["measurementIdOverride"], params["measurementId"]); value != "" {
			m.MeasurementID = types.StringValue(value)
		} else {
			m.MeasurementID = types.StringNull()
		}
		if value := params["eventName"]; value != "" {
			m.EventName = types.StringValue(value)
		} else {
			m.EventName = types.StringNull()
		}
		if value := params["html"]; value != "" {
			m.HTML = types.StringValue(value)
		} else {
			m.HTML = types.StringNull()
		}
		if value := params["conversionId"]; value != "" {
			m.ConversionID = types.StringValue(value)
		} else {
			m.ConversionID = types.StringNull()
		}
		if value := params["conversionLabel"]; value != "" {
			m.ConversionLabel = types.StringValue(value)
		} else {
			m.ConversionLabel = types.StringNull()
		}
	case "trigger":
		if value := customEventNameFromFilters(out["customEventFilter"]); value != "" {
			m.CustomEventName = types.StringValue(value)
		} else {
			m.CustomEventName = types.StringNull()
		}
		applyGTMTriggerFilterRemote(m, out["filter"])
	case "variable":
		params := gtmParamMap(out["parameter"])
		if value := params["name"]; value != "" {
			m.DataLayerName = types.StringValue(value)
		} else {
			m.DataLayerName = types.StringNull()
		}
		if value := params["value"]; value != "" {
			m.Value = types.StringValue(value)
		} else {
			m.Value = types.StringNull()
		}
		if value := params["cookieName"]; value != "" {
			m.CookieName = types.StringValue(value)
		} else {
			m.CookieName = types.StringNull()
		}
		if value := params["javascript"]; value != "" {
			m.JavaScript = types.StringValue(value)
		} else {
			m.JavaScript = types.StringNull()
		}
	}
}

func gtmParamMap(raw any) map[string]string {
	out := map[string]string{}
	params, _ := raw.([]any)
	for _, item := range params {
		param, _ := item.(map[string]any)
		key, _ := param["key"].(string)
		value, _ := param["value"].(string)
		if key != "" {
			out[key] = value
		}
	}
	return out
}

func stringsFromAny(raw any) []string {
	items, _ := raw.([]any)
	out := make([]string, 0, len(items))
	for _, item := range items {
		if value, _ := item.(string); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func customEventNameFromFilters(raw any) string {
	filters, _ := raw.([]any)
	if len(filters) == 0 {
		return ""
	}
	filter, _ := filters[0].(map[string]any)
	params := gtmParamMap(filter["parameter"])
	return params["arg1"]
}

func applyGTMTriggerFilterRemote(m *gtmTypedWorkspaceEntityModel, raw any) {
	filters, _ := raw.([]any)
	if len(filters) == 0 {
		m.FilterVariable = types.StringNull()
		m.FilterOperator = types.StringNull()
		m.FilterValue = types.StringNull()
		return
	}
	filter, _ := filters[0].(map[string]any)
	params := gtmParamMap(filter["parameter"])
	if value := params["arg0"]; value != "" {
		m.FilterVariable = types.StringValue(value)
	} else {
		m.FilterVariable = types.StringNull()
	}
	if operator := stringFromMap(filter, "type"); operator != "" {
		m.FilterOperator = types.StringValue(operator)
	} else {
		m.FilterOperator = types.StringNull()
	}
	if value := params["arg1"]; value != "" {
		m.FilterValue = types.StringValue(value)
	} else {
		m.FilterValue = types.StringNull()
	}
}

func stringList(ctx context.Context, value types.List) []string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	var out []string
	diags := value.ElementsAs(ctx, &out, false)
	if diags.HasError() {
		return nil
	}
	return out
}

func stringListValue(values []string) types.List {
	if len(values) == 0 {
		return types.ListNull(types.StringType)
	}
	return stringListValueAllowEmpty(values)
}

func stringListValueAllowEmpty(values []string) types.List {
	elements := make([]attr.Value, 0, len(values))
	for _, value := range values {
		elements = append(elements, types.StringValue(value))
	}
	return types.ListValueMust(types.StringType, elements)
}

func stringFromMap(m map[string]any, key string) string {
	value, _ := m[key].(string)
	return value
}
