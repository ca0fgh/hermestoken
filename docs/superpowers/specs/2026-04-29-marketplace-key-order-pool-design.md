# Marketplace Key Order Pool Design

## Summary

HermesToken will add a marketplace where any user can escrow an AI API key as a seller. The platform keeps the raw key hidden, validates and monitors it, then exposes the key through two buyer-facing entry points:

- Order list: buyers purchase a fixed-route quota order bound to one seller key.
- Order pool: buyers submit marketplace requests and the platform routes each request to a suitable seller key.

Seller configuration is the source of buyer filtering and pricing. Buyers do not receive the seller API key, do not create their own seller-side terms, and do not get exclusive ownership of a key.

## Goals

- Let sellers escrow one AI API key and configure its sellable attributes once.
- Automatically make eligible escrowed keys visible in both the order list and order pool.
- Let buyers filter by seller-configured attributes such as model vendor, model, limit mode, multiplier, latency, success rate, and health.
- Keep fixed-route orders and pool routing separate at the buyer experience level while sharing the same seller credential state.
- Charge buyers from platform-controlled balances or fixed-order quota, not by exposing seller keys.
- Settle seller income from actual successful usage, with platform fee and frozen income states.

## Non-Goals

- Buyers do not receive raw seller API keys.
- Fixed-route orders are not exclusive key ownership.
- A buyer purchasing a fixed-route order does not remove the seller key from the order list or order pool.
- Marketplace seller keys do not enter the existing normal `Channel`, `Ability`, or `auto` routing pools by default.
- Marketplace settlement does not reuse ordinary usage logs as the financial ledger.

## Hard Invariants

- Seller escrow configuration is a filter and pricing source, not a buyer-editable package template.
- Fixed-route orders snapshot the purchase-time pricing, expiry, and quota terms.
- Order-pool usage and other buyers' fixed-route purchases must not reduce or cancel an existing fixed-route order.
- Seller credential quota exhaustion can stop new fixed-route purchases and order-pool routing, but it must not retroactively consume a fixed-route order's remaining quota.
- A fixed-route order can fail to serve only when its own quota/expiry is invalid, the bound credential is disabled, the bound credential is technically unavailable, or platform risk controls block it.
- Seller edits after purchase affect new order-list display and pool routing; existing fixed-route orders keep their snapshots.
- Seller unlisting stops new purchases and pool routing, but it should not stop already purchased fixed-route orders unless the key is disabled, technically unavailable, or risk-paused.
- Seller disabling stops new purchases, pool routing, and fixed-route serving through that credential until it is enabled again. It does not consume, reduce, or cancel existing fixed-route order quota.
- Seller credential deletion must be blocked while active fixed-route orders exist, or converted into an unlisted no-new-sales state until those orders end.
- Seller income is created from actual successful usage, not from unused fixed-route quota.
- Fixed-route purchase affects seller credential state as sold exposure, not as inventory removal.
- Successful fixed-route and pool calls affect seller credential usage state.
- Every credential state/configuration change must be recorded in that credential's history, including seller actions, admin actions, system health/capacity changes, and buyer-caused usage or order changes.

## Actors

- Seller: any existing user who escrows an AI API key.
- Buyer: any existing user who buys fixed-route quota or uses the order pool.
- Admin: operator who can unlist, disable, review risk, freeze settlements, and manage withdrawals.
- Platform: stores encrypted credentials, proxies requests, calculates official-price-based charges, and settles income.

## Seller Escrow Flow

The seller creates one escrow record with:

- Model vendor, selected from the project's existing create-channel type options.
- API key, stored encrypted and never returned in plaintext.
- Supported models.
- Limit mode: unlimited or limited.
- Multiplier used for buyer-facing price display and actual charge.
- Concurrency limit.

Vendor endpoint configuration should be system-controlled or allowlisted. The MVP should not let sellers submit arbitrary base URLs.

The marketplace should reuse the project's existing create-channel type options for model vendor selection instead of inventing marketplace-only vendor identifiers. Each supported vendor type still needs marketplace-specific validation, key handling, pricing compatibility, and safe endpoint policy before it is enabled.

After submission:

1. The platform validates basic input.
2. The API key is encrypted and stored.
3. The platform tests the key against selected model vendor and model rules.
4. A dynamic stats record is initialized.
5. If healthy, the key becomes visible in the order list and eligible for order pool routing.

The seller does not configure separate switches for order-list display or pool participation. Those surfaces are derived from the same escrow configuration and runtime status.

After escrow, seller lifecycle operations are:

- List: make a healthy credential available for new fixed-route purchases and pool routing.
- Unlist: stop new fixed-route purchases and pool routing without breaking existing fixed-route orders.
- Enable: allow the credential to serve marketplace requests when other eligibility checks pass.
- Disable: stop new purchases, pool routing, and fixed-route serving through this credential until re-enabled.
- Edit: update supported models, quota mode, multiplier, concurrency, or replace the API key after validation.

Each lifecycle operation must write a credential history event.

