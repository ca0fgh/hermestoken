package controller

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
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

func TestShouldRetry_SelectedChannelRetriesAnyUpstreamErrorWithinBudget(t *testing.T) {
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
