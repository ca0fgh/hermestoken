package token_verifier

import "strings"

const (
	// Earlier reports without a scoring_version field are implicitly v1.
	ScoringVersionV2 = "v2" // AA-baseline-aware scoring.
	ScoringVersionV3 = "v3" // Adds reproducibility checks to stability/risk reporting.

	BaselineSourceAA       = "artificial_analysis"
	BaselineSourceFallback = "fallback_absolute"
	BaselineSourceMixed    = "mixed"
	BaselineSourceNone     = "none"
)

type modelPerfKey struct {
	provider string
	model    string
}

type modelPerfData struct {
	streamTTFTMs   int64
	streamLatency  int64
	streamTokensPS float64
	hasStream      bool
}

func BuildReport(results []CheckResult) ReportSummary {
	dimensions := map[string]int{
		"availability":   0,
		"model_access":   0,
		"model_identity": 0,
		"stability":      0,
		"performance":    0,
		"stream":         0,
		"json":           0,
	}

	totalChecks := 0
	successChecks := 0
	modelChecks := 0
	modelSuccess := 0
	identityChecks := 0
	identityScoreTotal := 0
	latencyTotal := int64(0)
	latencyCount := 0
	ttftTotal := int64(0)
	ttftCount := 0
	tokensPSTotal := 0.0
	tokensPSCount := 0
	models := make([]ModelSummary, 0)
	risks := make([]string, 0)
	checklist := make([]ChecklistItem, 0, len(results))
	modelIdentity := make([]ModelIdentitySummary, 0)
	reproducibility := make([]ReproducibilitySummary, 0)
	perf := make(map[modelPerfKey]*modelPerfData)

	for _, result := range results {
		checklist = append(checklist, buildChecklistItem(result))
		if !result.Skipped {
			totalChecks++
			if result.Success {
				successChecks++
			}
		}
		if result.LatencyMs > 0 {
			latencyTotal += result.LatencyMs
			latencyCount++
		}
		if result.TTFTMs > 0 {
			ttftTotal += result.TTFTMs
			ttftCount++
		}
		if result.TokensPS > 0 {
			tokensPSTotal += result.TokensPS
			tokensPSCount++
		}

		switch result.CheckKey {
		case CheckAvailability:
			if result.Success {
				dimensions["availability"] = 20
			}
		case CheckModelAccess:
			modelChecks++
			if result.Success {
				modelSuccess++
			}
			models = append(models, ModelSummary{
				Provider:  result.Provider,
				ModelName: result.ModelName,
				Available: result.Success,
				LatencyMs: result.LatencyMs,
				Message:   result.Message,
			})
		case CheckModelIdentity:
			identityChecks++
			identityScoreTotal += result.IdentityConfidence
			modelIdentity = append(modelIdentity, ModelIdentitySummary{
				Provider:           result.Provider,
				ClaimedModel:       result.ClaimedModel,
				ObservedModel:      result.ObservedModel,
				IdentityConfidence: result.IdentityConfidence,
				SuspectedDowngrade: result.SuspectedDowngrade,
				Message:            result.Message,
			})
			if result.SuspectedDowngrade {
				risks = append(risks, "模型响应与声明不一致，疑似存在降级或替换")
			}
		case CheckStream:
			if result.Success {
				dimensions["stream"] = 10
				key := modelPerfKey{provider: result.Provider, model: result.ModelName}
				if _, ok := perf[key]; !ok {
					perf[key] = &modelPerfData{}
				}
				perf[key].streamTTFTMs = result.TTFTMs
				perf[key].streamLatency = result.LatencyMs
				perf[key].streamTokensPS = result.TokensPS
				perf[key].hasStream = true
			}
		case CheckJSON:
			if result.Success {
				dimensions["json"] = 5
			}
		case CheckReproducibility:
			reproducibility = append(reproducibility, ReproducibilitySummary{
				Provider:   result.Provider,
				ModelName:  result.ModelName,
				Consistent: result.Consistent,
				Method:     result.ConsistencyMethod,
				Skipped:    result.Skipped,
				Message:    result.Message,
			})
			if !result.Skipped && !result.Consistent && result.ConsistencyMethod == ConsistencyMethodSystemFingerprintChanged {
				risks = append(risks, "上游 system_fingerprint 在两次相同 seed 请求之间发生变化，疑似路由抖动或模型替换")
			}
		}

		if !result.Success && result.Message != "" {
			message := strings.ToLower(result.Message)
			switch {
			case strings.Contains(message, "rate limit") || strings.Contains(message, "429"):
				risks = append(risks, "检测过程中出现限流风险")
			case strings.Contains(message, "timeout") || strings.Contains(message, "deadline"):
				risks = append(risks, "检测过程中出现超时风险")
			case strings.Contains(message, "insufficient") || strings.Contains(message, "quota"):
				risks = append(risks, "Token 额度可能不足")
			}
		}
	}

	if modelChecks > 0 {
		dimensions["model_access"] = int(float64(modelSuccess) / float64(modelChecks) * 20)
	}
	if identityChecks > 0 {
		dimensions["model_identity"] = int(float64(identityScoreTotal) / float64(identityChecks) * 15 / 100)
	}
	if totalChecks > 0 {
		dimensions["stability"] = int(float64(successChecks) / float64(totalChecks) * 15)
	}

	enrichModelsWithPerfAndBaseline(models, perf)
	perfScore, baselineSource, perfMetrics := computePerformanceDimension(models, latencyTotal, latencyCount)
	dimensions["performance"] = perfScore

	score := 0
	for _, value := range dimensions {
		score += value
	}
	if score > 100 {
		score = 100
	}
	grade := gradeForScore(score)

	metrics := map[string]float64{}
	if latencyCount > 0 {
		metrics["avg_latency_ms"] = float64(latencyTotal) / float64(latencyCount)
	}
	if ttftCount > 0 {
		metrics["avg_ttft_ms"] = float64(ttftTotal) / float64(ttftCount)
	}
	if tokensPSCount > 0 {
		metrics["avg_tokens_per_second"] = tokensPSTotal / float64(tokensPSCount)
	}
	for k, v := range perfMetrics {
		metrics[k] = v
	}

	uniqueRisks := uniqueStrings(risks)
	conclusion := conclusionForGrade(grade)
	return ReportSummary{
		Score:           score,
		Grade:           grade,
		Conclusion:      conclusion,
		Dimensions:      dimensions,
		Checklist:       checklist,
		Models:          models,
		ModelIdentity:   modelIdentity,
		Reproducibility: reproducibility,
		Metrics:         metrics,
		Risks:           uniqueRisks,
		ScoringVersion:  ScoringVersionV3,
		BaselineSource:  baselineSource,
		FinalRating: FinalRating{
			Score:      score,
			Grade:      grade,
			Conclusion: conclusion,
			Dimensions: dimensions,
			Risks:      uniqueRisks,
		},
	}
}

