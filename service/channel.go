package service

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
)

const autoDisabledModelAbilitiesInfoKey = "auto_disabled_model_abilities"

type AutoDisabledModelAbilityInfo struct {
	Reason     string   `json:"reason"`
	StatusTime int64    `json:"status_time"`
	Groups     []string `json:"groups,omitempty"`
}

func formatNotifyType(channelId int, status int) string {
	return fmt.Sprintf("%s_%d_%d", dto.NotifyTypeChannelUpdate, channelId, status)
}

// disable & notify
func DisableChannel(channelError types.ChannelError, reason string) {
	common.SysLog(fmt.Sprintf("通道「%s」（#%d）发生错误，准备禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason))

	// 检查是否启用自动禁用功能
	if !channelError.AutoBan {
		common.SysLog(fmt.Sprintf("通道「%s」（#%d）未启用自动禁用功能，跳过禁用操作", channelError.ChannelName, channelError.ChannelId))
		return
	}

	success := model.UpdateChannelStatus(channelError.ChannelId, channelError.UsingKey, common.ChannelStatusAutoDisabled, reason)
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被禁用", channelError.ChannelName, channelError.ChannelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被禁用，原因：%s", channelError.ChannelName, channelError.ChannelId, reason)
		NotifyRootUser(formatNotifyType(channelError.ChannelId, common.ChannelStatusAutoDisabled), subject, content)
	}
}

func EnableChannel(channelId int, usingKey string, channelName string) {
	success := model.UpdateChannelStatus(channelId, usingKey, common.ChannelStatusEnabled, "")
	if success {
		subject := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		content := fmt.Sprintf("通道「%s」（#%d）已被启用", channelName, channelId)
		NotifyRootUser(formatNotifyType(channelId, common.ChannelStatusEnabled), subject, content)
	}
}

func ShouldDisableChannel(err *types.NewAPIError) bool {
	if !common.AutomaticDisableChannelEnabled {
		return false
	}
	if err == nil {
		return false
	}
	if types.IsChannelError(err) {
		return true
	}
	if types.IsSkipRetryError(err) {
		return false
	}
	if operation_setting.ShouldDisableByStatusCode(err.StatusCode) {
		return true
	}

	lowerMessage := strings.ToLower(err.Error())
	if isUpstreamTokenPoolExhausted(lowerMessage) {
		return true
	}
	search, _ := AcSearch(lowerMessage, operation_setting.AutomaticDisableKeywords, true)
	if search {
		return true
	}

	// Once a channel has been selected, any non-skip error means the channel call
	// did not succeed. Treat it as unavailable so auto-ban can take it out of use.
	return true
}

func ShouldDisableChannelModelAbility(err *types.NewAPIError) bool {
	if !common.AutomaticDisableChannelEnabled || err == nil {
		return false
	}
	lowerMessage := strings.ToLower(err.Error())
	return strings.Contains(lowerMessage, "distributor") &&
		(strings.Contains(lowerMessage, "no available channel for model") ||
			strings.Contains(lowerMessage, "无可用渠道"))
}

func DisableChannelModelAbility(channelError types.ChannelError, modelName string, reason string) {
	modelName = strings.TrimSpace(modelName)
	if channelError.ChannelId <= 0 || modelName == "" {
		return
	}
	common.SysLog(fmt.Sprintf("通道「%s」（#%d）模型 %s 不可用，准备禁用该模型能力，原因：%s", channelError.ChannelName, channelError.ChannelId, modelName, reason))
	groups, err := model.GetEnabledGroupsForChannelModel(channelError.ChannelId, modelName)
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to get enabled groups before disabling channel model ability: channel_id=%d, model=%s, error=%v", channelError.ChannelId, modelName, err))
	}
	if len(groups) == 0 {
		if channel, err := model.GetChannelById(channelError.ChannelId, true); err == nil {
			groups = GetAutoDisabledChannelModelAbilities(channel)[modelName].Groups
		}
	}
	if err := model.UpdateAbilityStatusByChannelModelGroups(channelError.ChannelId, modelName, groups, false); err != nil {
		common.SysLog(fmt.Sprintf("failed to disable channel model ability: channel_id=%d, model=%s, error=%v", channelError.ChannelId, modelName, err))
		return
	}
	if err := SaveAutoDisabledChannelModelAbility(channelError.ChannelId, modelName, reason, groups); err != nil {
		common.SysLog(fmt.Sprintf("failed to save disabled channel model ability info: channel_id=%d, model=%s, error=%v", channelError.ChannelId, modelName, err))
	}
	model.CacheDisableChannelModel(channelError.ChannelId, modelName)
}

