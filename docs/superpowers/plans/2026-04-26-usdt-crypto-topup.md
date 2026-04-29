# USDT Crypto Top-up Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build Phase 1 self-hosted USDT top-up support for TRON TRC-20 and BSC using fixed receiving addresses, unique payment amounts, chain scanners, and idempotent quota crediting.

**Architecture:** Crypto top-up is added as a new payment gateway that reuses existing `TopUp` records for business accounting and adds crypto-specific order, transaction, and scanner-state tables. Multi-instance safety comes from Redis scanner locks plus DB uniqueness and transactional completion. Chain integrations use focused scanner packages with fakeable clients so most behavior is tested without live RPC.

**Tech Stack:** Go 1.25, Gin, GORM, SQLite/MySQL/PostgreSQL compatibility, Redis v8, `shopspring/decimal`, React 18, Vite, Semi UI, Bun for frontend scripts.

---

## Scope

This plan implements Phase 1 from `docs/superpowers/specs/2026-04-26-usdt-crypto-topup-design.md`:

- backend models, migrations, settings, APIs, completion transaction, scanner lock/state, BSC scanner, TRON scanner
- frontend user payment modal, polling, and admin crypto settings
- basic admin order/transaction visibility and secure admin completion

This plan does not implement USDC, Polygon, HD wallets, automatic refund, automatic sweeping, or additional chains.

## File Map

### Backend model layer

- Create `model/topup_crypto.go` for crypto constants, structs, amount helpers, order creation, matching, scanner state, and completion transaction.
- Create `model/topup_crypto_test.go` for DB-backed tests around amount conversion, matching, idempotent completion, expiry, and active amount collision.
- Modify `model/main.go` to AutoMigrate `CryptoPaymentOrder`, `CryptoPaymentTransaction`, and `CryptoScannerState` in both migration paths.
- Modify `model/topup.go` only to add `PaymentMethodCryptoUSDT` and to make manual completion reject crypto orders unless a crypto evidence path is used.

### Backend settings and controllers

- Create `setting/payment_crypto.go` for crypto settings, defaults, network config, and validators.
- Modify `model/option.go` to load and update crypto options.
- Create `controller/topup_crypto.go` for user and admin HTTP handlers.
- Modify `router/api-router.go` to register user and admin crypto routes.
- Modify `controller/topup.go` to include crypto availability in `GetTopUpInfo`.

### Backend scanner service

- Create `service/crypto_payment/lock.go` for Redis scanner lock acquisition and renewal.
- Create `service/crypto_payment/scanner.go` for common scanner loop orchestration.
- Create `service/crypto_payment/bsc.go` for BSC JSON-RPC log scanning.
- Create `service/crypto_payment/tron.go` for TRON event scanning.
- Create `service/crypto_payment/*_test.go` for lock behavior with fake Redis, scanner state transitions, BSC log decoding, and TRON event decoding.
- Modify `main.go` to call `service.StartCryptoPaymentScanners()` after Redis initialization and option loading.

### Frontend

- Create `web/src/components/topup/modals/CryptoPaymentModal.jsx` for crypto payment instructions and polling.
- Modify `web/src/components/topup/index.jsx` to create crypto orders, poll status, and refresh quota.
- Modify `web/src/components/topup/RechargeCard.jsx` to render the USDT option and network selector.
- Modify `web/src/components/topup/modals/TopupHistoryModal.jsx` to show crypto network, amount, and transaction hash when present.
- Create `web/src/pages/Setting/Payment/SettingsPaymentGatewayCrypto.jsx` for admin crypto configuration.
- Modify `web/src/components/settings/PaymentSetting.jsx` to add the crypto settings tab and options wiring.
- Update `web/src/i18n/locales/*.json` using the project i18n workflow after strings are added.

---

### Task 1: Add Crypto Models and Amount Helpers

**Files:**
- Create: `model/topup_crypto.go`
- Create: `model/topup_crypto_test.go`
- Modify: `model/topup.go`

- [ ] **Step 1: Write failing tests for amount conversion and active statuses**

Add `model/topup_crypto_test.go` with these tests. They do not require live RPC.

```go
package model

import (
	"testing"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCryptoPayAmountFromSuffix_TRONSixDecimals(t *testing.T) {
	pay, units, err := CryptoPayAmountFromSuffix(decimal.NewFromInt(10), 6, 3721)
	require.NoError(t, err)
	assert.Equal(t, "10.003721", pay)
	assert.Equal(t, "10003721", units)
}

func TestCryptoPayAmountFromSuffix_BSCUnitsWith18Decimals(t *testing.T) {
	pay, units, err := CryptoPayAmountFromSuffix(decimal.NewFromInt(10), 18, 3721)
	require.NoError(t, err)
	assert.Equal(t, "10.003721", pay)
	assert.Equal(t, "10003721000000000000", units)
}

func TestCryptoPayAmountFromSuffixRejectsInvalidSuffix(t *testing.T) {
	_, _, err := CryptoPayAmountFromSuffix(decimal.NewFromInt(10), 6, 0)
	require.Error(t, err)
	_, _, err = CryptoPayAmountFromSuffix(decimal.NewFromInt(10), 6, 10000)
	require.Error(t, err)
}

func TestCryptoOrderActiveStatus(t *testing.T) {
	assert.True(t, IsActiveCryptoOrderStatus(CryptoPaymentStatusPending))
	assert.True(t, IsActiveCryptoOrderStatus(CryptoPaymentStatusDetected))
	assert.False(t, IsActiveCryptoOrderStatus(CryptoPaymentStatusSuccess))
	assert.False(t, IsActiveCryptoOrderStatus(CryptoPaymentStatusExpired))
}

func TestCryptoOrderIsExpired(t *testing.T) {
	order := &CryptoPaymentOrder{ExpiresAt: time.Now().Add(-time.Second).Unix()}
	assert.True(t, order.IsExpired(time.Now()))
}

func TestCryptoPaymentMethodConstant(t *testing.T) {
	assert.Equal(t, "crypto_usdt", PaymentMethodCryptoUSDT)
	assert.Equal(t, common.TopUpStatusPending, CryptoTopUpInitialStatus())
}
```

- [ ] **Step 2: Run the failing tests**

Run:

```bash
go test ./model -run 'TestCryptoPayAmountFromSuffix|TestCryptoOrderActiveStatus|TestCryptoOrderIsExpired|TestCryptoPaymentMethodConstant' -count=1
```

Expected: FAIL with undefined `CryptoPayAmountFromSuffix`, `CryptoPaymentOrder`, status constants, and `PaymentMethodCryptoUSDT`.

- [ ] **Step 3: Add crypto constants, structs, and helpers**

Create `model/topup_crypto.go`:

```go
package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/shopspring/decimal"
)

const (
	CryptoNetworkTronTRC20 = "tron_trc20"
	CryptoNetworkBSCERC20  = "bsc_erc20"

	CryptoTokenUSDT = "USDT"

	CryptoPaymentStatusPending   = "pending"
	CryptoPaymentStatusDetected  = "detected"
	CryptoPaymentStatusConfirmed = "confirmed"
	CryptoPaymentStatusSuccess   = "success"
	CryptoPaymentStatusExpired   = "expired"
	CryptoPaymentStatusUnderpaid = "underpaid"
	CryptoPaymentStatusOverpaid  = "overpaid"
	CryptoPaymentStatusAmbiguous = "ambiguous"
	CryptoPaymentStatusLatePaid  = "late_paid"
	CryptoPaymentStatusFailed    = "failed"

	CryptoTransactionStatusSeen      = "seen"
	CryptoTransactionStatusConfirmed = "confirmed"
	CryptoTransactionStatusIgnored   = "ignored"
	CryptoTransactionStatusOrphaned  = "orphaned"
)

var (
	ErrCryptoInvalidAmount       = errors.New("invalid crypto payment amount")
	ErrCryptoInvalidSuffix       = errors.New("invalid crypto payment suffix")
	ErrCryptoOrderNotFound       = errors.New("crypto payment order not found")
	ErrCryptoOrderStatusInvalid  = errors.New("crypto payment order status invalid")
	ErrCryptoTransactionMismatch = errors.New("crypto transaction evidence mismatch")
	ErrCryptoAmountCollision     = errors.New("crypto payment amount collision")
)

type CryptoPaymentOrder struct {
	Id                    int    `json:"id"`
	TopUpId               int    `json:"topup_id" gorm:"uniqueIndex"`
	TradeNo               string `json:"trade_no" gorm:"uniqueIndex;type:varchar(255)"`
	UserId                int    `json:"user_id" gorm:"index"`
	Network               string `json:"network" gorm:"type:varchar(32);index"`
	TokenSymbol           string `json:"token_symbol" gorm:"type:varchar(16)"`
	TokenContract         string `json:"token_contract" gorm:"type:varchar(128);index"`
	TokenDecimals         int    `json:"token_decimals"`
	ReceiveAddress        string `json:"receive_address" gorm:"type:varchar(128);index"`
	BaseAmount            string `json:"base_amount" gorm:"type:varchar(64)"`
	PayAmount             string `json:"pay_amount" gorm:"type:varchar(64)"`
	PayAmountBaseUnits    string `json:"pay_amount_base_units" gorm:"type:varchar(128);index"`
	UniqueSuffix          int    `json:"unique_suffix"`
	ExpiresAt             int64  `json:"expires_at" gorm:"index"`
	RequiredConfirmations int    `json:"required_confirmations"`
	Status                string `json:"status" gorm:"type:varchar(32);index"`
	MatchedTxHash         string `json:"matched_tx_hash" gorm:"type:varchar(128);index"`
	MatchedLogIndex       int    `json:"matched_log_index" gorm:"default:-1"`
	DetectedAt            int64  `json:"detected_at"`
	ConfirmedAt           int64  `json:"confirmed_at"`
	CompletedAt           int64  `json:"completed_at"`
	CreateTime            int64  `json:"create_time"`
	UpdateTime            int64  `json:"update_time"`
}

type CryptoPaymentTransaction struct {
	Id              int    `json:"id"`
	Network         string `json:"network" gorm:"type:varchar(32);uniqueIndex:idx_crypto_tx_event"`
	TxHash          string `json:"tx_hash" gorm:"type:varchar(128);uniqueIndex:idx_crypto_tx_event"`
	LogIndex        int    `json:"log_index" gorm:"uniqueIndex:idx_crypto_tx_event"`
	BlockNumber     int64  `json:"block_number" gorm:"index"`
	BlockTimestamp  int64  `json:"block_timestamp"`
	FromAddress     string `json:"from_address" gorm:"type:varchar(128);index"`
	ToAddress       string `json:"to_address" gorm:"type:varchar(128);index"`
	TokenContract   string `json:"token_contract" gorm:"type:varchar(128);index"`
	TokenSymbol     string `json:"token_symbol" gorm:"type:varchar(16)"`
	TokenDecimals   int    `json:"token_decimals"`
	Amount          string `json:"amount" gorm:"type:varchar(64)"`
	AmountBaseUnits string `json:"amount_base_units" gorm:"type:varchar(128);index"`
	Confirmations   int64  `json:"confirmations"`
	Status          string `json:"status" gorm:"type:varchar(32);index"`
	MatchedOrderId  int    `json:"matched_order_id" gorm:"index"`
	RawPayload      string `json:"raw_payload" gorm:"type:text"`
	CreateTime      int64  `json:"create_time"`
	UpdateTime      int64  `json:"update_time"`
}

type CryptoScannerState struct {
	Network            string `json:"network" gorm:"primaryKey;type:varchar(32)"`
	LastScannedBlock   int64  `json:"last_scanned_block"`
	LastFinalizedBlock int64  `json:"last_finalized_block"`
	UpdatedAt          int64  `json:"updated_at"`
}

func CryptoTopUpInitialStatus() string {
	return common.TopUpStatusPending
}

func IsActiveCryptoOrderStatus(status string) bool {
	switch status {
	case CryptoPaymentStatusPending, CryptoPaymentStatusDetected, CryptoPaymentStatusConfirmed:
		return true
	default:
		return false
	}
}

func (o *CryptoPaymentOrder) IsExpired(now time.Time) bool {
	if o == nil || o.ExpiresAt <= 0 {
		return false
	}
	return now.Unix() > o.ExpiresAt
}

func CryptoPayAmountFromSuffix(baseAmount decimal.Decimal, tokenDecimals int, suffix int) (string, string, error) {
	if baseAmount.LessThanOrEqual(decimal.Zero) || tokenDecimals < 6 {
		return "", "", ErrCryptoInvalidAmount
	}
	if suffix < 1 || suffix > 9999 {
		return "", "", ErrCryptoInvalidSuffix
	}
	payAmount := baseAmount.Add(decimal.NewFromInt(int64(suffix)).Div(decimal.NewFromInt(1_000_000)))
	payDisplay := payAmount.StringFixed(6)
	unitMultiplier := decimal.NewFromInt(10).Pow(decimal.NewFromInt(int64(tokenDecimals)))
	baseUnits := payAmount.Mul(unitMultiplier).Round(0)
	return payDisplay, baseUnits.StringFixed(0), nil
}

func NormalizeCryptoNetwork(network string) string {
	return strings.ToLower(strings.TrimSpace(network))
}

func cryptoNow() int64 {
	return time.Now().Unix()
}

func cryptoRefCol(column string) string {
	if common.UsingPostgreSQL {
		return fmt.Sprintf("\"%s\"", column)
	}
	return fmt.Sprintf("`%s`", column)
}
```

Modify `model/topup.go` by replacing the existing payment-method const block with:

```go
const (
	PaymentMethodStripe       = "stripe"
	PaymentMethodCreem        = "creem"
	PaymentMethodWaffo        = "waffo"
	PaymentMethodWaffoPancake = "waffo_pancake"
	PaymentMethodCryptoUSDT   = "crypto_usdt"
)
```

- [ ] **Step 4: Run the helper tests**

Run:

```bash
go test ./model -run 'TestCryptoPayAmountFromSuffix|TestCryptoOrderActiveStatus|TestCryptoOrderIsExpired|TestCryptoPaymentMethodConstant' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add model/topup_crypto.go model/topup_crypto_test.go model/topup.go
git commit -m "feat: add crypto topup models"
```

---

### Task 2: Migrate Crypto Tables

**Files:**
- Modify: `model/main.go`
- Modify: `model/task_cas_test.go`
- Modify: `model/subscription_test_helpers_test.go`
- Modify: `model/topup_crypto_test.go`

- [ ] **Step 1: Write failing migration test**

Append to `model/topup_crypto_test.go`:

