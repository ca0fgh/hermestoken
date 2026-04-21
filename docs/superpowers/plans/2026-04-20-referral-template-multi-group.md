# Referral Template Multi-Group Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add multi-group referral template management so admins can edit one template group across multiple system groups while the runtime keeps resolving bindings and settlements by a single `referral_type + group`.

**Architecture:** Keep `referral_templates` as one-row-per-group at runtime and add a lightweight `bundle_key` only for admin management. The admin API gains a bundle view and bundle write path, while the existing row-level view remains for user binding and runtime-facing flows. The settings UI switches to bundle mode with a multi-select group field, but the binding modal continues to choose concrete row templates.

**Tech Stack:** Go, GORM, Gin, SQLite-backed unit tests, React 18, Vite, Semi UI, i18next JSON locale files, source-reading Node tests.

---

## File Structure / Responsibility Map

- `model/referral_template.go`
  - Add `bundle_key`, relax name uniqueness to `referral_type + group + name`, and implement row/bundle read-write helpers.
- `model/referral_template_test.go`
  - Add TDD coverage for bundle grouping, multi-group create/update/delete behavior, and the new uniqueness rule.
- `model/main.go`
  - Ensure the updated `ReferralTemplate` schema is migrated in normal startup.
- `dto/referral_admin.go`
  - Extend the admin upsert request to accept `groups[]` while keeping `group` compatibility.
- `controller/referral_admin.go`
  - Add `view=bundle` handling, bundle-aware create/update/delete logic, and keep row view as the default.
- `controller/referral_admin_test.go`
  - Cover multi-group creation, bundle listing, bundle update/delete, and rollback-on-conflict behavior.
- `web/src/helpers/referralTemplate.js`
  - Normalize bundle-view items for the settings page.
- `web/src/helpers/referralLabels.js`
  - Make row-level labels safe when multiple templates share the same name across different groups.
- `web/src/pages/Setting/Referral/SettingsReferralTemplates.jsx`
  - Load `view=bundle`, render the group field as multi-select, send `groups[]`, and show bundle-level copy.
- `web/src/components/table/users/modals/ReferralTemplateBindingSection.jsx`
  - Continue loading row templates and render disambiguated labels for same-name templates in different groups.
- `web/tests/referral-settings-route.test.mjs`
  - Assert the settings surface now uses bundle view and multi-group copy.
- `web/tests/user-referral-binding.test.mjs`
  - Assert the binding modal still avoids editing group directly and uses row labels that include group context.
- `web/tests/referral-template-labels.test.mjs`
  - Cover the helper behavior for named templates in different groups.
- `web/src/i18n/locales/en.json`
- `web/src/i18n/locales/fr.json`
- `web/src/i18n/locales/ja.json`
- `web/src/i18n/locales/ru.json`
- `web/src/i18n/locales/vi.json`
- `web/src/i18n/locales/zh-CN.json`
- `web/src/i18n/locales/zh-TW.json`
  - Update the settings and binding copy to match multi-group behavior.

### Task 1: Add Bundle-Aware Referral Template Model Helpers

**Files:**
- Modify: `model/referral_template.go`
- Modify: `model/referral_template_test.go`
- Modify: `model/main.go`
- Test: `model/referral_template_test.go`

- [ ] **Step 1: Write the failing model tests**

```go
func TestReferralTemplateAllowsDuplicateNameAcrossDifferentGroups(t *testing.T) {
	db := setupReferralTemplateDB(t)

	firstTemplate := &ReferralTemplate{
		Name:         "shared-template-name",
		ReferralType: ReferralTypeSubscription,
		Group:        "vip",
		LevelType:    ReferralLevelTypeDirect,
		DirectCapBps: 1000,
	}
	if err := db.Create(firstTemplate).Error; err != nil {
		t.Fatalf("failed to create first template: %v", err)
	}

	secondTemplate := &ReferralTemplate{
		Name:         "shared-template-name",
		ReferralType: ReferralTypeSubscription,
		Group:        "default",
		LevelType:    ReferralLevelTypeTeam,
		TeamCapBps:   2500,
	}
	if err := db.Create(secondTemplate).Error; err != nil {
		t.Fatalf("Create(secondTemplate) error = %v, want success across groups", err)
	}
}

func TestCreateReferralTemplateBundleCreatesOneRowPerGroup(t *testing.T) {
	setupReferralTemplateDB(t)

	rows, err := CreateReferralTemplateBundle(ReferralTemplateBundleUpsertInput{
		ReferralType:           ReferralTypeSubscription,
		Groups:                 []string{"vip", "default"},
		Name:                   "starter-direct",
		LevelType:              ReferralLevelTypeDirect,
		Enabled:                true,
		DirectCapBps:           1200,
		InviteeShareDefaultBps: 300,
	}, 1)
	if err != nil {
		t.Fatalf("CreateReferralTemplateBundle() error = %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("bundle row count = %d, want 2", len(rows))
	}
	if rows[0].BundleKey == "" || rows[1].BundleKey == "" || rows[0].BundleKey != rows[1].BundleKey {
		t.Fatalf("expected a shared bundle key, got %#v", rows)
	}
}

func TestListReferralTemplateBundlesAggregatesRowsByBundleKey(t *testing.T) {
	setupReferralTemplateDB(t)

	created, err := CreateReferralTemplateBundle(ReferralTemplateBundleUpsertInput{
		ReferralType: ReferralTypeSubscription,
		Groups:       []string{"default", "vip"},
		Name:         "bundle-team",
		LevelType:    ReferralLevelTypeTeam,
		Enabled:      true,
		TeamCapBps:   2600,
	}, 1)
	if err != nil {
		t.Fatalf("CreateReferralTemplateBundle() error = %v", err)
	}

	bundles, err := ListReferralTemplateBundles(ReferralTypeSubscription)
	if err != nil {
		t.Fatalf("ListReferralTemplateBundles() error = %v", err)
	}
	if len(bundles) != 1 {
		t.Fatalf("bundle count = %d, want 1", len(bundles))
	}
	if bundles[0].BundleKey != created[0].BundleKey {
		t.Fatalf("BundleKey = %q, want %q", bundles[0].BundleKey, created[0].BundleKey)
	}
	if strings.Join(bundles[0].Groups, ",") != "default,vip" {
		t.Fatalf("Groups = %#v, want [default vip]", bundles[0].Groups)
	}
}

func TestBackfillReferralTemplateBundleKeysPopulatesLegacyRows(t *testing.T) {
	db := setupReferralTemplateDB(t)

	legacy := &ReferralTemplate{
		ReferralType: ReferralTypeSubscription,
		Group:        "legacy",
		Name:         "legacy-row",
		LevelType:    ReferralLevelTypeDirect,
		DirectCapBps: 1000,
	}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatalf("create legacy template: %v", err)
	}
	if err := db.Model(&ReferralTemplate{}).Where("id = ?", legacy.Id).Update("bundle_key", "").Error; err != nil {
		t.Fatalf("clear bundle key: %v", err)
	}

	if err := BackfillReferralTemplateBundleKeys(); err != nil {
		t.Fatalf("BackfillReferralTemplateBundleKeys() error = %v", err)
	}

	reloaded, err := GetReferralTemplateByID(legacy.Id)
	if err != nil {
		t.Fatalf("reload legacy template: %v", err)
	}
	if strings.TrimSpace(reloaded.BundleKey) == "" {
		t.Fatal("expected bundle key to be backfilled")
	}
}
```

