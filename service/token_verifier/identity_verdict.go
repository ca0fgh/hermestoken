package token_verifier

import (
	"fmt"
	"math"
)

const v3HighConfidence = 0.80

type IdentitySignal struct {
	Family string
	Score  float64
}

type IdentitySubmodelSignal struct {
	Family      string
	ModelID     string
	DisplayName string
	Score       float64
}

type IdentityVerdictInput struct {
	ClaimedFamily string
	ClaimedModel  string
	Surface       *IdentitySignal
	Behavior      *IdentitySignal
	V3            *IdentitySubmodelSignal
}

func computeIdentityVerdict(input IdentityVerdictInput) IdentityVerdictSummary {
	signals := usableIdentitySignals(input)
	reasoning := make([]string, 0, len(signals)+2)
	for _, signal := range signals {
		tag := ""
		if input.ClaimedFamily != "" {
			if signal.family == input.ClaimedFamily {
				tag = " matches_claim"
			} else {
				tag = " diverges_from_claim"
			}
		}
		reasoning = append(reasoning, fmt.Sprintf("%s %s: %s (%.0f%%)%s", signal.id, signal.label, signal.family, signal.score*100, tag))
	}
	if len(signals) == 0 {
		return IdentityVerdictSummary{
			Status:         "insufficient_data",
			ConfidenceBand: "low",
			Reasoning:      []string{"no fingerprints available"},
		}
	}

	if len(signals) >= 2 {
		first := signals[0].family
		unanimous := true
		for _, signal := range signals {
			if signal.family != first {
				unanimous = false
				break
			}
		}
		if unanimous {
			trueModel := ""
			if input.V3 != nil && input.V3.Family == first {
				trueModel = input.V3.DisplayName
			}
			if input.ClaimedFamily == "" || input.ClaimedFamily == first {
				if input.V3 != nil && input.V3.Family == first && input.V3.Score >= v3HighConfidence && input.ClaimedModel != "" && modelBareName(input.V3.ModelID) != modelBareName(input.ClaimedModel) {
					reasoning = append(reasoning, fmt.Sprintf("sub-model mismatch: claim=%s v3=%s @%.0f%%", modelBareName(input.ClaimedModel), modelBareName(input.V3.ModelID), input.V3.Score*100))
					return IdentityVerdictSummary{Status: "clean_match_submodel_mismatch", TrueFamily: first, TrueModel: trueModel, ConfidenceBand: confidenceBandForSignals(len(signals)), Reasoning: reasoning}
				}
				if input.V3 != nil && input.V3.Family == first && input.V3.Score >= 0.60 && input.V3.Score < 0.75 {
					reasoning = append(reasoning, fmt.Sprintf("wrapper-hint: V3 sub-model match is borderline (%.0f%%)", input.V3.Score*100))
				}
				return IdentityVerdictSummary{Status: "clean_match", TrueFamily: first, TrueModel: trueModel, ConfidenceBand: confidenceBandForSignals(len(signals)), Reasoning: reasoning}
			}
			return IdentityVerdictSummary{Status: "plain_mismatch", TrueFamily: first, TrueModel: trueModel, ConfidenceBand: confidenceBandForSignals(len(signals)), Reasoning: reasoning}
		}
	}

	if input.ClaimedFamily != "" && len(signals) >= 2 {
		diverging := make([]identityVote, 0)
		matching := make([]identityVote, 0)
		for _, signal := range signals {
			if signal.family == input.ClaimedFamily {
				matching = append(matching, signal)
			} else {
				diverging = append(diverging, signal)
			}
		}
		if len(diverging) == 1 && diverging[0].label == "behavior" && diverging[0].score >= 0.70 {
			return IdentityVerdictSummary{Status: "spoof_selfclaim_forged", TrueFamily: diverging[0].family, SpoofMethod: "selfclaim_forged", ConfidenceBand: "medium", Reasoning: reasoning}
		}
		if len(diverging) == 1 && diverging[0].label == "surface" && diverging[0].score >= 0.50 {
			return IdentityVerdictSummary{Status: "spoof_behavior_induced", TrueFamily: diverging[0].family, SpoofMethod: "behavior_induced", ConfidenceBand: "medium", Reasoning: reasoning}
		}
		if len(diverging) >= 2 {
			first := diverging[0].family
			same := true
			behaviorDiverged := false
			for _, signal := range diverging {
				if signal.family != first {
					same = false
					break
				}
				if signal.label == "behavior" {
					behaviorDiverged = true
				}
			}
			if same {
				status := "spoof_behavior_induced"
				method := "behavior_induced"
				if behaviorDiverged {
					status = "spoof_selfclaim_forged"
					method = "selfclaim_forged"
				}
				trueModel := ""
				if input.V3 != nil && input.V3.Family == first {
					trueModel = input.V3.DisplayName
				}
				confidence := "medium"
				if len(diverging) >= 3 || len(matching) == 0 {
					confidence = "high"
				}
				return IdentityVerdictSummary{Status: status, TrueFamily: first, TrueModel: trueModel, SpoofMethod: method, ConfidenceBand: confidence, Reasoning: reasoning}
			}
		}
		return IdentityVerdictSummary{Status: "ambiguous", ConfidenceBand: "low", Reasoning: reasoning}
	}

	return IdentityVerdictSummary{Status: "insufficient_data", ConfidenceBand: "low", Reasoning: reasoning}
}