```go
func TestCryptoTablesAutoMigrate(t *testing.T) {
	truncateTables(t)
	require.True(t, DB.Migrator().HasTable(&CryptoPaymentOrder{}))
	require.True(t, DB.Migrator().HasTable(&CryptoPaymentTransaction{}))
	require.True(t, DB.Migrator().HasTable(&CryptoScannerState{}))
}
```

- [ ] **Step 2: Run the migration test**

Run:

```bash
go test ./model -run TestCryptoTablesAutoMigrate -count=1
```

Expected before migration wiring: FAIL because the tables are not migrated by `TestMain` or production migration.

- [ ] **Step 3: Add crypto tables to production migrations**

In `model/main.go`, add these models to both `migrateDB()` and `migrateDBFast()` AutoMigrate lists, immediately after `&TopUp{}`:

```go
&CryptoPaymentOrder{},
&CryptoPaymentTransaction{},
&CryptoScannerState{},
```

- [ ] **Step 4: Add crypto tables to test migrations and truncation**

In `model/task_cas_test.go`, add to `TestMain` AutoMigrate after `&TopUp{}`:

```go
&CryptoPaymentOrder{},
&CryptoPaymentTransaction{},
&CryptoScannerState{},
```

In `truncateTables`, add cleanup inside `t.Cleanup`:

```go
DB.Exec("DELETE FROM crypto_payment_orders")
DB.Exec("DELETE FROM crypto_payment_transactions")
DB.Exec("DELETE FROM crypto_scanner_states")
```

In `model/subscription_test_helpers_test.go`, add the three crypto models to `setupSubscriptionReferralSettlementDB` AutoMigrate after `&TopUp{}` so shared payment tests can create crypto records.

- [ ] **Step 5: Run migration tests**

Run:

```bash
go test ./model -run 'TestCryptoTablesAutoMigrate|TestCryptoPayAmountFromSuffix' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add model/main.go model/task_cas_test.go model/subscription_test_helpers_test.go model/topup_crypto_test.go
git commit -m "feat: migrate crypto topup tables"
```

---

### Task 3: Add Crypto Payment Settings

**Files:**
- Create: `setting/payment_crypto.go`
- Create: `setting/payment_crypto_test.go`
- Modify: `model/option.go`

- [ ] **Step 1: Write failing settings tests**

Create `setting/payment_crypto_test.go`:

```go
package setting

import (
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/stretchr/testify/assert"
)

func TestGetCryptoPaymentNetworksHidesIncompleteNetworks(t *testing.T) {
	original := common.OptionMap
	common.OptionMap = map[string]string{
		"CryptoPaymentEnabled":     "true",
		"CryptoTronEnabled":        "true",
		"CryptoTronReceiveAddress": "TQ4mVnPz4jG4n4hD9QJf9U9gKfZVfUiH9z",
		"CryptoTronUSDTContract":   "TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj",
		"CryptoBSCEnabled":         "true",
		"CryptoBSCReceiveAddress":  "",
		"CryptoBSCUSDTContract":    "0x55d398326f99059fF775485246999027B3197955",
	}
	LoadCryptoPaymentSettingsFromOptionMap()
	t.Cleanup(func() {
		common.OptionMap = original
		LoadCryptoPaymentSettingsFromOptionMap()
	})

	networks := GetEnabledCryptoPaymentNetworks()
	assert.Len(t, networks, 1)
	assert.Equal(t, "tron_trc20", networks[0].Network)
	assert.Equal(t, 20, networks[0].Confirmations)
	assert.Equal(t, 10, CryptoOrderExpireMinutes)
}

func TestCryptoPaymentConfigValidation(t *testing.T) {
	assert.True(t, IsValidTronAddress("TQ4mVnPz4jG4n4hD9QJf9U9gKfZVfUiH9z"))
	assert.False(t, IsValidTronAddress("0x1111111111111111111111111111111111111111"))
	assert.True(t, IsValidEVMAddress("0x55d398326f99059fF775485246999027B3197955"))
	assert.False(t, IsValidEVMAddress("TQ4mVnPz4jG4n4hD9QJf9U9gKfZVfUiH9z"))
}
```

- [ ] **Step 2: Run the failing settings tests**

Run:

```bash
go test ./setting -run 'TestGetCryptoPaymentNetworks|TestCryptoPaymentConfigValidation' -count=1
```

Expected: FAIL with undefined crypto setting functions and variables.

- [ ] **Step 3: Add crypto settings implementation**

Create `setting/payment_crypto.go`:

```go
package setting

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/ca0fgh/hermestoken/common"
)

var (
	CryptoPaymentEnabled     = false
	CryptoTronEnabled        = false
	CryptoTronReceiveAddress = ""
	CryptoTronUSDTContract   = "TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj"
	CryptoTronRPCURL         = ""
	CryptoTronAPIKey         = ""
	CryptoTronConfirmations  = 20
	CryptoBSCEnabled         = false
	CryptoBSCReceiveAddress  = ""
	CryptoBSCUSDTContract    = "0x55d398326f99059fF775485246999027B3197955"
	CryptoBSCRPCURL          = ""
	CryptoBSCConfirmations   = 15
	CryptoOrderExpireMinutes = 10
	CryptoUniqueSuffixMax    = 9999
	CryptoScannerEnabled     = true
)

type CryptoPaymentNetworkConfig struct {
	Network        string `json:"network"`
	DisplayName    string `json:"display_name"`
	Token          string `json:"token"`
	Contract       string `json:"contract"`
	ReceiveAddress string `json:"receive_address,omitempty"`
	Decimals       int    `json:"decimals"`
	Confirmations  int    `json:"confirmations"`
	MinTopUp       int    `json:"min_topup"`
}

var evmAddressPattern = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)
var tronAddressPattern = regexp.MustCompile(`^T[1-9A-HJ-NP-Za-km-z]{33}$`)

func IsValidEVMAddress(address string) bool {
	return evmAddressPattern.MatchString(strings.TrimSpace(address))
}

func IsValidTronAddress(address string) bool {
	return tronAddressPattern.MatchString(strings.TrimSpace(address))
}

func GetEnabledCryptoPaymentNetworks() []CryptoPaymentNetworkConfig {
	if !CryptoPaymentEnabled {
		return nil
	}
	networks := make([]CryptoPaymentNetworkConfig, 0, 2)
	if CryptoTronEnabled && IsValidTronAddress(CryptoTronReceiveAddress) && IsValidTronAddress(CryptoTronUSDTContract) {
		networks = append(networks, CryptoPaymentNetworkConfig{
			Network:        "tron_trc20",
			DisplayName:    "TRON TRC-20",
			Token:          "USDT",
			Contract:       strings.TrimSpace(CryptoTronUSDTContract),
			ReceiveAddress: strings.TrimSpace(CryptoTronReceiveAddress),
			Decimals:       6,
			Confirmations:  normalizedConfirmations(CryptoTronConfirmations, 20, 10),
			MinTopUp:       1,
		})
	}
	if CryptoBSCEnabled && IsValidEVMAddress(CryptoBSCReceiveAddress) && IsValidEVMAddress(CryptoBSCUSDTContract) {
		networks = append(networks, CryptoPaymentNetworkConfig{
			Network:        "bsc_erc20",
			DisplayName:    "BSC",
			Token:          "USDT",
			Contract:       strings.TrimSpace(CryptoBSCUSDTContract),
			ReceiveAddress: strings.TrimSpace(CryptoBSCReceiveAddress),
			Decimals:       18,
			Confirmations:  normalizedConfirmations(CryptoBSCConfirmations, 15, 8),
			MinTopUp:       1,
		})
	}
	return networks
}

func LoadCryptoPaymentSettingsFromOptionMap() {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	CryptoPaymentEnabled = optionBool("CryptoPaymentEnabled", false)
	CryptoTronEnabled = optionBool("CryptoTronEnabled", false)
	CryptoTronReceiveAddress = strings.TrimSpace(common.OptionMap["CryptoTronReceiveAddress"])
	CryptoTronUSDTContract = optionString("CryptoTronUSDTContract", "TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj")
	CryptoTronRPCURL = strings.TrimSpace(common.OptionMap["CryptoTronRPCURL"])
	CryptoTronAPIKey = strings.TrimSpace(common.OptionMap["CryptoTronAPIKey"])
	CryptoTronConfirmations = optionInt("CryptoTronConfirmations", 20)
	CryptoBSCEnabled = optionBool("CryptoBSCEnabled", false)
	CryptoBSCReceiveAddress = strings.TrimSpace(common.OptionMap["CryptoBSCReceiveAddress"])
	CryptoBSCUSDTContract = optionString("CryptoBSCUSDTContract", "0x55d398326f99059fF775485246999027B3197955")
	CryptoBSCRPCURL = strings.TrimSpace(common.OptionMap["CryptoBSCRPCURL"])
	CryptoBSCConfirmations = optionInt("CryptoBSCConfirmations", 15)
	CryptoOrderExpireMinutes = optionInt("CryptoOrderExpireMinutes", 10)
	CryptoUniqueSuffixMax = optionInt("CryptoUniqueSuffixMax", 9999)
	CryptoScannerEnabled = optionBool("CryptoScannerEnabled", true)
}

func optionString(key string, fallback string) string {
	value := strings.TrimSpace(common.OptionMap[key])
	if value == "" {
		return fallback
	}
	return value
}

func optionInt(key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(common.OptionMap[key]))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func optionBool(key string, fallback bool) bool {
	value := strings.TrimSpace(common.OptionMap[key])
	if value == "" {
		return fallback
	}
	return value == "true"
}

func normalizedConfirmations(value int, fallback int, minimum int) int {
	if value <= 0 {
		value = fallback
	}
	if value < minimum {
		return minimum
	}
	return value
}
```

- [ ] **Step 4: Wire option defaults and updates**

In `model/option.go`:

1. Add default OptionMap values in `InitOptionMap` near other payment settings:

```go
common.OptionMap["CryptoPaymentEnabled"] = "false"
common.OptionMap["CryptoTronEnabled"] = "false"
common.OptionMap["CryptoTronReceiveAddress"] = ""
common.OptionMap["CryptoTronUSDTContract"] = setting.CryptoTronUSDTContract
common.OptionMap["CryptoTronRPCURL"] = ""
common.OptionMap["CryptoTronAPIKey"] = ""
common.OptionMap["CryptoTronConfirmations"] = strconv.Itoa(setting.CryptoTronConfirmations)
common.OptionMap["CryptoBSCEnabled"] = "false"
common.OptionMap["CryptoBSCReceiveAddress"] = ""
common.OptionMap["CryptoBSCUSDTContract"] = setting.CryptoBSCUSDTContract
common.OptionMap["CryptoBSCRPCURL"] = ""
common.OptionMap["CryptoBSCConfirmations"] = strconv.Itoa(setting.CryptoBSCConfirmations)
common.OptionMap["CryptoOrderExpireMinutes"] = strconv.Itoa(setting.CryptoOrderExpireMinutes)
common.OptionMap["CryptoUniqueSuffixMax"] = strconv.Itoa(setting.CryptoUniqueSuffixMax)
common.OptionMap["CryptoScannerEnabled"] = strconv.FormatBool(setting.CryptoScannerEnabled)
```

2. Add update cases in `updateOptionMap` switch:

```go
case "CryptoPaymentEnabled":
	setting.CryptoPaymentEnabled = value == "true"
case "CryptoTronEnabled":
	setting.CryptoTronEnabled = value == "true"
case "CryptoTronReceiveAddress":
	setting.CryptoTronReceiveAddress = value
case "CryptoTronUSDTContract":
	setting.CryptoTronUSDTContract = value
case "CryptoTronRPCURL":
	setting.CryptoTronRPCURL = value
case "CryptoTronAPIKey":
	setting.CryptoTronAPIKey = value
case "CryptoTronConfirmations":
	setting.CryptoTronConfirmations, _ = strconv.Atoi(value)
case "CryptoBSCEnabled":
	setting.CryptoBSCEnabled = value == "true"
case "CryptoBSCReceiveAddress":
	setting.CryptoBSCReceiveAddress = value
case "CryptoBSCUSDTContract":
	setting.CryptoBSCUSDTContract = value
case "CryptoBSCRPCURL":
	setting.CryptoBSCRPCURL = value
case "CryptoBSCConfirmations":
	setting.CryptoBSCConfirmations, _ = strconv.Atoi(value)
case "CryptoOrderExpireMinutes":
	setting.CryptoOrderExpireMinutes, _ = strconv.Atoi(value)
case "CryptoUniqueSuffixMax":
	setting.CryptoUniqueSuffixMax, _ = strconv.Atoi(value)
case "CryptoScannerEnabled":
	setting.CryptoScannerEnabled = value == "true"
```

3. Call `setting.LoadCryptoPaymentSettingsFromOptionMap()` after options are initialized in `InitOptionMap`.

- [ ] **Step 5: Run settings tests**

Run:

```bash
go test ./setting -run 'TestGetCryptoPaymentNetworks|TestCryptoPaymentConfigValidation' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add setting/payment_crypto.go setting/payment_crypto_test.go model/option.go
git commit -m "feat: add crypto payment settings"
```

---

### Task 4: Create Crypto Top-up Orders

**Files:**
- Modify: `model/topup_crypto.go`
- Modify: `model/topup_crypto_test.go`

- [ ] **Step 1: Write failing order creation tests**

Append to `model/topup_crypto_test.go`:

```go
func TestCreateCryptoTopUpOrderCreatesTopUpAndCryptoOrder(t *testing.T) {
	truncateTables(t)
	common.QuotaPerUnit = 500000
	insertUserForPaymentGuardTest(t, 701, 0)

	input := CreateCryptoTopUpOrderInput{
		UserID:                701,
		Network:               CryptoNetworkTronTRC20,
		Amount:                10,
		ReceiveAddress:        "TQ4mVnPz4jG4n4hD9QJf9U9gKfZVfUiH9z",
		TokenContract:         "TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj",
		TokenDecimals:         6,
		RequiredConfirmations: 20,
		ExpireMinutes:         10,
		SuffixMax:             9999,
		Now:                   time.Unix(1000, 0),
		SuffixGenerator: func(max int) int {
			return 3721
		},
	}

	order, err := CreateCryptoTopUpOrder(input)
	require.NoError(t, err)
	assert.Equal(t, "10.003721", order.PayAmount)
	assert.Equal(t, "10003721", order.PayAmountBaseUnits)
	assert.Equal(t, CryptoPaymentStatusPending, order.Status)
	assert.Equal(t, int64(1600), order.ExpiresAt)

	topUp := GetTopUpByTradeNo(order.TradeNo)
	require.NotNil(t, topUp)
	assert.Equal(t, PaymentMethodCryptoUSDT, topUp.PaymentMethod)
	assert.Equal(t, "USDT", topUp.Currency)
	assert.Equal(t, int64(10), topUp.Amount)
	assert.Equal(t, 10.003721, topUp.Money)
}

func TestCreateCryptoTopUpOrderRejectsActiveAmountCollision(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 702, 0)

	input := CreateCryptoTopUpOrderInput{
		UserID:                702,
		Network:               CryptoNetworkTronTRC20,
		Amount:                10,
		ReceiveAddress:        "TQ4mVnPz4jG4n4hD9QJf9U9gKfZVfUiH9z",
		TokenContract:         "TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj",
		TokenDecimals:         6,
		RequiredConfirmations: 20,
		ExpireMinutes:         10,
		SuffixMax:             9999,
		Now:                   time.Unix(1000, 0),
		SuffixGenerator: func(max int) int {
			return 3721
		},
	}
	_, err := CreateCryptoTopUpOrder(input)
	require.NoError(t, err)
	_, err = CreateCryptoTopUpOrder(input)
	require.ErrorIs(t, err, ErrCryptoAmountCollision)
}
```

