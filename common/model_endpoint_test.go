package common

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/require"
)

func TestIsImageGenerationModelRecognizesCommonFamilies(t *testing.T) {
	models := []string{
		"dall-e-3",
		"openai/gpt-image-2",
		"imagen-4.0-generate-preview-06-06",
		"qwen-image-plus",
		"gemini-3-pro-image-preview",
		"seedream-4-0",
		"flux.1-kontext-pro",
	}

	for _, model := range models {
		require.Truef(t, IsImageGenerationModel(model), "expected %s to be recognized as image generation", model)
	}

	require.False(t, IsImageGenerationModel("gpt-4o-mini"))
	require.False(t, IsImageGenerationModel("claude-3-5-sonnet"))
}

func TestIsVideoGenerationModelRecognizesCommonFamilies(t *testing.T) {
	models := []string{
		"veo_3_1",
		"veo-3.1-generate-preview",
		"sora-2",
		"grok-imagine-video",
		"kling-v2-master",
		"hailuo-02",
		"wan2.5-t2v",
		"cogvideo-x",
		"vidu-q1",
	}

	for _, model := range models {
		require.Truef(t, IsVideoGenerationModel(model), "expected %s to be recognized as video generation", model)
	}

	require.False(t, IsVideoGenerationModel("gpt-4o-mini"))
	require.False(t, IsVideoGenerationModel("gemini-2.5-pro"))
}

func TestGetEndpointTypesByChannelTypePrependsMediaGenerationEndpoints(t *testing.T) {
	require.Equal(t,
		[]constant.EndpointType{constant.EndpointTypeImageGeneration, constant.EndpointTypeOpenAI},
		GetEndpointTypesByChannelType(constant.ChannelTypeOpenAI, "qwen-image-plus"),
	)

	require.Equal(t,
		[]constant.EndpointType{constant.EndpointTypeOpenAIVideo, constant.EndpointTypeOpenAI},
		GetEndpointTypesByChannelType(constant.ChannelTypeOpenAI, "veo_3_1"),
	)

	require.Equal(t,
		[]constant.EndpointType{constant.EndpointTypeOpenAIVideo},
		GetEndpointTypesByChannelType(constant.ChannelTypeSora, "sora-2"),
	)
}

func TestDefaultOpenAIVideoEndpointInfo(t *testing.T) {
	info, ok := GetDefaultEndpointInfo(constant.EndpointTypeOpenAIVideo)

	require.True(t, ok)
	require.Equal(t, "/v1/videos", info.Path)
	require.Equal(t, "POST", info.Method)
}