- [ ] **Step 2: Run the model tests to verify they fail**

Run: `go test ./model -run 'TestReferralTemplateAllowsDuplicateNameAcrossDifferentGroups|TestCreateReferralTemplateBundleCreatesOneRowPerGroup|TestListReferralTemplateBundlesAggregatesRowsByBundleKey|TestBackfillReferralTemplateBundleKeysPopulatesLegacyRows'`

Expected: FAIL because the current model still enforces global template-name uniqueness and has no bundle helpers.

- [ ] **Step 3: Implement bundle-aware model behavior**

```go
// model/referral_template.go
type ReferralTemplate struct {
	Id                     int    `json:"id"`
	BundleKey              string `json:"bundle_key" gorm:"type:varchar(64);not null;default:'';index"`
	ReferralType           string `json:"referral_type" gorm:"type:varchar(64);not null;index:idx_referral_template_scope_name,priority:1"`
	Group                  string `json:"group" gorm:"type:varchar(64);not null;default:'';uniqueIndex:uk_referral_template_scope_name,priority:2;index:idx_referral_template_scope_name,priority:2"`
	Name                   string `json:"name" gorm:"type:varchar(128);not null;uniqueIndex:uk_referral_template_scope_name,priority:3;index:idx_referral_template_scope_name,priority:3"`
	LevelType              string `json:"level_type" gorm:"type:varchar(32);not null;index"`
	Enabled                bool   `json:"enabled" gorm:"not null;default:false"`
	DirectCapBps           int    `json:"direct_cap_bps" gorm:"type:int;not null;default:0"`
	TeamCapBps             int    `json:"team_cap_bps" gorm:"type:int;not null;default:0"`
	InviteeShareDefaultBps int    `json:"invitee_share_default_bps" gorm:"type:int;not null;default:0"`
	CreatedBy              int    `json:"created_by" gorm:"type:int;not null;default:0"`
	UpdatedBy              int    `json:"updated_by" gorm:"type:int;not null;default:0"`
	CreatedAt              int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt              int64  `json:"updated_at" gorm:"bigint"`
}

type ReferralTemplateBundleUpsertInput struct {
	ReferralType           string
	Groups                 []string
	Name                   string
	LevelType              string
	Enabled                bool
	DirectCapBps           int
	TeamCapBps             int
	InviteeShareDefaultBps int
}

type ReferralTemplateBundle struct {
	BundleKey              string   `json:"bundle_key"`
	TemplateIDs            []int    `json:"template_ids"`
	ReferralType           string   `json:"referral_type"`
	Groups                 []string `json:"groups"`
	Name                   string   `json:"name"`
	LevelType              string   `json:"level_type"`
	Enabled                bool     `json:"enabled"`
	DirectCapBps           int      `json:"direct_cap_bps"`
	TeamCapBps             int      `json:"team_cap_bps"`
	InviteeShareDefaultBps int      `json:"invitee_share_default_bps"`
	CreatedAt              int64    `json:"created_at"`
	UpdatedAt              int64    `json:"updated_at"`
}

func newReferralTemplateBundleKey() string {
	return strings.ReplaceAll(common.GetUUID(), "-", "")
}

func normalizeReferralTemplateGroups(groups []string) []string {
	seen := make(map[string]struct{}, len(groups))
	normalized := make([]string, 0, len(groups))
	for _, group := range groups {
		trimmed := strings.TrimSpace(group)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	sort.Strings(normalized)
	return normalized
}

func (t *ReferralTemplate) validateUniqueName(tx *gorm.DB) error {
	if tx == nil {
		tx = DB
	}
	var existing ReferralTemplate
	err := tx.Where(
		"referral_type = ? AND "+commonGroupCol+" = ? AND name = ? AND id <> ?",
		t.ReferralType,
		t.Group,
		t.Name,
		t.Id,
	).First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("template name already exists")
}

func normalizeReferralTemplatePersistenceError(err error) error {
	if err == nil {
		return nil
	}
	lowerError := strings.ToLower(err.Error())
	if strings.Contains(lowerError, "uk_referral_template_scope_name") ||
		strings.Contains(lowerError, "referral_templates.referral_type") {
		return fmt.Errorf("template name already exists")
	}
	return err
}
```

