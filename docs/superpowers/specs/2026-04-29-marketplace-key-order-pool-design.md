# Marketplace Key Order Pool Design

## Summary

HermesToken will add a marketplace where any user can escrow an AI API key as a seller. The platform keeps the raw key hidden, validates and monitors it, then exposes the key through two buyer-facing entry points:

- Order list: buyers purchase a fixed-route quota order bound to one seller key.
- Order pool: buyers submit marketplace requests and the platform routes each request to a suitable seller key.

Seller configuration is the source of buyer filtering and pricing. Buyers do not receive the seller API key, do not create their own seller-side terms, and do not get exclusive ownership of a key.

## Goals

- Let sellers escrow one AI API key and configure its sellable attributes once.
- Automatically make eligible escrowed keys visible in both the order list and order pool.
- Let buyers filter by seller-configured attributes such as model, limit mode, time mode, multiplier, latency, success rate, and health.
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
- Seller credential quota/time exhaustion can stop new fixed-route purchases and order-pool routing, but it must not retroactively consume a fixed-route order's remaining quota.
- A fixed-route order can fail to serve only when its own quota/expiry is invalid, the bound credential is technically unavailable, or platform risk controls block it.
- Seller edits after purchase affect new order-list display and pool routing; existing fixed-route orders keep their snapshots.
- Seller pause stops new purchases and pool routing, but it should not stop already purchased fixed-route orders unless the key is technically unavailable or risk-paused.
- Seller credential deletion must be blocked while active fixed-route orders exist, or converted into a no-new-sales state until those orders end.
- Seller income is created from actual successful usage, not from unused fixed-route quota.

## Actors

- Seller: any existing user who escrows an AI API key.
- Buyer: any existing user who buys fixed-route quota or uses the order pool.
- Admin: operator who can pause credentials, review risk, freeze settlements, and manage withdrawals.
- Platform: stores encrypted credentials, proxies requests, calculates official-price-based charges, and settles income.

## Seller Escrow Flow

The seller creates one escrow record with:

- Provider/channel type, selected from the project's existing channel types.
- API key, stored encrypted and never returned in plaintext.
- Supported models.
- Limit mode: unlimited or limited.
- Time mode: unlimited or limited.
- Multiplier used for buyer-facing price display and actual charge.
- Concurrency limit.

Provider endpoint configuration should be system-controlled or allowlisted. The MVP should not let sellers submit arbitrary base URLs.

The marketplace should support the project's existing channel types instead of inventing marketplace-only provider identifiers. Each supported channel type still needs marketplace-specific validation, key handling, pricing compatibility, and safe endpoint policy before it is enabled.

After submission:

1. The platform validates basic input.
2. The API key is encrypted and stored.
3. The platform tests the key against selected provider/model rules.
4. A dynamic stats record is initialized.
5. If healthy, the key becomes visible in the order list and eligible for order pool routing.

The seller does not configure separate switches for order-list display or pool participation. Those surfaces are derived from the same escrow configuration and runtime status.

## Buyer Entry Points

### Order List

The order list is fixed-route buying. Buyers filter escrowed keys by seller-configured and runtime attributes:

- Model.
- Limited or unlimited quota.
- Limited or unlimited time.
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
- Status.

Calls using this order always route to the bound seller credential and deduct from the fixed order's remaining quota.

The buyer selects the purchased quota amount at purchase time. Seller-configured attributes decide whether the key appears in the filtered result set and how usage is priced. Platform-level guardrails, not per-key seller fields, should define global minimum purchase amount, maximum purchase amount, and abuse limits.

### Order Pool

The order pool is automatic routing. Buyers choose a marketplace model and optional filtering preferences. The router selects from eligible seller credentials using:

- Model match.
- Seller status.
- Health status.
- Risk status.
- Capacity status.
- Expiry status.
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

- Seller status: active or paused.
- Health status: healthy, degraded, or failed.
- Capacity status: available, busy, exhausted, or expired.
- Risk status: normal, watching, or risk_paused.

Runtime state affects:

- Order pool eligibility.
- Order pool routing weight.
- Order list sorting.
- Whether new purchases are allowed.
- Whether the key is shown as busy, degraded, expired, or risk-paused.

Runtime state should not retroactively cancel valid fixed-route orders unless the key is technically unavailable or risk-paused.

Seller-configured quota and time limits affect market availability for new purchases and pool routing. Existing fixed-route orders use their own snapshots and are not consumed by pool usage. If the seller key later becomes invalid, rate-limited, revoked, or blocked by risk controls, fixed-route calls may fail because the platform can no longer proxy through that key.

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

Fixed-route call:

```text
official_cost = usage priced by official price snapshot
buyer_charge = official_cost * multiplier_snapshot
fixed_order.remaining_quota -= buyer_charge
seller_income = buyer_charge - platform_fee
```