- [ ] **Step 2: Run order creation tests to verify failure**

Run:

```bash
go test ./model -run 'TestCreateCryptoTopUpOrder' -count=1
```

Expected: FAIL with undefined `CreateCryptoTopUpOrderInput` and `CreateCryptoTopUpOrder`.

- [ ] **Step 3: Implement order creation**

Append to `model/topup_crypto.go`:

```go
type CreateCryptoTopUpOrderInput struct {
	UserID                int
	Network               string
	Amount                int64
	ReceiveAddress        string
	TokenContract         string
	TokenDecimals         int
	RequiredConfirmations int
	ExpireMinutes         int
	SuffixMax             int
	Now                   time.Time
	SuffixGenerator       func(max int) int
}

func CreateCryptoTopUpOrder(input CreateCryptoTopUpOrderInput) (*CryptoPaymentOrder, error) {
	if input.UserID <= 0 || input.Amount <= 0 || input.TokenDecimals < 6 || strings.TrimSpace(input.ReceiveAddress) == "" || strings.TrimSpace(input.TokenContract) == "" {
		return nil, ErrCryptoInvalidAmount
	}
	if input.ExpireMinutes <= 0 {
		input.ExpireMinutes = 10
	}
	if input.RequiredConfirmations <= 0 {
		input.RequiredConfirmations = 20
	}
	if input.SuffixMax <= 0 || input.SuffixMax > 9999 {
		input.SuffixMax = 9999
	}
	if input.Now.IsZero() {
		input.Now = time.Now()
	}
	if input.SuffixGenerator == nil {
		input.SuffixGenerator = func(max int) int {
			return common.GetRandomInt(max) + 1
		}
	}

	var created CryptoPaymentOrder
	err := DB.Transaction(func(tx *gorm.DB) error {
		for attempt := 0; attempt < 20; attempt++ {
			suffix := input.SuffixGenerator(input.SuffixMax)
			payAmount, payBaseUnits, amountErr := CryptoPayAmountFromSuffix(decimal.NewFromInt(input.Amount), input.TokenDecimals, suffix)
			if amountErr != nil {
				return amountErr
			}

			var count int64
			if err := tx.Model(&CryptoPaymentOrder{}).
				Where("network = ? AND receive_address = ? AND pay_amount_base_units = ? AND expires_at >= ? AND status IN ?",
					NormalizeCryptoNetwork(input.Network), strings.TrimSpace(input.ReceiveAddress), payBaseUnits, input.Now.Unix(), []string{CryptoPaymentStatusPending, CryptoPaymentStatusDetected, CryptoPaymentStatusConfirmed}).
				Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				continue
			}

			tradeNo := fmt.Sprintf("CRYPTO-%d-%d-%s", input.UserID, input.Now.UnixMilli(), common.GetRandomString(6))
			payMoney, parseErr := decimal.NewFromString(payAmount)
			if parseErr != nil {
				return parseErr
			}
			topUp := &TopUp{
				UserId:        input.UserID,
				Amount:        input.Amount,
				Money:         payMoney.InexactFloat64(),
				TradeNo:       tradeNo,
				PaymentMethod: PaymentMethodCryptoUSDT,
				Currency:      CryptoTokenUSDT,
				CreateTime:    input.Now.Unix(),
				Status:        common.TopUpStatusPending,
			}
			if err := tx.Create(topUp).Error; err != nil {
				return err
			}

			created = CryptoPaymentOrder{
				TopUpId:               topUp.Id,
				TradeNo:               tradeNo,
				UserId:                input.UserID,
				Network:               NormalizeCryptoNetwork(input.Network),
				TokenSymbol:           CryptoTokenUSDT,
				TokenContract:         strings.TrimSpace(input.TokenContract),
				TokenDecimals:         input.TokenDecimals,
				ReceiveAddress:        strings.TrimSpace(input.ReceiveAddress),
				BaseAmount:            decimal.NewFromInt(input.Amount).StringFixed(6),
				PayAmount:             payAmount,
				PayAmountBaseUnits:    payBaseUnits,
				UniqueSuffix:          suffix,
				ExpiresAt:             input.Now.Add(time.Duration(input.ExpireMinutes) * time.Minute).Unix(),
				RequiredConfirmations: input.RequiredConfirmations,
				Status:                CryptoPaymentStatusPending,
				MatchedLogIndex:       -1,
				CreateTime:            input.Now.Unix(),
				UpdateTime:            input.Now.Unix(),
			}
			return tx.Create(&created).Error
		}
		return ErrCryptoAmountCollision
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}
```

Add imports to `model/topup_crypto.go`:

```go
"gorm.io/gorm"
```

- [ ] **Step 4: Run order creation tests**

Run:

```bash
go test ./model -run 'TestCreateCryptoTopUpOrder' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add model/topup_crypto.go model/topup_crypto_test.go
git commit -m "feat: create crypto topup orders"
```

---

### Task 5: Implement User Crypto Top-up APIs

**Files:**
- Create: `controller/topup_crypto.go`
- Create: `controller/topup_crypto_test.go`
- Modify: `router/api-router.go`
- Modify: `controller/topup.go`

- [ ] **Step 1: Write controller tests for config and order creation**

Create `controller/topup_crypto_test.go`:

```go
package controller

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupCryptoControllerTest(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	model.InitColumnMetadata()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.TopUp{},
		&model.Log{},
		&model.CryptoPaymentOrder{},
		&model.CryptoPaymentTransaction{},
		&model.CryptoScannerState{},
	))
	require.NoError(t, db.Create(&model.User{
		Id:       801,
		Username: "crypto_controller_user",
		Password: "password123",
		Status:   common.UserStatusEnabled,
		Quota:    0,
		Group:    "default",
	}).Error)

	setting.CryptoPaymentEnabled = true
	setting.CryptoTronEnabled = true
	setting.CryptoTronReceiveAddress = "TQ4mVnPz4jG4n4hD9QJf9U9gKfZVfUiH9z"
	setting.CryptoTronUSDTContract = "TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj"
	setting.CryptoTronConfirmations = 20
	setting.CryptoOrderExpireMinutes = 10
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

func TestGetCryptoTopUpConfig(t *testing.T) {
	setupCryptoControllerTest(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("id", 801)

	GetCryptoTopUpConfig(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "tron_trc20")
	assert.Contains(t, w.Body.String(), "USDT")
}

func TestCreateCryptoTopUpOrder(t *testing.T) {
	setupCryptoControllerTest(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("id", 801)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/crypto/topup/order", bytes.NewReader([]byte(`{"network":"tron_trc20","amount":10}`)))
	c.Request.Header.Set("Content-Type", "application/json")

	CreateCryptoTopUpOrder(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "pay_amount")
	assert.Contains(t, w.Body.String(), setting.CryptoTronReceiveAddress)
	assert.Contains(t, w.Body.String(), "pending")
}

func TestCreateCryptoTopUpOrderRejectsDisabledNetwork(t *testing.T) {
	setupCryptoControllerTest(t)
	setting.CryptoTronEnabled = false
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("id", 801)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/crypto/topup/order", bytes.NewReader([]byte(`{"network":"tron_trc20","amount":10}`)))
	c.Request.Header.Set("Content-Type", "application/json")

	CreateCryptoTopUpOrder(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "不可用")
}

func TestGetCryptoTopUpOrderExpiresPendingOrder(t *testing.T) {
	setupCryptoControllerTest(t)
	order, err := model.CreateCryptoTopUpOrder(model.CreateCryptoTopUpOrderInput{
		UserID:                801,
		Network:               model.CryptoNetworkTronTRC20,
		Amount:                10,
		ReceiveAddress:        setting.CryptoTronReceiveAddress,
		TokenContract:         setting.CryptoTronUSDTContract,
		TokenDecimals:         6,
		RequiredConfirmations: 20,
		ExpireMinutes:         10,
		SuffixMax:             9999,
	})
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&model.CryptoPaymentOrder{}).Where("id = ?", order.Id).Update("expires_at", int64(1)).Error)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("id", 801)
	c.Params = gin.Params{{Key: "trade_no", Value: order.TradeNo}}

	GetCryptoTopUpOrder(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "expired")
}
```

Keep controller test DB setup local to the controller package so it does not depend on unexported model test helpers.


- [ ] **Step 2: Run controller tests to verify failure**

Run:

```bash
go test ./controller -run 'TestGetCryptoTopUpConfig|TestCreateCryptoTopUpOrder|TestGetCryptoTopUpOrder' -count=1
```

Expected: FAIL with undefined crypto controller functions.

- [ ] **Step 3: Implement crypto controller**

Create `controller/topup_crypto.go`:

```go
package controller

import (
	"net/http"
	"strings"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
	"github.com/gin-gonic/gin"
)

type cryptoTopUpOrderRequest struct {
	Network string `json:"network"`
	Amount  int64  `json:"amount"`
}

func GetCryptoTopUpConfig(c *gin.Context) {
	networks := setting.GetEnabledCryptoPaymentNetworks()
	common.ApiSuccess(c, gin.H{
		"enabled":        setting.CryptoPaymentEnabled && len(networks) > 0,
		"networks":       networks,
		"expire_minutes": setting.CryptoOrderExpireMinutes,
	})
}

func CreateCryptoTopUpOrder(c *gin.Context) {
	var req cryptoTopUpOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if req.Amount <= 0 {
		common.ApiErrorMsg(c, "充值金额无效")
		return
	}

	config, ok := resolveCryptoNetworkConfig(req.Network)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"message": "error", "data": "USDT 网络不可用"})
		return
	}

	order, err := model.CreateCryptoTopUpOrder(model.CreateCryptoTopUpOrderInput{
		UserID:                c.GetInt("id"),
		Network:               config.Network,
		Amount:                req.Amount,
		ReceiveAddress:        config.ReceiveAddress,
		TokenContract:         config.Contract,
		TokenDecimals:         config.Decimals,
		RequiredConfirmations: config.Confirmations,
		ExpireMinutes:         setting.CryptoOrderExpireMinutes,
		SuffixMax:             setting.CryptoUniqueSuffixMax,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, cryptoOrderResponse(order))
}

func GetCryptoTopUpOrder(c *gin.Context) {
	tradeNo := strings.TrimSpace(c.Param("trade_no"))
	order := model.GetCryptoPaymentOrderByTradeNo(tradeNo)
	if order == nil || order.UserId != c.GetInt("id") {
		common.ApiErrorMsg(c, "订单不存在")
		return
	}
	var err error
	order, err = model.ExpireCryptoPaymentOrderIfNeeded(order, time.Now())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, cryptoOrderResponse(order))
}

func resolveCryptoNetworkConfig(network string) (setting.CryptoPaymentNetworkConfig, bool) {
	for _, cfg := range setting.GetEnabledCryptoPaymentNetworks() {
		if cfg.Network == model.NormalizeCryptoNetwork(network) {
			return cfg, true
		}
	}
	return setting.CryptoPaymentNetworkConfig{}, false
}

func cryptoOrderResponse(order *model.CryptoPaymentOrder) gin.H {
	return gin.H{
		"trade_no":               order.TradeNo,
		"network":                order.Network,
		"token":                  order.TokenSymbol,
		"receive_address":        order.ReceiveAddress,
		"base_amount":            order.BaseAmount,
		"pay_amount":             order.PayAmount,
		"expires_at":             order.ExpiresAt,
		"required_confirmations": order.RequiredConfirmations,
		"status":                 order.Status,
		"tx_hash":                order.MatchedTxHash,
		"confirmations":          model.GetCryptoOrderConfirmations(order.Id),
	}
}
```

Add supporting functions to `model/topup_crypto.go`:

```go
func GetCryptoPaymentOrderByTradeNo(tradeNo string) *CryptoPaymentOrder {
	if strings.TrimSpace(tradeNo) == "" {
		return nil
	}
	var order CryptoPaymentOrder
	if err := DB.Where("trade_no = ?", strings.TrimSpace(tradeNo)).First(&order).Error; err != nil {
		return nil
	}
	return &order
}

func GetCryptoOrderConfirmations(orderID int) int64 {
	if orderID <= 0 {
		return 0
	}
	var tx CryptoPaymentTransaction
	if err := DB.Where("matched_order_id = ?", orderID).Order("id desc").First(&tx).Error; err != nil {
		return 0
	}
	return tx.Confirmations
}

func ExpireCryptoPaymentOrderIfNeeded(order *CryptoPaymentOrder, now time.Time) (*CryptoPaymentOrder, error) {
	if order == nil || order.Status != CryptoPaymentStatusPending || !order.IsExpired(now) {
		return order, nil
	}
	order.Status = CryptoPaymentStatusExpired
	order.UpdateTime = now.Unix()
	if err := DB.Save(order).Error; err != nil {
		return nil, err
	}
	return order, nil
}
```

- [ ] **Step 4: Register routes and top-up info**

In `router/api-router.go`, inside authenticated `selfRoute`, add:

```go
selfRoute.GET("/crypto/topup/config", controller.GetCryptoTopUpConfig)
selfRoute.POST("/crypto/topup/order", middleware.CriticalRateLimit(), controller.CreateCryptoTopUpOrder)
selfRoute.GET("/crypto/topup/order/:trade_no", controller.GetCryptoTopUpOrder)
```

In `controller/topup.go`, add to `GetTopUpInfo` data:

```go
"enable_crypto_usdt_topup": setting.CryptoPaymentEnabled && len(setting.GetEnabledCryptoPaymentNetworks()) > 0,
"crypto_networks":          setting.GetEnabledCryptoPaymentNetworks(),
```

- [ ] **Step 5: Run controller tests**

Run:

