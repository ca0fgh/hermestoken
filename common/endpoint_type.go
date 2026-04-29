package common

import (
	"net/url"
	"strings"

	"github.com/ca0fgh/hermestoken/constant"
)

type EndpointResolutionInput struct {
	Path                   string
	EndpointType           string
	ModelName              string
	ChannelType            int
	SupportedEndpointTypes []constant.EndpointType
}

func ResolveEndpointType(input EndpointResolutionInput) constant.EndpointType {
	endpointTypes := ResolveEndpointTypes(input)
	if len(endpointTypes) > 0 {
		return endpointTypes[0]
	}
	return constant.EndpointTypeOpenAI
}

func ResolveEndpointTypes(input EndpointResolutionInput) []constant.EndpointType {
	if endpointType, ok := ResolveEndpointTypeFromPath(input.Path); ok {
		return []constant.EndpointType{endpointType}
	}

	if endpointType := constant.EndpointType(strings.TrimSpace(input.EndpointType)); endpointType != "" {
		return []constant.EndpointType{endpointType}
	}

	if len(input.SupportedEndpointTypes) > 0 {
		return append([]constant.EndpointType(nil), input.SupportedEndpointTypes...)
	}

	endpointTypes := GetEndpointTypesByChannelType(input.ChannelType, input.ModelName)
	if len(endpointTypes) > 0 {
		return endpointTypes
	}

	return []constant.EndpointType{constant.EndpointTypeOpenAI}
}

func ResolveEndpointTypeFromPath(path string) (constant.EndpointType, bool) {
	path = normalizeEndpointPath(path)
	if path == "" {
		return "", false
	}

	switch {
	case endpointPathHasPrefix(path, "/v1/responses/compact"):
		return constant.EndpointTypeOpenAIResponseCompact, true
	case endpointPathHasPrefix(path, "/v1/responses"):
		return constant.EndpointTypeOpenAIResponse, true
	case endpointPathHasPrefix(path, "/v1/images/generations"),
		endpointPathHasPrefix(path, "/v1/images/edits"):
		return constant.EndpointTypeImageGeneration, true
	case endpointPathHasPrefix(path, "/v1/videos"),
		endpointPathHasPrefix(path, "/v1/video/generations"),
		endpointPathHasPrefix(path, "/kling/v1/videos"),
		strings.HasSuffix(path, "/mj/submit/video"):
		return constant.EndpointTypeOpenAIVideo, true
	case endpointPathHasPrefix(path, "/v1/embeddings"),
		strings.HasSuffix(path, "/embeddings"):
		return constant.EndpointTypeEmbeddings, true
	case endpointPathHasPrefix(path, "/v1/rerank"),
		path == "/rerank",
		strings.HasSuffix(path, "/rerank"):
		return constant.EndpointTypeJinaRerank, true
	case endpointPathHasPrefix(path, "/v1/messages"):
		return constant.EndpointTypeAnthropic, true
	case endpointPathHasPrefix(path, "/v1beta/models"),
		endpointPathHasPrefix(path, "/v1/models"):
		return constant.EndpointTypeGemini, true
	case endpointPathHasPrefix(path, "/v1/chat/completions"),
		endpointPathHasPrefix(path, "/pg/chat/completions"),
		endpointPathHasPrefix(path, "/v1/completions"):
		return constant.EndpointTypeOpenAI, true
	}

	return "", false
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

func endpointPathHasPrefix(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
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
