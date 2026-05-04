package token_verifier

import (
	"encoding/json"
	"os"
	"strings"
	"time"
)

type ReportOptions struct {
	IdentityJudge *IdentityJudgeConfig
	Embedding     *EmbeddingConfig
	Executor      *CurlExecutor
}

type IdentityJudgeConfig struct {
	BaseURL string
	APIKey  string
	ModelID string
}

type EmbeddingConfig struct {
	BaseURL    string
	APIKey     string
	ModelID    string
	References []EmbeddingReference
}

type EmbeddingReference struct {
	Family    string    `json:"family"`
	Embedding []float64 `json:"embedding"`
}

func DefaultReportOptionsFromEnv() ReportOptions {
	options := ReportOptions{Executor: NewCurlExecutor(30 * time.Second)}
	if judge := identityJudgeConfigFromEnv(); judge != nil {
		options.IdentityJudge = judge
	}
	if embedding := embeddingConfigFromEnv(); embedding != nil {
		options.Embedding = embedding
	}
	return options
}

func identityJudgeConfigFromEnv() *IdentityJudgeConfig {
	config := &IdentityJudgeConfig{
		BaseURL: strings.TrimSpace(os.Getenv("TOKEN_VERIFIER_IDENTITY_JUDGE_BASE_URL")),
		APIKey:  strings.TrimSpace(os.Getenv("TOKEN_VERIFIER_IDENTITY_JUDGE_API_KEY")),
		ModelID: strings.TrimSpace(os.Getenv("TOKEN_VERIFIER_IDENTITY_JUDGE_MODEL")),
	}
	if config.BaseURL == "" || config.APIKey == "" || config.ModelID == "" {
		return nil
	}
	return config
}

func embeddingConfigFromEnv() *EmbeddingConfig {
	config := &EmbeddingConfig{
		BaseURL: strings.TrimSpace(os.Getenv("TOKEN_VERIFIER_EMBEDDING_BASE_URL")),
		APIKey:  strings.TrimSpace(os.Getenv("TOKEN_VERIFIER_EMBEDDING_API_KEY")),
		ModelID: strings.TrimSpace(os.Getenv("TOKEN_VERIFIER_EMBEDDING_MODEL")),
	}
	if config.BaseURL == "" || config.APIKey == "" || config.ModelID == "" {
		return nil
	}
	refsRaw := strings.TrimSpace(os.Getenv("TOKEN_VERIFIER_EMBEDDING_REFERENCES_JSON"))
	if refsRaw == "" {
		return nil
	}
	var refs []EmbeddingReference
	if err := json.Unmarshal([]byte(refsRaw), &refs); err != nil {
		return nil
	}
	for _, ref := range refs {
		if strings.TrimSpace(ref.Family) != "" && len(ref.Embedding) > 0 {
			config.References = append(config.References, ref)
		}
	}
	if len(config.References) == 0 {
		return nil
	}
	return config
}