```bash
go test ./controller -run 'TestGetCryptoTopUpConfig|TestCreateCryptoTopUpOrder|TestGetCryptoTopUpOrder' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add controller/topup_crypto.go controller/topup_crypto_test.go router/api-router.go controller/topup.go model/topup_crypto.go
git commit -m "feat: add crypto topup user APIs"
```

---

### Task 6: Implement Idempotent Crypto Completion

**Files:**
- Modify: `model/topup_crypto.go`
- Modify: `model/topup_crypto_test.go`
- Modify: `model/topup.go`

- [ ] **Step 1: Write failing completion tests**

Append to `model/topup_crypto_test.go`:

```go
func TestCompleteCryptoTopUpCreditsQuotaOnce(t *testing.T) {
	truncateTables(t)
	common.QuotaPerUnit = 500000
	insertUserForPaymentGuardTest(t, 901, 0)
	order := seedCryptoOrderForCompletion(t, 901, 10, "10003721")
	evidence := CryptoTxEvidence{
		Network:          order.Network,
		TxHash:           "0xcomplete1",
		LogIndex:         0,
		ToAddress:        order.ReceiveAddress,
		TokenContract:    order.TokenContract,
		AmountBaseUnits:  order.PayAmountBaseUnits,
		Confirmations:    int64(order.RequiredConfirmations),
		BlockNumber:      100,
		BlockTimestamp:   time.Now().Unix(),
		FromAddress:      "sender",
		RawPayload:       `{"ok":true}`,
	}

	require.NoError(t, CompleteCryptoTopUp(order.TradeNo, evidence))
	require.NoError(t, CompleteCryptoTopUp(order.TradeNo, evidence))
	assert.Equal(t, 5000000, getUserQuotaForPaymentGuardTest(t, 901))
	assert.Equal(t, common.TopUpStatusSuccess, getTopUpStatusForPaymentGuardTest(t, order.TradeNo))
}

func TestCompleteCryptoTopUpRejectsAmountMismatch(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 902, 0)
	order := seedCryptoOrderForCompletion(t, 902, 10, "10003721")
	err := CompleteCryptoTopUp(order.TradeNo, CryptoTxEvidence{
		Network:         order.Network,
		TxHash:          "0xmismatch",
		LogIndex:        0,
		ToAddress:       order.ReceiveAddress,
		TokenContract:   order.TokenContract,
		AmountBaseUnits: "10000000",
		Confirmations:   int64(order.RequiredConfirmations),
	})
	require.ErrorIs(t, err, ErrCryptoTransactionMismatch)
	assert.Equal(t, 0, getUserQuotaForPaymentGuardTest(t, 902))
}

func TestCompleteCryptoTopUpRejectsReusedTransaction(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 903, 0)
	insertUserForPaymentGuardTest(t, 904, 0)
	firstOrder := seedCryptoOrderForCompletion(t, 903, 10, "10003721")
	secondOrder := seedCryptoOrderForCompletion(t, 904, 10, "10003721")
	require.NoError(t, DB.Create(&CryptoPaymentTransaction{
		Network:         firstOrder.Network,
		TxHash:          "0xreused",
		LogIndex:        0,
		ToAddress:       firstOrder.ReceiveAddress,
		TokenContract:   firstOrder.TokenContract,
		TokenSymbol:     CryptoTokenUSDT,
		TokenDecimals:   firstOrder.TokenDecimals,
		Amount:          firstOrder.PayAmount,
		AmountBaseUnits: firstOrder.PayAmountBaseUnits,
		Confirmations:   20,
		Status:          CryptoTransactionStatusConfirmed,
		MatchedOrderId:  firstOrder.Id,
		CreateTime:      time.Now().Unix(),
		UpdateTime:      time.Now().Unix(),
	}).Error)

	err := CompleteCryptoTopUp(secondOrder.TradeNo, CryptoTxEvidence{
		Network:         secondOrder.Network,
		TxHash:          "0xreused",
		LogIndex:        0,
		ToAddress:       secondOrder.ReceiveAddress,
		TokenContract:   secondOrder.TokenContract,
		AmountBaseUnits: secondOrder.PayAmountBaseUnits,
		Confirmations:   int64(secondOrder.RequiredConfirmations),
	})
	require.ErrorIs(t, err, ErrCryptoTransactionMismatch)
	assert.Equal(t, 0, getUserQuotaForPaymentGuardTest(t, 904))
}
```

Add helper in the same test file:

```go
func seedCryptoOrderForCompletion(t *testing.T, userID int, amount int64, payUnits string) *CryptoPaymentOrder {
	t.Helper()
	topUp := &TopUp{
		UserId:        userID,
		Amount:        amount,
		Money:         10.003721,
		TradeNo:       "crypto-complete-" + time.Now().Format("150405.000000000"),
		PaymentMethod: PaymentMethodCryptoUSDT,
		Currency:      CryptoTokenUSDT,
		Status:        common.TopUpStatusPending,
		CreateTime:    time.Now().Unix(),
	}
	require.NoError(t, DB.Create(topUp).Error)
	order := &CryptoPaymentOrder{
		TopUpId:               topUp.Id,
		TradeNo:               topUp.TradeNo,
		UserId:                userID,
		Network:               CryptoNetworkTronTRC20,
		TokenSymbol:           CryptoTokenUSDT,
		TokenContract:         "TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj",
		TokenDecimals:         6,
		ReceiveAddress:        "TQ4mVnPz4jG4n4hD9QJf9U9gKfZVfUiH9z",
		BaseAmount:            "10.000000",
		PayAmount:             "10.003721",
		PayAmountBaseUnits:    payUnits,
		UniqueSuffix:          3721,
		ExpiresAt:             time.Now().Add(10 * time.Minute).Unix(),
		RequiredConfirmations: 20,
		Status:                CryptoPaymentStatusConfirmed,
		MatchedLogIndex:       -1,
		CreateTime:            time.Now().Unix(),
		UpdateTime:            time.Now().Unix(),
	}
	require.NoError(t, DB.Create(order).Error)
	return order
}
```

- [ ] **Step 2: Run completion tests to verify failure**

Run:

```bash
go test ./model -run 'TestCompleteCryptoTopUp' -count=1
```

Expected: FAIL with undefined `CryptoTxEvidence` and `CompleteCryptoTopUp`.

- [ ] **Step 3: Implement completion transaction**

Append to `model/topup_crypto.go`:

```go
type CryptoTxEvidence struct {
	Network          string
	TxHash           string
	LogIndex         int
	BlockNumber      int64
	BlockTimestamp   int64
	FromAddress      string
	ToAddress        string
	TokenContract    string
	AmountBaseUnits  string
	Confirmations    int64
	RawPayload       string
}

func CompleteCryptoTopUp(tradeNo string, evidence CryptoTxEvidence) error {
	if strings.TrimSpace(tradeNo) == "" {
		return ErrCryptoOrderNotFound
	}
	var quotaToAdd int64
	var completedOrder CryptoPaymentOrder
	var completedTopUp TopUp
	now := cryptoNow()

	err := DB.Transaction(func(tx *gorm.DB) error {
		var order CryptoPaymentOrder
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(cryptoRefCol("trade_no")+" = ?", tradeNo).First(&order).Error; err != nil {
			return ErrCryptoOrderNotFound
		}
		if order.Status == CryptoPaymentStatusSuccess {
			completedOrder = order
			return nil
		}
		if evidence.Confirmations < int64(order.RequiredConfirmations) {
			return ErrCryptoOrderStatusInvalid
		}
		if !cryptoEvidenceMatchesOrder(&order, evidence) {
			return ErrCryptoTransactionMismatch
		}

		var topUp TopUp
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("id = ?", order.TopUpId).First(&topUp).Error; err != nil {
			return err
		}
		if topUp.Status == common.TopUpStatusSuccess {
			order.Status = CryptoPaymentStatusSuccess
			order.CompletedAt = now
			order.UpdateTime = now
			return tx.Save(&order).Error
		}
		if topUp.Status != common.TopUpStatusPending || topUp.PaymentMethod != PaymentMethodCryptoUSDT {
			return ErrTopUpStatusInvalid
		}

		quotaToAdd = quotaFromStandardTopUpAmount(topUp.Amount)
		if quotaToAdd <= 0 {
			return ErrCryptoInvalidAmount
		}

		txRecord := CryptoPaymentTransaction{
			Network:         order.Network,
			TxHash:          evidence.TxHash,
			LogIndex:        evidence.LogIndex,
			BlockNumber:     evidence.BlockNumber,
			BlockTimestamp:  evidence.BlockTimestamp,
			FromAddress:     evidence.FromAddress,
			ToAddress:       evidence.ToAddress,
			TokenContract:   evidence.TokenContract,
			TokenSymbol:     CryptoTokenUSDT,
			TokenDecimals:   order.TokenDecimals,
			Amount:          order.PayAmount,
			AmountBaseUnits: evidence.AmountBaseUnits,
			Confirmations:   evidence.Confirmations,
			Status:          CryptoTransactionStatusConfirmed,
			MatchedOrderId:  order.Id,
			RawPayload:      evidence.RawPayload,
			CreateTime:      now,
			UpdateTime:      now,
		}
		var existingTx CryptoPaymentTransaction
		findErr := tx.Where("network = ? AND tx_hash = ? AND log_index = ?", txRecord.Network, txRecord.TxHash, txRecord.LogIndex).First(&existingTx).Error
		if findErr == nil {
			if existingTx.MatchedOrderId != 0 && existingTx.MatchedOrderId != order.Id {
				return ErrCryptoTransactionMismatch
			}
			existingTx.MatchedOrderId = order.Id
			existingTx.Status = CryptoTransactionStatusConfirmed
			existingTx.Confirmations = evidence.Confirmations
			existingTx.UpdateTime = now
			if err := tx.Save(&existingTx).Error; err != nil {
				return err
			}
		} else if errors.Is(findErr, gorm.ErrRecordNotFound) {
			if err := tx.Create(&txRecord).Error; err != nil {
				return err
			}
		} else {
			return findErr
		}

		topUp.Status = common.TopUpStatusSuccess
		topUp.CompleteTime = now
		if err := tx.Save(&topUp).Error; err != nil {
			return err
		}
		if err := tx.Model(&User{}).Where("id = ?", topUp.UserId).Update("quota", gorm.Expr("quota + ?", quotaToAdd)).Error; err != nil {
			return err
		}
		order.Status = CryptoPaymentStatusSuccess
		order.MatchedTxHash = evidence.TxHash
		order.MatchedLogIndex = evidence.LogIndex
		order.ConfirmedAt = now
		order.CompletedAt = now
		order.UpdateTime = now
		if err := tx.Save(&order).Error; err != nil {
			return err
		}
		completedOrder = order
		completedTopUp = topUp
		return nil
	})
	if err != nil {
		return err
	}
	if quotaToAdd > 0 {
		RecordLog(completedTopUp.UserId, LogTypeTopup, fmt.Sprintf("USDT充值成功，网络: %s，充值额度: %v，支付金额: %s USDT", completedOrder.Network, quotaToAdd, completedOrder.PayAmount))
	}
	return nil
}

func cryptoEvidenceMatchesOrder(order *CryptoPaymentOrder, evidence CryptoTxEvidence) bool {
	if order == nil {
		return false
	}
	return NormalizeCryptoNetwork(evidence.Network) == order.Network &&
		strings.EqualFold(strings.TrimSpace(evidence.TokenContract), strings.TrimSpace(order.TokenContract)) &&
		strings.EqualFold(strings.TrimSpace(evidence.ToAddress), strings.TrimSpace(order.ReceiveAddress)) &&
		strings.TrimSpace(evidence.AmountBaseUnits) == order.PayAmountBaseUnits &&
		strings.TrimSpace(evidence.TxHash) != "" &&
		evidence.LogIndex >= 0
}
```

- [ ] **Step 4: Protect manual completion path from crypto orders**

In `model/topup.go`, inside `ManualCompleteTopUp`, after pending status check add:

```go
if topUp.PaymentMethod == PaymentMethodCryptoUSDT {
	return errors.New("USDT 充值订单必须通过链上交易证据补单")
}
```

- [ ] **Step 5: Run completion tests**

Run:

```bash
go test ./model -run 'TestCompleteCryptoTopUp|TestRechargeWaffoPancake_RejectsMismatchedPaymentMethod' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add model/topup_crypto.go model/topup_crypto_test.go model/topup.go
git commit -m "feat: complete crypto topups idempotently"
```

---

### Task 7: Add Transaction Matching and Scanner State

**Files:**
- Modify: `model/topup_crypto.go`
- Modify: `model/topup_crypto_test.go`

- [ ] **Step 1: Write failing matching and scanner state tests**

Append to `model/topup_crypto_test.go`:

```go
func TestRecordCryptoTransferMatchesPendingOrder(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 1001, 0)
	order := seedCryptoOrderForCompletion(t, 1001, 10, "10003721")
	order.Status = CryptoPaymentStatusPending
	require.NoError(t, DB.Save(order).Error)

	tx, matched, err := RecordCryptoTransfer(CryptoObservedTransfer{
		Network:         order.Network,
		TxHash:          "0xseen1",
		LogIndex:        0,
		BlockNumber:     123,
		FromAddress:     "sender",
		ToAddress:       order.ReceiveAddress,
		TokenContract:   order.TokenContract,
		TokenDecimals:   order.TokenDecimals,
		Amount:          order.PayAmount,
		AmountBaseUnits: order.PayAmountBaseUnits,
		Confirmations:   1,
		ObservedAt:       time.Now(),
	})
	require.NoError(t, err)
	require.NotNil(t, tx)
	require.NotNil(t, matched)
	assert.Equal(t, order.Id, matched.Id)
	assert.Equal(t, CryptoPaymentStatusDetected, GetCryptoPaymentOrderByTradeNo(order.TradeNo).Status)
}

func TestRecordCryptoTransferMarksLatePaidForExpiredExactAmount(t *testing.T) {
	truncateTables(t)
	insertUserForPaymentGuardTest(t, 1002, 0)
	order := seedCryptoOrderForCompletion(t, 1002, 10, "10003721")
	order.Status = CryptoPaymentStatusExpired
	order.ExpiresAt = time.Now().Add(-time.Minute).Unix()
	require.NoError(t, DB.Save(order).Error)

	_, matched, err := RecordCryptoTransfer(CryptoObservedTransfer{
		Network:         order.Network,
		TxHash:          "0xlate1",
		LogIndex:        0,
		BlockNumber:     124,
		ToAddress:       order.ReceiveAddress,
		TokenContract:   order.TokenContract,
		TokenDecimals:   order.TokenDecimals,
		Amount:          order.PayAmount,
		AmountBaseUnits: order.PayAmountBaseUnits,
		Confirmations:   1,
		ObservedAt:       time.Now(),
	})
	require.NoError(t, err)
	require.NotNil(t, matched)
	assert.Equal(t, CryptoPaymentStatusLatePaid, GetCryptoPaymentOrderByTradeNo(order.TradeNo).Status)
}

func TestCryptoScannerStateUpsert(t *testing.T) {
	truncateTables(t)
	require.NoError(t, UpsertCryptoScannerState(CryptoNetworkBSCERC20, 100, 85))
	state, err := GetCryptoScannerState(CryptoNetworkBSCERC20)
	require.NoError(t, err)
	assert.EqualValues(t, 100, state.LastScannedBlock)
	assert.EqualValues(t, 85, state.LastFinalizedBlock)
}
```