// enrichModelsWithPerfAndBaseline mutates models in place: attaches stream-derived TTFT/TPS metrics
// and looks up the AA baseline for each model when available.
func enrichModelsWithPerfAndBaseline(models []ModelSummary, perf map[modelPerfKey]*modelPerfData) {
	for i := range models {
		key := modelPerfKey{provider: models[i].Provider, model: models[i].ModelName}
		if data, ok := perf[key]; ok && data.hasStream {
			models[i].StreamTTFTMs = data.streamTTFTMs
			models[i].StreamTokensPS = data.streamTokensPS
		}
		baseline := LookupAABaseline(models[i].ModelName)
		if baseline == nil {
			continue
		}
		ref := &ModelBaselineRef{
			Source:         BaselineSourceAA,
			Slug:           baseline.Slug,
			BaselineTTFTMs: int64(baseline.TTFTSec * 1000),
			BaselineTPS:    baseline.OutputTPS,
		}
		if models[i].StreamTTFTMs > 0 && ref.BaselineTTFTMs > 0 {
			ref.TTFTRatio = float64(models[i].StreamTTFTMs) / float64(ref.BaselineTTFTMs)
		}
		if models[i].StreamTokensPS > 0 && ref.BaselineTPS > 0 {
			ref.TPSRatio = models[i].StreamTokensPS / ref.BaselineTPS
		}
		if ref.TTFTRatio == 0 && ref.TPSRatio == 0 {
			ref.Note = "未采集到流式性能数据，基线仅供参考"
		}
		models[i].Baseline = ref
	}
}

// computePerformanceDimension produces the performance dimension score in [0,15] plus a baseline source label.
// When at least one tested model has an AA baseline AND stream perf data, scoring is ratio-based.
// Otherwise it falls back to the legacy absolute-latency ladder.
func computePerformanceDimension(models []ModelSummary, latencyTotal int64, latencyCount int) (int, string, map[string]float64) {
	metrics := make(map[string]float64)
	withBaseline := 0
	withoutBaseline := 0
	scoreSum := 0.0
	ttftRatioSum := 0.0
	ttftRatioCount := 0
	tpsRatioSum := 0.0
	tpsRatioCount := 0

	for _, m := range models {
		if m.Baseline == nil || m.Baseline.Source != BaselineSourceAA {
			if m.LatencyMs > 0 || m.StreamTTFTMs > 0 {
				withoutBaseline++
			}
			continue
		}
		if m.Baseline.TTFTRatio == 0 && m.Baseline.TPSRatio == 0 {
			withoutBaseline++
			continue
		}
		withBaseline++
		scoreSum += scorePerModelAgainstBaseline(m.Baseline)
		if m.Baseline.TTFTRatio > 0 {
			ttftRatioSum += m.Baseline.TTFTRatio
			ttftRatioCount++
		}
		if m.Baseline.TPSRatio > 0 {
			tpsRatioSum += m.Baseline.TPSRatio
			tpsRatioCount++
		}
	}

	if ttftRatioCount > 0 {
		metrics["aa_ttft_ratio_avg"] = ttftRatioSum / float64(ttftRatioCount)
	}
	if tpsRatioCount > 0 {
		metrics["aa_tps_ratio_avg"] = tpsRatioSum / float64(tpsRatioCount)
	}

	if withBaseline > 0 {
		avg := scoreSum / float64(withBaseline)
		score := int(avg + 0.5)
		if score > 15 {
			score = 15
		}
		if score < 0 {
			score = 0
		}
		source := BaselineSourceAA
		if withoutBaseline > 0 {
			source = BaselineSourceMixed
		}
		return score, source, metrics
	}

	if latencyCount == 0 {
		return 0, BaselineSourceNone, metrics
	}
	avgLatency := latencyTotal / int64(latencyCount)
	return ladderPerformanceScore(avgLatency), BaselineSourceFallback, metrics
}

