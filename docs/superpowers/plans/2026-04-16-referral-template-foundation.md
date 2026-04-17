# Referral Template Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Introduce the generic referral template data model, engine-route registry, generic invitee-share persistence, and admin CRUD APIs without changing live settlement behavior yet.

**Architecture:** Add the new referral framework as a parallel backend layer beside the existing `subscription_referral_*` tables. The runtime must still default to legacy settlement until a later plan flips `referral_engine_routes` to `template`, so this plan focuses on schema, validation, repositories, and admin-only APIs.

**Tech Stack:** Go 1.26, Gin, Gorm, existing controller/model test harness, SQLite/PostgreSQL/MySQL migration paths.

---

### Task 1: Add Generic Referral Framework Persistence

**Files:**
- Create: `model/referral_template.go`
- Create: `model/referral_template_binding.go`
- Create: `model/referral_invitee_share_override.go`
- Create: `model/referral_engine_route.go`
- Create: `model/referral_settlement_batch.go`
- Create: `model/referral_settlement_record.go`
- Create: `model/referral_template_test.go`
- Modify: `model/main.go`
- Test: `model/referral_template_test.go`

- [ ] **Step 1: Write the failing model tests**

```go
package model

func TestReferralTemplateRejectsInvalidSubscriptionRules(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	DB = db
	if err := db.AutoMigrate(&ReferralTemplate{}); err != nil {
		t.Fatalf("auto migrate referral template: %v", err)
	}

	tpl := &ReferralTemplate{
		ReferralType:           ReferralTypeSubscription,
		Group:                  "vip",
		Name:                   "bad-direct",
		LevelType:              ReferralLevelTypeDirect,
		Enabled:                true,
		DirectCapBps:           2600,
		TeamCapBps:             2500,
		TeamDecayRatio:         0.5,
		TeamMaxDepth:           3,
		InviteeShareDefaultBps: 1200,
	}

	err := tpl.Validate()
	if err == nil || !strings.Contains(err.Error(), "direct_cap_bps") {
		t.Fatalf("Validate() error = %v, want direct/team cap validation", err)
	}
}

func TestReferralTemplateBindingRejectsCrossDimensionTemplate(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	DB = db
	if err := db.AutoMigrate(&ReferralTemplate{}, &ReferralTemplateBinding{}); err != nil {
		t.Fatalf("auto migrate referral template + binding: %v", err)
	}

	tpl := ReferralTemplate{
		ReferralType: ReferralTypeSubscription,
		Group:        "vip",
		Name:         "direct-vip",
		LevelType:    ReferralLevelTypeDirect,
		Enabled:      true,
		DirectCapBps: 1000,
		TeamCapBps:   2500,
		TeamDecayRatio: 0.5,
		TeamMaxDepth: 3,
	}
	if err := db.Create(&tpl).Error; err != nil {
		t.Fatalf("create template: %v", err)
	}

	binding := ReferralTemplateBinding{
		UserId:       1,
		ReferralType: ReferralTypeSubscription,
		Group:        "retail",
		TemplateId:   tpl.Id,
	}
	err := binding.ValidateAgainstTemplate(&tpl)
	if err == nil || !strings.Contains(err.Error(), "binding group") {
		t.Fatalf("ValidateAgainstTemplate() error = %v, want cross-dimension rejection", err)
	}
}

func TestResolveReferralEngineModeDefaultsToLegacy(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	DB = db
	if err := db.AutoMigrate(&ReferralEngineRoute{}); err != nil {
		t.Fatalf("auto migrate referral engine route: %v", err)
	}

	mode, err := ResolveReferralEngineMode(ReferralTypeSubscription, "vip")
	if err != nil {
		t.Fatalf("ResolveReferralEngineMode() error = %v", err)
	}
	if mode != ReferralEngineModeLegacy {
		t.Fatalf("ResolveReferralEngineMode() = %q, want %q", mode, ReferralEngineModeLegacy)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./model -run 'TestReferralTemplateRejectsInvalidSubscriptionRules|TestReferralTemplateBindingRejectsCrossDimensionTemplate|TestResolveReferralEngineModeDefaultsToLegacy'`
Expected: FAIL with errors such as `undefined: ReferralTemplate`, `undefined: ReferralTemplateBinding`, and `undefined: ResolveReferralEngineMode`.

