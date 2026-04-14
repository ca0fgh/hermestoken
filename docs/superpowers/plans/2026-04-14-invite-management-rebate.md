# Invite Management Rebate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a new user-facing `邀请管理 -> 邀请返佣` area that lets inviters review their invitees, see each invitee's cumulative contribution, edit grouped default subscription referral rules, and override grouped subscription referral rates per invitee.

**Architecture:** Keep the existing three-layer referral model intact: admin total-rate ceilings stay in the admin override flow, inviter defaults stay in `UserSetting.SubscriptionReferralInviteeRateBpsByGroup`, and a new invitee-specific override table adds the lowest-precedence layer. Add self-service API endpoints in the subscription referral controller domain, then wire a new standalone React page into the sidebar and sidebar settings system without folding this work back into `TopUp` or `PersonalSetting`.

**Tech Stack:** Go 1.26 + Gin + GORM + existing `common.PageInfo` pagination, React 18 + Semi UI + Vite, Node built-in source tests for frontend regression coverage, existing i18n JSON locale files

---

### Task 1: Add The New Sidebar Section And Route Skeleton

**Files:**
- Create: `web/src/pages/InviteRebate/index.js`
- Modify: `web/src/App.jsx`
- Modify: `web/src/components/layout/SiderBar.jsx`
- Modify: `web/src/helpers/sidebarIcons.jsx`
- Modify: `web/src/hooks/common/useSidebar.js`
- Modify: `web/src/components/settings/personal/cards/NotificationSettings.jsx`
- Modify: `controller/user.go`
- Modify: `controller/user_setting_test.go`
- Test: `web/tests/invite-management-sidebar.test.mjs`

- [ ] **Step 1: Write the failing sidebar and default-config regression tests**

```js
// web/tests/invite-management-sidebar.test.mjs
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

test('app exposes the invite rebate route', () => {
  const appSource = readFileSync(
    new URL('../src/App.jsx', import.meta.url),
    'utf8',
  );

  assert.match(appSource, /const InviteRebate = lazy\(\(\) => import\('\.\/pages\/InviteRebate'\)\)/);
  assert.match(appSource, /path='\/console\/invite\/rebate'/);
});

test('sidebar renders invite management as a standalone group', () => {
  const sidebarSource = readFileSync(
    new URL('../src/components/layout/SiderBar.jsx', import.meta.url),
    'utf8',
  );

  assert.match(sidebarSource, /rebate:\s*'\/console\/invite\/rebate'/);
  assert.match(sidebarSource, /const inviteItems = useMemo/);
  assert.match(sidebarSource, /t\('邀请管理'\)/);
  assert.match(sidebarSource, /t\('邀请返佣'\)/);
});

test('sidebar settings expose the invite section and rebate toggle', () => {
  const notificationSettingsSource = readFileSync(
    new URL('../src/components/settings/personal/cards/NotificationSettings.jsx', import.meta.url),
    'utf8',
  );
  const useSidebarSource = readFileSync(
    new URL('../src/hooks/common/useSidebar.js', import.meta.url),
    'utf8',
  );

  assert.match(useSidebarSource, /invite:\s*\{\s*enabled:\s*true,\s*rebate:\s*true/s);
  assert.match(notificationSettingsSource, /key:\s*'invite'/);
  assert.match(notificationSettingsSource, /key:\s*'rebate'/);
  assert.match(notificationSettingsSource, /t\('邀请管理区域'\)/);
  assert.match(notificationSettingsSource, /t\('邀请返佣'\)/);
});
```

```go
// controller/user_setting_test.go
func TestGenerateDefaultSidebarConfigIncludesInviteSection(t *testing.T) {
	configJSON := generateDefaultSidebarConfig(common.RoleCommonUser)

	var parsed map[string]map[string]any
	if err := json.Unmarshal([]byte(configJSON), &parsed); err != nil {
		t.Fatalf("failed to parse generated sidebar config: %v", err)
	}

	inviteSection, ok := parsed["invite"]
	if !ok {
		t.Fatal("expected invite section in generated sidebar config")
	}
	if inviteSection["enabled"] != true {
		t.Fatalf("invite.enabled = %#v, want true", inviteSection["enabled"])
	}
	if inviteSection["rebate"] != true {
		t.Fatalf("invite.rebate = %#v, want true", inviteSection["rebate"])
	}
}
```

- [ ] **Step 2: Run the focused sidebar tests and verify they fail first**

Run:

```bash
node --test web/tests/invite-management-sidebar.test.mjs
go test ./controller -run TestGenerateDefaultSidebarConfigIncludesInviteSection -count=1
```

Expected:

- The web test fails because the route, sidebar section, and settings toggle do not exist yet.
- The Go test fails because `generateDefaultSidebarConfig` does not include `invite`.

- [ ] **Step 3: Implement the sidebar section, route shell, and config defaults**

```jsx
// web/src/App.jsx
const InviteRebate = lazy(() => import('./pages/InviteRebate'));

<Route
  path='/console/invite/rebate'
  element={
    <PrivateRoute>{renderWithSuspense(<InviteRebate />)}</PrivateRoute>
  }
/>
```

```jsx
// web/src/components/layout/SiderBar.jsx
const routerMap = {
  topup: '/console/topup',
  personal: '/console/personal',
  rebate: '/console/invite/rebate',
};

const inviteItems = useMemo(
  () =>
    [
      {
        text: t('邀请返佣'),
        itemKey: 'rebate',
        to: '/invite/rebate',
      },
    ].filter((item) => isModuleVisible('invite', item.itemKey)),
  [t, isModuleVisible],
);

{hasSectionVisibleModules('invite') && (
  <>
    <Divider className='sidebar-divider' />
    <div>
      {!collapsed && (
        <div className='sidebar-group-label'>{t('邀请管理')}</div>
      )}
      {inviteItems.map((item) => renderNavItem(item))}
    </div>
  </>
)}
```

