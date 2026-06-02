package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/constant"
	"github.com/ca0fgh/hermestoken/dto"
	relaycommon "github.com/ca0fgh/hermestoken/relay/common"
	"github.com/ca0fgh/hermestoken/service"
	"github.com/ca0fgh/hermestoken/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// 验证「/v1/responses 退化空响应失败转移」（渠道 46 抽风：HTTP 200、只推 response.created
// 后即 EOF，零 usage 零输出文本）：
//   - 默认开启时合成可重试渠道错误（channel:empty_response / 503）→ 触发故障转移；
//   - 关闭开关时保持旧行为（返回 0-token usage、无错误）；
//   - 显式 response.failed 事件 → 同样抛可重试渠道错误；
//   - 正常 response.completed 带 usage / 有输出文本 → 绝不误触发，正常计费。

func driveResponsesStream(t *testing.T, body string, emptyFailover bool) (*dto.Usage, *types.HermesTokenError) {
	t.Helper()

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	oldToggle := common.ResponsesEmptyStreamFailover
	common.ResponsesEmptyStreamFailover = emptyFailover
	t.Cleanup(func() { common.ResponsesEmptyStreamFailover = oldToggle })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	info := &relaycommon.RelayInfo{
		IsStream:    true,
		DisablePing: true,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gpt-5.4"},
	}

	type result struct {
		usage *dto.Usage
		herr  *types.HermesTokenError
	}
	ch := make(chan result, 1)
	go func() {
		u, e := OaiResponsesStreamHandler(c, info, resp)
		ch <- result{u, e}
	}()
	select {
	case r := <-ch:
		return r.usage, r.herr
	case <-time.After(15 * time.Second):
		t.Fatal("OaiResponsesStreamHandler 未在预期时间内返回")
		return nil, nil
	}
}

// 退化空响应：上游只推 response.created 后即 EOF（零 usage、零输出文本）→ 合成可重试渠道错误。
func TestResponsesStream_EmptyFailover_Fires(t *testing.T) {
	body := "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_1\"}}\n\n"
	_, herr := driveResponsesStream(t, body, true)
	assert.NotNil(t, herr, "退化空响应应抛错以触发故障转移")
	assert.True(t, types.IsChannelError(herr), "应为渠道错误（channel: 前缀），以绕过亲和跳过重试")
	assert.Equal(t, http.StatusServiceUnavailable, herr.StatusCode, "应为可重试的 503")
}

// 开关关闭：保持旧行为——返回 0-token usage，不抛错。
func TestResponsesStream_EmptyFailover_Disabled(t *testing.T) {
	body := "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_1\"}}\n\n"
	usage, herr := driveResponsesStream(t, body, false)
	assert.Nil(t, herr, "开关关闭时不应抛错")
	if assert.NotNil(t, usage) {
		assert.Equal(t, 0, usage.TotalTokens, "保持旧行为：0 token")
	}
}

// 显式 response.failed 事件 → 抛可重试渠道错误。
func TestResponsesStream_ExplicitFailure_Fires(t *testing.T) {
	body := "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_1\"}}\n\n" +
		"data: {\"type\":\"response.failed\",\"response\":{\"id\":\"resp_1\",\"error\":{\"type\":\"server_error\",\"message\":\"boom\"}}}\n\n"
	_, herr := driveResponsesStream(t, body, true)
	assert.NotNil(t, herr, "显式失败事件应抛错")
	assert.True(t, types.IsChannelError(herr))
	assert.Equal(t, http.StatusServiceUnavailable, herr.StatusCode)
	assert.Contains(t, herr.Error(), "boom", "应携带上游错误信息")
}

// 裸 error 事件（message/code 在事件顶层，不在 response 下）→ 抛可重试渠道错误，
// 且日志携带上游真实错误原因（不再退化成无信息的 "(error)"）。
func TestResponsesStream_BareErrorEvent_CapturesDetail(t *testing.T) {
	body := "data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_1\"}}\n\n" +
		"data: {\"type\":\"error\",\"code\":\"context_length_exceeded\",\"message\":\"input is too long\"}\n\n"
	_, herr := driveResponsesStream(t, body, true)
	assert.NotNil(t, herr, "裸 error 事件应抛错")
	assert.True(t, types.IsChannelError(herr))
	assert.Equal(t, http.StatusServiceUnavailable, herr.StatusCode)
	assert.Contains(t, herr.Error(), "input is too long", "应携带上游顶层错误 message")
	assert.Contains(t, herr.Error(), "context_length_exceeded", "应携带上游顶层错误 code")
}

// 正常 completed 带 usage → 不抛错，usage 正确，绝不误触发。
func TestResponsesStream_CompletedWithUsage_NoFailover(t *testing.T) {
	body := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_1\",\"usage\":{\"input_tokens\":100,\"output_tokens\":20,\"total_tokens\":120}}}\n\n"
	usage, herr := driveResponsesStream(t, body, true)
	assert.Nil(t, herr, "成功响应绝不应触发失败转移")
	if assert.NotNil(t, usage) {
		assert.Equal(t, 100, usage.PromptTokens)
		assert.Equal(t, 20, usage.CompletionTokens)
		assert.Equal(t, 120, usage.TotalTokens)
	}
}

// 有输出文本但上游漏发 usage → 用文本估算 completion，total>0，不触发失败转移。
func TestResponsesStream_TextOnlyNoUsage_NoFailover(t *testing.T) {
	service.InitTokenEncoders() // 文本 token 估算需要分词器
	body := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"some real answer text\"}\n\n"
	usage, herr := driveResponsesStream(t, body, true)
	assert.Nil(t, herr, "有真实输出文本时不应触发失败转移")
	if assert.NotNil(t, usage) {
		assert.Greater(t, usage.TotalTokens, 0, "应据输出文本估算出非零 token")
	}
}
