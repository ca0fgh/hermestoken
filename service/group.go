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

func ValidateTokenSelectableGroupForUser(userId int, userGroup, tokenGroup string) error {
	if tokenGroup == "" {
		if GroupInUserSelectableGroupsForUser(userId, userGroup, userGroup) {
			return nil
		}
		return errors.New("当前用户默认分组未开放为用户可选分组，请选择一个用户可选分组")
	}
	if !GroupInUserSelectableGroupsForUser(userId, userGroup, tokenGroup) {
		return fmt.Errorf("分组 %s 不是用户可选分组", tokenGroup)
	}
	if tokenGroup != "auto" && !ratio_setting.ContainsGroupRatio(tokenGroup) {
		return fmt.Errorf("分组 %s 已被弃用", tokenGroup)
	}
	return nil
}

func ResolveTokenGroupForRequest(userGroup, tokenGroup string) (string, error) {
	return ResolveTokenGroupForUserRequest(0, userGroup, tokenGroup)
}

func ResolveTokenGroupForUserRequest(userId int, userGroup, tokenGroup string) (string, error) {
	if tokenGroup == "" {
		if !GroupInUserSelectableGroupsForUser(userId, userGroup, userGroup) {
			return "", errors.New("当前令牌未配置可用分组，请选择一个用户可选分组")
		}
		return userGroup, nil
	}
	if !GroupInUserUsableGroupsForUser(userId, userGroup, tokenGroup) {
		return "", fmt.Errorf("无权访问 %s 分组", tokenGroup)
	}
	if tokenGroup != "auto" && !ratio_setting.ContainsGroupRatio(tokenGroup) {
		return "", fmt.Errorf("分组 %s 已被弃用", tokenGroup)
	}
	return tokenGroup, nil
}

// GetUserAutoGroup 根据用户分组获取自动分组设置
func GetUserAutoGroup(userGroup string) []string {
	return GetUserAutoGroupForUser(0, userGroup)
}

func GetUserAutoGroupForUser(userId int, userGroup string) []string {
	groups := GetUserUsableGroupsForUser(userId, userGroup)
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
