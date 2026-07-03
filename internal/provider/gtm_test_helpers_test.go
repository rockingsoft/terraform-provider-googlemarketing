package provider

import (
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func gtmReleaseTestResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func gtmReleaseTestPayloadName(t *testing.T, raw []byte) string {
	t.Helper()
	body := string(raw)
	const prefix = `"name":"`
	start := strings.Index(body, prefix)
	if start == -1 {
		t.Fatalf("payload has no name: %s", body)
	}
	start += len(prefix)
	end := strings.Index(body[start:], `"`)
	if end == -1 {
		t.Fatalf("payload has unterminated name: %s", body)
	}
	return body[start : start+end]
}

func assertStringRequiresReplace(t *testing.T, name string, attribute schema.StringAttribute) {
	t.Helper()
	if len(attribute.PlanModifiers) == 0 {
		t.Fatalf("%s has no string plan modifiers", name)
	}
	want := reflect.TypeOf(stringplanmodifier.RequiresReplace())
	for _, modifier := range attribute.PlanModifiers {
		if reflect.TypeOf(modifier) == want {
			return
		}
	}
	t.Fatalf("%s does not require replacement", name)
}

func assertBoolRequiresReplace(t *testing.T, name string, attribute schema.BoolAttribute) {
	t.Helper()
	if len(attribute.PlanModifiers) == 0 {
		t.Fatalf("%s has no bool plan modifiers", name)
	}
	want := reflect.TypeOf(boolplanmodifier.RequiresReplace())
	for _, modifier := range attribute.PlanModifiers {
		if reflect.TypeOf(modifier) == want {
			return
		}
	}
	t.Fatalf("%s does not require replacement", name)
}