- [ ] **Step 3: Write the minimal persistence models and migration wiring**

```go
package model

const (
	ReferralTypeSubscription = "subscription_referral"

	ReferralLevelTypeDirect = "direct"
	ReferralLevelTypeTeam   = "team"

	ReferralEngineModeLegacy   = "legacy"
	ReferralEngineModeTemplate = "template"
)

type ReferralTemplate struct {
	Id                     int     `json:"id"`
	ReferralType           string  `json:"referral_type" gorm:"type:varchar(64);index:idx_referral_template_scope_name,priority:1"`
	Group                  string  `json:"group" gorm:"type:varchar(64);index:idx_referral_template_scope_name,priority:2"`
	Name                   string  `json:"name" gorm:"type:varchar(128);index:idx_referral_template_scope_name,priority:3"`
	LevelType              string  `json:"level_type" gorm:"type:varchar(32);index"`
	Enabled                bool    `json:"enabled" gorm:"not null;default:false"`
	DirectCapBps           int     `json:"direct_cap_bps" gorm:"not null;default:0"`
	TeamCapBps             int     `json:"team_cap_bps" gorm:"not null;default:0"`
	TeamDecayRatio         float64 `json:"team_decay_ratio" gorm:"not null;default:0"`
	TeamMaxDepth           int     `json:"team_max_depth" gorm:"not null;default:0"`
	InviteeShareDefaultBps int     `json:"invitee_share_default_bps" gorm:"not null;default:0"`
	CreatedBy              int     `json:"created_by" gorm:"not null;default:0"`
	UpdatedBy              int     `json:"updated_by" gorm:"not null;default:0"`
	CreatedAt              int64   `json:"created_at" gorm:"bigint"`
	UpdatedAt              int64   `json:"updated_at" gorm:"bigint"`
}

func (t *ReferralTemplate) Validate() error {
	t.ReferralType = strings.TrimSpace(t.ReferralType)
	t.Group = strings.TrimSpace(t.Group)
	t.Name = strings.TrimSpace(t.Name)
	t.LevelType = strings.TrimSpace(t.LevelType)
	t.InviteeShareDefaultBps = NormalizeSubscriptionReferralRateBps(t.InviteeShareDefaultBps)
	if t.ReferralType == ReferralTypeSubscription {
		if t.LevelType != ReferralLevelTypeDirect && t.LevelType != ReferralLevelTypeTeam {
			return fmt.Errorf("invalid subscription level_type %q", t.LevelType)
		}
		if t.DirectCapBps < 0 || t.TeamCapBps < t.DirectCapBps || t.TeamCapBps > 10000 {
			return fmt.Errorf("invalid direct_cap_bps/team_cap_bps")
		}
		if t.TeamDecayRatio <= 0 || t.TeamDecayRatio > 1 {
			return fmt.Errorf("invalid team_decay_ratio")
		}
		if t.TeamMaxDepth < 1 {
			return fmt.Errorf("invalid team_max_depth")
		}
	}
	return nil
}

type ReferralTemplateBinding struct {
	Id                      int    `json:"id"`
	UserId                  int    `json:"user_id" gorm:"uniqueIndex:idx_referral_template_binding_scope"`
	ReferralType            string `json:"referral_type" gorm:"type:varchar(64);uniqueIndex:idx_referral_template_binding_scope"`
	Group                   string `json:"group" gorm:"type:varchar(64);uniqueIndex:idx_referral_template_binding_scope"`
	TemplateId              int    `json:"template_id" gorm:"index"`
	InviteeShareOverrideBps *int   `json:"invitee_share_override_bps"`
	CreatedBy               int    `json:"created_by" gorm:"not null;default:0"`
	UpdatedBy               int    `json:"updated_by" gorm:"not null;default:0"`
	CreatedAt               int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt               int64  `json:"updated_at" gorm:"bigint"`
}

func (b *ReferralTemplateBinding) ValidateAgainstTemplate(tpl *ReferralTemplate) error {
	if tpl == nil {
		return errors.New("template is required")
	}
	if strings.TrimSpace(b.ReferralType) != strings.TrimSpace(tpl.ReferralType) {
		return fmt.Errorf("binding referral type %q does not match template", b.ReferralType)
	}
	if strings.TrimSpace(b.Group) != strings.TrimSpace(tpl.Group) {
		return fmt.Errorf("binding group %q does not match template", b.Group)
	}
	return nil
}

type ReferralEngineRoute struct {
	Id           int    `json:"id"`
	ReferralType string `json:"referral_type" gorm:"type:varchar(64);uniqueIndex:idx_referral_engine_route_scope"`
	Group        string `json:"group" gorm:"type:varchar(64);uniqueIndex:idx_referral_engine_route_scope"`
	EngineMode   string `json:"engine_mode" gorm:"type:varchar(32);not null;default:'legacy'"`
	CreatedBy    int    `json:"created_by" gorm:"not null;default:0"`
	UpdatedBy    int    `json:"updated_by" gorm:"not null;default:0"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt    int64  `json:"updated_at" gorm:"bigint"`
}

