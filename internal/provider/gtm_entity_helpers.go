package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type gtmTypedWorkspaceEntityModel struct {
	ID                 types.String `tfsdk:"id"`
	AccountID          types.String `tfsdk:"account_id"`
	ContainerID        types.String `tfsdk:"container_id"`
	WorkspaceName      types.String `tfsdk:"workspace_name"`
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
	AdditionalParams   types.Map    `tfsdk:"additional_params"`
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

func buildGTMPayload(ctx context.Context, kind string, m gtmTypedWorkspaceEntityModel) map[string]any {
	payload := map[string]any{"name": m.Name.ValueString()}
	if !m.Type.IsNull() && !m.Type.IsUnknown() && m.Type.ValueString() != "" {
		payload["type"] = gtmAPIType(kind, m.Type.ValueString())
	}
	if !m.Notes.IsNull() && !m.Notes.IsUnknown() && m.Notes.ValueString() != "" {
		payload["notes"] = m.Notes.ValueString()
	}
	var params []any
	switch kind {
	case "tag":
		if ids := stringList(ctx, m.FiringTriggerIDs); len(ids) > 0 {
			payload["firingTriggerId"] = ids
		}
		if ids := stringList(ctx, m.BlockingTriggerIDs); len(ids) > 0 {
			payload["blockingTriggerId"] = ids
		}
		params = appendGTMParam(params, gtmMeasurementIDParamKey(m.Type.ValueString()), m.MeasurementID)
		params = appendGTMParam(params, "eventName", m.EventName)
		params = appendGTMParam(params, "html", m.HTML)
		params = appendGTMParam(params, "conversionId", m.ConversionID)
		params = appendGTMParam(params, "conversionLabel", m.ConversionLabel)
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
		params = appendGTMParam(params, "value", m.Value)
		params = appendGTMParam(params, "name", m.DataLayerName)
		params = appendGTMParam(params, "cookieName", m.CookieName)
		params = appendGTMParam(params, "javascript", m.JavaScript)
	}
	params = appendGTMAdditionalParams(ctx, params, m.AdditionalParams)
	if len(params) > 0 {
		payload["parameter"] = params
	}
	return payload
}

// validateGTMTagAdditionalRequirements catches GTM template requirements
// that aren't expressible as plain schema validation, mirroring what Google
// itself rejects at write time so the error surfaces with resource context.
func validateGTMTagAdditionalRequirements(m gtmTypedWorkspaceEntityModel) error {
	if m.Type.ValueString() == "gaawe" && (m.MeasurementID.IsNull() || m.MeasurementID.ValueString() == "") {
		return fmt.Errorf("measurement_id is required when type is \"gaawe\" because the GTM gaawe template requires measurementIdOverride")
	}
	return nil
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

func gtmMeasurementIDParamKey(tagType string) string {
	if tagType == "gaawe" {
		return "measurementIdOverride"
	}
	return "measurementId"
}

var gtmTriggerTypeAliases = map[string]string{
	"ALL_ELEMENTS":    "allElements",
	"CLICK":           "click",
	"CUSTOM_EVENT":    "customEvent",
	"DOM_READY":       "domReady",
	"FORM_SUBMISSION": "formSubmission",
	"HISTORY_CHANGE":  "historyChange",
	"INIT":            "init",
	"JS_ERROR":        "jsError",
	"LINK_CLICK":      "linkClick",
	"PAGEVIEW":        "pageview",
	"TIMER":           "timer",
	"WINDOW_LOADED":   "windowLoaded",
}

func gtmAPIType(kind, typeName string) string {
	if kind != "trigger" {
		return typeName
	}
	if value, ok := gtmTriggerTypeAliases[typeName]; ok {
		return value
	}
	return typeName
}

func gtmStateType(kind string, prior types.String, remoteType string) string {
	if remoteType == "" {
		return ""
	}
	if kind == "trigger" && !prior.IsNull() && !prior.IsUnknown() {
		priorType := prior.ValueString()
		if priorType != "" && gtmAPIType(kind, priorType) == remoteType {
			return priorType
		}
	}
	return remoteType
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

// appendGTMAdditionalParams renders the additional_params escape hatch as
// GTM template parameters, sorted by key for a deterministic payload.
func appendGTMAdditionalParams(ctx context.Context, params []any, value types.Map) []any {
	if value.IsNull() || value.IsUnknown() {
		return params
	}
	elements := make(map[string]string)
	if diags := value.ElementsAs(ctx, &elements, false); diags.HasError() {
		return params
	}
	keys := make([]string, 0, len(elements))
	for key := range elements {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		params = append(params, map[string]any{
			"type":  "template",
			"key":   key,
			"value": elements[key],
		})
	}
	return params
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
	// The workspace segment of path (and hence workspace_id) is
	// current-as-of-this-response only: GTM recycles workspace IDs on
	// every publish. id stays anchored to account/container/entity so it
	// survives publishes, while path/workspace_id are refreshed on every
	// Create/Read/Update.
	if pathValue, _ := out["path"].(string); pathValue != "" {
		m.Path = types.StringValue(pathValue)
		if parsed, err := parseGTMWorkspaceEntityPath(pathValue); err == nil {
			m.WorkspaceID = types.StringValue(parsed.WorkspaceID)
		}
	}
	idKey := kind + "Id"
	if kind == "folder" {
		idKey = "folderId"
	}
	if id, _ := out[idKey].(string); id != "" {
		m.EntityID = types.StringValue(id)
	} else if pathValue := m.Path.ValueString(); pathValue != "" {
		parts := strings.Split(strings.Trim(pathValue, "/"), "/")
		m.EntityID = types.StringValue(parts[len(parts)-1])
	}
	if m.EntityID.ValueString() != "" && !m.AccountID.IsNull() && !m.ContainerID.IsNull() {
		m.ID = types.StringValue(gtmContainerEntityID(m.AccountID.ValueString(), m.ContainerID.ValueString(), kindCollection(kind), m.EntityID.ValueString()))
	}
}

func kindCollection(kind string) string {
	collection, err := gtmEntityCollection(kind)
	if err != nil {
		return kind
	}
	return collection
}

func applyGTMRemoteTypedFields(m *gtmTypedWorkspaceEntityModel, kind string, out map[string]any) {
	applyGTMRemoteIDs(m, kind, out)
	if name := stringFromMap(out, "name"); name != "" {
		m.Name = types.StringValue(name)
	}
	if typeName := stringFromMap(out, "type"); typeName != "" {
		m.Type = types.StringValue(gtmStateType(kind, m.Type, typeName))
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

func googleTagConfigsSupported(out map[string]any) (bool, bool) {
	features, _ := out["features"].(map[string]any)
	if features == nil {
		return false, false
	}
	supported, ok := features["supportGtagConfigs"].(bool)
	return supported, ok
}

func existingGoogleTagConfigPath(out map[string]any, tagID string) string {
	for _, key := range []string{"gtagConfig", "gtagConfigs", "gtag_config"} {
		items, _ := out[key].([]any)
		for _, item := range items {
			config, _ := item.(map[string]any)
			if gtmParamMap(config["parameter"])["tagId"] == tagID {
				return stringFromMap(config, "path")
			}
		}
	}
	if gtmParamMap(out["parameter"])["tagId"] == tagID {
		return stringFromMap(out, "path")
	}
	return ""
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
