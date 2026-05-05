package token_verifier

import "strings"

func RedactDirectProbeResponse(response *DirectProbeResponse, secrets ...string) *DirectProbeResponse {
	if response == nil {
		return nil
	}
	normalizedSecrets := normalizeRedactionSecrets(secrets)
	if len(normalizedSecrets) == 0 {
		return response
	}
	redacted := *response
	redacted.BaseURL = redactSecretString(response.BaseURL, normalizedSecrets)
	redacted.Provider = redactSecretString(response.Provider, normalizedSecrets)
	redacted.Model = redactSecretString(response.Model, normalizedSecrets)
	redacted.ProbeProfile = redactSecretString(response.ProbeProfile, normalizedSecrets)
	redacted.ClientProfile = redactSecretString(response.ClientProfile, normalizedSecrets)
	redacted.Results = redactCheckResults(response.Results, normalizedSecrets)
	redacted.Report = redactReportSummary(response.Report, normalizedSecrets)
	return &redacted
}

func normalizeRedactionSecrets(secrets []string) []string {
	normalized := make([]string, 0, len(secrets))
	seen := make(map[string]struct{}, len(secrets))
	for _, secret := range secrets {
		secret = strings.TrimSpace(secret)
		if secret == "" {
			continue
		}
		if _, ok := seen[secret]; ok {
			continue
		}
		seen[secret] = struct{}{}
		normalized = append(normalized, secret)
	}
	return normalized
}

func redactCheckResults(results []CheckResult, secrets []string) []CheckResult {
	if len(results) == 0 {
		return results
	}
	redacted := make([]CheckResult, len(results))
	for i, result := range results {
		redacted[i] = result
		redacted[i].Provider = redactSecretString(result.Provider, secrets)
		redacted[i].Group = redactSecretString(result.Group, secrets)
		redacted[i].ModelName = redactSecretString(result.ModelName, secrets)
		redacted[i].ErrorCode = redactSecretString(result.ErrorCode, secrets)
		redacted[i].Message = redactSecretString(result.Message, secrets)
		redacted[i].Raw = redactSecretMap(result.Raw, secrets)
		redacted[i].PrivateResponseText = ""
	}
	return redacted
}

func redactReportSummary(report ReportSummary, secrets []string) ReportSummary {
	redacted := report
	redacted.Conclusion = redactSecretString(report.Conclusion, secrets)
	redacted.Checklist = redactChecklistItems(report.Checklist, secrets)
	redacted.IdentityAssessments = redactIdentityAssessments(report.IdentityAssessments, secrets)
	redacted.Risks = redactSecretStrings(report.Risks, secrets)
	redacted.FinalRating = redactFinalRating(report.FinalRating, secrets)
	redacted.ScoringVersion = redactSecretString(report.ScoringVersion, secrets)
	return redacted
}

func redactChecklistItems(items []ChecklistItem, secrets []string) []ChecklistItem {
	if len(items) == 0 {
		return items
	}
	redacted := make([]ChecklistItem, len(items))
	for i, item := range items {
		redacted[i] = item
		redacted[i].Provider = redactSecretString(item.Provider, secrets)
		redacted[i].Group = redactSecretString(item.Group, secrets)
		redacted[i].CheckKey = redactSecretString(item.CheckKey, secrets)
		redacted[i].CheckName = redactSecretString(item.CheckName, secrets)
		redacted[i].ModelName = redactSecretString(item.ModelName, secrets)
		redacted[i].Status = redactSecretString(item.Status, secrets)
		redacted[i].ErrorCode = redactSecretString(item.ErrorCode, secrets)
		redacted[i].Message = redactSecretString(item.Message, secrets)
		redacted[i].RiskLevel = redactSecretString(item.RiskLevel, secrets)
		redacted[i].Evidence = redactSecretStrings(item.Evidence, secrets)
	}
	return redacted
}