```jsx
// web/src/helpers/sidebarIcons.jsx
case 'rebate':
  return <Gift {...commonProps} color={iconColor} />;
```

```js
// web/src/hooks/common/useSidebar.js
export const DEFAULT_ADMIN_CONFIG = {
  chat: { enabled: true, playground: true, chat: true },
  console: { enabled: true, detail: true, token: true, log: true, midjourney: true, task: true },
  invite: { enabled: true, rebate: true },
  personal: { enabled: true, topup: true, personal: true },
  admin: { enabled: true, channel: true, models: true, deployment: true, redemption: true, user: true, subscription: true, setting: true },
};
```

```go
// controller/user.go
defaultConfig["invite"] = map[string]interface{}{
	"enabled": true,
	"rebate":  true,
}
```

```js
// web/src/pages/InviteRebate/index.js
import React from 'react';

const InviteRebate = () => {
  return <div>Invite rebate page placeholder</div>;
};

export default InviteRebate;
```

- [ ] **Step 4: Re-run the focused sidebar tests and confirm they pass**

Run:

```bash
node --test web/tests/invite-management-sidebar.test.mjs
go test ./controller -run TestGenerateDefaultSidebarConfigIncludesInviteSection -count=1
```

Expected:

- PASS for both tests, with the route, standalone sidebar section, and default sidebar config now present.

- [ ] **Step 5: Commit the navigation scaffold**

```bash
git add web/src/pages/InviteRebate/index.js web/src/App.jsx web/src/components/layout/SiderBar.jsx web/src/helpers/sidebarIcons.jsx web/src/hooks/common/useSidebar.js web/src/components/settings/personal/cards/NotificationSettings.jsx controller/user.go controller/user_setting_test.go web/tests/invite-management-sidebar.test.mjs
git commit -m "feat: add invite management sidebar scaffold"
```

### Task 2: Add Invitee Override Persistence And Contribution Queries

**Files:**
- Create: `model/subscription_referral_invitee_override.go`
- Create: `model/subscription_referral_invitee_override_test.go`
- Modify: `model/main.go`
- Modify: `model/subscription_referral.go`

- [ ] **Step 1: Write the failing model tests for grouped invitee overrides and contribution aggregation**

```go
// model/subscription_referral_invitee_override_test.go
func TestUpsertSubscriptionReferralInviteeOverrideStoresGroupedRate(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	inviter := seedReferralUser(t, db, "inviter", 0, dto.UserSetting{})
	invitee := seedReferralUser(t, db, "invitee", inviter.Id, dto.UserSetting{})

	override, err := UpsertSubscriptionReferralInviteeOverride(inviter.Id, invitee.Id, "vip", 1500)
	if err != nil {
		t.Fatalf("UpsertSubscriptionReferralInviteeOverride() error = %v", err)
	}
	if override.InviterUserId != inviter.Id || override.InviteeUserId != invitee.Id {
		t.Fatalf("unexpected override owner pair: %+v", override)
	}
	if override.InviteeRateBps != 1500 {
		t.Fatalf("InviteeRateBps = %d, want 1500", override.InviteeRateBps)
	}
}

func TestListSubscriptionReferralInviteeContributionSummariesUsesNetInviterReward(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	inviter := seedReferralUser(t, db, "sum-inviter", 0, dto.UserSetting{})
	invitee := seedReferralUser(t, db, "sum-invitee", inviter.Id, dto.UserSetting{})

	record := &SubscriptionReferralRecord{
		OrderId:           1,
		OrderTradeNo:      "trade-agg-1",
		PlanId:            1,
		ReferralGroup:     "vip",
		PayerUserId:       invitee.Id,
		InviterUserId:     inviter.Id,
		BeneficiaryUserId: inviter.Id,
		BeneficiaryRole:   SubscriptionReferralBeneficiaryRoleInviter,
		RewardQuota:       500,
		ReversedQuota:     100,
		DebtQuota:         50,
		Status:            SubscriptionReferralStatusPartialRevert,
	}
	if err := db.Create(record).Error; err != nil {
		t.Fatalf("failed to create record: %v", err)
	}

	pageInfo := &common.PageInfo{Page: 1, PageSize: 20}
	items, total, contributionTotal, err := ListSubscriptionReferralInviteeContributionSummaries(inviter.Id, "", pageInfo)
	if err != nil {
		t.Fatalf("ListSubscriptionReferralInviteeContributionSummaries() error = %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if contributionTotal != 350 {
		t.Fatalf("contributionTotal = %d, want 350", contributionTotal)
	}
	if len(items) != 1 || items[0].ContributionQuota != 350 {
		t.Fatalf("unexpected items: %+v", items)
	}
}
```

- [ ] **Step 2: Run the focused model tests and verify they fail**

Run:

```bash
go test ./model -run 'Test(UpsertSubscriptionReferralInviteeOverrideStoresGroupedRate|ListSubscriptionReferralInviteeContributionSummariesUsesNetInviterReward)' -count=1
```

Expected:

- FAIL because the invitee override model and contribution aggregation helpers do not exist.

- [ ] **Step 3: Implement the new model, migration, and contribution query helpers**

