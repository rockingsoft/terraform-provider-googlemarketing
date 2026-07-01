# Terraform Provider Google Marketing

Terraform provider for managing Google Tag Manager releases, Google Analytics 4 Admin, and selected Google Ads API resources.

## Local Setup

```bash
git clone https://github.com/rockingsoft/terraform-provider-googlemarketing.git
cd terraform-provider-googlemarketing
go mod tidy
go install .

export GOOGLEMARKETING_PLATFORM="$(go env GOOS)_$(go env GOARCH)"
mkdir -p "$HOME/.terraform.d/plugins/registry.terraform.io/rockingsoft/googlemarketing/1.0.0/$GOOGLEMARKETING_PLATFORM"
cp "$(go env GOPATH)/bin/terraform-provider-googlemarketing" \
  "$HOME/.terraform.d/plugins/registry.terraform.io/rockingsoft/googlemarketing/1.0.0/$GOOGLEMARKETING_PLATFORM/terraform-provider-googlemarketing_v1.0.0"
```

## Credentials

Enable the APIs:

```bash
gcloud services enable tagmanager.googleapis.com analyticsadmin.googleapis.com googleads.googleapis.com
```

Authenticate with Application Default Credentials:

```bash
gcloud auth application-default login \
  --scopes=https://www.googleapis.com/auth/cloud-platform,https://www.googleapis.com/auth/tagmanager.edit.containers,https://www.googleapis.com/auth/tagmanager.edit.containerversions,https://www.googleapis.com/auth/tagmanager.manage.accounts,https://www.googleapis.com/auth/tagmanager.publish,https://www.googleapis.com/auth/analytics.edit,https://www.googleapis.com/auth/adwords
```

You can also use Google JSON credentials:

```bash
export GOOGLE_APPLICATION_CREDENTIALS="$PWD/credentials.json"
```

## Provider

```hcl
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

Google Tag Manager enforces a low project quota. By default the provider spaces GTM API calls by 4000 ms. Override this only if your project has a higher quota:

```hcl
provider "googlemarketing" {
  gtm_request_interval_ms = 4000
}
```

You can also set `GOOGLEMARKETING_GTM_REQUEST_INTERVAL_MS`. Use `0` to disable provider-side GTM pacing.

## GA4 and GTM Release

```hcl
resource "googlemarketing_ga4_web_data_stream" "web" {
  parent_id    = var.ga4_property_id
  display_name = "Website"
  default_uri  = var.site_url
}

resource "googlemarketing_gtm_container_release" "tracking" {
  account_id     = var.gtm_account_id
  container_id   = var.gtm_container_id
  workspace_name = "Default Workspace"
  name           = "Terraform ${var.release_revision}"
  notes          = "Published by Terraform"
  revision       = var.release_revision

  trigger {
    key               = "purchase"
    name              = "Event - purchase"
    type              = "customEvent"
    custom_event_name = "purchase"
  }

  ga4_event_tag {
    key                     = "ga4_purchase"
    name                    = "GA4 - purchase"
    event_name              = "purchase"
    measurement_id_override = googlemarketing_ga4_web_data_stream.web.measurement_id
    trigger_keys            = ["purchase"]
  }
}

resource "googlemarketing_ga4_key_event" "purchase" {
  parent_id  = var.ga4_property_id
  event_name = "purchase"
}
```

`googlemarketing_gtm_container_release` treats a GTM workspace as an editable release area. Terraform state is anchored to the published container version, not to workspace-scoped tag, trigger, or variable IDs that can rotate after publish.

## Resources

GTM:

- `googlemarketing_gtm_container_release`

GA4:

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

## Validation

```bash
go test ./...
terraform init
terraform plan
```