Seller-escrowed API keys do not have seller-configured expiry time. Expiry exists on fixed-route orders, not on the seller credential itself.

### Seller Terms Semantics

Seller-configured terms are buyer filters, pricing inputs, and market eligibility controls. They are not inventory buckets.

Quota mode:

- `unlimited` means the platform does not apply a seller-declared marketplace quota cap.
- `limited` means successful marketplace usage increments the credential's used quota against the seller-declared cap.
- Fixed-route purchases increase sold exposure and active order count, but they do not subtract from a sellable inventory field.
- When the seller-declared cap is exhausted, the credential stops accepting new fixed-route purchases and pool routing. Existing fixed-route orders keep their own remaining quota and can continue while the credential is enabled, technically usable, and not risk-paused.

This keeps the user's "no inventory" rule intact while still letting order-list purchases and pool usage dynamically affect seller credential status, sorting, risk, and new-sale eligibility.

## Buyer Entry Points

### Order List

The order list is fixed-route buying. Buyers filter escrowed keys by seller-configured and runtime attributes:

- Model.
- Limited or unlimited quota.
- Multiplier range.
- Effective price derived from official price times multiplier.
- Average latency.
- Success rate.
- Health state.

When a buyer purchases a fixed-route quota order, the order stores snapshots:

- Buyer and seller user IDs.
- Credential ID.
- Purchased quota.
- Remaining quota.
- Effective expiry.
- Multiplier snapshot.
- Official price snapshot.
- Buyer price snapshot.
- Platform fee rate snapshot.
- Status.

Calls using this order always route to the bound seller credential and deduct from the fixed order's remaining quota.

The buyer selects the purchased quota amount at purchase time. The fixed-order expiry is generated by marketplace fixed-order policy, not by a seller credential expiry. Seller-configured attributes decide whether the key appears in the filtered result set and how usage is priced. Platform-level guardrails, not per-key seller fields, should define global minimum purchase amount, maximum purchase amount, expiry policy, and abuse limits.

### Order Pool

The order pool is automatic routing. Buyers choose a marketplace model and optional filtering preferences. The router selects from eligible seller credentials using:

- Model match.
- Listing status.
- Service status.
- Health status.
- Risk status.
- Capacity status.
- Seller limit status.
- Current concurrency.
- Multiplier.
- Success rate.
- Average latency.

Pool calls deduct from the buyer's normal platform quota balance.

## Fixed-Route Order Definition

A fixed-route order is a quota package bound to one seller credential. It is not exclusive ownership of the credential.

A fixed-route purchase:

- Does not remove the key from the order list.
- Does not remove the key from the order pool.
- Does not prevent other buyers from purchasing fixed-route orders for the same key.
- Does not change the seller's escrow configuration.
- Does not expose the raw API key.

An existing fixed-route order is affected only by:

- Its own remaining quota.
- Its own expiry.
- The bound credential being disabled.
- The bound credential becoming technically unavailable.
- Platform risk controls.
- Upstream provider outage or refusal.

Other buyers' fixed-route purchases and order-pool usage do not cancel or reduce an already purchased fixed-route order.

## Dynamic Credential State

Each escrowed key has runtime state updated by both fixed-route calls and pool calls:

- Current concurrency.
- Total request count.
- Pool request count.
- Fixed-route request count.
- Total official cost.
- Used seller quota.
- Fixed-route sold quota.
- Active fixed-route order count.
- Success count.
- Upstream error count.
- Timeout count.
- Rate-limit count.
- Platform error count.
- Average latency.
- Last success time.
- Last failed time.
- Last failed reason.

External buyer-facing display should show success rate, not both success rate and error rate. Internal risk logic can derive error rates from typed error counters.

Suggested status dimensions:

- Listing status: listed or unlisted.
- Service status: enabled or disabled.
- Health status: healthy, degraded, or failed.
- Capacity status: available, busy, or exhausted.
- Risk status: normal, watching, or risk_paused.

Runtime state affects:

- Order pool eligibility.
- Order pool routing weight.
- Order list sorting.
- Whether new purchases are allowed.
- Whether the key is shown as busy, degraded, exhausted, unlisted, disabled, or risk-paused.

Runtime state should not retroactively cancel valid fixed-route orders unless the key is disabled, technically unavailable, or risk-paused.

Seller-configured quota limits and unlisting affect market availability for new purchases and pool routing. Existing fixed-route orders use their own snapshots and are not consumed by pool usage. If the seller key later becomes disabled, invalid, rate-limited, revoked, or blocked by risk controls, fixed-route calls may fail only while the platform cannot or must not proxy through that key.

## Credential Lifecycle Matrix

Seller and admin actions update explicit status dimensions. Buyer activity updates usage, exposure, health, capacity, and settlement state. Buyer activity must never rewrite the seller's configured terms such as model vendor, models, quota mode, multiplier, or concurrency limit.