func redactIdentityAssessments(items []IdentityAssessmentSummary, secrets []string) []IdentityAssessmentSummary {
	if len(items) == 0 {
		return items
	}
	redacted := make([]IdentityAssessmentSummary, len(items))
	for i, item := range items {
		redacted[i] = item
		redacted[i].Provider = redactSecretString(item.Provider, secrets)
		redacted[i].ModelName = redactSecretString(item.ModelName, secrets)
		redacted[i].ClaimedModel = redactSecretString(item.ClaimedModel, secrets)
		redacted[i].Status = redactSecretString(item.Status, secrets)
		redacted[i].PredictedFamily = redactSecretString(item.PredictedFamily, secrets)
		redacted[i].PredictedModel = redactSecretString(item.PredictedModel, secrets)
		redacted[i].Method = redactSecretString(item.Method, secrets)
		redacted[i].Candidates = redactIdentityCandidates(item.Candidates, secrets)
		redacted[i].SubmodelAssessments = redactSubmodelAssessments(item.SubmodelAssessments, secrets)
		if item.Verdict != nil {
			verdict := *item.Verdict
			verdict.Status = redactSecretString(verdict.Status, secrets)
			verdict.TrueFamily = redactSecretString(verdict.TrueFamily, secrets)
			verdict.TrueModel = redactSecretString(verdict.TrueModel, secrets)
			verdict.SpoofMethod = redactSecretString(verdict.SpoofMethod, secrets)
			verdict.ConfidenceBand = redactSecretString(verdict.ConfidenceBand, secrets)
			verdict.Reasoning = redactSecretStrings(verdict.Reasoning, secrets)
			redacted[i].Verdict = &verdict
		}
		redacted[i].RiskFlags = redactSecretStrings(item.RiskFlags, secrets)
		redacted[i].Evidence = redactSecretStrings(item.Evidence, secrets)
	}
	return redacted
}

func redactIdentityCandidates(items []IdentityCandidateSummary, secrets []string) []IdentityCandidateSummary {
	if len(items) == 0 {
		return items
	}
	redacted := make([]IdentityCandidateSummary, len(items))
	for i, item := range items {
		redacted[i] = item
		redacted[i].Family = redactSecretString(item.Family, secrets)
		redacted[i].Model = redactSecretString(item.Model, secrets)
		redacted[i].Reasons = redactSecretStrings(item.Reasons, secrets)
	}
	return redacted
}

func redactSubmodelAssessments(items []SubmodelAssessmentSummary, secrets []string) []SubmodelAssessmentSummary {
	if len(items) == 0 {
		return items
	}
	redacted := make([]SubmodelAssessmentSummary, len(items))
	for i, item := range items {
		redacted[i] = item
		redacted[i].Method = redactSecretString(item.Method, secrets)
		redacted[i].Family = redactSecretString(item.Family, secrets)
		redacted[i].ModelID = redactSecretString(item.ModelID, secrets)
		redacted[i].DisplayName = redactSecretString(item.DisplayName, secrets)
		redacted[i].Candidates = redactIdentityCandidates(item.Candidates, secrets)
		redacted[i].Evidence = redactSecretStrings(item.Evidence, secrets)
	}
	return redacted
}

func redactFinalRating(rating FinalRating, secrets []string) FinalRating {
	redacted := rating
	redacted.Conclusion = redactSecretString(rating.Conclusion, secrets)
	redacted.Risks = redactSecretStrings(rating.Risks, secrets)
	return redacted
}

func redactSecretStrings(values []string, secrets []string) []string {
	if len(values) == 0 {
		return values
	}
	redacted := make([]string, len(values))
	for i, value := range values {
		redacted[i] = redactSecretString(value, secrets)
	}
	return redacted
}

func redactSecretMap(values map[string]any, secrets []string) map[string]any {
	if len(values) == 0 {
		return values
	}
	redacted := make(map[string]any, len(values))
	for key, value := range values {
		redacted[redactSecretString(key, secrets)] = redactSecretValue(value, secrets)
	}
	return redacted
}

func redactSecretValue(value any, secrets []string) any {
	switch typedValue := value.(type) {
	case string:
		return redactSecretString(typedValue, secrets)
	case map[string]any:
		return redactSecretMap(typedValue, secrets)
	case []any:
		redacted := make([]any, len(typedValue))
		for i, item := range typedValue {
			redacted[i] = redactSecretValue(item, secrets)
		}
		return redacted
	default:
		return value
	}
}

func redactSecretString(value string, secrets []string) string {
	redacted := value
	for _, secret := range secrets {
		redacted = strings.ReplaceAll(redacted, secret, "[REDACTED]")
	}
	return redacted
}
