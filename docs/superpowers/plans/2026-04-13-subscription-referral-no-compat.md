# Subscription Referral No-Compat Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the legacy mixed-mode subscription referral compatibility layer so grouped referral behavior is driven only by real `subscription_plans.upgrade_group` values, with one-time migration or startup failure for incompatible old data.

**Architecture:** Keep the grouped referral core already shipped, but delete the legacy empty-group and scalar-rate semantics around it. Add startup migration/validation for old override rows and old inviter settings, tighten grouped-only controller payloads, align frontend pages to grouped-only API shapes, and change the local/public Docker launchers to rebuild images by default so runtime code always matches the repo.

**Tech Stack:** Go 1.26 + Gin + GORM + PostgreSQL/SQLite migration helpers, existing OptionMap runtime config, React 18 + Semi UI + Vite, Node built-in test runner for frontend helpers, Python launcher scripts for local/public Docker startup

---

### Task 1: Startup Migration Guard For Legacy Override Rows And Legacy User Setting Rates

**Files:**
- Modify: `model/subscription_referral.go`
- Modify: `model/main.go`
- Modify: `dto/user_settings.go`
- Modify: `model/subscription_referral_test.go`

- [ ] **Step 1: Write the failing backend tests for legacy empty-group override migration and startup failure in multi-group mode**

```go
func TestMigrateLegacySubscriptionReferralOverridesMigratesSingleGroupSystem(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	plan := seedReferralPlan(t, db, 19.9)
	setReferralPlanUpgradeGroup(t, db, plan, "vip")

	user := seedReferralUser(t, db, "legacy-override-user", 0, dto.UserSetting{})
	if _, err := UpsertSubscriptionReferralOverride(user.Id, "", 3200, 1); err != nil {
		t.Fatalf("failed to seed legacy override: %v", err)
	}

	if err := migrateLegacySubscriptionReferralOverrides(); err != nil {
		t.Fatalf("migrateLegacySubscriptionReferralOverrides() error = %v", err)
	}

	override, err := GetSubscriptionReferralOverrideByUserIDAndGroup(user.Id, "vip")
	if err != nil {
		t.Fatalf("failed to load migrated vip override: %v", err)
	}
	if override.TotalRateBps != 3200 {
		t.Fatalf("migrated vip override rate = %d, want 3200", override.TotalRateBps)
	}
	if _, err := getLegacyUngroupedSubscriptionReferralOverrideByUserID(user.Id); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected legacy empty-group override to be gone, got %v", err)
	}
}

func TestMigrateLegacySubscriptionReferralOverridesFailsWhenMultipleGroupsExist(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	planA := seedReferralPlan(t, db, 19.9)
	setReferralPlanUpgradeGroup(t, db, planA, "vip")
	planB := seedReferralPlan(t, db, 29.9)
	setReferralPlanUpgradeGroup(t, db, planB, "pro")

	user := seedReferralUser(t, db, "legacy-override-multi", 0, dto.UserSetting{})
	if _, err := UpsertSubscriptionReferralOverride(user.Id, "", 3200, 1); err != nil {
		t.Fatalf("failed to seed legacy override: %v", err)
	}

	err := migrateLegacySubscriptionReferralOverrides()
	if err == nil {
		t.Fatal("expected multi-group migration to fail")
	}
	if !strings.Contains(err.Error(), "legacy subscription referral override") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMigrateLegacySubscriptionReferralInviteeRatesMigratesSingleGroupUserSetting(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	plan := seedReferralPlan(t, db, 19.9)
	setReferralPlanUpgradeGroup(t, db, plan, "vip")

	user := seedReferralUser(t, db, "legacy-invitee-rate-user", 0, dto.UserSetting{
		SubscriptionReferralInviteeRateBps: 700,
	})

	if err := migrateLegacySubscriptionReferralInviteeRates(); err != nil {
		t.Fatalf("migrateLegacySubscriptionReferralInviteeRates() error = %v", err)
	}

	after, err := GetUserById(user.Id, true)
	if err != nil {
		t.Fatalf("failed to reload user: %v", err)
	}
	setting := after.GetSetting()
	if setting.SubscriptionReferralInviteeRateBps != 0 {
		t.Fatalf("legacy scalar invitee rate = %d, want 0", setting.SubscriptionReferralInviteeRateBps)
	}
	if got := setting.SubscriptionReferralInviteeRateBpsByGroup["vip"]; got != 700 {
		t.Fatalf("grouped invitee rate = %d, want 700", got)
	}
}
```

