# googlemarketing_ga4_admin_resource

Legacy generic GA4 Admin child resource.

Prefer typed GA4 resources for supported API objects. Use this resource as an escape hatch for GA4 Admin collections that are not yet typed.

## Import

Import with `{property_id}.{collection}.{resource_id}`:

```bash
terraform import googlemarketing_ga4_admin_resource.example 123456789.keyEvents.987654321
```

The full GA4 Admin path is also accepted, for example `properties/123456789/keyEvents/987654321`.
