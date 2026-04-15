package service

import (
	"fmt"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
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
		return PricingGroupResolution{Source: PricingGroupResolutionSourceUnknown}, fmt.Errorf("unknown pricing group: %s", trimmed)
	}

	var canonical model.PricingGroup
	if err := model.DB.Where("group_key = ?", trimmed).First(&canonical).Error; err == nil {
		return PricingGroupResolution{CanonicalKey: canonical.GroupKey, Source: PricingGroupResolutionSourceCanonical}, nil
	}

	if model.DB.Migrator().HasTable(&model.PricingGroupAlias{}) {
		var alias model.PricingGroupAlias
		if err := model.DB.Where("alias_key = ?", trimmed).First(&alias).Error; err == nil {
			if err := model.DB.Where("id = ?", alias.GroupId).First(&canonical).Error; err == nil {
				return PricingGroupResolution{CanonicalKey: canonical.GroupKey, Source: PricingGroupResolutionSourceAlias}, nil
			}
		}
	}

	return PricingGroupResolution{Source: PricingGroupResolutionSourceUnknown}, fmt.Errorf("unknown pricing group: %s", trimmed)
}

func ListCanonicalPricingGroupKeysOrFallback() []string {
	if model.DB == nil || !model.DB.Migrator().HasTable(&model.PricingGroup{}) {
		return listLegacyGroupRatioKeys()
	}

	var count int64
	if err := model.DB.Model(&model.PricingGroup{}).Count(&count).Error; err != nil {
		return make([]string, 0)
	}
	if count == 0 {
		return listLegacyGroupRatioKeys()
	}

	var groups []model.PricingGroup
	if err := model.DB.Order("sort_order asc").Order("group_key asc").Find(&groups).Error; err != nil {
		return make([]string, 0)
	}

	keys := make([]string, 0, len(groups))
	for _, group := range groups {
		if group.GroupKey == "" {
			continue
		}
		keys = append(keys, group.GroupKey)
	}
	return keys
}

func BuildPricingGroupConsistencyReport() PricingGroupConsistencyReport {
	unresolved := make([]PricingGroupLegacyReference, 0)
	seen := make(map[string]struct{})

	appendUnknown := func(scope string, raw string) {
		value := normalizePricingGroupAuditValue(raw)
		if value == "" {
			return
		}
		resolution, err := ResolveCanonicalPricingGroupKey(value)
		if err == nil || resolution.Source != PricingGroupResolutionSourceUnknown {
			return
		}
		key := scope + "\x00" + value
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		unresolved = append(unresolved, PricingGroupLegacyReference{Scope: scope, Value: value})
	}

	for groupKey := range ratio_setting.GetGroupRatioCopy() {
		appendUnknown("group_ratio", groupKey)
	}
	for subjectGroup, targetGroups := range ratio_setting.GetGroupRatioSetting().GroupGroupRatio.ReadAll() {
		appendUnknown("group_group_ratio.subject", subjectGroup)
		for targetGroup := range targetGroups {
			appendUnknown("group_group_ratio.target", targetGroup)
		}
	}
	for groupKey := range setting.GetUserUsableGroupsCopy() {
		appendUnknown("user_usable_groups", groupKey)
	}
	for subjectGroup, entries := range ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.ReadAll() {
		appendUnknown("group_special_usable_group.subject", subjectGroup)
		for rawGroup := range entries {
			appendUnknown("group_special_usable_group.target", rawGroup)
		}
	}

	sort.Slice(unresolved, func(i, j int) bool {
		if unresolved[i].Scope == unresolved[j].Scope {
			return unresolved[i].Value < unresolved[j].Value
		}
		return unresolved[i].Scope < unresolved[j].Scope
	})

	return PricingGroupConsistencyReport{UnresolvedLegacyReferences: unresolved}
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
