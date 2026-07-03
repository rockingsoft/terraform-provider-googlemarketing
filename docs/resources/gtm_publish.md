# googlemarketing_gtm_publish

Publishes a Google Tag Manager container whenever its workspace has pending changes, automatically — without a manual `depends_on` list of every `googlemarketing_gtm_variable`/`gtm_trigger`/`gtm_tag` resource.

One `googlemarketing_gtm_publish` resource per container is normal; it stays in state across every apply, republishing only when there is actually something to publish.

## How the automatic publish works

`googlemarketing_gtm_publish` never forces a replace: `version_name`, `notes`, `workspace_name`, and `publish` are plain attributes with no plan modifiers, so changing them by itself only updates metadata for whenever the next real publish happens — it never triggers a version bump on its own.

Whether a version actually gets created is decided by GTM's own workspace status, not by a hash you maintain:

- At plan time, `terraform plan` shows an update to this resource when either (a) an entity resource for the same container planned a change earlier in the same plan, or (b) GTM's workspace status API reports pending changes (this also catches drift made directly in the GTM UI).
- At apply time, the resource re-checks GTM's workspace status authoritatively before doing anything. If there is nothing pending, it leaves the previous `version_path`/`container_version_id` untouched instead of creating a no-op version.

For the plan-time preview to see entity changes from the *same* apply, this resource needs to be ordered after them. Reference their outputs (for example `firing_trigger_ids = [googlemarketing_gtm_trigger.x.entity_id]`) where you naturally can, and add `depends_on` for anything that isn't otherwise referenced — a single `depends_on = [module.gtm]` covering a submodule of entities is enough; you do not need to list every resource individually. Without any ordering, the provider still converges correctly, just possibly one `terraform apply` later (the authoritative status check at apply time never misses a real change; only the plan-time preview can lag by one run).

## Example

```terraform
resource "googlemarketing_gtm_publish" "tracking" {
  account_id   = var.gtm_account_id
  container_id = var.gtm_container_id
  version_name = "Terraform marketing tracking"
  depends_on   = [module.gtm]
}
```

## Arguments

- `account_id` - (Required, forces replacement) GTM account ID.
- `container_id` - (Required, forces replacement) GTM container ID.
- `workspace_name` - Workspace to publish from. Defaults to `Default Workspace`.
- `version_name` - (Required) Name recorded on the GTM container version whenever a new version is actually created.
- `notes` - Notes recorded on the GTM container version whenever a new version is actually created.
- `publish` - Whether to publish the created version, versus only creating it. Defaults to `true`.

## Attributes

- `id` - `accounts/{account_id}/containers/{container_id}`.
- `version_path` - Path of the most recently created GTM container version.
- `container_version_id` - ID of the most recently created GTM container version.
- `new_workspace_path` - Path of the workspace GTM created to replace the one consumed by the last publish.
- `published` - Whether the most recently created version was published.

## Notes

- Deleting this resource does not unpublish anything — GTM versions cannot be deleted through the API, so the last published version stays live. Deleting only stops Terraform from managing further publishes.
- If the version this resource last published disappears out of band, the next `terraform plan` republishes instead of erroring.

## Migrating from googlemarketing_gtm_container_release

`googlemarketing_gtm_container_release` (removed) bundled every variable, trigger, and tag into one resource where any change replaced the whole thing. To migrate:

1. Remove the old resource from state — its `Delete` was a no-op, so this does not touch anything in GTM: `terraform state rm googlemarketing_gtm_container_release.tracking`.
2. Rewrite the old `variable`/`trigger`/`tag`/`ga4_event_tag` blocks as one `googlemarketing_gtm_variable`/`gtm_trigger`/`gtm_tag` resource each (`ga4_event_tag` becomes `gtm_tag` with `type = "gaawe"`), replacing `trigger_keys`/`blocking_trigger_keys` with `firing_trigger_ids`/`blocking_trigger_ids` referencing `entity_id`.
3. Add one `googlemarketing_gtm_publish` resource for the container.
4. Apply. Each entity resource adopts (updates in place) the existing GTM entity with the same `name` instead of creating a duplicate, so this does not require an `import` block per entity.
