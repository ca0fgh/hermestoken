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
	return search
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
	if err := model.UpdateAbilityStatusByChannelModel(channelError.ChannelId, modelName, false); err != nil {
		common.SysLog(fmt.Sprintf("failed to disable channel model ability: channel_id=%d, model=%s, error=%v", channelError.ChannelId, modelName, err))
		return
	}
	model.CacheDisableChannelModel(channelError.ChannelId, modelName)
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
