package provider

import "testing"

func TestDecodeJSONObject(t *testing.T) {
	got, err := decodeJSONObject(`{"b":2,"a":1}`)
	if err != nil {
		t.Fatalf("decodeJSONObject returned error: %v", err)
	}
	if got["a"].(float64) != 1 || got["b"].(float64) != 2 {
		t.Fatalf("unexpected object: %#v", got)
	}
	if _, err := decodeJSONObject(`[1,2]`); err == nil {
		t.Fatal("expected non-object JSON to fail")
	}
}

func TestUpdateMaskFromPayload(t *testing.T) {
	got := updateMaskFromPayload(`{"displayName":"Site","name":"properties/1","timeZone":"UTC"}`)
	want := "displayName,timeZone"
	if got != want {
		t.Fatalf("updateMaskFromPayload() = %q, want %q", got, want)
	}
}

func TestParseGTMWorkspaceEntityPath(t *testing.T) {
	got, err := parseGTMWorkspaceEntityPath("accounts/1/containers/2/workspaces/3/tags/4")
	if err != nil {
		t.Fatalf("parseGTMWorkspaceEntityPath returned error: %v", err)
	}
	if got.AccountID != "1" || got.ContainerID != "2" || got.WorkspaceID != "3" || got.Kind != "tag" {
		t.Fatalf("unexpected parsed path: %#v", got)
	}
}

func TestParseGTMGoogleTagConfigPath(t *testing.T) {
	got, err := parseGTMGoogleTagConfigPath("accounts/1/containers/2/workspaces/3/gtag_config/4")
	if err != nil {
		t.Fatalf("parseGTMGoogleTagConfigPath returned error: %v", err)
	}
	if got.AccountID != "1" || got.ContainerID != "2" || got.WorkspaceID != "3" || got.GtagConfigID != "4" {
		t.Fatalf("unexpected parsed path: %#v", got)
	}
}

func TestParseGTMContainerEntityID(t *testing.T) {
	got, err := parseGTMContainerEntityID("accounts/1/containers/2/triggers/123")
	if err != nil {
		t.Fatalf("parseGTMContainerEntityID returned error: %v", err)
	}
	if got.AccountID != "1" || got.ContainerID != "2" || got.Kind != "trigger" || got.EntityID != "123" {
		t.Fatalf("unexpected parsed ID: %#v", got)
	}
	if _, err := parseGTMContainerEntityID("accounts/1/containers/2/workspaces/3/triggers/123"); err == nil {
		t.Fatal("expected workspace-scoped path to fail the stable-form parser")
	}
}

func TestGTMContainerEntityID(t *testing.T) {
	got := gtmContainerEntityID("1", "2", "triggers", "123")
	want := "accounts/1/containers/2/triggers/123"
	if got != want {
		t.Fatalf("gtmContainerEntityID() = %q, want %q", got, want)
	}
}

func TestParseGA4AdminPath(t *testing.T) {
	got, err := parseGA4AdminPath("properties/123/keyEvents/456")
	if err != nil {
		t.Fatalf("parseGA4AdminPath returned error: %v", err)
	}
	if got.Parent != "properties/123" || got.Collection != "keyEvents" || got.Name != "properties/123/keyEvents/456" {
		t.Fatalf("unexpected parsed path: %#v", got)
	}
}

func TestParseGA4AdminPathShortForm(t *testing.T) {
	got, err := parseGA4AdminPath("123.keyEvents.456")
	if err != nil {
		t.Fatalf("parseGA4AdminPath returned error: %v", err)
	}
	if got.Parent != "properties/123" || got.Collection != "keyEvents" || got.Name != "properties/123/keyEvents/456" {
		t.Fatalf("unexpected parsed path: %#v", got)
	}
}
