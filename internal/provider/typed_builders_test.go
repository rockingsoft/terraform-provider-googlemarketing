package provider

import (
	"context"
	"reflect"
	"strings"
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

func TestBuildAdsConversionActionPayloadNativeFields(t *testing.T) {
	model := adsConversionActionModel{
		Name:                           types.StringValue("Demo request"),
		Type:                           types.StringValue("WEBPAGE"),
		Category:                       types.StringValue("SUBMIT_LEAD_FORM"),
		Status:                         types.StringValue("ENABLED"),
		CountingType:                   types.StringValue("ONE_PER_CLICK"),
		ClickThroughLookbackWindowDays: types.Int64Value(30),
		ViewThroughLookbackWindowDays:  types.Int64Value(1),
	}

	payload := buildAdsConversionActionPayload(model, false)

	want := map[string]any{
		"name":                           "Demo request",
		"type":                           "WEBPAGE",
		"category":                       "SUBMIT_LEAD_FORM",
		"status":                         "ENABLED",
		"countingType":                   "ONE_PER_CLICK",
		"clickThroughLookbackWindowDays": int64(30),
		"viewThroughLookbackWindowDays":  int64(1),
	}
	if !reflect.DeepEqual(payload, want) {
		t.Fatalf("payload = %#v, want %#v", payload, want)
	}
}

func TestAdsConversionActionMutateBodyUsesResourceSpecificOperationShape(t *testing.T) {
	operation := map[string]any{
		"create": map[string]any{
			"name": "Demo request",
		},
	}

	body := adsConversionActionMutateBody(operation)

	want := map[string]any{
		"operations": []any{operation},
	}
	if !reflect.DeepEqual(body, want) {
		t.Fatalf("body = %#v, want %#v", body, want)
	}
	operations := body["operations"].([]any)
	first := operations[0].(map[string]any)
	if _, ok := first["conversionActionOperation"]; ok {
		t.Fatalf("body contains conversionActionOperation wrapper: %#v", body)
	}
}

func TestAdsConversionActionReadQueryOmitsProhibitedValueSettings(t *testing.T) {
	query := adsConversionActionReadQuery("customers/123/conversionActions/456")

	if strings.Contains(query, "conversion_action.value_settings") {
		t.Fatalf("query includes prohibited value_settings field: %s", query)
	}
	if !strings.Contains(query, "conversion_action.tag_snippets") {
		t.Fatalf("query does not include tag_snippets: %s", query)
	}
}

func TestParseAdsConversionActionImportIDShortForm(t *testing.T) {
	customerID, resourceName, err := parseAdsConversionActionImportID("123.456")
	if err != nil {
		t.Fatalf("parse returned error: %v", err)
	}
	if customerID != "123" {
		t.Fatalf("customerID = %q, want 123", customerID)
	}
	if resourceName != "customers/123/conversionActions/456" {
		t.Fatalf("resourceName = %q, want customers/123/conversionActions/456", resourceName)
	}
}

func TestParseAdsConversionActionImportIDLongForm(t *testing.T) {
	customerID, resourceName, err := parseAdsConversionActionImportID("123/customers/123/conversionActions/456")
	if err != nil {
		t.Fatalf("parse returned error: %v", err)
	}
	if customerID != "123" {
		t.Fatalf("customerID = %q, want 123", customerID)
	}
	if resourceName != "customers/123/conversionActions/456" {
		t.Fatalf("resourceName = %q, want customers/123/conversionActions/456", resourceName)
	}
}

func TestParseGA4TypedImportIDShortForms(t *testing.T) {
	tests := []struct {
		name       string
		kind       string
		raw        string
		wantName   string
		wantParent string
	}{
		{
			name:       "property",
			kind:       "property",
			raw:        "111.222",
			wantName:   "properties/222",
			wantParent: "111",
		},
		{
			name:       "web data stream",
			kind:       "web_data_stream",
			raw:        "222.333",
			wantName:   "properties/222/dataStreams/333",
			wantParent: "222",
		},
		{
			name:       "key event",
			kind:       "key_event",
			raw:        "222.444",
			wantName:   "properties/222/keyEvents/444",
			wantParent: "222",
		},
		{
			name:       "custom dimension",
			kind:       "custom_dimension",
			raw:        "222.555",
			wantName:   "properties/222/customDimensions/555",
			wantParent: "222",
		},
		{
			name:       "custom metric",
			kind:       "custom_metric",
			raw:        "222.666",
			wantName:   "properties/222/customMetrics/666",
			wantParent: "222",
		},
		{
			name:       "data retention settings",
			kind:       "data_retention_settings",
			raw:        "222",
			wantName:   "properties/222/dataRetentionSettings",
			wantParent: "222",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseGA4TypedImportID(tt.kind, tt.raw)
			if err != nil {
				t.Fatalf("parse returned error: %v", err)
			}
			if got.Name != tt.wantName || got.ParentID != tt.wantParent {
				t.Fatalf("parse = %#v, want name=%q parent=%q", got, tt.wantName, tt.wantParent)
			}
		})
	}
}

