package controller

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
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