func ResolveReferralEngineMode(referralType string, group string) (string, error) {
	var route ReferralEngineRoute
	err := DB.Where("referral_type = ? AND "+commonGroupCol+" = ?", strings.TrimSpace(referralType), strings.TrimSpace(group)).First(&route).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ReferralEngineModeLegacy, nil
	}
	if err != nil {
		return "", err
	}
	return route.EngineMode, nil
}

type ReferralSettlementBatch struct {
	Id                       int     `json:"id"`
	ReferralType             string  `json:"referral_type" gorm:"type:varchar(64);index"`
	Group                    string  `json:"group" gorm:"type:varchar(64);index"`
	SourceType               string  `json:"source_type" gorm:"type:varchar(64);index"`
	SourceId                 int     `json:"source_id" gorm:"index"`
	SourceTradeNo            string  `json:"source_trade_no" gorm:"type:varchar(255);uniqueIndex"`
	PayerUserId              int     `json:"payer_user_id" gorm:"index"`
	ImmediateInviterUserId   int     `json:"immediate_inviter_user_id" gorm:"index"`
	ActiveTemplateSnapshotJSON string `json:"active_template_snapshot_json" gorm:"type:text"`
	TeamChainSnapshotJSON      string `json:"team_chain_snapshot_json" gorm:"type:text"`
	SettlementMode           string  `json:"settlement_mode" gorm:"type:varchar(64);index"`
	QuotaPerUnitSnapshot     float64 `json:"quota_per_unit_snapshot"`
	Status                   string  `json:"status" gorm:"type:varchar(32);index"`
	SettledAt                int64   `json:"settled_at" gorm:"bigint"`
	CreatedAt                int64   `json:"created_at" gorm:"bigint"`
	UpdatedAt                int64   `json:"updated_at" gorm:"bigint"`
}