```go
// model/referral_template.go
func CreateReferralTemplateBundle(input ReferralTemplateBundleUpsertInput, operatorID int) ([]ReferralTemplate, error) {
	groups := normalizeReferralTemplateGroups(input.Groups)
	if len(groups) == 0 {
		return nil, fmt.Errorf("at least one group is required")
	}

	bundleKey := newReferralTemplateBundleKey()
	rows := make([]ReferralTemplate, 0, len(groups))
	err := DB.Transaction(func(tx *gorm.DB) error {
		for _, group := range groups {
			row := ReferralTemplate{
				BundleKey:              bundleKey,
				ReferralType:           strings.TrimSpace(input.ReferralType),
				Group:                  group,
				Name:                   strings.TrimSpace(input.Name),
				LevelType:              strings.TrimSpace(input.LevelType),
				Enabled:                input.Enabled,
				DirectCapBps:           input.DirectCapBps,
				TeamCapBps:             input.TeamCapBps,
				InviteeShareDefaultBps: input.InviteeShareDefaultBps,
				CreatedBy:              operatorID,
				UpdatedBy:              operatorID,
			}
			if err := normalizeReferralTemplatePersistenceError(tx.Create(&row).Error); err != nil {
				return err
			}
			rows = append(rows, row)
		}
		return nil
	})
	return rows, err
}

func ListReferralTemplateRows(referralType string, group string) ([]ReferralTemplate, error) {
	query := DB.Model(&ReferralTemplate{}).Order("referral_type ASC, " + commonGroupCol + " ASC, name ASC")
	if trimmedReferralType := strings.TrimSpace(referralType); trimmedReferralType != "" {
		query = query.Where("referral_type = ?", trimmedReferralType)
	}
	if trimmedGroup := strings.TrimSpace(group); trimmedGroup != "" {
		query = query.Where(commonGroupCol+" = ?", trimmedGroup)
	}
	var templates []ReferralTemplate
	if err := query.Find(&templates).Error; err != nil {
		return nil, err
	}
	return templates, nil
}

func ListReferralTemplateBundles(referralType string) ([]ReferralTemplateBundle, error) {
	rows, err := ListReferralTemplateRows(referralType, "")
	if err != nil {
		return nil, err
	}
	byBundle := make(map[string]*ReferralTemplateBundle, len(rows))
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		bundleKey := strings.TrimSpace(row.BundleKey)
		if bundleKey == "" {
			bundleKey = fmt.Sprintf("template:%d", row.Id)
		}
		bundle, exists := byBundle[bundleKey]
		if !exists {
			bundle = &ReferralTemplateBundle{
				BundleKey:              bundleKey,
				ReferralType:           row.ReferralType,
				Name:                   row.Name,
				LevelType:              row.LevelType,
				Enabled:                row.Enabled,
				DirectCapBps:           row.DirectCapBps,
				TeamCapBps:             row.TeamCapBps,
				InviteeShareDefaultBps: row.InviteeShareDefaultBps,
				CreatedAt:              row.CreatedAt,
				UpdatedAt:              row.UpdatedAt,
			}
			byBundle[bundleKey] = bundle
			order = append(order, bundleKey)
		}
		bundle.TemplateIDs = append(bundle.TemplateIDs, row.Id)
		bundle.Groups = append(bundle.Groups, row.Group)
		if row.UpdatedAt > bundle.UpdatedAt {
			bundle.UpdatedAt = row.UpdatedAt
		}
	}
	bundles := make([]ReferralTemplateBundle, 0, len(order))
	for _, bundleKey := range order {
		bundle := byBundle[bundleKey]
		sort.Strings(bundle.Groups)
		bundles = append(bundles, *bundle)
	}
	return bundles, nil
}

func BackfillReferralTemplateBundleKeys() error {
	var rows []ReferralTemplate
	if err := DB.Where("bundle_key = '' OR bundle_key IS NULL").Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		bundleKey := fmt.Sprintf("template:%d", row.Id)
		if err := DB.Model(&ReferralTemplate{}).
			Where("id = ?", row.Id).
			Update("bundle_key", bundleKey).Error; err != nil {
			return err
		}
	}
	return nil
}
```

```go
// model/main.go inside migrateReferralRuntimeTables()
referralRuntimeModels := []struct {
	model interface{}
	name  string
}{
	{&ReferralTemplate{}, "ReferralTemplate"},
	{&ReferralTemplateBinding{}, "ReferralTemplateBinding"},
	{&ReferralInviteeShareOverride{}, "ReferralInviteeShareOverride"},
	{&ReferralSettlementBatch{}, "ReferralSettlementBatch"},
	{&ReferralSettlementRecord{}, "ReferralSettlementRecord"},
	{&UserWithdrawal{}, "UserWithdrawal"},
}

if err := BackfillReferralTemplateBundleKeys(); err != nil {
	return fmt.Errorf("failed to backfill referral template bundle keys: %v", err)
}
```

- [ ] **Step 4: Run the model tests to verify the bundle helpers pass**

