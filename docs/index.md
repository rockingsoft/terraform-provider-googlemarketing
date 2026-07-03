# Google Marketing Provider

The Google Marketing provider manages Google Tag Manager releases, Google Analytics 4 Admin, and selected Google Ads API resources.

## Installation

```terraform
terraform {
  required_providers {
    googlemarketing = {
      source  = "rockingsoft/googlemarketing"
      version = "1.0.0"
    }
  }
}

provider "googlemarketing" {}
```

Google Tag Manager has a low project quota. The provider spaces GTM API calls by 4000 ms by default; configure `gtm_request_interval_ms` or `GOOGLEMARKETING_GTM_REQUEST_INTERVAL_MS` only if your project has a higher quota.

## Authentication

Enable the Google APIs used by the resources you plan to manage:

```bash
gcloud services enable tagmanager.googleapis.com analyticsadmin.googleapis.com googleads.googleapis.com
```

Authenticate with Application Default Credentials:

```bash
gcloud auth application-default login \
  --scopes=https://www.googleapis.com/auth/cloud-platform,https://www.googleapis.com/auth/tagmanager.edit.containers,https://www.googleapis.com/auth/tagmanager.edit.containerversions,https://www.googleapis.com/auth/tagmanager.manage.accounts,https://www.googleapis.com/auth/tagmanager.publish,https://www.googleapis.com/auth/analytics.edit,https://www.googleapis.com/auth/adwords
```

You can also provide service account or user credentials through the standard Google environment variable:

```bash
export GOOGLE_APPLICATION_CREDENTIALS="$PWD/credentials.json"
```

## Example Usage

```terraform
resource "googlemarketing_gtm_trigger" "purchase" {
  account_id        = var.gtm_account_id
  container_id      = var.gtm_container_id
  name              = "Event - purchase"
  type              = "CUSTOM_EVENT"
  custom_event_name = "purchase"
}

resource "googlemarketing_gtm_tag" "custom_purchase" {
  account_id         = var.gtm_account_id
  container_id       = var.gtm_container_id
  name               = "Custom purchase tag"
  type               = "html"
  html               = "<script>window.dataLayer.push({event: 'purchase_seen'});</script>"
  firing_trigger_ids = [googlemarketing_gtm_trigger.purchase.entity_id]
}

resource "googlemarketing_gtm_publish" "tracking" {
  account_id   = var.gtm_account_id
  container_id = var.gtm_container_id
  version_name = "Terraform release"
  # custom_purchase already depends on purchase (firing_trigger_ids), so
  # depending on the tag is enough to order this after both.
  depends_on = [googlemarketing_gtm_tag.custom_purchase]
}
```

`googlemarketing_gtm_variable`, `googlemarketing_gtm_trigger`, and `googlemarketing_gtm_tag` are independent resources: changing one only plans a change for that entity, not for the whole container. `googlemarketing_gtm_publish` creates and publishes a new GTM container version automatically whenever the workspace actually has pending changes — see [`googlemarketing_gtm_publish`](resources/gtm_publish.md) for how it decides that without a manual `depends_on` list per entity.

## Resources

Google Tag Manager:

- `googlemarketing_gtm_variable`
- `googlemarketing_gtm_trigger`
- `googlemarketing_gtm_tag`
- `googlemarketing_gtm_publish`

Google Analytics 4 Admin:

- `googlemarketing_ga4_property`
- `googlemarketing_ga4_web_data_stream`
- `googlemarketing_ga4_key_event`
- `googlemarketing_ga4_custom_dimension`
- `googlemarketing_ga4_custom_metric`
- `googlemarketing_ga4_data_retention_settings`

Google Ads:

- `googlemarketing_ads_conversion_action`
- `googlemarketing_ads_mutate`

## Data Sources

- `googlemarketing_gtm_accounts`
- `googlemarketing_ads_accessible_customers`
