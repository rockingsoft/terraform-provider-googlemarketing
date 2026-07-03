package provider

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

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

func TestDoJSONAppliesGTMRateLimit(t *testing.T) {
	client := &marketingClient{
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
				Request:    req,
			}, nil
		})},
		gtmLimiter: newGTMRateLimiter(25 * time.Millisecond),
	}

	start := time.Now()
	if err := client.doJSON(context.Background(), http.MethodGet, gtmURL("accounts"), nil, nil, nil); err != nil {
		t.Fatalf("first doJSON returned error: %v", err)
	}
	if err := client.doJSON(context.Background(), http.MethodGet, gtmURL("accounts"), nil, nil, nil); err != nil {
		t.Fatalf("second doJSON returned error: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 20*time.Millisecond {
		t.Fatalf("elapsed = %s, want GTM rate limit delay", elapsed)
	}
}

func TestDoJSONDoesNotRateLimitNonGTM(t *testing.T) {
	client := &marketingClient{
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"ok":true}`)),
				Request:    req,
			}, nil
		})},
		gtmLimiter: newGTMRateLimiter(time.Hour),
	}

	start := time.Now()
	if err := client.doJSON(context.Background(), http.MethodGet, gaURL("properties"), nil, nil, nil); err != nil {
		t.Fatalf("first doJSON returned error: %v", err)
	}
	if err := client.doJSON(context.Background(), http.MethodGet, gaURL("properties"), nil, nil, nil); err != nil {
		t.Fatalf("second doJSON returned error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("elapsed = %s, non-GTM requests should not be GTM rate limited", elapsed)
	}
}

func TestKeyedMutexSerializesSameKey(t *testing.T) {
	locks := newKeyedMutex()
	firstRelease := make(chan struct{})
	firstLocked := make(chan struct{})
	secondDone := make(chan struct{})
	var concurrent int32
	var maxConcurrent int32

	go func() {
		unlock := locks.lock("123")
		current := atomic.AddInt32(&concurrent, 1)
		atomic.StoreInt32(&maxConcurrent, current)
		close(firstLocked)
		<-firstRelease
		atomic.AddInt32(&concurrent, -1)
		unlock()
	}()

	<-firstLocked
	go func() {
		unlock := locks.lock("123")
		current := atomic.AddInt32(&concurrent, 1)
		for {
			max := atomic.LoadInt32(&maxConcurrent)
			if current <= max || atomic.CompareAndSwapInt32(&maxConcurrent, max, current) {
				break
			}
		}
		atomic.AddInt32(&concurrent, -1)
		unlock()
		close(secondDone)
	}()

	select {
	case <-secondDone:
		t.Fatal("second lock acquired before first lock was released")
	case <-time.After(20 * time.Millisecond):
	}

	close(firstRelease)
	select {
	case <-secondDone:
	case <-time.After(time.Second):
		t.Fatal("second lock did not acquire after first lock was released")
	}
	if got := atomic.LoadInt32(&maxConcurrent); got != 1 {
		t.Fatalf("max concurrent locks = %d, want 1", got)
	}
}

func TestGTMCollectionCacheReusesListResponse(t *testing.T) {
	requests := 0
	client := &marketingClient{
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests++
			if got, want := req.URL.Path, "/tagmanager/v2/accounts/1/containers/2/workspaces/3/tags"; got != want {
				t.Fatalf("request path = %q, want %q", got, want)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"tag": [
						{"path":"accounts/1/containers/2/workspaces/3/tags/4","tagId":"4","name":"A"},
						{"path":"accounts/1/containers/2/workspaces/3/tags/5","tagId":"5","name":"B"}
					]
				}`)),
				Request: req,
			}, nil
		})},
		gtmLimiter: newGTMRateLimiter(0),
		gtmCache:   newGTMCollectionCache(),
	}

	first, found, err := client.getGTMWorkspaceEntity(context.Background(), "accounts/1/containers/2/workspaces/3/tags/4")
	if err != nil || !found {
		t.Fatalf("first lookup found=%t err=%v", found, err)
	}
	if first["name"] != "A" {
		t.Fatalf("unexpected first item: %#v", first)
	}
	second, found, err := client.getGTMWorkspaceEntity(context.Background(), "accounts/1/containers/2/workspaces/3/tags/5")
	if err != nil || !found {
		t.Fatalf("second lookup found=%t err=%v", found, err)
	}
	if second["name"] != "B" {
		t.Fatalf("unexpected second item: %#v", second)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func TestGTMCollectionInvalidationReloadsListResponse(t *testing.T) {
	requests := 0
	client := &marketingClient{
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests++
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(`{"tag":[{"path":"accounts/1/containers/2/workspaces/3/tags/4","tagId":"4"}]}`)),
				Request:    req,
			}, nil
		})},
		gtmLimiter: newGTMRateLimiter(0),
		gtmCache:   newGTMCollectionCache(),
	}

	pathValue := "accounts/1/containers/2/workspaces/3/tags/4"
	if _, found, err := client.getGTMWorkspaceEntity(context.Background(), pathValue); err != nil || !found {
		t.Fatalf("first lookup found=%t err=%v", found, err)
	}
	client.invalidateGTMWorkspaceEntityCollection(pathValue)
	if _, found, err := client.getGTMWorkspaceEntity(context.Background(), pathValue); err != nil || !found {
		t.Fatalf("second lookup found=%t err=%v", found, err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d, want 2", requests)
	}
}