Run: `go test ./model -run 'TestReferralTemplateAllowsDuplicateNameAcrossDifferentGroups|TestCreateReferralTemplateBundleCreatesOneRowPerGroup|TestListReferralTemplateBundlesAggregatesRowsByBundleKey|TestBackfillReferralTemplateBundleKeysPopulatesLegacyRows'`

Expected: PASS

- [ ] **Step 5: Commit the bundle-aware model layer**

```bash
git add model/referral_template.go model/referral_template_test.go model/main.go
git commit -m "feat: add bundle-aware referral template model helpers"
```

### Task 2: Wire Bundle View and Bundle Writes Into the Admin API

**Files:**
- Modify: `dto/referral_admin.go`
- Modify: `controller/referral_admin.go`
- Modify: `controller/referral_admin_test.go`
- Modify: `model/referral_template.go`
- Test: `controller/referral_admin_test.go`

- [ ] **Step 1: Write the failing controller tests**

```go
func TestAdminCreateReferralTemplateWithGroupsCreatesBundleRows(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)

	body := map[string]interface{}{
		"referral_type":             model.ReferralTypeSubscription,
		"groups":                    []string{"default", "vip"},
		"name":                      "bundle-direct",
		"level_type":                model.ReferralLevelTypeDirect,
		"enabled":                   true,
		"direct_cap_bps":            1100,
		"invitee_share_default_bps": 400,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/referral/templates", body, 1)
	AdminCreateReferralTemplate(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success, got %s", response.Message)
	}

	var count int64
	if err := db.Model(&model.ReferralTemplate{}).Where("name = ?", "bundle-direct").Count(&count).Error; err != nil {
		t.Fatalf("count templates: %v", err)
	}
	if count != 2 {
		t.Fatalf("template row count = %d, want 2", count)
	}
}

func TestAdminListReferralTemplatesBundleViewAggregatesGroups(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	if _, err := model.CreateReferralTemplateBundle(model.ReferralTemplateBundleUpsertInput{
		ReferralType: model.ReferralTypeSubscription,
		Groups:       []string{"default", "vip"},
		Name:         "bundle-team",
		LevelType:    model.ReferralLevelTypeTeam,
		Enabled:      true,
		TeamCapBps:   2500,
	}, 1); err != nil {
		t.Fatalf("seed bundle: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/referral/templates?view=bundle", nil, 1)
	AdminListReferralTemplates(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success, got %s", response.Message)
	}
	if !strings.Contains(recorder.Body.String(), `"groups":["default","vip"]`) {
		t.Fatalf("expected bundle groups in response, body=%s", recorder.Body.String())
	}
}

func TestAdminUpdateReferralTemplateBundleReplacesGroupSet(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	rows, err := model.CreateReferralTemplateBundle(model.ReferralTemplateBundleUpsertInput{
		ReferralType: model.ReferralTypeSubscription,
		Groups:       []string{"default", "vip"},
		Name:         "mutable-bundle",
		LevelType:    model.ReferralLevelTypeDirect,
		Enabled:      true,
		DirectCapBps: 900,
	}, 1)
	if err != nil {
		t.Fatalf("seed bundle: %v", err)
	}

	body := map[string]interface{}{
		"referral_type":             model.ReferralTypeSubscription,
		"groups":                    []string{"vip", "premium"},
		"name":                      "mutable-bundle",
		"level_type":                model.ReferralLevelTypeDirect,
		"enabled":                   false,
		"direct_cap_bps":            1400,
		"invitee_share_default_bps": 600,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/referral/templates/"+strconv.Itoa(rows[0].Id), body, 1)
	ctx.Params = gin.Params{{Key: "id", Value: strconv.Itoa(rows[0].Id)}}
	AdminUpdateReferralTemplate(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success, got %s", response.Message)
	}

	var groups []string
	if err := db.Model(&model.ReferralTemplate{}).
		Where("bundle_key = ?", rows[0].BundleKey).
		Order("`group` ASC").
		Pluck("`group`", &groups).Error; err != nil {
		t.Fatalf("load updated groups: %v", err)
	}
	if strings.Join(groups, ",") != "premium,vip" {
		t.Fatalf("groups = %v, want [premium vip]", groups)
	}
}
```

- [ ] **Step 2: Run the controller tests to verify they fail**

Run: `go test ./controller -run 'TestAdminCreateReferralTemplateWithGroupsCreatesBundleRows|TestAdminListReferralTemplatesBundleViewAggregatesGroups|TestAdminUpdateReferralTemplateBundleReplacesGroupSet'`

Expected: FAIL because the controller only accepts `group`, never returns bundle view, and updates a single row.

- [ ] **Step 3: Implement bundle-aware request parsing and controller wiring**

```go
// dto/referral_admin.go
type ReferralTemplateUpsertRequest struct {
	ReferralType           string   `json:"referral_type"`
	Group                  string   `json:"group"`
	Groups                 []string `json:"groups"`
	Name                   string   `json:"name"`
	LevelType              string   `json:"level_type"`
	Enabled                bool     `json:"enabled"`
	DirectCapBps           int      `json:"direct_cap_bps"`
	TeamCapBps             int      `json:"team_cap_bps"`
	InviteeShareDefaultBps int      `json:"invitee_share_default_bps"`
}
```