| Credential state | Order-list display | New fixed-route purchase | Order-pool routing | Existing fixed-route calls |
| --- | --- | --- | --- | --- |
| listed, enabled, healthy or degraded, not exhausted, risk normal or watching | Visible | Allowed | Allowed if concurrency is available | Allowed |
| unlisted | Hidden | Blocked | Blocked | Allowed while enabled, technically usable, and not risk-paused |
| disabled | Hidden | Blocked | Blocked | Blocked until enabled; fixed-order quota remains unchanged |
| failed or technically unavailable | Hidden or shown as unavailable | Blocked | Blocked | Blocked while unavailable; fixed-order quota remains unchanged |
| busy due to concurrency | Visible as busy or lower ranked | Allowed unless capacity is exhausted | Blocked or deprioritized until capacity returns | Allowed when a slot is available, with fixed-route priority over pool calls |
| exhausted by seller-declared marketplace quota | Visible as exhausted or hidden by default | Blocked | Blocked | Allowed for already purchased fixed-route orders if other checks pass |
| risk_paused | Hidden | Blocked | Blocked | Blocked until risk is resumed; related settlements may remain pending or blocked |

Important consequences:

- Unlisting is a market visibility action, not a service shutdown.
- Disabling is a service shutdown for marketplace usage, but it does not refund, consume, or cancel fixed-route quota by itself.
- Seller-declared quota exhaustion stops new market demand and pool routing. It does not subtract from fixed-route orders already purchased.
- Health, capacity, and risk are runtime or operator states. They can temporarily prevent serving through the credential without changing the buyer's purchased quota balance.
- When a blocked state is resolved, existing fixed-route orders continue with their remaining quota and original expiry.

## Credential History

Each seller-escrowed AI API key needs a credential-level history timeline. The history is the audit trail for why the key's market state changed and which user or system action caused it.

History must record:

- Seller actions: list, unlist, enable, disable, edit configuration, replace key, test key.
- Admin actions: force unlist, force disable, risk pause, risk resume, settlement block or release if tied to the credential.
- System actions: validation result, health status change, capacity status change, quota exhausted, quota recovered, stats aggregation changes that affect eligibility.
- Buyer-caused actions: fixed-route order purchase, fixed-route usage, order-pool usage, fixed-route order exhaustion, fixed-route order expiry, refund/reversal exceptions.

Buyer-caused history should be visible under the seller credential without exposing buyer private data. Seller views can show event type, order/fill reference, model, quota delta, status impact, and timestamp. Admin views can include buyer user IDs and reconciliation details.

High-volume call details still live in `marketplace_fixed_order_fills` and `marketplace_pool_fills`. The credential history should reference those rows and record the state delta that matters to the credential, such as used quota, sold exposure, active order count, health status, capacity status, or service eligibility.

## Credential Event Write Policy

Credential events are the product-facing audit layer. They must be written in the same transaction as the state, stats, order, fill, or settlement mutation that caused the event. If the mutation commits and the event does not, the system loses explainability for seller-visible status changes.

Required event writes:

| Trigger | Event type | Source | Required delta |
| --- | --- | --- | --- |
| Seller lists or unlists a credential | `credential_listed`, `credential_unlisted` | seller | `listing_status`, reason |
| Seller enables or disables a credential | `credential_enabled`, `credential_disabled` | seller | `service_status`, reason |
| Seller edits terms | `credential_edited` | seller | changed fields, old and new safe snapshots |
| Seller replaces key | `credential_key_replaced` | seller | old and new key fingerprint snapshots, validation result |
| Seller or system tests key | `credential_tested` | seller or system | health result, model tested, failure class |
| Admin market or service action | credential lifecycle event, `risk_paused`, `risk_resumed` | admin | status changed, admin reason |
| Fixed-route order purchase | `fixed_order_purchased` | buyer | purchased quota, sold exposure delta, active order count delta |
| Fixed-route successful usage | `fixed_order_used` | buyer | fill id, charged quota, remaining fixed-order quota, credential quota used delta |
| Fixed-route exhaustion, expiry, refund exception | `fixed_order_exhausted`, `fixed_order_expired`, `fixed_order_refunded` | system, buyer, admin, or reconciliation | order status delta, expired or refunded quota |
| Pool successful usage | `pool_used` | buyer | fill id, charged quota, credential quota used delta |
| Runtime eligibility changes | `health_changed`, `capacity_changed`, `quota_exhausted`, `quota_recovered` | system or reconciliation | old and new status, computed reason |
| Settlement review action | `settlement_blocked`, `settlement_released` | admin, system, or reconciliation | settlement id, old and new settlement status |

Seller-visible event payloads must be sanitized:

- Allowed: event type, event source category, source reference, model, quota delta, price or settlement delta, safe old/new status, safe changed-field names, timestamp, non-sensitive reason.
- Not allowed: raw API key, request body, response body, buyer private data, buyer token, upstream authorization headers, raw provider error bodies, unmasked sensitive URLs.
- Buyer user IDs can be stored for admin and reconciliation views, but seller APIs must not return them.
- Key replacement events can show fingerprints and validation results, never plaintext key values.

