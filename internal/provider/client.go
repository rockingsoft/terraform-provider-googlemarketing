package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	gtmBaseURL = "https://tagmanager.googleapis.com/tagmanager/v2"
	gaBaseURL  = "https://analyticsadmin.googleapis.com/v1beta"
	adsBaseURL = "https://googleads.googleapis.com"
)

var retrySleep = time.Sleep

var googleScopes = []string{
	"https://www.googleapis.com/auth/tagmanager.edit.containers",
	"https://www.googleapis.com/auth/tagmanager.edit.containerversions",
	"https://www.googleapis.com/auth/tagmanager.manage.accounts",
	"https://www.googleapis.com/auth/tagmanager.publish",
	"https://www.googleapis.com/auth/analytics.edit",
	"https://www.googleapis.com/auth/adwords",
}

type clientConfig struct {
	CredentialsFile    string
	CredentialsJSON    string
	AdsDeveloperToken  string
	AdsLoginCustomerID string
	AdsAPIVersion      string
	GTMRequestInterval time.Duration
}

type marketingClient struct {
	httpClient         *http.Client
	adsDeveloperToken  string
	adsLoginCustomerID string
	adsAPIVersion      string
	gtmLimiter         *gtmRateLimiter
	gtmCache           *gtmCollectionCache
	adsMutationLocks   *keyedMutex
	dirtyMu            sync.Mutex
	dirtyContainers    map[string]bool
}

func newClient(ctx context.Context, cfg clientConfig) (*marketingClient, error) {
	var ts oauth2.TokenSource

	switch {
	case cfg.CredentialsJSON != "":
		creds, err := google.CredentialsFromJSON(ctx, []byte(cfg.CredentialsJSON), googleScopes...)
		if err != nil {
			return nil, err
		}
		ts = creds.TokenSource
	case cfg.CredentialsFile != "":
		raw, err := os.ReadFile(cfg.CredentialsFile)
		if err != nil {
			return nil, err
		}
		creds, err := google.CredentialsFromJSON(ctx, raw, googleScopes...)
		if err != nil {
			return nil, err
		}
		ts = creds.TokenSource
	default:
		defaultTS, err := google.DefaultTokenSource(ctx, googleScopes...)
		if err != nil {
			return nil, err
		}
		ts = defaultTS
	}

	return &marketingClient{
		httpClient:         oauth2.NewClient(ctx, ts),
		adsDeveloperToken:  cfg.AdsDeveloperToken,
		adsLoginCustomerID: cfg.AdsLoginCustomerID,
		adsAPIVersion:      cfg.AdsAPIVersion,
		gtmLimiter:         newGTMRateLimiter(cfg.GTMRequestInterval),
		gtmCache:           newGTMCollectionCache(),
		adsMutationLocks:   newKeyedMutex(),
	}, nil
}

func (c *marketingClient) doJSON(ctx context.Context, method, url string, in any, out any, headers map[string]string) error {
	var bodyBytes []byte
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return err
		}
		bodyBytes = raw
	}

	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if c.gtmLimiter != nil && isGTMURL(url) {
			if err := c.gtmLimiter.wait(ctx); err != nil {
				return err
			}
		}

		var body io.Reader
		if bodyBytes != nil {
			body = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, body)
		if err != nil {
			return err
		}
		if in != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		for k, v := range headers {
			if v != "" {
				req.Header.Set(k, v)
			}
		}

		res, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
		} else {
			raw, readErr := io.ReadAll(res.Body)
			res.Body.Close()
			if readErr != nil {
				return readErr
			}
			if res.StatusCode == http.StatusNotFound {
				return errNotFound
			}
			if res.StatusCode >= 200 && res.StatusCode < 300 {
				if out != nil && len(raw) > 0 {
					if err := json.Unmarshal(raw, out); err != nil {
						return err
					}
				}
				return nil
			}
			lastErr = fmt.Errorf("%s %s failed: status %d: %s", method, url, res.StatusCode, string(raw))
			if !retryableStatus(res.StatusCode) {
				return lastErr
			}
		}
		if attempt < 3 {
			retrySleep(time.Duration(1<<attempt) * 250 * time.Millisecond)
		}
	}
	return lastErr
}

func gtmURL(path string) string {
	return gtmBaseURL + "/" + strings.TrimPrefix(path, "/")
}

func isGTMURL(url string) bool {
	return strings.HasPrefix(url, gtmBaseURL+"/") || url == gtmBaseURL
}

func gaURL(path string) string {
	return gaBaseURL + "/" + strings.TrimPrefix(path, "/")
}

