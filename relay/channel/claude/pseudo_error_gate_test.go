package claude

import (
	"io"
	"net/http"
	"strings"
	"testing"

	relaycommon "github.com/ca0fgh/hermestoken/relay/common"
	"github.com/ca0fgh/hermestoken/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func claudeStreamBody(chunks ...string) *http.Response {
	lines := make([]string, 0, len(chunks)*2)
	for _, chunk := range chunks {
		lines = append(lines, "data: "+chunk, "")
	}
	return &http.Response{Body: io.NopCloser(strings.NewReader(strings.Join(lines, "\n")))}
}

func claudeJSONBody(text string) *http.Response {
	escaped := strings.ReplaceAll(text, `"`, `\"`)
	escaped = strings.ReplaceAll(escaped, "\n", `\n`)
	body := `{"model":"claude-opus-4-6","usage":{"input_tokens":12,"output_tokens":40},` +
		`"content":[{"type":"text","text":"` + escaped + `"}]}`
	return &http.Response{Body: io.NopCloser(strings.NewReader(body))}
}

func openAIRelayInfo(stream bool) *relaycommon.RelayInfo {
	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "claude-opus-4-6"},
	}
	if stream {
		info.StreamStatus = relaycommon.NewStreamStatus()
	}
	return info
}

// An assistant asked what a stack trace means opens with exactly these words and
// then keeps writing. The old probe judged the first chunk, turning a successful
// answer into our own HTTP 401 — and 401 is inside AutomaticRetryStatusCodeRanges,
// so the request was re-run against every other channel and billed each time.
func TestClaudeStreamHandler_AllowsAnswerThatOpensWithErrorUnauthorized(t *testing.T) {
	ctx, recorder := newClaudeTestContext(t)

	resp := claudeStreamBody(
		`{"type":"content_block_delta","delta":{"text":"Error: unauthorized"}}`,
		`{"type":"content_block_delta","delta":{"text":" means the server rejected your key.\n\nRotate it and retry."}}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`,
	)

	usage, err := ClaudeStreamHandler(ctx, resp, openAIRelayInfo(true))
	require.Nil(t, err, "a model explaining an error is not an upstream failure")
	require.NotNil(t, usage)
	assert.Contains(t, recorder.Body.String(), "Rotate it and retry", "the answer must reach the caller")
}

func TestClaudeHandler_AllowsSingleLineAnswerThatOpensWithErrorUnauthorized(t *testing.T) {
	ctx, recorder := newClaudeTestContext(t)

	resp := claudeJSONBody("Error: unauthorized — the server rejected your API key. Rotate it and retry.")

	usage, err := ClaudeHandler(ctx, resp, openAIRelayInfo(false))
	require.Nil(t, err, "an unterminated marker is model output, not the upstream's error")
	require.NotNil(t, usage)
	assert.Contains(t, recorder.Body.String(), "Rotate it and retry")
}

// The bracket is the upstream's own terminator, so a closed marker must still be
// caught — tightening the gate must not blind the feature it exists for.
func TestClaudeHandler_StillRejectsClosedPseudoErrorMarkers(t *testing.T) {
	cases := []struct {
		name       string
		text       string
		wantStatus int
	}{
		{"invalid token", "[Error: claude-opus-4.6 无效的令牌]", http.StatusUnauthorized},
		{"insufficient quota", "[Error: claude-opus-4.6 额度不足]", http.StatusForbidden},
		{"rate limited", "[Error: claude-opus-4.6 所有账号均已达速率限制，请 140 秒后重试]", http.StatusTooManyRequests},
		{"upstream overloaded", "[Error: claude-opus-4.6 upstream error]", http.StatusServiceUnavailable},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, recorder := newClaudeTestContext(t)

			usage, err := ClaudeHandler(ctx, claudeJSONBody(testCase.text), openAIRelayInfo(false))
			require.NotNil(t, err, "a closed upstream marker must still be rejected")
			assert.Equal(t, testCase.wantStatus, err.StatusCode)
			assert.Nil(t, usage)
			assert.Empty(t, recorder.Body.String(), "the marker must never reach the caller as content")
		})
	}
}

// A closed marker arriving across chunks is still decisive mid-stream: the caller
// must not be billed for a channel that never produced an answer.
func TestClaudeStreamHandler_StillRejectsClosedMarkerSplitAcrossChunks(t *testing.T) {
	ctx, recorder := newClaudeTestContext(t)

	resp := claudeStreamBody(
		`{"type":"content_block_delta","delta":{"text":"[Error: claude-opus-4.6 "}}`,
		`{"type":"content_block_delta","delta":{"text":"额度不足]"}}`,
	)

	usage, err := ClaudeStreamHandler(ctx, resp, openAIRelayInfo(true))
	require.NotNil(t, err)
	assert.Equal(t, http.StatusForbidden, err.StatusCode)
	assert.Nil(t, usage)
	assert.Empty(t, recorder.Body.String())
}

func TestDetectPseudoClaudeTextErrorGate(t *testing.T) {
	cases := []struct {
		name          string
		text          string
		replyFinished bool
		wantStatus    int
	}{
		{"closed marker is decisive even mid-stream", "[Error: 无效的令牌]", false, http.StatusUnauthorized},
		{"open marker mid-stream is not yet judged", "Error: unauthorized", false, 0},
		{"open marker on a finished reply is model output", "Error: unauthorized", true, 0},
		{"an explanation is never the upstream's error", "Error: unauthorized\nThis means the key was rejected.", true, 0},
		{"the bare rate-limit sentence needs the reply to be over", "Reached message rate limit for this model. Resets in: 5m", false, 0},
		{"the bare rate-limit sentence counts once the reply is over", "Reached message rate limit for this model. Resets in: 5m", true, http.StatusTooManyRequests},
		{"a discussion of rate limits is untouched", "To avoid rate limits, use exponential backoff.", true, 0},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			detected := detectPseudoClaudeTextError(testCase.text, testCase.replyFinished)
			if testCase.wantStatus == 0 {
				assert.Nil(t, detected)
				return
			}
			require.NotNil(t, detected)
			assert.Equal(t, testCase.wantStatus, detected.StatusCode)
		})
	}
}