- [ ] **Step 2: Run matching tests to verify failure**

Run:

```bash
go test ./model -run 'TestRecordCryptoTransfer|TestCryptoScannerStateUpsert' -count=1
```

Expected: FAIL with undefined transfer and scanner state functions.

- [ ] **Step 3: Implement observed transfer matching**

Append to `model/topup_crypto.go`:

```go
type CryptoObservedTransfer struct {
	Network         string
	TxHash          string
	LogIndex        int
	BlockNumber     int64
	BlockTimestamp  int64
	FromAddress     string
	ToAddress       string
	TokenContract   string
	TokenDecimals   int
	Amount          string
	AmountBaseUnits string
	Confirmations   int64
	RawPayload      string
	ObservedAt      time.Time
}

func RecordCryptoTransfer(transfer CryptoObservedTransfer) (*CryptoPaymentTransaction, *CryptoPaymentOrder, error) {
	if transfer.ObservedAt.IsZero() {
		transfer.ObservedAt = time.Now()
	}
	var savedTx CryptoPaymentTransaction
	var matchedOrder *CryptoPaymentOrder
	err := DB.Transaction(func(tx *gorm.DB) error {
		txRecord := CryptoPaymentTransaction{
			Network:         NormalizeCryptoNetwork(transfer.Network),
			TxHash:          strings.TrimSpace(transfer.TxHash),
			LogIndex:        transfer.LogIndex,
			BlockNumber:     transfer.BlockNumber,
			BlockTimestamp:  transfer.BlockTimestamp,
			FromAddress:     strings.TrimSpace(transfer.FromAddress),
			ToAddress:       strings.TrimSpace(transfer.ToAddress),
			TokenContract:   strings.TrimSpace(transfer.TokenContract),
			TokenSymbol:     CryptoTokenUSDT,
			TokenDecimals:   transfer.TokenDecimals,
			Amount:          transfer.Amount,
			AmountBaseUnits: transfer.AmountBaseUnits,
			Confirmations:   transfer.Confirmations,
			Status:          CryptoTransactionStatusSeen,
			RawPayload:      transfer.RawPayload,
			CreateTime:      transfer.ObservedAt.Unix(),
			UpdateTime:      transfer.ObservedAt.Unix(),
		}
		if err := tx.Where("network = ? AND tx_hash = ? AND log_index = ?", txRecord.Network, txRecord.TxHash, txRecord.LogIndex).FirstOrCreate(&savedTx, txRecord).Error; err != nil {
			return err
		}

		var orders []CryptoPaymentOrder
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(
			"network = ? AND receive_address = ? AND token_contract = ? AND pay_amount_base_units = ? AND expires_at >= ? AND status = ?",
			txRecord.Network, txRecord.ToAddress, txRecord.TokenContract, txRecord.AmountBaseUnits, transfer.ObservedAt.Unix(), CryptoPaymentStatusPending,
		).Find(&orders).Error; err != nil {
			return err
		}
		if len(orders) == 1 {
			order := orders[0]
			order.Status = CryptoPaymentStatusDetected
			order.MatchedTxHash = txRecord.TxHash
			order.MatchedLogIndex = txRecord.LogIndex
			order.DetectedAt = transfer.ObservedAt.Unix()
			order.UpdateTime = transfer.ObservedAt.Unix()
			if err := tx.Save(&order).Error; err != nil {
				return err
			}
			savedTx.MatchedOrderId = order.Id
			if err := tx.Save(&savedTx).Error; err != nil {
				return err
			}
			matchedOrder = &order
			return nil
		}
		if len(orders) > 1 {
			for _, order := range orders {
				if err := tx.Model(&CryptoPaymentOrder{}).Where("id = ?", order.Id).Updates(map[string]any{"status": CryptoPaymentStatusAmbiguous, "update_time": transfer.ObservedAt.Unix()}).Error; err != nil {
					return err
				}
			}
			return nil
		}

		var expired CryptoPaymentOrder
		if err := tx.Set("gorm:query_option", "FOR UPDATE").Where(
			"network = ? AND receive_address = ? AND token_contract = ? AND pay_amount_base_units = ? AND expires_at < ? AND status IN ?",
			txRecord.Network, txRecord.ToAddress, txRecord.TokenContract, txRecord.AmountBaseUnits, transfer.ObservedAt.Unix(), []string{CryptoPaymentStatusPending, CryptoPaymentStatusExpired},
		).Order("expires_at desc").First(&expired).Error; err == nil {
			expired.Status = CryptoPaymentStatusLatePaid
			expired.MatchedTxHash = txRecord.TxHash
			expired.MatchedLogIndex = txRecord.LogIndex
			expired.UpdateTime = transfer.ObservedAt.Unix()
			if err := tx.Save(&expired).Error; err != nil {
				return err
			}
			matchedOrder = &expired
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return &savedTx, matchedOrder, nil
}
```

- [ ] **Step 4: Implement scanner state helpers**

Append to `model/topup_crypto.go`:

```go
func GetCryptoScannerState(network string) (*CryptoScannerState, error) {
	var state CryptoScannerState
	if err := DB.Where("network = ?", NormalizeCryptoNetwork(network)).First(&state).Error; err != nil {
		return nil, err
	}
	return &state, nil
}

func UpsertCryptoScannerState(network string, lastScannedBlock int64, lastFinalizedBlock int64) error {
	state := CryptoScannerState{
		Network:            NormalizeCryptoNetwork(network),
		LastScannedBlock:   lastScannedBlock,
		LastFinalizedBlock: lastFinalizedBlock,
		UpdatedAt:          cryptoNow(),
	}
	return DB.Save(&state).Error
}
```

- [ ] **Step 5: Run matching and state tests**

Run:

```bash
go test ./model -run 'TestRecordCryptoTransfer|TestCryptoScannerStateUpsert' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add model/topup_crypto.go model/topup_crypto_test.go
git commit -m "feat: match crypto transfers to orders"
```

---

### Task 8: Add Scanner Lock and Common Scanner Loop

**Files:**
- Create: `service/crypto_payment/lock.go`
- Create: `service/crypto_payment/scanner.go`
- Create: `service/crypto_payment/lock_test.go`

- [ ] **Step 1: Write failing lock tests**

Create `service/crypto_payment/lock_test.go`:

```go
package crypto_payment

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeLockStore struct {
	owner string
}

func (s *fakeLockStore) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	if s.owner != "" {
		return false, nil
	}
	s.owner = value
	return true, nil
}

func (s *fakeLockStore) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	if len(args) >= 1 && s.owner == args[0].(string) {
		return int64(1), nil
	}
	return int64(0), nil
}

func TestScannerLockAcquireAndRenew(t *testing.T) {
	store := &fakeLockStore{}
	lock := NewScannerLock(store, "crypto:scanner:test", "owner-a", 30*time.Second)
	acquired, err := lock.Acquire(context.Background())
	require.NoError(t, err)
	assert.True(t, acquired)
	renewed, err := lock.Renew(context.Background())
	require.NoError(t, err)
	assert.True(t, renewed)
}

func TestScannerLockRejectsSecondOwner(t *testing.T) {
	store := &fakeLockStore{}
	first := NewScannerLock(store, "crypto:scanner:test", "owner-a", 30*time.Second)
	second := NewScannerLock(store, "crypto:scanner:test", "owner-b", 30*time.Second)
	acquired, err := first.Acquire(context.Background())
	require.NoError(t, err)
	assert.True(t, acquired)
	acquired, err = second.Acquire(context.Background())
	require.NoError(t, err)
	assert.False(t, acquired)
}
```

- [ ] **Step 2: Run lock tests to verify failure**

Run:

```bash
go test ./service/crypto_payment -run TestScannerLock -count=1
```

Expected: FAIL because scanner lock package does not exist.

- [ ] **Step 3: Implement scanner lock**

Create `service/crypto_payment/lock.go`:

```go
package crypto_payment

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

type LockStore interface {
	SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error)
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error)
}

type ScannerLock struct {
	store LockStore
	key   string
	owner string
	ttl   time.Duration
}

type redisLockStore struct {
	client *redis.Client
}

func NewRedisLockStore(client *redis.Client) LockStore {
	return &redisLockStore{client: client}
}

func (s *redisLockStore) SetNX(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	return s.client.SetNX(ctx, key, value, ttl).Result()
}

func (s *redisLockStore) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	return s.client.Eval(ctx, script, keys, args...).Result()
}

func NewScannerLock(store LockStore, key string, owner string, ttl time.Duration) *ScannerLock {
	return &ScannerLock{store: store, key: key, owner: owner, ttl: ttl}
}

func (l *ScannerLock) Acquire(ctx context.Context) (bool, error) {
	return l.store.SetNX(ctx, l.key, l.owner, l.ttl)
}

func (l *ScannerLock) Renew(ctx context.Context) (bool, error) {
	result, err := l.store.Eval(ctx, renewLockScript, []string{l.key}, l.owner, int(l.ttl.Milliseconds()))
	if err != nil {
		return false, err
	}
	switch value := result.(type) {
	case int64:
		return value == 1, nil
	case int:
		return value == 1, nil
	default:
		return false, nil
	}
}

const renewLockScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
  redis.call("PEXPIRE", KEYS[1], ARGV[2])
  return 1
end
return 0
`
```

- [ ] **Step 4: Implement common scanner loop skeleton**

Create `service/crypto_payment/scanner.go`:

```go
package crypto_payment

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
)

type NetworkScanner interface {
	Network() string
	ScanOnce(ctx context.Context) error
}

func StartCryptoPaymentScanners() {
	if !setting.CryptoScannerEnabled || !setting.CryptoPaymentEnabled {
		return
	}
	owner := fmt.Sprintf("%s-%d", common.GetRandomString(8), os.Getpid())
	for _, scanner := range BuildConfiguredScanners() {
		go runScannerLoop(context.Background(), scanner, owner)
	}
}

func BuildConfiguredScanners() []NetworkScanner {
	scanners := make([]NetworkScanner, 0, 2)
	for _, network := range setting.GetEnabledCryptoPaymentNetworks() {
		switch network.Network {
		case model.CryptoNetworkBSCERC20:
			scanners = append(scanners, NewBSCScanner(network))
		case model.CryptoNetworkTronTRC20:
			scanners = append(scanners, NewTronScanner(network))
		}
	}
	return scanners
}

func runScannerLoop(ctx context.Context, scanner NetworkScanner, owner string) {
	if !common.RedisEnabled || common.RDB == nil {
		common.SysLog("crypto scanner paused because Redis is not enabled")
		return
	}
	lock := NewScannerLock(NewRedisLockStore(common.RDB), "crypto:scanner:"+scanner.Network(), owner, 30*time.Second)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	ownsLock := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var err error
			if ownsLock {
				ownsLock, err = lock.Renew(ctx)
			} else {
				ownsLock, err = lock.Acquire(ctx)
			}
			if err != nil || !ownsLock {
				continue
			}
			if err := scanner.ScanOnce(ctx); err != nil {
				common.SysLog("crypto scanner error: " + err.Error())
			}
			_, _ = lock.Renew(ctx)
		}
	}
}
```

- [ ] **Step 5: Run scanner lock tests**

Run:

```bash
go test ./service/crypto_payment -run TestScannerLock -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add service/crypto_payment/lock.go service/crypto_payment/scanner.go service/crypto_payment/lock_test.go
git commit -m "feat: add crypto scanner lock"
```

---

### Task 9: Implement BSC Scanner

**Files:**
- Create: `service/crypto_payment/bsc.go`
- Create: `service/crypto_payment/bsc_test.go`

- [ ] **Step 1: Write failing BSC decoding tests**

Create `service/crypto_payment/bsc_test.go`:

```go
package crypto_payment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeBSCTransferLog(t *testing.T) {
	log := bscRPCLog{
		Address: "0x55d398326f99059fF775485246999027B3197955",
		Topics: []string{
			bscTransferTopic,
			"0x0000000000000000000000001111111111111111111111111111111111111111",
			"0x0000000000000000000000002222222222222222222222222222222222222222",
		},
		Data:        "0x0000000000000000000000000000000000000000000000008ac7230489e80000",
		BlockNumber: "0x64",
		TxHash:      "0xtx",
		LogIndex:    "0x1",
	}
	transfer, err := decodeBSCTransferLog(log, 18)
	require.NoError(t, err)
	assert.Equal(t, "0x1111111111111111111111111111111111111111", transfer.FromAddress)
	assert.Equal(t, "0x2222222222222222222222222222222222222222", transfer.ToAddress)
	assert.Equal(t, "10000000000000000000", transfer.AmountBaseUnits)
	assert.Equal(t, int64(100), transfer.BlockNumber)
	assert.Equal(t, 1, transfer.LogIndex)
}
```

- [ ] **Step 2: Run BSC test to verify failure**

Run:

```bash
go test ./service/crypto_payment -run TestDecodeBSCTransferLog -count=1
```

Expected: FAIL with undefined BSC decoder symbols.

- [ ] **Step 3: Implement BSC scanner and decoder**

Create `service/crypto_payment/bsc.go`:

```go
package crypto_payment

import (
	"context"
	"bytes"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
)

const bscTransferTopic = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"

type BSCScanner struct {
	config setting.CryptoPaymentNetworkConfig
	client *http.Client
}

func NewBSCScanner(config setting.CryptoPaymentNetworkConfig) *BSCScanner {
	return &BSCScanner{config: config, client: &http.Client{Timeout: 15 * time.Second}}
}

func (s *BSCScanner) Network() string { return model.CryptoNetworkBSCERC20 }

