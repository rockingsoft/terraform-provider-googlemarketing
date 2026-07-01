# googlemarketing_gtm_container_release

Publishes a complete Google Tag Manager container release from logical variables, triggers, and tags.

This resource is the supported GTM workflow in v1.0.0. It uses the current workspace named by `workspace_name` as an editable release area, resolves internal references by stable keys, creates a container version, and publishes it when `publish = true`.

Terraform state is anchored to the published container version. It is not anchored to workspace-scoped tag, trigger, or variable IDs, because GTM can rotate workspace IDs after publication.

## Example

```terraform
resource "googlemarketing_gtm_container_release" "tracking" {
  account_id     = var.gtm_account_id
  container_id   = var.gtm_container_id
  workspace_name = "Default Workspace"
  name           = "Terraform ${var.release_revision}"
  notes          = "Published by Terraform"
  revision       = var.release_revision
  publish        = true

  variable {
    key             = "page_path"
    name            = "DLV - page_path"
    type            = "v"
    data_layer_name = "page_path"
  }

  trigger {
    key               = "page_view"
    name              = "Event - page_view"
    type              = "customEvent"
    custom_event_name = "page_view"
  }

  tag {
    key          = "posthog_page_view"
    name         = "PostHog - page_view"
    type         = "html"
    html         = "<script>posthog.capture('page_view')</script>"
    trigger_keys = ["page_view"]
  }

  ga4_event_tag {
    key                     = "ga4_page_view"
    name                    = "GA4 - page_view"
    event_name              = "page_view"
    measurement_id_override = var.ga4_measurement_id
    trigger_keys            = ["page_view"]
  }
}
```

## Arguments

- `account_id` - GTM account ID.
- `container_id` - GTM container API ID.
- `workspace_name` - Workspace name to use as the editable release workspace. Defaults to `Default Workspace`.
- `name` - Name for the created container version.
- `notes` - Optional notes for the created container version.
- `revision` - Caller-controlled release fingerprint. Change this value to publish a new version.
- `publish` - Whether to publish the created version. Defaults to `true`.

## Nested Blocks

- `variable` - Declares a GTM variable. Supported fields include `key`, `name`, `type`, `notes`, `value`, `data_layer_name`, `cookie_name`, and `javascript`.
- `trigger` - Declares a GTM trigger. Supported fields include `key`, `name`, `type`, `notes`, `custom_event_name`, `filter_variable`, `filter_operator`, and `filter_value`.
- `tag` - Declares a generic GTM tag. Supported fields include `key`, `name`, `type`, `notes`, `html`, `measurement_id`, `event_name`, `conversion_id`, `conversion_label`, `trigger_keys`, and `blocking_trigger_keys`.
- `ga4_event_tag` - Declares a GA4 event tag. Supported fields include `key`, `name`, `notes`, `event_name`, `measurement_id_override`, `trigger_keys`, and `blocking_trigger_keys`.

`trigger_keys` and `blocking_trigger_keys` reference the logical `key` values of `trigger` blocks. The provider resolves those keys to GTM trigger IDs during apply.

## Attributes

- `id` - Published GTM version path.
- `workspace_id_used` - Workspace ID used for the most recent release operation.
- `container_version_id` - Created container version ID.
- `version_path` - Created container version path.
- `published` - Whether the version was published by this resource.
