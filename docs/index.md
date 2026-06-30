# Google Marketing Provider

The Google Marketing provider manages Google Tag Manager, Google Analytics 4 Admin, and selected Google Ads API resources.

Use typed resources for normal infrastructure. Legacy generic resources remain available for API escape hatches and backwards compatibility.

## Installation

```terraform
terraform {
  required_providers {
    googlemarketing = {
      source  = "rockingsoft/googlemarketing"
      version = "0.1.0"
    }
  }
}

provider "googlemarketing" {}
```

## Authentication

Enable the Google APIs used by the resources you plan to manage:

```bash
gcloud services enable tagmanager.googleapis.com analyticsadmin.googleapis.com googleads.googleapis.com
```

Authenticate with Application Default Credentials:

```bash
gcloud auth application-default login \
  --scopes=https://www.googleapis.com/auth/cloud-platform,https://www.googleapis.com/auth/tagmanager.edit.containers,https://www.googleapis.com/auth/tagmanager.manage.accounts,https://www.googleapis.com/auth/analytics.edit,https://www.googleapis.com/auth/adwords
```

You can also provide service account or user credentials through the standard Google environment variable:

```bash
export GOOGLE_APPLICATION_CREDENTIALS="$PWD/credentials.json"
```

## Example Usage

```terraform
resource "googlemarketing_ga4_web_data_stream" "web" {
  parent_id    = var.ga4_property_id
  display_name = "Website"
  default_uri  = var.site_url
}

resource "googlemarketing_gtm_google_tag_config" "ga4" {
  account_id   = var.gtm_account_id
  container_id = var.gtm_container_id
  workspace_id = var.gtm_workspace_id
  tag_id       = googlemarketing_ga4_web_data_stream.web.measurement_id
}
```

## Resources

Google Tag Manager:

- `googlemarketing_gtm_tag`
- `googlemarketing_gtm_google_tag_config`
- `googlemarketing_gtm_ga4_event_tag`
- `googlemarketing_gtm_trigger`
- `googlemarketing_gtm_variable`
- `googlemarketing_gtm_folder`
- `googlemarketing_gtm_container_version`
- `googlemarketing_gtm_version_publication`

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
- `googlemarketing_gtm_workspaces`
- `googlemarketing_ads_accessible_customers`
