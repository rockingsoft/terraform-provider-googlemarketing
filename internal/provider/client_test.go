package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestURLConstruction(t *testing.T) {
	if got, want := gtmURL("/accounts/1"), gtmBaseURL+"/accounts/1"; got != want {
		t.Fatalf("gtmURL() = %q, want %q", got, want)
	}
	if got, want := gaURL("properties/1"), gaBaseURL+"/properties/1"; got != want {
		t.Fatalf("gaURL() = %q, want %q", got, want)
	}
	c := &marketingClient{adsAPIVersion: "v24"}
	if got, want := c.adsURL("123", "googleAds:mutate"), adsBaseURL+"/v24/customers/123/googleAds:mutate"; got != want {
		t.Fatalf("adsURL() = %q, want %q", got, want)
	}
}

func TestAdsHeaders(t *testing.T) {
	c := &marketingClient{adsDeveloperToken: "dev", adsLoginCustomerID: "login"}
	got := c.adsHeaders()
	if got["developer-token"] != "dev" || got["login-customer-id"] != "login" {
		t.Fatalf("unexpected headers: %#v", got)
	}
}

func TestDoJSONRetriesRetryableStatus(t *testing.T) {
	originalSleep := retrySleep
	retrySleep = func(time.Duration) {}
	defer func() { retrySleep = originalSleep }()

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client := &marketingClient{httpClient: server.Client()}
	var out map[string]any
	if err := client.doJSON(context.Background(), http.MethodGet, server.URL, nil, &out, nil); err != nil {
		t.Fatalf("doJSON returned error: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if out["ok"] != true {
		t.Fatalf("unexpected response: %#v", out)
	}
}
