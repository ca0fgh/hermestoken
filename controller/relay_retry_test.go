package controller

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/constant"
	"github.com/ca0fgh/hermestoken/dto"
	relaycommon "github.com/ca0fgh/hermestoken/relay/common"
	"github.com/ca0fgh/hermestoken/service"
	"github.com/ca0fgh/hermestoken/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newRetryTestContext() *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c
}

func TestFormatRetryChannels(t *testing.T) {
	t.Parallel()

	require.Equal(t, "", formatRetryChannels(nil))
	require.Equal(t, "", formatRetryChannels([]string{"17"}))
	require.Equal(t, "17->9", formatRetryChannels([]string{"17", "9"}))
}

func TestShouldRetry_SelectedChannelTreatsAnyUpstreamErrorAsChannelUnavailable(t *testing.T) {
	t.Parallel()

	c := newRetryTestContext()
	c.Set("channel_id", 123)

	require.True(t, shouldRetry(
		c,
		types.NewOpenAIError(errors.New("upstream rejected request"), types.ErrorCodeBadResponseStatusCode, http.StatusBadRequest),
		1,
	))
	require.True(t, shouldRetry(
		c,
		types.NewOpenAIError(errors.New("invalid upstream body"), types.ErrorCodeBadResponseBody, http.StatusInternalServerError),
		1,
	))
	require.True(t, shouldRetry(
		c,
		types.NewOpenAIError(errors.New("only allows Claude Code clients"), types.ErrorCodeBadResponseStatusCode, http.StatusForbidden),
		1,
	))
	require.True(t, shouldRetry(
		c,
		types.NewOpenAIError(errors.New("upstream timeout"), types.ErrorCodeBadResponseStatusCode, http.StatusGatewayTimeout),
		1,
	))
}

func TestShouldRetryTaskRelay_SelectedChannelTreatsAnyUpstreamErrorAsChannelUnavailable(t *testing.T) {
	t.Parallel()

	for _, statusCode := range []int{
		http.StatusBadRequest,
		http.StatusForbidden,
		http.StatusTooManyRequests,
		http.StatusGatewayTimeout,
		http.StatusServiceUnavailable,
	} {
		statusCode := statusCode
		t.Run(fmt.Sprintf("status %d", statusCode), func(t *testing.T) {
			t.Parallel()

			c := newRetryTestContext()
			err := &dto.TaskError{
				Code:       "upstream_error",
				Message:    "selected channel unavailable",
				StatusCode: statusCode,
				Error:      errors.New("selected channel unavailable"),
			}

			require.True(t, shouldRetryTaskRelay(c, 123, err, 1))
		})
	}
}

func TestShouldRetryTaskRelay_SelectedChannelStillHonorsNonRetryGuards(t *testing.T) {
	t.Parallel()

	t.Run("local errors", func(t *testing.T) {
		t.Parallel()

		c := newRetryTestContext()
		err := &dto.TaskError{
			Code:       "read_request_body_failed",
			Message:    "invalid local request",
			StatusCode: http.StatusBadRequest,
			LocalError: true,
			Error:      errors.New("invalid local request"),
		}

		require.False(t, shouldRetryTaskRelay(c, 123, err, 1))
	})

	t.Run("retry budget exhausted", func(t *testing.T) {
		t.Parallel()

		c := newRetryTestContext()
		err := &dto.TaskError{
			Code:       "upstream_error",
			Message:    "selected channel unavailable",
			StatusCode: http.StatusServiceUnavailable,
			Error:      errors.New("selected channel unavailable"),
		}

		require.False(t, shouldRetryTaskRelay(c, 123, err, 0))
	})

	t.Run("specific channel requests", func(t *testing.T) {
		t.Parallel()

		c := newRetryTestContext()
		c.Set("specific_channel_id", 123)
		err := &dto.TaskError{
			Code:       "upstream_error",
			Message:    "selected channel unavailable",
			StatusCode: http.StatusServiceUnavailable,
			Error:      errors.New("selected channel unavailable"),
		}

		require.False(t, shouldRetryTaskRelay(c, 123, err, 1))
	})

	t.Run("locked channel tasks", func(t *testing.T) {
		t.Parallel()

		c := newRetryTestContext()
		err := &dto.TaskError{
			Code:       "upstream_error",
			Message:    "locked channel unavailable",
			StatusCode: http.StatusServiceUnavailable,
			Error:      errors.New("locked channel unavailable"),
		}

		shouldRetryResult, reason := shouldRetryTaskRelayWithReason(c, 123, true, err, 1)
		require.False(t, shouldRetryResult)
		require.Equal(t, retryReasonLockedChannel, reason)
	})
}