type ReferralSettlementRecord struct {
	Id                       int    `json:"id"`
	BatchId                  int    `json:"batch_id" gorm:"index"`
	ReferralType             string `json:"referral_type" gorm:"type:varchar(64);index"`
	Group                    string `json:"group" gorm:"type:varchar(64);index"`
	BeneficiaryUserId        int    `json:"beneficiary_user_id" gorm:"index"`
	BeneficiaryLevelType     *string `json:"beneficiary_level_type" gorm:"type:varchar(32)"`
	RewardComponent          string `json:"reward_component" gorm:"type:varchar(64);index"`
	SourceRewardComponent    *string `json:"source_reward_component" gorm:"type:varchar(64)"`
	PathDistance             *int    `json:"path_distance"`
	MatchedTeamIndex         *int    `json:"matched_team_index"`
	WeightSnapshot           *float64 `json:"weight_snapshot"`
	ShareSnapshot            *float64 `json:"share_snapshot"`
	GrossRewardQuotaSnapshot *int64   `json:"gross_reward_quota_snapshot"`
	InviteeShareBpsSnapshot  *int     `json:"invitee_share_bps_snapshot"`
	PoolRateBpsSnapshot      *int     `json:"pool_rate_bps_snapshot"`
	AppliedRateBps           *int     `json:"applied_rate_bps"`
	RewardQuota              int64    `json:"reward_quota"`
	ReversedQuota            int64    `json:"reversed_quota"`
	DebtQuota                int64    `json:"debt_quota"`
	Status                   string   `json:"status" gorm:"type:varchar(32);index"`
	CreatedAt                int64    `json:"created_at" gorm:"bigint"`
	UpdatedAt                int64    `json:"updated_at" gorm:"bigint"`
}
```

```go
// model/main.go
err := DB.AutoMigrate(
	&ReferralTemplate{},
	&ReferralTemplateBinding{},
	&ReferralInviteeShareOverride{},
	&ReferralEngineRoute{},
	&ReferralSettlementBatch{},
	&ReferralSettlementRecord{},
)
```

- [ ] **Step 4: Run the targeted tests to verify the models pass**

Run: `go test ./model -run 'TestReferralTemplateRejectsInvalidSubscriptionRules|TestReferralTemplateBindingRejectsCrossDimensionTemplate|TestResolveReferralEngineModeDefaultsToLegacy'`
Expected: PASS

- [ ] **Step 5: Commit the persistence layer**

```bash
git add model/main.go model/referral_template.go model/referral_template_binding.go model/referral_invitee_share_override.go model/referral_engine_route.go model/referral_settlement_batch.go model/referral_settlement_record.go model/referral_template_test.go
git commit -m "feat: add referral template foundation models"
```

### Task 2: Add Repository Helpers and Legacy Seed Read Models

**Files:**
- Create: `model/referral_seed.go`
- Modify: `model/referral_template.go`
- Modify: `model/referral_template_binding.go`
- Modify: `model/referral_invitee_share_override.go`
- Modify: `model/referral_template_test.go`
- Test: `model/referral_template_test.go`

- [ ] **Step 1: Write the failing repository/helper tests**

```go
func TestUpsertReferralInviteeShareOverrideRequiresActiveBinding(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	DB = db
	if err := db.AutoMigrate(&User{}, &ReferralTemplate{}, &ReferralTemplateBinding{}, &ReferralInviteeShareOverride{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}

	inviter := User{Username: "inviter", Password: "hashed", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	invitee := User{Username: "invitee", Password: "hashed", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, InviterId: 1}
	if err := db.Create(&inviter).Error; err != nil {
		t.Fatalf("create inviter: %v", err)
	}
	invitee.InviterId = inviter.Id
	if err := db.Create(&invitee).Error; err != nil {
		t.Fatalf("create invitee: %v", err)
	}

	_, err := UpsertReferralInviteeShareOverride(inviter.Id, invitee.Id, ReferralTypeSubscription, "vip", 1200, inviter.Id)
	if err == nil || !strings.Contains(err.Error(), "active binding") {
		t.Fatalf("UpsertReferralInviteeShareOverride() error = %v, want active binding validation", err)
	}
}

func TestListLegacySubscriptionReferralSeedRowsReturnsOverrideAndInviteeSeeds(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	DB = db
	if err := db.AutoMigrate(&User{}, &SubscriptionReferralOverride{}, &SubscriptionReferralInviteeOverride{}); err != nil {
		t.Fatalf("auto migrate legacy referral tables: %v", err)
	}

	seedRows, err := ListLegacySubscriptionReferralSeedRows("vip")
	if err != nil {
		t.Fatalf("ListLegacySubscriptionReferralSeedRows() error = %v", err)
	}
	if seedRows == nil {
		t.Fatal("ListLegacySubscriptionReferralSeedRows() returned nil")
	}
}
```

- [ ] **Step 2: Run the helper tests to verify they fail**

Run: `go test ./model -run 'TestUpsertReferralInviteeShareOverrideRequiresActiveBinding|TestListLegacySubscriptionReferralSeedRowsReturnsOverrideAndInviteeSeeds'`
Expected: FAIL with `undefined: UpsertReferralInviteeShareOverride` and `undefined: ListLegacySubscriptionReferralSeedRows`.

- [ ] **Step 3: Implement the repository helpers and read-only legacy seed exporter**

```go
type ReferralInviteeShareOverride struct {
	Id               int    `json:"id"`
	InviterUserId    int    `json:"inviter_user_id" gorm:"uniqueIndex:idx_referral_invitee_share_override_scope"`
	InviteeUserId    int    `json:"invitee_user_id" gorm:"uniqueIndex:idx_referral_invitee_share_override_scope"`
	ReferralType     string `json:"referral_type" gorm:"type:varchar(64);uniqueIndex:idx_referral_invitee_share_override_scope"`
	Group            string `json:"group" gorm:"type:varchar(64);uniqueIndex:idx_referral_invitee_share_override_scope"`
	InviteeShareBps  int    `json:"invitee_share_bps" gorm:"not null;default:0"`
	CreatedBy        int    `json:"created_by" gorm:"not null;default:0"`
	UpdatedBy        int    `json:"updated_by" gorm:"not null;default:0"`
	CreatedAt        int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt        int64  `json:"updated_at" gorm:"bigint"`
}

func HasActiveReferralTemplateBinding(userID int, referralType string, group string) (bool, *ReferralTemplateBinding, error) {
	var binding ReferralTemplateBinding
	err := DB.Where("user_id = ? AND referral_type = ? AND "+commonGroupCol+" = ?", userID, strings.TrimSpace(referralType), strings.TrimSpace(group)).
		First(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}
	tpl, err := GetReferralTemplateByID(binding.TemplateId)
	if err != nil {
		return false, nil, err
	}
	return tpl.Enabled, &binding, nil
}

func UpsertReferralInviteeShareOverride(inviterUserID int, inviteeUserID int, referralType string, group string, inviteeShareBps int, operatorID int) (*ReferralInviteeShareOverride, error) {
	if err := validateSubscriptionReferralInviteeOwnership(inviterUserID, inviteeUserID); err != nil {
		return nil, err
	}
	active, _, err := HasActiveReferralTemplateBinding(inviterUserID, referralType, group)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, errors.New("active binding is required before invitee share override can be written")
	}
	override := &ReferralInviteeShareOverride{
		InviterUserId:   inviterUserID,
		InviteeUserId:   inviteeUserID,
		ReferralType:    strings.TrimSpace(referralType),
		Group:           strings.TrimSpace(group),
		InviteeShareBps: NormalizeSubscriptionReferralRateBps(inviteeShareBps),
		CreatedBy:       operatorID,
		UpdatedBy:       operatorID,
	}
	return override, DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "inviter_user_id"}, {Name: "invitee_user_id"}, {Name: "referral_type"}, {Name: "group"}},
		DoUpdates: clause.Assignments(map[string]any{
			"invitee_share_bps": override.InviteeShareBps,
			"updated_by":        operatorID,
			"updated_at":        common.GetTimestamp(),
		}),
	}).Create(override).Error
}