Usage events may be paged, summarized, or compacted for seller UI performance, but the underlying fill and settlement records must remain the source of financial truth.

## Visibility And Permissions

Seller views:

- Can see only credentials owned by the seller.
- Can see masked key fingerprint, model vendor display name, configured models, quota mode, multiplier, concurrency limit, listing status, service status, health, capacity, risk label, aggregate usage, fixed-route sold exposure, active order count, and seller income states.
- Can see seller-visible credential events, including buyer-caused effects, without buyer identity or request content.
- Can list, unlist, enable, disable, edit, test, and replace their own credential subject to validation and risk rules.

Buyer views:

- Can see order-list and pool candidate fields needed for purchase or routing decisions: model vendor, model, limit mode, multiplier, effective price, success rate, latency, health label, capacity label, and fixed-route expiry policy.
- Can see their own fixed-route orders, remaining quota, expiry, status, fills, and charges.
- Cannot see raw seller API keys, seller-side encrypted values, other buyers' orders, or credential history internals.

Admin views:

- Can see all marketplace credentials, events, fixed orders, fills, settlements, and reconciliation states.
- Can see buyer and seller user IDs where needed for audit.
- Must still never receive plaintext seller API keys through normal admin APIs.

## Pricing

Marketplace pricing is based on official provider pricing multiplied by the seller multiplier.

```text
buyer_price = official_price * seller_multiplier
```

All buyer-facing debits, fixed-order quotas, and seller settlements should use the platform's existing quota unit. Official provider costs should also keep a normalized price snapshot, including the original official unit such as USD per token or USD per request, so audit and display can explain how the platform quota charge was derived.

Each purchase and call should snapshot:

- Official price version.
- Official price values.
- Seller multiplier.
- Buyer price values.

This makes historical settlement explainable even when official prices change later.

## Billing And Settlement

Fixed-route order purchase:

- Buyer prepays quota from the buyer's platform balance into the fixed order.
- The prepaid quota remains attached to that fixed order.
- Seller income is released by actual usage, not immediately at purchase.
- Unused fixed-route quota is not seller income. When a fixed-route order expires, unused quota expires and is not refunded.
- Expired unused quota must be recorded as expired fixed-order quota for audit. It is platform-retained expired balance, not seller income.

Fixed-route call:

```text
official_cost = usage priced by official price snapshot
buyer_charge = official_cost * multiplier_snapshot
fixed_order.remaining_quota -= buyer_charge
platform_fee = buyer_charge * fixed_order.platform_fee_rate_snapshot
seller_income = buyer_charge - platform_fee
```

Platform fee is globally configurable for the marketplace. The global fee rate is applied consistently to fixed-route and pool settlements.

Fixed-route orders snapshot the global marketplace fee rate at purchase time. Fixed-route fills use the order's fee snapshot so seller payout terms do not change after purchase. Pool fills use the current global marketplace fee rate at request time and snapshot it onto the settlement row.

Pool call:

```text
official_cost = usage priced by official pricing
buyer_charge = official_cost * credential.multiplier
buyer user.quota -= buyer_charge
platform_fee = buyer_charge * current_global_fee_rate
seller_income = buyer_charge - platform_fee
```

Billing mutations must be idempotent and atomic per request:

- Deduct fixed-order quota or user quota.
- Write the usage record.
- Write the settlement record.
- Update credential stats.

If one step fails after upstream success, the system must record a recoverable reconciliation state rather than silently dropping seller income or double-charging the buyer.

Seller income state:

```text
pending -> available -> withdrawn
pending -> blocked -> available
pending -> blocked -> reversed
```

The pending window protects against provider failures, abuse, refunds, and risk review.

## Data Model

### marketplace_credentials

Seller escrowed key and seller-configured attributes.

```text
id
seller_user_id
vendor_type
vendor_name_snapshot
encrypted_api_key
key_fingerprint
models
quota_mode
quota_limit
multiplier
concurrency_limit
listing_status
service_status
health_status
capacity_status
risk_status
created_at
updated_at
```

### marketplace_credential_stats

High-churn runtime stats separated from credential configuration.

```text
credential_id
current_concurrency
total_request_count
pool_request_count
fixed_order_request_count
total_official_cost
quota_used
fixed_order_sold_quota
active_fixed_order_count
success_count
upstream_error_count
timeout_count
rate_limit_count
platform_error_count
avg_latency_ms
last_success_at
last_failed_at
last_failed_reason
updated_at
```

### marketplace_credential_events

Credential-level history for seller, buyer, admin, and system-caused state changes.

```text
id
credential_id
event_type
event_source
actor_user_id
buyer_user_id
source_type
source_id
old_state_snapshot
new_state_snapshot
delta_snapshot
changed_fields
reason
seller_visible
admin_visible
created_at
```

Event source values:

```text
seller
buyer
admin
system
reconciliation
```

Event types should include at least:

