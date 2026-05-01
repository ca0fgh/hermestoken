package token_verifier

type CheckKey string

const (
	ProviderOpenAI    = "openai"
	ProviderAnthropic = "anthropic"

	CheckAvailability  CheckKey = "availability"
	CheckModelAccess   CheckKey = "model_access"
	CheckStream        CheckKey = "stream_support"
	CheckJSON          CheckKey = "json_stability"
	CheckModelsList    CheckKey = "models_list"
	CheckModelIdentity CheckKey = "model_identity"
)

type CheckResult struct {
	Provider           string         `json:"provider"`
	CheckKey           CheckKey       `json:"check_key"`
	ModelName          string         `json:"model_name,omitempty"`
	ClaimedModel       string         `json:"claimed_model,omitempty"`
	ObservedModel      string         `json:"observed_model,omitempty"`
	IdentityConfidence int            `json:"identity_confidence,omitempty"`
	SuspectedDowngrade bool           `json:"suspected_downgrade,omitempty"`
	Success            bool           `json:"success"`
	Score              int            `json:"score"`
	LatencyMs          int64          `json:"latency_ms,omitempty"`
	TTFTMs             int64          `json:"ttft_ms,omitempty"`
	TokensPS           float64        `json:"tokens_ps,omitempty"`
	ErrorCode          string         `json:"error_code,omitempty"`
	Message            string         `json:"message,omitempty"`
	Raw                map[string]any `json:"raw,omitempty"`
}

type ModelSummary struct {
	Provider  string `json:"provider"`
	ModelName string `json:"model_name"`
	Available bool   `json:"available"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
	Message   string `json:"message,omitempty"`
}

type ChecklistItem struct {
	Provider           string  `json:"provider"`
	CheckKey           string  `json:"check_key"`
	CheckName          string  `json:"check_name"`
	ModelName          string  `json:"model_name,omitempty"`
	ClaimedModel       string  `json:"claimed_model,omitempty"`
	ObservedModel      string  `json:"observed_model,omitempty"`
	IdentityConfidence int     `json:"identity_confidence,omitempty"`
	SuspectedDowngrade bool    `json:"suspected_downgrade,omitempty"`
	Passed             bool    `json:"passed"`
	Status             string  `json:"status"`
	Score              int     `json:"score"`
	LatencyMs          int64   `json:"latency_ms,omitempty"`
	TTFTMs             int64   `json:"ttft_ms,omitempty"`
	TokensPS           float64 `json:"tokens_ps,omitempty"`
	ErrorCode          string  `json:"error_code,omitempty"`
	Message            string  `json:"message,omitempty"`
}

type ModelIdentitySummary struct {
	Provider           string `json:"provider"`
	ClaimedModel       string `json:"claimed_model"`
	ObservedModel      string `json:"observed_model,omitempty"`
	IdentityConfidence int    `json:"identity_confidence"`
	SuspectedDowngrade bool   `json:"suspected_downgrade"`
	Message            string `json:"message,omitempty"`
}

type FinalRating struct {
	Score      int            `json:"score"`
	Grade      string         `json:"grade"`
	Conclusion string         `json:"conclusion"`
	Dimensions map[string]int `json:"dimensions"`
	Risks      []string       `json:"risks"`
}

type ReportSummary struct {
	Score         int                    `json:"score"`
	Grade         string                 `json:"grade"`
	Conclusion    string                 `json:"conclusion"`
	Dimensions    map[string]int         `json:"dimensions"`
	Checklist     []ChecklistItem        `json:"checklist"`
	Models        []ModelSummary         `json:"models"`
	ModelIdentity []ModelIdentitySummary `json:"model_identity"`
	Metrics       map[string]float64     `json:"metrics"`
	Risks         []string               `json:"risks"`
	FinalRating   FinalRating            `json:"final_rating"`
}
