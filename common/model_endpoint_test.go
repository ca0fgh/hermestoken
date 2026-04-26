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

func TestResolveEndpointTypePrefersRequestPath(t *testing.T) {
	cases := []struct {
		name     string
		path     string
		expected constant.EndpointType
	}{
		{name: "chat completions", path: "/v1/chat/completions", expected: constant.EndpointTypeOpenAI},
		{name: "responses compact", path: "/v1/responses/compact", expected: constant.EndpointTypeOpenAIResponseCompact},
		{name: "responses", path: "/v1/responses", expected: constant.EndpointTypeOpenAIResponse},
		{name: "image generations", path: "/v1/images/generations", expected: constant.EndpointTypeImageGeneration},
		{name: "image edits", path: "/v1/images/edits", expected: constant.EndpointTypeImageGeneration},
		{name: "openai videos", path: "/v1/videos", expected: constant.EndpointTypeOpenAIVideo},
		{name: "openai video fetch", path: "/v1/videos/task_123", expected: constant.EndpointTypeOpenAIVideo},
		{name: "generic video generations", path: "/v1/video/generations", expected: constant.EndpointTypeOpenAIVideo},
		{name: "kling videos", path: "/kling/v1/videos/text2video", expected: constant.EndpointTypeOpenAIVideo},
		{name: "embeddings", path: "/v1/embeddings", expected: constant.EndpointTypeEmbeddings},
		{name: "rerank", path: "/v1/rerank", expected: constant.EndpointTypeJinaRerank},
		{name: "gemini", path: "/v1beta/models/gemini-2.5-pro:generateContent", expected: constant.EndpointTypeGemini},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, ResolveEndpointType(EndpointResolutionInput{
				Path:         tc.path,
				EndpointType: string(constant.EndpointTypeOpenAI),
				ModelName:    "gpt-4o-mini",
				ChannelType:  constant.ChannelTypeOpenAI,
			}))
		})
	}
}

func TestResolveEndpointTypeDoesNotMatchSimilarPathPrefixes(t *testing.T) {
	paths := []string{
		"/v1/videos-old",
		"/v1/video/generations-old",
		"/v1/images/generations-old",
		"/v1/responses-compact",
		"/v1/chat/completions-old",
		"/kling/v1/videos-old/text2video",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			endpointType, ok := ResolveEndpointTypeFromPath(path)
			require.False(t, ok)
			require.Empty(t, endpointType)
		})
	}
}

func TestResolveEndpointTypeNormalizesFullURLAndQuery(t *testing.T) {
	require.Equal(t, constant.EndpointTypeOpenAIVideo, ResolveEndpointType(EndpointResolutionInput{
		Path: "https://example.com/v1/videos?model=sora-2",
	}))
	require.Equal(t, constant.EndpointTypeImageGeneration, ResolveEndpointType(EndpointResolutionInput{
		Path: "v1/images/generations?model=qwen-image-plus",
	}))
}

func TestResolveEndpointTypeFallsBackWithoutRequestPath(t *testing.T) {
	require.Equal(t, constant.EndpointTypeOpenAIVideo, ResolveEndpointType(EndpointResolutionInput{
		EndpointType: string(constant.EndpointTypeOpenAIVideo),
		ModelName:    "gpt-4o-mini",
		ChannelType:  constant.ChannelTypeOpenAI,
	}))

	require.Equal(t, constant.EndpointTypeImageGeneration, ResolveEndpointType(EndpointResolutionInput{
		ModelName:              "unknown-model",
		ChannelType:            constant.ChannelTypeOpenAI,
		SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeImageGeneration, constant.EndpointTypeOpenAI},
	}))

	require.Equal(t, constant.EndpointTypeOpenAIVideo, ResolveEndpointType(EndpointResolutionInput{
		ModelName:   "veo_3_1",
		ChannelType: constant.ChannelTypeOpenAI,
	}))

	require.Equal(t, constant.EndpointTypeOpenAI, ResolveEndpointType(EndpointResolutionInput{
		ModelName:   "gpt-4o-mini",
		ChannelType: constant.ChannelTypeOpenAI,
	}))
}

func TestShouldApplyResponseTimeDisableThresholdForEndpoint(t *testing.T) {
	require.False(t, ShouldApplyResponseTimeDisableThresholdForEndpoint(constant.EndpointTypeImageGeneration))
	require.False(t, ShouldApplyResponseTimeDisableThresholdForEndpoint(constant.EndpointTypeOpenAIVideo))

	require.True(t, ShouldApplyResponseTimeDisableThresholdForEndpoint(constant.EndpointTypeOpenAI))
	require.True(t, ShouldApplyResponseTimeDisableThresholdForEndpoint(constant.EndpointTypeOpenAIResponse))
	require.True(t, ShouldApplyResponseTimeDisableThresholdForEndpoint(constant.EndpointTypeEmbeddings))
	require.True(t, ShouldApplyResponseTimeDisableThresholdForEndpoint(constant.EndpointTypeJinaRerank))
}

func TestShouldApplyResponseTimeDisableThresholdChecksAllResolvedEndpoints(t *testing.T) {
	require.False(t, ShouldApplyResponseTimeDisableThreshold(EndpointResolutionInput{
		ModelName:              "custom-image-model",
		ChannelType:            constant.ChannelTypeOpenAI,
		SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeOpenAI, constant.EndpointTypeImageGeneration},
	}))

	require.False(t, ShouldApplyResponseTimeDisableThreshold(EndpointResolutionInput{
		ModelName:              "custom-video-model",
		ChannelType:            constant.ChannelTypeOpenAI,
		SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeOpenAI, constant.EndpointTypeOpenAIVideo},
	}))

	require.True(t, ShouldApplyResponseTimeDisableThreshold(EndpointResolutionInput{
		ModelName:              "custom-text-model",
		ChannelType:            constant.ChannelTypeOpenAI,
		SupportedEndpointTypes: []constant.EndpointType{constant.EndpointTypeOpenAI, constant.EndpointTypeOpenAIResponse},
	}))
}
