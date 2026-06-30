# Terraform Provider Google Marketing

Provider Terraform para gestionar Google Tag Manager y Google Analytics 4 Admin con recursos tipados. La UX principal no requiere conocer ni escribir payloads de las APIs de Google.

## Setup local

```bash
git clone https://github.com/rockingsoft/terraform-googlemarketing-provider.git
cd terraform-googlemarketing-provider
go mod tidy
go install .

export GOOGLEMARKETING_PLATFORM="$(go env GOOS)_$(go env GOARCH)"
mkdir -p "$HOME/.terraform.d/plugins/registry.terraform.io/rockingsoft/googlemarketing/0.1.0/$GOOGLEMARKETING_PLATFORM"
cp "$(go env GOPATH)/bin/terraform-provider-googlemarketing" \
  "$HOME/.terraform.d/plugins/registry.terraform.io/rockingsoft/googlemarketing/0.1.0/$GOOGLEMARKETING_PLATFORM/terraform-provider-googlemarketing_v0.1.0"
```

## Credenciales

Habilitá las APIs:

```bash
gcloud services enable tagmanager.googleapis.com analyticsadmin.googleapis.com googleads.googleapis.com
```

Autenticación con Application Default Credentials:

```bash
gcloud auth application-default login \
  --scopes=https://www.googleapis.com/auth/cloud-platform,https://www.googleapis.com/auth/tagmanager.edit.containers,https://www.googleapis.com/auth/tagmanager.manage.accounts,https://www.googleapis.com/auth/analytics.edit,https://www.googleapis.com/auth/adwords
```

También podés usar credenciales JSON de Google:

```bash
export GOOGLE_APPLICATION_CREDENTIALS="$PWD/credentials.json"
```

## Provider

```hcl
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

## GA4 y GTM

```hcl
resource "googlemarketing_ga4_web_data_stream" "web" {
  parent_id    = var.ga4_property_id
  display_name = "Website"
  default_uri  = var.site_url
}

resource "googlemarketing_gtm_google_tag_config" "ga4" {
  account_id    = var.gtm_account_id
  container_id  = var.gtm_container_id
  workspace_id  = var.gtm_workspace_id
  tag_id        = googlemarketing_ga4_web_data_stream.web.measurement_id
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
```

Si administrás la propiedad GA4 con este provider y necesitás conservar un output numérico, usá `property_id`:

```hcl
output "ga4_property_id" {
  value = googlemarketing_ga4_property.landing.property_id
}
```

## Recursos Tipados

GTM:

- `googlemarketing_gtm_tag`
- `googlemarketing_gtm_google_tag_config`
- `googlemarketing_gtm_ga4_event_tag`
- `googlemarketing_gtm_trigger`
- `googlemarketing_gtm_variable`
- `googlemarketing_gtm_folder`
- `googlemarketing_gtm_container_version`
- `googlemarketing_gtm_version_publication`

GA4:

- `googlemarketing_ga4_property`
- `googlemarketing_ga4_web_data_stream`
- `googlemarketing_ga4_key_event`
- `googlemarketing_ga4_custom_dimension`
- `googlemarketing_ga4_custom_metric`
- `googlemarketing_ga4_data_retention_settings`

Los recursos genéricos legacy siguen existiendo para compatibilidad, pero no son la ruta recomendada.

## Validación

```bash
go test ./...
terraform init
terraform plan
```

Acceptance tests contra APIs reales:

```bash
export GOOGLEMARKETING_ACC=1
export GOOGLEMARKETING_GTM_ACCOUNT_ID="..."
export GOOGLEMARKETING_GTM_CONTAINER_ID="..."
export GOOGLEMARKETING_GTM_WORKSPACE_ID="..."
export GOOGLEMARKETING_ACC_MEASUREMENT_ID="G-..."
go test ./internal/provider -run TestAcc -count=1 -v
```

Para Google Tag Manager, el flujo GA4 recomendado es crear primero `googlemarketing_gtm_google_tag_config` y luego eventos con `googlemarketing_gtm_ga4_event_tag`. El recurso genérico `googlemarketing_gtm_tag` conserva compatibilidad legacy, incluyendo `measurementIdOverride` para `type = "gaawe"`.