func TestParseGA4TypedImportIDLongForm(t *testing.T) {
	got, err := parseGA4TypedImportID("web_data_stream", "properties/222/dataStreams/333")
	if err != nil {
		t.Fatalf("parse returned error: %v", err)
	}
	if got.Name != "properties/222/dataStreams/333" || got.ParentID != "" {
		t.Fatalf("parse = %#v, want long name without parent hint", got)
	}
}

func TestApplyAdsConversionActionRemoteExtractsTrackingOutputs(t *testing.T) {
	model := adsConversionActionModel{}
	applyAdsConversionActionRemote(&model, map[string]any{
		"resourceName": "customers/123/conversionActions/456",
		"name":         "Demo request",
		"type":         "WEBPAGE",
		"category":     "SUBMIT_LEAD_FORM",
		"status":       "ENABLED",
		"tagSnippets": []any{
			map[string]any{
				"type":         "WEBPAGE_ONCLICK",
				"pageFormat":   "HTML",
				"eventSnippet": "gtag('event', 'conversion', {'send_to': 'AW-999/click'});",
			},
			map[string]any{
				"type":          "WEBPAGE",
				"pageFormat":    "HTML",
				"globalSiteTag": "<script>gtag('config', 'AW-123');</script>",
				"eventSnippet":  "gtag('event', 'conversion', {'send_to': 'AW-123/label'});",
			},
		},
	})

	if model.SendTo.ValueString() != "AW-123/label" {
		t.Fatalf("send_to = %q, want AW-123/label", model.SendTo.ValueString())
	}
	if model.ConversionID.ValueString() != "AW-123" || model.ConversionLabel.ValueString() != "label" {
		t.Fatalf("conversion outputs = %q %q, want AW-123 label", model.ConversionID.ValueString(), model.ConversionLabel.ValueString())
	}
	var snippets []adsTagSnippetModel
	diags := model.TagSnippets.ElementsAs(context.Background(), &snippets, false)
	if diags.HasError() {
		t.Fatalf("tag snippets could not be decoded: %#v", diags)
	}
	if len(snippets) != 2 || snippets[1].GlobalSiteTag.ValueString() == "" {
		t.Fatalf("tag snippets were not mapped: %#v", snippets)
	}
}

func TestApplyAdsConversionActionRemoteMapsMissingTagSnippetsToKnownEmptyList(t *testing.T) {
	model := adsConversionActionModel{
		TagSnippets: types.ListUnknown(adsTagSnippetObjectType),
	}
	applyAdsConversionActionRemote(&model, map[string]any{
		"resourceName": "customers/123/conversionActions/456",
		"name":         "Demo request",
		"type":         "WEBPAGE",
		"category":     "SUBMIT_LEAD_FORM",
		"status":       "ENABLED",
	})

	if model.TagSnippets.IsUnknown() || model.TagSnippets.IsNull() {
		t.Fatalf("tag snippets = %#v, want known empty list", model.TagSnippets)
	}
	if len(model.TagSnippets.Elements()) != 0 {
		t.Fatalf("tag snippets length = %d, want 0", len(model.TagSnippets.Elements()))
	}
	if !model.SendTo.IsNull() || !model.ConversionID.IsNull() || !model.ConversionLabel.IsNull() {
		t.Fatalf("tracking outputs = %#v %#v %#v, want nulls", model.SendTo, model.ConversionID, model.ConversionLabel)
	}
}

func TestAdsSendToFromEventSnippetSupportsDoubleQuotes(t *testing.T) {
	got := adsSendToFromEventSnippet(`gtag("event", "conversion", {"send_to": "AW-123/abc_DEF-1"});`)
	if got != "AW-123/abc_DEF-1" {
		t.Fatalf("send_to = %q, want AW-123/abc_DEF-1", got)
	}
}

