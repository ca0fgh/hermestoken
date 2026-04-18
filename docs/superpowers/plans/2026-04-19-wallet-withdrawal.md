# Wallet Withdrawal Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a complete manual Alipay withdrawal flow for wallet balance, including frozen-balance accounting, configurable fee rules, user self-service submission, admin approval, and payout confirmation.

**Architecture:** Add a dedicated `user_withdrawals` domain model rather than overloading `TopUp`, store fee/config in `OptionMap`, and freeze wallet balance at submission time by introducing `user.withdraw_frozen_quota`. Reuse the existing wallet page for user-facing actions and the existing admin route/sidebar/table patterns for withdrawal operations, with state transitions guarded by transactional row locks.

**Tech Stack:** Go + Gin + GORM + OptionMap config, React + Semi UI, existing CardPro/CardTable/ListPagination helpers, node --test frontend source assertions, Go test integration/controller tests.

---

## File Structure

### Backend domain and persistence

- Create: `model/user_withdrawal.go`
  - Withdrawal entity, status constants, fee rule structs, transactional create/approve/reject/mark-paid helpers, list/detail queries.
- Create: `model/user_withdrawal_test.go`
  - Model-level tests for fee rules, frozen balance accounting, and state transitions.
- Modify: `model/user.go`
  - Add `WithdrawFrozenQuota` field, helper methods for loading wallet snapshots, and safe balance mutation helpers used by withdrawals.
- Modify: `model/main.go`
  - Auto-migrate `UserWithdrawal` in normal and fast migration paths.
- Modify: `model/option.go`
  - Register default withdrawal options in `OptionMap` and wire update hooks.
- Create: `model/user_withdrawal_setting.go`
  - Parse/validate `WithdrawalEnabled`, `WithdrawalMinAmount`, `WithdrawalInstruction`, `WithdrawalFeeRules` from `OptionMap`.

### Backend HTTP surface

- Create: `controller/user_withdrawal.go`
  - User and admin withdrawal handlers plus request DTO validation.
- Create: `controller/user_withdrawal_test.go`
  - Controller tests for user submit/list/detail and admin approve/reject/mark-paid.
- Modify: `router/api-router.go`
  - Register user and admin withdrawal routes.
- Modify: `controller/user.go`
  - Expose `withdraw_frozen_quota` in `/api/user/self`; include it anywhere wallet summary already depends on self payload.

### Admin configuration surface

- Create: `web/src/pages/Setting/Payment/SettingsWithdrawal.jsx`
  - Root-only withdrawal settings card with toggle, minimum amount, instruction text, and fee-rule editor.
- Modify: `web/src/components/settings/PaymentSetting.jsx`
  - Load/save withdrawal options and render the new settings card.
- Create: `web/tests/withdrawal-settings.test.mjs`
  - Source tests for payment settings wiring and fee-rule editor presence.

### User wallet UI

- Create: `web/src/components/topup/WithdrawalCard.jsx`
  - Wallet summary card for available/frozen balance and entry buttons.
- Create: `web/src/components/topup/modals/WithdrawalApplyModal.jsx`
  - User submission modal with live fee preview.
- Create: `web/src/components/topup/modals/WithdrawalHistoryModal.jsx`
  - Paginated user withdrawal history modal.
- Modify: `web/src/components/topup/index.jsx`
  - Fetch withdrawal config, render withdrawal card, open modals, refresh self state after submit.
- Create: `web/src/helpers/withdrawal.js`
  - Shared currency/frequency helpers, fee preview helpers, status labels.
- Create: `web/tests/wallet-withdrawal.test.mjs`
  - Source tests for wallet rendering and API usage.

### Admin management UI

- Create: `web/src/pages/Withdrawal/index.jsx`
  - Admin withdrawal management page shell.
- Create: `web/src/components/table/withdrawals/index.jsx`
  - CardPro wrapper.
- Create: `web/src/components/table/withdrawals/WithdrawalsTable.jsx`
  - Desktop/mobile list.
- Create: `web/src/components/table/withdrawals/WithdrawalsColumnDefs.jsx`
  - Status badges, masked Alipay account, action buttons.
- Create: `web/src/components/table/withdrawals/WithdrawalsActions.jsx`
  - Refresh shortcuts and filter actions.
- Create: `web/src/components/table/withdrawals/WithdrawalsFilters.jsx`
  - Search/status/date filter form.
- Create: `web/src/components/table/withdrawals/WithdrawalsDescription.jsx`
  - Page description and compact-mode controls.
- Create: `web/src/components/table/withdrawals/modals/WithdrawalReviewModal.jsx`
  - Approve/reject modal.
