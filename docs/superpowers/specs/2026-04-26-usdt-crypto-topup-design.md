# USDT Crypto Top-up Design

Date: 2026-04-26
Project: hermestoken
Status: Draft approved for implementation planning

## 1. Summary

Add self-hosted USDT top-up support to the existing recharge system.

The first version supports:

- USDT on TRON TRC-20
- USDT on BSC ERC-20 compatible token events
- One fixed receiving address per network
- Unique payment amounts for order matching
- A 10 minute order validity window
- 1 USDT = 1 USD pricing
- TRON confirmations: 20 blocks
- BSC confirmations: 15 blocks
- Multi-instance deployments with Redis scanner locks and DB idempotency

The feature must reuse the existing `TopUp` and wallet quota model. It must not create a separate user balance or bypass existing recharge history, admin completion, or logging paths.

## 2. Goals

- Let users top up with USDT on TRON and BSC.
- Automatically detect payments to fixed receiving addresses.
- Match payments by exact unique amount, not by user-submitted transaction hash.
- Credit user quota after the required confirmations.
- Keep the system safe under multi-instance deployment.
- Keep payment completion idempotent under retries, repeated scans, and admin actions.
- Avoid storing private keys in the application.

## 3. Non-goals

The first version will not include:

- USDC support
- Polygon, Base, Ethereum mainnet, or other chains
- HD wallet or per-order addresses
- Automatic refund
- Automatic sweeping or consolidation
- Hot wallet private key storage
- User-submitted transaction hash auto-claim
- Dynamic stablecoin depeg handling
- Complex analytics dashboards

## 4. Architecture

USDT top-up is a new payment gateway inside the existing top-up system.

High-level flow:

```text
User selects top-up amount
  -> backend creates TopUp and CryptoPaymentOrder
  -> frontend displays network, fixed address, exact amount, and countdown
  -> user sends USDT
  -> scanner reads TRON/BSC transfer events
  -> scanner stores CryptoPaymentTransaction
  -> scanner matches exact payment amount to pending order
  -> scanner waits for required confirmations
  -> backend completes TopUp and credits quota in one DB transaction
  -> frontend polling shows success
```

Core components:

- `TopUp`: existing business recharge order.
- `CryptoPaymentOrder`: crypto-specific order metadata and state.
- `CryptoPaymentTransaction`: observed on-chain transfer evidence.
- `CryptoPaymentScanner`: background scanner for TRON and BSC.
- Redis lock: elects one scanner per network across multiple instances.
- DB transaction: enforces idempotent final crediting.

## 5. Payment Method

Add a new internal payment method:

```text
crypto_usdt
```

The network is stored separately:

```text
tron_trc20
bsc_erc20
```

`TopUp` fields for crypto orders:

- `amount`: USD baseline amount purchased, for example `10`.
- `money`: exact USDT amount owed, for example `10.003721`.
- `payment_method`: `crypto_usdt`.
- `currency`: `USDT`.
- `trade_no`: local crypto order number.
- `status`: existing status values where possible, such as `pending`, `success`, `expired`, `failed`.

## 6. Database Design

### 6.1 `crypto_payment_orders`

Stores one crypto top-up order.

Suggested fields:

```text
id
 topup_id
 trade_no
 user_id
 network
 token_symbol
 token_contract
 token_decimals
 receive_address
 base_amount
 pay_amount
 pay_amount_base_units
 unique_suffix
 expires_at
 required_confirmations
 status
 matched_tx_hash
 matched_log_index
 detected_at
 confirmed_at
 completed_at
 create_time
 update_time
```

Field notes:

- `network`: `tron_trc20` or `bsc_erc20`.
- `token_symbol`: `USDT`.
- `token_decimals`: TRON uses 6; BSC should be configured or read from the contract, but payment display remains 6 decimals.
- `base_amount`: amount the user intended to buy, denominated in USD/USDT.
- `pay_amount`: exact display payment amount.
- `pay_amount_base_units`: integer string in token base units. This is used for matching.
- `status`: crypto-specific status.

Suggested constraints:

- `trade_no` unique.
- `topup_id` unique.
- Application-level uniqueness for active orders by `network + receive_address + pay_amount_base_units` within the unexpired window.
- Unique matched transaction evidence by `network + matched_tx_hash + matched_log_index` when present.

### 6.2 `crypto_payment_transactions`

Stores observed token transfer events.

Suggested fields:

```text
id
 network
 tx_hash
 log_index
 block_number
 block_timestamp
 from_address
 to_address
 token_contract
 token_symbol
 token_decimals
 amount
 amount_base_units
 confirmations
 status
 matched_order_id
 raw_payload
 create_time
 update_time
```

Suggested constraints:

