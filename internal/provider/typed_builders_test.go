package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestBuildGTMTagPayloadUsesShortTriggerIDs(t *testing.T) {
	model := gtmTypedWorkspaceEntityModel{
		Name:             types.StringValue("GA4 purchase"),
		Type:             types.StringValue("gaawe"),
		MeasurementID:    types.StringValue("G-ABC123"),
		EventName:        types.StringValue("purchase"),
		FiringTriggerIDs: stringListValue([]string{"123"}),
	}
	payload := buildGTMPayload(context.Background(), "tag", model)

	if got, want := payload["firingTriggerId"], []string{"123"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("firingTriggerId = %#v, want %#v", got, want)
	}
	params := payload["parameter"].([]any)
	if !hasGTMParam(params, "measurementIdOverride", "G-ABC123") || !hasGTMParam(params, "eventName", "purchase") {
		t.Fatalf("unexpected tag parameters: %#v", params)
	}
}

func TestBuildGTMTagPayloadIncludesNotes(t *testing.T) {
	model := gtmTypedWorkspaceEntityModel{
		Name:  types.StringValue("GA4 purchase"),
		Type:  types.StringValue("gaawe"),
		Notes: types.StringValue("Published by Terraform"),
	}
	payload := buildGTMPayload(context.Background(), "tag", model)

	if got, want := payload["notes"], "Published by Terraform"; got != want {
		t.Fatalf("notes = %#v, want %#v", got, want)
	}
}

func TestBuildGTMTriggerPayloadCustomEvent(t *testing.T) {
	model := gtmTypedWorkspaceEntityModel{
		Name:            types.StringValue("Purchase event"),
		Type:            types.StringValue("CUSTOM_EVENT"),
		CustomEventName: types.StringValue("purchase"),
	}
	payload := buildGTMPayload(context.Background(), "trigger", model)
	filters := payload["customEventFilter"].([]any)
	first := filters[0].(map[string]any)
	if first["type"] != "EQUALS" {
		t.Fatalf("custom event filter type = %#v, want EQUALS", first["type"])
	}
	params := first["parameter"].([]any)
	if !hasGTMParam(params, "arg0", "{{_event}}") || !hasGTMParam(params, "arg1", "purchase") {
		t.Fatalf("unexpected custom event parameters: %#v", params)
	}
}

func TestApplyGTMRemoteIDsExtractsEntityID(t *testing.T) {
	model := gtmTypedWorkspaceEntityModel{}
	applyGTMRemoteIDs(&model, "trigger", map[string]any{
		"path":      "accounts/1/containers/2/workspaces/3/triggers/123",
		"triggerId": "123",
	})
	if model.EntityID.ValueString() != "123" {
		t.Fatalf("entity_id = %q, want 123", model.EntityID.ValueString())
	}
}

func TestApplyGA4RemoteExtractsMeasurementID(t *testing.T) {
	model := ga4TypedModel{}
	applyGA4Remote(&model, "web_data_stream", map[string]any{
		"name": "properties/1/dataStreams/2",
		"webStreamData": map[string]any{
			"measurementId": "G-ABC123",
		},
	})
	if model.MeasurementID.ValueString() != "G-ABC123" {
		t.Fatalf("measurement_id = %q, want G-ABC123", model.MeasurementID.ValueString())
	}
}

func TestApplyGA4RemoteExtractsPropertyID(t *testing.T) {
	model := ga4TypedModel{}
	applyGA4Remote(&model, "property", map[string]any{
		"name": "properties/123",
	})
	if model.PropertyID.ValueString() != "123" {
		t.Fatalf("property_id = %q, want 123", model.PropertyID.ValueString())
	}
}

func TestBuildGTMGoogleTagConfigPayload(t *testing.T) {
	model := gtmGoogleTagConfigModel{
		TagID: types.StringValue("G-ABC123"),
	}
	payload := buildGTMGoogleTagConfigPayload(model)

	if got, want := payload["type"], "google"; got != want {
		t.Fatalf("type = %#v, want %#v", got, want)
	}
	params := payload["parameter"].([]any)
	if !hasGTMParam(params, "tagId", "G-ABC123") {
		t.Fatalf("unexpected Google tag config parameters: %#v", params)
	}
}

func TestApplyGTMRemoteTypedFieldsTagCompletesDriftFields(t *testing.T) {
	model := gtmTypedWorkspaceEntityModel{}
	applyGTMRemoteTypedFields(&model, "tag", map[string]any{
		"path":              "accounts/1/containers/2/workspaces/3/tags/10",
		"tagId":             "10",
		"name":              "Ads conversion",
		"type":              "awct",
		"notes":             "remote notes",
		"firingTriggerId":   []any{"1"},
		"blockingTriggerId": []any{"2"},
		"parameter": []any{
			map[string]any{"key": "measurementId", "value": "G-ABC123"},
			map[string]any{"key": "eventName", "value": "purchase"},
			map[string]any{"key": "html", "value": "<script></script>"},
			map[string]any{"key": "conversionId", "value": "AW-123"},
			map[string]any{"key": "conversionLabel", "value": "label"},
		},
	})

	if model.HTML.ValueString() != "<script></script>" || model.ConversionID.ValueString() != "AW-123" || model.ConversionLabel.ValueString() != "label" {
		t.Fatalf("tag drift fields were not applied: %#v", model)
	}
	if got := stringList(context.Background(), model.FiringTriggerIDs); !reflect.DeepEqual(got, []string{"1"}) {
		t.Fatalf("firing triggers = %#v, want [1]", got)
	}
}

