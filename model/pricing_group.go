package model

import (
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
)

const (
	// Pricing group lifecycle values. Follow the existing integer-status style used in other models.
	PricingGroupStatusActive     = 1
	PricingGroupStatusDeprecated = 2
	PricingGroupStatusArchived   = 3
)

const (
	// Visibility rule actions mirror the planned add/remove semantics for per-subject group lists.
	PricingGroupVisibilityActionAdd    = "add"
	PricingGroupVisibilityActionRemove = "remove"
)

type PricingGroup struct {
	Id             int            `json:"id"`
	GroupKey       string         `json:"group_key" gorm:"type:varchar(64);not null;uniqueIndex"`
	DisplayName    string         `json:"display_name" gorm:"type:varchar(128);not null;default:''"`
	Description    string         `json:"description" gorm:"type:text"`
	BillingRatio   float64        `json:"billing_ratio" gorm:"type:decimal(12,6);not null;default:1"`
	UserSelectable bool           `json:"user_selectable" gorm:"not null;default:false"`
	Status         int            `json:"status" gorm:"type:int;not null;default:1;index"` // active/deprecated/archived
	SortOrder      int            `json:"sort_order" gorm:"type:int;not null;default:0"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

type PricingGroupAlias struct {
	Id       int    `json:"id"`
	AliasKey string `json:"alias_key" gorm:"type:varchar(64);not null;uniqueIndex"`
	GroupId  int    `json:"group_id" gorm:"not null;index"`
	Reason   string `json:"reason" gorm:"type:varchar(255);not null;default:''"`
}

type PricingGroupRatioOverride struct {
	Id            int     `json:"id"`
	SourceGroupId int     `json:"source_group_id" gorm:"not null;uniqueIndex:uk_pricing_group_ratio_overrides,priority:1"`
	TargetGroupId int     `json:"target_group_id" gorm:"not null;uniqueIndex:uk_pricing_group_ratio_overrides,priority:2"`
	Ratio         float64 `json:"ratio" gorm:"type:decimal(12,6);not null;default:1"`
}

type PricingGroupVisibilityRule struct {
	Id                  int    `json:"id"`
	SubjectGroupId      int    `json:"subject_group_id" gorm:"not null;index"`
	Action              string `json:"action" gorm:"type:varchar(32);not null;default:'add';index"` // add/remove
	TargetGroupId       int    `json:"target_group_id" gorm:"not null;index"`
	DescriptionOverride string `json:"description_override" gorm:"type:varchar(255);not null;default:''"`
	SortOrder           int    `json:"sort_order" gorm:"type:int;not null;default:0"`
}

type PricingGroupAutoPriority struct {
	Id       int `json:"id"`
	GroupId  int `json:"group_id" gorm:"not null;index"`
	Priority int `json:"priority" gorm:"type:int;not null;default:0;uniqueIndex:uk_pricing_group_auto_priorities_priority"`
}

type pricingGroupSeedSpec struct {
	GroupKey       string
	DisplayName    string
	BillingRatio   float64
	UserSelectable bool
	SortOrder      int
}

func SeedPricingGroupsFromLegacyOptions(
	groupRatioJSON string,
	userUsableGroupsJSON string,
	groupGroupRatioJSON string,
	autoGroupsJSON string,
	groupSpecialUsableGroupJSON string,
) error {
	if DB == nil {
		return fmt.Errorf("pricing group store unavailable")
	}

	groupRatios, err := parsePricingGroupRatioJSON(groupRatioJSON)
	if err != nil {
		return err
	}
	userUsableGroups, err := parsePricingGroupStringMapJSON(userUsableGroupsJSON)
	if err != nil {
		return err
	}
	groupGroupRatios, err := parsePricingGroupNestedRatioJSON(groupGroupRatioJSON)
	if err != nil {
		return err
	}
	autoGroups, err := parsePricingGroupStringListJSON(autoGroupsJSON)
	if err != nil {
		return err
	}
	groupSpecialUsableGroups, err := parsePricingGroupNestedStringJSON(groupSpecialUsableGroupJSON)
	if err != nil {
		return err
	}

	specs := buildPricingGroupSeedSpecs(
		groupRatios,
		userUsableGroups,
		groupGroupRatios,
		autoGroups,
		groupSpecialUsableGroups,
	)

	return DB.Transaction(func(tx *gorm.DB) error {
		keyToID := make(map[string]int, len(specs))
		seededKeys := make([]string, 0, len(specs))

		for _, spec := range specs {
			group := PricingGroup{}
			err := tx.Where("group_key = ?", spec.GroupKey).First(&group).Error
			if err != nil && err != gorm.ErrRecordNotFound {
				return err
			}

			group.GroupKey = spec.GroupKey
			group.DisplayName = spec.DisplayName
			group.BillingRatio = spec.BillingRatio
			group.UserSelectable = spec.UserSelectable
			group.Status = PricingGroupStatusActive
			group.SortOrder = spec.SortOrder
			if group.Description == "" {
				group.Description = spec.DisplayName
			}

			if group.Id == 0 {
				if err := tx.Create(&group).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Save(&group).Error; err != nil {
					return err
				}
			}

			keyToID[spec.GroupKey] = group.Id
			seededKeys = append(seededKeys, spec.GroupKey)
		}

		if len(seededKeys) > 0 {
			if err := tx.Model(&PricingGroup{}).
				Where("group_key NOT IN ?", seededKeys).
				Update("status", PricingGroupStatusArchived).Error; err != nil {
				return err
			}
		}

		if err := tx.Where("1 = 1").Delete(&PricingGroupRatioOverride{}).Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&PricingGroupVisibilityRule{}).Error; err != nil {
			return err
		}
		if err := tx.Where("1 = 1").Delete(&PricingGroupAutoPriority{}).Error; err != nil {
			return err
		}

		for sourceGroup, targetGroups := range groupGroupRatios {
			sourceID, ok := keyToID[sourceGroup]
			if !ok {
				continue
			}
			for targetGroup, ratio := range targetGroups {
				targetID, ok := keyToID[targetGroup]
				if !ok {
					continue
				}
				override := PricingGroupRatioOverride{
					SourceGroupId: sourceID,
					TargetGroupId: targetID,
					Ratio:         ratio,
				}
				if err := tx.Create(&override).Error; err != nil {
					return err
				}
			}
		}

		for subjectGroup, entries := range groupSpecialUsableGroups {
			subjectID, ok := keyToID[subjectGroup]
			if !ok {
				continue
			}
			sortOrder := 0
			entryKeys := make([]string, 0, len(entries))
			for rawTarget := range entries {
				entryKeys = append(entryKeys, rawTarget)
			}
			sort.Strings(entryKeys)
			for _, rawTarget := range entryKeys {
				targetGroup, action := normalizePricingGroupVisibilityTarget(rawTarget)
				targetID, ok := keyToID[targetGroup]
				if !ok {
					continue
				}
				rule := PricingGroupVisibilityRule{
					SubjectGroupId:      subjectID,
					Action:              action,
					TargetGroupId:       targetID,
					DescriptionOverride: strings.TrimSpace(entries[rawTarget]),
					SortOrder:           sortOrder,
				}
				sortOrder++
				if err := tx.Create(&rule).Error; err != nil {
					return err
				}
			}
		}

		for idx, groupKey := range autoGroups {
			groupID, ok := keyToID[groupKey]
			if !ok {
				continue
			}
			priority := PricingGroupAutoPriority{
				GroupId:  groupID,
				Priority: idx,
			}
			if err := tx.Create(&priority).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func SyncCanonicalPricingGroupRulesFromCurrentOptions() error {
	if DB == nil || !DB.Migrator().HasTable(&PricingGroup{}) {
		return nil
	}
	return SeedPricingGroupsFromLegacyOptions(
		ratio_setting.GroupRatio2JSONString(),
		setting.UserUsableGroups2JSONString(),
		ratio_setting.GroupGroupRatio2JSONString(),
		setting.AutoGroups2JsonString(),
		ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.MarshalJSONString(),
	)
}

func LoadEffectivePricingGroups() ([]PricingGroup, error) {
	if DB == nil || !DB.Migrator().HasTable(&PricingGroup{}) {
		return buildPricingGroupsFromLegacySettings(), nil
	}

	var groups []PricingGroup
	if err := DB.Where("status = ?", PricingGroupStatusActive).
		Order("sort_order asc").
		Order("id asc").
		Find(&groups).Error; err != nil {
		return nil, err
	}
	if len(groups) > 0 {
		return groups, nil
	}
	return buildPricingGroupsFromLegacySettings(), nil
}

func LoadEffectivePricingGroupRatios() (map[string]float64, error) {
	groups, err := LoadEffectivePricingGroups()
	if err != nil {
		return nil, err
	}

	ratios := make(map[string]float64, len(groups))
	for _, group := range groups {
		if strings.TrimSpace(group.GroupKey) == "" {
			continue
		}
		ratios[group.GroupKey] = group.BillingRatio
	}
	return ratios, nil
}

func LoadEffectiveUserUsableGroups() (map[string]string, error) {
	groups, err := LoadEffectivePricingGroups()
	if err != nil {
		return nil, err
	}

	usableGroups := make(map[string]string)
	for _, group := range groups {
		if !group.UserSelectable {
			continue
		}
		usableGroups[group.GroupKey] = pricingGroupDisplayText(group)
	}
	return usableGroups, nil
}

func LoadEffectivePricingGroupRatioOverrides() (map[string]map[string]float64, error) {
	fallback := cloneNestedFloatMap(ratio_setting.GetGroupRatioSetting().GroupGroupRatio.ReadAll())
	if DB == nil || !DB.Migrator().HasTable(&PricingGroupRatioOverride{}) {
		return fallback, nil
	}

	var rows []PricingGroupRatioOverride
	if err := DB.Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return fallback, nil
	}

	keyByID, err := loadPricingGroupKeyByIDMap()
	if err != nil {
		return nil, err
	}

	overrides := make(map[string]map[string]float64)
	for _, row := range rows {
		sourceKey := keyByID[row.SourceGroupId]
		targetKey := keyByID[row.TargetGroupId]
		if sourceKey == "" || targetKey == "" {
			continue
		}
		if _, ok := overrides[sourceKey]; !ok {
			overrides[sourceKey] = make(map[string]float64)
		}
		overrides[sourceKey][targetKey] = row.Ratio
	}
	return overrides, nil
}

func LoadEffectivePricingGroupVisibilityRules() (map[string]map[string]string, error) {
	fallback := cloneNestedStringMap(ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.ReadAll())
	if DB == nil || !DB.Migrator().HasTable(&PricingGroupVisibilityRule{}) {
		return fallback, nil
	}

	var rows []PricingGroupVisibilityRule
	if err := DB.Order("sort_order asc").Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return fallback, nil
	}

	keyByID, err := loadPricingGroupKeyByIDMap()
	if err != nil {
		return nil, err
	}
	nameByID, err := loadPricingGroupDisplayNameByIDMap()
	if err != nil {
		return nil, err
	}

	rules := make(map[string]map[string]string)
	for _, row := range rows {
		subjectKey := keyByID[row.SubjectGroupId]
		targetKey := keyByID[row.TargetGroupId]
		if subjectKey == "" || targetKey == "" {
			continue
		}
		if _, ok := rules[subjectKey]; !ok {
			rules[subjectKey] = make(map[string]string)
		}
		entryKey := targetKey
		if row.Action == PricingGroupVisibilityActionRemove {
			entryKey = "-:" + targetKey
		} else {
			entryKey = "+:" + targetKey
		}
		description := strings.TrimSpace(row.DescriptionOverride)
		if description == "" {
			description = nameByID[row.TargetGroupId]
		}
		rules[subjectKey][entryKey] = description
	}
	return rules, nil
}

func LoadEffectiveAutoGroupKeys() ([]string, error) {
	fallback := append([]string(nil), setting.GetAutoGroups()...)
	if DB == nil || !DB.Migrator().HasTable(&PricingGroupAutoPriority{}) {
		return fallback, nil
	}

	var rows []PricingGroupAutoPriority
	if err := DB.Order("priority asc").Order("id asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return fallback, nil
	}

	keyByID, err := loadPricingGroupKeyByIDMap()
	if err != nil {
		return nil, err
	}

	groupKeys := make([]string, 0, len(rows))
	for _, row := range rows {
		if key := keyByID[row.GroupId]; key != "" {
			groupKeys = append(groupKeys, key)
		}
	}
	return groupKeys, nil
}

func buildPricingGroupsFromLegacySettings() []PricingGroup {
	specs := buildPricingGroupSeedSpecs(
		ratio_setting.GetGroupRatioCopy(),
		setting.GetUserUsableGroupsCopy(),
		ratio_setting.GetGroupRatioSetting().GroupGroupRatio.ReadAll(),
		setting.GetAutoGroups(),
		ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.ReadAll(),
	)

	groups := make([]PricingGroup, 0, len(specs))
	for _, spec := range specs {
		groups = append(groups, PricingGroup{
			GroupKey:       spec.GroupKey,
			DisplayName:    spec.DisplayName,
			BillingRatio:   spec.BillingRatio,
			UserSelectable: spec.UserSelectable,
			Status:         PricingGroupStatusActive,
			SortOrder:      spec.SortOrder,
			Description:    spec.DisplayName,
		})
	}
	return groups
}

func buildPricingGroupSeedSpecs(
	groupRatios map[string]float64,
	userUsableGroups map[string]string,
	groupGroupRatios map[string]map[string]float64,
	autoGroups []string,
	groupSpecialUsableGroups map[string]map[string]string,
) []pricingGroupSeedSpec {
	specByKey := make(map[string]*pricingGroupSeedSpec)
	autoPriority := make(map[string]int, len(autoGroups))
	for idx, groupKey := range autoGroups {
		if trimmed := strings.TrimSpace(groupKey); trimmed != "" {
			autoPriority[trimmed] = idx
		}
	}

	ensure := func(groupKey string) *pricingGroupSeedSpec {
		trimmed := strings.TrimSpace(groupKey)
		if trimmed == "" {
			return nil
		}
		if existing, ok := specByKey[trimmed]; ok {
			return existing
		}
		spec := &pricingGroupSeedSpec{
			GroupKey:     trimmed,
			DisplayName:  trimmed,
			BillingRatio: 1,
			SortOrder:    len(specByKey),
		}
		if priority, ok := autoPriority[trimmed]; ok {
			spec.SortOrder = priority
		}
		specByKey[trimmed] = spec
		return spec
	}

	for groupKey, ratio := range groupRatios {
		if spec := ensure(groupKey); spec != nil {
			spec.BillingRatio = ratio
		}
	}
	for groupKey, description := range userUsableGroups {
		if spec := ensure(groupKey); spec != nil {
			spec.UserSelectable = true
			if trimmedDescription := strings.TrimSpace(description); trimmedDescription != "" {
				spec.DisplayName = trimmedDescription
			}
		}
	}
	for subjectGroup, targetGroups := range groupGroupRatios {
		ensure(subjectGroup)
		for targetGroup := range targetGroups {
			ensure(targetGroup)
		}
	}
	for _, groupKey := range autoGroups {
		ensure(groupKey)
	}
	for subjectGroup, entries := range groupSpecialUsableGroups {
		ensure(subjectGroup)
		for rawTarget, description := range entries {
			targetGroup, action := normalizePricingGroupVisibilityTarget(rawTarget)
			spec := ensure(targetGroup)
			if spec == nil {
				continue
			}
			if action == PricingGroupVisibilityActionAdd {
				if trimmedDescription := strings.TrimSpace(description); trimmedDescription != "" {
					spec.DisplayName = trimmedDescription
				}
			}
		}
	}

	keys := make([]string, 0, len(specByKey))
	for groupKey := range specByKey {
		keys = append(keys, groupKey)
	}
	sort.Slice(keys, func(i, j int) bool {
		left := specByKey[keys[i]]
		right := specByKey[keys[j]]
		if left.SortOrder == right.SortOrder {
			return left.GroupKey < right.GroupKey
		}
		return left.SortOrder < right.SortOrder
	})

	specs := make([]pricingGroupSeedSpec, 0, len(keys))
	for index, groupKey := range keys {
		spec := specByKey[groupKey]
		spec.SortOrder = index
		specs = append(specs, *spec)
	}
	return specs
}

func parsePricingGroupRatioJSON(raw string) (map[string]float64, error) {
	parsed := make(map[string]float64)
	if strings.TrimSpace(raw) == "" {
		return parsed, nil
	}
	if err := common.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func parsePricingGroupStringMapJSON(raw string) (map[string]string, error) {
	parsed := make(map[string]string)
	if strings.TrimSpace(raw) == "" {
		return parsed, nil
	}
	if err := common.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func parsePricingGroupNestedRatioJSON(raw string) (map[string]map[string]float64, error) {
	parsed := make(map[string]map[string]float64)
	if strings.TrimSpace(raw) == "" {
		return parsed, nil
	}
	if err := common.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func parsePricingGroupNestedStringJSON(raw string) (map[string]map[string]string, error) {
	parsed := make(map[string]map[string]string)
	if strings.TrimSpace(raw) == "" {
		return parsed, nil
	}
	if err := common.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func parsePricingGroupStringListJSON(raw string) ([]string, error) {
	parsed := make([]string, 0)
	if strings.TrimSpace(raw) == "" {
		return parsed, nil
	}
	if err := common.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, err
	}
	return parsed, nil
}

func normalizePricingGroupVisibilityTarget(raw string) (string, string) {
	trimmed := strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(trimmed, "-:"):
		return strings.TrimSpace(trimmed[2:]), PricingGroupVisibilityActionRemove
	case strings.HasPrefix(trimmed, "+:"):
		return strings.TrimSpace(trimmed[2:]), PricingGroupVisibilityActionAdd
	default:
		return trimmed, PricingGroupVisibilityActionAdd
	}
}

func loadPricingGroupKeyByIDMap() (map[int]string, error) {
	var groups []PricingGroup
	if err := DB.Where("status = ?", PricingGroupStatusActive).Find(&groups).Error; err != nil {
		return nil, err
	}
	keyByID := make(map[int]string, len(groups))
	for _, group := range groups {
		keyByID[group.Id] = group.GroupKey
	}
	return keyByID, nil
}

func loadPricingGroupDisplayNameByIDMap() (map[int]string, error) {
	var groups []PricingGroup
	if err := DB.Where("status = ?", PricingGroupStatusActive).Find(&groups).Error; err != nil {
		return nil, err
	}
	nameByID := make(map[int]string, len(groups))
	for _, group := range groups {
		nameByID[group.Id] = pricingGroupDisplayText(group)
	}
	return nameByID, nil
}

func pricingGroupDisplayText(group PricingGroup) string {
	if trimmed := strings.TrimSpace(group.DisplayName); trimmed != "" {
		return trimmed
	}
	return group.GroupKey
}

func cloneNestedFloatMap(source map[string]map[string]float64) map[string]map[string]float64 {
	cloned := make(map[string]map[string]float64, len(source))
	for key, nested := range source {
		copiedNested := make(map[string]float64, len(nested))
		for nestedKey, value := range nested {
			copiedNested[nestedKey] = value
		}
		cloned[key] = copiedNested
	}
	return cloned
}

func cloneNestedStringMap(source map[string]map[string]string) map[string]map[string]string {
	cloned := make(map[string]map[string]string, len(source))
	for key, nested := range source {
		copiedNested := make(map[string]string, len(nested))
		for nestedKey, value := range nested {
			copiedNested[nestedKey] = value
		}
		cloned[key] = copiedNested
	}
	return cloned
}