- `network + tx_hash + log_index` unique.

Notes:

- This table is evidence only. It must not credit users directly.
- Final crediting is done by completing a matched crypto order and linked `TopUp`.

### 6.3 `crypto_scanner_state`

Stores scanner progress.

Suggested fields:

```text
network
last_scanned_block
last_finalized_block
updated_at
```

Notes:

- A standalone table is clearer than storing scanner cursors in generic options.
- Scanner must not advance the cursor if RPC or DB processing fails for the scanned range.

## 7. Status Machine

Normal path:

```text
pending -> detected -> confirmed -> success
```

Exceptional states:

```text
expired
late_paid
underpaid
overpaid
ambiguous
failed
```

Rules:

- Only `confirmed -> success` can credit user quota.
- `success` must be idempotent.
- `underpaid`, `overpaid`, `late_paid`, and `ambiguous` are not auto-credited in version one.
- Admin completion must use the same idempotent transaction and must reference on-chain evidence.

## 8. Unique Amount Algorithm

Use exact unique amounts because all orders share one fixed receiving address per network.

Example:

```text
base_amount = 10.000000 USDT
unique_suffix = 0.003721 USDT
pay_amount = 10.003721 USDT
```

Generation rules:

- Generate random suffix from `1` to `9999`.
- Format suffix as six decimals: `suffix / 1_000_000`.
- Add suffix to the base amount.
- Store the exact display string and base unit integer string.
- In a DB transaction, check that no unexpired active order has the same `network + receive_address + pay_amount_base_units`.
- Retry up to 20 times on collision.
- If still colliding, return a temporary busy error.

Do not use `float64` for matching. Use decimal/string/big integer logic.

USDT precision:

- TRON TRC-20 USDT: 6 decimals.
- BSC USDT: token may use 18 decimals, but display should still be 6 decimals. Convert the exact 6-decimal payment amount into contract base units.

## 9. Matching Algorithm

When the scanner observes a transfer event:

1. Verify network matches a configured network.
2. Verify token contract equals the configured USDT contract.
3. Verify `to_address` equals the configured fixed receiving address.
4. Convert transfer amount into `amount_base_units`.
5. Look for a `pending` order with the same `network + receive_address + amount_base_units` and `now <= expires_at`.
6. If exactly one order is found, mark it `detected` and attach transaction evidence.
7. If no active order is found but an expired order has the same exact amount, mark that order `late_paid`.
8. If multiple orders match, mark the situation `ambiguous` and do not credit.
9. If amount is close but not exact, do not guess. Keep the transaction unmatched or mark the related order as exceptional only when there is deterministic evidence.

Strict behavior:

- Underpayment is not auto-credited.
- Overpayment is not auto-credited.
- Late payment is not auto-credited.
- Approximate matching is not allowed.

## 10. Scanner Design

### 10.1 Multi-instance Locking

Each instance can start scanner goroutines. Only the instance that owns the Redis lock scans a given network.

Locks:

```text
crypto:scanner:tron_trc20
crypto:scanner:bsc_erc20
```

Rules:

- Lock TTL: 30 seconds.
- Renew every 10 seconds.
- If lock is lost, stop scanning before processing the next batch.
- If Redis is unavailable, pause scanner work but keep user APIs available.
- DB idempotency must still protect against accidental double scanning.

### 10.2 Cursor Management

Startup:

- If `last_scanned_block` exists, resume from it.
- Otherwise initialize to `current_block - confirmations - safety_window`.

Suggested safety windows:

- TRON: 40 blocks.
- BSC: 30 blocks.

Failure behavior:

- RPC failure: do not advance cursor.
- DB write failure: do not advance cursor.
- Parse failure for one event: store/log safely and continue only if the batch can be processed deterministically.
- Completion failure after confirmation: keep order retryable.

### 10.3 TRON TRC-20 Scanner

Target event:

```text
Transfer(address,address,uint256)
```

Mainnet USDT contract default:

```text
TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj
```

Scanner should:

- Query TRON contract events using configured TronGrid/fullnode API.
- Filter transfers to the configured receiving address.
- Normalize TRON Base58 and hex address forms before comparing.
- Use 6 decimals.
- Store `tx_hash`, event index, block number, timestamp, from, to, and amount.
- Wait for 20 confirmations before final completion.

### 10.4 BSC Scanner

Target event:

```text
Transfer(address,address,uint256)
```

Mainnet USDT contract default:

```text
0x55d398326f99059fF775485246999027B3197955
```

Scanner should:

- Use `eth_getLogs`.
- Filter by token contract address and transfer topic.
- Filter `topic2` by the padded receiving address.
- Scan bounded block ranges, for example 500 to 1000 blocks per request.
- Store `tx_hash`, `log_index`, block number, timestamp, from, to, and amount.
- Wait for 15 confirmations before final completion.

