package dto

import "github.com/ca0fgh/hermestoken/constant"

// 这里不好动就不动了，本来想独立出来的（
type OpenAIModels struct {
	Id                     string                  `json:"id"`
	Object                 string                  `json:"object"`
	Created                int                     `json:"created"`
	OwnedBy                string                  `json:"owned_by"`
	SupportedEndpointTypes []constant.EndpointType `json:"supported_endpoint_types"`
	ModelName              string                  `json:"model_name,omitempty"`
	QuotaType              string                  `json:"quota_type,omitempty"`
	BillingMode            string                  `json:"billing_mode,omitempty"`
	BillingExpr            string                  `json:"billing_expr,omitempty"`
	ModelPrice             float64                 `json:"model_price,omitempty"`
	ModelRatio             float64                 `json:"model_ratio,omitempty"`
	CompletionRatio        float64                 `json:"completion_ratio,omitempty"`
	CacheRatio             *float64                `json:"cache_ratio,omitempty"`
	CreateCacheRatio       *float64                `json:"create_cache_ratio,omitempty"`
	InputPricePerMTok      float64                 `json:"input_price_per_mtok,omitempty"`
	OutputPricePerMTok     float64                 `json:"output_price_per_mtok,omitempty"`
	CacheReadPricePerMTok  *float64                `json:"cache_read_price_per_mtok,omitempty"`
	CacheWritePricePerMTok *float64                `json:"cache_write_price_per_mtok,omitempty"`
	TaskPerRequestPrice    float64                 `json:"task_per_request_price,omitempty"`
	TaskPerSecondPrice     float64                 `json:"task_per_second_price,omitempty"`
	Configured             bool                    `json:"configured,omitempty"`
}

type AnthropicModel struct {
	ID          string `json:"id"`
	CreatedAt   string `json:"created_at"`
	DisplayName string `json:"display_name"`
	Type        string `json:"type"`
}

type GeminiModel struct {
	Name                       interface{}   `json:"name"`
	BaseModelId                interface{}   `json:"baseModelId"`
	Version                    interface{}   `json:"version"`
	DisplayName                interface{}   `json:"displayName"`
	Description                interface{}   `json:"description"`
	InputTokenLimit            interface{}   `json:"inputTokenLimit"`
	OutputTokenLimit           interface{}   `json:"outputTokenLimit"`
	SupportedGenerationMethods []interface{} `json:"supportedGenerationMethods"`
	Thinking                   interface{}   `json:"thinking"`
	Temperature                interface{}   `json:"temperature"`
	MaxTemperature             interface{}   `json:"maxTemperature"`
	TopP                       interface{}   `json:"topP"`
	TopK                       interface{}   `json:"topK"`
}
