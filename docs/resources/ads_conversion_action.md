# googlemarketing_ads_conversion_action

Typed Google Ads conversion action resource. This resource uses native Terraform
attributes only; it does not accept raw JSON payloads.

## Example

```hcl
resource "googlemarketing_ads_conversion_action" "demo" {
  customer_id = var.google_ads_customer_id
  name        = "Mercalis - Demo request"
  type        = "WEBPAGE"
  category    = "SUBMIT_LEAD_FORM"
  status      = "ENABLED"
}

module "marketing_tracking" {
  source = "./modules/marketing_tracking"

  google_ads_demo_send_to = googlemarketing_ads_conversion_action.demo.send_to
}
```

Use `send_to` directly in Google Tag Manager Google Ads conversion tags. The
provider derives it from the Google Ads event snippet in `AW-.../<label>` format
and also exposes `conversion_id` and `conversion_label` separately.

## Tracking Snippet Selection

Google Ads can return multiple `tag_snippets`. The provider selects the snippet
used for `send_to` in this order:

1. `type == "WEBPAGE"` and `page_format == "HTML"` with an event snippet.
2. Any `WEBPAGE` snippet with an event snippet.
3. Any `WEBPAGE_ONCLICK` snippet with an event snippet.

If Google Ads does not return a usable web event snippet with a `send_to` value,
`send_to`, `conversion_id`, and `conversion_label` are null and Terraform emits
a warning during read/import.

## Import

Import by Google Ads customer ID and conversion action ID:

```bash
terraform import googlemarketing_ads_conversion_action.demo 1234567890.987654321
```

The full Google Ads resource name is also accepted with the customer ID prefix,
for example `1234567890/customers/1234567890/conversionActions/987654321`.