## 11. Completion Transaction

Add a crypto-specific completion function, for example:

```text
CompleteCryptoTopUp(tradeNo, txEvidence)
```

It should run in one DB transaction:

1. Lock `crypto_payment_orders` by `trade_no`.
2. Lock the linked `TopUp`.
3. Verify crypto order is not already `success`.
4. Verify linked `TopUp` is still `pending`.
5. Verify transaction evidence: network, token contract, address, amount, confirmations.
6. Compute quota using the same standard top-up rules used by current non-Creem online recharge.
7. Mark `TopUp` `success`.
8. Mark crypto order `success`.
9. Credit `users.quota`.
10. Write top-up log.

Idempotency requirements:

- Repeating the same completion call must not add quota twice.
- A transaction already matched to a successful order cannot be reused.
- Admin completion uses the same transaction path.

## 12. Backend API

### 12.1 User APIs

Add authenticated routes:

```text
GET  /api/user/crypto/topup/config
POST /api/user/crypto/topup/order
GET  /api/user/crypto/topup/order/:trade_no
```

`GET /config` response shape:

```json
{
  "enabled": true,
  "networks": [
    {
      "network": "tron_trc20",
      "display_name": "TRON TRC-20",
      "token": "USDT",
      "confirmations": 20,
      "min_topup": 1
    },
    {
      "network": "bsc_erc20",
      "display_name": "BSC",
      "token": "USDT",
      "confirmations": 15,
      "min_topup": 1
    }
  ],
  "expire_minutes": 10
}
```

`POST /order` request:

```json
{
  "network": "tron_trc20",
  "amount": 10
}
```

`POST /order` response:

```json
{
  "trade_no": "CRYPTO-123-...",
  "network": "tron_trc20",
  "token": "USDT",
  "receive_address": "T...",
  "base_amount": "10.000000",
  "pay_amount": "10.003721",
  "expires_at": 1710000000,
  "required_confirmations": 20,
  "status": "pending"
}
```

`GET /order/:trade_no` response:

```json
{
  "trade_no": "CRYPTO-123-...",
  "status": "detected",
  "tx_hash": "...",
  "confirmations": 8,
  "required_confirmations": 20,
  "expires_at": 1710000000
}
```

### 12.2 Admin APIs

Add admin routes for operational visibility:

```text
GET  /api/admin/crypto/topup/orders
GET  /api/admin/crypto/topup/transactions
POST /api/admin/crypto/topup/orders/:trade_no/complete
POST /api/admin/crypto/topup/orders/:trade_no/ignore
POST /api/admin/crypto/scanner/rescan
```

Completion and state-changing admin routes should require:

- `RootAuth`
- `CriticalRateLimit`
- `SecureVerificationRequired`

## 13. Configuration

Add settings:

```text
CryptoPaymentEnabled
CryptoTronEnabled
CryptoTronReceiveAddress
CryptoTronUSDTContract
CryptoTronRPCURL
CryptoTronAPIKey
CryptoTronConfirmations
CryptoBSCEnabled
CryptoBSCReceiveAddress
CryptoBSCUSDTContract
CryptoBSCRPCURL
CryptoBSCConfirmations
CryptoOrderExpireMinutes
CryptoUniqueSuffixMax
CryptoScannerEnabled
```

Defaults:

```text
CryptoPaymentEnabled = false
CryptoTronConfirmations = 20
CryptoBSCConfirmations = 15
CryptoOrderExpireMinutes = 10
CryptoUniqueSuffixMax = 9999
CryptoScannerEnabled = true
```

Validation:

- TRON address must use the expected TRON address format.
- BSC address must be a valid `0x` 20-byte address.
- Token contract must match the chain address format.
- TRON confirmations must be at least 10.
- BSC confirmations must be at least 8.
- Order expiry should be between 5 and 60 minutes.
- A network is hidden from users if its required config is incomplete.

Secrets:

- API keys must not be printed in logs.
- API keys must not be returned to the frontend in plaintext.

## 14. Frontend UX

Add USDT to the top-up payment area.

Flow:

```text
Select top-up amount
  -> click USDT
  -> choose network: TRON TRC-20 or BSC
  -> create order
  -> show payment modal
```

Payment modal must show:

- Network badge.
- Token: USDT.
- Receiving address with copy button.
- Exact amount with copy button.
- 10 minute countdown.
- Status: waiting, detected, confirming, success, expired, exceptional.
- Confirmation count after transaction detection.
- Transaction hash after detection.
- Clear warnings about exact amount and network.

Polling:

