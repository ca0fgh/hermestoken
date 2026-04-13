package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

func GetUserUsableGroups(userGroup string) map[string]string {
	return getUserGroupsByMode(userGroup, true)
}

func GetUserSelectableGroups(userGroup string) map[string]string {
	return getUserGroupsByMode(userGroup, false)
}

func getUserGroupsByMode(userGroup string, includeAssignedGroup bool) map[string]string {
	groupsCopy := setting.GetUserUsableGroupsCopy()
	if userGroup != "" {
		specialSettings, b := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Get(userGroup)
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
	return groupsCopy
}

func GroupInUserUsableGroups(userGroup, groupName string) bool {
	_, ok := GetUserUsableGroups(userGroup)[groupName]
	return ok
}

func GroupInUserSelectableGroups(userGroup, groupName string) bool {
	_, ok := GetUserSelectableGroups(userGroup)[groupName]
	return ok
}

func ValidateTokenSelectableGroup(userGroup, tokenGroup string) error {
	if tokenGroup == "" {
		if GroupInUserSelectableGroups(userGroup, userGroup) {
			return nil
		}
		return errors.New("当前用户默认分组未开放为用户可选分组，请选择一个用户可选分组")
	}
	if !GroupInUserSelectableGroups(userGroup, tokenGroup) {
		return fmt.Errorf("分组 %s 不是用户可选分组", tokenGroup)
	}
	if tokenGroup != "auto" && !ratio_setting.ContainsGroupRatio(tokenGroup) {
		return fmt.Errorf("分组 %s 已被弃用", tokenGroup)
	}
	return nil
}

func ResolveTokenGroupForRequest(userGroup, tokenGroup string) (string, error) {
	if tokenGroup == "" {
		if !GroupInUserSelectableGroups(userGroup, userGroup) {
			return "", errors.New("当前令牌未配置可用分组，请选择一个用户可选分组")
		}
		return userGroup, nil
	}
	if !GroupInUserUsableGroups(userGroup, tokenGroup) {
		return "", fmt.Errorf("无权访问 %s 分组", tokenGroup)
	}
	if tokenGroup != "auto" && !ratio_setting.ContainsGroupRatio(tokenGroup) {
		return "", fmt.Errorf("分组 %s 已被弃用", tokenGroup)
	}
	return tokenGroup, nil
}

// GetUserAutoGroup 根据用户分组获取自动分组设置
func GetUserAutoGroup(userGroup string) []string {
	groups := GetUserUsableGroups(userGroup)
	autoGroups := make([]string, 0)
	for _, group := range setting.GetAutoGroups() {
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
	ratio, ok := ratio_setting.GetGroupGroupRatio(userGroup, group)
	if ok {
		return ratio
	}
	return ratio_setting.GetGroupRatio(group)
}
