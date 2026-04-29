package claude

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/constant"
	relaycommon "github.com/ca0fgh/hermestoken/relay/common"
	"github.com/ca0fgh/hermestoken/types"

	"github.com/gin-gonic/gin"
)

func newClaudeTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	oldStreamingTimeout := constant.StreamingTimeout
	t.Cleanup(func() {
		constant.StreamingTimeout = oldStreamingTimeout
	})
	constant.StreamingTimeout = 30
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	return ctx, recorder
}

func TestClaudeStreamHandler_RejectsPseudoSuccessRateLimitStream(t *testing.T) {
	ctx, recorder := newClaudeTestContext(t)
	info := &relaycommon.RelayInfo{
		RelayFormat:  types.RelayFormatOpenAI,
		StreamStatus: relaycommon.NewStreamStatus(),
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-opus-4-6",
		},
	}

	body := strings.Join([]string{
		`data: {"type":"content_block_delta","delta":{"text":"[Error: claude-opus-4.6-thinking "}}`,
		"",
		`data: {"type":"content_block_delta","delta":{"text":"所有账号均已达速率限制，请 140 秒后重试]"}}`,
		"",
	}, "\n")
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(body)),
	}

	usage, err := ClaudeStreamHandler(ctx, resp, info)
	if err == nil {
		t.Fatal("expected stream pseudo-success error")
	}
	if err.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("StatusCode = %d, want %d", err.StatusCode, http.StatusTooManyRequests)
	}
	if usage != nil {
		t.Fatalf("expected nil usage, got %#v", usage)
	}
	if got := recorder.Body.String(); got != "" {
		t.Fatalf("expected no streamed content, got %q", got)
	}
}

func TestClaudeStreamHandler_RejectsSplitPrefixPseudoSuccessRateLimitStream(t *testing.T) {
	ctx, recorder := newClaudeTestContext(t)
	info := &relaycommon.RelayInfo{
		RelayFormat:  types.RelayFormatOpenAI,
		StreamStatus: relaycommon.NewStreamStatus(),
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-opus-4-6",
		},
	}

	body := strings.Join([]string{
		`data: {"type":"content_block_delta","delta":{"text":"["}}`,
		"",
		`data: {"type":"content_block_delta","delta":{"text":"Error: claude-opus-4.6-thinking 所有账号均已达速率限制，请 103 秒后重试]"}}`,
		"",
	}, "\n")
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(body)),
	}

	usage, err := ClaudeStreamHandler(ctx, resp, info)
	if err == nil {
		t.Fatal("expected split-prefix pseudo-success error")
	}
	if err.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("StatusCode = %d, want %d", err.StatusCode, http.StatusTooManyRequests)
	}
	if usage != nil {
		t.Fatalf("expected nil usage, got %#v", usage)
	}
	if got := recorder.Body.String(); got != "" {
		t.Fatalf("expected no streamed content, got %q", got)
	}
}

func TestClaudeHandler_RejectsPseudoSuccessRateLimitBody(t *testing.T) {
	ctx, recorder := newClaudeTestContext(t)
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-opus-4-6",
		},
	}

	body := `{
		"model":"claude-opus-4-6",
		"content":[
			{"type":"text","text":"[Error: claude-opus-4.6-thinking 所有账号均已达速率限制，请 140 秒后重试]"}
		]
	}`
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(body)),
	}

	usage, err := ClaudeHandler(ctx, resp, info)
	if err == nil {
		t.Fatal("expected body pseudo-success error")
	}
	if err.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("StatusCode = %d, want %d", err.StatusCode, http.StatusTooManyRequests)
	}
	if usage != nil {
		t.Fatalf("expected nil usage, got %#v", usage)
	}
	if got := recorder.Body.String(); got != "" {
		t.Fatalf("expected no body content, got %q", got)
	}
}

func TestClaudeStreamHandler_RejectsPseudoSuccessRateLimitClaudeStream(t *testing.T) {
	ctx, recorder := newClaudeTestContext(t)
	info := &relaycommon.RelayInfo{
		RelayFormat:  types.RelayFormatClaude,
		StreamStatus: relaycommon.NewStreamStatus(),
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-opus-4-6",
		},
	}

	body := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_1","model":"claude-opus-4-6","usage":{"input_tokens":12,"output_tokens":0}}}`,
		"",
		`data: {"type":"content_block_start","content_block":{"type":"text","text":"[Error: claude-opus-4.6-thinking "}}`,
		"",
		`data: {"type":"content_block_delta","delta":{"text":"所有账号均已达速率限制，请 140 秒后重试]"}}`,
		"",
	}, "\n")
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(body)),
	}

	usage, err := ClaudeStreamHandler(ctx, resp, info)
	if err == nil {
		t.Fatal("expected claude pseudo-success error")
	}
	if err.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("StatusCode = %d, want %d", err.StatusCode, http.StatusTooManyRequests)
	}
	if usage != nil {
		t.Fatalf("expected nil usage, got %#v", usage)
	}
	if got := recorder.Body.String(); got != "" {
		t.Fatalf("expected no streamed content, got %q", got)
	}
}

