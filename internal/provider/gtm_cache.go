package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func gtmWorkspaceCollectionPath(accountID, containerID, workspaceID, collection string) string {
	return fmt.Sprintf("accounts/%s/containers/%s/workspaces/%s/%s", accountID, containerID, workspaceID, collection)
}

func gtmCollectionPathForWorkspaceEntityPath(pathValue string) (string, error) {
	parsed, err := parseGTMWorkspaceEntityPath(pathValue)
	if err != nil {
		return "", err
	}
	collection, err := gtmEntityCollection(parsed.Kind)
	if err != nil {
		return "", err
	}
	return gtmWorkspaceCollectionPath(parsed.AccountID, parsed.ContainerID, parsed.WorkspaceID, collection), nil
}

func gtmCollectionPathForGoogleTagConfigPath(pathValue string) (string, error) {
	parsed, err := parseGTMGoogleTagConfigPath(pathValue)
	if err != nil {
		return "", err
	}
	return gtmWorkspaceCollectionPath(parsed.AccountID, parsed.ContainerID, parsed.WorkspaceID, "gtag_config"), nil
}

func (c *marketingClient) getGTMWorkspaceEntity(ctx context.Context, pathValue string) (map[string]any, bool, error) {
	collectionPath, err := gtmCollectionPathForWorkspaceEntityPath(pathValue)
	if err != nil {
		return nil, false, err
	}
	return c.getGTMCollectionItem(ctx, collectionPath, pathValue)
}

func (c *marketingClient) getGTMGoogleTagConfig(ctx context.Context, pathValue string) (map[string]any, bool, error) {
	collectionPath, err := gtmCollectionPathForGoogleTagConfigPath(pathValue)
	if err != nil {
		return nil, false, err
	}
	return c.getGTMCollectionItem(ctx, collectionPath, pathValue)
}

func (c *marketingClient) getGTMCollectionItem(ctx context.Context, collectionPath, itemPath string) (map[string]any, bool, error) {
	items, err := c.getGTMCollection(ctx, collectionPath)
	if err != nil {
		return nil, false, err
	}
	item, ok := items[strings.Trim(itemPath, "/")]
	if !ok {
		return nil, false, nil
	}
	return item, true, nil
}

func (c *marketingClient) getGTMCollection(ctx context.Context, collectionPath string) (map[string]map[string]any, error) {
	collectionPath = strings.Trim(collectionPath, "/")
	if c.gtmCache != nil {
		if items, ok := c.gtmCache.getCollection(collectionPath); ok {
			return items, nil
		}
	}

	items := map[string]map[string]any{}
	pageToken := ""
	for {
		apiURL := gtmURL(collectionPath)
		if pageToken != "" {
			u, err := url.Parse(apiURL)
			if err != nil {
				return nil, err
			}
			q := u.Query()
			q.Set("pageToken", pageToken)
			u.RawQuery = q.Encode()
			apiURL = u.String()
		}

		var out map[string]any
		if err := c.doJSON(ctx, http.MethodGet, apiURL, nil, &out, nil); err != nil {
			return nil, err
		}
		for _, item := range gtmCollectionItems(out, collectionPath) {
			pathValue := stringFromMap(item, "path")
			if pathValue != "" {
				items[strings.Trim(pathValue, "/")] = item
			}
		}
		pageToken = stringFromMap(out, "nextPageToken")
		if pageToken == "" {
			break
		}
	}

	if c.gtmCache != nil {
		c.gtmCache.setCollection(collectionPath, items)
	}
	return items, nil
}

func (c *marketingClient) invalidateGTMWorkspaceEntityCollection(pathValue string) {
	if c.gtmCache == nil {
		return
	}
	collectionPath, err := gtmCollectionPathForWorkspaceEntityPath(pathValue)
	if err != nil {
		return
	}
	c.gtmCache.invalidateCollection(collectionPath)
}

func (c *marketingClient) invalidateGTMGoogleTagConfigCollection(pathValue string) {
	if c.gtmCache == nil {
		return
	}
	collectionPath, err := gtmCollectionPathForGoogleTagConfigPath(pathValue)
	if err != nil {
		return
	}
	c.gtmCache.invalidateCollection(collectionPath)
}

func (c *marketingClient) invalidateGTMCollection(collectionPath string) {
	if c.gtmCache == nil {
		return
	}
	c.gtmCache.invalidateCollection(strings.Trim(collectionPath, "/"))
}

func gtmCollectionItems(out map[string]any, collectionPath string) []map[string]any {
	keys := gtmCollectionResponseKeys(collectionPath)
	items := []map[string]any{}
	for _, key := range keys {
		rawItems, _ := out[key].([]any)
		for _, rawItem := range rawItems {
			item, _ := rawItem.(map[string]any)
			if item != nil {
				items = append(items, item)
			}
		}
	}
	return items
}

func gtmCollectionResponseKeys(collectionPath string) []string {
	switch {
	case strings.HasSuffix(collectionPath, "/tags"):
		return []string{"tag", "tags"}
	case strings.HasSuffix(collectionPath, "/triggers"):
		return []string{"trigger", "triggers"}
	case strings.HasSuffix(collectionPath, "/variables"):
		return []string{"variable", "variables"}
	case strings.HasSuffix(collectionPath, "/folders"):
		return []string{"folder", "folders"}
	case strings.HasSuffix(collectionPath, "/gtag_config"):
		return []string{"gtagConfig", "gtagConfigs", "gtag_config"}
	default:
		return nil
	}
}
