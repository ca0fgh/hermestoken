package common

import "strings"

var (
	// OpenAIResponseOnlyModels is a list of models that are only available for OpenAI responses.
	OpenAIResponseOnlyModels = []string{
		"o3-pro",
		"o3-deep-research",
		"o4-mini-deep-research",
	}
	ImageGenerationModels = []string{
		"dall-e-",
		"gpt-image-",
		"prefix:imagen-",
		"qwen-image",
		"image-preview",
		"image-edit",
		"seedream",
		"flux-",
		"flux.1-",
	}
	VideoGenerationModels = []string{
		"prefix:veo",
		"sora-",
		"grok-imagine",
		"kling",
		"hailuo",
		"wan2",
		"vidu",
		"cogvideo",
		"seedance",
		"runway",
		"pika",
		"video",
	}
	OpenAITextModels = []string{
		"gpt-",
		"o1",
		"o3",
		"o4",
		"chatgpt",
	}
)

func IsOpenAIResponseOnlyModel(modelName string) bool {
	for _, m := range OpenAIResponseOnlyModels {
		if strings.Contains(modelName, m) {
			return true
		}
	}
	return false
}

func IsImageGenerationModel(modelName string) bool {
	return matchesModelNameRules(modelName, ImageGenerationModels)
}

func IsVideoGenerationModel(modelName string) bool {
	return matchesModelNameRules(modelName, VideoGenerationModels)
}

func IsOpenAITextModel(modelName string) bool {
	modelName = strings.ToLower(modelName)
	for _, m := range OpenAITextModels {
		if strings.Contains(modelName, m) {
			return true
		}
	}
	return false
}

func matchesModelNameRules(modelName string, rules []string) bool {
	modelName = strings.ToLower(strings.TrimSpace(modelName))
	if modelName == "" {
		return false
	}
	for _, rule := range rules {
		rule = strings.ToLower(strings.TrimSpace(rule))
		if rule == "" {
			continue
		}
		if strings.HasPrefix(rule, "prefix:") {
			if strings.HasPrefix(modelName, strings.TrimPrefix(rule, "prefix:")) {
				return true
			}
			continue
		}
		if strings.Contains(modelName, rule) {
			return true
		}
	}
	return false
}