```go
// model/subscription_referral_invitee_override.go
type SubscriptionReferralInviteeOverride struct {
	Id             int   `json:"id"`
	InviterUserId  int   `json:"inviter_user_id" gorm:"uniqueIndex:idx_sub_referral_invitee_override;index"`
	InviteeUserId  int   `json:"invitee_user_id" gorm:"uniqueIndex:idx_sub_referral_invitee_override;index"`
	Group          string `json:"group" gorm:"type:varchar(64);uniqueIndex:idx_sub_referral_invitee_override"`
	InviteeRateBps int   `json:"invitee_rate_bps" gorm:"type:int;not null;default:0"`
	CreatedAt      int64 `json:"created_at" gorm:"bigint"`
	UpdatedAt      int64 `json:"updated_at" gorm:"bigint"`
}

type SubscriptionReferralInviteeContributionSummary struct {
	Id                 int    `json:"id"`
	Username           string `json:"username"`
	Group              string `json:"group"`
	ContributionQuota  int64  `json:"contribution_quota"`
	OverrideGroupCount int    `json:"override_group_count"`
}

func UpsertSubscriptionReferralInviteeOverride(inviterUserId int, inviteeUserId int, group string, inviteeRateBps int) (*SubscriptionReferralInviteeOverride, error) {
	if inviterUserId <= 0 || inviteeUserId <= 0 {
		return nil, errors.New("invalid inviter or invitee id")
	}
	trimmedGroup := strings.TrimSpace(group)
	if trimmedGroup == "" {
		return nil, errors.New("group is required")
	}

	override := &SubscriptionReferralInviteeOverride{
		InviterUserId:  inviterUserId,
		InviteeUserId:  inviteeUserId,
		Group:          trimmedGroup,
		InviteeRateBps: NormalizeSubscriptionReferralRateBps(inviteeRateBps),
	}

	if err := DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "inviter_user_id"},
			{Name: "invitee_user_id"},
			{Name: commonGroupCol},
		},
		DoUpdates: clause.AssignmentColumns([]string{"invitee_rate_bps", "updated_at"}),
	}).Create(override).Error; err != nil {
		return nil, err
	}

	if err := DB.Where(
		"inviter_user_id = ? AND invitee_user_id = ? AND "+commonGroupCol+" = ?",
		inviterUserId,
		inviteeUserId,
		trimmedGroup,
	).First(override).Error; err != nil {
		return nil, err
	}
	return override, nil
}

func DeleteSubscriptionReferralInviteeOverride(inviterUserId int, inviteeUserId int, group string) error {
	return DB.Where(
		"inviter_user_id = ? AND invitee_user_id = ? AND "+commonGroupCol+" = ?",
		inviterUserId,
		inviteeUserId,
		strings.TrimSpace(group),
	).Delete(&SubscriptionReferralInviteeOverride{}).Error
}

func ListSubscriptionReferralInviteeOverrides(inviterUserId int, inviteeUserId int) ([]SubscriptionReferralInviteeOverride, error) {
	var overrides []SubscriptionReferralInviteeOverride
	err := DB.Where("inviter_user_id = ? AND invitee_user_id = ?", inviterUserId, inviteeUserId).
		Order(commonGroupCol + " asc").
		Find(&overrides).Error
	return overrides, err
}

func ListSubscriptionReferralInviteeContributionSummaries(inviterUserId int, keyword string, pageInfo *common.PageInfo) ([]SubscriptionReferralInviteeContributionSummary, int64, int64, error) {
	base := DB.Table("users").
		Select(`
			users.id,
			users.username,
			users.group,
			COALESCE(SUM(subscription_referral_records.reward_quota - subscription_referral_records.reversed_quota - subscription_referral_records.debt_quota), 0) AS contribution_quota,
			COUNT(subscription_referral_invitee_overrides.id) AS override_group_count
		`).
		Joins("LEFT JOIN subscription_referral_records ON subscription_referral_records.payer_user_id = users.id AND subscription_referral_records.inviter_user_id = ? AND subscription_referral_records.beneficiary_role = ?", inviterUserId, SubscriptionReferralBeneficiaryRoleInviter).
		Joins("LEFT JOIN subscription_referral_invitee_overrides ON subscription_referral_invitee_overrides.invitee_user_id = users.id AND subscription_referral_invitee_overrides.inviter_user_id = ?", inviterUserId).
		Where("users.inviter_id = ?", inviterUserId).
		Group("users.id, users.username, users.group")

	if keyword != "" {
		base = base.Where(
			"CAST(users.id AS TEXT) LIKE ? OR users.username LIKE ?",
			"%"+keyword+"%",
			"%"+keyword+"%",
		)
	}

	var total int64
	if err := DB.Table("(?) as invitee_rows", base).Count(&total).Error; err != nil {
		return nil, 0, 0, err
	}

	var contributionTotal int64
	if err := DB.Table("(?) as invitee_rows", base).
		Select("COALESCE(SUM(contribution_quota), 0)").
		Scan(&contributionTotal).Error; err != nil {
		return nil, 0, 0, err
	}

	var items []SubscriptionReferralInviteeContributionSummary
	err := base.Order("contribution_quota desc, users.id asc").
		Offset(pageInfo.GetStartIdx()).
		Limit(pageInfo.GetPageSize()).
		Scan(&items).Error
	return items, total, contributionTotal, err
}
```

```go
// model/main.go
if err := DB.AutoMigrate(
	&SubscriptionReferralOverride{},
	&SubscriptionReferralRecord{},
	&SubscriptionReferralInviteeOverride{},
); err != nil {
	return err
}
```

