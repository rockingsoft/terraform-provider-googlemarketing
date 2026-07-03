# googlemarketing_ga4_data_retention_settings

Typed GA4 data retention settings singleton.

Manages `event_data_retention` and `reset_user_data_on_new_activity` for a property.

## Import

Import with `{property_id}`:

```bash
terraform import googlemarketing_ga4_data_retention_settings.example 123456789
```

The full GA4 Admin name is also accepted, for example `properties/123456789/dataRetentionSettings`.
