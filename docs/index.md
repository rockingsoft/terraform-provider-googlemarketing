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
resource "googlemarketing_gtm_container_release" "tracking" {
  account_id     = var.gtm_account_id
  container_id   = var.gtm_container_id
  workspace_name = "Default Workspace"
  name           = "Terraform ${var.release_revision}"
  revision       = var.release_revision

  trigger {
    key               = "purchase"
    name              = "Event - purchase"
    type              = "customEvent"
    custom_event_name = "purchase"
  }

  tag {
    key          = "custom_purchase"
    name         = "Custom purchase tag"
    type         = "html"
    html         = "<script>window.dataLayer.push({event: 'purchase_seen'});</script>"
    trigger_keys = ["purchase"]
  }
}
```

## Resources

Google Tag Manager:

- `googlemarketing_gtm_container_release`

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