- [ ] **Step 4: Re-run the focused model tests and confirm they pass**

Run:

```bash
go test ./model -run 'Test(UpsertSubscriptionReferralInviteeOverrideStoresGroupedRate|ListSubscriptionReferralInviteeContributionSummariesUsesNetInviterReward)' -count=1
```

Expected:

- PASS with grouped invitee overrides persisted and inviter-side contribution aggregated as net reward.

- [ ] **Step 5: Commit the model layer**

```bash
git add model/subscription_referral_invitee_override.go model/subscription_referral_invitee_override_test.go model/main.go model/subscription_referral.go
git commit -m "feat: add invitee referral override model support"
```

### Task 3: Add Self-Service Referral APIs For Defaults, Invitee Lists, Details, And Mutations

**Files:**
- Modify: `router/api-router.go`
- Modify: `controller/subscription_referral.go`
- Create: `controller/subscription_referral_invitee_test.go`
- Modify: `controller/subscription_referral_test.go`

- [ ] **Step 1: Write the failing controller tests for default deletion, invitee listing, ownership checks, and invitee override writes**

```go
// controller/subscription_referral_invitee_test.go
func TestDeleteSubscriptionReferralSelfRemovesGroupedDefaultRate(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	common.SubscriptionReferralEnabled = true
	if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{"vip":4500}`); err != nil {
		t.Fatalf("failed to seed rates: %v", err)
	}

	user := seedSubscriptionReferralControllerUser(t, "default-delete", 0, dto.UserSetting{
		SubscriptionReferralInviteeRateBpsByGroup: map[string]int{"vip": 1200},
	})

	ctx, recorder := newAuthenticatedContext(t, http.MethodDelete, "/api/user/referral/subscription?group=vip", nil, user.Id)
	ctx.Request.URL.RawQuery = "group=vip"
	DeleteSubscriptionReferralSelf(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected success, got %s", resp.Message)
	}

	after, _ := model.GetUserById(user.Id, true)
	if _, ok := after.GetSetting().SubscriptionReferralInviteeRateBpsByGroup["vip"]; ok {
		t.Fatal("expected vip default rule to be deleted")
	}
}

func TestListSubscriptionReferralInviteesReturnsOnlyOwnedInvitees(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	inviter := seedSubscriptionReferralControllerUser(t, "owner", 0, dto.UserSetting{})
	seedSubscriptionReferralControllerUser(t, "owned-a", inviter.Id, dto.UserSetting{})
	seedSubscriptionReferralControllerUser(t, "owned-b", inviter.Id, dto.UserSetting{})
	other := seedSubscriptionReferralControllerUser(t, "other-owner", 0, dto.UserSetting{})
	seedSubscriptionReferralControllerUser(t, "foreign", other.Id, dto.UserSetting{})

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/referral/subscription/invitees?p=1&page_size=20", nil, inviter.Id)
	ctx.Request.URL.RawQuery = "p=1&page_size=20"
	ListSubscriptionReferralInvitees(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatalf("expected success, got %s", resp.Message)
	}
	var page struct {
		Total int `json:"total"`
	}
	body, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("failed to marshal page payload: %v", err)
	}
	if err := json.Unmarshal(body, &page); err != nil {
		t.Fatalf("failed to unmarshal page payload: %v", err)
	}
	if page.Total != 2 {
		t.Fatalf("page.Total = %d, want 2", page.Total)
	}
}