func (s *BSCScanner) ScanOnce(ctx context.Context) error {
	if strings.TrimSpace(setting.CryptoBSCRPCURL) == "" {
		return fmt.Errorf("BSC RPC URL is not configured")
	}
	currentBlock, err := s.currentBlock(ctx)
	if err != nil {
		return err
	}
	state, err := model.GetCryptoScannerState(s.Network())
	fromBlock := currentBlock - int64(s.config.Confirmations) - 30
	if err == nil && state.LastScannedBlock > 0 {
		fromBlock = state.LastScannedBlock + 1
	}
	if fromBlock < 0 {
		fromBlock = 0
	}
	toBlock := fromBlock + 500
	maxSafe := currentBlock - int64(s.config.Confirmations) + 1
	if toBlock > maxSafe {
		toBlock = maxSafe
	}
	if toBlock < fromBlock {
		return nil
	}
	logs, err := s.getLogs(ctx, fromBlock, toBlock)
	if err != nil {
		return err
	}
	for _, item := range logs {
		transfer, err := decodeBSCTransferLog(item, s.config.Decimals)
		if err != nil {
			return err
		}
		if !strings.EqualFold(transfer.ToAddress, s.config.ReceiveAddress) {
			continue
		}
		transfer.Network = s.Network()
		transfer.TokenContract = s.config.Contract
		transfer.Confirmations = currentBlock - transfer.BlockNumber + 1
		transfer.ObservedAt = time.Now()
		if _, _, err := model.RecordCryptoTransfer(transfer); err != nil {
			return err
		}
	}
	return model.UpsertCryptoScannerState(s.Network(), toBlock, maxSafe)
}

type bscRPCLog struct {
	Address     string   `json:"address"`
	Topics      []string `json:"topics"`
	Data        string   `json:"data"`
	BlockNumber string   `json:"blockNumber"`
	TxHash      string   `json:"transactionHash"`
	LogIndex    string   `json:"logIndex"`
}

func decodeBSCTransferLog(log bscRPCLog, decimals int) (model.CryptoObservedTransfer, error) {
	if len(log.Topics) < 3 || strings.ToLower(log.Topics[0]) != bscTransferTopic {
		return model.CryptoObservedTransfer{}, fmt.Errorf("not a transfer log")
	}
	amount := new(big.Int)
	if _, ok := amount.SetString(strings.TrimPrefix(log.Data, "0x"), 16); !ok {
		return model.CryptoObservedTransfer{}, fmt.Errorf("invalid transfer amount")
	}
	blockNumber, err := parseHexInt64(log.BlockNumber)
	if err != nil {
		return model.CryptoObservedTransfer{}, err
	}
	logIndex, err := parseHexInt64(log.LogIndex)
	if err != nil {
		return model.CryptoObservedTransfer{}, err
	}
	return model.CryptoObservedTransfer{
		TxHash:          log.TxHash,
		LogIndex:        int(logIndex),
		BlockNumber:     blockNumber,
		FromAddress:     topicToEVMAddress(log.Topics[1]),
		ToAddress:       topicToEVMAddress(log.Topics[2]),
		TokenContract:   log.Address,
		TokenDecimals:   decimals,
		AmountBaseUnits: amount.String(),
	}, nil
}

func topicToEVMAddress(topic string) string {
	trimmed := strings.TrimPrefix(topic, "0x")
	if len(trimmed) < 40 {
		return "0x" + trimmed
	}
	return "0x" + strings.ToLower(trimmed[len(trimmed)-40:])
}

