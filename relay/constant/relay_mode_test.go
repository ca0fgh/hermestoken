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
