package controller

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/require"
)

func TestShouldWaitAutoDisabledChannelRecoveryCooldown(t *testing.T) {
	now := int64(1777135200)
	cooldown := 30 * time.Minute

	recentlyDisabled := &model.Channel{Status: common.ChannelStatusAutoDisabled}
	recentlyDisabled.SetOtherInfo(map[string]interface{}{
		"status_time": now - int64((29 * time.Minute).Seconds()),
	})
	require.True(t, shouldWaitAutoDisabledChannelRecoveryCooldown(recentlyDisabled, now, cooldown))

	cooldownElapsed := &model.Channel{Status: common.ChannelStatusAutoDisabled}
	cooldownElapsed.SetOtherInfo(map[string]interface{}{
		"status_time": now - int64((31 * time.Minute).Seconds()),
	})
	require.False(t, shouldWaitAutoDisabledChannelRecoveryCooldown(cooldownElapsed, now, cooldown))

	unknownDisableTime := &model.Channel{Status: common.ChannelStatusAutoDisabled}
	require.False(t, shouldWaitAutoDisabledChannelRecoveryCooldown(unknownDisableTime, now, cooldown))

	enabledChannel := &model.Channel{Status: common.ChannelStatusEnabled}
	enabledChannel.SetOtherInfo(map[string]interface{}{
		"status_time": now - int64(time.Minute.Seconds()),
	})
	require.False(t, shouldWaitAutoDisabledChannelRecoveryCooldown(enabledChannel, now, cooldown))

	require.False(t, shouldWaitAutoDisabledChannelRecoveryCooldown(recentlyDisabled, now, 0))
}

func TestShouldWaitAutoDisabledModelAbilityRecoveryCooldown(t *testing.T) {
	now := int64(1777135200)
	cooldown := 30 * time.Minute

	recentlyDisabled := service.AutoDisabledModelAbilityInfo{
		StatusTime: now - int64((29 * time.Minute).Seconds()),
	}
	require.True(t, shouldWaitAutoDisabledModelAbilityRecoveryCooldown(recentlyDisabled, now, cooldown))

	cooldownElapsed := service.AutoDisabledModelAbilityInfo{
		StatusTime: now - int64((31 * time.Minute).Seconds()),
	}
	require.False(t, shouldWaitAutoDisabledModelAbilityRecoveryCooldown(cooldownElapsed, now, cooldown))

	unknownDisableTime := service.AutoDisabledModelAbilityInfo{}
	require.False(t, shouldWaitAutoDisabledModelAbilityRecoveryCooldown(unknownDisableTime, now, cooldown))

	require.False(t, shouldWaitAutoDisabledModelAbilityRecoveryCooldown(recentlyDisabled, now, 0))
}

func TestResolveChannelTestModel(t *testing.T) {
	configured := " configured-model "
	channel := &model.Channel{
		TestModel: &configured,
		Models:    "first-model,second-model",
	}

	require.Equal(t, "explicit-model", resolveChannelTestModel(channel, " explicit-model "))
	require.Equal(t, "configured-model", resolveChannelTestModel(channel, ""))

	channel.TestModel = nil
	require.Equal(t, "first-model", resolveChannelTestModel(channel, ""))

	channel.Models = ""
	require.Equal(t, "gpt-4o-mini", resolveChannelTestModel(channel, ""))
}

func TestNormalizeChannelTestEndpointSelectsSupportedMediaEndpoint(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeOpenAI}

	require.Equal(t, string(constant.EndpointTypeImageGeneration), normalizeChannelTestEndpoint(channel, "qwen-image-plus", ""))
	require.Equal(t, string(constant.EndpointTypeImageGeneration), normalizeChannelTestEndpoint(channel, "openai/gpt-image-2", ""))
	require.Equal(t, "", normalizeChannelTestEndpoint(channel, "veo_3_1", ""))
	require.Equal(t, string(constant.EndpointTypeOpenAIVideo), normalizeChannelTestEndpoint(channel, "veo_3_1", string(constant.EndpointTypeOpenAIVideo)))
}

func TestShouldApplyResponseTimeDisableThresholdSkipsMediaGenerationEndpoints(t *testing.T) {
	channel := &model.Channel{Type: constant.ChannelTypeOpenAI}

	require.False(t, shouldApplyResponseTimeDisableThreshold(channel, "qwen-image-plus"))
	require.False(t, shouldApplyResponseTimeDisableThreshold(channel, "openai/gpt-image-2"))
	require.False(t, shouldApplyResponseTimeDisableThreshold(channel, "imagen-4.0-generate-preview-06-06"))
	require.False(t, shouldApplyResponseTimeDisableThreshold(channel, "seedream-4-0"))
	require.False(t, shouldApplyResponseTimeDisableThreshold(channel, "veo_3_1"))
	require.False(t, shouldApplyResponseTimeDisableThreshold(channel, "veo_3_1-4K"))
	require.False(t, shouldApplyResponseTimeDisableThreshold(channel, "sora-2"))
	require.False(t, shouldApplyResponseTimeDisableThreshold(channel, "grok-imagine-video"))
	require.False(t, shouldApplyResponseTimeDisableThreshold(channel, "kling-v2-master"))
	require.False(t, shouldApplyResponseTimeDisableThreshold(channel, "wan2.5-t2v"))
	require.False(t, shouldApplyResponseTimeDisableThreshold(&model.Channel{Type: constant.ChannelTypeSora}, "sora-2"))
	require.False(t, shouldApplyResponseTimeDisableThreshold(channel, "gpt-4o-mini", string(constant.EndpointTypeImageGeneration)))
	require.False(t, shouldApplyResponseTimeDisableThreshold(channel, "gpt-4o-mini", string(constant.EndpointTypeOpenAIVideo)))

	require.True(t, shouldApplyResponseTimeDisableThreshold(channel, "gpt-4o-mini"))
	require.True(t, shouldApplyResponseTimeDisableThreshold(&model.Channel{Type: constant.ChannelTypeGemini}, "gemini-2.5-pro"))
}

func TestTestChannelSkipsOpenAIVideoWithoutSynchronousProbe(t *testing.T) {
	result := testChannel(&model.Channel{Type: constant.ChannelTypeOpenAI}, "veo_3_1", "", false)

	require.True(t, result.skipped)
	require.Nil(t, result.newAPIError)
	require.ErrorContains(t, result.localErr, string(constant.EndpointTypeOpenAIVideo))
}

func TestTestChannelSkipsUnsupportedTaskChannelTypes(t *testing.T) {
	result := testChannel(&model.Channel{Type: constant.ChannelTypeKling}, "kling-v2-master", "", false)

	require.True(t, result.skipped)
	require.Nil(t, result.newAPIError)
	require.ErrorContains(t, result.localErr, "channel test is not supported")
}
