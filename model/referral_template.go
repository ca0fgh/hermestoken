package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	ReferralTypeSubscription = "subscription_referral"

	ReferralLevelTypeDirect = "direct"
	ReferralLevelTypeTeam   = "team"
)

type ReferralTemplate struct {
	Id                     int    `json:"id"`
	BundleKey              string `json:"bundle_key" gorm:"type:varchar(64);not null;default:'';index"`
	ReferralType           string `json:"referral_type" gorm:"type:varchar(64);not null;index:idx_referral_template_scope_name,priority:1;uniqueIndex:uk_referral_template_scope_name,priority:1"`
	Group                  string `json:"group" gorm:"type:varchar(64);not null;default:'';index:idx_referral_template_scope_name,priority:2;uniqueIndex:uk_referral_template_scope_name,priority:2"`
	Name                   string `json:"name" gorm:"type:varchar(128);not null;index:idx_referral_template_scope_name,priority:3;uniqueIndex:uk_referral_template_scope_name,priority:3"`
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

func (t *ReferralTemplate) normalize() {
	t.ReferralType = strings.TrimSpace(t.ReferralType)
	t.BundleKey = strings.TrimSpace(t.BundleKey)
	t.Group = strings.TrimSpace(t.Group)
	t.Name = strings.TrimSpace(t.Name)
	t.LevelType = strings.TrimSpace(t.LevelType)
	t.InviteeShareDefaultBps = NormalizeSubscriptionReferralRateBps(t.InviteeShareDefaultBps)
	if t.ReferralType == ReferralTypeSubscription {
		switch t.LevelType {
		case ReferralLevelTypeDirect:
			t.TeamCapBps = 0
		case ReferralLevelTypeTeam:
			t.DirectCapBps = 0
		}
	}
}

func (t *ReferralTemplate) Validate() error {
	t.normalize()
	if t.ReferralType == "" {
		return fmt.Errorf("referral type is required")
	}
	if t.Group == "" {
		return fmt.Errorf("group is required")
	}
	if t.Name == "" {
		return fmt.Errorf("name is required")
	}

	if t.ReferralType != ReferralTypeSubscription {
		return nil
	}

	if t.LevelType != ReferralLevelTypeDirect && t.LevelType != ReferralLevelTypeTeam {
		return fmt.Errorf("invalid subscription level type: %s", t.LevelType)
	}
	switch t.LevelType {
	case ReferralLevelTypeDirect:
		if t.DirectCapBps < 0 || t.DirectCapBps > SubscriptionReferralMaxRateBps {
			return fmt.Errorf("invalid subscription cap bps")
		}
	case ReferralLevelTypeTeam:
		if t.TeamCapBps < 0 || t.TeamCapBps > SubscriptionReferralMaxRateBps {
			return fmt.Errorf("invalid subscription cap bps")
		}
	}
	return nil
}

func (t *ReferralTemplate) validateUniqueName(tx *gorm.DB) error {
	if tx == nil {
		tx = DB
	}
	if tx == nil {
		return errors.New("database is not initialized")
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

func (t *ReferralTemplate) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	t.CreatedAt = now
	t.UpdatedAt = now
	if strings.TrimSpace(t.BundleKey) == "" {
		t.BundleKey = newReferralTemplateBundleKey()
	}
	if err := t.Validate(); err != nil {
		return err
	}
	return t.validateUniqueName(tx)
}

func (t *ReferralTemplate) BeforeUpdate(tx *gorm.DB) error {
	t.UpdatedAt = common.GetTimestamp()
	if err := t.Validate(); err != nil {
		return err
	}
	return t.validateUniqueName(tx)
}

func normalizeReferralTemplatePersistenceError(err error) error {
	if err == nil {
		return nil
	}
	lowerError := strings.ToLower(err.Error())
	if strings.Contains(lowerError, "uk_referral_template_scope_name") ||
		strings.Contains(lowerError, "uk_referral_template_name") ||
		strings.Contains(lowerError, "idx_referral_templates_name") ||
		(strings.Contains(lowerError, "referral_templates.referral_type") &&
			strings.Contains(lowerError, "referral_templates.group") &&
			strings.Contains(lowerError, "referral_templates.name")) ||
		strings.Contains(lowerError, "referral_templates.name") {
		return fmt.Errorf("template name already exists")
	}
	return err
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

func referralTemplateBundleKeyForRow(template ReferralTemplate) string {
	if trimmed := strings.TrimSpace(template.BundleKey); trimmed != "" {
		return trimmed
	}
	return fmt.Sprintf("template:%d", template.Id)
}

func isSyntheticReferralTemplateBundleKey(bundleKey string) bool {
	trimmed := strings.TrimSpace(bundleKey)
	return trimmed == "" || strings.HasPrefix(trimmed, "template:")
}

func GetReferralTemplateByID(id int) (*ReferralTemplate, error) {
	if id <= 0 {
		return nil, gorm.ErrRecordNotFound
	}

	var template ReferralTemplate
	if err := DB.First(&template, id).Error; err != nil {
		return nil, err
	}
	return &template, nil
}

func CreateReferralTemplate(template *ReferralTemplate) error {
	if template == nil {
		return fmt.Errorf("template is required")
	}
	return normalizeReferralTemplatePersistenceError(DB.Create(template).Error)
}

func UpdateReferralTemplate(template *ReferralTemplate) error {
	if template == nil {
		return fmt.Errorf("template is required")
	}
	return normalizeReferralTemplatePersistenceError(DB.Save(template).Error)
}

func DeleteReferralTemplate(id int) error {
	if id <= 0 {
		return gorm.ErrRecordNotFound
	}
	return DB.Delete(&ReferralTemplate{}, id).Error
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

func ListReferralTemplates(referralType string, group string) ([]ReferralTemplate, error) {
	return ListReferralTemplateRows(referralType, group)
}

func ListReferralTemplateBundles(referralType string) ([]ReferralTemplateBundle, error) {
	rows, err := ListReferralTemplateRows(referralType, "")
	if err != nil {
		return nil, err
	}

	bundleOrder := make([]string, 0)
	bundleMap := make(map[string]*ReferralTemplateBundle, len(rows))
	for _, row := range rows {
		bundleKey := referralTemplateBundleKeyForRow(row)
		bundle, exists := bundleMap[bundleKey]
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
			bundleMap[bundleKey] = bundle
			bundleOrder = append(bundleOrder, bundleKey)
		}
		bundle.TemplateIDs = append(bundle.TemplateIDs, row.Id)
		bundle.Groups = append(bundle.Groups, row.Group)
		if bundle.CreatedAt == 0 || (row.CreatedAt != 0 && row.CreatedAt < bundle.CreatedAt) {
			bundle.CreatedAt = row.CreatedAt
		}
		if row.UpdatedAt > bundle.UpdatedAt {
			bundle.UpdatedAt = row.UpdatedAt
		}
	}

	bundles := make([]ReferralTemplateBundle, 0, len(bundleOrder))
	for _, bundleKey := range bundleOrder {
		bundle := bundleMap[bundleKey]
		sort.Ints(bundle.TemplateIDs)
		bundle.Groups = normalizeReferralTemplateGroups(bundle.Groups)
		bundles = append(bundles, *bundle)
	}
	return bundles, nil
}

func CreateReferralTemplateBundle(input ReferralTemplateBundleUpsertInput, operatorID int) ([]ReferralTemplate, error) {
	if DB == nil {
		return nil, errors.New("database is not initialized")
	}

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
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func UpdateReferralTemplateBundleByTemplateID(templateID int, input ReferralTemplateBundleUpsertInput, operatorID int) ([]ReferralTemplate, error) {
	if DB == nil {
		return nil, errors.New("database is not initialized")
	}

	existing, err := GetReferralTemplateByID(templateID)
	if err != nil {
		return nil, err
	}

	bundleKey := referralTemplateBundleKeyForRow(*existing)
	groups := normalizeReferralTemplateGroups(input.Groups)
	if len(groups) == 0 {
		return nil, fmt.Errorf("at least one group is required")
	}
	if len(groups) > 1 && isSyntheticReferralTemplateBundleKey(existing.BundleKey) {
		bundleKey = newReferralTemplateBundleKey()
	}

	rows := make([]ReferralTemplate, 0, len(groups))
	err = DB.Transaction(func(tx *gorm.DB) error {
		var currentRows []ReferralTemplate
		if strings.TrimSpace(existing.BundleKey) != "" {
			if err := tx.Where("bundle_key = ?", existing.BundleKey).Order("id ASC").Find(&currentRows).Error; err != nil {
				return err
			}
		}
		if len(currentRows) == 0 {
			currentRows = []ReferralTemplate{*existing}
		}

		currentByGroup := make(map[string]ReferralTemplate, len(currentRows))
		for _, row := range currentRows {
			row.BundleKey = bundleKey
			currentByGroup[row.Group] = row
		}

		trimmedReferralType := strings.TrimSpace(input.ReferralType)
		trimmedName := strings.TrimSpace(input.Name)
		trimmedLevelType := strings.TrimSpace(input.LevelType)
		for _, group := range groups {
			row, exists := currentByGroup[group]
			if !exists {
				row = ReferralTemplate{
					BundleKey: bundleKey,
					CreatedBy: operatorID,
				}
			}
			row.BundleKey = bundleKey
			row.ReferralType = trimmedReferralType
			row.Group = group
			row.Name = trimmedName
			row.LevelType = trimmedLevelType
			row.Enabled = input.Enabled
			row.DirectCapBps = input.DirectCapBps
			row.TeamCapBps = input.TeamCapBps
			row.InviteeShareDefaultBps = input.InviteeShareDefaultBps
			row.UpdatedBy = operatorID

			var saveErr error
			if row.Id > 0 {
				saveErr = tx.Save(&row).Error
			} else {
				saveErr = tx.Create(&row).Error
			}
			if err := normalizeReferralTemplatePersistenceError(saveErr); err != nil {
				return err
			}
			rows = append(rows, row)
			delete(currentByGroup, group)
		}

		staleIDs := make([]int, 0, len(currentByGroup))
		for _, row := range currentByGroup {
			if row.Id > 0 {
				staleIDs = append(staleIDs, row.Id)
			}
		}
		if len(staleIDs) > 0 {
			if err := deleteReferralTemplateBindingsByTemplateIDs(tx, staleIDs); err != nil {
				return err
			}
			if err := tx.Delete(&ReferralTemplate{}, staleIDs).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Group < rows[j].Group
	})
	return rows, nil
}

func DeleteReferralTemplateBundleByTemplateID(templateID int) error {
	if DB == nil {
		return errors.New("database is not initialized")
	}

	template, err := GetReferralTemplateByID(templateID)
	if err != nil {
		return err
	}

	bundleKey := strings.TrimSpace(template.BundleKey)
	if bundleKey == "" || strings.HasPrefix(bundleKey, "template:") {
		return DB.Transaction(func(tx *gorm.DB) error {
			if err := deleteReferralTemplateBindingsByTemplateIDs(tx, []int{template.Id}); err != nil {
				return err
			}
			return tx.Delete(&ReferralTemplate{}, template.Id).Error
		})
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var bundleRows []ReferralTemplate
		if err := tx.Where("bundle_key = ?", bundleKey).Find(&bundleRows).Error; err != nil {
			return err
		}
		templateIDs := make([]int, 0, len(bundleRows))
		for _, row := range bundleRows {
			if row.Id > 0 {
				templateIDs = append(templateIDs, row.Id)
			}
		}
		if err := deleteReferralTemplateBindingsByTemplateIDs(tx, templateIDs); err != nil {
			return err
		}
		return tx.Where("bundle_key = ?", bundleKey).Delete(&ReferralTemplate{}).Error
	})
}

func BackfillReferralTemplateBundleKeys() error {
	if DB == nil {
		return errors.New("database is not initialized")
	}

	var templates []ReferralTemplate
	if err := DB.Where("TRIM(COALESCE(bundle_key, '')) = ''").Order("id ASC").Find(&templates).Error; err != nil {
		return err
	}
	for _, template := range templates {
		bundleKey := fmt.Sprintf("template:%d", template.Id)
		if err := DB.Model(&ReferralTemplate{}).
			Where("id = ? AND TRIM(COALESCE(bundle_key, '')) = ''", template.Id).
			UpdateColumn("bundle_key", bundleKey).Error; err != nil {
			return err
		}
	}
	return nil
}

func ensureReferralTemplateSchema() error {
	if DB == nil {
		return errors.New("database is not initialized")
	}

	const bundleKeyIndexName = "idx_referral_templates_bundle_key"
	legacyGlobalNameArtifacts := []string{
		"uk_referral_template_name",
		"idx_referral_templates_name",
	}

	if !DB.Migrator().HasColumn(&ReferralTemplate{}, "BundleKey") {
		if err := DB.Migrator().AddColumn(&ReferralTemplate{}, "BundleKey"); err != nil {
			return err
		}
	}
	if !DB.Migrator().HasIndex(&ReferralTemplate{}, bundleKeyIndexName) {
		if err := DB.Migrator().CreateIndex(&ReferralTemplate{}, "BundleKey"); err != nil {
			return err
		}
	}
	if common.UsingPostgreSQL {
		for _, legacyConstraintName := range legacyGlobalNameArtifacts {
			if DB.Migrator().HasConstraint(&ReferralTemplate{}, legacyConstraintName) {
				if err := DB.Migrator().DropConstraint(&ReferralTemplate{}, legacyConstraintName); err != nil {
					return err
				}
			}
		}
	}
	for _, legacyIndexName := range legacyGlobalNameArtifacts {
		if DB.Migrator().HasIndex(&ReferralTemplate{}, legacyIndexName) {
			if err := DB.Migrator().DropIndex(&ReferralTemplate{}, legacyIndexName); err != nil {
				return err
			}
		}
	}
	if !DB.Migrator().HasIndex(&ReferralTemplate{}, "idx_referral_template_scope_name") {
		if err := DB.Migrator().CreateIndex(&ReferralTemplate{}, "idx_referral_template_scope_name"); err != nil {
			return err
		}
	}
	if !DB.Migrator().HasIndex(&ReferralTemplate{}, "uk_referral_template_scope_name") {
		if err := DB.Migrator().CreateIndex(&ReferralTemplate{}, "uk_referral_template_scope_name"); err != nil {
			return err
		}
	}
	return BackfillReferralTemplateBundleKeys()
}

func deleteReferralTemplateBindingsByTemplateIDs(tx *gorm.DB, templateIDs []int) error {
	if tx == nil {
		return errors.New("transaction is required")
	}
	if len(templateIDs) == 0 {
		return nil
	}
	return tx.Where("template_id IN ?", templateIDs).Delete(&ReferralTemplateBinding{}).Error
}