- Create: `web/src/components/table/withdrawals/modals/WithdrawalPaidModal.jsx`
  - Mark-paid modal with optional receipt info.
- Create: `web/src/hooks/withdrawals/useWithdrawalsData.jsx`
  - Admin page data hook.
- Modify: `web/src/App.jsx`
  - Add lazy route and admin route for `/console/withdrawal`.
- Modify: `web/src/components/layout/SiderBar.jsx`
  - Add admin sidebar item.
- Modify: `web/src/hooks/common/useSidebar.js`
  - Add admin default module key.
- Modify: `web/src/pages/Setting/Operation/SettingsSidebarModulesAdmin.jsx`
  - Add admin sidebar module toggle.
- Modify: `web/src/components/settings/personal/cards/NotificationSettings.jsx`
  - Add admin module visibility entry so non-root admin users can toggle it consistently.
- Modify: `model/user.go`
  - Add default sidebar module key for admins.
- Create: `web/tests/withdrawal-admin-route.test.mjs`
  - Route/sidebar/settings source assertions.

### Localization and regression coverage

- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh-CN.json`
- Modify: `web/src/i18n/locales/zh-TW.json`
- Modify: `web/src/i18n/locales/ja.json`
- Modify: `web/src/i18n/locales/fr.json`
- Modify: `web/src/i18n/locales/ru.json`
- Modify: `web/src/i18n/locales/vi.json`
- Create: `web/tests/withdrawal-locales.test.mjs`
  - Ensure new keys exist in all locales.

---

### Task 1: Add Withdrawal Domain Model and Config Parsing

**Files:**
- Create: `model/user_withdrawal.go`
- Create: `model/user_withdrawal_setting.go`
- Modify: `model/user.go`
- Modify: `model/main.go`
- Modify: `model/option.go`
- Test: `model/user_withdrawal_test.go`

- [ ] **Step 1: Write the failing model/config tests**

```go
package model

import (
    "testing"

    "github.com/QuantumNous/new-api/common"
)

func TestCreateUserWithdrawalFreezesQuotaAndStoresSnapshots(t *testing.T) {
    db := setupTestDB(t)
    user := seedUserWithQuota(t, db, 1, 100000)
    common.OptionMap = map[string]string{
        "WithdrawalEnabled":    "true",
        "WithdrawalMinAmount":  "10",
        "WithdrawalInstruction": "manual payout",
        "WithdrawalFeeRules":   `[{"min_amount":10,"max_amount":0,"fee_type":"fixed","fee_value":2,"enabled":true,"sort_order":1}]`,
    }

    order, err := CreateUserWithdrawal(&CreateUserWithdrawalParams{
        UserID:         user.Id,
        Amount:         100,
        AlipayAccount:  "alice@example.com",
        AlipayRealName: "Alice",
    })
    if err != nil {
        t.Fatalf("CreateUserWithdrawal returned error: %v", err)
    }

    refreshed, _ := GetUserById(user.Id, true)
    if refreshed.Quota != 99900 {
        t.Fatalf("quota = %d, want 99900", refreshed.Quota)
    }
    if refreshed.WithdrawFrozenQuota != 100 {
        t.Fatalf("withdraw_frozen_quota = %d, want 100", refreshed.WithdrawFrozenQuota)
    }
    if order.Status != UserWithdrawalStatusPending {
        t.Fatalf("status = %s, want pending", order.Status)
    }
    if order.FeeAmount != 2 || order.NetAmount != 98 {
        t.Fatalf("fee/net = %v/%v, want 2/98", order.FeeAmount, order.NetAmount)
    }
}

func TestRejectApprovedWithdrawalReturnsFrozenQuota(t *testing.T) {
    db := setupTestDB(t)
    user := seedUserWithQuota(t, db, 1, 100000)
    withdrawal := seedApprovedWithdrawal(t, db, user.Id, 100)

    if err := RejectUserWithdrawal(withdrawal.Id, 99, "manual reject"); err != nil {
        t.Fatalf("RejectUserWithdrawal returned error: %v", err)
    }

    refreshed, _ := GetUserById(user.Id, true)
    if refreshed.Quota != 100000 {
        t.Fatalf("quota = %d, want restored 100000", refreshed.Quota)
    }
    if refreshed.WithdrawFrozenQuota != 0 {
        t.Fatalf("withdraw_frozen_quota = %d, want 0", refreshed.WithdrawFrozenQuota)
    }
}
```

- [ ] **Step 2: Run model tests to verify they fail**

Run: `cd /Users/money/project/subproject/hermestoken && go test ./model -run 'Test(CreateUserWithdrawalFreezesQuotaAndStoresSnapshots|RejectApprovedWithdrawalReturnsFrozenQuota)'`
Expected: FAIL with `undefined: CreateUserWithdrawalParams`, missing `WithdrawFrozenQuota`, and missing withdrawal status constants.

- [ ] **Step 3: Add the withdrawal model and config parser**

```go
package model

