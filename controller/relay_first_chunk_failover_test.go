package controller

import (
	"errors"
	"net/http"
	"testing"

	relaycommon "github.com/ca0fgh/hermestoken/relay/common"
	"github.com/ca0fgh/hermestoken/setting/operation_setting"
	"github.com/ca0fgh/hermestoken/types"
	"github.com/stretchr/testify/require"
)

// 首字超时失败转移合成的错误必须可重试，即使 channel_id 未置入 ctx（常规文本中继路径如此）。
// 503 落在自动重试区间(500-503)；504/524 被显式列为「永不重试」，故必须用 503 而非 504。
func TestFirstChunkFailover_SynthesizedErrorRetries(t *testing.T) {
	c := newRetryTestContext() // 不设置 channel_id，贴近生产文本中继

	require.True(t, shouldRetry(
		c,
		types.NewErrorWithStatusCode(errors.New("first chunk timeout"), types.ErrorCodeBadResponse, http.StatusServiceUnavailable),
		1,
	), "合成的 503 必须可重试，否则失败转移永远不触发")

	// 反证：504 不会重试（解释为何选 503 而非 504）。
	require.False(t, shouldRetry(
		c,
		types.NewErrorWithStatusCode(errors.New("gateway timeout"), types.ErrorCodeBadResponse, http.StatusGatewayTimeout),
		1,
	), "504 被列为永不重试，不能用于失败转移")
}

// 默认（开关关闭）时，首字超时阈值恒为 0 —— 零行为变更。
func TestFirstChunkTimeoutSeconds_DisabledByDefault(t *testing.T) {
	c := newRetryTestContext()
	info := &relaycommon.RelayInfo{IsStream: true}

	gs := operation_setting.GetGeneralSetting()
	old := gs.FirstChunkTimeoutEnabled
	gs.FirstChunkTimeoutEnabled = false
	t.Cleanup(func() { gs.FirstChunkTimeoutEnabled = old })

	require.Equal(t, 0, firstChunkTimeoutSecondsForRequest(c, info), "默认关闭时应恒为 0")
}

// 开关开启时的门控：流式 + 非亲和 + 未指定渠道 → 正阈值；其余情形 → 0。
func TestFirstChunkTimeoutSeconds_Gating(t *testing.T) {
	gs := operation_setting.GetGeneralSetting()
	oldEnabled, oldSecs := gs.FirstChunkTimeoutEnabled, gs.FirstChunkTimeoutSeconds
	gs.FirstChunkTimeoutEnabled = true
	gs.FirstChunkTimeoutSeconds = 45
	t.Cleanup(func() {
		gs.FirstChunkTimeoutEnabled = oldEnabled
		gs.FirstChunkTimeoutSeconds = oldSecs
	})

	stream := &relaycommon.RelayInfo{IsStream: true}

	// 普通非亲和流式 → 正阈值
	require.Equal(t, 45, firstChunkTimeoutSecondsForRequest(newRetryTestContext(), stream))

	// 非流式 → 0
	require.Equal(t, 0, firstChunkTimeoutSecondsForRequest(newRetryTestContext(), &relaycommon.RelayInfo{IsStream: false}))

	// 渠道亲和（如 codex 钉 channel 51）→ 0，永不失败转移，保护 prompt 缓存。
	cAff := newRetryTestContext()
	cAff.Set("channel_affinity_skip_retry_on_failure", true)
	require.Equal(t, 0, firstChunkTimeoutSecondsForRequest(cAff, stream), "亲和请求不得失败转移")

	// 用户指定固定渠道 → 0，不转移。
	cSpecific := newRetryTestContext()
	cSpecific.Set("specific_channel_id", 51)
	require.Equal(t, 0, firstChunkTimeoutSecondsForRequest(cSpecific, stream))

	// nil info → 0（防御）。
	require.Equal(t, 0, firstChunkTimeoutSecondsForRequest(newRetryTestContext(), nil))
}