```text
credential_listed
credential_unlisted
credential_enabled
credential_disabled
credential_edited
credential_key_replaced
credential_tested
health_changed
capacity_changed
quota_exhausted
quota_recovered
fixed_order_purchased
fixed_order_used
fixed_order_exhausted
fixed_order_expired
fixed_order_refunded
pool_used
risk_paused
risk_resumed
settlement_blocked
settlement_released
```

### marketplace_fixed_orders

Buyer fixed-route quota orders.

```text
id
buyer_user_id
seller_user_id
credential_id
purchased_quota
remaining_quota
spent_quota
expired_quota
multiplier_snapshot
official_price_snapshot
buyer_price_snapshot
platform_fee_rate_snapshot
expires_at
status
created_at
updated_at
```

### marketplace_fixed_order_fills

Fixed-route call records.

```text
id
request_id
fixed_order_id
buyer_user_id
seller_user_id
credential_id
model
official_cost
multiplier_snapshot
buyer_charge
status
latency_ms
error_code
created_at
```

### marketplace_pool_fills

Order-pool routed call records.

```text
id
request_id
buyer_user_id
seller_user_id
credential_id
model
official_cost
multiplier_snapshot
buyer_charge
status
latency_ms
error_code
created_at
```

### marketplace_settlements

Financial ledger for seller income and platform fee.

```text
id
request_id
buyer_user_id
seller_user_id
credential_id
source_type
source_id
buyer_charge
platform_fee
platform_fee_rate_snapshot
seller_income
official_cost
multiplier_snapshot
status
available_at
created_at
updated_at
```

`request_id` or a source-specific idempotency key must be unique to prevent duplicate settlement.

Fixed-order expiry should also write an auditable balance event, either in a dedicated expiry table or as a non-settlement ledger row. It must not create seller income.

## API Surfaces

### Seller

```text
POST   /api/marketplace/seller/credentials
GET    /api/marketplace/seller/credentials
GET    /api/marketplace/seller/credentials/:id
PUT    /api/marketplace/seller/credentials/:id
POST   /api/marketplace/seller/credentials/:id/test
POST   /api/marketplace/seller/credentials/:id/list
POST   /api/marketplace/seller/credentials/:id/unlist
POST   /api/marketplace/seller/credentials/:id/enable
POST   /api/marketplace/seller/credentials/:id/disable
GET    /api/marketplace/seller/credentials/:id/events
GET    /api/marketplace/seller/income
GET    /api/marketplace/seller/settlements
```

Sellers can replace API keys but cannot read plaintext keys back.

### Buyer

```text
GET  /api/marketplace/orders
POST /api/marketplace/fixed-orders
GET  /api/marketplace/fixed-orders
GET  /api/marketplace/fixed-orders/:id
GET  /api/marketplace/pool/models
GET  /api/marketplace/pool/candidates
```

Marketplace relay entry points:

```text
POST /marketplace/v1/chat/completions
POST /marketplace/v1/responses
```

Fixed-route calls can identify the order with:

```text
X-Marketplace-Fixed-Order-Id: <id>
```

Without a fixed-order ID, the request uses order-pool routing.

### Admin

```text
GET  /api/marketplace/admin/credentials
GET  /api/marketplace/admin/credentials/:id/events
POST /api/marketplace/admin/credentials/:id/unlist
POST /api/marketplace/admin/credentials/:id/disable
POST /api/marketplace/admin/credentials/:id/enable
POST /api/marketplace/admin/credentials/:id/risk-pause
POST /api/marketplace/admin/credentials/:id/risk-resume
GET  /api/marketplace/admin/fixed-orders
GET  /api/marketplace/admin/fixed-order-fills
GET  /api/marketplace/admin/pool-fills
GET  /api/marketplace/admin/settlements
POST /api/marketplace/admin/settlements/:id/block
POST /api/marketplace/admin/settlements/:id/release
```

## Product Flow Contracts

### Seller Creates A Credential

1. Seller selects model vendor from the existing create-channel type list.
2. Seller enters API key, supported models, quota mode, optional quota cap, multiplier, and concurrency limit.
3. Platform validates marketplace support for the selected model vendor and models.
4. Platform encrypts the API key, stores a keyed fingerprint, and tests the credential.
5. Platform creates credential stats and writes initial history events.
6. A healthy listed and enabled credential becomes available to the order list and order pool through derived eligibility rules.

### Seller Changes Credential State

- List: exposes the credential for new fixed-route purchases and pool routing when other eligibility checks pass.
- Unlist: removes the credential from new purchases and pool routing, but existing fixed-route orders can continue.
- Enable: allows the credential to serve marketplace traffic again when health, capacity, and risk checks pass.
- Disable: stops all marketplace traffic through the credential until re-enabled. Existing fixed-route order balances do not change.
- Edit: changes future market display, filtering, and pool eligibility. Existing fixed-route orders keep purchase-time snapshots.
- Replace key: validates the replacement before it can serve traffic. A failed replacement does not break the currently serving key.

Every action writes a credential event and updates only the state dimension or configuration fields that the action owns.

### Buyer Purchases A Fixed-Route Order