Platform fee is globally configurable for the marketplace. The global fee rate is applied consistently to fixed-route and pool settlements.

Pool call:

```text
official_cost = usage priced by official pricing
buyer_charge = official_cost * credential.multiplier
buyer user.quota -= buyer_charge
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
```

The pending window protects against provider failures, abuse, refunds, and risk review.

## Data Model

### marketplace_credentials

Seller escrowed key and seller-configured attributes.

```text
id
seller_user_id
provider
channel_type
encrypted_api_key
key_fingerprint
models
quota_mode
quota_limit
time_mode
expires_at
multiplier
concurrency_limit
seller_status
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

### marketplace_fixed_orders

Buyer fixed-route quota orders.

```text
id
buyer_user_id
seller_user_id
credential_id
purchased_quota
remaining_quota
multiplier_snapshot
official_price_snapshot
buyer_price_snapshot
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
seller_income
official_cost
multiplier_snapshot
status
available_at
created_at
updated_at
```

`request_id` or a source-specific idempotency key must be unique to prevent duplicate settlement.

## API Surfaces

### Seller

```text
POST   /api/marketplace/seller/credentials
GET    /api/marketplace/seller/credentials
GET    /api/marketplace/seller/credentials/:id
PUT    /api/marketplace/seller/credentials/:id
POST   /api/marketplace/seller/credentials/:id/test
POST   /api/marketplace/seller/credentials/:id/pause
POST   /api/marketplace/seller/credentials/:id/resume
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
POST /api/marketplace/admin/credentials/:id/pause
POST /api/marketplace/admin/credentials/:id/risk-pause
POST /api/marketplace/admin/credentials/:id/resume
GET  /api/marketplace/admin/fixed-orders
GET  /api/marketplace/admin/fixed-order-fills
GET  /api/marketplace/admin/pool-fills
GET  /api/marketplace/admin/settlements
POST /api/marketplace/admin/settlements/:id/block
POST /api/marketplace/admin/settlements/:id/release
```

## Existing System Integration

Reuse:

- Existing users for buyer and seller identity.
- Existing token authentication for buyer marketplace calls.
- Existing user quota for pool-call buyer balance.
- Existing top-up flows.
- Existing withdrawal flows, extended with marketplace income source.
- Existing official pricing data where suitable.
- Existing upstream adapter code where it can be safely reused without registering marketplace credentials as normal channels.
- Existing channel type definitions for marketplace provider selection, with marketplace-specific validation before enablement.

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

The marketplace should be implemented as a separate domain module, even though it reuses existing identity, quota, pricing, channel type, and withdrawal capabilities.

Suggested backend packages:

```text
model/marketplace_credential.go
model/marketplace_fixed_order.go
model/marketplace_fill.go
model/marketplace_settlement.go
model/marketplace_setting.go
service/marketplace_crypto.go
service/marketplace_pricing.go
service/marketplace_router.go
service/marketplace_settlement.go
controller/marketplace_seller.go
controller/marketplace_buyer.go
controller/marketplace_admin.go
controller/marketplace_relay.go
```

The marketplace relay can reuse existing provider adaptors, but it should build a marketplace-specific relay context instead of creating or mutating normal `Channel` records. This prevents seller credentials from leaking into existing channel selection, channel testing, ability sync, and auto-routing behavior.

### Global Settings

Use the existing option system for operator-configurable marketplace policy:

```text
MarketplaceEnabled
MarketplaceEnabledChannelTypes
MarketplaceFeeRate
MarketplaceSellerIncomeHoldSeconds
MarketplaceMinFixedOrderQuota
MarketplaceMaxFixedOrderQuota
MarketplaceMaxSellerMultiplier
MarketplaceMaxCredentialConcurrency
MarketplaceCredentialEncryptionEnabled
```

`MarketplaceFeeRate` is the global transaction fee. It should be snapshotted onto every settlement row.

`MarketplaceEnabledChannelTypes` should store existing `constant.ChannelType*` integer values. A channel type appearing in this list only means the marketplace may accept that type; the implementation must still have explicit validation and relay support for it.

The API-key encryption secret should not be stored in options. It must come from environment or secret manager configuration.

### Pricing Contract

Marketplace pricing is:

```text
official_cost_quota = official model usage converted to platform quota
buyer_charge = official_cost_quota * seller_multiplier
platform_fee = buyer_charge * global_fee_rate
seller_income = buyer_charge - platform_fee
```

The existing `ratio_setting`, `billing_setting`, `types.PriceData`, and `common.QuotaPerUnit` can be reused to obtain official model cost and unit conversion. The marketplace pricing helper must not accidentally apply the buyer's normal group ratio on top of seller multiplier. If an existing helper always includes group ratio, create a marketplace-specific helper that extracts official/base cost before buyer group pricing.

Snapshots required on purchase and fill:

- Price source: price map, ratio map, or tiered expression.
- Price version or config hash.
- Official/base price fields.
- Seller multiplier.
- Global fee rate.
- Final buyer charge.

### State Transitions

Credential state is represented by multiple dimensions instead of one overloaded status:

```text
seller_status: active -> paused -> active
health_status: untested -> healthy -> degraded -> failed -> healthy
capacity_status: available -> busy -> exhausted -> available
capacity_status: available -> expired
risk_status: normal -> watching -> risk_paused -> normal
```

Eligibility rules:

- Order-list display requires seller active, risk normal or watching, not expired, and supported models.
- New fixed-route purchase additionally requires health healthy or degraded and capacity not exhausted.
- Pool routing additionally requires current concurrency below limit.
- Existing fixed-route call can proceed when the fixed order is valid and the credential is technically usable, unless risk is `risk_paused`.

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
5. Create fixed order with purchase snapshots.
6. Write buyer-facing balance log.

Fixed-route call settlement must be idempotent:

1. Validate buyer token and fixed-order ownership.
2. Lock fixed order and credential capacity.
3. Reserve estimated fixed-order quota if pre-consume is required.
4. Proxy through the encrypted seller credential.
5. Calculate actual official cost and buyer charge.
6. Adjust fixed-order remaining quota.
7. Create fill record.
8. Create settlement row with fee snapshot.
9. Update credential stats.

Pool call settlement must be idempotent:

1. Validate buyer token.
2. Select and lock a credential candidate.
3. Reserve estimated buyer quota if pre-consume is required.
4. Proxy through the encrypted seller credential.
5. Calculate actual official cost and buyer charge.
6. Adjust buyer quota.
7. Create pool fill record.
8. Create settlement row with fee snapshot.
9. Update credential stats.

All fill and settlement writes need a unique request id or idempotency key. Retried client requests and retry-after-upstream-success paths must not double-charge buyers or double-create seller income.

### Pool Router

The router should use a two-step process: hard filtering first, scoring second.

Hard filters:

- Marketplace enabled.
- Channel type enabled for marketplace.
- Model supported.
- Seller active.
- Risk not paused.
- Credential not expired.
- Credential not exhausted.
- Current concurrency below limit.
- Provider endpoint policy valid.

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
- Credential expiry check: mark expired seller-limited keys and stop new sales/routing.
- Fixed-order expiry check: mark expired orders and invalidate unused quota without refund.
- Settlement release: move pending seller income to available after the hold period.
- Stats aggregation: refresh success rate, latency, request counts, and failure counters.
- Reconciliation: repair requests that succeeded upstream but failed before local billing completed.

### UI Surfaces

Seller:

- Escrow key form: channel type, models, limit mode, time mode, multiplier, concurrency.
- Seller credential list: masked key fingerprint, models, multiplier, health, active orders, pool usage, income.
- Seller income page: pending, available, withdrawn, blocked.

Buyer:

- Order list: filters based on seller configuration and runtime status.
- Fixed-order purchase modal: choose quota amount within global guardrails, show effective price snapshot.
- Fixed-order list: remaining quota, expiry, status, bound credential health.
- Pool page: model selection, optional filters, estimated effective price range.

Admin:

- Marketplace settings.
- Enabled channel type allowlist.
- Credential risk review.
- Fixed-order review.
- Fill and settlement audit.
- Reconciliation queue.

## Acceptance Criteria

- A seller can escrow one supported channel-type API key and never see the plaintext key again.
- The same escrowed key appears in order-list and pool candidates when eligible.
- A buyer can buy fixed-route quota for part of a seller key without removing the key from the market.
- Fixed-route usage deducts only that order's remaining quota.
- Pool usage deducts the buyer's normal quota balance.
- Other buyers' usage never reduces an existing fixed-route order.
- Fixed-route unused quota expires without refund at order expiry.
- Seller income is created only after successful usage.
- Platform fee uses the global marketplace fee setting and is snapshotted per settlement.
- Buyer-facing price equals official/base cost times seller multiplier.
- Buyer-facing stats show success rate, not both success rate and error rate.
- Raw seller API keys are encrypted at rest, masked in logs, and never returned by APIs.
- Unsupported channel types are rejected even if they exist in the project's global channel constants.

## Security Requirements

- Raw seller API keys must be encrypted at rest.
- The encryption key must come from environment or secret manager configuration.
- API key plaintext must never be returned by seller, buyer, or admin list APIs.
- Seller key replacement should not expose old key material.
- Marketplace calls must verify buyer token ownership and fixed-order ownership.
- Marketplace router must validate provider and base URL against a safe provider policy.
- Marketplace router must reject any existing channel type that has not been explicitly enabled for marketplace escrow.
- Settlement writes must be idempotent.
- Logs must mask API keys and sensitive upstream URLs.
- Admin access must be required for risk overrides and settlement blocking.

## Open Decisions Before Implementation

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