func parseHexInt64(value string) (int64, error) {
	trimmed := strings.TrimPrefix(value, "0x")
	parsed, err := strconv.ParseInt(trimmed, 16, 64)
	if err != nil {
		return 0, err
	}
	return parsed, nil
}
```

Implement `currentBlock` and `getLogs` in the same file using JSON-RPC and `common.Marshal` / `common.Unmarshal`:

```go
type bscRPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type bscRPCResponse struct {
	Result interface{} `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (s *BSCScanner) currentBlock(ctx context.Context) (int64, error) {
	var result string
	if err := s.rpc(ctx, "eth_blockNumber", nil, &result); err != nil {
		return 0, err
	}
	return parseHexInt64(result)
}

func (s *BSCScanner) getLogs(ctx context.Context, fromBlock int64, toBlock int64) ([]bscRPCLog, error) {
	filter := map[string]interface{}{
		"fromBlock": fmt.Sprintf("0x%x", fromBlock),
		"toBlock":   fmt.Sprintf("0x%x", toBlock),
		"address":   s.config.Contract,
		"topics": []interface{}{
			bscTransferTopic,
			nil,
			"0x000000000000000000000000" + strings.TrimPrefix(strings.ToLower(s.config.ReceiveAddress), "0x"),
		},
	}
	var logs []bscRPCLog
	if err := s.rpc(ctx, "eth_getLogs", []interface{}{filter}, &logs); err != nil {
		return nil, err
	}
	return logs, nil
}

func (s *BSCScanner) rpc(ctx context.Context, method string, params []interface{}, out interface{}) error {
	if params == nil {
		params = []interface{}{}
	}
	payload, err := common.Marshal(bscRPCRequest{JSONRPC: "2.0", ID: 1, Method: method, Params: params})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, setting.CryptoBSCRPCURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var envelope bscRPCResponse
	if err := common.DecodeJson(resp.Body, &envelope); err != nil {
		return err
	}
	if envelope.Error != nil {
		return fmt.Errorf("BSC RPC error %d: %s", envelope.Error.Code, envelope.Error.Message)
	}
	encoded, err := common.Marshal(envelope.Result)
	if err != nil {
		return err
	}
	return common.Unmarshal(encoded, out)
}
```

- [ ] **Step 4: Run BSC tests**

Run:

```bash
go test ./service/crypto_payment -run TestDecodeBSCTransferLog -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add service/crypto_payment/bsc.go service/crypto_payment/bsc_test.go
git commit -m "feat: scan bsc usdt transfers"
```

---

### Task 10: Implement TRON Scanner

**Files:**
- Create: `service/crypto_payment/tron.go`
- Create: `service/crypto_payment/tron_test.go`

- [ ] **Step 1: Write failing TRON event decode tests**

Create `service/crypto_payment/tron_test.go`:

```go
package crypto_payment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeTronTransferEvent(t *testing.T) {
	event := tronGridEvent{
		TransactionID:  "abc123",
		BlockNumber:    100,
		BlockTimestamp: 1710000000000,
		EventIndex:     2,
		Result: map[string]string{
			"from":  "TFromAddress1111111111111111111111111",
			"to":    "TToAddress111111111111111111111111111",
			"value": "10003721",
		},
	}
	transfer, err := decodeTronTransferEvent(event, 6)
	require.NoError(t, err)
	assert.Equal(t, "abc123", transfer.TxHash)
	assert.Equal(t, 2, transfer.LogIndex)
	assert.Equal(t, "10003721", transfer.AmountBaseUnits)
	assert.EqualValues(t, 100, transfer.BlockNumber)
}
```

- [ ] **Step 2: Run TRON test to verify failure**

Run:

```bash
go test ./service/crypto_payment -run TestDecodeTronTransferEvent -count=1
```

Expected: FAIL with undefined TRON symbols.

- [ ] **Step 3: Implement TRON scanner and decoder**

Create `service/crypto_payment/tron.go`:

```go
package crypto_payment

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
)

type TronScanner struct {
	config setting.CryptoPaymentNetworkConfig
	client *http.Client
}

func NewTronScanner(config setting.CryptoPaymentNetworkConfig) *TronScanner {
	return &TronScanner{config: config, client: &http.Client{Timeout: 15 * time.Second}}
}

func (s *TronScanner) Network() string { return model.CryptoNetworkTronTRC20 }

func (s *TronScanner) ScanOnce(ctx context.Context) error {
	if strings.TrimSpace(setting.CryptoTronRPCURL) == "" {
		return fmt.Errorf("TRON RPC URL is not configured")
	}
	currentBlock, err := s.currentBlock(ctx)
	if err != nil {
		return err
	}
	state, err := model.GetCryptoScannerState(s.Network())
	fromBlock := currentBlock - int64(s.config.Confirmations) - 40
	if err == nil && state.LastScannedBlock > 0 {
		fromBlock = state.LastScannedBlock + 1
	}
	if fromBlock < 0 {
		fromBlock = 0
	}
	toBlock := fromBlock + 200
	maxSafe := currentBlock - int64(s.config.Confirmations) + 1
	if toBlock > maxSafe {
		toBlock = maxSafe
	}
	if toBlock < fromBlock {
		return nil
	}
	events, err := s.getTransferEvents(ctx, fromBlock, toBlock)
	if err != nil {
		return err
	}
	for _, event := range events {
		transfer, err := decodeTronTransferEvent(event, s.config.Decimals)
		if err != nil {
			return err
		}
		if !strings.EqualFold(transfer.ToAddress, s.config.ReceiveAddress) {
			continue
		}
		transfer.Network = s.Network()
		transfer.TokenContract = s.config.Contract
		transfer.Confirmations = currentBlock - transfer.BlockNumber + 1
		transfer.ObservedAt = time.Now()
		if _, _, err := model.RecordCryptoTransfer(transfer); err != nil {
			return err
		}
	}
	return model.UpsertCryptoScannerState(s.Network(), toBlock, maxSafe)
}

type tronGridEventResponse struct {
	Data []tronGridEvent `json:"data"`
}

type tronGridEvent struct {
	TransactionID  string            `json:"transaction_id"`
	BlockNumber    int64             `json:"block_number"`
	BlockTimestamp int64             `json:"block_timestamp"`
	EventIndex     int               `json:"event_index"`
	Result         map[string]string `json:"result"`
}

func decodeTronTransferEvent(event tronGridEvent, decimals int) (model.CryptoObservedTransfer, error) {
	value := strings.TrimSpace(event.Result["value"])
	if value == "" {
		return model.CryptoObservedTransfer{}, fmt.Errorf("missing TRON transfer value")
	}
	return model.CryptoObservedTransfer{
		TxHash:          event.TransactionID,
		LogIndex:        event.EventIndex,
		BlockNumber:     event.BlockNumber,
		BlockTimestamp:  event.BlockTimestamp / 1000,
		FromAddress:     strings.TrimSpace(event.Result["from"]),
		ToAddress:       strings.TrimSpace(event.Result["to"]),
		TokenDecimals:   decimals,
		AmountBaseUnits: value,
	}, nil
}
```

Add HTTP methods in the same file:

```go
func (s *TronScanner) currentBlock(ctx context.Context) (int64, error) {
	endpoint := strings.TrimRight(setting.CryptoTronRPCURL, "/") + "/wallet/getnowblock"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader("{}"))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if setting.CryptoTronAPIKey != "" {
		req.Header.Set("TRON-PRO-API-KEY", setting.CryptoTronAPIKey)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var payload struct {
		BlockHeader struct {
			RawData struct {
				Number int64 `json:"number"`
			} `json:"raw_data"`
		} `json:"block_header"`
	}
	if err := common.DecodeJson(resp.Body, &payload); err != nil {
		return 0, err
	}
	if payload.BlockHeader.RawData.Number <= 0 {
		return 0, fmt.Errorf("TRON current block response missing block number")
	}
	return payload.BlockHeader.RawData.Number, nil
}

func (s *TronScanner) getTransferEvents(ctx context.Context, fromBlock int64, toBlock int64) ([]tronGridEvent, error) {
	base := strings.TrimRight(setting.CryptoTronRPCURL, "/")
	if !strings.Contains(base, "/v1/") {
		base = "https://api.trongrid.io"
	}
	endpoint, err := url.Parse(base + "/v1/contracts/" + url.PathEscape(s.config.Contract) + "/events")
	if err != nil {
		return nil, err
	}
	query := endpoint.Query()
	query.Set("event_name", "Transfer")
	query.Set("only_confirmed", "false")
	query.Set("limit", "200")
	query.Set("order_by", "block_timestamp,asc")
	query.Set("min_block_number", strconv.FormatInt(fromBlock, 10))
	query.Set("max_block_number", strconv.FormatInt(toBlock, 10))
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	if setting.CryptoTronAPIKey != "" {
		req.Header.Set("TRON-PRO-API-KEY", setting.CryptoTronAPIKey)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var payload tronGridEventResponse
	if err := common.DecodeJson(resp.Body, &payload); err != nil {
		return nil, err
	}
	filtered := make([]tronGridEvent, 0, len(payload.Data))
	for _, event := range payload.Data {
		if event.BlockNumber >= fromBlock && event.BlockNumber <= toBlock {
			filtered = append(filtered, event)
		}
	}
	return filtered, nil
}
```

If the configured TRON provider does not support `min_block_number` and `max_block_number`, keep the local filtering and add pagination in a follow-up task before enabling production scanning.

- [ ] **Step 4: Run TRON tests**

Run:

```bash
go test ./service/crypto_payment -run TestDecodeTronTransferEvent -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add service/crypto_payment/tron.go service/crypto_payment/tron_test.go
git commit -m "feat: scan tron usdt transfers"
```

---

### Task 11: Complete Confirmed Orders from Scanner

**Files:**
- Modify: `service/crypto_payment/scanner.go`
- Modify: `model/topup_crypto.go`
- Modify: `model/topup_crypto_test.go`

- [ ] **Step 1: Write failing test for confirmed transaction processing**

Append to `model/topup_crypto_test.go`:

```go
func TestCompleteReadyCryptoOrders(t *testing.T) {
	truncateTables(t)
	common.QuotaPerUnit = 500000
	insertUserForPaymentGuardTest(t, 1101, 0)
	order := seedCryptoOrderForCompletion(t, 1101, 10, "10003721")
	order.Status = CryptoPaymentStatusDetected
	order.MatchedTxHash = "0xready"
	order.MatchedLogIndex = 0
	require.NoError(t, DB.Save(order).Error)
	require.NoError(t, DB.Create(&CryptoPaymentTransaction{
		Network:         order.Network,
		TxHash:          "0xready",
		LogIndex:        0,
		BlockNumber:     100,
		ToAddress:       order.ReceiveAddress,
		TokenContract:   order.TokenContract,
		TokenSymbol:     CryptoTokenUSDT,
		TokenDecimals:   order.TokenDecimals,
		Amount:          order.PayAmount,
		AmountBaseUnits: order.PayAmountBaseUnits,
		Confirmations:   20,
		Status:          CryptoTransactionStatusConfirmed,
		MatchedOrderId:  order.Id,
		CreateTime:      time.Now().Unix(),
		UpdateTime:      time.Now().Unix(),
	}).Error)

	completed, err := CompleteReadyCryptoOrders(order.Network)
	require.NoError(t, err)
	assert.Equal(t, 1, completed)
	assert.Equal(t, 5000000, getUserQuotaForPaymentGuardTest(t, 1101))
}
```

- [ ] **Step 2: Run test to verify failure**

Run:

```bash
go test ./model -run TestCompleteReadyCryptoOrders -count=1
```

Expected: FAIL with undefined `CompleteReadyCryptoOrders`.

- [ ] **Step 3: Implement ready completion helper**

Append to `model/topup_crypto.go`:

```go
func CompleteReadyCryptoOrders(network string) (int, error) {
	var orders []CryptoPaymentOrder
	if err := DB.Where("network = ? AND status IN ? AND matched_tx_hash <> ''", NormalizeCryptoNetwork(network), []string{CryptoPaymentStatusDetected, CryptoPaymentStatusConfirmed}).Find(&orders).Error; err != nil {
		return 0, err
	}
	completed := 0
	for _, order := range orders {
		var tx CryptoPaymentTransaction
		if err := DB.Where("network = ? AND tx_hash = ? AND log_index = ? AND matched_order_id = ?", order.Network, order.MatchedTxHash, order.MatchedLogIndex, order.Id).First(&tx).Error; err != nil {
			continue
		}
		if tx.Confirmations < int64(order.RequiredConfirmations) {
			continue
		}
		evidence := CryptoTxEvidence{
			Network:         tx.Network,
			TxHash:          tx.TxHash,
			LogIndex:        tx.LogIndex,
			BlockNumber:     tx.BlockNumber,
			BlockTimestamp:  tx.BlockTimestamp,
			FromAddress:     tx.FromAddress,
			ToAddress:       tx.ToAddress,
			TokenContract:   tx.TokenContract,
			AmountBaseUnits: tx.AmountBaseUnits,
			Confirmations:   tx.Confirmations,
			RawPayload:      tx.RawPayload,
		}
		if err := CompleteCryptoTopUp(order.TradeNo, evidence); err != nil {
			return completed, err
		}
		completed++
	}
	return completed, nil
}
```

- [ ] **Step 4: Call ready completion after each scan**

In `service/crypto_payment/scanner.go`, inside `runScannerLoop`, after successful `scanner.ScanOnce(ctx)`, call:

```go
if _, err := model.CompleteReadyCryptoOrders(scanner.Network()); err != nil {
	common.SysLog("crypto completion error: " + err.Error())
}
```

- [ ] **Step 5: Run completion readiness test**

Run:

```bash
go test ./model -run TestCompleteReadyCryptoOrders -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add model/topup_crypto.go model/topup_crypto_test.go service/crypto_payment/scanner.go
git commit -m "feat: complete confirmed crypto orders"
```

---

### Task 12: Start Scanners on Application Startup

**Files:**
- Modify: `main.go`
- Modify: `service/crypto_payment/scanner.go`

- [ ] **Step 1: Add import and startup call**

In `main.go`, add import:

```go
cryptoPayment "github.com/ca0fgh/hermestoken/service/crypto_payment"
```

After Redis initialization in `InitResources()` and after `model.InitOptionMap()`, scanners need settings and Redis. Add this in `main()` after `service.StartSubscriptionQuotaResetTask()`:

```go
cryptoPayment.StartCryptoPaymentScanners()
```

- [ ] **Step 2: Run package compile**

Run:

```bash
go test ./service/crypto_payment ./model -run 'TestScannerLock|TestCompleteReadyCryptoOrders' -count=1
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add main.go
git commit -m "feat: start crypto payment scanners"
```

---

### Task 13: Add Secure Admin Crypto APIs

**Files:**
- Modify: `controller/topup_crypto.go`
- Create: `controller/topup_crypto_admin_test.go`
- Modify: `router/api-router.go`

- [ ] **Step 1: Write admin API tests**

Create `controller/topup_crypto_admin_test.go`:

```go
package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ca0fgh/hermestoken/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminListCryptoTopUpOrders(t *testing.T) {
	setupCryptoControllerTest(t)
	_, err := model.CreateCryptoTopUpOrder(model.CreateCryptoTopUpOrderInput{
		UserID:                801,
		Network:               model.CryptoNetworkTronTRC20,
		Amount:                10,
		ReceiveAddress:        "TQ4mVnPz4jG4n4hD9QJf9U9gKfZVfUiH9z",
		TokenContract:         "TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj",
		TokenDecimals:         6,
		RequiredConfirmations: 20,
		ExpireMinutes:         10,
		SuffixMax:             9999,
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/crypto/topup/orders", nil)

	AdminListCryptoTopUpOrders(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "crypto_usdt")
}
```

- [ ] **Step 2: Run admin test to verify failure**

Run:

```bash
go test ./controller -run TestAdminListCryptoTopUpOrders -count=1
```

Expected: FAIL with undefined admin handler.

- [ ] **Step 3: Implement admin list handlers**

Append to `controller/topup_crypto.go`:

```go
func AdminListCryptoTopUpOrders(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	orders, total, err := model.ListCryptoPaymentOrders(pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(orders)
	common.ApiSuccess(c, pageInfo)
}

func AdminListCryptoTransactions(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	transactions, total, err := model.ListCryptoPaymentTransactions(pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(transactions)
	common.ApiSuccess(c, pageInfo)
}
```

Append to `model/topup_crypto.go`:

```go
func ListCryptoPaymentOrders(pageInfo *common.PageInfo) ([]*CryptoPaymentOrder, int64, error) {
	var orders []*CryptoPaymentOrder
	var total int64
	query := DB.Model(&CryptoPaymentOrder{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&orders).Error; err != nil {
		return nil, 0, err
	}
	return orders, total, nil
}

func ListCryptoPaymentTransactions(pageInfo *common.PageInfo) ([]*CryptoPaymentTransaction, int64, error) {
	var transactions []*CryptoPaymentTransaction
	var total int64
	query := DB.Model(&CryptoPaymentTransaction{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&transactions).Error; err != nil {
		return nil, 0, err
	}
	return transactions, total, nil
}
```

- [ ] **Step 4: Register admin routes**

In `router/api-router.go`, add:

```go
cryptoAdminRoute := apiRouter.Group("/admin/crypto")
cryptoAdminRoute.Use(middleware.AdminAuth())
{
	cryptoAdminRoute.GET("/topup/orders", controller.AdminListCryptoTopUpOrders)
	cryptoAdminRoute.GET("/topup/transactions", controller.AdminListCryptoTransactions)
	cryptoAdminRoute.POST("/topup/orders/:trade_no/complete", middleware.RootAuth(), middleware.CriticalRateLimit(), middleware.SecureVerificationRequired(), controller.AdminCompleteCryptoTopUp)
}
```

Add `AdminCompleteCryptoTopUp` only after implementing evidence validation. If route compilation requires the symbol in this task, add a handler that returns `common.ApiErrorMsg(c, "链上证据不完整")` until Task 14.

- [ ] **Step 5: Run admin tests**

Run:

```bash
go test ./controller -run TestAdminListCryptoTopUpOrders -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add controller/topup_crypto.go controller/topup_crypto_admin_test.go router/api-router.go model/topup_crypto.go
git commit -m "feat: add crypto topup admin lists"
```

---

### Task 14: Add Admin Crypto Completion with Evidence

**Files:**
- Modify: `controller/topup_crypto.go`
- Modify: `controller/topup_crypto_admin_test.go`

- [ ] **Step 1: Write failing admin completion validation test**

Append to `controller/topup_crypto_admin_test.go`:

```go
func TestAdminCompleteCryptoTopUpRequiresEvidence(t *testing.T) {
	setupCryptoControllerTest(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Params = gin.Params{{Key: "trade_no", Value: "missing"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/admin/crypto/topup/orders/missing/complete", nil)

	AdminCompleteCryptoTopUp(c)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "链上证据")
}
```

- [ ] **Step 2: Run admin completion test**

Run:

```bash
go test ./controller -run TestAdminCompleteCryptoTopUpRequiresEvidence -count=1
```

Expected: FAIL if handler is missing or returns the wrong validation error.

- [ ] **Step 3: Implement admin completion handler**

Append to `controller/topup_crypto.go`:

```go
type adminCompleteCryptoTopUpRequest struct {
	Network         string `json:"network"`
	TxHash          string `json:"tx_hash"`
	LogIndex        int    `json:"log_index"`
	ToAddress       string `json:"to_address"`
	TokenContract   string `json:"token_contract"`
	AmountBaseUnits string `json:"amount_base_units"`
	Confirmations   int64  `json:"confirmations"`
	Reason          string `json:"reason"`
}

func AdminCompleteCryptoTopUp(c *gin.Context) {
	tradeNo := strings.TrimSpace(c.Param("trade_no"))
	var req adminCompleteCryptoTopUpRequest
	if err := c.ShouldBindJSON(&req); err != nil || tradeNo == "" || strings.TrimSpace(req.TxHash) == "" || strings.TrimSpace(req.Reason) == "" {
		common.ApiErrorMsg(c, "链上证据不完整")
		return
	}
	order := model.GetCryptoPaymentOrderByTradeNo(tradeNo)
	if order == nil {
		common.ApiErrorMsg(c, "订单不存在")
		return
	}
	err := model.CompleteCryptoTopUp(tradeNo, model.CryptoTxEvidence{
		Network:         req.Network,
		TxHash:          req.TxHash,
		LogIndex:        req.LogIndex,
		ToAddress:       req.ToAddress,
		TokenContract:   req.TokenContract,
		AmountBaseUnits: req.AmountBaseUnits,
		Confirmations:   req.Confirmations,
		RawPayload:      common.GetJsonString(req),
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordLogWithAdminInfo(order.UserId, model.LogTypeTopup, "管理员USDT补单成功，订单: "+tradeNo+"，原因: "+req.Reason, map[string]interface{}{
		"admin_id": c.GetInt("id"),
		"ip":       c.ClientIP(),
		"network":  req.Network,
		"tx_hash":  req.TxHash,
		"reason":   req.Reason,
	})
	common.ApiSuccess(c, nil)
}
```

- [ ] **Step 4: Run admin completion tests**

Run:

```bash
go test ./controller -run 'TestAdminCompleteCryptoTopUpRequiresEvidence|TestAdminListCryptoTopUpOrders' -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add controller/topup_crypto.go controller/topup_crypto_admin_test.go
git commit -m "feat: secure crypto topup completion"
```

---

### Task 15: Add Frontend Crypto Payment Modal

**Files:**
- Create: `web/src/components/topup/modals/CryptoPaymentModal.jsx`
- Modify: `web/src/components/topup/index.jsx`
- Modify: `web/src/components/topup/RechargeCard.jsx`

- [ ] **Step 1: Add modal component**

Create `web/src/components/topup/modals/CryptoPaymentModal.jsx`:

```jsx
import React, { useMemo } from 'react';
import { Modal, Typography, Button, Tag, Progress, Banner } from '@douyinfe/semi-ui';
import { Copy, Wallet } from 'lucide-react';
import { copy } from '../../../helpers';

const { Text, Title } = Typography;

const terminalStatuses = new Set([
  'success',
  'expired',
  'failed',
  'underpaid',
  'overpaid',
  'late_paid',
  'ambiguous',
]);

const CryptoPaymentModal = ({ t, open, order, onCancel }) => {
  const secondsLeft = useMemo(() => {
    if (!order?.expires_at) return 0;
    return Math.max(0, order.expires_at - Math.floor(Date.now() / 1000));
  }, [order?.expires_at]);

  const progress = order?.required_confirmations
    ? Math.min(100, Math.round(((order.confirmations || 0) / order.required_confirmations) * 100))
    : 0;
  const expired = order?.status === 'expired' || secondsLeft <= 0;
  const terminal = terminalStatuses.has(order?.status);

  return (
    <Modal
      visible={open}
      title={<span className='flex items-center gap-2'><Wallet size={18} />{t('USDT 充值')}</span>}
      onCancel={onCancel}
      footer={null}
      maskClosable={false}
      size='medium'
      centered
    >
      {!order ? null : (
        <div className='space-y-4'>
          <div className='flex items-center gap-2'>
            <Tag color={order.network === 'tron_trc20' ? 'green' : 'yellow'}>{order.network === 'tron_trc20' ? 'TRON TRC-20' : 'BSC'}</Tag>
            <Tag color='blue'>USDT</Tag>
            <Tag color={terminal ? 'grey' : 'orange'}>{order.status}</Tag>
          </div>

          <Banner
            type='warning'
            closeIcon={null}
            description={t('请严格使用当前网络并支付完整显示金额。少付、多付、超时到账都不会自动入账。')}
          />

          <div className='rounded-xl border border-gray-200 p-4 space-y-3'>
            <div>
              <Text type='secondary'>{t('应付金额')}</Text>
              <div className='flex items-center justify-between gap-2'>
                <Title heading={3} style={{ margin: 0 }}>{order.pay_amount} USDT</Title>
                <Button icon={<Copy size={14} />} disabled={expired} onClick={() => copy(order.pay_amount)}>{t('复制')}</Button>
              </div>
            </div>
            <div>
              <Text type='secondary'>{t('收款地址')}</Text>
              <div className='flex items-center justify-between gap-2 break-all'>
                <Text copyable={false}>{order.receive_address}</Text>
                <Button icon={<Copy size={14} />} disabled={expired} onClick={() => copy(order.receive_address)}>{t('复制')}</Button>
              </div>
            </div>
          </div>

          <div className='space-y-2'>
            <div className='flex justify-between'>
              <Text type='secondary'>{t('订单倒计时')}</Text>
              <Text strong>{expired ? t('已过期') : `${Math.floor(secondsLeft / 60)}:${String(secondsLeft % 60).padStart(2, '0')}`}</Text>
            </div>
            <div className='flex justify-between'>
              <Text type='secondary'>{t('确认进度')}</Text>
              <Text>{order.confirmations || 0}/{order.required_confirmations}</Text>
            </div>
            <Progress percent={progress} showInfo={false} />
          </div>

          {order.tx_hash && (
            <div className='break-all'>
              <Text type='secondary'>TX Hash: </Text>
              <Text>{order.tx_hash}</Text>
            </div>
          )}
        </div>
      )}
    </Modal>
  );
};

export default CryptoPaymentModal;
```

- [ ] **Step 2: Wire state and API calls in top-up page**

In `web/src/components/topup/index.jsx`, import the modal:

```jsx
import CryptoPaymentModal from './modals/CryptoPaymentModal';
```

Add state:

```jsx
const [enableCryptoTopUp, setEnableCryptoTopUp] = useState(false);
const [cryptoNetworks, setCryptoNetworks] = useState([]);
const [cryptoOrder, setCryptoOrder] = useState(null);
const [cryptoModalOpen, setCryptoModalOpen] = useState(false);
```

In `getTopupInfo`, after Waffo state handling, add:

```jsx
setEnableCryptoTopUp(data.enable_crypto_usdt_topup || false);
setCryptoNetworks(data.crypto_networks || []);
```

Add functions:

```jsx
const createCryptoTopUpOrder = async (network) => {
  try {
    setPaymentLoading(true);
    const res = await API.post('/api/user/crypto/topup/order', {
      network,
      amount: parseInt(topUpCount),
    });
    if (res.data?.success) {
      setCryptoOrder(res.data.data);
      setCryptoModalOpen(true);
    } else {
      showError(res.data?.data || res.data?.message || t('支付请求失败'));
    }
  } catch (error) {
    showError(t('支付请求失败'));
  } finally {
    setPaymentLoading(false);
  }
};

useEffect(() => {
  if (!cryptoModalOpen || !cryptoOrder?.trade_no) return;
  const timer = setInterval(async () => {
    const res = await API.get(`/api/user/crypto/topup/order/${cryptoOrder.trade_no}`);
    if (res.data?.success) {
      const nextOrder = res.data.data;
      setCryptoOrder(nextOrder);
      if (nextOrder.status === 'success') {
        showSuccess(t('充值成功'));
        getUserQuota().then();
      }
      if (['success', 'expired', 'failed', 'underpaid', 'overpaid', 'late_paid', 'ambiguous'].includes(nextOrder.status)) {
        clearInterval(timer);
      }
    }
  }, 3000);
  return () => clearInterval(timer);
}, [cryptoModalOpen, cryptoOrder?.trade_no]);
```

Render modal near other modals:

```jsx
<CryptoPaymentModal
  t={t}
  open={cryptoModalOpen}
  order={cryptoOrder}
  onCancel={() => setCryptoModalOpen(false)}
/>
```

- [ ] **Step 3: Render USDT network buttons in RechargeCard**

In `web/src/components/topup/RechargeCard.jsx`, add props:

```jsx
enableCryptoTopUp,
cryptoNetworks,
createCryptoTopUpOrder,
```

Inside the online recharge form, after Waffo area, add:

```jsx
{enableCryptoTopUp && cryptoNetworks && cryptoNetworks.length > 0 && (
  <Form.Slot label={t('USDT 充值')}>
    <Space wrap>
      {cryptoNetworks.map((network) => (
        <Button
          key={network.network}
          theme='outline'
          type='tertiary'
          onClick={() => createCryptoTopUpOrder(network.network)}
          loading={paymentLoading}
          icon={<CreditCard size={18} color='var(--semi-color-text-2)' />}
          className='!rounded-lg !px-4 !py-2'
        >
          {network.display_name} USDT
        </Button>
      ))}
    </Space>
  </Form.Slot>
)}
```

Pass the new props from `index.jsx` into `RechargeCard`.

- [ ] **Step 4: Run frontend lint/build for touched area**

Run:

```bash
cd web
bun run lint
```

Expected: no lint errors. If the project has no lint script, run:

```bash
cd web
bun run build
```

Expected: production build completes.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/topup/modals/CryptoPaymentModal.jsx web/src/components/topup/index.jsx web/src/components/topup/RechargeCard.jsx
git commit -m "feat: add usdt topup payment modal"
```

---

### Task 16: Add Admin Crypto Settings UI

**Files:**
- Create: `web/src/pages/Setting/Payment/SettingsPaymentGatewayCrypto.jsx`
- Modify: `web/src/components/settings/PaymentSetting.jsx`

- [ ] **Step 1: Add settings component**

Create `web/src/pages/Setting/Payment/SettingsPaymentGatewayCrypto.jsx`:

```jsx
import React, { useEffect, useRef, useState } from 'react';
import { Button, Form, Row, Col, Spin, Typography } from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../../helpers';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

export default function SettingsPaymentGatewayCrypto({ options, refresh }) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const formApiRef = useRef(null);
  const [inputs, setInputs] = useState({
    CryptoPaymentEnabled: false,
    CryptoScannerEnabled: true,
    CryptoOrderExpireMinutes: 10,
    CryptoUniqueSuffixMax: 9999,
    CryptoTronEnabled: false,
    CryptoTronReceiveAddress: '',
    CryptoTronUSDTContract: 'TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj',
    CryptoTronRPCURL: '',
    CryptoTronAPIKey: '',
    CryptoTronConfirmations: 20,
    CryptoBSCEnabled: false,
    CryptoBSCReceiveAddress: '',
    CryptoBSCUSDTContract: '0x55d398326f99059fF775485246999027B3197955',
    CryptoBSCRPCURL: '',
    CryptoBSCConfirmations: 15,
  });

  useEffect(() => {
    if (!options || !formApiRef.current) return;
    const nextInputs = {
      CryptoPaymentEnabled: options.CryptoPaymentEnabled === 'true',
      CryptoScannerEnabled: options.CryptoScannerEnabled !== 'false',
      CryptoOrderExpireMinutes: Number(options.CryptoOrderExpireMinutes || 10),
      CryptoUniqueSuffixMax: Number(options.CryptoUniqueSuffixMax || 9999),
      CryptoTronEnabled: options.CryptoTronEnabled === 'true',
      CryptoTronReceiveAddress: options.CryptoTronReceiveAddress || '',
      CryptoTronUSDTContract: options.CryptoTronUSDTContract || 'TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj',
      CryptoTronRPCURL: options.CryptoTronRPCURL || '',
      CryptoTronAPIKey: '',
      CryptoTronConfirmations: Number(options.CryptoTronConfirmations || 20),
      CryptoBSCEnabled: options.CryptoBSCEnabled === 'true',
      CryptoBSCReceiveAddress: options.CryptoBSCReceiveAddress || '',
      CryptoBSCUSDTContract: options.CryptoBSCUSDTContract || '0x55d398326f99059fF775485246999027B3197955',
      CryptoBSCRPCURL: options.CryptoBSCRPCURL || '',
      CryptoBSCConfirmations: Number(options.CryptoBSCConfirmations || 15),
    };
    setInputs(nextInputs);
    formApiRef.current.setValues(nextInputs);
  }, [options]);

  const submit = async () => {
    setLoading(true);
    try {
      const entries = Object.entries(inputs).filter(([key, value]) => {
        if (key === 'CryptoTronAPIKey' && value === '') return false;
        return value !== undefined;
      });
      const requests = entries.map(([key, value]) =>
        API.put('/api/option/', { key, value: typeof value === 'boolean' ? String(value) : String(value) }),
      );
      const results = await Promise.all(requests);
      const failed = results.find((res) => !res.data?.success);
      if (failed) {
        showError(failed.data?.message || t('更新失败'));
        return;
      }
      showSuccess(t('更新成功'));
      refresh && refresh();
    } catch (error) {
      showError(t('更新失败'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Spin spinning={loading}>
      <Form initValues={inputs} onValueChange={setInputs} getFormApi={(api) => (formApiRef.current = api)}>
        <Form.Section text={t('USDT 设置')}>
          <Text type='secondary'>{t('仅支持 USDT，用户必须严格支付系统生成的唯一金额。')}</Text>
          <Row gutter={16}>
            <Col span={8}><Form.Switch field='CryptoPaymentEnabled' label={t('启用 USDT 充值')} /></Col>
            <Col span={8}><Form.Switch field='CryptoScannerEnabled' label={t('启用扫链器')} /></Col>
            <Col span={4}><Form.InputNumber field='CryptoOrderExpireMinutes' label={t('订单有效期分钟')} min={5} max={60} precision={0} /></Col>
            <Col span={4}><Form.InputNumber field='CryptoUniqueSuffixMax' label={t('尾数上限')} min={99} max={9999} precision={0} /></Col>
          </Row>
        </Form.Section>
        <Form.Section text='TRON TRC-20'>
          <Row gutter={16}>
            <Col span={6}><Form.Switch field='CryptoTronEnabled' label={t('启用 TRON')} /></Col>
            <Col span={18}><Form.Input field='CryptoTronReceiveAddress' label={t('TRON 收款地址')} /></Col>
            <Col span={12}><Form.Input field='CryptoTronUSDTContract' label={t('TRON USDT 合约地址')} /></Col>
            <Col span={12}><Form.Input field='CryptoTronRPCURL' label='TRON RPC / API URL' /></Col>
            <Col span={12}><Form.Input field='CryptoTronAPIKey' label='TRON API Key' type='password' extraText={t('敏感信息不会发送到前端显示')} /></Col>
            <Col span={12}><Form.InputNumber field='CryptoTronConfirmations' label={t('TRON 确认数')} min={10} precision={0} /></Col>
          </Row>
        </Form.Section>
        <Form.Section text='BSC'>
          <Row gutter={16}>
            <Col span={6}><Form.Switch field='CryptoBSCEnabled' label={t('启用 BSC')} /></Col>
            <Col span={18}><Form.Input field='CryptoBSCReceiveAddress' label={t('BSC 收款地址')} /></Col>
            <Col span={12}><Form.Input field='CryptoBSCUSDTContract' label={t('BSC USDT 合约地址')} /></Col>
            <Col span={12}><Form.Input field='CryptoBSCRPCURL' label='BSC RPC URL' /></Col>
            <Col span={12}><Form.InputNumber field='CryptoBSCConfirmations' label={t('BSC 确认数')} min={8} precision={0} /></Col>
          </Row>
        </Form.Section>
        <Button type='primary' onClick={submit}>{t('更新支付设置')}</Button>
      </Form>
    </Spin>
  );
}
```

- [ ] **Step 2: Wire crypto tab in PaymentSetting**

In `web/src/components/settings/PaymentSetting.jsx`, import:

```jsx
import SettingsPaymentGatewayCrypto from '../../pages/Setting/Payment/SettingsPaymentGatewayCrypto';
```

Add crypto defaults to the `inputs` state object.

Add tab after Waffo tabs:

```jsx
<Tabs.TabPane tab={t('USDT 设置')} itemKey='crypto'>
  <SettingsPaymentGatewayCrypto options={inputs} refresh={refresh} />
</Tabs.TabPane>
```

- [ ] **Step 3: Run frontend build**

Run:

```bash
cd web
bun run build
```

Expected: build completes.

- [ ] **Step 4: Commit**

```bash
git add web/src/pages/Setting/Payment/SettingsPaymentGatewayCrypto.jsx web/src/components/settings/PaymentSetting.jsx
git commit -m "feat: add crypto payment settings UI"
```

---

### Task 17: Update History Display and i18n

**Files:**
- Modify: `web/src/components/topup/modals/TopupHistoryModal.jsx`
- Modify: `web/src/i18n/locales/zh-CN.json`
- Modify: `web/src/i18n/locales/en.json`

- [ ] **Step 1: Add crypto labels in history modal**

In `TopupHistoryModal.jsx`, find payment method rendering and add a branch:

```jsx
if (record.payment_method === 'crypto_usdt') {
  return 'USDT';
}
```

If the modal has a detail section, display:

```jsx
{record.payment_method === 'crypto_usdt' && (
  <Tag color='blue'>USDT</Tag>
)}
```

- [ ] **Step 2: Add translation keys**

Add these keys to `web/src/i18n/locales/zh-CN.json` and `web/src/i18n/locales/en.json`:

```json
"USDT 设置": "USDT Settings",
"启用 USDT 充值": "Enable USDT top-up",
"启用扫链器": "Enable scanner",
"订单有效期分钟": "Order expiry minutes",
"尾数上限": "Unique suffix max",
"启用 TRON": "Enable TRON",
"TRON 收款地址": "TRON receive address",
"TRON USDT 合约地址": "TRON USDT contract address",
"TRON 确认数": "TRON confirmations",
"启用 BSC": "Enable BSC",
"BSC 收款地址": "BSC receive address",
"BSC USDT 合约地址": "BSC USDT contract address",
"BSC 确认数": "BSC confirmations",
"USDT 充值": "USDT top-up",
"应付金额": "Amount to pay",
"收款地址": "Receive address",
"订单倒计时": "Order countdown",
"确认进度": "Confirmation progress",
"请严格使用当前网络并支付完整显示金额。少付、多付、超时到账都不会自动入账。": "Use the selected network and pay the exact displayed amount. Underpaid, overpaid, or late transfers are not credited automatically."
```

For `zh-CN.json`, the value can equal the key for Chinese strings.

- [ ] **Step 3: Run frontend build**

Run:

```bash
cd web
bun run build
```

Expected: build completes.

- [ ] **Step 4: Commit**

```bash
git add web/src/components/topup/modals/TopupHistoryModal.jsx web/src/i18n/locales/zh-CN.json web/src/i18n/locales/en.json
git commit -m "feat: show crypto topup history labels"
```

---

### Task 18: Final Verification

**Files:**
- Read-only verification across the repo.

- [ ] **Step 1: Run backend crypto tests**

```bash
go test ./model ./controller ./service/crypto_payment -run 'Crypto|Scanner|BSC|Tron' -count=1
```

Expected: PASS.

- [ ] **Step 2: Run broader backend tests for touched packages**

```bash
go test ./model ./controller ./service/crypto_payment -count=1
```

Expected: PASS.

- [ ] **Step 3: Run frontend build**

```bash
cd web
bun run build
```

Expected: build completes.

- [ ] **Step 4: Review diff for secrets and unsafe behavior**

```bash
git diff --stat HEAD~18..HEAD
git diff HEAD~18..HEAD -- ':!web/dist' ':!web/build'
```

Check manually:

- No private keys in code or tests.
- API keys are not logged.
- No direct quota update outside `CompleteCryptoTopUp` for crypto success.
- `ManualCompleteTopUp` rejects `crypto_usdt`.
- Scanner cursor is not advanced after errors.
- Admin completion route uses `RootAuth`, `CriticalRateLimit`, and `SecureVerificationRequired`.

- [ ] **Step 5: Commit verification notes if docs changed**

If a verification note is added to docs, commit it:

```bash
git add docs/superpowers/plans/2026-04-26-usdt-crypto-topup.md
git commit -m "docs: record crypto topup verification plan"
```

---

## Self-Review Checklist

- Spec coverage: this plan maps to Phase 1 requirements for models, settings, APIs, scanner lock, BSC scanner, TRON scanner, frontend payment modal, admin settings, and basic admin visibility.
- Security coverage: private keys are not stored; scanner uses configured RPC only; admin completion requires chain evidence; crypto manual completion is blocked outside the evidence path.
- Testing coverage: each backend unit has a failing test step before implementation; frontend changes have build verification; final verification covers backend and frontend touched areas.
- Type consistency: model constants use `CryptoNetworkTronTRC20`, `CryptoNetworkBSCERC20`, `PaymentMethodCryptoUSDT`, and crypto statuses consistently across controller, scanner, and tests.
- Scope control: USDC, HD addresses, refunds, sweeping, and additional chains remain outside Phase 1.
