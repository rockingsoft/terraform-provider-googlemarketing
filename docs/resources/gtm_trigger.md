# googlemarketing_gtm_trigger

Manages a single Google Tag Manager trigger as its own resource.

Writes go straight to the resolved GTM workspace (`workspace_name`, defaulting to `Default Workspace`) — nothing is published until a `googlemarketing_gtm_publish` resource for the same container creates and publishes a version. Reference `entity_id` from `googlemarketing_gtm_tag.firing_trigger_ids` / `blocking_trigger_ids` instead of the old key-based indirection.

## Example

```terraform
resource "googlemarketing_gtm_trigger" "purchase" {
  account_id        = var.gtm_account_id
  container_id      = var.gtm_container_id
  name              = "Event - purchase"
  type              = "CUSTOM_EVENT"
  custom_event_name = "purchase"
}
```

## Arguments

- `account_id` - (Required, forces replacement) GTM account ID.
- `container_id` - (Required, forces replacement) GTM container ID.
- `workspace_name` - Workspace to edit. Defaults to `Default Workspace`. Re-resolved to the current workspace ID on every operation.
- `name` - (Required) Display name. An existing trigger with this name in the workspace is adopted (updated in place) instead of creating a duplicate.
- `type` - (Required) GTM trigger type, for example `CUSTOM_EVENT`, `PAGEVIEW`, `CLICK`, `FORM_SUBMISSION`, `TIMER`, or `HISTORY_CHANGE`.
- `notes` - Optional notes.
- `custom_event_name` - Event name to match for `CUSTOM_EVENT` triggers.
- `filter_variable`, `filter_operator`, `filter_value` - Optional single condition filter, for example `filter_variable = "{{Page URL}}"`, `filter_operator = "CONTAINS"`, `filter_value = "/checkout"`.
- `additional_params` - Map of extra GTM template parameters not covered by a typed field above (for example `interval`/`eventName` on `TIMER` triggers).

## Attributes

- `id` - Stable identifier (`accounts/{account_id}/containers/{container_id}/triggers/{entity_id}`) that survives publishes.
- `entity_id` - Short GTM trigger ID. Stable across publishes. Reference this from `googlemarketing_gtm_tag.firing_trigger_ids`/`blocking_trigger_ids`.
- `path` - Current GTM workspace-relative API path. Rotates on every publish; refreshed on every read.
- `workspace_id` - Current GTM workspace ID. Rotates on every publish; refreshed on every read.

## Import

```
terraform import googlemarketing_gtm_trigger.purchase accounts/1/containers/2/triggers/9
```
