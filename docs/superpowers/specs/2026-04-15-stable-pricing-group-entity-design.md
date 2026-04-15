# Stable Pricing Group Entity Design

## Summary

Hermestoken currently treats a pricing group as a mutable string value. The same
string is stored across users, tokens, subscription plans, subscription
snapshots, runtime option JSON, and channel/model routing data. This makes a
"rename" operation unsafe because a display-level change behaves like a global
primary key rewrite.

The production inconsistency between `cc-oups4.6-福利渠道` and
`cc-opus4.6-福利渠道` is a direct result of that model: configuration and plan
data were updated, while historical user and subscription records were not.

The correct long-term fix is to make pricing groups first-class entities with a
stable internal identifier, move runtime rules out of free-form JSON where
practical, and convert rename/delete operations into explicit, auditable domain
operations.

## Problem Statement

Today a group name is doing four jobs at once:

1. It is the user-facing display name.
2. It is the runtime routing key.
3. It is the billing key.
4. It is the historical snapshot value stored in business records.

That design creates the following failure modes:

- Renaming a group can silently orphan users, tokens, or subscriptions.
- Deleting a group can break token access or pricing resolution at runtime.
- Different subsystems can disagree on the canonical spelling of the same group.
- Existing option JSON values cannot be validated with foreign keys.
- Admin UI edits can mutate data semantics far beyond what the UI suggests.

The existing fallback logic for subscription upgrade groups is a necessary
safety net, but it only masks one symptom. It does not solve the underlying
data model problem.

## Goals

- Separate stable group identity from editable display text.
- Ensure a group rename does not require rewriting unrelated business history.
- Make all active group references queryable and auditable.
- Allow safe migration from legacy string-based storage with minimal downtime.
- Block future invalid states through schema constraints and admin workflow
  changes.

## Non-Goals

- Redesign all pricing logic unrelated to groups.
- Rebuild channel or model management UI from scratch.
- Remove every legacy JSON option in one release if a staged migration is safer.

## Target Domain Model

### Core Entity

Add a dedicated `pricing_groups` table:

- `id` bigint primary key
- `group_key` varchar unique, immutable, internal identifier
- `display_name` varchar, editable
- `billing_ratio` decimal or float, not null
- `user_selectable` boolean, not null
- `status` enum or smallint for active/deprecated/archived
- `sort_order` integer, not null default `0`
- `description` varchar or text
- `created_at`, `updated_at`
- `created_by`, `updated_by` where admin audit is desired

Rules:

- `group_key` is the canonical identity used by code and persistence.
- `display_name` is presentation only.
- Admin "rename" changes `display_name`, not `group_key`.

### Alias Compatibility

Add `pricing_group_aliases`:

- `id` bigint primary key
- `alias_key` varchar unique
- `group_id` bigint foreign key to `pricing_groups.id`
- `created_at`
- optional `reason`

Purpose:

- Preserve compatibility with historical string values during migration.
- Resolve known legacy spellings such as `cc-oups4.6-福利渠道`.
- Support staged rollout where old workers or stale records may still emit
  legacy strings temporarily.

Aliases are transitional compatibility data, not a permanent substitute for
clean references.

## Reference Model Changes

Every domain record that currently stores a mutable group string should move to
stable references.

### Users

Replace or supplement `users.group` with:

- `users.group_key`

During migration, keep `users.group` as a legacy column until cutover
completes.

### Tokens

Replace or supplement `tokens.group` with:

- `tokens.group_key`
- `tokens.selection_mode`

`selection_mode` values:

- `inherit_user_default`
- `fixed`
- `auto`

This is preferable to encoding `auto` as a fake group name. `group_key` is only
meaningful when `selection_mode = fixed`.

### Subscription Plans

Replace or supplement `subscription_plans.upgrade_group` with:

- `subscription_plans.upgrade_group_key`

### User Subscriptions

Replace or supplement `user_subscriptions.upgrade_group` with:

- `upgrade_group_key_snapshot`
- `upgrade_group_name_snapshot`

Reasoning:

- The key snapshot preserves the purchased entitlement identity.
- The name snapshot preserves historical display context for admin timelines and
  receipts.

Runtime upgrade-group resolution should always prefer the key snapshot. The name
snapshot is informational only.

### Rules and Overrides

The current option JSON should be normalized where possible:

- `GroupRatio` becomes data in `pricing_groups`
- `UserUsableGroups` becomes data in `pricing_groups`
- `GroupGroupRatio` moves to `pricing_group_ratio_overrides`
- `group_ratio_setting.group_special_usable_group` moves to
  `pricing_group_visibility_rules`
- `AutoGroups` moves to `pricing_group_auto_priorities`

Suggested normalized tables:

#### `pricing_group_ratio_overrides`

- `id`
- `source_group_id`
- `target_group_id`
- `ratio`
- unique `(source_group_id, target_group_id)`

#### `pricing_group_visibility_rules`

- `id`
- `subject_group_id`
- `action` enum: `add`, `remove`
- `target_group_id`
- optional `description_override`
- `sort_order`

#### `pricing_group_auto_priorities`

- `id`
- `group_id`
- `priority`
- unique `priority`