type identityVote struct {
	id     string
	label  string
	family string
	score  float64
}

func usableIdentitySignals(input IdentityVerdictInput) []identityVote {
	signals := make([]identityVote, 0, 3)
	if input.Surface != nil && input.Surface.Family != "" && input.Surface.Score >= 0.30 {
		signals = append(signals, identityVote{id: "surface", label: "surface", family: input.Surface.Family, score: input.Surface.Score})
	}
	if input.Behavior != nil && input.Behavior.Family != "" && input.Behavior.Score >= 0.40 {
		signals = append(signals, identityVote{id: "behavior", label: "behavior", family: input.Behavior.Family, score: input.Behavior.Score})
	}
	if input.V3 != nil && input.V3.Family != "" && input.V3.Score >= 0.50 {
		signals = append(signals, identityVote{id: "v3", label: "v3", family: input.V3.Family, score: input.V3.Score})
	}
	return signals
}

func confidenceBandForSignals(count int) string {
	if count >= 3 {
		return "high"
	}
	return "medium"
}

func shouldAbstainSubModel(topFamily string, topFamilyScore float64, secondFamilyScore float64, claimedFamily string) bool {
	if topFamilyScore < 0.70 && topFamilyScore-secondFamilyScore < 0.30 {
		return true
	}
	return claimedFamily != "" && topFamily != claimedFamily && topFamilyScore < 0.85
}

func applySubmodelAbstainGuard(submodels []SubmodelAssessmentSummary, candidates []IdentityCandidateSummary, claimedFamily string) []SubmodelAssessmentSummary {
	if len(submodels) == 0 || len(candidates) == 0 {
		return submodels
	}
	top := candidates[0]
	secondScore := 0.0
	if len(candidates) > 1 {
		secondScore = candidates[1].Score
	}
	if !shouldAbstainSubModel(top.Family, top.Score, secondScore, claimedFamily) {
		return submodels
	}
	guarded := make([]SubmodelAssessmentSummary, len(submodels))
	for i, submodel := range submodels {
		guarded[i] = submodel
		if submodel.Abstained {
			continue
		}
		guarded[i].Family = ""
		guarded[i].ModelID = ""
		guarded[i].DisplayName = ""
		guarded[i].Score = 0
		guarded[i].Abstained = true
		guarded[i].Evidence = uniqueStrings(append(guarded[i].Evidence, "family_confidence_abstain=true"))
	}
	return guarded
}

func surfaceIdentitySignal(responses map[CheckKey]string) *IdentitySignal {
	features := extractSourceSelfClaim(sourceResponseIDMap(responses))
	bestFamily := ""
	bestScore := 0.0
	for family, score := range features {
		if family == "vague" || score <= bestScore {
			continue
		}
		bestFamily = family
		bestScore = score
	}
	if bestFamily == "" {
		return nil
	}
	return &IdentitySignal{Family: bestFamily, Score: math.Min(1, bestScore)}
}

func behaviorIdentitySignalWithoutSelfClaim(results []CheckResult, responses map[CheckKey]string) *IdentitySignal {
	baselines := loadSourceFamilyBaselines()
	if len(baselines) == 0 {
		return nil
	}
	features := extractSourceFingerprintFeatures(results, responses)
	features["selfClaim"] = map[string]float64{}
	candidates := sourceBehavioralIdentityCandidatesFromFeatures(features, baselines)
	if len(candidates) == 0 {
		return nil
	}
	return &IdentitySignal{Family: candidates[0].Family, Score: candidates[0].Score}
}

func v3IdentitySignal(submodels []SubmodelAssessmentSummary) *IdentitySubmodelSignal {
	for _, submodel := range submodels {
		if submodel.Method == "v3" && !submodel.Abstained && submodel.Family != "" {
			return &IdentitySubmodelSignal{
				Family:      submodel.Family,
				ModelID:     submodel.ModelID,
				DisplayName: submodel.DisplayName,
				Score:       submodel.Score,
			}
		}
	}
	return nil
}