- [ ] **Step 2: Run the focused migration tests and verify they fail before implementation**

Run:

```bash
go test ./model -run 'Test(MigrateLegacySubscriptionReferralOverridesMigratesSingleGroupSystem|MigrateLegacySubscriptionReferralOverridesFailsWhenMultipleGroupsExist|MigrateLegacySubscriptionReferralInviteeRatesMigratesSingleGroupUserSetting)' -count=1
```

Expected:

- FAIL because the startup migration helpers do not exist yet.

- [ ] **Step 3: Implement startup migration helpers and wire them into startup before normal runtime use**

```go
// model/subscription_referral.go
func getSingleSubscriptionReferralGroupForMigration() (string, error) {
	groups := ListSubscriptionReferralConfiguredGroups()
	if len(groups) != 1 {
		return "", fmt.Errorf("legacy subscription referral migration requires exactly one real group, got %d", len(groups))
	}
	if groups[0] == "default" {
		return "", errors.New("legacy subscription referral migration cannot target synthetic default")
	}
	return groups[0], nil
}

func migrateLegacySubscriptionReferralOverrides() error {
	var legacyOverrides []SubscriptionReferralOverride
	if err := DB.Where(commonGroupCol+" = ?", "").Find(&legacyOverrides).Error; err != nil {
		return err
	}
	if len(legacyOverrides) == 0 {
		return nil
	}
	targetGroup, err := getSingleSubscriptionReferralGroupForMigration()
	if err != nil {
		return err
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		for _, override := range legacyOverrides {
			if err := tx.Where("id = ?", override.Id).Delete(&SubscriptionReferralOverride{}).Error; err != nil {
				return err
			}
			if _, err := upsertSubscriptionReferralOverrideTx(tx, override.UserId, targetGroup, override.TotalRateBps, override.CreatedBy, override.UpdatedBy); err != nil {
				return err
			}
		}
		return nil
	})
}

func migrateLegacySubscriptionReferralInviteeRates() error {
	var users []User
	if err := DB.Where("setting LIKE ?", `%subscription_referral_invitee_rate_bps%`).Find(&users).Error; err != nil {
		return err
	}
	if len(users) == 0 {
		return nil
	}
	targetGroup, err := getSingleSubscriptionReferralGroupForMigration()
	if err != nil {
		return err
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		for _, user := range users {
			setting := user.GetSetting()
			if setting.SubscriptionReferralInviteeRateBps <= 0 {
				continue
			}
			nextByGroup := copySubscriptionReferralInviteeRatesByGroup(setting.SubscriptionReferralInviteeRateBpsByGroup)
			nextByGroup[targetGroup] = setting.SubscriptionReferralInviteeRateBps
			setting.SubscriptionReferralInviteeRateBpsByGroup = nextByGroup
			setting.SubscriptionReferralInviteeRateBps = 0
			user.SetSetting(setting)
			if err := tx.Model(&User{}).Where("id = ?", user.Id).Update("setting", user.Setting).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
```

```go
// model/main.go
func migrateDB() error {
	migrateSubscriptionPlanPriceAmount()
	if err := migrateTokenModelLimitsToText(); err != nil {
		return err
	}
	if err := prepareSubscriptionReferralOverrideSchemaBeforeAutoMigrate(); err != nil {
		return err
	}
	if err := prepareSubscriptionReferralRecordSchemaBeforeAutoMigrate(); err != nil {
		return err
	}
	if err := DB.AutoMigrate(...); err != nil {
		return err
	}
	if err := ensureSubscriptionReferralOverrideSchema(); err != nil {
		return err
	}
	if err := ensureSubscriptionReferralRecordSchema(); err != nil {
		return err
	}
	if err := migrateLegacySubscriptionReferralOverrides(); err != nil {
		return err
	}
	if err := migrateLegacySubscriptionReferralInviteeRates(); err != nil {
		return err
	}
	return nil
}
```

- [ ] **Step 4: Re-run the focused migration tests and confirm they pass**

Run:

