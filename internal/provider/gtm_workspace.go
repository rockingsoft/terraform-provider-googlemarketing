package provider

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// resolveGTMWorkspaceID looks up the workspace ID for workspaceName (default
// "Default Workspace") within a container, caching the result. GTM recycles
// the workspace ID on every publish (create_version deletes the workspace it
// was built from), so every CRUD operation must re-resolve the current
// workspace by name rather than trusting a previously stored workspace ID.
func (c *marketingClient) resolveGTMWorkspaceID(ctx context.Context, accountID, containerID, workspaceName string) (string, error) {
	if workspaceName == "" {
		workspaceName = "Default Workspace"
	}
	cacheKey := fmt.Sprintf("accounts/%s/containers/%s|%s", accountID, containerID, workspaceName)
	if c.gtmCache != nil {
		if id, ok := c.gtmCache.getWorkspaceID(cacheKey); ok {
			return id, nil
		}
	}

	apiPath := fmt.Sprintf("accounts/%s/containers/%s/workspaces", accountID, containerID)
	var out struct {
		Workspace []map[string]any `json:"workspace"`
	}
	if err := c.doJSON(ctx, http.MethodGet, gtmURL(apiPath), nil, &out, nil); err != nil {
		return "", err
	}
	for _, workspace := range out.Workspace {
		if stringFromMap(workspace, "name") == workspaceName {
			workspaceID := stringFromMap(workspace, "workspaceId")
			if workspaceID == "" {
				return "", fmt.Errorf("workspace %q did not include workspaceId", workspaceName)
			}
			if c.gtmCache != nil {
				c.gtmCache.setWorkspaceID(cacheKey, workspaceID)
			}
			return workspaceID, nil
		}
	}
	return "", fmt.Errorf("workspace %q not found in container %s", workspaceName, containerID)
}

// invalidateGTMWorkspaces drops cached workspace IDs for a container.
// Publishing rotates the workspace ID (create_version deletes the workspace
// it was built from), so the cache must be invalidated once a version is
// created or entities would keep resolving to the now-deleted workspace.
func (c *marketingClient) invalidateGTMWorkspaces(accountID, containerID string) {
	if c.gtmCache == nil {
		return
	}
	prefix := fmt.Sprintf("accounts/%s/containers/%s|", accountID, containerID)
	c.gtmCache.invalidateWorkspaceIDsWithPrefix(prefix)
}

// gtmContainerKey identifies a container for the dirty registry and the
// workspace status cache, independent of which workspace is currently active.
func gtmContainerKey(accountID, containerID string) string {
	return fmt.Sprintf("accounts/%s/containers/%s", accountID, containerID)
}

// markGTMContainerDirty records that a plan or apply observed a pending
// change to an entity in this container. googlemarketing_gtm_publish reads
// this registry during ModifyPlan so it can show an update instead of
// requiring a manual depends_on list per entity.
func (c *marketingClient) markGTMContainerDirty(accountID, containerID string) {
	c.dirtyMu.Lock()
	defer c.dirtyMu.Unlock()
	if c.dirtyContainers == nil {
		c.dirtyContainers = map[string]bool{}
	}
	c.dirtyContainers[gtmContainerKey(accountID, containerID)] = true
}

func (c *marketingClient) isGTMContainerDirty(accountID, containerID string) bool {
	c.dirtyMu.Lock()
	defer c.dirtyMu.Unlock()
	return c.dirtyContainers[gtmContainerKey(accountID, containerID)]
}

func (c *marketingClient) clearGTMContainerDirty(accountID, containerID string) {
	c.dirtyMu.Lock()
	defer c.dirtyMu.Unlock()
	delete(c.dirtyContainers, gtmContainerKey(accountID, containerID))
}

// gtmWorkspaceHasPendingChanges asks GTM whether the given workspace has any
// uncommitted entity changes. This is the authoritative signal used at
// apply-time before creating a version, and as a fallback at plan-time to
// catch drift made outside Terraform (for example directly in the GTM UI).
func (c *marketingClient) gtmWorkspaceHasPendingChanges(ctx context.Context, accountID, containerID, workspaceID string) (bool, error) {
	apiPath := fmt.Sprintf("accounts/%s/containers/%s/workspaces/%s/status", accountID, containerID, workspaceID)
	var out struct {
		WorkspaceChange []map[string]any `json:"workspaceChange"`
	}
	if err := c.doJSON(ctx, http.MethodGet, gtmURL(apiPath), nil, &out, nil); err != nil {
		return false, err
	}
	return len(out.WorkspaceChange) > 0, nil
}

func gtmContainerEntityID(accountID, containerID, collection, entityID string) string {
	return fmt.Sprintf("accounts/%s/containers/%s/%s/%s", accountID, containerID, collection, entityID)
}

var gtmContainerEntityIDRE = regexp.MustCompile(`^accounts/([^/]+)/containers/([^/]+)/(tags|triggers|variables|folders)/([^/]+)$`)

type gtmContainerEntityImport struct {
	AccountID   string
	ContainerID string
	Kind        string
	EntityID    string
}

// parseGTMContainerEntityID parses the stable, workspace-agnostic import form
// accounts/{account}/containers/{container}/{collection}/{id}.
func parseGTMContainerEntityID(raw string) (gtmContainerEntityImport, error) {
	matches := gtmContainerEntityIDRE.FindStringSubmatch(strings.Trim(raw, "/"))
	if matches == nil {
		return gtmContainerEntityImport{}, fmt.Errorf("expected accounts/{account_id}/containers/{container_id}/{tags|triggers|variables|folders}/{id}")
	}
	return gtmContainerEntityImport{
		AccountID:   matches[1],
		ContainerID: matches[2],
		Kind:        strings.TrimSuffix(matches[3], "s"),
		EntityID:    matches[4],
	}, nil
}
