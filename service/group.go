package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting"
	"github.com/ca0fgh/hermestoken/setting/ratio_setting"
)

func GetUserUsableGroups(userGroup string) map[string]string {
	return getUserGroupsByMode(0, userGroup, true)
}

func GetUserUsableGroupsForUser(userId int, userGroup string) map[string]string {
	return getUserGroupsByMode(userId, userGroup, true)
}

func GetUserSelectableGroups(userGroup string) map[string]string {
	return getUserGroupsByMode(0, userGroup, true)
}

func GetUserSelectableGroupsForUser(userId int, userGroup string) map[string]string {
	return getUserGroupsByMode(userId, userGroup, true)
}

func GetUserTokenSelectableGroups(userGroup string) map[string]string {
	return getUserGroupsByMode(0, userGroup, false)
}

func GetUserTokenSelectableGroupsForUser(userId int, userGroup string) map[string]string {
	return getUserGroupsByMode(userId, userGroup, false)
}

func getUserGroupsByMode(userId int, userGroup string, includeAssignedUserGroup bool) map[string]string {
	groupsCopy := setting.GetUserUsableGroupsCopy()
	if userGroup != "" {
		applySpecialUsableGroups(groupsCopy, userGroup)
		if shouldIncludeAssignedUserGroup(userGroup, includeAssignedUserGroup) {
			groupsCopy[userGroup] = "用户分组"
		}
	}
	if userId > 0 {
		appendActiveSubscriptionGroups(groupsCopy, userId)
	}
	if !includeAssignedUserGroup {
		delete(groupsCopy, "default")
	}
	return groupsCopy
}

func shouldIncludeAssignedUserGroup(userGroup string, includeAssignedUserGroup bool) bool {
	if userGroup == "" {
		return false
	}
	if userGroup == "default" {
		return false
	}
	if includeAssignedUserGroup {
		return true
	}
	return ratio_setting.ContainsGroupRatio(userGroup)
}

func applySpecialUsableGroups(groups map[string]string, userGroup string) {
	specialSettings, ok := ratio_setting.GetGroupRatioSetting().GroupSpecialUsableGroup.Get(userGroup)
	if !ok {
		return
	}
	for specialGroup, desc := range specialSettings {
		if strings.HasPrefix(specialGroup, "-:") {
			delete(groups, strings.TrimPrefix(specialGroup, "-:"))
			continue
		}
		if strings.HasPrefix(specialGroup, "+:") {
			groups[strings.TrimPrefix(specialGroup, "+:")] = desc
			continue
		}
		groups[specialGroup] = desc
	}
}

func appendActiveSubscriptionGroups(groups map[string]string, userId int) {
	subscriptionGroups, err := model.GetActiveUserSubscriptionUpgradeGroups(userId)
	if err != nil {
		common.SysError(fmt.Sprintf("failed to load active subscription upgrade groups for user %d: %v", userId, err))
		return
	}
	for _, subscriptionGroup := range subscriptionGroups {
		if _, ok := groups[subscriptionGroup]; !ok {
			groups[subscriptionGroup] = setting.GetUsableGroupDescription(subscriptionGroup)
		}
	}
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

func ResolveUsableGroupForUserRequest(userId int, userGroup, requestedGroup string) (string, error) {
	groups := GetUserUsableGroupsForUser(userId, userGroup)
	if requestedGroup == "" {
		if _, ok := groups[userGroup]; !ok {
			return "", errors.New("当前用户未配置可用分组，请选择一个用户可用分组")
		}
		return userGroup, nil
	}
	if _, ok := groups[requestedGroup]; !ok {
		return "", fmt.Errorf("无权访问 %s 分组", requestedGroup)
	}
	if requestedGroup != "auto" && !ratio_setting.ContainsGroupRatio(requestedGroup) {
		return "", fmt.Errorf("分组 %s 已被弃用", requestedGroup)
	}
	return requestedGroup, nil
}

func ValidateTokenSelectableGroup(userGroup, tokenGroup string) error {
	return ValidateTokenSelectableGroupForUser(0, userGroup, tokenGroup)
}

func ValidateTokenSelectableGroupForUser(userId int, userGroup, tokenGroup string) error {
	if tokenGroup == "" {
		return nil
	}
	if _, ok := GetUserTokenSelectableGroupsForUser(userId, userGroup)[tokenGroup]; !ok {
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
	tokenSelectableGroups := GetUserTokenSelectableGroupsForUser(userId, userGroup)
	if tokenGroup == "" {
		if _, ok := tokenSelectableGroups[userGroup]; !ok {
			return "", errors.New("当前令牌未配置可用分组，请选择一个用户可选分组")
		}
		return userGroup, nil
	}
	if _, ok := tokenSelectableGroups[tokenGroup]; !ok {
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