const (
    UserWithdrawalStatusPending  = "pending"
    UserWithdrawalStatusApproved = "approved"
    UserWithdrawalStatusPaid     = "paid"
    UserWithdrawalStatusRejected = "rejected"
)

type UserWithdrawal struct {
    Id                   int     `json:"id"`
    UserId               int     `json:"user_id" gorm:"index;not null"`
    TradeNo              string  `json:"trade_no" gorm:"uniqueIndex;type:varchar(64);not null"`
    Channel              string  `json:"channel" gorm:"type:varchar(32);not null;default:'alipay'"`
    Currency             string  `json:"currency" gorm:"type:varchar(16);not null"`
    ExchangeRateSnapshot float64 `json:"exchange_rate_snapshot" gorm:"type:decimal(18,6);not null;default:1"`
    AvailableQuotaSnapshot int   `json:"available_quota_snapshot" gorm:"not null;default:0"`
    FrozenQuotaSnapshot  int     `json:"frozen_quota_snapshot" gorm:"not null;default:0"`
    ApplyAmount          float64 `json:"apply_amount" gorm:"type:decimal(18,2);not null;default:0"`
    FeeAmount            float64 `json:"fee_amount" gorm:"type:decimal(18,2);not null;default:0"`
    NetAmount            float64 `json:"net_amount" gorm:"type:decimal(18,2);not null;default:0"`
    ApplyQuota           int     `json:"apply_quota" gorm:"not null;default:0"`
    FeeQuota             int     `json:"fee_quota" gorm:"not null;default:0"`
    NetQuota             int     `json:"net_quota" gorm:"not null;default:0"`
    AlipayAccount        string  `json:"alipay_account" gorm:"type:varchar(128);not null"`
    AlipayRealName       string  `json:"alipay_real_name" gorm:"type:varchar(64);default:''"`
    Status               string  `json:"status" gorm:"type:varchar(32);index;not null;default:'pending'"`
    FeeRuleSnapshotJSON  string  `json:"fee_rule_snapshot_json" gorm:"type:text"`
    ReviewAdminId        int     `json:"review_admin_id" gorm:"index;not null;default:0"`
    RejectedAdminId      int     `json:"rejected_admin_id" gorm:"index;not null;default:0"`
    PaidAdminId          int     `json:"paid_admin_id" gorm:"index;not null;default:0"`
    ReviewNote           string  `json:"review_note" gorm:"type:text"`
    RejectionNote        string  `json:"rejection_note" gorm:"type:text"`
    PayReceiptNo         string  `json:"pay_receipt_no" gorm:"type:varchar(128);default:''"`
    PayReceiptUrl        string  `json:"pay_receipt_url" gorm:"type:text"`
    PaidNote             string  `json:"paid_note" gorm:"type:text"`
    ReviewedAt           int64   `json:"reviewed_at" gorm:"bigint"`
    PaidAt               int64   `json:"paid_at" gorm:"bigint"`
    CreatedAt            int64   `json:"created_at" gorm:"bigint"`
    UpdatedAt            int64   `json:"updated_at" gorm:"bigint"`
}

type WithdrawalFeeRule struct {
    MinAmount float64 `json:"min_amount"`
    MaxAmount float64 `json:"max_amount"`
    FeeType   string  `json:"fee_type"`
    FeeValue  float64 `json:"fee_value"`
    MinFee    float64 `json:"min_fee"`
    MaxFee    float64 `json:"max_fee"`
    Enabled   bool    `json:"enabled"`
    SortOrder int     `json:"sort_order"`
}
```

- [ ] **Step 4: Wire migration/default options and add the user field**

```go
// model/user.go
WithdrawFrozenQuota int `json:"withdraw_frozen_quota" gorm:"type:int;default:0;column:withdraw_frozen_quota"`

// model/main.go inside AutoMigrate lists
&UserWithdrawal{},