func GetReferralTemplateByID(id int) (*ReferralTemplate, error) {
	var tpl ReferralTemplate
	if err := DB.First(&tpl, id).Error; err != nil {
		return nil, err
	}
	return &tpl, nil
}

func ListReferralTemplateBindingsByUser(userID int, referralType string) ([]ReferralTemplateBindingView, error) {
	var bindings []ReferralTemplateBinding
	if err := DB.Where("user_id = ? AND referral_type = ?", userID, strings.TrimSpace(referralType)).
		Order(commonGroupCol+" ASC").
		Find(&bindings).Error; err != nil {
		return nil, err
	}

	views := make([]ReferralTemplateBindingView, 0, len(bindings))
	for _, binding := range bindings {
		tpl, err := GetReferralTemplateByID(binding.TemplateId)
		if err != nil {
			return nil, err
		}
		views = append(views, ReferralTemplateBindingView{Binding: binding, Template: *tpl})
	}
	return views, nil
}

func ResolveBindingInviteeShareDefault(binding ReferralTemplateBindingView, tpl ReferralTemplate) int {
	if binding.InviteeShareOverrideBps != nil {
		return NormalizeSubscriptionReferralRateBps(*binding.InviteeShareOverrideBps)
	}
	return NormalizeSubscriptionReferralRateBps(tpl.InviteeShareDefaultBps)
}