```go
// controller/referral_admin.go
func referralTemplateRequestGroups(req dto.ReferralTemplateUpsertRequest) []string {
	if len(req.Groups) > 0 {
		return req.Groups
	}
	if trimmed := strings.TrimSpace(req.Group); trimmed != "" {
		return []string{trimmed}
	}
	return nil
}

func AdminListReferralTemplates(c *gin.Context) {
	if strings.EqualFold(strings.TrimSpace(c.Query("view")), "bundle") {
		bundles, err := model.ListReferralTemplateBundles(c.Query("referral_type"))
		if err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, gin.H{"items": bundles})
		return
	}

	templates, err := model.ListReferralTemplateRows(c.Query("referral_type"), c.Query("group"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": templates})
}

func AdminCreateReferralTemplate(c *gin.Context) {
	var req dto.ReferralTemplateUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	rows, err := model.CreateReferralTemplateBundle(model.ReferralTemplateBundleUpsertInput{
		ReferralType:           req.ReferralType,
		Groups:                 referralTemplateRequestGroups(req),
		Name:                   req.Name,
		LevelType:              req.LevelType,
		Enabled:                req.Enabled,
		DirectCapBps:           req.DirectCapBps,
		TeamCapBps:             req.TeamCapBps,
		InviteeShareDefaultBps: req.InviteeShareDefaultBps,
	}, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": rows})
}
```

```go
// model/referral_template.go
func UpdateReferralTemplateBundleByTemplateID(templateID int, input ReferralTemplateBundleUpsertInput, operatorID int) ([]ReferralTemplate, error) {
	existing, err := GetReferralTemplateByID(templateID)
	if err != nil {
		return nil, err
	}

	bundleKey := strings.TrimSpace(existing.BundleKey)
	if bundleKey == "" {
		bundleKey = fmt.Sprintf("template:%d", existing.Id)
	}

	groups := normalizeReferralTemplateGroups(input.Groups)
	if len(groups) == 0 {
		return nil, fmt.Errorf("at least one group is required")
	}

	rows := make([]ReferralTemplate, 0, len(groups))
	err = DB.Transaction(func(tx *gorm.DB) error {
		var currentRows []ReferralTemplate
		if err := tx.Where("bundle_key = ?", bundleKey).Find(&currentRows).Error; err != nil {
			return err
		}
		if len(currentRows) == 0 {
			existing.BundleKey = bundleKey
			currentRows = []ReferralTemplate{*existing}
		}

		currentByGroup := make(map[string]ReferralTemplate, len(currentRows))
		for _, row := range currentRows {
			currentByGroup[row.Group] = row
		}

		for _, group := range groups {
			row, exists := currentByGroup[group]
			if !exists {
				row = ReferralTemplate{
					BundleKey: bundleKey,
					CreatedBy: operatorID,
				}
			}
			row.ReferralType = strings.TrimSpace(input.ReferralType)
			row.BundleKey = bundleKey
			row.Group = group
			row.Name = strings.TrimSpace(input.Name)
			row.LevelType = strings.TrimSpace(input.LevelType)
			row.Enabled = input.Enabled
			row.DirectCapBps = input.DirectCapBps
			row.TeamCapBps = input.TeamCapBps
			row.InviteeShareDefaultBps = input.InviteeShareDefaultBps
			row.UpdatedBy = operatorID

			var saveErr error
			if row.Id == 0 {
				saveErr = tx.Create(&row).Error
			} else {
				saveErr = tx.Save(&row).Error
			}
			if err := normalizeReferralTemplatePersistenceError(saveErr); err != nil {
				return err
			}
			rows = append(rows, row)
			delete(currentByGroup, group)
		}

		for _, stale := range currentByGroup {
			if err := tx.Delete(&ReferralTemplate{}, stale.Id).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return rows, err
}

func DeleteReferralTemplateBundleByTemplateID(templateID int) error {
	template, err := GetReferralTemplateByID(templateID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(template.BundleKey) == "" {
		return DB.Delete(&ReferralTemplate{}, template.Id).Error
	}
	return DB.Where("bundle_key = ?", template.BundleKey).Delete(&ReferralTemplate{}).Error
}
```

- [ ] **Step 4: Run the controller tests to verify the API now supports bundle behavior**

Run: `go test ./controller -run 'TestAdminCreateReferralTemplateWithGroupsCreatesBundleRows|TestAdminListReferralTemplatesBundleViewAggregatesGroups|TestAdminUpdateReferralTemplateBundleReplacesGroupSet|TestAdminUpsertReferralTemplateBindingUsesTemplateScope'`

Expected: PASS

- [ ] **Step 5: Commit the bundle-aware admin API**

```bash
git add dto/referral_admin.go controller/referral_admin.go controller/referral_admin_test.go model/referral_template.go
git commit -m "feat: add bundle-aware referral template admin api"
```

### Task 3: Switch the Settings Page to Bundle View and Multi-Select Groups

**Files:**
- Modify: `web/src/helpers/referralTemplate.js`
- Modify: `web/src/pages/Setting/Referral/SettingsReferralTemplates.jsx`
- Modify: `web/tests/referral-settings-route.test.mjs`
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/fr.json`
- Modify: `web/src/i18n/locales/ja.json`
- Modify: `web/src/i18n/locales/ru.json`
- Modify: `web/src/i18n/locales/vi.json`
- Modify: `web/src/i18n/locales/zh-CN.json`
- Modify: `web/src/i18n/locales/zh-TW.json`
- Test: `web/tests/referral-settings-route.test.mjs`

- [ ] **Step 1: Write the failing settings-page tests**

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';

test('referral settings page loads template bundles and renders multi-group editor copy', () => {
  const templateSource = fs.readFileSync(
    'web/src/pages/Setting/Referral/SettingsReferralTemplates.jsx',
    'utf8',
  );

  assert.match(templateSource, /API\.get\('\/api\/referral\/templates',\s*\{\s*params:\s*\{\s*view:\s*'bundle'/);
  assert.match(templateSource, /multiple/);
  assert.match(templateSource, /groups:/);
  assert.match(templateSource, /选择一个或多个已存在的系统分组/);
  assert.match(templateSource, /会为每个分组生成一条运行时模板/);
  assert.doesNotMatch(templateSource, /模板名全局唯一/);
});
```

