package common

import (
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/constant"
)

type EndpointResolutionInput struct {
	Method                 string
	Path                   string
	EndpointType           string
	ModelName              string
	ChannelType            int
	SupportedEndpointTypes []constant.EndpointType
}

func ResolveEndpointType(input EndpointResolutionInput) constant.EndpointType {
	if endpointType, ok := ResolveEndpointTypeFromPath(input.Path); ok {
		return endpointType
	}

	if endpointType := constant.EndpointType(strings.TrimSpace(input.EndpointType)); endpointType != "" {
		return endpointType
	}

	if len(input.SupportedEndpointTypes) > 0 {
		return input.SupportedEndpointTypes[0]
	}

	endpointTypes := GetEndpointTypesByChannelType(input.ChannelType, input.ModelName)
	if len(endpointTypes) > 0 {
		return endpointTypes[0]
	}

	return constant.EndpointTypeOpenAI
}

func ResolveEndpointTypeFromPath(path string) (constant.EndpointType, bool) {
	path = normalizeEndpointPath(path)
	if path == "" {
		return "", false
	}

	switch {
	case strings.HasPrefix(path, "/v1/responses/compact"):
		return constant.EndpointTypeOpenAIResponseCompact, true
	case strings.HasPrefix(path, "/v1/responses"):
		return constant.EndpointTypeOpenAIResponse, true
	case strings.HasPrefix(path, "/v1/images/generations"),
		strings.HasPrefix(path, "/v1/images/edits"):
		return constant.EndpointTypeImageGeneration, true
	case strings.HasPrefix(path, "/v1/videos"),
		strings.HasPrefix(path, "/v1/video/generations"),
		strings.HasPrefix(path, "/kling/v1/videos"),
		strings.HasSuffix(path, "/mj/submit/video"):
		return constant.EndpointTypeOpenAIVideo, true
	case strings.HasPrefix(path, "/v1/embeddings"),
		strings.HasSuffix(path, "/embeddings"):
		return constant.EndpointTypeEmbeddings, true
	case strings.HasPrefix(path, "/v1/rerank"),
		path == "/rerank",
		strings.HasSuffix(path, "/rerank"):
		return constant.EndpointTypeJinaRerank, true
	case strings.HasPrefix(path, "/v1/messages"):
		return constant.EndpointTypeAnthropic, true
	case strings.HasPrefix(path, "/v1beta/models"),
		strings.HasPrefix(path, "/v1/models"):
		return constant.EndpointTypeGemini, true
	case strings.HasPrefix(path, "/v1/chat/completions"),
		strings.HasPrefix(path, "/pg/chat/completions"),
		strings.HasPrefix(path, "/v1/completions"):
		return constant.EndpointTypeOpenAI, true
	}

	return "", false
}

func ShouldApplyResponseTimeDisableThresholdForEndpoint(endpointType constant.EndpointType) bool {
	switch endpointType {
	case constant.EndpointTypeImageGeneration, constant.EndpointTypeOpenAIVideo:
		return false
	default:
		return true
	}
}

func normalizeEndpointPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if parsedURL, err := url.Parse(path); err == nil && parsedURL.Path != "" {
		path = parsedURL.Path
	}
	if idx := strings.Index(path, "?"); idx >= 0 {
		path = path[:idx]
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

// GetEndpointTypesByChannelType 获取渠道最优先端点类型（所有的渠道都支持 OpenAI 端点）
func GetEndpointTypesByChannelType(channelType int, modelName string) []constant.EndpointType {
	var endpointTypes []constant.EndpointType
	switch channelType {
	case constant.ChannelTypeJina:
		endpointTypes = []constant.EndpointType{constant.EndpointTypeJinaRerank}
	//case constant.ChannelTypeMidjourney, constant.ChannelTypeMidjourneyPlus:
	//	endpointTypes = []constant.EndpointType{constant.EndpointTypeMidjourney}
	//case constant.ChannelTypeSunoAPI:
	//	endpointTypes = []constant.EndpointType{constant.EndpointTypeSuno}
	//case constant.ChannelTypeKling:
	//	endpointTypes = []constant.EndpointType{constant.EndpointTypeKling}
	//case constant.ChannelTypeJimeng:
	//	endpointTypes = []constant.EndpointType{constant.EndpointTypeJimeng}
	case constant.ChannelTypeAws:
		fallthrough
	case constant.ChannelTypeAnthropic:
		endpointTypes = []constant.EndpointType{constant.EndpointTypeAnthropic, constant.EndpointTypeOpenAI}
	case constant.ChannelTypeVertexAi:
		fallthrough
	case constant.ChannelTypeGemini:
		endpointTypes = []constant.EndpointType{constant.EndpointTypeGemini, constant.EndpointTypeOpenAI}
	case constant.ChannelTypeOpenRouter: // OpenRouter 只支持 OpenAI 端点
		endpointTypes = []constant.EndpointType{constant.EndpointTypeOpenAI}
	case constant.ChannelTypeXai:
		endpointTypes = []constant.EndpointType{constant.EndpointTypeOpenAI, constant.EndpointTypeOpenAIResponse}
	case constant.ChannelTypeSora:
		endpointTypes = []constant.EndpointType{constant.EndpointTypeOpenAIVideo}
	default:
		if IsOpenAIResponseOnlyModel(modelName) {
			endpointTypes = []constant.EndpointType{constant.EndpointTypeOpenAIResponse}
		} else {
			endpointTypes = []constant.EndpointType{constant.EndpointTypeOpenAI}
		}
	}
	if IsImageGenerationModel(modelName) {
		endpointTypes = prependEndpointType(endpointTypes, constant.EndpointTypeImageGeneration)
	}
	if IsVideoGenerationModel(modelName) {
		endpointTypes = prependEndpointType(endpointTypes, constant.EndpointTypeOpenAIVideo)
	}
	return endpointTypes
}

func prependEndpointType(endpointTypes []constant.EndpointType, endpointType constant.EndpointType) []constant.EndpointType {
	for _, current := range endpointTypes {
		if current == endpointType {
			return endpointTypes
		}
	}
	return append([]constant.EndpointType{endpointType}, endpointTypes...)
}