func EnableChannelModelAbility(channelId int, modelName string, channelName string) {
	modelName = strings.TrimSpace(modelName)
	if channelId <= 0 || modelName == "" {
		return
	}
	channel, err := model.GetChannelById(channelId, true)
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to get channel before enabling channel model ability: channel_id=%d, model=%s, error=%v", channelId, modelName, err))
		return
	}
	disabledInfo := GetAutoDisabledChannelModelAbilities(channel)[modelName]
	if err := model.UpdateAbilityStatusByChannelModelGroups(channelId, modelName, disabledInfo.Groups, true); err != nil {
		common.SysLog(fmt.Sprintf("failed to enable channel model ability: channel_id=%d, model=%s, error=%v", channelId, modelName, err))
		return
	}
	if err := ClearAutoDisabledChannelModelAbility(channelId, modelName); err != nil {
		common.SysLog(fmt.Sprintf("failed to clear disabled channel model ability info: channel_id=%d, model=%s, error=%v", channelId, modelName, err))
	}
	model.InitChannelCache()
	subject := fmt.Sprintf("通道「%s」（#%d）模型 %s 已被启用", channelName, channelId, modelName)
	content := fmt.Sprintf("通道「%s」（#%d）模型 %s 探活成功，已自动启用", channelName, channelId, modelName)
	NotifyRootUser(formatNotifyType(channelId, common.ChannelStatusEnabled), subject, content)
}

func GetAutoDisabledChannelModelAbilities(channel *model.Channel) map[string]AutoDisabledModelAbilityInfo {
	if channel == nil {
		return map[string]AutoDisabledModelAbilityInfo{}
	}
	raw, ok := channel.GetOtherInfo()[autoDisabledModelAbilitiesInfoKey]
	if !ok || raw == nil {
		return map[string]AutoDisabledModelAbilityInfo{}
	}
	rawBytes, err := common.Marshal(raw)
	if err != nil {
		return map[string]AutoDisabledModelAbilityInfo{}
	}
	disabled := make(map[string]AutoDisabledModelAbilityInfo)
	if err := common.Unmarshal(rawBytes, &disabled); err != nil {
		return map[string]AutoDisabledModelAbilityInfo{}
	}
	return disabled
}

func SaveAutoDisabledChannelModelAbility(channelId int, modelName string, reason string, groups []string) error {
	modelName = strings.TrimSpace(modelName)
	if channelId <= 0 || modelName == "" {
		return nil
	}
	channel, err := model.GetChannelById(channelId, true)
	if err != nil {
		return err
	}
	disabled := GetAutoDisabledChannelModelAbilities(channel)
	disabled[modelName] = AutoDisabledModelAbilityInfo{
		Reason:     reason,
		StatusTime: common.GetTimestamp(),
		Groups:     groups,
	}
	info := channel.GetOtherInfo()
	info[autoDisabledModelAbilitiesInfoKey] = disabled
	channel.SetOtherInfo(info)
	return channel.SaveWithoutKey()
}

func ClearAutoDisabledChannelModelAbility(channelId int, modelName string) error {
	modelName = strings.TrimSpace(modelName)
	if channelId <= 0 || modelName == "" {
		return nil
	}
	channel, err := model.GetChannelById(channelId, true)
	if err != nil {
		return err
	}
	disabled := GetAutoDisabledChannelModelAbilities(channel)
	if len(disabled) == 0 {
		return nil
	}
	delete(disabled, modelName)
	info := channel.GetOtherInfo()
	if len(disabled) == 0 {
		delete(info, autoDisabledModelAbilitiesInfoKey)
	} else {
		info[autoDisabledModelAbilitiesInfoKey] = disabled
	}
	channel.SetOtherInfo(info)
	return channel.SaveWithoutKey()
}

func isUpstreamTokenPoolExhausted(lowerMessage string) bool {
	normalized := strings.ReplaceAll(lowerMessage, " ", "")
	return strings.Contains(normalized, "没有可用token") ||
		strings.Contains(lowerMessage, "no available token")
}

func ShouldEnableChannel(newAPIError *types.NewAPIError, status int) bool {
	if !common.AutomaticEnableChannelEnabled {
		return false
	}
	if newAPIError != nil {
		return false
	}
	if status != common.ChannelStatusAutoDisabled {
		return false
	}
	return true
}