```js
test('bundle normalization helper keeps groups arrays for the settings page', async () => {
  const { normalizeReferralTemplateItems } = await import('../src/helpers/referralTemplate.js');
  assert.deepEqual(
    normalizeReferralTemplateItems([
      {
        bundle_key: 'bundle-1',
        template_ids: [11, 12],
        referral_type: 'subscription_referral',
        groups: ['default', 'vip'],
        name: 'starter',
        level_type: 'direct',
        enabled: true,
        direct_cap_bps: 1000,
        team_cap_bps: 0,
        invitee_share_default_bps: 300,
      },
    ])[0],
    {
      id: 11,
      bundleKey: 'bundle-1',
      templateIds: [11, 12],
      referralType: 'subscription_referral',
      groups: ['default', 'vip'],
      name: 'starter',
      levelType: 'direct',
      enabled: true,
      directCapBps: 1000,
      teamCapBps: 0,
      inviteeShareDefaultBps: 300,
    },
  );
});
```

- [ ] **Step 2: Run the settings tests to verify they fail**

Run: `node --test web/tests/referral-settings-route.test.mjs`

Expected: FAIL because the page still loads row view, stores a single `group`, and mentions global name uniqueness.

- [ ] **Step 3: Implement the bundle-view normalization and multi-select editor**

```js
// web/src/helpers/referralTemplate.js
export function normalizeReferralTemplateItems(items = []) {
  if (!Array.isArray(items)) {
    return [];
  }

  return items.map((item) => ({
    id: Number(item?.id || item?.template_ids?.[0] || 0),
    bundleKey: String(item?.bundle_key || '').trim(),
    templateIds: Array.isArray(item?.template_ids)
      ? item.template_ids.map((id) => Number(id || 0)).filter(Boolean)
      : [],
    referralType: String(item?.referral_type || '').trim(),
    groups: Array.isArray(item?.groups)
      ? item.groups.map((group) => String(group || '').trim()).filter(Boolean)
      : [String(item?.group || '').trim()].filter(Boolean),
    name: String(item?.name || '').trim(),
    levelType: String(item?.level_type || '').trim(),
    enabled: Boolean(item?.enabled),
    directCapBps: Number(item?.direct_cap_bps || 0),
    teamCapBps: Number(item?.team_cap_bps || 0),
    inviteeShareDefaultBps: Number(item?.invitee_share_default_bps || 0),
  }));
}
```

```jsx
// web/src/pages/Setting/Referral/SettingsReferralTemplates.jsx
const createDraftTemplate = () => ({
  id: `draft-${Date.now()}-${Math.random()}`,
  bundleKey: '',
  templateIds: [],
  referralType: 'subscription_referral',
  groups: [],
  name: '',
  levelType: 'direct',
  enabled: true,
  directCapBps: 1000,
  teamCapBps: 0,
  inviteeShareDefaultBps: 0,
  isDraft: true,
});

const load = async () => {
  setLoading(true);
  try {
    const [templateRes, groupRes, settingRes] = await Promise.all([
      API.get('/api/referral/templates', {
        params: { view: 'bundle' },
      }),
      API.get('/api/group'),
      API.get('/api/referral/settings/subscription'),
    ]);
    if (templateRes.data?.success) {
      setItems(normalizeReferralTemplateItems(templateRes.data?.data?.items));
    }
    // keep the existing group and setting loading code
  } finally {
    setLoading(false);
  }
};

const saveRow = async (row) => {
  const payload = {
    referral_type: row.referralType,
    groups: row.groups,
    name: row.name,
    level_type: row.levelType,
    enabled: row.enabled,
    direct_cap_bps: row.levelType === 'direct' ? Number(row.directCapBps || 0) : 0,
    team_cap_bps: row.levelType === 'team' ? Number(row.teamCapBps || 0) : 0,
    invitee_share_default_bps: Number(row.inviteeShareDefaultBps || 0),
  };
  const res = row.isDraft
    ? await API.post('/api/referral/templates', payload)
    : await API.put(`/api/referral/templates/${row.id}`, payload);
  // keep existing success/error handling
};
```

```jsx
// web/src/pages/Setting/Referral/SettingsReferralTemplates.jsx inside the field block
<ReferralFieldBlock
  label={t('分组')}
  description={t('选择一个或多个已存在的系统分组。保存后会为每个分组生成一条运行时模板；结算和用户激活仍按返佣类型 + 单个分组命中。')}
>
  <Select
    value={row.groups}
    optionList={groupOptions}
    placeholder={t('分组')}
    multiple
    onChange={(values) =>
      updateRow(row.id, {
        groups: Array.isArray(values)
          ? values.map((value) => String(value || '').trim()).filter(Boolean)
          : [],
      })
    }
  />
</ReferralFieldBlock>

<ReferralFieldBlock
  label={t('模板名')}
  description={t('只用于后台识别，不参与返佣计算。保存到多个分组时会复用同一个模板名；唯一性按返佣类型 + 分组 + 模板名校验。')}
>
  <Input
    value={row.name}
    placeholder={t('模板名')}
    onChange={(value) => updateRow(row.id, { name: value })}
  />
</ReferralFieldBlock>
```