```bash
go test ./model -run 'Test(MigrateLegacySubscriptionReferralOverridesMigratesSingleGroupSystem|MigrateLegacySubscriptionReferralOverridesFailsWhenMultipleGroupsExist|MigrateLegacySubscriptionReferralInviteeRatesMigratesSingleGroupUserSetting)' -count=1
```

Expected:

- PASS with single-group migration succeeding and multi-group startup protection failing loudly.

- [ ] **Step 5: Commit the startup migration guard layer**

```bash
git add model/subscription_referral.go model/main.go dto/user_settings.go model/subscription_referral_test.go
git commit -m "feat: add no-compat subscription referral startup migration guards"
```

### Task 2: Remove Legacy Runtime Semantics From Models And Controller Responses

**Files:**
- Modify: `model/subscription_referral.go`
- Modify: `controller/subscription_referral.go`
- Modify: `controller/subscription_referral_test.go`
- Modify: `model/subscription_referral_test.go`

- [ ] **Step 1: Write the failing tests that prove old legacy/default response semantics are gone**

```go
func TestGetSubscriptionReferralSelfReturnsGroupedOnlyShape(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	common.SubscriptionReferralEnabled = true
	if err := common.UpdateSubscriptionReferralGroupRatesByJSONString(`{"vip":4500}`); err != nil {
		t.Fatalf("failed to seed group rates: %v", err)
	}

	user := seedSubscriptionReferralControllerUser(t, "grouped-self-shape", 0, dto.UserSetting{
		SubscriptionReferralInviteeRateBpsByGroup: map[string]int{"vip": 500},
	})
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/referral/subscription", nil, user.Id)
	GetSubscriptionReferralSelf(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatal("expected success")
	}
	var payload map[string]any
	if err := common.Unmarshal(resp.Data, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if _, ok := payload["total_rate_bps"]; ok {
		t.Fatal("did not expect legacy top-level total_rate_bps")
	}
	if _, ok := payload["invitee_rate_bps"]; ok {
		t.Fatal("did not expect legacy top-level invitee_rate_bps")
	}
	if _, ok := payload["groups"]; !ok {
		t.Fatal("expected grouped payload")
	}
}

func TestUpdateSubscriptionReferralSelfRejectsMissingGroup(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionReferralControllerUser(t, "grouped-self-write", 0, dto.UserSetting{})
	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodPut,
		"/api/user/referral/subscription",
		UpdateSubscriptionReferralSelfRequest{InviteeRateBps: 500},
		user.Id,
	)
	UpdateSubscriptionReferralSelf(ctx)

	resp := decodeAPIResponse(t, recorder)
	if resp.Success {
		t.Fatal("expected missing group request to fail")
	}
}

func TestAdminGetSubscriptionReferralOverrideReturnsGroupedOnlyShape(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionReferralControllerUser(t, "grouped-admin-shape", 0, dto.UserSetting{})
	if _, err := model.UpsertSubscriptionReferralOverride(user.Id, "vip", 3500, 1); err != nil {
		t.Fatalf("failed to seed grouped override: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/subscription/admin/referral/users/1", nil, 1)
	ctx.Set("role", common.RoleRootUser)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(user.Id)}}
	AdminGetSubscriptionReferralOverride(ctx)

	resp := decodeAPIResponse(t, recorder)
	if !resp.Success {
		t.Fatal("expected success")
	}
	var payload map[string]any
	if err := common.Unmarshal(resp.Data, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if _, ok := payload["effective_total_rate_bps"]; ok {
		t.Fatal("did not expect legacy top-level effective_total_rate_bps")
	}
	if _, ok := payload["has_override"]; ok {
		t.Fatal("did not expect legacy top-level has_override")
	}
}
```

- [ ] **Step 2: Run the focused controller tests and verify they fail before removing legacy fields**

Run:

```bash
go test ./controller -run 'Test(GetSubscriptionReferralSelfReturnsGroupedOnlyShape|UpdateSubscriptionReferralSelfRejectsMissingGroup|AdminGetSubscriptionReferralOverrideReturnsGroupedOnlyShape)' -count=1
```

Expected:

- FAIL because the old top-level fields and no-group write compatibility still exist.

- [ ] **Step 3: Delete legacy runtime semantics and make grouped payloads the only API shape**

