terraform {
  required_providers {
    googlemarketing = {
      source  = "rockingsoft/googlemarketing"
      version = "1.0.0"
    }
  }
}

provider "googlemarketing" {}

data "googlemarketing_gtm_accounts" "all" {}

resource "googlemarketing_ga4_web_data_stream" "web" {
  parent_id    = var.ga4_property_id
  display_name = "Website"
  default_uri  = var.site_url
}

resource "googlemarketing_gtm_container_release" "release" {
  account_id     = var.gtm_account_id
  container_id   = var.gtm_container_id
  workspace_name = var.gtm_workspace_name
  name           = "Terraform ${var.release_revision}"
  notes          = "Published by Terraform"
  revision       = var.release_revision

  trigger {
    key               = "purchase"
    name              = "Purchase event"
    type              = "customEvent"
    custom_event_name = "purchase"
  }

  ga4_event_tag {
    key                     = "ga4_purchase"
    name                    = "GA4 purchase event"
    event_name              = "purchase"
    measurement_id_override = googlemarketing_ga4_web_data_stream.web.measurement_id
    trigger_keys            = ["purchase"]
  }
}

resource "googlemarketing_ga4_key_event" "purchase" {
  parent_id  = var.ga4_property_id
  event_name = "purchase"
}