1. Buyer filters the order list using seller-configured terms and runtime health fields.
2. Buyer chooses a quota amount within global marketplace guardrails.
3. Platform verifies the credential is eligible for new purchase.
4. Platform snapshots pricing, multiplier, fee rate, expiry policy, seller ID, buyer ID, credential ID, and model/vendor metadata.
5. Platform deducts the buyer's platform quota into the fixed order.
6. Platform increments seller credential sold exposure and active fixed-order count.
7. Platform writes `fixed_order_purchased` to the credential history.

The purchase does not remove the credential from the market, does not reduce other buyers' fixed-route orders, and does not grant raw key access.

### Buyer Uses A Fixed-Route Order

1. Buyer calls the marketplace relay with `X-Marketplace-Fixed-Order-Id`.
2. Platform verifies buyer ownership, fixed-order status, remaining quota, expiry, and bound credential service eligibility.
3. Platform proxies through the encrypted seller credential.
4. Platform prices actual usage from the fixed order's snapshots.
5. Platform deducts only the fixed order's remaining quota.
6. Platform writes fill, settlement, stats, and credential history records.
7. If the fixed order is exhausted or expired, the order status changes and the credential history records why.

The buyer's normal quota balance is not charged again for the same fixed-route usage.

### Buyer Uses The Order Pool

1. Buyer calls a marketplace relay endpoint without a fixed-order ID.
2. Platform authenticates the buyer token and checks buyer quota balance.
3. Router filters eligible credentials by model vendor, model, listing, service, health, risk, capacity, seller quota, endpoint policy, and concurrency.
4. Router scores candidates by effective price, latency, success rate, fairness, and load.
5. Platform proxies through the selected encrypted seller credential.
6. Platform prices actual usage from current official pricing, seller multiplier, and current global marketplace fee rate.
7. Platform deducts the buyer's normal platform quota and writes fill, settlement, stats, and credential history records.

Pool usage may change seller credential usage, capacity, health, and quota state. It must not consume or alter any buyer's fixed-route order balance.

## Existing System Integration

Reuse:

- Existing users for buyer and seller identity.
- Existing token authentication for buyer marketplace calls.
- Existing user quota for pool-call buyer balance.
- Existing top-up flows.
- Existing withdrawal flows, extended with marketplace income source.
- Existing official pricing data where suitable.
- Existing upstream adapter code where it can be safely reused without registering marketplace credentials as normal channels.
- Existing create-channel type definitions for marketplace model vendor selection, with marketplace-specific validation before enablement.

Keep separate:

- Marketplace credentials.
- Fixed-route orders.
- Pool routing.
- Marketplace request records.
- Marketplace financial ledger.
- Marketplace risk state.

Do not place marketplace keys into the normal `channels`, `abilities`, or `auto` route unless a future design explicitly adds a safe bridge.

## Implementation Blueprint

### Module Boundaries

The marketplace should be implemented as a separate domain module, even though it reuses existing identity, quota, pricing, model vendor options, and withdrawal capabilities.

Suggested backend packages:

```text
model/marketplace_credential.go
model/marketplace_credential_event.go
model/marketplace_fixed_order.go
model/marketplace_fill.go
model/marketplace_settlement.go
model/marketplace_setting.go
service/marketplace_crypto.go
service/marketplace_credential_event.go
service/marketplace_pricing.go
service/marketplace_router.go
service/marketplace_settlement.go
controller/marketplace_seller.go
controller/marketplace_buyer.go
controller/marketplace_admin.go
controller/marketplace_relay.go
```

The marketplace relay can reuse existing provider adaptors, but it should build a marketplace-specific relay context instead of creating or mutating normal `Channel` records. This prevents seller credentials from leaking into existing channel selection, channel testing, ability sync, and auto-routing behavior.

The marketplace relay must not call the existing full relay billing path as-is. Fixed-route calls should not deduct the buyer's normal wallet quota, and pool calls should not enter normal channel billing or normal channel used-quota accounting. The safe reuse boundary is provider request/response adaptation, usage parsing, token counting, and official/base price lookup; marketplace quota deduction and seller settlement must stay in marketplace services.

### Global Settings

Use the existing option system for operator-configurable marketplace policy:

```text
MarketplaceEnabled
MarketplaceEnabledVendorTypes
MarketplaceFeeRate
MarketplaceSellerIncomeHoldSeconds
MarketplaceMinFixedOrderQuota
MarketplaceMaxFixedOrderQuota
MarketplaceFixedOrderDefaultExpirySeconds
MarketplaceMaxSellerMultiplier
MarketplaceMaxCredentialConcurrency
```

`MarketplaceFeeRate` is the global transaction fee. It should be snapshotted onto every settlement row.

`MarketplaceEnabledVendorTypes` should store the existing create-channel type values used by the model vendor selector. A vendor type appearing in this list only means the marketplace may accept that type; the implementation must still have explicit validation and relay support for it.

Credential encryption is mandatory and must not be feature-flagged off. The API-key encryption secret should not be stored in options. It must come from environment or secret manager configuration.