```go
// controller/subscription_referral.go
type UpdateSubscriptionReferralSelfRequest struct {
	Group          string `json:"group"`
	InviteeRateBps int    `json:"invitee_rate_bps"`
}

func GetSubscriptionReferralSelf(c *gin.Context) {
	userID := c.GetInt("id")
	user, err := model.GetUserById(userID, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"enabled":              common.SubscriptionReferralEnabled,
		"groups":               buildSubscriptionReferralSelfGroupViews(user),
		"pending_reward_quota": user.AffQuota,
		"history_reward_quota": user.AffHistoryQuota,
		"inviter_count":        user.AffCount,
	})
}

func UpdateSubscriptionReferralSelf(c *gin.Context) {
	var req UpdateSubscriptionReferralSelfRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	req.Group = strings.TrimSpace(req.Group)
	if req.Group == "" {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	// grouped-only validation + grouped-only write
}

func AdminGetSubscriptionReferralOverride(c *gin.Context) {
	// return only { user_id, groups }
}

func AdminUpsertSubscriptionReferralOverride(c *gin.Context) {
	// req.Group must be non-empty
}
```

Also remove from model/controller logic:

- legacy empty-group fallback readers in normal runtime paths
- legacy top-level response fields for self/admin grouped endpoints
- empty-group upsert path in grouped APIs

Keep only what is necessary for startup migration checks, not runtime compatibility.

- [ ] **Step 4: Re-run the focused controller tests and confirm they pass**

Run:

```bash
go test ./controller -run 'Test(GetSubscriptionReferralSelfReturnsGroupedOnlyShape|UpdateSubscriptionReferralSelfRejectsMissingGroup|AdminGetSubscriptionReferralOverrideReturnsGroupedOnlyShape)' -count=1
```

Expected:

- PASS with grouped-only response/write semantics.

- [ ] **Step 5: Commit the grouped-only API cleanup**

```bash
git add model/subscription_referral.go controller/subscription_referral.go controller/subscription_referral_test.go model/subscription_referral_test.go
git commit -m "refactor: remove legacy subscription referral runtime semantics"
```

### Task 3: Remove Legacy Frontend Compatibility Paths And Default Mapping

**Files:**
- Modify: `web/src/helpers/subscriptionReferral.js`
- Modify: `web/src/pages/Setting/Operation/SettingsSubscriptionReferral.jsx`
- Modify: `web/src/components/table/users/modals/SubscriptionReferralOverrideSection.jsx`
- Modify: `web/src/components/topup/index.jsx`
- Modify: `web/src/components/topup/InvitationCard.jsx`
- Modify: `web/tests/subscription-referral.test.mjs`

- [ ] **Step 1: Write the failing frontend tests that prove legacy `default`/top-level fallback behavior is gone**

```js
test('parseAdminReferralSettings does not synthesize default from legacy total_rate_bps', () => {
  assert.deepEqual(
    parseAdminReferralSettings({
      enabled: true,
      total_rate_bps: 4500,
    }),
    {
      enabled: true,
      groups: [],
      groupRates: {},
    },
  );
});

test('buildAdminReferralRows does not invent default rows when group list is empty', () => {
  assert.deepEqual(buildAdminReferralRows([], {}), []);
});
```

- [ ] **Step 2: Run the frontend tests and verify they fail before deleting legacy helpers**

Run:

```bash
cd web && node --test tests/subscription-referral.test.mjs
```

Expected:

- FAIL because helpers still synthesize `default` or legacy fallback rows.

- [ ] **Step 3: Remove legacy frontend fallback behavior and align all UI to grouped-only payloads**

```js
// web/src/helpers/subscriptionReferral.js
export function parseAdminReferralSettings(payload = {}) {
  return {
    enabled: normalizeEnabled(payload.enabled),
    groups: normalizeGroupNames(payload.groups),
    groupRates: normalizeGroupRateMap(payload.group_rates),
  };
}

export function buildAdminReferralRows(groupNames = [], groupRates = {}) {
  const normalizedGroupNames = normalizeGroupNames(groupNames);
  const normalizedGroupRates = normalizeGroupRateMap(groupRates);
  return normalizedGroupNames.map((group) => {
    const totalRateBps = normalizeRateBps(normalizedGroupRates[group] || 0);
    return {
      group,
      enabled: totalRateBps > 0,
      totalRateBps,
      totalRatePercent: rateBpsToPercentNumber(totalRateBps),
    };
  });
}
```

