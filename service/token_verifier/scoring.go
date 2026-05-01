package token_verifier

import "strings"

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

	for _, result := range results {
		checklist = append(checklist, buildChecklistItem(result))
		totalChecks++
		if result.Success {
			successChecks++
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
			}
		case CheckJSON:
			if result.Success {
				dimensions["json"] = 5
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
	if latencyCount > 0 {
		avgLatency := latencyTotal / int64(latencyCount)
		switch {
		case avgLatency <= 1500:
			dimensions["performance"] = 15
		case avgLatency <= 3000:
			dimensions["performance"] = 12
		case avgLatency <= 6000:
			dimensions["performance"] = 9
		case avgLatency <= 10000:
			dimensions["performance"] = 6
		default:
			dimensions["performance"] = 3
		}
	}

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

	uniqueRisks := uniqueStrings(risks)
	conclusion := conclusionForGrade(grade)
	return ReportSummary{
		Score:         score,
		Grade:         grade,
		Conclusion:    conclusion,
		Dimensions:    dimensions,
		Checklist:     checklist,
		Models:        models,
		ModelIdentity: modelIdentity,
		Metrics:       metrics,
		Risks:         uniqueRisks,
		FinalRating: FinalRating{
			Score:      score,
			Grade:      grade,
			Conclusion: conclusion,
			Dimensions: dimensions,
			Risks:      uniqueRisks,
		},
	}
}

func buildChecklistItem(result CheckResult) ChecklistItem {
	status := "failed"
	if result.Success {
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
		Passed:             result.Success,
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