- Poll `GET /api/user/crypto/topup/order/:trade_no` every 3 seconds.
- Stop polling on terminal states: `success`, `expired`, `failed`, `underpaid`, `overpaid`, `late_paid`, `ambiguous`.
- Refresh user quota after `success`.

Warnings to display:

```text
Only send USDT on the selected network.
Pay the exact displayed amount.
Do not round the amount.
This order expires in 10 minutes.
Late, underpaid, or overpaid transfers require admin review.
```

## 15. Admin UI

Add a `USDT Settings` or `Crypto Settings` tab under payment settings.

Fields:

- Enable USDT top-up.
- Enable TRON TRC-20.
- TRON receive address.
- TRON USDT contract.
- TRON RPC/API URL.
- TRON API key.
- TRON confirmations.
- Enable BSC.
- BSC receive address.
- BSC USDT contract.
- BSC RPC URL.
- BSC confirmations.
- Order expiry minutes.
- Unique suffix max.
- Enable scanner.

Add admin order/transaction pages if time allows in phase one. At minimum, expose enough list/detail views to investigate unmatched and exceptional payments.

## 16. Security

Key principles:

- Do not store private keys.
- Do not perform automatic refunds.
- Do not perform automatic sweeping.
- Do not trust client-submitted transaction data.
- Only configured tokens and receiving addresses are valid.
- Only exact amounts in an active order window can auto-match.
- Only confirmed transfers can credit users.
- All final crediting must be idempotent and transactional.

Admin completion must:

- Require strong admin authorization.
- Re-validate chain evidence before crediting.
- Record operation logs.
- Require a reason.
- Use the same completion transaction as the scanner.

## 17. Observability

Track or log:

- Scanner lock owner by network.
- Last scanned block by network.
- RPC failures.
- Pending order count.
- Detected but unconfirmed order count.
- Confirmed but not completed order count.
- Unmatched transaction count.
- Late, underpaid, overpaid, and ambiguous counts.
- Completion failures.

## 18. Testing Plan

### 18.1 Backend Unit Tests

- Unique amount generation avoids active-window collisions.
- Amount string to base unit conversion is correct.
- TRON 6-decimal conversion is correct.
- BSC 18-decimal conversion is correct.
- Expired orders are not auto-matched.
- Exact payment matches one order.
- Underpayment and overpayment do not auto-credit.
- Duplicate `network + tx_hash + log_index` is rejected.
- `CompleteCryptoTopUp` is idempotent.
- Completion rejects non-pending `TopUp`.
- Incomplete config hides the network from users.

### 18.2 Backend Integration Tests

- Create order, insert simulated transfer with insufficient confirmations, verify no credit.
- Reach required confirmations, verify successful credit.
- Simulate two instances competing for scanner lock.
- RPC failure does not advance cursor.
- Confirmed order completion failure retries later.
- Expired order receiving payment becomes `late_paid`.
- Active duplicate amount is prevented.

### 18.3 Frontend Tests

- Network selection creates an order.
- Payment modal shows network, address, exact amount, and countdown.
- Copy amount copies the full exact amount.
- Status states render correctly.
- Success refreshes user quota.
- Expired order disables or warns against payment.
- USDT option is hidden when config is disabled.

### 18.4 Manual Acceptance

- Small-value TRON mainnet or test environment transfer.
- Small-value BSC testnet or mainnet transfer.
- Restart service and verify scanner resumes from cursor.
- Run multiple app instances and verify only one active scanner per network.
- Rescan the same blocks and verify no duplicate credit.
- Verify exceptional payments appear for admin review.

## 19. Implementation Phases

### Phase 1: Minimum viable auto-credit

- Models and migrations.
- Settings and validation.
- Unique amount order creation.
- User APIs.
- Completion transaction.
- BSC scanner.
- TRON scanner.
- Payment modal and polling.
- Basic admin visibility for exceptional orders and transactions.

### Phase 2: Operational tooling

- Rich admin order and transaction views.
- Bind unmatched transaction to order.
- Admin handling for underpaid, overpaid, and late payments.
- Scanner status panel.
- Block explorer links.
- More detailed operation logs.
- RPC fallback and retry tuning.

### Phase 3: Expansion

- USDC.
- More EVM chains.
- HD per-order addresses.
- Automatic sweeping.
- Risk controls and address blacklist.
- Stablecoin depeg checks.

## 20. Open Implementation Notes

- The project supports SQLite, MySQL, and PostgreSQL. Migrations must stay cross-database compatible.
- Use GORM where possible. Raw SQL must branch for database differences when needed.
- Use existing API response conventions.
- Use the project's JSON helpers in Go business code.
- Reuse existing top-up logs and quota accounting semantics.
- Do not modify protected project identity or attribution text.