func TestApplyGTMRemoteTypedFieldsTriggerAndVariableCompletesDriftFields(t *testing.T) {
	trigger := gtmTypedWorkspaceEntityModel{}
	applyGTMRemoteTypedFields(&trigger, "trigger", map[string]any{
		"customEventFilter": []any{gtmCondition("EQUALS", "{{_event}}", "purchase")},
		"filter":            []any{gtmCondition("CONTAINS", "{{Page URL}}", "/checkout")},
	})
	if trigger.CustomEventName.ValueString() != "purchase" || trigger.FilterOperator.ValueString() != "CONTAINS" || trigger.FilterVariable.ValueString() != "{{Page URL}}" || trigger.FilterValue.ValueString() != "/checkout" {
		t.Fatalf("trigger drift fields were not applied: %#v", trigger)
	}

	variable := gtmTypedWorkspaceEntityModel{}
	applyGTMRemoteTypedFields(&variable, "variable", map[string]any{
		"parameter": []any{
			map[string]any{"key": "name", "value": "ecommerce.value"},
			map[string]any{"key": "value", "value": "constant"},
			map[string]any{"key": "cookieName", "value": "_ga"},
			map[string]any{"key": "javascript", "value": "function(){return 1}"},
		},
	})
	if variable.DataLayerName.ValueString() != "ecommerce.value" || variable.Value.ValueString() != "constant" || variable.CookieName.ValueString() != "_ga" || variable.JavaScript.ValueString() != "function(){return 1}" {
		t.Fatalf("variable drift fields were not applied: %#v", variable)
	}
}

func TestApplyGA4RemoteCompletesDriftFields(t *testing.T) {
	property := ga4TypedModel{}
	applyGA4Remote(&property, "property", map[string]any{
		"name":         "properties/123",
		"displayName":  "Landing",
		"timeZone":     "America/Argentina/Buenos_Aires",
		"currencyCode": "ARS",
	})
	if property.DisplayName.ValueString() != "Landing" || property.TimeZone.ValueString() != "America/Argentina/Buenos_Aires" || property.CurrencyCode.ValueString() != "ARS" {
		t.Fatalf("property drift fields were not applied: %#v", property)
	}

	stream := ga4TypedModel{}
	applyGA4Remote(&stream, "web_data_stream", map[string]any{
		"name":        "properties/123/dataStreams/456",
		"displayName": "Web",
		"webStreamData": map[string]any{
			"measurementId": "G-ABC123",
			"defaultUri":    "https://example.com",
		},
	})
	if stream.DisplayName.ValueString() != "Web" || stream.DefaultURI.ValueString() != "https://example.com" || stream.MeasurementID.ValueString() != "G-ABC123" {
		t.Fatalf("web stream drift fields were not applied: %#v", stream)
	}
}

func TestBuildGTMGA4EventTagPayloadOmitsMeasurementID(t *testing.T) {
	model := gtmGA4EventTagModel{
		Name:       types.StringValue("GA4 - signup_started"),
		EventName:  types.StringValue("signup_started"),
		TriggerIDs: stringListValue([]string{"123"}),
	}
	payload := buildGTMGA4EventTagPayload(context.Background(), model)

	if got, want := payload["type"], "gaawe"; got != want {
		t.Fatalf("type = %#v, want %#v", got, want)
	}
	if got, want := payload["firingTriggerId"], []string{"123"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("firingTriggerId = %#v, want %#v", got, want)
	}
	params := payload["parameter"].([]any)
	if !hasGTMParam(params, "eventName", "signup_started") {
		t.Fatalf("unexpected GA4 event tag parameters: %#v", params)
	}
	if hasGTMParamKey(params, "measurementId") || hasGTMParamKey(params, "measurementIdOverride") {
		t.Fatalf("GA4 event tag should not include measurement ID by default: %#v", params)
	}
}

func TestBuildGTMGA4EventTagPayloadAllowsMeasurementIDOverride(t *testing.T) {
	model := gtmGA4EventTagModel{
		Name:                  types.StringValue("GA4 - purchase"),
		EventName:             types.StringValue("purchase"),
		MeasurementIDOverride: types.StringValue("G-ABC123"),
		TriggerIDs:            stringListValue([]string{"123"}),
	}
	payload := buildGTMGA4EventTagPayload(context.Background(), model)
	params := payload["parameter"].([]any)
	if !hasGTMParam(params, "measurementIdOverride", "G-ABC123") {
		t.Fatalf("unexpected GA4 event tag parameters: %#v", params)
	}
}

func TestGA4WebStreamUpdateMaskIncludesDefaultURI(t *testing.T) {
	model := ga4TypedModel{
		DisplayName: types.StringValue("Web"),
		DefaultURI:  types.StringValue("https://example.com"),
	}
	got := ga4UpdateFields("web_data_stream", model)
	want := []string{"displayName", "webStreamData.defaultUri"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ga4UpdateFields() = %#v, want %#v", got, want)
	}
}

func hasGTMParam(params []any, key, value string) bool {
	for _, raw := range params {
		param, _ := raw.(map[string]any)
		if param["key"] == key && param["value"] == value {
			return true
		}
	}
	return false
}

func hasGTMParamKey(params []any, key string) bool {
	for _, raw := range params {
		param, _ := raw.(map[string]any)
		if param["key"] == key {
			return true
		}
	}
	return false
}