func ListReferralTemplates(referralType string, group string) ([]ReferralTemplate, error) {
	query := DB.Model(&ReferralTemplate{}).Order("referral_type ASC, "+commonGroupCol+" ASC, name ASC")
	if trimmedType := strings.TrimSpace(referralType); trimmedType != "" {
		query = query.Where("referral_type = ?", trimmedType)
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

func UpdateReferralTemplate(id int, req dto.ReferralTemplateUpsertRequest, operatorID int) (*ReferralTemplate, error) {
	tpl, err := GetReferralTemplateByID(id)
	if err != nil {
		return nil, err
	}
	tpl.ReferralType = req.ReferralType
	tpl.Group = req.Group
	tpl.Name = req.Name
	tpl.LevelType = req.LevelType
	tpl.Enabled = req.Enabled
	tpl.DirectCapBps = req.DirectCapBps
	tpl.TeamCapBps = req.TeamCapBps
	tpl.TeamDecayRatio = req.TeamDecayRatio
	tpl.TeamMaxDepth = req.TeamMaxDepth
	tpl.InviteeShareDefaultBps = req.InviteeShareDefaultBps
	tpl.UpdatedBy = operatorID
	if err := tpl.Validate(); err != nil {
		return nil, err
	}
	return tpl, DB.Save(tpl).Error
}

func DeleteReferralTemplate(id int) error {
	return DB.Delete(&ReferralTemplate{}, id).Error
}

func CreateReferralTemplate(tpl *ReferralTemplate) error {
	if tpl == nil {
		return errors.New("template is required")
	}
	if err := tpl.Validate(); err != nil {
		return err
	}
	return DB.Create(tpl).Error
}

func UpsertReferralTemplateBinding(userID int, req dto.ReferralTemplateBindingUpsertRequest, operatorID int) (*ReferralTemplateBinding, error) {
	tpl, err := GetReferralTemplateByID(req.TemplateID)
	if err != nil {
		return nil, err
	}
	binding := &ReferralTemplateBinding{
		UserId:                  userID,
		ReferralType:            req.ReferralType,
		Group:                   req.Group,
		TemplateId:              req.TemplateID,
		InviteeShareOverrideBps: req.InviteeShareOverrideBps,
		CreatedBy:               operatorID,
		UpdatedBy:               operatorID,
	}
	if err := binding.ValidateAgainstTemplate(tpl); err != nil {
		return nil, err
	}
	return binding, DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "referral_type"}, {Name: "group"}},
		DoUpdates: clause.Assignments(map[string]any{
			"template_id":                binding.TemplateId,
			"invitee_share_override_bps": binding.InviteeShareOverrideBps,
			"updated_by":                 operatorID,
			"updated_at":                 common.GetTimestamp(),
		}),
	}).Create(binding).Error
}

func ListReferralEngineRoutes(referralType string) ([]ReferralEngineRoute, error) {
	query := DB.Model(&ReferralEngineRoute{}).Order("referral_type ASC, "+commonGroupCol+" ASC")
	if trimmedType := strings.TrimSpace(referralType); trimmedType != "" {
		query = query.Where("referral_type = ?", trimmedType)
	}
	var routes []ReferralEngineRoute
	if err := query.Find(&routes).Error; err != nil {
		return nil, err
	}
	return routes, nil
}

func UpsertReferralEngineRoute(req dto.ReferralEngineRouteUpsertRequest, operatorID int) (*ReferralEngineRoute, error) {
	route := &ReferralEngineRoute{
		ReferralType: strings.TrimSpace(req.ReferralType),
		Group:        strings.TrimSpace(req.Group),
		EngineMode:   strings.TrimSpace(req.EngineMode),
		CreatedBy:    operatorID,
		UpdatedBy:    operatorID,
	}
	return route, DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "referral_type"}, {Name: "group"}},
		DoUpdates: clause.Assignments(map[string]any{
			"engine_mode": req.EngineMode,
			"updated_by":  operatorID,
			"updated_at":  common.GetTimestamp(),
		}),
	}).Create(route).Error
}

type LegacySubscriptionReferralSeedRows struct {
	Group             string                           `json:"group"`
	OverrideSeeds     []SubscriptionReferralOverride   `json:"override_seeds"`
	InviteeRateSeeds  map[int]int                      `json:"invitee_rate_seeds"`
	InviteeOverrideSeeds []SubscriptionReferralInviteeOverride `json:"invitee_override_seeds"`
}

func ListLegacySubscriptionReferralSeedRows(group string) (*LegacySubscriptionReferralSeedRows, error) {
	trimmedGroup := strings.TrimSpace(group)
	seeds := &LegacySubscriptionReferralSeedRows{
		Group:               trimmedGroup,
		InviteeRateSeeds:    map[int]int{},
		InviteeOverrideSeeds: []SubscriptionReferralInviteeOverride{},
	}
	if err := DB.Where(commonGroupCol+" = ?", trimmedGroup).Find(&seeds.OverrideSeeds).Error; err != nil {
		return nil, err
	}
	if err := DB.Where(commonGroupCol+" = ?", trimmedGroup).Find(&seeds.InviteeOverrideSeeds).Error; err != nil {
		return nil, err
	}
	var users []User
	if err := DB.Find(&users).Error; err != nil {
		return nil, err
	}
	for _, user := range users {
		if rate := user.GetSetting().SubscriptionReferralInviteeRateBpsByGroup[trimmedGroup]; rate > 0 {
			seeds.InviteeRateSeeds[user.Id] = rate
		}
	}
	return seeds, nil
}

