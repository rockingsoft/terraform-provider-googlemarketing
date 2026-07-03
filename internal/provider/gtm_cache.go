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

// getGTMCollectionItemByID finds an entity by its stable short ID (for
// example triggerId) within a workspace collection. The item's workspace
// path is looked up from the same cached collection fetch that
// getGTMCollection uses, so reading N entities of the same kind costs a
// single GET.
func (c *marketingClient) getGTMCollectionItemByID(ctx context.Context, collectionPath, idKey, id string) (map[string]any, bool, error) {
	items, err := c.getGTMCollection(ctx, collectionPath)
	if err != nil {
		return nil, false, err
	}
	for _, item := range items {
		if stringFromMap(item, idKey) == id {
			return item, true, nil
		}
	}
	return nil, false, nil
}

// getGTMCollectionItemByName finds an entity by its display name within a
// workspace collection, used to adopt pre-existing entities on Create
// instead of failing on a duplicate-name error from the API.
func (c *marketingClient) getGTMCollectionItemByName(ctx context.Context, collectionPath, name string) (map[string]any, bool, error) {
	items, err := c.getGTMCollection(ctx, collectionPath)
	if err != nil {
		return nil, false, err
	}
	for _, item := range items {
		if stringFromMap(item, "name") == name {
			return item, true, nil
		}
	}
	return nil, false, nil
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

// updateGTMCollectionItem writes a single item back into an already-cached
// collection instead of invalidating the whole collection. This keeps a
// create/update of one entity from forcing a refetch (and an extra GET) the
// next time a sibling entity of the same kind is read in the same plan/apply.
func (c *marketingClient) updateGTMCollectionItem(collectionPath, itemPath string, item map[string]any) {
	if c.gtmCache == nil {
		return
	}
	c.gtmCache.setCollectionItem(strings.Trim(collectionPath, "/"), strings.Trim(itemPath, "/"), item)
}

// removeGTMCollectionItem drops a single item from an already-cached
// collection after a successful delete, avoiding a full collection refetch.
func (c *marketingClient) removeGTMCollectionItem(collectionPath, itemPath string) {
	if c.gtmCache == nil {
		return
	}
	c.gtmCache.removeCollectionItem(strings.Trim(collectionPath, "/"), strings.Trim(itemPath, "/"))
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
