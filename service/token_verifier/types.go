package token_verifier

type CheckKey string

const (
	ProviderOpenAI    = "openai"
	ProviderAnthropic = "anthropic"

	CheckAvailability    CheckKey = "availability"
	CheckModelAccess     CheckKey = "model_access"
	CheckStream          CheckKey = "stream_support"
	CheckJSON            CheckKey = "json_stability"
	CheckModelsList      CheckKey = "models_list"
	CheckModelIdentity   CheckKey = "model_identity"
	CheckReproducibility CheckKey = "reproducibility"
)

// Consistency method labels for CheckReproducibility results.
const (
	ConsistencyMethodSystemFingerprint        = "system_fingerprint"
	ConsistencyMethodSystemFingerprintChanged = "system_fingerprint_changed"
	ConsistencyMethodContentHash              = "content_hash"
	ConsistencyMethodContentDiverged          = "content_diverged"
	ConsistencyMethodInsufficientData         = "insufficient_data"
)

type CheckResult struct {
	Provider           string         `json:"provider"`
	CheckKey           CheckKey       `json:"check_key"`
	ModelName          string         `json:"model_name,omitempty"`
	ClaimedModel       string         `json:"claimed_model,omitempty"`
	ObservedModel      string         `json:"observed_model,omitempty"`
	IdentityConfidence int            `json:"identity_confidence,omitempty"`
	SuspectedDowngrade bool           `json:"suspected_downgrade,omitempty"`
	Consistent         bool           `json:"consistent,omitempty"`
	ConsistencyMethod  string         `json:"consistency_method,omitempty"`
	Skipped            bool           `json:"skipped,omitempty"`
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
	Provider       string            `json:"provider"`
	ModelName      string            `json:"model_name"`
	Available      bool              `json:"available"`
	LatencyMs      int64             `json:"latency_ms,omitempty"`
	StreamTTFTMs   int64             `json:"stream_ttft_ms,omitempty"`
	StreamTokensPS float64           `json:"stream_tokens_ps,omitempty"`
	Message        string            `json:"message,omitempty"`
	Baseline       *ModelBaselineRef `json:"baseline,omitempty"`
}

// ModelBaselineRef captures comparison against an external public baseline (Artificial Analysis P50).
// TTFTRatio = measured_ttft / baseline_ttft (lower is better).
// TPSRatio  = measured_tps  / baseline_tps  (higher is better).
type ModelBaselineRef struct {
	Source         string  `json:"source"`
	Slug           string  `json:"slug,omitempty"`
	BaselineTTFTMs int64   `json:"baseline_ttft_ms"`
	BaselineTPS    float64 `json:"baseline_tokens_ps"`
	TTFTRatio      float64 `json:"ttft_ratio,omitempty"`
	TPSRatio       float64 `json:"tps_ratio,omitempty"`
	Note           string  `json:"note,omitempty"`
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
	Consistent         bool    `json:"consistent,omitempty"`
	ConsistencyMethod  string  `json:"consistency_method,omitempty"`
	Skipped            bool    `json:"skipped,omitempty"`
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

// ReproducibilitySummary captures whether two identical seeded probes returned
// the same response (preferred via system_fingerprint, fallback via content hash).
// Skipped is true on providers without seed support (e.g. Anthropic) — in that
// case the result is informational and excluded from stability scoring.
type ReproducibilitySummary struct {
	Provider   string `json:"provider"`
	ModelName  string `json:"model_name"`
	Consistent bool   `json:"consistent"`
	Method     string `json:"method,omitempty"`
	Skipped    bool   `json:"skipped"`
	Message    string `json:"message,omitempty"`
}

type FinalRating struct {
	Score      int            `json:"score"`
	Grade      string         `json:"grade"`
	Conclusion string         `json:"conclusion"`
	Dimensions map[string]int `json:"dimensions"`
	Risks      []string       `json:"risks"`
}

type ReportSummary struct {
	Score           int                      `json:"score"`
	Grade           string                   `json:"grade"`
	Conclusion      string                   `json:"conclusion"`
	Dimensions      map[string]int           `json:"dimensions"`
	Checklist       []ChecklistItem          `json:"checklist"`
	Models          []ModelSummary           `json:"models"`
	ModelIdentity   []ModelIdentitySummary   `json:"model_identity"`
	Reproducibility []ReproducibilitySummary `json:"reproducibility"`
	Metrics         map[string]float64       `json:"metrics"`
	Risks           []string                 `json:"risks"`
	FinalRating     FinalRating              `json:"final_rating"`
	ScoringVersion  string                   `json:"scoring_version"`
	BaselineSource  string                   `json:"baseline_source"`
}