```json
// web/src/i18n/locales/zh-CN.json
{
  "管理返佣类型与分组下的模板配置；一个模板组可以覆盖多个分组。": "管理返佣类型与分组下的模板配置；一个模板组可以覆盖多个分组。",
  "选择一个或多个已存在的系统分组。保存后会为每个分组生成一条运行时模板；结算和用户激活仍按返佣类型 + 单个分组命中。": "选择一个或多个已存在的系统分组。保存后会为每个分组生成一条运行时模板；结算和用户激活仍按返佣类型 + 单个分组命中。",
  "只用于后台识别，不参与返佣计算。保存到多个分组时会复用同一个模板名；唯一性按返佣类型 + 分组 + 模板名校验。": "只用于后台识别，不参与返佣计算。保存到多个分组时会复用同一个模板名；唯一性按返佣类型 + 分组 + 模板名校验。",
  "确认删除该返佣模板组？": "确认删除该返佣模板组？"
}
```

- [ ] **Step 4: Run the settings tests to verify the bundle page passes**

Run: `node --test web/tests/referral-settings-route.test.mjs`

Expected: PASS

- [ ] **Step 5: Commit the multi-group settings editor**

```bash
git add web/src/helpers/referralTemplate.js web/src/pages/Setting/Referral/SettingsReferralTemplates.jsx web/tests/referral-settings-route.test.mjs web/src/i18n/locales/en.json web/src/i18n/locales/fr.json web/src/i18n/locales/ja.json web/src/i18n/locales/ru.json web/src/i18n/locales/vi.json web/src/i18n/locales/zh-CN.json web/src/i18n/locales/zh-TW.json
git commit -m "feat: add multi-group referral template settings editor"
```

### Task 4: Keep the Binding Modal on Row View and Disambiguate Same-Name Templates

**Files:**
- Modify: `web/src/helpers/referralLabels.js`
- Modify: `web/src/components/table/users/modals/ReferralTemplateBindingSection.jsx`
- Modify: `web/tests/referral-template-labels.test.mjs`
- Modify: `web/tests/user-referral-binding.test.mjs`
- Test: `web/tests/referral-template-labels.test.mjs`
- Test: `web/tests/user-referral-binding.test.mjs`

- [ ] **Step 1: Write the failing label/binding tests**

```js
import test from 'node:test';
import assert from 'node:assert/strict';

import { formatReferralTemplateOptionLabel } from '../src/helpers/referralLabels.js';

const fakeT = (key) => ({ '直推模板（direct）': '直推模板（direct）' }[key] || key);

test('named row templates can include group suffixes when the caller requests it', () => {
  assert.equal(
    formatReferralTemplateOptionLabel(
      {
        name: 'starter',
        group: 'vip',
        level_type: 'direct',
      },
      fakeT,
      { includeGroupSuffixWhenNamed: true },
    ),
    'starter · vip',
  );
});
```

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';

