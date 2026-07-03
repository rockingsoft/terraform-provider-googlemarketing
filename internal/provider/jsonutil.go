package provider

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

func decodeJSONObject(raw string) (map[string]any, error) {
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, fmt.Errorf("payload_json must be a JSON object")
	}
	return m, nil
}

func encodeJSON(v any) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func normalizeJSONString(raw string) (string, error) {
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return "", err
	}
	return encodeJSON(v)
}

func normalizeJSONValue(v any) (string, error) {
	return encodeJSON(v)
}

func updateMaskFromPayload(raw string) string {
	m, err := decodeJSONObject(raw)
	if err != nil {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		if k != "name" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

var gtmWorkspaceEntityPathRE = regexp.MustCompile(`^accounts/([^/]+)/containers/([^/]+)/workspaces/([^/]+)/(tags|triggers|variables|folders)/([^/]+)$`)
var gtmGoogleTagConfigPathRE = regexp.MustCompile(`^accounts/([^/]+)/containers/([^/]+)/workspaces/([^/]+)/gtag_config/([^/]+)$`)

type gtmWorkspaceEntityImport struct {
	AccountID   string
	ContainerID string
	WorkspaceID string
	Kind        string
	Path        string
}

func parseGTMWorkspaceEntityPath(raw string) (gtmWorkspaceEntityImport, error) {
	matches := gtmWorkspaceEntityPathRE.FindStringSubmatch(strings.Trim(raw, "/"))
	if matches == nil {
		return gtmWorkspaceEntityImport{}, fmt.Errorf("expected accounts/{account_id}/containers/{container_id}/workspaces/{workspace_id}/{tags|triggers|variables|folders}/{id}")
	}
	kind := strings.TrimSuffix(matches[4], "s")
	return gtmWorkspaceEntityImport{
		AccountID:   matches[1],
		ContainerID: matches[2],
		WorkspaceID: matches[3],
		Kind:        kind,
		Path:        strings.Trim(raw, "/"),
	}, nil
}

type gtmGoogleTagConfigImport struct {
	AccountID    string
	ContainerID  string
	WorkspaceID  string
	GtagConfigID string
	Path         string
}

func parseGTMGoogleTagConfigPath(raw string) (gtmGoogleTagConfigImport, error) {
	name := strings.Trim(raw, "/")
	matches := gtmGoogleTagConfigPathRE.FindStringSubmatch(name)
	if matches == nil {
		return gtmGoogleTagConfigImport{}, fmt.Errorf("expected accounts/{account_id}/containers/{container_id}/workspaces/{workspace_id}/gtag_config/{gtag_config_id}")
	}
	return gtmGoogleTagConfigImport{
		AccountID:    matches[1],
		ContainerID:  matches[2],
		WorkspaceID:  matches[3],
		GtagConfigID: matches[4],
		Path:         name,
	}, nil
}

var ga4AdminPathRE = regexp.MustCompile(`^(properties/[^/]+)/([^/]+)/[^/]+$`)

type ga4AdminImport struct {
	Parent     string
	Collection string
	Name       string
}

func parseGA4AdminPath(raw string) (ga4AdminImport, error) {
	name := strings.Trim(raw, "/")
	if parts := strings.Split(name, "."); len(parts) == 3 {
		propertyID, collection, resourceID := parts[0], parts[1], parts[2]
		if propertyID != "" && collection != "" && resourceID != "" {
			name = fmt.Sprintf("properties/%s/%s/%s", propertyID, collection, resourceID)
			return ga4AdminImport{Parent: "properties/" + propertyID, Collection: collection, Name: name}, nil
		}
	}
	matches := ga4AdminPathRE.FindStringSubmatch(name)
	if matches == nil {
		return ga4AdminImport{}, fmt.Errorf("expected {property_id}.{collection}.{resource_id} or properties/{property_id}/{collection}/{resource_id}")
	}
	return ga4AdminImport{Parent: matches[1], Collection: matches[2], Name: name}, nil
}