### Pricing Contract

Marketplace pricing is:

```text
official_cost_quota = official model usage converted to platform quota
buyer_charge = official_cost_quota * seller_multiplier
platform_fee = buyer_charge * global_fee_rate
seller_income = buyer_charge - platform_fee
```

The existing `ratio_setting`, `billing_setting`, `types.PriceData`, and `common.QuotaPerUnit` can be reused to obtain official model cost and unit conversion. The marketplace pricing helper must not accidentally apply the buyer's normal group ratio on top of seller multiplier. If an existing helper always includes group ratio, create a marketplace-specific helper that extracts official/base cost before buyer group pricing.

Marketplace token authentication should identify the buyer and enforce ownership. It should not double-charge through the normal token or wallet billing path. If token-level quota limits are later needed as an extra buyer-side spending control, they should be designed as a separate marketplace policy rather than inherited accidentally from normal relay billing.

Snapshots required on purchase and fill:

- Price source: price map, ratio map, or tiered expression.
- Price version or config hash.
- Official/base price fields.
- Seller multiplier.
- Global fee rate.
- Final buyer charge.

`vendor_type` is the canonical model-vendor identity in marketplace records. Any vendor name should be a display snapshot derived from the existing create-channel type names, not an independently editable source of truth.

### State Transitions

Credential state is represented by multiple dimensions instead of one overloaded status:

```text
listing_status: listed -> unlisted -> listed
service_status: enabled -> disabled -> enabled
health_status: untested -> healthy -> degraded -> failed -> healthy
capacity_status: available -> busy -> exhausted -> available
risk_status: normal -> watching -> risk_paused -> normal
```

Eligibility rules:

- Order-list display requires listed status, enabled service status, risk normal or watching, and supported models.
- New fixed-route purchase additionally requires health healthy or degraded and capacity not exhausted.
- Pool routing additionally requires current concurrency below limit.
- Existing fixed-route call can proceed when the fixed order is valid and the credential is enabled and technically usable, unless risk is `risk_paused`.

Seller key replacement should be staged: the existing encrypted key remains serving until the replacement key passes validation for the same model vendor, compatible models, and endpoint policy. A failed replacement must not break active fixed-route orders.

Fixed-route order state:

```text
active -> exhausted
active -> expired
active -> suspended
suspended -> active
suspended -> refunded
```

`expired` means unused quota is invalid and not refunded. `refunded` is only for operator or risk exception handling, not normal expiry.

Settlement state:

```text
pending -> available -> withdrawn
pending -> blocked -> available
pending -> blocked -> reversed
```

`blocked` and `reversed` are operator/risk states. They should not exist in the normal happy path, but they are needed for abuse, upstream disputes, and reconciliation.

### Transaction Boundaries

Fixed-route purchase must be atomic:

1. Lock buyer balance.
2. Verify buyer has enough quota.
3. Verify credential is eligible for new purchase.
4. Deduct buyer quota into fixed-order escrow.
5. Create fixed order with purchase and fee snapshots.
6. Increment seller credential fixed-route sold exposure.
7. Write credential event `fixed_order_purchased`.
8. Write buyer-facing balance log.

Fixed-route call settlement must be idempotent:

1. Validate buyer token and fixed-order ownership.
2. Lock fixed order and credential capacity.
3. Reserve estimated fixed-order quota if pre-consume is required.
4. Proxy through the encrypted seller credential.
5. Calculate actual official cost and buyer charge.
6. Adjust fixed-order remaining quota.
7. Create fill record.
8. Create settlement row with fee snapshot.
9. Update credential stats and used seller quota.
10. Write credential event `fixed_order_used` and any resulting capacity or quota status event.

Pool call settlement must be idempotent:

1. Validate buyer token.
2. Select and lock a credential candidate.
3. Reserve estimated buyer quota if pre-consume is required.
4. Proxy through the encrypted seller credential.
5. Calculate actual official cost and buyer charge.
6. Adjust buyer quota.
7. Create pool fill record.
8. Create settlement row with fee snapshot.
9. Update credential stats and used seller quota.
10. Write credential event `pool_used` and any resulting capacity or quota status event.

All fill and settlement writes need a unique request id or idempotency key. Retried client requests and retry-after-upstream-success paths must not double-charge buyers or double-create seller income.

### Pool Router

The router should use a two-step process: hard filtering first, scoring second.

Hard filters:

- Marketplace enabled.
- Model vendor type enabled for marketplace.
- Model supported.
- Relay mode supported by the model vendor type for the marketplace endpoint.
- Credential listed.
- Credential enabled.
- Risk not paused.
- Credential not exhausted.
- Current concurrency below limit.
- Vendor endpoint policy valid.

Suggested score:

```text
score =
  price_weight * normalized_low_multiplier +
  latency_weight * normalized_low_latency +
  success_weight * success_rate +
  fairness_weight * seller_fairness_score -
  load_penalty * current_concurrency_ratio
```

