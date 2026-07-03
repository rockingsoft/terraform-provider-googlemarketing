# googlemarketing_gtm_tag

Manages a single Google Tag Manager tag as its own resource, including GA4 event tags (`type = "gaawe"`).

Writes go straight to the resolved GTM workspace (`workspace_name`, defaulting to `Default Workspace`) — nothing is published until a `googlemarketing_gtm_publish` resource for the same container creates and publishes a version.

## Example

```terraform
resource "googlemarketing_gtm_tag" "ga4_purchase" {
  account_id         = var.gtm_account_id
  container_id       = var.gtm_container_id
  name               = "GA4 - purchase"
  type               = "gaawe"
  event_name         = "purchase"
  measurement_id     = googlemarketing_ga4_web_data_stream.web.measurement_id
  firing_trigger_ids = [googlemarketing_gtm_trigger.purchase.entity_id]
}

resource "googlemarketing_gtm_tag" "posthog_page_view" {
  account_id         = var.gtm_account_id
  container_id       = var.gtm_container_id
  name               = "PostHog - page_view"
  type               = "html"
  html               = "<script>posthog.capture('page_view')</script>"
  firing_trigger_ids = [googlemarketing_gtm_trigger.page_view.entity_id]
}
```

## Arguments

- `account_id` - (Required, forces replacement) GTM account ID.
- `container_id` - (Required, forces replacement) GTM container ID.
- `workspace_name` - Workspace to edit. Defaults to `Default Workspace`. Re-resolved to the current workspace ID on every operation.
- `name` - (Required) Display name. An existing tag with this name in the workspace is adopted (updated in place) instead of creating a duplicate.
- `type` - (Required) GTM tag type, for example `gaawe` (GA4 event), `googtag` (GA4 config), `html` (custom HTML), or `awct` (Google Ads conversion).
- `notes` - Optional notes.
- `measurement_id` - GA4 measurement ID. Required when `type = "gaawe"` (GTM's `gaawe` template requires `measurementIdOverride`).
- `event_name` - GA4 event name.
- `html` - Custom HTML body for `html` tags.
- `conversion_id`, `conversion_label` - Google Ads conversion fields for `awct` tags.
- `firing_trigger_ids` / `blocking_trigger_ids` - `entity_id` values of `googlemarketing_gtm_trigger` resources.
- `additional_params` - Map of extra GTM template parameters not covered by a typed field above.

## Attributes

- `id` - Stable identifier (`accounts/{account_id}/containers/{container_id}/tags/{entity_id}`) that survives publishes.
- `entity_id` - Short GTM tag ID. Stable across publishes.
- `path` - Current GTM workspace-relative API path. Rotates on every publish; refreshed on every read.
- `workspace_id` - Current GTM workspace ID. Rotates on every publish; refreshed on every read.

## Import

```
terraform import googlemarketing_gtm_tag.ga4_purchase accounts/1/containers/2/tags/9
```