type ReferralTemplateBindingView struct {
	Binding  ReferralTemplateBinding `json:"binding"`
	Template ReferralTemplate        `json:"template"`
}
```

- [ ] **Step 4: Run the model tests to verify repository behavior**

Run: `go test ./model -run 'TestUpsertReferralInviteeShareOverrideRequiresActiveBinding|TestListLegacySubscriptionReferralSeedRowsReturnsOverrideAndInviteeSeeds'`
Expected: PASS

- [ ] **Step 5: Commit the repository/helper layer**

```bash
git add model/referral_template.go model/referral_template_binding.go model/referral_invitee_share_override.go model/referral_seed.go model/referral_template_test.go
git commit -m "feat: add referral template repository helpers"
```

### Task 3: Expose Admin CRUD and Seed Inspection APIs

**Files:**
- Create: `dto/referral_admin.go`
- Create: `controller/referral_admin.go`
- Create: `controller/referral_admin_test.go`
- Modify: `router/api-router.go`
- Test: `controller/referral_admin_test.go`

- [ ] **Step 1: Write the failing admin API tests**

```go
func TestAdminCreateReferralTemplate(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	admin := seedSubscriptionReferralControllerUser(t, "referral-root-admin", 0, dto.UserSetting{})
	admin.Role = common.RoleRootUser
	if err := model.DB.Save(admin).Error; err != nil {
		t.Fatalf("save root admin: %v", err)
	}

	body := `{
		"referral_type":"subscription_referral",
		"group":"vip",
		"name":"vip-direct",
		"level_type":"direct",
		"enabled":true,
		"direct_cap_bps":1000,
		"team_cap_bps":2500,
		"team_decay_ratio":0.5,
		"team_max_depth":3,
		"invitee_share_default_bps":1200
	}`

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/referral/templates", strings.NewReader(body), admin.Id)
	AdminCreateReferralTemplate(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
}

func TestAdminListLegacySubscriptionReferralSeeds(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	admin := seedSubscriptionReferralControllerUser(t, "referral-root-admin-seed", 0, dto.UserSetting{})
	admin.Role = common.RoleRootUser
	if err := model.DB.Save(admin).Error; err != nil {
		t.Fatalf("save root admin: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/referral/legacy-seeds/subscription?group=vip", nil, admin.Id)
	AdminListLegacySubscriptionReferralSeeds(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
}
```

- [ ] **Step 2: Run the controller tests to verify they fail**

Run: `go test ./controller -run 'TestAdminCreateReferralTemplate|TestAdminListLegacySubscriptionReferralSeeds'`
Expected: FAIL with `undefined: AdminCreateReferralTemplate` and `undefined: AdminListLegacySubscriptionReferralSeeds`.

- [ ] **Step 3: Implement the admin DTOs, controllers, and route wiring**

```go
package dto

type ReferralTemplateUpsertRequest struct {
	ReferralType           string  `json:"referral_type"`
	Group                  string  `json:"group"`
	Name                   string  `json:"name"`
	LevelType              string  `json:"level_type"`
	Enabled                bool    `json:"enabled"`
	DirectCapBps           int     `json:"direct_cap_bps"`
	TeamCapBps             int     `json:"team_cap_bps"`
	TeamDecayRatio         float64 `json:"team_decay_ratio"`
	TeamMaxDepth           int     `json:"team_max_depth"`
	InviteeShareDefaultBps int     `json:"invitee_share_default_bps"`
}

type ReferralTemplateBindingUpsertRequest struct {
	ReferralType            string `json:"referral_type"`
	Group                   string `json:"group"`
	TemplateID              int    `json:"template_id"`
	InviteeShareOverrideBps *int   `json:"invitee_share_override_bps"`
}

type ReferralEngineRouteUpsertRequest struct {
	ReferralType string `json:"referral_type"`
	Group        string `json:"group"`
	EngineMode   string `json:"engine_mode"`
}
```

```go
package controller

func AdminCreateReferralTemplate(c *gin.Context) {
	var req dto.ReferralTemplateUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	tpl := &model.ReferralTemplate{
		ReferralType:           req.ReferralType,
		Group:                  req.Group,
		Name:                   req.Name,
		LevelType:              req.LevelType,
		Enabled:                req.Enabled,
		DirectCapBps:           req.DirectCapBps,
		TeamCapBps:             req.TeamCapBps,
		TeamDecayRatio:         req.TeamDecayRatio,
		TeamMaxDepth:           req.TeamMaxDepth,
		InviteeShareDefaultBps: req.InviteeShareDefaultBps,
		CreatedBy:              c.GetInt("id"),
		UpdatedBy:              c.GetInt("id"),
	}

	if err := model.CreateReferralTemplate(tpl); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, tpl)
}

func AdminListReferralTemplates(c *gin.Context) {
	templates, err := model.ListReferralTemplates(strings.TrimSpace(c.Query("referral_type")), strings.TrimSpace(c.Query("group")))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": templates})
}

func AdminUpdateReferralTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid template id")
		return
	}
	var req dto.ReferralTemplateUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	updated, err := model.UpdateReferralTemplate(id, req, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, updated)
}

