terraform {
  required_providers {
    googlemarketing = {
      source  = "rockingsoft/googlemarketing"
      version = "0.1.0"
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

resource "googlemarketing_gtm_google_tag_config" "ga4" {
  account_id   = var.gtm_account_id
  container_id = var.gtm_container_id
  workspace_id = var.gtm_workspace_id
  tag_id       = googlemarketing_ga4_web_data_stream.web.measurement_id
}

resource "googlemarketing_gtm_trigger" "purchase" {
  account_id        = var.gtm_account_id
  container_id      = var.gtm_container_id
  workspace_id      = var.gtm_workspace_id
  name              = "Purchase event"
  type              = "CUSTOM_EVENT"
  custom_event_name = "purchase"
}

resource "googlemarketing_gtm_ga4_event_tag" "purchase" {
  account_id   = var.gtm_account_id
  container_id = var.gtm_container_id
  workspace_id = var.gtm_workspace_id
  name         = "GA4 purchase event"
  event_name   = "purchase"
  trigger_ids  = [googlemarketing_gtm_trigger.purchase.entity_id]
}

resource "googlemarketing_ga4_key_event" "purchase" {
  parent_id  = var.ga4_property_id
  event_name = "purchase"
}

resource "googlemarketing_gtm_container_version" "release" {
  account_id   = var.gtm_account_id
  container_id = var.gtm_container_id
  workspace_id = var.gtm_workspace_id
  name         = "Terraform release"
  notes        = "Published by Terraform"
  revision     = googlemarketing_gtm_ga4_event_tag.purchase.id
}

resource "googlemarketing_gtm_version_publication" "release" {
  account_id   = var.gtm_account_id
  container_id = var.gtm_container_id
  version_id   = googlemarketing_gtm_container_version.release.container_version_id
}
