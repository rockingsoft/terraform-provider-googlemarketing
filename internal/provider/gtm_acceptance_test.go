package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestAccGTMGoogleTagConfigPayloadRealAPI(t *testing.T) {
	if os.Getenv("GOOGLEMARKETING_ACC") != "1" {
		t.Skip("set GOOGLEMARKETING_ACC=1 to run acceptance tests")
	}

	accountID := requiredEnv(t, "GOOGLEMARKETING_GTM_ACCOUNT_ID")
	containerID := requiredEnv(t, "GOOGLEMARKETING_GTM_CONTAINER_ID")
	workspaceID := requiredEnv(t, "GOOGLEMARKETING_GTM_WORKSPACE_ID")
	measurementID := requiredEnv(t, "GOOGLEMARKETING_ACC_MEASUREMENT_ID")

	ctx := context.Background()
	client, err := newClient(ctx, clientConfig{
		CredentialsFile: os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"),
		CredentialsJSON: os.Getenv("GOOGLEMARKETING_CREDENTIALS_JSON"),
	})
	if err != nil {
		t.Fatalf("newClient() error = %v", err)
	}

	model := gtmGoogleTagConfigModel{
		TagID: typesString(measurementID),
	}
	apiPath := fmt.Sprintf("accounts/%s/containers/%s/workspaces/%s/gtag_config", accountID, containerID, workspaceID)
	var created map[string]any
	if err := client.doJSON(ctx, http.MethodPost, gtmURL(apiPath), buildGTMGoogleTagConfigPayload(model), &created, nil); err != nil {
		t.Fatalf("create gtag_config failed: %v", err)
	}

	pathValue := stringFromMap(created, "path")
	if pathValue == "" {
		t.Fatalf("created gtag_config missing path: %#v", created)
	}
	t.Cleanup(func() {
		_ = client.doJSON(context.Background(), http.MethodDelete, gtmURL(pathValue), nil, nil, nil)
	})

	var read map[string]any
	if err := client.doJSON(ctx, http.MethodGet, gtmURL(pathValue), nil, &read, nil); err != nil {
		t.Fatalf("read gtag_config failed: %v", err)
	}
	if got := stringFromMap(read, "type"); got != "google" {
		t.Fatalf("gtag_config type = %q, want google; response: %#v", got, read)
	}
	if got := gtmParamMap(read["parameter"])["tagId"]; got != measurementID {
		t.Fatalf("gtag_config tagId = %q, want %q; response: %#v", got, measurementID, read)
	}
	if got := stringFromMap(read, "gtagConfigId"); got == "" {
		t.Fatalf("gtag_config missing gtagConfigId: %#v", read)
	}
}

func requiredEnv(t *testing.T, key string) string {
	t.Helper()
	value := os.Getenv(key)
	if value == "" {
		t.Skipf("set %s to run acceptance tests", key)
	}
	return value
}

func typesString(value string) types.String {
	return types.StringValue(value)
}