This removes group relationship rules from opaque JSON blobs and makes them
validatable with foreign keys.

## Runtime Resolution Rules

### Group Lookup

Introduce a single resolver for legacy and canonical keys:

1. Resolve exact `group_key`
2. If not found, resolve by `pricing_group_aliases.alias_key`
3. If still not found, treat as invalid and surface an explicit consistency
   error

This resolver should be shared by:

- token auth
- user group loading
- subscription upgrade-group loading
- admin validation
- migration scripts

### Token Execution

Runtime token handling should use:

- `selection_mode = inherit_user_default`: use the current user `group_key`
- `selection_mode = fixed`: use `tokens.group_key`
- `selection_mode = auto`: use `pricing_group_auto_priorities`

This removes the current ambiguity where an empty string or `"auto"` can both
carry control-flow semantics.

### Subscription Upgrade Groups

Subscription entitlement loading should prefer:

1. valid `upgrade_group_key_snapshot`
2. valid `subscription_plans.upgrade_group_key`
3. alias resolution only during migration

The current fallback behavior can remain temporarily, but it should be treated
as migration compatibility, not permanent business logic.

## Admin UX Changes

The admin UI must stop implying that editing the visible text is a harmless
operation on the existing JSON object.

Required changes:

- "Edit group" may update:
  - `display_name`
  - `billing_ratio`
  - `user_selectable`
  - `description`
  - `status`
- `group_key` is immutable after creation
- "Delete group" becomes "archive group" by default
- Hard delete is only allowed when there are zero references
- Introduce an explicit "merge/migrate group" action

The merge/migrate action should:

1. Choose source group and target group
2. Preview impacted rows by table
3. Execute inside a transaction where possible
4. Record an audit entry
5. Add alias from old key to new key for the compatibility window

## Migration Plan

### Phase 0: Preflight Audit

Build a read-only audit command that:

- scans all known tables and option JSON blobs
- lists every distinct group string
- marks whether it resolves to a canonical group, alias, or nothing
- produces counts by table and key

This audit is the gate before any write migration.

### Phase 1: Schema Introduction

- add `pricing_groups`
- add `pricing_group_aliases`
- add normalized rule tables
- add new reference columns to existing tables
- keep old columns and old option readers intact

### Phase 2: Seed Canonical Data

- create canonical rows from current effective config
- seed alias records for known legacy spellings
- materialize normalized rules from current options

### Phase 3: Dual Read / Dual Write

Update application code so that:

- reads prefer new tables and new columns
- unresolved legacy values go through alias resolution
- writes update both new and legacy storage during the transition

This phase reduces deployment risk and allows rollback.

### Phase 4: Backfill Business Records

Backfill, in order:

1. `subscription_plans`
2. `user_subscriptions`
3. `users`
4. `tokens`
5. other rule and override tables

At the end of backfill, every live reference must point to a canonical
`group_key`.

### Phase 5: Consistency Enforcement

Add checks that fail loudly when:

- a business row references an unknown group
- an alias points to a missing group
- a normalized rule references an archived or missing group
- legacy and new columns disagree after the dual-write period

### Phase 6: Cutover

- stop reading old option JSON for group rules
- stop reading old string group columns
- retain old columns for one release as rollback support if needed
- after one stable release cycle, remove obsolete columns and compatibility
  branches in a dedicated cleanup migration

## Rollback Strategy

The migration must support rollback at the application layer during the dual
read/write window.

Safe rollback requirements:

- old columns remain populated until cutover stability is proven
- new writes continue syncing legacy columns during the transition
- alias resolution remains active until rollback is no longer needed
- backfill jobs are idempotent

After final cutover and cleanup, rollback becomes a database restore operation,
not a simple application rollback. That is acceptable only after multiple clean
deploy cycles and audit passes.

## Data Integrity Rules

The final state should enforce:

- unique `group_key`
- unique alias per canonical group mapping
- foreign keys from business tables to `pricing_groups`
- no hard deletes of referenced groups
- no direct edit of `group_key`
- no active token with `selection_mode = fixed` and null `group_key`
- no active subscription plan with invalid `upgrade_group_key`

## Observability and Verification

Add a consistency verification command or admin diagnostic endpoint that reports:

- canonical group count
- alias count
- unresolved legacy values by table
- archived groups still referenced by active entities
- dual-write mismatches during migration

Verification after rollout should include:

- token creation and token auth
- auto group routing
- fixed group routing
- pricing page group visibility
- subscription purchase and entitlement loading
- admin rename, archive, and merge flows

## Risks

- Dual-write bugs can create silent divergence if verification is weak.
- Rule normalization touches runtime-critical pricing and routing code.
- `auto` semantics are currently mixed with real group values and must be
  disentangled carefully.
- Existing code paths may still parse option JSON directly unless all entry
  points are audited.

## Recommendation

Implement this as a staged architecture migration, not a one-off repair.

The minimum acceptable long-term endpoint is:

- stable `group_key`
- editable `display_name`
- normalized references for live business data
- explicit admin merge/archive operations
- alias-based compatibility during migration

Anything less will reduce the immediate pain but leave Hermestoken vulnerable to
the same class of data-integrity failures whenever a group is renamed again.