func (c *marketingClient) adsURL(customerID, method string) string {
	return fmt.Sprintf("%s/%s/customers/%s/%s", adsBaseURL, c.adsAPIVersion, customerID, method)
}

func (c *marketingClient) adsHeaders() map[string]string {
	return map[string]string{
		"developer-token":   c.adsDeveloperToken,
		"login-customer-id": c.adsLoginCustomerID,
	}
}

func retryableStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

var errNotFound = fmt.Errorf("not found")

type keyedMutex struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func newKeyedMutex() *keyedMutex {
	return &keyedMutex{locks: map[string]*sync.Mutex{}}
}

func (m *keyedMutex) lock(key string) func() {
	if m == nil {
		return func() {}
	}
	m.mu.Lock()
	lock, ok := m.locks[key]
	if !ok {
		lock = &sync.Mutex{}
		m.locks[key] = lock
	}
	m.mu.Unlock()

	lock.Lock()
	return lock.Unlock
}

const defaultGTMRequestInterval = 4 * time.Second

type gtmRateLimiter struct {
	interval time.Duration
	mu       sync.Mutex
	next     time.Time
}

func newGTMRateLimiter(interval time.Duration) *gtmRateLimiter {
	if interval < 0 {
		interval = 0
	}
	return &gtmRateLimiter{interval: interval}
}

func (l *gtmRateLimiter) wait(ctx context.Context) error {
	if l.interval == 0 {
		return nil
	}

	l.mu.Lock()
	now := time.Now()
	waitUntil := l.next
	if waitUntil.Before(now) {
		waitUntil = now
	}
	l.next = waitUntil.Add(l.interval)
	l.mu.Unlock()

	delay := time.Until(waitUntil)
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type gtmCollectionCache struct {
	mu           sync.Mutex
	collections  map[string]map[string]map[string]any
	containers   map[string]containerSupportCacheEntry
	workspaceIDs map[string]string
}

type containerSupportCacheEntry struct {
	supported bool
	ok        bool
}

func newGTMCollectionCache() *gtmCollectionCache {
	return &gtmCollectionCache{
		collections: map[string]map[string]map[string]any{},
		containers:  map[string]containerSupportCacheEntry{},
	}
}

func (c *gtmCollectionCache) getCollection(collectionPath string) (map[string]map[string]any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	items, ok := c.collections[collectionPath]
	if !ok {
		return nil, false
	}
	clone := make(map[string]map[string]any, len(items))
	for path, item := range items {
		clone[path] = cloneMap(item)
	}
	return clone, true
}

func (c *gtmCollectionCache) setCollection(collectionPath string, items map[string]map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	clone := make(map[string]map[string]any, len(items))
	for path, item := range items {
		clone[path] = cloneMap(item)
	}
	c.collections[collectionPath] = clone
}

func (c *gtmCollectionCache) invalidateCollection(collectionPath string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.collections, collectionPath)
}

func (c *gtmCollectionCache) setCollectionItem(collectionPath, itemPath string, item map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	items, ok := c.collections[collectionPath]
	if !ok {
		// Collection was never loaded; leave it uncached so the next read
		// fetches the full, authoritative list instead of a partial one.
		return
	}
	items[itemPath] = cloneMap(item)
}

func (c *gtmCollectionCache) removeCollectionItem(collectionPath, itemPath string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	items, ok := c.collections[collectionPath]
	if !ok {
		return
	}
	delete(items, itemPath)
}

func (c *gtmCollectionCache) getWorkspaceID(cacheKey string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	id, ok := c.workspaceIDs[cacheKey]
	return id, ok
}

func (c *gtmCollectionCache) setWorkspaceID(cacheKey, workspaceID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.workspaceIDs == nil {
		c.workspaceIDs = map[string]string{}
	}
	c.workspaceIDs[cacheKey] = workspaceID
}

func (c *gtmCollectionCache) invalidateWorkspaceIDsWithPrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for key := range c.workspaceIDs {
		if strings.HasPrefix(key, prefix) {
			delete(c.workspaceIDs, key)
		}
	}
}

func (c *gtmCollectionCache) getContainerSupport(containerPath string) (bool, bool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, found := c.containers[containerPath]
	return entry.supported, entry.ok, found
}

func (c *gtmCollectionCache) setContainerSupport(containerPath string, supported, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.containers[containerPath] = containerSupportCacheEntry{supported: supported, ok: ok}
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func parseDurationMillis(raw string) (time.Duration, error) {
	if raw == "" {
		return 0, nil
	}
	ms, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if ms < 0 {
		return 0, fmt.Errorf("must be greater than or equal to 0")
	}
	return time.Duration(ms) * time.Millisecond, nil
}
