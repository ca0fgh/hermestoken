package constant

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPath2RelayModeRecognizesOpenAIVideoPaths(t *testing.T) {
	require.Equal(t, RelayModeVideoSubmit, Path2RelayMode("/v1/videos"))
	require.Equal(t, RelayModeVideoSubmit, Path2RelayMode("/v1/videos/video_123/remix"))
	require.Equal(t, RelayModeVideoFetchByID, Path2RelayMode("/v1/videos/video_123"))
	require.Equal(t, RelayModeVideoSubmit, Path2RelayMode("/v1/video/generations"))
	require.Equal(t, RelayModeVideoFetchByID, Path2RelayMode("/v1/video/generations/task_123"))
}

func TestPath2RelayModeRecognizesRootOpenAICompatiblePaths(t *testing.T) {
	require.Equal(t, RelayModeChatCompletions, Path2RelayMode("/chat/completions"))
	require.Equal(t, RelayModeCompletions, Path2RelayMode("/completions"))
	require.Equal(t, RelayModeResponses, Path2RelayMode("/responses"))
	require.Equal(t, RelayModeResponsesCompact, Path2RelayMode("/responses/compact"))
	require.Equal(t, RelayModeGemini, Path2RelayMode("/models/gemini-2.5-pro:generateContent"))
}

func TestPath2RelayModeRecognizesKlingVideoPaths(t *testing.T) {
	require.Equal(t, RelayModeVideoSubmit, Path2RelayMode("/kling/v1/videos/text2video"))
	require.Equal(t, RelayModeVideoSubmit, Path2RelayMode("/kling/v1/videos/image2video"))
	require.Equal(t, RelayModeVideoFetchByID, Path2RelayMode("/kling/v1/videos/text2video/task_123"))
	require.Equal(t, RelayModeVideoFetchByID, Path2RelayMode("/kling/v1/videos/image2video/task_123"))
}

func TestPath2RelayVideoModeUsesMethodWhenAvailable(t *testing.T) {
	require.Equal(t, RelayModeVideoSubmit, Path2RelayVideo(http.MethodPost, "/v1/videos"))
	require.Equal(t, RelayModeVideoFetchByID, Path2RelayVideo(http.MethodGet, "/v1/videos/video_123"))
	require.Equal(t, RelayModeUnknown, Path2RelayVideo(http.MethodDelete, "/v1/videos/video_123"))
}

func TestPath2RelayVideoDoesNotMatchSimilarPrefixes(t *testing.T) {
	require.Equal(t, RelayModeUnknown, Path2RelayMode("/v1/videos-old"))
	require.Equal(t, RelayModeUnknown, Path2RelayMode("/v1/video/generations-old"))
	require.Equal(t, RelayModeUnknown, Path2RelayMode("/kling/v1/videos-old/text2video"))
	require.Equal(t, RelayModeUnknown, Path2RelayVideo(http.MethodPost, "/v1/videos-old"))
}