func TestClaudeHandler_RejectsPlainRateLimitTextBody(t *testing.T) {
	ctx, recorder := newClaudeTestContext(t)
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-opus-4-6",
		},
	}

	body := `{
		"model":"claude-opus-4-6",
		"content":[
			{"type":"text","text":"Reached message rate limit for this model. Please try again later. Resets in: 29m53s"}
		]
	}`
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(body)),
	}

	usage, err := ClaudeHandler(ctx, resp, info)
	if err == nil {
		t.Fatal("expected plain rate-limit body error")
	}
	if err.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("StatusCode = %d, want %d", err.StatusCode, http.StatusTooManyRequests)
	}
	if usage != nil {
		t.Fatalf("expected nil usage, got %#v", usage)
	}
	if got := recorder.Body.String(); got != "" {
		t.Fatalf("expected no body content, got %q", got)
	}
}

func TestClaudeStreamHandler_AllowsNormalLeadingErrorLikeText(t *testing.T) {
	ctx, recorder := newClaudeTestContext(t)
	info := &relaycommon.RelayInfo{
		RelayFormat:  types.RelayFormatOpenAI,
		StreamStatus: relaycommon.NewStreamStatus(),
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-opus-4-6",
		},
	}

	body := strings.Join([]string{
		`data: {"type":"content_block_delta","delta":{"text":"[Error: this is just demo copy]"}}`,
		"",
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}`,
		"",
	}, "\n")
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(body)),
	}

	usage, err := ClaudeStreamHandler(ctx, resp, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usage == nil {
		t.Fatal("expected usage fallback for normal text response")
	}
	got := recorder.Body.String()
	if !strings.Contains(got, "[Error: this is just demo copy]") {
		t.Fatalf("expected buffered text to flush, got %q", got)
	}
}

func TestClaudeStreamHandler_AllowsNormalLeadingRateLimitDiscussion(t *testing.T) {
	ctx, recorder := newClaudeTestContext(t)
	info := &relaycommon.RelayInfo{
		RelayFormat:  types.RelayFormatOpenAI,
		StreamStatus: relaycommon.NewStreamStatus(),
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-opus-4-6",
		},
	}

	body := strings.Join([]string{
		`data: {"type":"content_block_delta","delta":{"text":"To avoid rate limits, use exponential backoff."}}`,
		"",
		`data: {"type":"message_delta","delta":{"stop_reason":"end_turn"}}`,
		"",
	}, "\n")
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(body)),
	}

	usage, err := ClaudeStreamHandler(ctx, resp, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usage == nil {
		t.Fatal("expected usage fallback for normal text response")
	}
	got := recorder.Body.String()
	if !strings.Contains(got, "To avoid rate limits, use exponential backoff.") {
		t.Fatalf("expected normal discussion text to flush, got %q", got)
	}
}

func TestClaudeStreamHandler_AllowsToolUseFirstStream(t *testing.T) {
	ctx, recorder := newClaudeTestContext(t)
	info := &relaycommon.RelayInfo{
		RelayFormat:  types.RelayFormatClaude,
		StreamStatus: relaycommon.NewStreamStatus(),
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-opus-4-6",
		},
	}

	body := strings.Join([]string{
		`data: {"type":"message_start","message":{"id":"msg_1","model":"claude-opus-4-6","usage":{"input_tokens":12,"output_tokens":0}}}`,
		"",
		`data: {"type":"content_block_start","content_block":{"type":"tool_use","id":"tool_1","name":"search"}}`,
		"",
		`data: {"type":"content_block_delta","delta":{"type":"input_json_delta","partial_json":"{\"q\":\"hi\"}"}}`,
		"",
		`data: {"type":"message_delta","delta":{"stop_reason":"tool_use"}}`,
		"",
	}, "\n")
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(body)),
	}

	usage, err := ClaudeStreamHandler(ctx, resp, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usage == nil {
		t.Fatal("expected usage for tool stream")
	}
	got := recorder.Body.String()
	if !strings.Contains(got, "tool_use") || !strings.Contains(got, "input_json_delta") {
		t.Fatalf("expected tool-use chunks to flush, got %q", got)
	}
}

func TestClaudeHandler_AllowsUnprefixedInvalidTokenDiscussion(t *testing.T) {
	ctx, recorder := newClaudeTestContext(t)
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "claude-opus-4-6",
		},
	}

	body := `{
		"model":"claude-opus-4-6",
		"usage":{"input_tokens":12,"output_tokens":8},
		"content":[
			{"type":"text","text":"Invalid token caching assumptions can cause auth bugs."}
		]
	}`
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(body)),
	}

	usage, err := ClaudeHandler(ctx, resp, info)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if usage == nil {
		t.Fatal("expected usage for normal explanatory text")
	}
	if got := recorder.Body.String(); !strings.Contains(got, "Invalid token caching assumptions") {
		t.Fatalf("expected explanatory text to pass through, got %q", got)
	}
}
