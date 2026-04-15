package service

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
)

type PricingGroupResolutionSource string

const (
	PricingGroupResolutionSourceCanonical PricingGroupResolutionSource = "canonical"
	PricingGroupResolutionSourceAlias     PricingGroupResolutionSource = "alias"
	PricingGroupResolutionSourceEmpty     PricingGroupResolutionSource = "empty"
	PricingGroupResolutionSourceUnknown   PricingGroupResolutionSource = "unknown"
)

type PricingGroupResolution struct {
	CanonicalKey string                       `json:"canonical_key"`
	Source       PricingGroupResolutionSource `json:"source"`
}

type PricingGroupLegacyReference struct {
	Scope string `json:"scope"`
	Value string `json:"value"`
}

type PricingGroupConsistencyReport struct {
	UnresolvedLegacyReferences []PricingGroupLegacyReference `json:"unresolved_legacy_references"`
}

func ResolveCanonicalPricingGroupKey(raw string) (PricingGroupResolution, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return PricingGroupResolution{Source: PricingGroupResolutionSourceEmpty}, nil
	}
	if model.DB == nil || !model.DB.Migrator().HasTable(&model.PricingGroup{}) {
		return PricingGroupResolution{}, fmt.Errorf("pricing group store unavailable")
	}

	var canonical model.PricingGroup
	if err := model.DB.Where("group_key = ?", trimmed).First(&canonical).Error; err == nil {
		return PricingGroupResolution{CanonicalKey: canonical.GroupKey, Source: PricingGroupResolutionSourceCanonical}, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return PricingGroupResolution{}, fmt.Errorf("query canonical pricing group %q: %w", trimmed, err)
	}

	if model.DB.Migrator().HasTable(&model.PricingGroupAlias{}) {
		var alias model.PricingGroupAlias
		if err := model.DB.Where("alias_key = ?", trimmed).First(&alias).Error; err == nil {
			if err := model.DB.Where("id = ?", alias.GroupId).First(&canonical).Error; err == nil {
				return PricingGroupResolution{CanonicalKey: canonical.GroupKey, Source: PricingGroupResolutionSourceAlias}, nil
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return PricingGroupResolution{}, fmt.Errorf("query alias target pricing group %q: %w", trimmed, err)
			}
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return PricingGroupResolution{}, fmt.Errorf("query pricing group alias %q: %w", trimmed, err)
		}
	}

	return PricingGroupResolution{Source: PricingGroupResolutionSourceUnknown}, fmt.Errorf("unknown pricing group: %s", trimmed)
}

func ListCanonicalPricingGroupKeysOrFallback() ([]string, error) {
	if model.DB == nil || !model.DB.Migrator().HasTable(&model.PricingGroup{}) {
		return listLegacyGroupRatioKeys(), nil
	}

	var count int64
	if err := model.DB.Model(&model.PricingGroup{}).Count(&count).Error; err != nil {
		return nil, fmt.Errorf("count pricing groups: %w", err)
	}
	if count == 0 {
		return listLegacyGroupRatioKeys(), nil
	}

	var groups []model.PricingGroup
	if err := model.DB.Order("sort_order asc").Order("group_key asc").Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("list pricing groups: %w", err)
	}

	keys := make([]string, 0, len(groups))
	for _, group := range groups {
		if group.GroupKey == "" {
			continue
		}
		keys = append(keys, group.GroupKey)
	}
	return keys, nil
}

func BuildPricingGroupConsistencyReport() (PricingGroupConsistencyReport, error) {
	unresolved := make([]PricingGroupLegacyReference, 0)
	seen := make(map[string]struct{})

	appendUnknown := func(scope string, raw string) error {
		value := normalizePricingGroupAuditValue(raw)
		if value == "" {
			return nil
		}
		resolution, err := ResolveCanonicalPricingGroupKey(value)
		if err == nil {
			return nil
		}
		if resolution.Source != PricingGroupResolutionSourceUnknown {
			return err
		}
		key := scope + "\x00" + value
		if _, ok := seen[key]; ok {
			return nil
		}
		seen[key] = struct{}{}
		unresolved = append(unresolved, PricingGroupLegacyReference{Scope: scope, Value: value})
		return nil
	}

	for groupKey := range ratio_setting.GetGroupRatioCopy() {
		if err := appendUnknown("group_ratio", groupKey); err != nil {
			return PricingGroupConsistencyReport{}, err
		}
	}
	for subjectGroup, targetGroups := range ratio_setting.GetGroupRatioSetting().GroupGroupRatio.ReadAll() {
		if err := appendUnknown("group_group_ratio.subject", subjectGroup); err != nil {
			return PricingGroupConsistencyReport{}, err
		}
		for targetGroup := range targetGroups {
			if err := appendUnknown("group_group_ratio.target", targetGroup); err != nil {
				return PricingGroupConsistencyReport{}, err
			}
		}
	}
	for groupKey := range setting.GetUserUsableGroupsCopy() {
		if err := appendUnknown("user_usable_groups", groupKey); err != nil {
			return PricingGroupConsistencyReport{}, err
		}
	}
	for subjectGroup, entries := range ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.ReadAll() {
		if err := appendUnknown("group_special_usable_group.subject", subjectGroup); err != nil {
			return PricingGroupConsistencyReport{}, err
		}
		for rawGroup := range entries {
			if err := appendUnknown("group_special_usable_group.target", rawGroup); err != nil {
				return PricingGroupConsistencyReport{}, err
			}
		}
	}
	for _, groupKey := range setting.GetAutoGroups() {
		if err := appendUnknown("auto_groups", groupKey); err != nil {
			return PricingGroupConsistencyReport{}, err
		}
	}

	sort.Slice(unresolved, func(i, j int) bool {
		if unresolved[i].Scope == unresolved[j].Scope {
			return unresolved[i].Value < unresolved[j].Value
		}
		return unresolved[i].Scope < unresolved[j].Scope
	})

	return PricingGroupConsistencyReport{UnresolvedLegacyReferences: unresolved}, nil
}

func listLegacyGroupRatioKeys() []string {
	groupRatios := ratio_setting.GetGroupRatioCopy()
	keys := make([]string, 0, len(groupRatios))
	for groupKey := range groupRatios {
		keys = append(keys, groupKey)
	}
	sort.Strings(keys)
	return keys
}

func normalizePricingGroupAuditValue(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "+:") || strings.HasPrefix(trimmed, "-:") {
		trimmed = strings.TrimSpace(trimmed[2:])
	}
	return trimmed
}
