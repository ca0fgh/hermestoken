package token_verifier

import (
	"sort"
	"strings"
)

const (
	fusionRuleWeight   = 0.4
	fusionJudgeWeight  = 0.4
	fusionVectorWeight = 0.2
)

func fuseIdentityCandidates(rule []IdentityCandidateSummary, judge []IdentityCandidateSummary, vector []IdentityCandidateSummary) []IdentityCandidateSummary {
	if len(rule) == 0 && len(judge) == 0 && len(vector) == 0 {
		return nil
	}
	wRule := fusionRuleWeight
	wJudge := 0.0
	wVector := 0.0
	if len(judge) > 0 {
		wJudge = fusionJudgeWeight
	}
	if len(vector) > 0 {
		wVector = fusionVectorWeight
	}
	total := wRule + wJudge + wVector
	if total <= 0 {
		return nil
	}
	wRule /= total
	wJudge /= total
	wVector /= total

	type fusedCandidate struct {
		family  string
		model   string
		score   float64
		reasons []string
	}
	byFamily := make(map[string]*fusedCandidate)
	add := func(items []IdentityCandidateSummary, weight float64, source string) {
		if weight <= 0 {
			return
		}
		for _, item := range items {
			family := strings.TrimSpace(item.Family)
			if family == "" || item.Score <= 0 {
				continue
			}
			existing, ok := byFamily[family]
			if !ok {
				existing = &fusedCandidate{family: family, model: firstNonEmptyString(item.Model, identityFamilyDisplayName(family))}
				byFamily[family] = existing
			}
			existing.score += weight * item.Score
			if existing.model == "" {
				existing.model = item.Model
			}
			for _, reason := range item.Reasons {
				if strings.TrimSpace(reason) != "" {
					existing.reasons = append(existing.reasons, source+": "+reason)
				}
			}
		}
	}
	add(rule, wRule, "rule")
	add(judge, wJudge, "judge")
	add(vector, wVector, "vector")

	items := make([]*fusedCandidate, 0, len(byFamily))
	for _, item := range byFamily {
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].score == items[j].score {
			return items[i].family < items[j].family
		}
		return items[i].score > items[j].score
	})
	if len(items) > 3 {
		items = items[:3]
	}
	out := make([]IdentityCandidateSummary, 0, len(items))
	for _, item := range items {
		out = append(out, IdentityCandidateSummary{
			Family:  item.family,
			Model:   firstNonEmptyString(item.model, identityFamilyDisplayName(item.family)),
			Score:   sourceCandidateScore(item.score),
			Reasons: uniqueStrings(firstNStrings(item.reasons, 5)),
		})
	}
	return out
}
