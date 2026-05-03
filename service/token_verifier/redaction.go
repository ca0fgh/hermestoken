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
		redacted[i].ModelName = redactSecretString(result.ModelName, secrets)
		redacted[i].ClaimedModel = redactSecretString(result.ClaimedModel, secrets)
		redacted[i].ObservedModel = redactSecretString(result.ObservedModel, secrets)
		redacted[i].ConsistencyMethod = redactSecretString(result.ConsistencyMethod, secrets)
		redacted[i].ErrorCode = redactSecretString(result.ErrorCode, secrets)
		redacted[i].Message = redactSecretString(result.Message, secrets)
		redacted[i].Raw = redactSecretMap(result.Raw, secrets)
	}
	return redacted
}

func redactReportSummary(report ReportSummary, secrets []string) ReportSummary {
	redacted := report
	redacted.Conclusion = redactSecretString(report.Conclusion, secrets)
	redacted.Checklist = redactChecklistItems(report.Checklist, secrets)
	redacted.Models = redactModelSummaries(report.Models, secrets)
	redacted.ModelIdentity = redactModelIdentitySummaries(report.ModelIdentity, secrets)
	redacted.Reproducibility = redactReproducibilitySummaries(report.Reproducibility, secrets)
	redacted.Risks = redactSecretStrings(report.Risks, secrets)
	redacted.FinalRating = redactFinalRating(report.FinalRating, secrets)
	redacted.ScoringVersion = redactSecretString(report.ScoringVersion, secrets)
	redacted.BaselineSource = redactSecretString(report.BaselineSource, secrets)
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
		redacted[i].CheckKey = redactSecretString(item.CheckKey, secrets)
		redacted[i].CheckName = redactSecretString(item.CheckName, secrets)
		redacted[i].ModelName = redactSecretString(item.ModelName, secrets)
		redacted[i].ClaimedModel = redactSecretString(item.ClaimedModel, secrets)
		redacted[i].ObservedModel = redactSecretString(item.ObservedModel, secrets)
		redacted[i].ConsistencyMethod = redactSecretString(item.ConsistencyMethod, secrets)
		redacted[i].Status = redactSecretString(item.Status, secrets)
		redacted[i].ErrorCode = redactSecretString(item.ErrorCode, secrets)
		redacted[i].Message = redactSecretString(item.Message, secrets)
	}
	return redacted
}

func redactModelSummaries(models []ModelSummary, secrets []string) []ModelSummary {
	if len(models) == 0 {
		return models
	}
	redacted := make([]ModelSummary, len(models))
	for i, model := range models {
		redacted[i] = model
		redacted[i].Provider = redactSecretString(model.Provider, secrets)
		redacted[i].ModelName = redactSecretString(model.ModelName, secrets)
		redacted[i].Message = redactSecretString(model.Message, secrets)
		if model.Baseline != nil {
			baseline := *model.Baseline
			baseline.Source = redactSecretString(baseline.Source, secrets)
			baseline.Slug = redactSecretString(baseline.Slug, secrets)
			baseline.Note = redactSecretString(baseline.Note, secrets)
			redacted[i].Baseline = &baseline
		}
	}
	return redacted
}

func redactModelIdentitySummaries(items []ModelIdentitySummary, secrets []string) []ModelIdentitySummary {
	if len(items) == 0 {
		return items
	}
	redacted := make([]ModelIdentitySummary, len(items))
	for i, item := range items {
		redacted[i] = item
		redacted[i].Provider = redactSecretString(item.Provider, secrets)
		redacted[i].ClaimedModel = redactSecretString(item.ClaimedModel, secrets)
		redacted[i].ObservedModel = redactSecretString(item.ObservedModel, secrets)
		redacted[i].Message = redactSecretString(item.Message, secrets)
	}
	return redacted
}

func redactReproducibilitySummaries(items []ReproducibilitySummary, secrets []string) []ReproducibilitySummary {
	if len(items) == 0 {
		return items
	}
	redacted := make([]ReproducibilitySummary, len(items))
	for i, item := range items {
		redacted[i] = item
		redacted[i].Provider = redactSecretString(item.Provider, secrets)
		redacted[i].ModelName = redactSecretString(item.ModelName, secrets)
		redacted[i].Method = redactSecretString(item.Method, secrets)
		redacted[i].Message = redactSecretString(item.Message, secrets)
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