func TestUpsertSubscriptionReferralInviteeOverrideRejectsForeignInvitee(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	common.SubscriptionReferralEnabled = true
	if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{"vip":4500}`); err != nil {
		t.Fatalf("failed to seed rates: %v", err)
	}

	inviter := seedSubscriptionReferralControllerUser(t, "owner", 0, dto.UserSetting{})
	foreignOwner := seedSubscriptionReferralControllerUser(t, "foreign-owner", 0, dto.UserSetting{})
	foreignInvitee := seedSubscriptionReferralControllerUser(t, "foreign-invitee", foreignOwner.Id, dto.UserSetting{})

	body := map[string]any{"group": "vip", "invitee_rate_bps": 900}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, fmt.Sprintf("/api/user/referral/subscription/invitees/%d", foreignInvitee.Id), body, inviter.Id)
	UpsertSubscriptionReferralInviteeOverrideSelf(ctx)

	resp := decodeAPIResponse(t, recorder)
	if resp.Success {
		t.Fatal("expected ownership validation failure")
	}
}
```

- [ ] **Step 2: Run the focused controller tests and verify they fail first**

Run:

```bash
go test ./controller -run 'Test(DeleteSubscriptionReferralSelfRemovesGroupedDefaultRate|ListSubscriptionReferralInviteesReturnsOnlyOwnedInvitees|UpsertSubscriptionReferralInviteeOverrideRejectsForeignInvitee)' -count=1
```

Expected:

- FAIL because the new delete/list/detail/mutation handlers and routes do not exist.

- [ ] **Step 3: Implement the self-service handlers and router bindings**

```go
// controller/subscription_referral.go
func DeleteSubscriptionReferralSelf(c *gin.Context) {
	userID := c.GetInt("id")
	group := strings.TrimSpace(c.Query("group"))
	if group == "" {
		common.ApiErrorMsg(c, "分组不能为空")
		return
	}

	user, err := model.GetUserById(userID, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	setting := user.GetSetting()
	nextByGroup := copySubscriptionReferralInviteeRatesByGroup(setting.SubscriptionReferralInviteeRateBpsByGroup)
	delete(nextByGroup, group)
	setting.SubscriptionReferralInviteeRateBpsByGroup = nextByGroup
	user.SetSetting(setting)
	if err := user.Update(false); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{"group": group})
}

func ListSubscriptionReferralInvitees(c *gin.Context) {
	userID := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	keyword := strings.TrimSpace(c.Query("keyword"))

	items, total, contributionTotal, err := model.ListSubscriptionReferralInviteeContributionSummaries(userID, keyword, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, gin.H{
		"page":                     pageInfo.Page,
		"page_size":                pageInfo.PageSize,
		"total":                    pageInfo.Total,
		"items":                    items,
		"invitee_count":            total,
		"total_contribution_quota": contributionTotal,
	})
}

func GetSubscriptionReferralInviteeDetail(c *gin.Context) {
	userID := c.GetInt("id")
	inviteeID, _ := strconv.Atoi(c.Param("invitee_id"))

	invitee, err := model.GetUserById(inviteeID, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if invitee.InviterId != userID {
		common.ApiErrorMsg(c, "无权限查看该被邀请人")
		return
	}

	inviter, err := model.GetUserById(userID, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	overrides, err := model.ListSubscriptionReferralInviteeOverrides(userID, inviteeID)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	groups := model.ListSubscriptionReferralConfiguredGroups()
	effectiveTotals := make(map[string]int, len(groups))
	for _, group := range groups {
		effectiveTotals[group] = model.GetEffectiveSubscriptionReferralTotalRateBps(userID, group)
	}

	common.ApiSuccess(c, gin.H{
		"invitee": gin.H{
			"id":       invitee.Id,
			"username": invitee.Username,
			"group":    invitee.Group,
		},
		"available_groups":                  groups,
		"default_invitee_rate_bps_by_group": inviter.GetSetting().SubscriptionReferralInviteeRateBpsByGroup,
		"effective_total_rate_bps_by_group": effectiveTotals,
		"overrides":                         overrides,
	})
}

func UpsertSubscriptionReferralInviteeOverrideSelf(c *gin.Context) {
	userID := c.GetInt("id")
	inviteeID, _ := strconv.Atoi(c.Param("invitee_id"))

	var req UpdateSubscriptionReferralSelfRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	invitee, err := model.GetUserById(inviteeID, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if invitee.InviterId != userID {
		common.ApiErrorMsg(c, "无权限修改该被邀请人")
		return
	}

	group := strings.TrimSpace(req.Group)
	totalRateBps := model.GetEffectiveSubscriptionReferralTotalRateBps(userID, group)
	if totalRateBps <= 0 || req.InviteeRateBps < 0 || req.InviteeRateBps > totalRateBps {
		common.ApiErrorMsg(c, "被邀请人比例不能超过总返佣率")
		return
	}

	override, err := model.UpsertSubscriptionReferralInviteeOverride(userID, inviteeID, group, req.InviteeRateBps)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, override)
}

func DeleteSubscriptionReferralInviteeOverrideSelf(c *gin.Context) {
	userID := c.GetInt("id")
	inviteeID, _ := strconv.Atoi(c.Param("invitee_id"))
	group := strings.TrimSpace(c.Query("group"))

	invitee, err := model.GetUserById(inviteeID, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if invitee.InviterId != userID {
		common.ApiErrorMsg(c, "无权限修改该被邀请人")
		return
	}
	if group == "" {
		common.ApiErrorMsg(c, "分组不能为空")
		return
	}
	if err := model.DeleteSubscriptionReferralInviteeOverride(userID, inviteeID, group); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{"invitee_id": inviteeID, "group": group})
}
```

```go
// router/api-router.go
selfRoute.GET("/referral/subscription/invitees", controller.ListSubscriptionReferralInvitees)
selfRoute.GET("/referral/subscription/invitees/:invitee_id", controller.GetSubscriptionReferralInviteeDetail)
selfRoute.PUT("/referral/subscription/invitees/:invitee_id", controller.UpsertSubscriptionReferralInviteeOverrideSelf)
selfRoute.DELETE("/referral/subscription/invitees/:invitee_id", controller.DeleteSubscriptionReferralInviteeOverrideSelf)
selfRoute.DELETE("/referral/subscription", controller.DeleteSubscriptionReferralSelf)
```

- [ ] **Step 4: Re-run the focused controller tests and confirm they pass**

Run:

```bash
go test ./controller -run 'Test(DeleteSubscriptionReferralSelfRemovesGroupedDefaultRate|ListSubscriptionReferralInviteesReturnsOnlyOwnedInvitees|UpsertSubscriptionReferralInviteeOverrideRejectsForeignInvitee)' -count=1
```

Expected:

- PASS with default deletion using `DELETE`, invitee paging restricted to owned invitees, and foreign invitee writes rejected.

- [ ] **Step 5: Commit the API layer**

```bash
git add router/api-router.go controller/subscription_referral.go controller/subscription_referral_test.go controller/subscription_referral_invitee_test.go
git commit -m "feat: add self-service invite rebate APIs"
```

### Task 4: Add Frontend Parsing Helpers For Default Rules, Invitee Lists, And Override Panels

**Files:**
- Create: `web/src/helpers/inviteRebate.js`
- Create: `web/tests/invite-rebate.test.mjs`
- Modify: `web/src/helpers/index.js`

- [ ] **Step 1: Write the failing frontend helper tests for list parsing and grouped override rows**

```js
// web/tests/invite-rebate.test.mjs
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  buildInviteDefaultRuleRows,
  buildInviteeOverrideRows,
  normalizeInviteeContributionPage,
} from '../src/helpers/inviteRebate.js';

test('normalizeInviteeContributionPage keeps invitee counters and page items', () => {
  const page = normalizeInviteeContributionPage({
    invitee_count: 2,
    total_contribution_quota: 350,
    page: 1,
    page_size: 20,
    total: 2,
    items: [{ id: 11, username: 'alice', contribution_quota: 200 }],
  });

  assert.equal(page.inviteeCount, 2);
  assert.equal(page.totalContributionQuota, 350);
  assert.equal(page.items[0].username, 'alice');
});

test('buildInviteDefaultRuleRows maps grouped inviter defaults into editable list rows', () => {
  const rows = buildInviteDefaultRuleRows(
    [{ group: 'vip', total_rate_bps: 4500, invitee_rate_bps: 1200 }],
    ['vip'],
  );

  assert.equal(rows.length, 1);
  assert.equal(rows[0].type, 'subscription');
  assert.equal(rows[0].group, 'vip');
  assert.equal(rows[0].inputPercent, 12);
});

test('buildInviteeOverrideRows keeps persisted invitee overrides and total-rate hints aligned by group', () => {
  const rows = buildInviteeOverrideRows({
    available_groups: ['vip'],
    effective_total_rate_bps_by_group: { vip: 4500 },
    default_invitee_rate_bps_by_group: { vip: 1200 },
    overrides: [{ group: 'vip', invitee_rate_bps: 1500 }],
  });

  assert.equal(rows[0].group, 'vip');
  assert.equal(rows[0].effectiveTotalRateBps, 4500);
  assert.equal(rows[0].defaultInviteeRateBps, 1200);
  assert.equal(rows[0].inputPercent, 15);
});
```

- [ ] **Step 2: Run the helper tests and verify they fail**

Run:

```bash
node --test web/tests/invite-rebate.test.mjs
```

Expected:

- FAIL because `inviteRebate.js` does not exist yet.

- [ ] **Step 3: Implement the dedicated invite rebate helper module**

```js
// web/src/helpers/inviteRebate.js
import {
  percentNumberToRateBps,
  rateBpsToPercentNumber,
} from './subscriptionReferral';

export function normalizeInviteeContributionPage(payload = {}) {
  return {
    page: Number(payload.page || 1),
    pageSize: Number(payload.page_size || 20),
    total: Number(payload.total || 0),
    inviteeCount: Number(payload.invitee_count || 0),
    totalContributionQuota: Number(payload.total_contribution_quota || 0),
    items: Array.isArray(payload.items) ? payload.items : [],
  };
}

export function buildInviteDefaultRuleRows(groups = [], availableGroups = []) {
  return (groups || [])
    .filter((item) => availableGroups.includes(String(item.group || '').trim()))
    .map((item) => ({
      id: `default-${item.group}`,
      type: 'subscription',
      group: item.group,
      inputPercent: rateBpsToPercentNumber(item.invitee_rate_bps || 0),
      effectiveTotalRateBps: Number(item.total_rate_bps || 0),
      hasOverride: true,
      isDraft: false,
    }));
}

export function buildInviteeOverrideRows(detail = {}) {
  const overrides = Array.isArray(detail.overrides) ? detail.overrides : [];
  return overrides.map((item) => ({
    id: `invitee-${detail.invitee?.id}-${item.group}`,
    type: 'subscription',
    group: item.group,
    inputPercent: rateBpsToPercentNumber(item.invitee_rate_bps || 0),
    effectiveTotalRateBps: Number(detail.effective_total_rate_bps_by_group?.[item.group] || 0),
    defaultInviteeRateBps: Number(detail.default_invitee_rate_bps_by_group?.[item.group] || 0),
    hasOverride: true,
    isDraft: false,
  }));
}

export function toInviteeRatePayload(row) {
  return {
    group: row.group,
    invitee_rate_bps: percentNumberToRateBps(row.inputPercent),
  };
}
```

- [ ] **Step 4: Re-run the helper tests and confirm they pass**

Run:

```bash
node --test web/tests/invite-rebate.test.mjs
```

Expected:

- PASS with normalized page shape and grouped default/override rows ready for the new page components.

- [ ] **Step 5: Commit the helper layer**

```bash
git add web/src/helpers/inviteRebate.js web/src/helpers/index.js web/tests/invite-rebate.test.mjs
git commit -m "feat: add invite rebate frontend helpers"
```

### Task 5: Build The Invite Rebate Page, Wire The APIs, And Add Locale Coverage

**Files:**
- Create: `web/src/components/invite-rebate/InviteRebatePage.jsx`
- Create: `web/src/components/invite-rebate/InviteRebateSummary.jsx`
- Create: `web/src/components/invite-rebate/InviteDefaultRuleSection.jsx`
- Create: `web/src/components/invite-rebate/InviteeListPanel.jsx`
- Create: `web/src/components/invite-rebate/InviteeOverridePanel.jsx`
- Modify: `web/src/pages/InviteRebate/index.js`
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/fr.json`
- Modify: `web/src/i18n/locales/ja.json`
- Modify: `web/src/i18n/locales/ru.json`
- Modify: `web/src/i18n/locales/vi.json`
- Modify: `web/src/i18n/locales/zh-CN.json`
- Modify: `web/src/i18n/locales/zh-TW.json`
- Create: `web/tests/invite-rebate-page.test.mjs`
- Create: `web/tests/invite-rebate-locales.test.mjs`

- [ ] **Step 1: Write the failing page and locale regression tests**

```js
// web/tests/invite-rebate-page.test.mjs
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

test('invite rebate page fetches defaults, invitee page data, and invitee detail data', () => {
  const pageSource = readFileSync(
    new URL('../src/components/invite-rebate/InviteRebatePage.jsx', import.meta.url),
    'utf8',
  );

  assert.match(pageSource, /API\.get\('\/api\/user\/referral\/subscription'\)/);
  assert.match(pageSource, /API\.get\('\/api\/user\/referral\/subscription\/invitees'/);
  assert.match(pageSource, /API\.get\(`\/api\/user\/referral\/subscription\/invitees\/\$\{selectedInviteeId\}`\)/);
});

test('invite rebate page renders separate default-rule and invitee-override sections', () => {
  const pageSource = readFileSync(
    new URL('../src/components/invite-rebate/InviteRebatePage.jsx', import.meta.url),
    'utf8',
  );

  assert.match(pageSource, /InviteDefaultRuleSection/);
  assert.match(pageSource, /InviteeListPanel/);
  assert.match(pageSource, /InviteeOverridePanel/);
  assert.match(pageSource, /t\('被邀请人数'\)/);
  assert.match(pageSource, /t\('累计返佣收益'\)/);
});
```

```js
// web/tests/invite-rebate-locales.test.mjs
import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const locales = ['en', 'fr', 'ja', 'ru', 'vi', 'zh-CN', 'zh-TW'];
const requiredKeys = [
  '邀请管理',
  '邀请管理区域',
  '邀请返佣',
  '邀请返佣管理',
  '被邀请人数',
  '累计返佣收益',
  '默认返佣规则',
  '默认给被邀请人的返佣率',
  '被邀请人列表',
  '累计贡献',
  '独立返佣',
  '请选择一个被邀请人',
  '未设置独立返佣时，使用默认规则',
];

test('invite rebate locales define the new page copy keys', () => {
  locales.forEach((locale) => {
    const content = readFileSync(
      new URL(`../src/i18n/locales/${locale}.json`, import.meta.url),
      'utf8',
    );
    const translation = JSON.parse(content).translation;
    requiredKeys.forEach((key) => {
      assert.ok(
        Object.prototype.hasOwnProperty.call(translation, key),
        `missing ${key} in ${locale}`,
      );
    });
  });
});
```

- [ ] **Step 2: Run the page and locale tests and verify they fail**

Run:

```bash
node --test web/tests/invite-rebate-page.test.mjs web/tests/invite-rebate-locales.test.mjs
```

Expected:

- FAIL because the new page components and locale keys do not exist yet.

- [ ] **Step 3: Implement the invite rebate page and its grouped editing flows**

```jsx
// web/src/components/invite-rebate/InviteRebatePage.jsx
const InviteRebatePage = () => {
  const { t } = useTranslation();
  const [summary, setSummary] = useState({ inviteeCount: 0, totalContributionQuota: 0 });
  const [defaultRows, setDefaultRows] = useState([]);
  const [inviteePage, setInviteePage] = useState({ items: [], total: 0, page: 1, pageSize: 20 });
  const [selectedInviteeId, setSelectedInviteeId] = useState(0);
  const [selectedInviteeDetail, setSelectedInviteeDetail] = useState(null);

  const loadDefaults = async () => {
    const res = await API.get('/api/user/referral/subscription');
    if (res.data?.success) {
      setDefaultRows(buildInviteDefaultRuleRows(res.data.data?.groups || [], res.data.data?.available_groups || []));
    }
  };

  const loadInviteePage = async (page = 1, keyword = '') => {
    const res = await API.get('/api/user/referral/subscription/invitees', {
      params: { page, page_size: 20, keyword },
    });
    if (res.data?.success) {
      const nextPage = normalizeInviteeContributionPage(res.data.data || {});
      setInviteePage(nextPage);
      setSummary({
        inviteeCount: nextPage.inviteeCount,
        totalContributionQuota: nextPage.totalContributionQuota,
      });
    }
  };

  const loadInviteeDetail = async (inviteeId) => {
    const res = await API.get(`/api/user/referral/subscription/invitees/${inviteeId}`);
    if (res.data?.success) {
      setSelectedInviteeDetail(res.data.data || null);
    }
  };

  return (
    <div className='flex flex-col gap-4'>
      <InviteRebateSummary t={t} summary={summary} />
      <InviteDefaultRuleSection t={t} rows={defaultRows} onReload={loadDefaults} />
      <div className='grid grid-cols-1 gap-4 xl:grid-cols-[1.1fr,1fr]'>
        <InviteeListPanel
          t={t}
          page={inviteePage}
          selectedInviteeId={selectedInviteeId}
          onSelect={(inviteeId) => {
            setSelectedInviteeId(inviteeId);
            loadInviteeDetail(inviteeId);
          }}
          onSearch={loadInviteePage}
        />
        <InviteeOverridePanel
          t={t}
          selectedInviteeId={selectedInviteeId}
          detail={selectedInviteeDetail}
          onReload={() => selectedInviteeId && loadInviteeDetail(selectedInviteeId)}
        />
      </div>
    </div>
  );
};
```

```json
// web/src/i18n/locales/zh-CN.json
{
  "translation": {
    "邀请管理": "邀请管理",
    "邀请管理区域": "邀请管理区域",
    "邀请返佣": "邀请返佣",
    "邀请返佣管理": "邀请返佣管理",
    "被邀请人数": "被邀请人数",
    "累计返佣收益": "累计返佣收益",
    "默认返佣规则": "默认返佣规则",
    "默认给被邀请人的返佣率": "默认给被邀请人的返佣率",
    "被邀请人列表": "被邀请人列表",
    "累计贡献": "累计贡献",
    "独立返佣": "独立返佣",
    "请选择一个被邀请人": "请选择一个被邀请人",
    "未设置独立返佣时，使用默认规则": "未设置独立返佣时，使用默认规则"
  }
}
```

- [ ] **Step 4: Run the page tests and the broader referral regression suite**

Run:

```bash
node --test web/tests/invite-management-sidebar.test.mjs web/tests/invite-rebate.test.mjs web/tests/invite-rebate-page.test.mjs web/tests/invite-rebate-locales.test.mjs web/tests/subscription-referral.test.mjs web/tests/subscription-referral-locales.test.mjs web/tests/subscription-referral-override-section.test.mjs
go test ./controller -run 'Test(DeleteSubscriptionReferralSelfRemovesGroupedDefaultRate|ListSubscriptionReferralInviteesReturnsOnlyOwnedInvitees|UpsertSubscriptionReferralInviteeOverrideRejectsForeignInvitee|GenerateDefaultSidebarConfigIncludesInviteSection)' -count=1
go test ./model -run 'Test(UpsertSubscriptionReferralInviteeOverrideStoresGroupedRate|ListSubscriptionReferralInviteeContributionSummariesUsesNetInviterReward)' -count=1
```

Expected:

- PASS for the new sidebar/page/locale tests.
- PASS for the focused controller and model tests.
- PASS for the existing grouped referral frontend regression suite.

- [ ] **Step 5: Commit the page integration**

```bash
git add web/src/components/invite-rebate web/src/pages/InviteRebate/index.js web/src/i18n/locales/en.json web/src/i18n/locales/fr.json web/src/i18n/locales/ja.json web/src/i18n/locales/ru.json web/src/i18n/locales/vi.json web/src/i18n/locales/zh-CN.json web/src/i18n/locales/zh-TW.json web/tests/invite-rebate-page.test.mjs web/tests/invite-rebate-locales.test.mjs
git commit -m "feat: add invite rebate management page"
```

### Task 6: End-To-End Cleanup And Final Verification

**Files:**
- Modify: `web/src/pages/InviteRebate/index.js`
- Modify: `web/src/components/invite-rebate/InviteRebatePage.jsx`
- Modify: `controller/subscription_referral.go`
- Modify: `model/subscription_referral_invitee_override.go`

- [ ] **Step 1: Add any missing empty-state, loader, and error-copy assertions before the final polish pass**

```js
// web/tests/invite-rebate-page.test.mjs
test('invite rebate override panel shows the empty-state fallback before a user is selected', () => {
  const panelSource = readFileSync(
    new URL('../src/components/invite-rebate/InviteeOverridePanel.jsx', import.meta.url),
    'utf8',
  );

  assert.match(panelSource, /t\('请选择一个被邀请人'\)/);
  assert.match(panelSource, /t\('未设置独立返佣时，使用默认规则'\)/);
});
```

- [ ] **Step 2: Run the new empty-state test and confirm it fails if the panel still lacks the UX copy**

Run:

```bash
node --test web/tests/invite-rebate-page.test.mjs
```

Expected:

- FAIL if the final UX polish strings are missing from the override panel.

- [ ] **Step 3: Apply the final UX polish and ensure the page uses the new management page as the primary editing surface**

```jsx
// web/src/components/invite-rebate/InviteeOverridePanel.jsx
if (!selectedInviteeId) {
  return (
    <Card className='!rounded-2xl shadow-sm border-0'>
      <Empty
        title={t('请选择一个被邀请人')}
        description={t('未设置独立返佣时，使用默认规则')}
      />
    </Card>
  );
}
```

```jsx
// web/src/pages/InviteRebate/index.js
import InviteRebatePage from '../../components/invite-rebate/InviteRebatePage';

export default InviteRebatePage;
```

- [ ] **Step 4: Run the final verification suite from the worktree root**

Run:

```bash
node --test web/tests/invite-management-sidebar.test.mjs web/tests/invite-rebate.test.mjs web/tests/invite-rebate-page.test.mjs web/tests/invite-rebate-locales.test.mjs web/tests/subscription-referral.test.mjs web/tests/subscription-referral-locales.test.mjs web/tests/subscription-referral-override-section.test.mjs
go test ./controller -run 'Test(DeleteSubscriptionReferralSelfRemovesGroupedDefaultRate|ListSubscriptionReferralInviteesReturnsOnlyOwnedInvitees|UpsertSubscriptionReferralInviteeOverrideRejectsForeignInvitee|GenerateDefaultSidebarConfigIncludesInviteSection)' -count=1
go test ./model -run 'Test(UpsertSubscriptionReferralInviteeOverrideStoresGroupedRate|ListSubscriptionReferralInviteeContributionSummariesUsesNetInviterReward)' -count=1
```

Expected:

- PASS across all invite-management-specific frontend and backend tests, with no regression in the existing grouped referral frontend suite.

- [ ] **Step 5: Commit the final polish pass**

```bash
git add web/src/pages/InviteRebate/index.js web/src/components/invite-rebate/InviteRebatePage.jsx web/src/components/invite-rebate/InviteeOverridePanel.jsx controller/subscription_referral.go model/subscription_referral_invitee_override.go web/tests/invite-rebate-page.test.mjs
git commit -m "feat: finalize invite rebate management flow"
```