UI changes:

- admin settings page only uses `groups` + `group_rates`
- admin override modal only renders grouped `groups[]`
- inviter self page only renders grouped `groups[]`
- no virtual `default` card, no legacy top-level total/invitee fallback rendering
- self save requires an explicit selected group from each card

- [ ] **Step 4: Re-run the frontend tests and confirm they pass**

Run:

```bash
cd web && node --test tests/subscription-referral.test.mjs
```

Expected:

- PASS with grouped-only frontend behavior.

- [ ] **Step 5: Commit the grouped-only frontend cleanup**

```bash
git add web/src/helpers/subscriptionReferral.js web/src/pages/Setting/Operation/SettingsSubscriptionReferral.jsx web/src/components/table/users/modals/SubscriptionReferralOverrideSection.jsx web/src/components/topup/index.jsx web/src/components/topup/InvitationCard.jsx web/tests/subscription-referral.test.mjs
git commit -m "refactor: remove legacy subscription referral frontend fallbacks"
```

### Task 4: Enforce Non-Empty Group Data Model And Fail Fast On Unmigrated Data

**Files:**
- Modify: `model/subscription_referral.go`
- Modify: `model/main.go`
- Modify: `controller/subscription_referral.go`
- Modify: `model/subscription_referral_test.go`

- [ ] **Step 1: Write the failing tests for non-empty override enforcement and startup failure on leftover unmigrated data**

```go
func TestUpsertSubscriptionReferralOverrideRejectsEmptyGroup(t *testing.T) {
	setupSubscriptionReferralSettlementDB(t)
	user := seedReferralUser(t, DB, "reject-empty-group-user", 0, dto.UserSetting{})
	_, err := UpsertSubscriptionReferralOverride(user.Id, "", 3000, 1)
	if err == nil {
		t.Fatal("expected empty group override write to fail")
	}
}

func TestValidateNoLegacySubscriptionReferralDataFailsOnEmptyGroupOverride(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	user := seedReferralUser(t, db, "legacy-empty-group-leftover", 0, dto.UserSetting{})
	if _, err := db.Exec("insert into subscription_referral_overrides (user_id, \"group\", total_rate_bps, created_by, updated_by, created_at, updated_at) values (?, ?, ?, ?, ?, ?, ?)", user.Id, "", 3200, 1, 1, common.GetTimestamp(), common.GetTimestamp()); err != nil {
		t.Fatalf("failed to seed legacy empty-group row: %v", err)
	}

	err := validateNoLegacySubscriptionReferralData()
	if err == nil {
		t.Fatal("expected validation to fail")
	}
}
```

- [ ] **Step 2: Run the focused tests and verify they fail before enforcement is added**

Run:

```bash
go test ./model -run 'Test(UpsertSubscriptionReferralOverrideRejectsEmptyGroup|ValidateNoLegacySubscriptionReferralDataFailsOnEmptyGroupOverride)' -count=1
```

Expected:

- FAIL because empty-group writes are still allowed in some paths and startup validation does not exist.

- [ ] **Step 3: Enforce non-empty grouped model semantics everywhere after migration**

```go
// model/subscription_referral.go
func UpsertSubscriptionReferralOverride(userID int, group string, totalRateBps int, operatorID int) (*SubscriptionReferralOverride, error) {
	group = strings.TrimSpace(group)
	if group == "" {
		return nil, errors.New("group is required")
	}
	// grouped-only write path
}

func validateNoLegacySubscriptionReferralData() error {
	var overrideCount int64
	if err := DB.Model(&SubscriptionReferralOverride{}).Where(commonGroupCol+" = ?", "").Count(&overrideCount).Error; err != nil {
		return err
	}
	if overrideCount > 0 {
		return fmt.Errorf("legacy subscription referral overrides remain after migration")
	}
	var users []User
	if err := DB.Where("setting LIKE ?", `%subscription_referral_invitee_rate_bps%`).Find(&users).Error; err != nil {
		return err
	}
	for _, user := range users {
		if user.GetSetting().SubscriptionReferralInviteeRateBps > 0 {
			return fmt.Errorf("legacy subscription referral invitee rate remains for user %d", user.Id)
		}
	}
	return nil
}
```

