package token_verifier

import (
	"os"
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
)

type identityAssessmentCorpus struct {
	Description  string                         `json:"description"`
	ManualReview CorpusManualReview             `json:"manual_review,omitempty"`
	Cases        []identityAssessmentCorpusCase `json:"cases"`
}

type identityAssessmentCorpusCase struct {
	Name                      string                           `json:"name"`
	Results                   []identityAssessmentCorpusResult `json:"results"`
	WantIdentityStatus        string                           `json:"want_identity_status"`
	WantVerdictStatus         string                           `json:"want_verdict_status,omitempty"`
	WantPredictedFamily       string                           `json:"want_predicted_family,omitempty"`
	WantPredictedModel        string                           `json:"want_predicted_model,omitempty"`
	WantTopCandidateFamily    string                           `json:"want_top_candidate_family,omitempty"`
	WantIdentityRisk          *bool                            `json:"want_identity_risk,omitempty"`
	WantNoPredictedFamily     bool                             `json:"want_no_predicted_family,omitempty"`
	WantNoPredictedModel      bool                             `json:"want_no_predicted_model,omitempty"`
	WantEvidenceNotContaining []string                         `json:"want_evidence_not_containing,omitempty"`
}

type identityAssessmentCorpusResult struct {
	CheckResult
	PrivateResponseText string `json:"private_response_text,omitempty"`
}

func TestIdentityAssessmentCorpusHarnessReplaysReportLevelOutcomes(t *testing.T) {
	corpus := loadIdentityAssessmentCorpusForTest(t, "testdata/identity_assessment_corpus_schema_golden.json")
	metric := evaluateIdentityAssessmentCorpus(t, corpus)

	if metric.Total < 6 {
		t.Fatalf("identity schema corpus total = %d, want representative match/mismatch/uncertain cases", metric.Total)
	}
	if metric.StatusCorrect != metric.Total || metric.VerdictCorrect != metric.Total || metric.FamilyCorrect != metric.Total || metric.ModelCorrect != metric.Total || metric.IdentityRiskCorrect != metric.Total {
		t.Fatalf("identity schema corpus metric = %+v, want all fields correct", metric)
	}
}

func TestOptionalRealIdentityAssessmentCorpus(t *testing.T) {
	const path = "testdata/identity_assessment_corpus_real.json"
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			t.Skip("real identity assessment corpus not present; add " + path + " with manually labeled real model outputs to measure identity detector accuracy")
		}
		t.Fatalf("stat real identity assessment corpus: %v", err)
	}
	skipOptionalRealCorpusAudit(t, "empirical identity detector accuracy corpus")

	auditRealIdentityAssessmentCorpus(t, path)
}

func TestRequiredRealIdentityAssessmentCorpusAudit(t *testing.T) {
	if os.Getenv(requireRealProbeCorpusEnv) != "1" {
		t.Skip("set " + requireRealProbeCorpusEnv + "=1 to require empirical identity detector accuracy corpus")
	}
	auditRealIdentityAssessmentCorpus(t, "testdata/identity_assessment_corpus_real.json")
}

func auditRealIdentityAssessmentCorpus(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read identity assessment corpus %s: %v", path, err)
	}
	if missing := validateIdentityRealCorpusEvidence(data, nil); len(missing) > 0 {
		t.Fatalf("real identity assessment corpus failed hard audit: %s", strings.Join(missing, ", "))
	}
	corpus := loadIdentityAssessmentCorpusForTest(t, path)
	missing := realIdentityCorpusMissingCoverage(toExportedIdentityAssessmentCorpus(corpus))
	if len(missing) > 0 {
		t.Fatalf("real identity assessment corpus is missing required coverage: %s", strings.Join(missing, ", "))
	}
	metric := evaluateIdentityAssessmentCorpus(t, corpus)
	t.Logf("identity corpus metric: %+v", metric)
	if metric.Total == 0 {
		t.Fatalf("real identity assessment corpus %s produced no scored samples", path)
	}
	if metric.StatusCorrect != metric.Total || metric.VerdictCorrect != metric.Total || metric.FamilyCorrect != metric.Total || metric.ModelCorrect != metric.Total || metric.IdentityRiskCorrect != metric.Total {
		t.Fatalf("real identity assessment corpus has mismatches: %+v", metric)
	}
}