// scorePerModelAgainstBaseline maps TTFT and TPS ratios to a per-model score in [0,15].
// TTFT is weighted equally with TPS (7.5 each).
func scorePerModelAgainstBaseline(ref *ModelBaselineRef) float64 {
	return ttftSubScore(ref.TTFTRatio) + tpsSubScore(ref.TPSRatio)
}

func ttftSubScore(ratio float64) float64 {
	if ratio <= 0 {
		return 0
	}
	switch {
	case ratio <= 1.0:
		return 7.5
	case ratio <= 1.15:
		return 6.5
	case ratio <= 1.5:
		return 5.0
	case ratio <= 2.0:
		return 3.5
	case ratio <= 3.0:
		return 2.0
	default:
		return 1.0
	}
}

func tpsSubScore(ratio float64) float64 {
	if ratio <= 0 {
		return 0
	}
	switch {
	case ratio >= 1.0:
		return 7.5
	case ratio >= 0.85:
		return 6.5
	case ratio >= 0.7:
		return 5.0
	case ratio >= 0.5:
		return 3.5
	case ratio >= 0.3:
		return 2.0
	default:
		return 1.0
	}
}

func ladderPerformanceScore(avgLatency int64) int {
	switch {
	case avgLatency <= 1500:
		return 15
	case avgLatency <= 3000:
		return 12
	case avgLatency <= 6000:
		return 9
	case avgLatency <= 10000:
		return 6
	default:
		return 3
	}
}

func buildChecklistItem(result CheckResult) ChecklistItem {
	status := "failed"
	switch {
	case result.Skipped:
		status = "skipped"
	case result.Success:
		status = "passed"
	}
	return ChecklistItem{
		Provider:           result.Provider,
		CheckKey:           string(result.CheckKey),
		CheckName:          checkDisplayName(result.CheckKey),
		ModelName:          result.ModelName,
		ClaimedModel:       result.ClaimedModel,
		ObservedModel:      result.ObservedModel,
		IdentityConfidence: result.IdentityConfidence,
		SuspectedDowngrade: result.SuspectedDowngrade,
		Consistent:         result.Consistent,
		ConsistencyMethod:  result.ConsistencyMethod,
		Skipped:            result.Skipped,
		Passed:             result.Success && !result.Skipped,
		Status:             status,
		Score:              result.Score,
		LatencyMs:          result.LatencyMs,
		TTFTMs:             result.TTFTMs,
		TokensPS:           result.TokensPS,
		ErrorCode:          result.ErrorCode,
		Message:            result.Message,
	}
}

func checkDisplayName(checkKey CheckKey) string {
	switch checkKey {
	case CheckModelsList:
		return "模型列表接口"
	case CheckAvailability:
		return "Token 基础可用性"
	case CheckModelAccess:
		return "模型调用可用性"
	case CheckModelIdentity:
		return "模型身份一致性"
	case CheckStream:
		return "流式输出能力"
	case CheckJSON:
		return "JSON 输出稳定性"
	case CheckReproducibility:
		return "复现性指纹"
	default:
		return string(checkKey)
	}
}

func gradeForScore(score int) string {
	switch {
	case score >= 90:
		return "S"
	case score >= 80:
		return "A"
	case score >= 65:
		return "B"
	case score >= 50:
		return "C"
	case score > 0:
		return "D"
	default:
		return "Fail"
	}
}

func conclusionForGrade(grade string) string {
	switch grade {
	case "S":
		return "优质 Token，适合生产调用"
	case "A":
		return "稳定可用，适合日常调用"
	case "B":
		return "普通可用，存在轻微质量波动"
	case "C":
		return "可用但风险较高，建议谨慎使用"
	case "D":
		return "质量较差，不建议高频使用"
	default:
		return "核心检测未通过"
	}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
