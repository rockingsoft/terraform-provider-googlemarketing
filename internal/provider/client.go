package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
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
	"https://www.googleapis.com/auth/tagmanager.manage.accounts",
	"https://www.googleapis.com/auth/analytics.edit",
	"https://www.googleapis.com/auth/adwords",
}

type clientConfig struct {
	CredentialsFile    string
	CredentialsJSON    string
	AdsDeveloperToken  string
	AdsLoginCustomerID string
	AdsAPIVersion      string
}

type marketingClient struct {
	httpClient         *http.Client
	adsDeveloperToken  string
	adsLoginCustomerID string
	adsAPIVersion      string
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
