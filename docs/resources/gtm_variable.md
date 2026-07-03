# googlemarketing_gtm_variable

Manages a single Google Tag Manager variable as its own resource.

Writes go straight to the resolved GTM workspace (`workspace_name`, defaulting to `Default Workspace`) — nothing is published until a `googlemarketing_gtm_publish` resource for the same container creates and publishes a version. Because each variable is its own resource, changing one variable only plans a change for that variable, not for every variable/trigger/tag in the container.

## Example

```terraform
resource "googlemarketing_gtm_variable" "page_path" {
  account_id      = var.gtm_account_id
  container_id    = var.gtm_container_id
  name            = "DLV - page_path"
  type            = "v"
  data_layer_name = "page_path"
}
```

## Arguments

- `account_id` - (Required, forces replacement) GTM account ID.
- `container_id` - (Required, forces replacement) GTM container ID.
- `workspace_name` - Workspace to edit. Defaults to `Default Workspace`. The provider re-resolves this to the current workspace ID on every operation, since GTM recycles workspace IDs on every publish.
- `name` - (Required) Display name. If a variable with this name already exists in the workspace, it is adopted (updated in place) instead of creating a duplicate — this is what lets a first `terraform apply` of this resource take over variables left behind by hand-editing or by the old `googlemarketing_gtm_container_release` resource.
- `type` - (Required) GTM variable type, for example `v` (data layer), `c` (constant), `k` (cookie), or `jsm` (custom JavaScript).
- `notes` - Optional notes.
- `value` - Constant or lookup value.
- `data_layer_name` - Data layer variable name.
- `cookie_name` - First-party cookie name.
- `javascript` - Custom JavaScript body.
- `additional_params` - Map of extra GTM template parameters not covered by a typed field above.

## Attributes

- `id` - Stable identifier (`accounts/{account_id}/containers/{container_id}/variables/{entity_id}`) that survives publishes.
- `entity_id` - Short GTM variable ID. Stable across publishes.
- `path` - Current GTM workspace-relative API path. Rotates on every publish; refreshed on every read.
- `workspace_id` - Current GTM workspace ID. Rotates on every publish; refreshed on every read.

## Import

```
terraform import googlemarketing_gtm_variable.page_path accounts/1/containers/2/variables/9
```

A workspace-scoped path copied from the GTM UI/API also works and skips one lookup call:

```
terraform import googlemarketing_gtm_variable.page_path accounts/1/containers/2/workspaces/3/variables/9
```