func TestBuildGTMTriggerPayloadCustomEvent(t *testing.T) {
	model := gtmTypedWorkspaceEntityModel{
		Name:            types.StringValue("Purchase event"),
		Type:            types.StringValue("CUSTOM_EVENT"),
		CustomEventName: types.StringValue("purchase"),
	}
	payload := buildGTMPayload(context.Background(), "trigger", model)
	if got, want := payload["type"], "customEvent"; got != want {
		t.Fatalf("trigger type = %#v, want %#v", got, want)
	}
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

func TestExistingGoogleTagConfigPathFindsMatchingTagID(t *testing.T) {
	out := map[string]any{
		"gtagConfig": []any{
			map[string]any{
				"path": "accounts/1/containers/2/workspaces/3/gtag_config/10",
				"parameter": []any{
					map[string]any{"key": "tagId", "value": "G-OTHER"},
				},
			},
			map[string]any{
				"path": "accounts/1/containers/2/workspaces/3/gtag_config/11",
				"parameter": []any{
					map[string]any{"key": "tagId", "value": "G-ABC123"},
				},
			},
		},
	}

	if got, want := existingGoogleTagConfigPath(out, "G-ABC123"), "accounts/1/containers/2/workspaces/3/gtag_config/11"; got != want {
		t.Fatalf("existingGoogleTagConfigPath() = %q, want %q", got, want)
	}
	if got := existingGoogleTagConfigPath(out, "G-MISSING"); got != "" {
		t.Fatalf("existingGoogleTagConfigPath() = %q, want empty", got)
	}
}

func TestGoogleTagConfigsSupported(t *testing.T) {
	supported, ok := googleTagConfigsSupported(map[string]any{
		"features": map[string]any{
			"supportGtagConfigs": false,
		},
	})
	if !ok {
		t.Fatalf("googleTagConfigsSupported() ok = false, want true")
	}
	if supported {
		t.Fatalf("googleTagConfigsSupported() supported = true, want false")
	}

	if _, ok := googleTagConfigsSupported(map[string]any{"features": map[string]any{}}); ok {
		t.Fatalf("googleTagConfigsSupported() ok = true, want false")
	}
}

func TestGTMTriggerTypeStatePreservesEquivalentPriorCasing(t *testing.T) {
	model := gtmTypedWorkspaceEntityModel{Type: types.StringValue("CUSTOM_EVENT")}
	applyGTMRemoteTypedFields(&model, "trigger", map[string]any{
		"type":              "customEvent",
		"customEventFilter": []any{gtmCondition("EQUALS", "{{_event}}", "purchase")},
	})

	if got, want := model.Type.ValueString(), "CUSTOM_EVENT"; got != want {
		t.Fatalf("trigger type = %q, want %q", got, want)
	}
}

func TestGTMTriggerTypeStateUsesRemoteWithoutPriorCasing(t *testing.T) {
	model := gtmTypedWorkspaceEntityModel{}
	applyGTMRemoteTypedFields(&model, "trigger", map[string]any{
		"type": "customEvent",
	})

	if got, want := model.Type.ValueString(), "customEvent"; got != want {
		t.Fatalf("trigger type = %q, want %q", got, want)
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

func TestValidateGTMTagAdditionalRequirementsRequiresMeasurementIDForGaawe(t *testing.T) {
	model := gtmTypedWorkspaceEntityModel{
		Name:             types.StringValue("GA4 - signup_started"),
		Type:             types.StringValue("gaawe"),
		EventName:        types.StringValue("signup_started"),
		FiringTriggerIDs: stringListValue([]string{"123"}),
	}

	if err := validateGTMTagAdditionalRequirements(model); err == nil {
		t.Fatalf("validateGTMTagAdditionalRequirements() error = nil, want error")
	}
}

func TestBuildGTMTagPayloadGaaweIncludesMeasurementIDOverride(t *testing.T) {
	model := gtmTypedWorkspaceEntityModel{
		Name:             types.StringValue("GA4 - purchase"),
		Type:             types.StringValue("gaawe"),
		EventName:        types.StringValue("purchase"),
		MeasurementID:    types.StringValue("G-ABC123"),
		FiringTriggerIDs: stringListValue([]string{"123"}),
	}
	payload := buildGTMPayload(context.Background(), "tag", model)
	if got, want := payload["type"], "gaawe"; got != want {
		t.Fatalf("type = %#v, want %#v", got, want)
	}
	if got, want := payload["firingTriggerId"], []string{"123"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("firingTriggerId = %#v, want %#v", got, want)
	}
	params := payload["parameter"].([]any)
	if !hasGTMParam(params, "eventName", "purchase") {
		t.Fatalf("unexpected GA4 event tag parameters: %#v", params)
	}
	if !hasGTMParam(params, "measurementIdOverride", "G-ABC123") {
		t.Fatalf("unexpected GA4 event tag parameters: %#v", params)
	}
	if err := validateGTMTagAdditionalRequirements(model); err != nil {
		t.Fatalf("validateGTMTagAdditionalRequirements() error = %v", err)
	}
}

func TestBuildGTMPayloadMergesAdditionalParamsSortedByKey(t *testing.T) {
	elements, diags := types.MapValueFrom(context.Background(), types.StringType, map[string]string{
		"zeta":  "z",
		"alpha": "a",
	})
	if diags.HasError() {
		t.Fatalf("MapValueFrom() diagnostics: %#v", diags)
	}
	model := gtmTypedWorkspaceEntityModel{
		Name:             types.StringValue("Timer"),
		Type:             types.StringValue("TIMER"),
		AdditionalParams: elements,
	}
	payload := buildGTMPayload(context.Background(), "trigger", model)
	params := payload["parameter"].([]any)
	if len(params) != 2 {
		t.Fatalf("parameter count = %d, want 2: %#v", len(params), params)
	}
	first := params[0].(map[string]any)
	if first["key"] != "alpha" || first["value"] != "a" {
		t.Fatalf("first param = %#v, want alpha=a", first)
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