func AdminDeleteReferralTemplate(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorMsg(c, "invalid template id")
		return
	}
	if err := model.DeleteReferralTemplate(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"id": id})
}

func AdminListReferralTemplateBindingsByUser(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil || userID <= 0 {
		common.ApiErrorMsg(c, "invalid user id")
		return
	}
	items, err := model.ListReferralTemplateBindingsByUser(userID, strings.TrimSpace(c.Query("referral_type")))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": items})
}

func AdminUpsertReferralTemplateBindingForUser(c *gin.Context) {
	userID, err := strconv.Atoi(c.Param("id"))
	if err != nil || userID <= 0 {
		common.ApiErrorMsg(c, "invalid user id")
		return
	}
	var req dto.ReferralTemplateBindingUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	binding, err := model.UpsertReferralTemplateBinding(userID, req, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, binding)
}

func AdminListReferralEngineRoutes(c *gin.Context) {
	items, err := model.ListReferralEngineRoutes(strings.TrimSpace(c.Query("referral_type")))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": items})
}

func AdminUpsertReferralEngineRoute(c *gin.Context) {
	var req dto.ReferralEngineRouteUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	route, err := model.UpsertReferralEngineRoute(req, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, route)
}

func AdminListLegacySubscriptionReferralSeeds(c *gin.Context) {
	group := strings.TrimSpace(c.Query("group"))
	seeds, err := model.ListLegacySubscriptionReferralSeedRows(group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, seeds)
}
```

```go
// router/api-router.go
referralAdminRoute := apiRouter.Group("/referral")
referralAdminRoute.Use(middleware.RootAuth())
{
	referralAdminRoute.GET("/templates", controller.AdminListReferralTemplates)
	referralAdminRoute.POST("/templates", controller.AdminCreateReferralTemplate)
	referralAdminRoute.PUT("/templates/:id", controller.AdminUpdateReferralTemplate)
	referralAdminRoute.DELETE("/templates/:id", controller.AdminDeleteReferralTemplate)
	referralAdminRoute.GET("/bindings/users/:id", controller.AdminListReferralTemplateBindingsByUser)
	referralAdminRoute.PUT("/bindings/users/:id", controller.AdminUpsertReferralTemplateBindingForUser)
	referralAdminRoute.GET("/engine-routes", controller.AdminListReferralEngineRoutes)
	referralAdminRoute.PUT("/engine-routes", controller.AdminUpsertReferralEngineRoute)
	referralAdminRoute.GET("/legacy-seeds/subscription", controller.AdminListLegacySubscriptionReferralSeeds)
}
```

- [ ] **Step 4: Run the targeted API tests**

Run: `go test ./controller -run 'TestAdminCreateReferralTemplate|TestAdminListLegacySubscriptionReferralSeeds'`
Expected: PASS

- [ ] **Step 5: Commit the admin APIs**

```bash
git add dto/referral_admin.go controller/referral_admin.go controller/referral_admin_test.go router/api-router.go
git commit -m "feat: add referral template admin apis"
```
