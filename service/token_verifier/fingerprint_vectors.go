package token_verifier

import (
	"context"
	"math"
	"sort"
	"strings"

	"github.com/ca0fgh/hermestoken/common"
)

func cosineSimilarity(a []float64, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	dot := 0.0
	normA := 0.0
	normB := 0.0
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

func pickTopVectorScores(query []float64, refs []EmbeddingReference) []IdentityCandidateSummary {
	if len(query) == 0 || len(refs) == 0 {
		return nil
	}
	familyMax := make(map[string]float64)
	for _, ref := range refs {
		family := strings.TrimSpace(ref.Family)
		if family == "" || len(ref.Embedding) != len(query) {
			continue
		}
		sim := cosineSimilarity(query, ref.Embedding)
		if sim < 0 {
			sim = 0
		}
		if existing, ok := familyMax[family]; !ok || sim > existing {
			familyMax[family] = sim
		}
	}
	if len(familyMax) == 0 {
		return nil
	}
	maxSim := 0.0001
	for _, score := range familyMax {
		if score > maxSim {
			maxSim = score
		}
	}
	items := make([]IdentityCandidateSummary, 0, len(familyMax))
	for family, score := range familyMax {
		items = append(items, IdentityCandidateSummary{
			Family:  family,
			Model:   identityFamilyDisplayName(family),
			Score:   roundScore(score / maxSim),
			Reasons: []string{"embedding cosine similarity"},
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Score == items[j].Score {
			return items[i].Family < items[j].Family
		}
		return items[i].Score > items[j].Score
	})
	return items
}

func embedProbeResponses(ctx context.Context, executor *CurlExecutor, responses map[string]string, config EmbeddingConfig) ([]float64, bool) {
	if executor == nil {
		executor = NewCurlExecutor(0)
	}
	if strings.TrimSpace(config.BaseURL) == "" || strings.TrimSpace(config.APIKey) == "" || strings.TrimSpace(config.ModelID) == "" || len(responses) == 0 {
		return nil, false
	}
	keys := sortedStringKeys(responses)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, "["+key+"] "+truncateRunes(responses[key], 600))
	}
	body := map[string]any{
		"model": config.ModelID,
		"input": strings.Join(parts, "\n\n"),
	}
	payload, _ := common.Marshal(body)
	headers := map[string]string{
		"Authorization": "Bearer " + config.APIKey,
		"Content-Type":  "application/json",
	}
	resp, err := executor.Do(ctx, "POST", endpointWithSuffix(config.BaseURL, "/embeddings"), headers, payload)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false
	}
	var decoded struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
	}
	if err := common.Unmarshal(resp.Body, &decoded); err != nil || len(decoded.Data) == 0 || len(decoded.Data[0].Embedding) == 0 {
		return nil, false
	}
	return decoded.Data[0].Embedding, true
}
