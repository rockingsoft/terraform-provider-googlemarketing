variable "gtm_account_id" {
  type = string
}

variable "gtm_container_id" {
  type = string
}

variable "gtm_workspace_name" {
  type    = string
  default = "Default Workspace"
}

variable "release_revision" {
  type        = string
  description = "Fingerprint of the desired GTM release content. Avoid global deploy hashes unless every deploy should publish GTM."
}

variable "ga4_property_id" {
  type = string
}

variable "site_url" {
  type = string
}