// model/option.go defaults
common.OptionMap["WithdrawalEnabled"] = "false"
common.OptionMap["WithdrawalMinAmount"] = "10"
common.OptionMap["WithdrawalInstruction"] = ""
common.OptionMap["WithdrawalFeeRules"] = "[]"
```

Run: `cd /Users/money/project/subproject/hermestoken && go test ./model -run 'Test(CreateUserWithdrawalFreezesQuotaAndStoresSnapshots|RejectApprovedWithdrawalReturnsFrozenQuota)'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git -C /Users/money/project/subproject/hermestoken add model/user.go model/main.go model/option.go model/user_withdrawal.go model/user_withdrawal_setting.go model/user_withdrawal_test.go
git -C /Users/money/project/subproject/hermestoken commit -m "feat: add wallet withdrawal domain model"
```

### Task 2: Implement Withdrawal Accounting and Query Helpers

**Files:**
- Modify: `model/user_withdrawal.go`
- Modify: `model/user.go`
- Test: `model/user_withdrawal_test.go`

- [ ] **Step 1: Write failing tests for single-open-order, list queries, and mark-paid behavior**

```go
func TestCreateUserWithdrawalRejectsSecondOpenOrder(t *testing.T) {
    db := setupTestDB(t)
    user := seedUserWithQuota(t, db, 2, 100000)
    seedPendingWithdrawal(t, db, user.Id, 100)

    _, err := CreateUserWithdrawal(&CreateUserWithdrawalParams{
        UserID:        user.Id,
        Amount:        50,
        AlipayAccount: "alice@example.com",
    })
    if err == nil {
        t.Fatal("expected second open withdrawal to fail")
    }
}

func TestMarkPaidConsumesFrozenQuotaWithoutTouchingAvailableQuota(t *testing.T) {
    db := setupTestDB(t)
    user := seedUserWithQuota(t, db, 3, 99900)
    withdrawal := seedApprovedWithdrawal(t, db, user.Id, 100)

    if err := MarkUserWithdrawalPaid(withdrawal.Id, 88, MarkUserWithdrawalPaidParams{PayReceiptNo: "ALI123"}); err != nil {
        t.Fatalf("MarkUserWithdrawalPaid returned error: %v", err)
    }

    refreshed, _ := GetUserById(user.Id, true)
    if refreshed.Quota != 99900 {
        t.Fatalf("quota = %d, want unchanged 99900", refreshed.Quota)
    }
    if refreshed.WithdrawFrozenQuota != 0 {
        t.Fatalf("withdraw_frozen_quota = %d, want 0", refreshed.WithdrawFrozenQuota)
    }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd /Users/money/project/subproject/hermestoken && go test ./model -run 'Test(CreateUserWithdrawalRejectsSecondOpenOrder|MarkUserWithdrawalPaidConsumesFrozenQuotaWithoutTouchingAvailableQuota)'`
Expected: FAIL because open-order guard and mark-paid helpers are not implemented yet.

- [ ] **Step 3: Implement transactional helpers and list/detail queries**

```go
type CreateUserWithdrawalParams struct {
    UserID         int
    Amount         float64
    AlipayAccount  string
    AlipayRealName string
}

type MarkUserWithdrawalPaidParams struct {
    PayReceiptNo  string
    PayReceiptURL string
    PaidNote      string
}

func CreateUserWithdrawal(params *CreateUserWithdrawalParams) (*UserWithdrawal, error) {
    return createUserWithdrawalTx(DB, params)
}

func ListUserWithdrawals(userID int, pageInfo *common.PageInfo) ([]*UserWithdrawal, int64, error) {
    var items []*UserWithdrawal
    var total int64
    query := DB.Model(&UserWithdrawal{}).Where("user_id = ?", userID)
    if err := query.Count(&total).Error; err != nil {
        return nil, 0, err
    }
    err := query.Order("id DESC").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&items).Error
    return items, total, err
}

func ListAdminWithdrawals(filter AdminWithdrawalFilter, pageInfo *common.PageInfo) ([]*UserWithdrawal, int64, error) {
    var items []*UserWithdrawal
    var total int64
    query := DB.Model(&UserWithdrawal{})
    if filter.Status != "" {
        query = query.Where("status = ?", filter.Status)
    }
    if filter.Keyword != "" {
        like := "%" + filter.Keyword + "%"
        query = query.Where("trade_no LIKE ? OR alipay_account LIKE ?", like, like)
    }
    if filter.UserID > 0 {
        query = query.Where("user_id = ?", filter.UserID)
    }
    if err := query.Count(&total).Error; err != nil {
        return nil, 0, err
    }
    err := query.Order("id DESC").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&items).Error
    return items, total, err
}
```

- [ ] **Step 4: Run the model test set**

Run: `cd /Users/money/project/subproject/hermestoken && go test ./model -run 'Test(CreateUserWithdrawal|RejectApprovedWithdrawal|MarkUserWithdrawalPaid|CreateUserWithdrawalRejectsSecondOpenOrder)'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git -C /Users/money/project/subproject/hermestoken add model/user_withdrawal.go model/user.go model/user_withdrawal_test.go
git -C /Users/money/project/subproject/hermestoken commit -m "feat: add wallet withdrawal accounting helpers"
```

### Task 3: Add User and Admin Withdrawal APIs

**Files:**
- Create: `controller/user_withdrawal.go`
- Modify: `controller/user.go`
- Modify: `router/api-router.go`
- Test: `controller/user_withdrawal_test.go`

- [ ] **Step 1: Write failing controller tests for submit/list/detail and admin transitions**

```go
func TestUserCreateWithdrawal(t *testing.T) {
    router, user := setupWithdrawalRouter(t)

    w := performAuthJSONRequest(router, user, "POST", "/api/user/withdrawals", `{
        "amount": 100,
        "alipay_account": "alice@example.com",
        "alipay_real_name": "Alice"
    }`)

    if w.Code != http.StatusOK {
        t.Fatalf("status = %d, want 200", w.Code)
    }
    assertJSONSuccess(t, w.Body.Bytes())
}