test('binding modal keeps row view and disambiguates same-name options by group', () => {
  const source = fs.readFileSync(
    'web/src/components/table/users/modals/ReferralTemplateBindingSection.jsx',
    'utf8',
  );
  assert.match(source, /params:\s*\{\s*referral_type:\s*'subscription_referral',\s*view:\s*'row'/);
  assert.match(source, /includeGroupSuffixWhenNamed:\s*true/);
  assert.doesNotMatch(source, /{t\('分组'\)}/);
});
```

- [ ] **Step 2: Run the label and binding tests to verify they fail**

Run: `node --test web/tests/referral-template-labels.test.mjs web/tests/user-referral-binding.test.mjs`

Expected: FAIL because the label helper ignores group when a name exists and the binding modal does not request row view explicitly.

- [ ] **Step 3: Implement row-view label disambiguation**

```js
// web/src/helpers/referralLabels.js
export function formatReferralTemplateOptionLabel(template, t, options = {}) {
  const name = String(template?.name || '').trim();
  const groupLabel = formatReferralGroupLabel(template?.group, t);
  const includeGroupSuffixWhenNamed = Boolean(options?.includeGroupSuffixWhenNamed);

  if (name !== '') {
    return includeGroupSuffixWhenNamed ? `${name} · ${groupLabel}` : name;
  }

  return [groupLabel, formatReferralLevelTypeLabel(template?.level_type, t)]
    .filter(Boolean)
    .join(' · ');
}
```

```jsx
// web/src/components/table/users/modals/ReferralTemplateBindingSection.jsx
const templateOptions = useMemo(
  () =>
    templates.map((template) => ({
      label: formatReferralTemplateOptionLabel(template, t, {
        includeGroupSuffixWhenNamed: true,
      }),
      value: template.id,
    })),
  [t, templates],
);

const [templateRes, bindingRes] = await Promise.all([
  API.get('/api/referral/templates', {
    params: { referral_type: 'subscription_referral', view: 'row' },
  }),
  API.get(`/api/referral/bindings/users/${userId}`, {
    params: { referral_type: 'subscription_referral' },
  }),
]);
```

- [ ] **Step 4: Run the row-label tests to verify binding compatibility**

Run: `node --test web/tests/referral-template-labels.test.mjs web/tests/user-referral-binding.test.mjs`

Expected: PASS

- [ ] **Step 5: Commit the binding compatibility pass**

```bash
git add web/src/helpers/referralLabels.js web/src/components/table/users/modals/ReferralTemplateBindingSection.jsx web/tests/referral-template-labels.test.mjs web/tests/user-referral-binding.test.mjs
git commit -m "fix: disambiguate referral template binding labels by group"
```

### Task 5: Run the Targeted Regression Suite and Capture the New Contract

**Files:**
- Modify: `controller/referral_admin_test.go`
- Modify: `model/referral_template_test.go`
- Modify: `web/tests/referral-settings-route.test.mjs`
- Modify: `web/tests/referral-template-labels.test.mjs`
- Modify: `web/tests/user-referral-binding.test.mjs`
- Modify: `docs/superpowers/specs/2026-04-20-referral-template-multi-group-design.md`
- Test: `go test ./model`
- Test: `go test ./controller`
- Test: `node --test web/tests/referral-settings-route.test.mjs web/tests/referral-template-labels.test.mjs web/tests/user-referral-binding.test.mjs`

- [ ] **Step 1: Add the missing rollback and delete regression tests**

```go
func TestAdminCreateReferralTemplateWithGroupsRollsBackOnConflict(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	if _, err := model.CreateReferralTemplateBundle(model.ReferralTemplateBundleUpsertInput{
		ReferralType: model.ReferralTypeSubscription,
		Groups:       []string{"vip"},
		Name:         "conflict-name",
		LevelType:    model.ReferralLevelTypeDirect,
		Enabled:      true,
		DirectCapBps: 1000,
	}, 1); err != nil {
		t.Fatalf("seed existing row: %v", err)
	}

	body := map[string]interface{}{
		"referral_type":  model.ReferralTypeSubscription,
		"groups":         []string{"default", "vip"},
		"name":           "conflict-name",
		"level_type":     model.ReferralLevelTypeDirect,
		"enabled":        true,
		"direct_cap_bps": 1000,
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/referral/templates", body, 1)
	AdminCreateReferralTemplate(ctx)

	response := decodeAPIResponse(t, recorder)
	if response.Success {
		t.Fatal("expected conflict create to fail")
	}

	var count int64
	if err := db.Model(&model.ReferralTemplate{}).Where("name = ? AND `group` = ?", "conflict-name", "default").Count(&count).Error; err != nil {
		t.Fatalf("count default rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("default group should not be partially inserted, count=%d", count)
	}
}
```

```go
func TestDeleteReferralTemplateBundleByTemplateIDRemovesAllRows(t *testing.T) {
	db := setupReferralTemplateDB(t)
	rows, err := CreateReferralTemplateBundle(ReferralTemplateBundleUpsertInput{
		ReferralType: ReferralTypeSubscription,
		Groups:       []string{"default", "vip"},
		Name:         "delete-me",
		LevelType:    ReferralLevelTypeTeam,
		Enabled:      true,
		TeamCapBps:   2300,
	}, 1)
	if err != nil {
		t.Fatalf("CreateReferralTemplateBundle() error = %v", err)
	}

	if err := DeleteReferralTemplateBundleByTemplateID(rows[0].Id); err != nil {
		t.Fatalf("DeleteReferralTemplateBundleByTemplateID() error = %v", err)
	}

	var count int64
	if err := db.Model(&ReferralTemplate{}).Where("bundle_key = ?", rows[0].BundleKey).Count(&count).Error; err != nil {
		t.Fatalf("count deleted rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("bundle row count = %d, want 0", count)
	}
}
```

- [ ] **Step 2: Run the focused regression suite and verify at least one test still fails**

Run: `go test ./model -run 'TestDeleteReferralTemplateBundleByTemplateIDRemovesAllRows|TestCreateReferralTemplateBundleCreatesOneRowPerGroup'`

Run: `go test ./controller -run 'TestAdminCreateReferralTemplateWithGroupsRollsBackOnConflict|TestAdminListReferralTemplatesBundleViewAggregatesGroups'`

Run: `node --test web/tests/referral-settings-route.test.mjs web/tests/referral-template-labels.test.mjs web/tests/user-referral-binding.test.mjs`

Expected: Any newly added regression that is not covered yet should fail before the last code pass.

- [ ] **Step 3: Patch the remaining gaps and update the spec notes if field names drifted**

```go
// controller/referral_admin.go
func AdminDeleteReferralTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}
	if err := model.DeleteReferralTemplateBundleByTemplateID(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"id": id})
}
```

```md
<!-- docs/superpowers/specs/2026-04-20-referral-template-multi-group-design.md -->
- `GET /api/referral/templates` keeps `row view` as the default contract for binding pages.
- `GET /api/referral/templates?view=bundle` is the admin-only aggregated contract for multi-group management.
```

- [ ] **Step 4: Run the final targeted suite**

Run: `go test ./model -run 'TestReferralTemplateAllowsDuplicateNameAcrossDifferentGroups|TestCreateReferralTemplateBundleCreatesOneRowPerGroup|TestListReferralTemplateBundlesAggregatesRowsByBundleKey|TestDeleteReferralTemplateBundleByTemplateIDRemovesAllRows'`

Run: `go test ./controller -run 'TestAdminCreateReferralTemplateWithGroupsCreatesBundleRows|TestAdminListReferralTemplatesBundleViewAggregatesGroups|TestAdminUpdateReferralTemplateBundleReplacesGroupSet|TestAdminCreateReferralTemplateWithGroupsRollsBackOnConflict|TestAdminUpsertReferralTemplateBindingUsesTemplateScope'`

Run: `node --test web/tests/referral-settings-route.test.mjs web/tests/referral-template-labels.test.mjs web/tests/user-referral-binding.test.mjs`

Expected: PASS on all three commands

- [ ] **Step 5: Commit the regression coverage**

```bash
git add controller/referral_admin.go controller/referral_admin_test.go model/referral_template.go model/referral_template_test.go web/tests/referral-settings-route.test.mjs web/tests/referral-template-labels.test.mjs web/tests/user-referral-binding.test.mjs docs/superpowers/specs/2026-04-20-referral-template-multi-group-design.md
git commit -m "test: cover referral template multi-group regression paths"
```
