package service

import (
	"errors"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/stretchr/testify/require"
)

func TestShouldDisableChannel_UpstreamNoAvailableToken(t *testing.T) {
	previous := common.AutomaticDisableChannelEnabled
	previousKeywords := append([]string(nil), operation_setting.AutomaticDisableKeywords...)
	common.AutomaticDisableChannelEnabled = true
	operation_setting.AutomaticDisableKeywords = []string{"unrelated upstream failure"}
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = previous
		operation_setting.AutomaticDisableKeywords = previousKeywords
	})

	tests := []struct {
		name    string
		message string
	}{
		{
			name:    "chinese compact token wording",
			message: "没有可用token（traceid: b2f0e54a96a5a9186b0f8f0f7ce4ddfd）",
		},
		{
			name:    "chinese spaced token wording",
			message: "没有可用 token",
		},
		{
			name:    "english token wording",
			message: "No available tokens for this upstream pool",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			err := types.NewOpenAIError(errors.New(tt.message), types.ErrorCodeBadResponseStatusCode, http.StatusInternalServerError)
			require.True(t, ShouldDisableChannel(err))
		})
	}
}

func TestShouldDisableChannelModelAbility_UpstreamDistributorNoAvailableModel(t *testing.T) {
	previous := common.AutomaticDisableChannelEnabled
	common.AutomaticDisableChannelEnabled = true
	t.Cleanup(func() {
		common.AutomaticDisableChannelEnabled = previous
	})

	err := types.NewOpenAIError(
		errors.New("No available channel for model claude-haiku-4-5-20251001 under group 自营 kiro (distributor)"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusServiceUnavailable,
	)
	require.True(t, ShouldDisableChannelModelAbility(err))

	plainErr := types.NewOpenAIError(
		errors.New("No available channel for model claude-haiku-4-5-20251001"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusServiceUnavailable,
	)
	require.False(t, ShouldDisableChannelModelAbility(plainErr))
}