func TestRealIdentityAssessmentCorpusExampleIsNotUsedAsEmpiricalEvidence(t *testing.T) {
	if _, err := os.Stat("testdata/identity_assessment_corpus_real.example.json"); err != nil {
		t.Fatalf("real identity assessment corpus example fixture missing: %v", err)
	}
	if os.Getenv(requireRealProbeCorpusEnv) != "1" {
		t.Skip("set " + requireRealProbeCorpusEnv + "=1 to require empirical identity detector accuracy corpus")
	}
	if _, err := os.Stat("testdata/identity_assessment_corpus_real.json"); err == nil {
		t.Skip("real identity assessment corpus exists; empirical identity test owns evaluation")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat real identity assessment corpus: %v", err)
	}
}

func TestIdentityAssessmentCorpusRejectsInvalidCases(t *testing.T) {
	_, err := evaluateIdentityAssessmentCorpusCases(toExportedIdentityAssessmentCorpus(identityAssessmentCorpus{
		Cases: []identityAssessmentCorpusCase{
			{Name: "missing_status", Results: []identityAssessmentCorpusResult{identityCorpusProbeResult(CheckProbeIdentitySelfKnowledge, "I am ChatGPT by OpenAI.")}},
		},
	}))
	if err == nil || !strings.Contains(err.Error(), "missing want_identity_status") {
		t.Fatalf("invalid identity corpus error = %v, want missing want_identity_status", err)
	}

	_, err = evaluateIdentityAssessmentCorpusCases(toExportedIdentityAssessmentCorpus(identityAssessmentCorpus{
		Cases: []identityAssessmentCorpusCase{
			{
				Name:               "no_identity_results",
				WantIdentityStatus: identityStatusMatch,
				Results: []identityAssessmentCorpusResult{
					{CheckResult: CheckResult{Provider: ProviderOpenAI, Group: probeGroupQuality, CheckKey: CheckProbeInstructionFollow, ModelName: "gpt-test", Success: true, Score: 100}},
				},
			},
		},
	}))
	if err == nil || !strings.Contains(err.Error(), "missing identity assessment") {
		t.Fatalf("invalid identity corpus error = %v, want missing identity assessment", err)
	}
}

func loadIdentityAssessmentCorpusForTest(t *testing.T, path string) identityAssessmentCorpus {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read identity assessment corpus %s: %v", path, err)
	}
	var corpus identityAssessmentCorpus
	if err := common.Unmarshal(data, &corpus); err != nil {
		t.Fatalf("parse identity assessment corpus %s: %v", path, err)
	}
	if len(corpus.Cases) == 0 {
		t.Fatalf("identity assessment corpus %s must not be empty", path)
	}
	return corpus
}

func evaluateIdentityAssessmentCorpus(t *testing.T, corpus identityAssessmentCorpus) identityAccuracyMetric {
	t.Helper()
	metric, err := evaluateIdentityAssessmentCorpusCases(toExportedIdentityAssessmentCorpus(corpus))
	if err != nil {
		t.Fatal(err)
	}
	return metric
}

func identityCorpusProbeResult(key CheckKey, text string) identityAssessmentCorpusResult {
	return identityAssessmentCorpusResult{
		CheckResult: CheckResult{
			Provider:  ProviderOpenAI,
			Group:     probeGroupIdentity,
			CheckKey:  key,
			ModelName: "gpt-5.5",
			Neutral:   true,
			Success:   true,
			Raw: map[string]any{
				"response_sample": text,
			},
		},
		PrivateResponseText: text,
	}
}

func toExportedIdentityAssessmentCorpus(corpus identityAssessmentCorpus) IdentityAssessmentCorpus {
	out := IdentityAssessmentCorpus{Description: corpus.Description, ManualReview: corpus.ManualReview, Cases: make([]IdentityAssessmentCorpusCase, 0, len(corpus.Cases))}
	for _, item := range corpus.Cases {
		results := make([]IdentityAssessmentCorpusResult, 0, len(item.Results))
		for _, result := range item.Results {
			results = append(results, IdentityAssessmentCorpusResult{
				CheckResult:         result.CheckResult,
				PrivateResponseText: result.PrivateResponseText,
			})
		}
		out.Cases = append(out.Cases, IdentityAssessmentCorpusCase{
			Name:                      item.Name,
			Results:                   results,
			WantIdentityStatus:        item.WantIdentityStatus,
			WantVerdictStatus:         item.WantVerdictStatus,
			WantPredictedFamily:       item.WantPredictedFamily,
			WantPredictedModel:        item.WantPredictedModel,
			WantTopCandidateFamily:    item.WantTopCandidateFamily,
			WantIdentityRisk:          item.WantIdentityRisk,
			WantNoPredictedFamily:     item.WantNoPredictedFamily,
			WantNoPredictedModel:      item.WantNoPredictedModel,
			WantEvidenceNotContaining: item.WantEvidenceNotContaining,
		})
	}
	return out
}