For MVP, fixed-route calls should have priority over pool calls when a credential is near its concurrency limit. This protects buyers who already prepaid fixed-route quota.

### Background Jobs

Required jobs:

- Credential health check: test active credentials and update health status.
- Credential history retention or compaction job if event volume grows too large.
- Fixed-order expiry check: mark expired orders, persist expired quota, and invalidate unused quota without refund.
- Settlement release: move pending seller income to available after the hold period.
- Stats aggregation: refresh success rate, latency, request counts, and failure counters.
- Reconciliation: repair requests that succeeded upstream but failed before local billing completed.

### UI Surfaces

Seller:

- Escrow key form: model vendor, models, limit mode, multiplier, concurrency.
- Seller credential list: masked key fingerprint, listing status, service status, models, multiplier, health, used quota, sold fixed-route exposure, active orders, pool usage, income.
- Seller credential detail: state-change history including seller actions, system changes, and buyer-caused order/usage effects.
- Seller income page: pending, available, withdrawn, blocked.

Buyer:

- Order list: filters based on seller configuration and runtime status.
- Fixed-order purchase modal: choose quota amount within global guardrails, show effective price snapshot, fee snapshot, expiry, and no-refund expiry policy before purchase.
- Fixed-order list: remaining quota, expiry, status, bound credential health.
- Pool page: model selection, optional filters, estimated effective price range.

Admin:

- Marketplace settings.
- Enabled model vendor allowlist.
- Credential risk review.
- Credential full history review.
- Fixed-order review.
- Fill and settlement audit.
- Reconciliation queue.

## Acceptance Criteria

- A seller can escrow one supported model-vendor API key and never see the plaintext key again.
- The same escrowed key appears in order-list and pool candidates when eligible.
- A buyer can buy fixed-route quota for part of a seller key without removing the key from the market.
- Fixed-route usage deducts only that order's remaining quota.
- Pool usage deducts the buyer's normal quota balance.
- Other buyers' usage never reduces an existing fixed-route order.
- Fixed-route unused quota expires without refund at order expiry.
- Fixed-route expired quota is auditable and does not become seller income.
- Seller income is created only after successful usage.
- Platform fee uses the global marketplace fee setting and is snapshotted per settlement.
- Buyer-facing price equals official/base cost times seller multiplier.
- Buyer-facing stats show success rate, not both success rate and error rate.
- Raw seller API keys are encrypted at rest, masked in logs, and never returned by APIs.
- Unsupported model vendor types are rejected even if they exist in the project's create-channel type selector.
- Seller quota limits stop new sales and pool routing after exhaustion, but they do not remove or reduce already purchased fixed-route quota.
- Seller credentials support list, unlist, enable, disable, and edit actions.
- Each seller credential has a state-change history that includes seller, admin, system, and buyer-caused events.

## Security Requirements

- Raw seller API keys must be encrypted at rest.
- The encryption key must come from environment or secret manager configuration.
- API key plaintext must never be returned by seller, buyer, or admin list APIs.
- Key fingerprints should use a keyed hash or other non-reversible fingerprinting scheme, not a raw hash that can be compared across deployments.
- Seller key replacement should not expose old key material.
- Seller key replacement for an active credential must revalidate model vendor, models, and endpoint policy before it can serve existing fixed-route orders.
- Marketplace calls must verify buyer token ownership and fixed-order ownership.
- Marketplace router must validate model vendor type and base URL against a safe provider policy.
- Marketplace router must reject any existing vendor type that has not been explicitly enabled for marketplace escrow.
- Custom, localhost, private-network, and seller-supplied arbitrary base URLs should be disabled for marketplace escrow until a separate SSRF and credential-exfiltration review explicitly enables them.
- Settlement writes must be idempotent.
- Logs must mask API keys and sensitive upstream URLs.
- Credential event history must not store raw API keys, request bodies, response bodies, or buyer private data in seller-visible fields.
- Admin access must be required for risk overrides and settlement blocking.

## Open Decisions Before Implementation

- Default `MarketplaceFeeRate` value.
- Initial `MarketplaceEnabledVendorTypes` and supported marketplace relay modes.
- Fixed-route order expiry policy: default duration and whether buyers can choose among platform-defined durations.
- Seller income hold period: exact default pending duration before income becomes available.
- Pool routing weights: exact weight values for lower multiplier, lower latency, higher success rate, fair seller distribution, and load penalty.
- Abuse limit values: global minimum/maximum fixed-route purchase amount and per-buyer/per-seller rate limits.

## MVP Phases

1. Seller credential escrow, encryption, validation, and seller list.
2. Order list display and fixed-route order purchase.
3. Fixed-route marketplace relay call and usage-based settlement.
4. Order pool candidate listing and router.
5. Pool relay call and usage-based settlement.
6. Seller income views and withdrawal integration.
7. Admin risk and settlement tooling.

The first usable slice should prove:

```text
seller escrow -> order list -> buyer fixed-route purchase -> marketplace relay call -> fixed-order quota deduction -> seller pending income
```
