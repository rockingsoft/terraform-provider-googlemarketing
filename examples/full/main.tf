terraform {
  required_providers {
    googlemarketing = {
      source  = "rockingsoft/googlemarketing"
      version = "1.0.6"
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

resource "googlemarketing_gtm_trigger" "purchase" {
  account_id        = var.gtm_account_id
  container_id      = var.gtm_container_id
  workspace_name    = var.gtm_workspace_name
  name              = "Purchase event"
  type              = "CUSTOM_EVENT"
  custom_event_name = "purchase"
}

resource "googlemarketing_gtm_tag" "ga4_purchase" {
  account_id         = var.gtm_account_id
  container_id       = var.gtm_container_id
  workspace_name     = var.gtm_workspace_name
  name               = "GA4 purchase event"
  type               = "gaawe"
  event_name         = "purchase"
  measurement_id     = googlemarketing_ga4_web_data_stream.web.measurement_id
  firing_trigger_ids = [googlemarketing_gtm_trigger.purchase.entity_id]
}

resource "googlemarketing_gtm_publish" "release" {
  account_id     = var.gtm_account_id
  container_id   = var.gtm_container_id
  workspace_name = var.gtm_workspace_name
  version_name   = "Terraform release"
  notes          = "Published by Terraform"
  depends_on     = [googlemarketing_gtm_tag.ga4_purchase]
}

resource "googlemarketing_ga4_key_event" "purchase" {
  parent_id  = var.ga4_property_id
  event_name = "purchase"
}
