package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func GetUserUsableGroups(userGroup string) map[string]string {
	return getUserGroupsByMode(0, userGroup, true)
}

func NormalizeTokenSelectionMode(selectionMode, groupKey, legacyGroup string) string {
	normalizedMode := strings.TrimSpace(selectionMode)
	switch normalizedMode {
	case "", "inherit_user_default", "fixed", "auto":
	default:
		return normalizedMode
	}

	trimmedGroupKey := strings.TrimSpace(groupKey)
	trimmedLegacyGroup := strings.TrimSpace(legacyGroup)
	if trimmedGroupKey != "" {
		return "fixed"
	}
	if trimmedLegacyGroup == "auto" {
		return "auto"
	}
	if trimmedLegacyGroup != "" {
		return "fixed"
	}

	if normalizedMode == "fixed" || normalizedMode == "auto" {
		return normalizedMode
	}
	return "inherit_user_default"
}

func GetUserUsableGroupsForUser(userId int, userGroup string) map[string]string {
	return getUserGroupsByMode(userId, userGroup, true)
}

func GetUserSelectableGroups(userGroup string) map[string]string {
	return getUserGroupsByMode(0, userGroup, false)
}

func GetUserSelectableGroupsForUser(userId int, userGroup string) map[string]string {
	return getUserGroupsByMode(userId, userGroup, false)
}

func getUserGroupsByMode(userId int, userGroup string, includeAssignedGroup bool) map[string]string {
	groupsCopy, err := model.LoadEffectiveUserUsableGroups()
	if err != nil {
		common.SysError(fmt.Sprintf("failed to load effective user usable pricing groups: %v", err))
		groupsCopy = setting.GetUserUsableGroupsCopy()
	}
	specialGroupRules, err := model.LoadEffectivePricingGroupVisibilityRules()
	if err != nil {
		common.SysError(fmt.Sprintf("failed to load effective pricing group visibility rules: %v", err))
		specialGroupRules = ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.ReadAll()
	}

	if userGroup != "" {
		specialSettings, b := specialGroupRules[userGroup]
		if b {
			// 处理特殊可用分组
			for specialGroup, desc := range specialSettings {
				if strings.HasPrefix(specialGroup, "-:") {
					// 移除分组
					groupToRemove := strings.TrimPrefix(specialGroup, "-:")
					delete(groupsCopy, groupToRemove)
				} else if strings.HasPrefix(specialGroup, "+:") {
					// 添加分组
					groupToAdd := strings.TrimPrefix(specialGroup, "+:")
					groupsCopy[groupToAdd] = desc
				} else {
					// 直接添加分组
					groupsCopy[specialGroup] = desc
				}
			}
		}
		// 已分配的用户分组始终可用，但未必应该出现在用户自选列表中。
		if includeAssignedGroup {
			if _, ok := groupsCopy[userGroup]; !ok {
				groupsCopy[userGroup] = "用户分组"
			}
		}
	}
	if userId > 0 {
		subscriptionGroups, err := model.GetActiveUserSubscriptionUpgradeGroups(userId)
		if err != nil {
			common.SysError(fmt.Sprintf("failed to load active subscription upgrade groups for user %d: %v", userId, err))
			return groupsCopy
		}
		for _, subscriptionGroup := range subscriptionGroups {
			if _, ok := groupsCopy[subscriptionGroup]; !ok {
				groupsCopy[subscriptionGroup] = setting.GetUsableGroupDescription(subscriptionGroup)
			}
		}
	}
	return groupsCopy
}

func GroupInUserUsableGroups(userGroup, groupName string) bool {
	_, ok := GetUserUsableGroups(userGroup)[groupName]
	return ok
}

func GroupInUserUsableGroupsForUser(userId int, userGroup, groupName string) bool {
	_, ok := GetUserUsableGroupsForUser(userId, userGroup)[groupName]
	return ok
}

func GroupInUserSelectableGroups(userGroup, groupName string) bool {
	_, ok := GetUserSelectableGroups(userGroup)[groupName]
	return ok
}

func GroupInUserSelectableGroupsForUser(userId int, userGroup, groupName string) bool {
	_, ok := GetUserSelectableGroupsForUser(userId, userGroup)[groupName]
	return ok
}

func ValidateTokenSelectableGroup(userGroup, tokenGroup string) error {
	return ValidateTokenSelectableGroupForUser(0, userGroup, tokenGroup)
}

func canonicalizeRequestGroup(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "auto" {
		return trimmed, nil
	}

	resolution, err := ResolveCanonicalPricingGroupKey(trimmed)
	if err == nil && resolution.CanonicalKey != "" {
		return resolution.CanonicalKey, nil
	}
	if err != nil {
		switch resolution.Source {
		case PricingGroupResolutionSourceUnknown, PricingGroupResolutionSourceEmpty:
			return trimmed, nil
		default:
			if strings.Contains(err.Error(), "pricing group store unavailable") {
				return trimmed, nil
			}
			return "", err
		}
	}
	return trimmed, nil
}