Wire `validateNoLegacySubscriptionReferralData()` into startup after the migration helpers. If data remains and could not be migrated, startup must fail.

- [ ] **Step 4: Re-run the focused enforcement tests and confirm they pass**

Run:

```bash
go test ./model -run 'Test(UpsertSubscriptionReferralOverrideRejectsEmptyGroup|ValidateNoLegacySubscriptionReferralDataFailsOnEmptyGroupOverride)' -count=1
```

Expected:

- PASS with fail-fast protection in place.

- [ ] **Step 5: Commit the grouped-only data model enforcement**

```bash
git add model/subscription_referral.go model/main.go controller/subscription_referral.go model/subscription_referral_test.go
git commit -m "refactor: enforce grouped-only subscription referral data"
```

### Task 5: Make Local/Public Docker Launchers Rebuild By Default

**Files:**
- Modify: `scripts/local.py`
- Modify: `scripts/public.py`
- Modify: `scripts/tests/test_local.py`
- Modify: `scripts/tests/test_public.py`
- Modify: `docs/payment-local-public-launcher.zh-CN.md`

- [ ] **Step 1: Write the failing launcher tests showing local/public startup must use `--build` by default**

```python
def test_run_local_stack_uses_build_flag_by_default(...):
    run_local_stack(config, output=stdout, repo_root=repo_root)
    run_command.assert_any_call(
        ["docker", "compose", "-f", str(compose_file_path), "up", "-d", "--build"],
        check=True,
        stream_output=True,
        cwd=repo_root,
        stdout_stream=stdout,
    )


def test_run_public_stack_still_invokes_local_stack_with_building_behavior(...):
    # assert the delegated local startup path now uses compose up -d --build
```

- [ ] **Step 2: Run the launcher tests and verify they fail before implementation**

Run:

```bash
python3 -m unittest scripts.tests.test_local scripts.tests.test_public
```

Expected:

- FAIL because local/public startup still uses `docker compose up -d` without `--build`.

- [ ] **Step 3: Change launcher behavior and docs to rebuild by default**

```python
# scripts/local.py
run_command(
    ["docker", "compose", "-f", str(compose_file_path), "up", "-d", "--build"],
    check=True,
    stream_output=True,
    cwd=effective_repo_root,
    stdout_stream=stream,
)
```

Update docs to clearly say local/public startup rebuilds by default.

- [ ] **Step 4: Re-run the launcher tests and confirm they pass**

Run:

```bash
python3 -m unittest scripts.tests.test_local scripts.tests.test_public
```

Expected:

- PASS with `--build` enforced.

- [ ] **Step 5: Commit the launcher hardening**

```bash
git add scripts/local.py scripts/public.py scripts/tests/test_local.py scripts/tests/test_public.py docs/payment-local-public-launcher.zh-CN.md
git commit -m "fix: rebuild docker launchers by default"
```

### Task 6: Final Verification For No-Compat Migration Path

**Files:**
- Modify: `docs/superpowers/plans/2026-04-13-subscription-referral-no-compat.md`

- [ ] **Step 1: Run the grouped referral backend verification suite**

Run:

```bash
go test ./model ./controller -run 'SubscriptionReferral|Referral' -count=1
```

Expected:

- PASS with grouped-only APIs and startup migration guards.

- [ ] **Step 2: Run the frontend verification suite**

Run:

```bash
cd web && node --test tests/subscription-referral.test.mjs && npm run build
```

Expected:

- PASS for helper tests and production build.

- [ ] **Step 3: Run launcher verification**

Run:

```bash
python3 -m unittest scripts.tests.test_local scripts.tests.test_public
```

Expected:

- PASS with local/public launchers rebuilding by default.

- [ ] **Step 4: Inspect the diff for no-compat scope only**

Run:

```bash
git diff --stat HEAD~4..HEAD
```

Expected:

- Only no-compat migration, grouped-only referral cleanup, and launcher hardening changes appear.

- [ ] **Step 5: Mark this plan’s verification notes complete if you want them tracked in git**

```md
- [x] Backend grouped-only migration suite green
- [x] Frontend grouped-only suite green
- [x] Launcher rebuild-default tests green
- [x] Diff inspected for no-compat scope
```