func TestAppendRetryAdminInfo(t *testing.T) {
	t.Parallel()

	c := newRetryTestContext()
	adminInfo := map[string]interface{}{}
	appendRetryAdminInfo(c, adminInfo)
	require.Empty(t, adminInfo)

	setRetryAdminInfo(c, retryAdminInfo{
		Index:     2,
		Remaining: 1,
		WillRetry: true,
		Reason:    retryReasonSelectedChannelUnavailable,
	})
	appendRetryAdminInfo(c, adminInfo)

	require.Equal(t, 2, adminInfo["retry_index"])
	require.Equal(t, 1, adminInfo["retry_remaining"])
	require.Equal(t, true, adminInfo["retry_will_retry"])
	require.Equal(t, false, adminInfo["retry_final_attempt"])
	require.Equal(t, retryReasonSelectedChannelUnavailable, adminInfo["retry_reason"])
}

func TestWillRetryFromAdminInfo(t *testing.T) {
	t.Parallel()

	// 缺失重试信息时按终态处理（false）。
	require.False(t, willRetryFromAdminInfo(newRetryTestContext()))

	// 还会重试的中间失败。
	cRetry := newRetryTestContext()
	setRetryAdminInfo(cRetry, retryAdminInfo{Index: 0, Remaining: 2, WillRetry: true, Reason: retryReasonChannelError})
	require.True(t, willRetryFromAdminInfo(cRetry))

	// 重试耗尽的终态失败。
	cFinal := newRetryTestContext()
	setRetryAdminInfo(cFinal, retryAdminInfo{Index: 2, Remaining: 0, WillRetry: false, Reason: retryReasonBudgetExhausted})
	require.False(t, willRetryFromAdminInfo(cFinal))
}

func TestRetrySeedGroupUsesSelectedAutoGroup(t *testing.T) {
	t.Parallel()

	c := newRetryTestContext()
	common.SetContextKey(c, constant.ContextKeyAutoGroup, "default")

	require.Equal(t, "default", retrySeedGroup(c, &relaycommon.RelayInfo{
		TokenGroup: "auto",
		UsingGroup: "auto",
	}))
	require.Equal(t, "vip", retrySeedGroup(c, &relaycommon.RelayInfo{
		TokenGroup: "vip",
		UsingGroup: "vip",
	}))
}

func TestShouldRetryWithReason(t *testing.T) {
	t.Parallel()

	c := newRetryTestContext()
	c.Set("channel_id", 123)

	shouldRetryResult, reason := shouldRetryWithReason(
		c,
		types.NewOpenAIError(errors.New("bad gateway"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway),
		1,
	)

	require.True(t, shouldRetryResult)
	require.Equal(t, retryReasonSelectedChannelUnavailable, reason)

	c = newRetryTestContext()
	shouldRetryResult, reason = shouldRetryWithReason(
		c,
		types.NewOpenAIError(errors.New("bad gateway"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway),
		1,
	)

	require.True(t, shouldRetryResult)
	require.Equal(t, retryReasonStatusCode, reason)
}

func TestShouldRetry_SelectedChannelRetriesWhenChannelAffinityRuleSkipsOnCacheMiss(t *testing.T) {
	c := newRetryTestContext()
	c.Request = httptest.NewRequest(
		http.MethodPost,
		"/v1/messages",
		bytes.NewBufferString(fmt.Sprintf(`{"metadata":{"user_id":"retry-affinity-miss-%d"}}`, time.Now().UnixNano())),
	)
	c.Request.Header.Set("Content-Type", "application/json")

	_, found := service.GetPreferredChannelByAffinity(c, "claude-sonnet-4-6", "cc-opus-福利渠道")
	require.False(t, found)
	require.True(t, service.ShouldSkipRetryAfterChannelAffinityFailure(c))

	c.Set("channel_id", 123)
	require.True(t, shouldRetry(
		c,
		types.NewOpenAIError(errors.New("temporary upstream error"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway),
		1,
	))
}

func TestShouldRetry_SelectedChannelStillHonorsNonRetryGuards(t *testing.T) {
	t.Parallel()

	t.Run("skip retry errors", func(t *testing.T) {
		c := newRetryTestContext()
		c.Set("channel_id", 123)

		err := types.NewError(errors.New("invalid local request"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		require.False(t, shouldRetry(c, err, 1))
	})

	t.Run("retry budget exhausted", func(t *testing.T) {
		c := newRetryTestContext()
		c.Set("channel_id", 123)

		err := types.NewOpenAIError(errors.New("temporary upstream error"), types.ErrorCodeBadResponseStatusCode, http.StatusServiceUnavailable)
		require.False(t, shouldRetry(c, err, 0))

		channelErr := types.NewError(errors.New("no available key"), types.ErrorCodeChannelNoAvailableKey)
		require.False(t, shouldRetry(c, channelErr, 0))
	})

	t.Run("specific channel requests", func(t *testing.T) {
		c := newRetryTestContext()
		c.Set("channel_id", 123)
		c.Set("specific_channel_id", 123)

		err := types.NewOpenAIError(errors.New("temporary upstream error"), types.ErrorCodeBadResponseStatusCode, http.StatusServiceUnavailable)
		require.False(t, shouldRetry(c, err, 1))

		channelErr := types.NewError(errors.New("no available key"), types.ErrorCodeChannelNoAvailableKey)
		require.False(t, shouldRetry(c, channelErr, 1))
	})
}