func ValidateTokenSelectableGroupForUser(userId int, userGroup, tokenGroup string) error {
	canonicalUserGroup, err := canonicalizeRequestGroup(userGroup)
	if err != nil {
		return err
	}
	if tokenGroup == "" {
		if GroupInUserSelectableGroupsForUser(userId, canonicalUserGroup, canonicalUserGroup) {
			return nil
		}
		return errors.New("当前用户默认分组未开放为用户可选分组，请选择一个用户可选分组")
	}
	canonicalTokenGroup, err := canonicalizeRequestGroup(tokenGroup)
	if err != nil {
		return err
	}
	if !GroupInUserSelectableGroupsForUser(userId, canonicalUserGroup, canonicalTokenGroup) {
		return fmt.Errorf("分组 %s 不是用户可选分组", canonicalTokenGroup)
	}
	groupRatios, err := model.LoadEffectivePricingGroupRatios()
	if err != nil {
		return err
	}
	if canonicalTokenGroup != "auto" {
		if _, ok := groupRatios[canonicalTokenGroup]; !ok {
			return fmt.Errorf("分组 %s 已被弃用", canonicalTokenGroup)
		}
	}
	return nil
}

func ResolveTokenGroupForRequest(userGroup, tokenGroup string) (string, error) {
	return ResolveTokenGroupForUserRequest(0, userGroup, tokenGroup)
}

func ResolveTokenGroupForUserToken(userId int, userGroup string, token *model.Token) (string, error) {
	if token == nil {
		return "", errors.New("token is nil")
	}

	switch NormalizeTokenSelectionMode(token.SelectionMode, token.GroupKey, token.Group) {
	case "", "inherit_user_default":
		return ResolveTokenGroupForUserRequest(userId, userGroup, "")
	case "fixed":
		requestedGroup := strings.TrimSpace(token.GroupKey)
		if requestedGroup == "" {
			requestedGroup = strings.TrimSpace(token.Group)
		}
		return ResolveTokenGroupForUserRequest(userId, userGroup, requestedGroup)
	case "auto":
		return "auto", nil
	default:
		return "", fmt.Errorf("invalid token selection mode: %s", token.SelectionMode)
	}
}

func GetTokenSelectionRoutingGroup(token *model.Token) string {
	if token == nil {
		return ""
	}

	switch NormalizeTokenSelectionMode(token.SelectionMode, token.GroupKey, token.Group) {
	case "fixed":
		if groupKey := strings.TrimSpace(token.GroupKey); groupKey != "" {
			return groupKey
		}
		return strings.TrimSpace(token.Group)
	case "auto":
		return "auto"
	default:
		return ""
	}
}

func ResolveTokenGroupForUserRequest(userId int, userGroup, tokenGroup string) (string, error) {
	canonicalUserGroup, err := canonicalizeRequestGroup(userGroup)
	if err != nil {
		return "", err
	}
	if tokenGroup == "" {
		if !GroupInUserSelectableGroupsForUser(userId, canonicalUserGroup, canonicalUserGroup) {
			return "", errors.New("当前令牌未配置可用分组，请选择一个用户可选分组")
		}
		return canonicalUserGroup, nil
	}
	canonicalTokenGroup, err := canonicalizeRequestGroup(tokenGroup)
	if err != nil {
		return "", err
	}
	if !GroupInUserUsableGroupsForUser(userId, canonicalUserGroup, canonicalTokenGroup) {
		return "", fmt.Errorf("无权访问 %s 分组", canonicalTokenGroup)
	}
	groupRatios, err := model.LoadEffectivePricingGroupRatios()
	if err != nil {
		return "", err
	}
	if canonicalTokenGroup != "auto" {
		if _, ok := groupRatios[canonicalTokenGroup]; !ok {
			return "", fmt.Errorf("分组 %s 已被弃用", canonicalTokenGroup)
		}
	}
	return canonicalTokenGroup, nil
}

// GetUserAutoGroup 根据用户分组获取自动分组设置
func GetUserAutoGroup(userGroup string) []string {
	return GetUserAutoGroupForUser(0, userGroup)
}

func GetUserAutoGroupForUser(userId int, userGroup string) []string {
	groups := GetUserUsableGroupsForUser(userId, userGroup)
	effectiveAutoGroups, err := model.LoadEffectiveAutoGroupKeys()
	if err != nil {
		common.SysError(fmt.Sprintf("failed to load effective auto pricing groups: %v", err))
		effectiveAutoGroups = setting.GetAutoGroups()
	}
	autoGroups := make([]string, 0)
	for _, group := range effectiveAutoGroups {
		if _, ok := groups[group]; ok {
			autoGroups = append(autoGroups, group)
		}
	}
	return autoGroups
}

// GetUserGroupRatio 获取用户使用某个分组的倍率
// userGroup 用户分组
// group 需要获取倍率的分组
func GetUserGroupRatio(userGroup, group string) float64 {
	groupOverrides, err := model.LoadEffectivePricingGroupRatioOverrides()
	if err == nil {
		if ratiosByTarget, ok := groupOverrides[userGroup]; ok {
			if ratio, ok := ratiosByTarget[group]; ok {
				return ratio
			}
		}
	} else {
		common.SysError(fmt.Sprintf("failed to load effective pricing group ratio overrides: %v", err))
		if ratio, ok := ratio_setting.GetGroupGroupRatio(userGroup, group); ok {
			return ratio
		}
	}

	groupRatios, err := model.LoadEffectivePricingGroupRatios()
	if err == nil {
		if ratio, ok := groupRatios[group]; ok {
			return ratio
		}
	} else {
		common.SysError(fmt.Sprintf("failed to load effective pricing group ratios: %v", err))
	}
	return ratio_setting.GetGroupRatio(group)
}