func TestAdminApproveRejectAndMarkPaidWithdrawal(t *testing.T) {
    router, admin, withdrawal := setupAdminWithdrawalRouter(t)

    approve := performAdminJSONRequest(router, admin, "POST", fmt.Sprintf("/api/admin/withdrawals/%d/approve", withdrawal.Id), `{"review_note":"ok"}`)
    if approve.Code != http.StatusOK {
        t.Fatalf("approve status = %d", approve.Code)
    }
}
```

- [ ] **Step 2: Run controller tests to verify they fail**

Run: `cd /Users/money/project/subproject/hermestoken && go test ./controller -run 'Test(UserCreateWithdrawal|AdminApproveRejectAndMarkPaidWithdrawal)'`
Expected: FAIL with missing handlers and routes.

- [ ] **Step 3: Implement controllers and route wiring**

```go
// controller/user_withdrawal.go
func GetUserWithdrawalConfig(c *gin.Context) {
    userID := c.GetInt("id")
    cfg := model.GetUserWithdrawalConfigView(userID)
    common.ApiSuccess(c, cfg)
}

func CreateUserWithdrawal(c *gin.Context) {
    var req struct {
        Amount         float64 `json:"amount"`
        AlipayAccount  string  `json:"alipay_account"`
        AlipayRealName string  `json:"alipay_real_name"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        common.ApiErrorMsg(c, "无效的提现参数")
        return
    }
    order, err := model.CreateUserWithdrawal(&model.CreateUserWithdrawalParams{
        UserID:         c.GetInt("id"),
        Amount:         req.Amount,
        AlipayAccount:  req.AlipayAccount,
        AlipayRealName: req.AlipayRealName,
    })
    if err != nil {
        common.ApiErrorMsg(c, err.Error())
        return
    }
    common.ApiSuccess(c, order)
}
```

```go
// router/api-router.go under selfRoute
selfRoute.GET("/withdrawal/config", controller.GetUserWithdrawalConfig)
selfRoute.GET("/withdrawals", controller.ListUserWithdrawals)
selfRoute.GET("/withdrawals/:id", controller.GetUserWithdrawal)
selfRoute.POST("/withdrawals", middleware.CriticalRateLimit(), controller.CreateUserWithdrawal)

// dedicated admin group to avoid /api/user route conflicts
withdrawalAdminRoute := apiRouter.Group("/admin/withdrawals")
withdrawalAdminRoute.Use(middleware.AdminAuth())
withdrawalAdminRoute.GET("", controller.AdminListWithdrawals)
withdrawalAdminRoute.GET("/:id", controller.AdminGetWithdrawal)
withdrawalAdminRoute.POST("/:id/approve", controller.AdminApproveWithdrawal)
withdrawalAdminRoute.POST("/:id/reject", controller.AdminRejectWithdrawal)
withdrawalAdminRoute.POST("/:id/mark-paid", controller.AdminMarkWithdrawalPaid)
```

- [ ] **Step 4: Expose frozen balance in self payload and verify tests pass**

```go
// controller/user.go GetSelf responseData additions
"withdraw_frozen_quota": user.WithdrawFrozenQuota,
```

Run: `cd /Users/money/project/subproject/hermestoken && go test ./controller -run 'Test(UserCreateWithdrawal|AdminApproveRejectAndMarkPaidWithdrawal)'`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git -C /Users/money/project/subproject/hermestoken add controller/user_withdrawal.go controller/user.go controller/user_withdrawal_test.go router/api-router.go
git -C /Users/money/project/subproject/hermestoken commit -m "feat: add wallet withdrawal api flows"
```

### Task 4: Add Withdrawal Settings to Root Payment Configuration

**Files:**
- Create: `web/src/pages/Setting/Payment/SettingsWithdrawal.jsx`
- Modify: `web/src/components/settings/PaymentSetting.jsx`
- Test: `web/tests/withdrawal-settings.test.mjs`

- [ ] **Step 1: Write the failing frontend settings assertions**

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';

test('payment settings render withdrawal settings card', () => {
  const source = fs.readFileSync('web/src/components/settings/PaymentSetting.jsx', 'utf8');
  assert.match(source, /SettingsWithdrawal/);
  assert.match(source, /WithdrawalEnabled/);
  assert.match(source, /WithdrawalFeeRules/);
});
```

- [ ] **Step 2: Run the frontend settings test to verify it fails**

Run: `cd /Users/money/project/subproject/hermestoken/web && node --test tests/withdrawal-settings.test.mjs`
Expected: FAIL because `SettingsWithdrawal` does not exist.

- [ ] **Step 3: Implement withdrawal settings card**

```jsx
export default function SettingsWithdrawal({ options, refresh }) {
  const [inputs, setInputs] = useState({
    WithdrawalEnabled: toBoolean(options.WithdrawalEnabled),
    WithdrawalMinAmount: parseFloat(options.WithdrawalMinAmount || 10),
    WithdrawalInstruction: options.WithdrawalInstruction || '',
    WithdrawalFeeRules: formatJSONOption(options.WithdrawalFeeRules || '[]'),
  });

  const submit = async () => {
    await API.put('/api/option/', { key: 'WithdrawalEnabled', value: inputs.WithdrawalEnabled });
    await API.put('/api/option/', { key: 'WithdrawalMinAmount', value: inputs.WithdrawalMinAmount });
    await API.put('/api/option/', { key: 'WithdrawalInstruction', value: inputs.WithdrawalInstruction });
    await API.put('/api/option/', { key: 'WithdrawalFeeRules', value: normalizeJSON(inputs.WithdrawalFeeRules) });
    refresh();
  };
}
```

- [ ] **Step 4: Wire it into payment settings and rerun tests**

Run: `cd /Users/money/project/subproject/hermestoken/web && node --test tests/withdrawal-settings.test.mjs`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git -C /Users/money/project/subproject/hermestoken add web/src/components/settings/PaymentSetting.jsx web/src/pages/Setting/Payment/SettingsWithdrawal.jsx web/tests/withdrawal-settings.test.mjs
git -C /Users/money/project/subproject/hermestoken commit -m "feat: add withdrawal payment settings"
```

### Task 5: Build User Wallet Withdrawal UI

**Files:**
- Create: `web/src/components/topup/WithdrawalCard.jsx`
- Create: `web/src/components/topup/modals/WithdrawalApplyModal.jsx`
- Create: `web/src/components/topup/modals/WithdrawalHistoryModal.jsx`
- Create: `web/src/helpers/withdrawal.js`
- Modify: `web/src/components/topup/index.jsx`
- Test: `web/tests/wallet-withdrawal.test.mjs`

- [ ] **Step 1: Write the failing wallet UI assertions**

```js
test('wallet topup page loads withdrawal config and renders withdrawal entry', () => {
  const source = fs.readFileSync('web/src/components/topup/index.jsx', 'utf8');
  assert.match(source, /\/api\/user\/withdrawal\/config/);
  assert.match(source, /WithdrawalCard/);
  assert.match(source, /WithdrawalApplyModal/);
  assert.match(source, /WithdrawalHistoryModal/);
});
```

- [ ] **Step 2: Run the wallet UI test to verify it fails**

Run: `cd /Users/money/project/subproject/hermestoken/web && node --test tests/wallet-withdrawal.test.mjs`
Expected: FAIL because the withdrawal UI files and API usage are absent.

- [ ] **Step 3: Implement wallet withdrawal helpers and components**

```jsx
// WithdrawalCard.jsx
const WithdrawalCard = ({ t, summary, onApply, onViewHistory }) => (
  <Card className='!rounded-2xl shadow-sm border-0'>
    <div className='grid grid-cols-3 gap-4'>
      <Metric label={t('当前余额')} value={summary.availableAmountText} />
      <Metric label={t('冻结中余额')} value={summary.frozenAmountText} />
      <Metric label={t('可提现余额')} value={summary.withdrawableAmountText} />
    </div>
    <div className='flex gap-2 mt-4'>
      <Button onClick={onApply}>{t('申请提现')}</Button>
      <Button theme='borderless' onClick={onViewHistory}>{t('提现记录')}</Button>
    </div>
  </Card>
);
```

- [ ] **Step 4: Fetch config in wallet page and rerun wallet tests**

```jsx
const [withdrawalConfig, setWithdrawalConfig] = useState(null);
const loadWithdrawalConfig = async () => {
  const res = await API.get('/api/user/withdrawal/config');
  if (res.data.success) {
    setWithdrawalConfig(res.data.data);
  }
};

useEffect(() => {
  loadWithdrawalConfig().then();
}, []);
```

Run: `cd /Users/money/project/subproject/hermestoken/web && node --test tests/wallet-withdrawal.test.mjs`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git -C /Users/money/project/subproject/hermestoken add web/src/components/topup/index.jsx web/src/components/topup/WithdrawalCard.jsx web/src/components/topup/modals/WithdrawalApplyModal.jsx web/src/components/topup/modals/WithdrawalHistoryModal.jsx web/src/helpers/withdrawal.js web/tests/wallet-withdrawal.test.mjs
git -C /Users/money/project/subproject/hermestoken commit -m "feat: add wallet withdrawal user interface"
```

### Task 6: Add Admin Withdrawal Management Page and Sidebar Entry

**Files:**
- Create: `web/src/pages/Withdrawal/index.jsx`
- Create: `web/src/components/table/withdrawals/index.jsx`
- Create: `web/src/components/table/withdrawals/WithdrawalsTable.jsx`
- Create: `web/src/components/table/withdrawals/WithdrawalsColumnDefs.jsx`
- Create: `web/src/components/table/withdrawals/WithdrawalsActions.jsx`
- Create: `web/src/components/table/withdrawals/WithdrawalsFilters.jsx`
- Create: `web/src/components/table/withdrawals/WithdrawalsDescription.jsx`
- Create: `web/src/components/table/withdrawals/modals/WithdrawalReviewModal.jsx`
- Create: `web/src/components/table/withdrawals/modals/WithdrawalPaidModal.jsx`
- Create: `web/src/hooks/withdrawals/useWithdrawalsData.jsx`
- Modify: `web/src/App.jsx`
- Modify: `web/src/components/layout/SiderBar.jsx`
- Modify: `web/src/hooks/common/useSidebar.js`
- Modify: `web/src/pages/Setting/Operation/SettingsSidebarModulesAdmin.jsx`
- Modify: `web/src/components/settings/personal/cards/NotificationSettings.jsx`
- Modify: `model/user.go`
- Test: `web/tests/withdrawal-admin-route.test.mjs`

- [ ] **Step 1: Write the failing route/sidebar tests**

```js
test('admin route tree exposes wallet withdrawal management', () => {
  const appSource = fs.readFileSync('web/src/App.jsx', 'utf8');
  const sidebarSource = fs.readFileSync('web/src/components/layout/SiderBar.jsx', 'utf8');
  assert.match(appSource, /const Withdrawal = lazyWithRetry\(/);
  assert.match(appSource, /path='\/console\/withdrawal'/);
  assert.match(sidebarSource, /withdrawal:\s*'\/console\/withdrawal'/);
  assert.match(sidebarSource, /text:\s*t\('提现管理'\)/);
});
```

- [ ] **Step 2: Run the route/sidebar tests to verify they fail**

Run: `cd /Users/money/project/subproject/hermestoken/web && node --test tests/withdrawal-admin-route.test.mjs`
Expected: FAIL because the route and sidebar entries do not exist.

- [ ] **Step 3: Implement the admin page shell and data hook**

```jsx
// web/src/pages/Withdrawal/index.jsx
import WithdrawalsTable from '../../components/table/withdrawals';

export default function Withdrawal() {
  return <div className='mt-[60px] px-2'><WithdrawalsTable /></div>;
}
```

```jsx
// useWithdrawalsData.jsx
const res = await API.get(`/api/admin/withdrawals?status=${status}&keyword=${keyword}&p=${page}&page_size=${pageSize}`);
setWithdrawals((res.data.data.items || []).map((item) => ({ ...item, key: item.id })));
```

- [ ] **Step 4: Wire route, sidebar, sidebar-settings, and rerun tests**

```jsx
// App.jsx
const Withdrawal = lazyWithRetry(() => import('./pages/Withdrawal'), 'withdrawal-route');
<Route path='/console/withdrawal' element={<AdminRoute>{renderWithSuspense(<Withdrawal />)}</AdminRoute>} />
```

```js
// SiderBar routerMap/adminItems
withdrawal: '/console/withdrawal'
{ text: t('提现管理'), itemKey: 'withdrawal', to: '/withdrawal' }
```

Run: `cd /Users/money/project/subproject/hermestoken/web && node --test tests/withdrawal-admin-route.test.mjs`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git -C /Users/money/project/subproject/hermestoken add web/src/pages/Withdrawal/index.jsx web/src/components/table/withdrawals web/src/hooks/withdrawals/useWithdrawalsData.jsx web/src/App.jsx web/src/components/layout/SiderBar.jsx web/src/hooks/common/useSidebar.js web/src/pages/Setting/Operation/SettingsSidebarModulesAdmin.jsx web/src/components/settings/personal/cards/NotificationSettings.jsx model/user.go web/tests/withdrawal-admin-route.test.mjs
git -C /Users/money/project/subproject/hermestoken commit -m "feat: add admin withdrawal management page"
```

### Task 7: Add Localization, Verify, and Ship-Readiness Checks

**Files:**
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh-CN.json`
- Modify: `web/src/i18n/locales/zh-TW.json`
- Modify: `web/src/i18n/locales/ja.json`
- Modify: `web/src/i18n/locales/fr.json`
- Modify: `web/src/i18n/locales/ru.json`
- Modify: `web/src/i18n/locales/vi.json`
- Create: `web/tests/withdrawal-locales.test.mjs`

- [ ] **Step 1: Write the failing locale coverage test**

```js
test('withdrawal locales define wallet and admin copy', () => {
  const keys = ['申请提现', '提现记录', '提现管理', '冻结中余额', '确认已打款', '驳回原因'];
  for (const locale of ['en', 'zh-CN', 'zh-TW', 'ja', 'fr', 'ru', 'vi']) {
    const raw = fs.readFileSync(`web/src/i18n/locales/${locale}.json`, 'utf8');
    for (const key of keys) {
      assert.match(raw, new RegExp(`"${key}"`));
    }
  }
});
```

- [ ] **Step 2: Run the locale test to verify it fails**

Run: `cd /Users/money/project/subproject/hermestoken/web && node --test tests/withdrawal-locales.test.mjs`
Expected: FAIL because the new keys are not in every locale.

- [ ] **Step 3: Add localization keys and rerun source tests**

```json
{
  "申请提现": "Apply for withdrawal",
  "提现记录": "Withdrawal history",
  "提现管理": "Withdrawal Management",
  "冻结中余额": "Frozen balance",
  "确认已打款": "Mark as paid",
  "驳回原因": "Rejection reason"
}
```

- [ ] **Step 4: Run full targeted verification**

Run: `cd /Users/money/project/subproject/hermestoken && go test ./model ./controller -run 'Test(CreateUserWithdrawal|RejectApprovedWithdrawal|MarkUserWithdrawalPaid|UserCreateWithdrawal|AdminApproveRejectAndMarkPaidWithdrawal)'`
Expected: PASS

Run: `cd /Users/money/project/subproject/hermestoken/web && node --test tests/withdrawal-settings.test.mjs tests/wallet-withdrawal.test.mjs tests/withdrawal-admin-route.test.mjs tests/withdrawal-locales.test.mjs`
Expected: PASS

Run: `cd /Users/money/project/subproject/hermestoken/web && bun run build`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git -C /Users/money/project/subproject/hermestoken add web/src/i18n/locales/en.json web/src/i18n/locales/zh-CN.json web/src/i18n/locales/zh-TW.json web/src/i18n/locales/ja.json web/src/i18n/locales/fr.json web/src/i18n/locales/ru.json web/src/i18n/locales/vi.json web/tests/withdrawal-locales.test.mjs
git -C /Users/money/project/subproject/hermestoken commit -m "feat: add wallet withdrawal localization"
```

## Self-Review

### Spec coverage

- Wallet balance withdrawal target: Task 1 + Task 2 + Task 5
- Frozen balance accounting: Task 1 + Task 2 + Task 3
- Manual Alipay payout with admin approval: Task 2 + Task 3 + Task 6
- Configurable minimum amount and fee rules: Task 1 + Task 4
- User wallet entry and history: Task 5
- Admin page, sidebar, approval, payout confirmation: Task 6
- Optional receipt fields: Task 2 + Task 3 + Task 6
- Pagination and locale coverage: Task 5 + Task 6 + Task 7

### Placeholder scan

- No `TBD`, `TODO`, or “similar to Task N” placeholders remain.
- Every task includes exact file paths, at least one concrete test snippet, explicit commands, and expected outcomes.

### Type consistency

- `WithdrawFrozenQuota`, `UserWithdrawalStatus*`, `CreateUserWithdrawalParams`, and `MarkUserWithdrawalPaidParams` are introduced before later tasks use them.
- API naming stays consistent with `/api/user/withdrawals` and `/api/admin/withdrawals`.
- Review/reject/paid note fields stay aligned with the refined spec (`review_note`, `rejection_note`, `paid_note`).
