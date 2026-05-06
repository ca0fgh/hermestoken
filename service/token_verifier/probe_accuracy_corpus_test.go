package token_verifier

import (
	"os"
	"strings"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
)

const requireRealProbeCorpusEnv = "TOKEN_VERIFICATION_REQUIRE_REAL_CORPUS"

func TestLabeledProbeCorpusHarnessComputesPerCheckMetrics(t *testing.T) {
	corpus := loadLabeledProbeCorpusForTest(t, "testdata/labeled_probe_corpus_schema_golden.json")
	metrics := evaluateLabeledProbeCorpus(t, corpus)

	for key, metric := range metrics {
		if metric.Positive == 0 {
			t.Fatalf("%s corpus metric has no positive samples: %+v", key, metric)
		}
		if metric.Negative == 0 {
			t.Fatalf("%s corpus metric has no negative samples: %+v", key, metric)
		}
		if metric.FalsePositive != 0 || metric.FalseNegative != 0 {
			t.Fatalf("%s corpus metric = %+v, want no FP/FN on schema golden corpus", key, metric)
		}
	}
}

func TestOptionalRealLabeledProbeCorpus(t *testing.T) {
	const path = "testdata/labeled_probe_corpus_real.json"
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			t.Skip("real labeled probe corpus not present; add " + path + " to measure empirical detector accuracy")
		}
		t.Fatalf("stat real labeled probe corpus: %v", err)
	}
	skipOptionalRealCorpusAudit(t, "empirical pass/fail detector accuracy corpus")

	auditRealLabeledProbeCorpus(t, path)
}

func TestRequiredRealLabeledProbeCorpusAudit(t *testing.T) {
	if os.Getenv(requireRealProbeCorpusEnv) != "1" {
		t.Skip("set " + requireRealProbeCorpusEnv + "=1 to require empirical pass/fail detector accuracy corpus")
	}
	auditRealLabeledProbeCorpus(t, "testdata/labeled_probe_corpus_real.json")
}

func TestRequiredReviewOnlyProbeJudgeAudit(t *testing.T) {
	if os.Getenv(requireRealProbeCorpusEnv) != "1" {
		t.Skip("set " + requireRealProbeCorpusEnv + "=1 to require judge-backed review-only detector audit")
	}
	if missing := reviewOnlyProbeJudgeAuditMissingRequirements(probeJudgeConfigFromEnv(), probeBaselineFromEnv()); len(missing) > 0 {
		t.Fatalf("review-only probe judge audit is missing requirements: %s", strings.Join(missing, ", "))
	}
}

func TestRequiredRealInformationalProbeCorpusAudit(t *testing.T) {
	if os.Getenv(requireRealProbeCorpusEnv) != "1" {
		t.Skip("set " + requireRealProbeCorpusEnv + "=1 to require empirical informational detector accuracy corpus")
	}
	auditRealInformationalProbeCorpus(t, "testdata/informational_probe_corpus_real.json")
}

func skipOptionalRealCorpusAudit(t *testing.T, description string) {
	t.Helper()
	if os.Getenv(requireRealProbeCorpusEnv) != "1" {
		t.Skip("set " + requireRealProbeCorpusEnv + "=1 to run " + description)
	}
}

func auditRealLabeledProbeCorpus(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read labeled probe corpus %s: %v", path, err)
	}
	if missing := validatePassFailRealCorpusEvidence(data, nil); len(missing) > 0 {
		t.Fatalf("real labeled corpus failed hard audit: %s", strings.Join(missing, ", "))
	}
	corpus := loadLabeledProbeCorpusForTest(t, path)
	metrics := evaluateLabeledProbeCorpus(t, corpus)
	if missing := realCorpusMissingCoverage(metrics); len(missing) > 0 {
		t.Fatalf("real labeled corpus is missing required per-check coverage: %s", strings.Join(missing, ", "))
	}
	totalFalsePositive := 0
	totalFalseNegative := 0
	totalSamples := 0
	for key, metric := range metrics {
		totalFalsePositive += metric.FalsePositive
		totalFalseNegative += metric.FalseNegative
		totalSamples += metric.Positive + metric.Negative
		t.Logf("%s: %+v", key, metric)
	}
	if totalSamples == 0 {
		t.Fatalf("real labeled probe corpus %s produced no scored samples", path)
	}
	if totalFalsePositive != 0 || totalFalseNegative != 0 {
		t.Fatalf("real labeled corpus has false positives=%d false negatives=%d", totalFalsePositive, totalFalseNegative)
	}
}

func auditRealInformationalProbeCorpus(t *testing.T, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read informational probe corpus %s: %v", path, err)
	}
	if missing := validateInformationalRealCorpusEvidence(data, nil); len(missing) > 0 {
		t.Fatalf("real informational corpus failed hard audit: %s", strings.Join(missing, ", "))
	}
	corpus := loadInformationalProbeCorpusForTest(t, path)
	metrics, err := evaluateInformationalProbeCorpusCases(corpus)
	if err != nil {
		t.Fatal(err)
	}
	if missing := realInformationalCorpusMissingCoverage(metrics); len(missing) > 0 {
		t.Fatalf("real informational corpus is missing required coverage: %s", strings.Join(missing, ", "))
	}
	for key, metric := range metrics {
		t.Logf("%s: %+v", key, metric)
		if metric.FalsePositive != 0 || metric.FalseNegative != 0 {
			t.Fatalf("%s real informational corpus has false positives=%d false negatives=%d", key, metric.FalsePositive, metric.FalseNegative)
		}
	}
}

func TestRealCorpusCoverageRequirementsMatchRuntimeFullProbeSuite(t *testing.T) {
	requirements := realCorpusCoverageRequirements()
	if len(requirements) == 0 {
		t.Fatal("real corpus coverage requirements must not be empty")
	}
	runtimeKeys := checkKeySetForTest(runtimeProbeSuiteDefinitions(ProbeProfileFull))
	for _, probe := range runtimeProbeSuiteDefinitions(ProbeProfileFull) {
		requirement, ok := requirements[probe.Key]
		if !isPassFailCorpusProbe(probe) {
			if ok {
				t.Fatalf("%s should not be required in pass/fail real corpus: %+v", probe.Key, requirement)
			}
			continue
		}
		if !ok {
			t.Fatalf("%s missing real corpus coverage requirement", probe.Key)
		}
		if requirement.Total < 1 {
			t.Fatalf("%s real corpus requirement must include at least one replayable sample: %+v", probe.Key, requirement)
		}
		if requirement.Positive != 0 || requirement.Negative != 0 {
			t.Fatalf("%s real corpus requirement should not force naturally occurring pass/fail polarity: %+v", probe.Key, requirement)
		}
	}
	for _, probe := range probeSuiteDefinitions(ProbeProfileFull) {
		if runtimeKeys[probe.Key] || !isPassFailCorpusProbe(probe) {
			continue
		}
		if _, ok := requirements[probe.Key]; ok {
			t.Fatalf("%s is not in runtime full suite and should not be required by default audit", probe.Key)
		}
	}
}

func TestHardAccuracyAuditRequirementsCoverEveryRuntimeFullDetectionItem(t *testing.T) {
	passFail := realCorpusCoverageRequirements()
	reviewOnly := reviewOnlyProbeJudgeCoverageRequirements()
	informational := informationalProbeCorpusCoverageRequirements()
	identityFeature := make(map[CheckKey]bool)
	for _, checkKey := range identityFeatureCorpusCoverageRequirements() {
		identityFeature[checkKey] = true
	}

	for _, probe := range runtimeProbeSuiteDefinitions(ProbeProfileFull) {
		coveredBy := make([]string, 0, 3)
		if _, ok := passFail[probe.Key]; ok {
			coveredBy = append(coveredBy, "pass_fail_real_corpus")
		}
		if _, ok := reviewOnly[probe.Key]; ok {
			coveredBy = append(coveredBy, "review_only_judge")
		}
		if _, ok := informational[probe.Key]; ok {
			coveredBy = append(coveredBy, "informational_real_corpus")
		}
		if identityFeature[probe.Key] {
			coveredBy = append(coveredBy, "identity_real_corpus")
		}
		if len(coveredBy) == 0 {
			t.Fatalf("%s has no hard empirical accuracy audit path: %+v", probe.Key, probe)
		}
		if len(coveredBy) > 1 {
			t.Fatalf("%s has ambiguous hard accuracy audit paths %v: %+v", probe.Key, coveredBy, probe)
		}
	}
}

func TestDefaultAccuracyAuditDoesNotRequireLegacyTargetedChecks(t *testing.T) {
	if _, ok := checkKeySetForTest(runtimeProbeSuiteDefinitions(ProbeProfileFull))[CheckProbeBedrockProbe]; ok {
		t.Fatal("test requires bedrock probe to remain outside the default runtime suite")
	}
	if _, ok := realCorpusCoverageRequirements()[CheckProbeBedrockProbe]; ok {
		t.Fatal("legacy targeted bedrock probe should not be required by default pass/fail audit")
	}
	if _, ok := reviewOnlyProbeJudgeCoverageRequirements()[CheckProbeZHReasoning]; ok {
		t.Fatal("legacy targeted zh reasoning probe should not require default judge audit coverage")
	}

	metrics, err := evaluateLabeledProbeCorpusCases(LabeledProbeCorpus{
		Cases: []LabeledProbeCorpusCase{
			{
				Name:         "legacy_targeted_bedrock_safe",
				CheckKey:     CheckProbeBedrockProbe,
				ResponseText: "I do not know what cloud infrastructure serves this API.",
				WantPassed:   true,
			},
		},
	})
	if err != nil {
		t.Fatalf("evaluate legacy targeted corpus case: %v", err)
	}
	if got := metrics[CheckProbeBedrockProbe]; got.TruePositive != 1 || got.FalsePositive != 0 || got.FalseNegative != 0 {
		t.Fatalf("legacy targeted bedrock metrics = %+v, want evaluated but not required", got)
	}
}

func TestRealLabeledProbeCorpusExampleIsNotUsedAsEmpiricalEvidence(t *testing.T) {
	if _, err := os.Stat("testdata/labeled_probe_corpus_real.example.json"); err != nil {
		t.Fatalf("real corpus example fixture missing: %v", err)
	}
	if os.Getenv(requireRealProbeCorpusEnv) != "1" {
		t.Skip("set " + requireRealProbeCorpusEnv + "=1 to require empirical pass/fail detector accuracy corpus")
	}
	if _, err := os.Stat("testdata/labeled_probe_corpus_real.json"); err == nil {
		t.Skip("real labeled probe corpus exists; empirical accuracy test owns evaluation")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat real labeled probe corpus: %v", err)
	}
}

func TestInformationalProbeCorpusHarnessComputesMetrics(t *testing.T) {
	corpus := loadInformationalProbeCorpusForTest(t, "testdata/informational_probe_corpus_schema_golden.json")
	metrics, err := evaluateInformationalProbeCorpusCases(corpus)
	if err != nil {
		t.Fatal(err)
	}
	for key, metric := range metrics {
		if metric.Positive == 0 {
			t.Fatalf("%s informational metric has no positive samples: %+v", key, metric)
		}
		if metric.Negative == 0 {
			t.Fatalf("%s informational metric has no negative samples: %+v", key, metric)
		}
		if metric.FalsePositive != 0 || metric.FalseNegative != 0 {
			t.Fatalf("%s informational metric = %+v, want no FP/FN on schema golden corpus", key, metric)
		}
	}
}

func TestRealInformationalProbeCorpusExampleIsNotUsedAsEmpiricalEvidence(t *testing.T) {
	if _, err := os.Stat("testdata/informational_probe_corpus_real.example.json"); err != nil {
		t.Fatalf("real informational corpus example fixture missing: %v", err)
	}
	if os.Getenv(requireRealProbeCorpusEnv) != "1" {
		t.Skip("set " + requireRealProbeCorpusEnv + "=1 to require empirical informational detector accuracy corpus")
	}
	if _, err := os.Stat("testdata/informational_probe_corpus_real.json"); err == nil {
		t.Skip("real informational probe corpus exists; empirical accuracy test owns evaluation")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat real informational probe corpus: %v", err)
	}
}

func TestLabeledProbeCorpusRejectsInvalidCases(t *testing.T) {
	_, err := evaluateLabeledProbeCorpusCases(LabeledProbeCorpus{
		Cases: []LabeledProbeCorpusCase{
			{Name: "unknown_check", CheckKey: "probe_does_not_exist", ResponseText: "OK", WantPassed: true},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown check_key") {
		t.Fatalf("invalid corpus error = %v, want unknown check_key", err)
	}
}

func TestLabeledProbeCorpusCaseMatchesCurrentScorer(t *testing.T) {
	stable := LabeledProbeCorpusCase{
		Name:         "stable",
		CheckKey:     CheckProbeInstructionFollow,
		ResponseText: "Fortran\nLisp\nCOBOL\nBASIC\nC",
		WantPassed:   true,
	}
	if !LabeledProbeCorpusCaseMatchesCurrentScorer(stable) {
		t.Fatal("expected stable labeled case to match current scorer")
	}
	mismatch := stable
	mismatch.Name = "mismatch"
	mismatch.ResponseText = "this omits required languages"
	if LabeledProbeCorpusCaseMatchesCurrentScorer(mismatch) {
		t.Fatal("expected mismatched labeled case to be rejected")
	}
	unknown := stable
	unknown.CheckKey = "probe_unknown"
	if LabeledProbeCorpusCaseMatchesCurrentScorer(unknown) {
		t.Fatal("expected unknown check key to be rejected")
	}
}

func loadInformationalProbeCorpusForTest(t *testing.T, path string) InformationalProbeCorpus {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read informational probe corpus %s: %v", path, err)
	}
	var corpus InformationalProbeCorpus
	if err := common.Unmarshal(data, &corpus); err != nil {
		t.Fatalf("parse informational probe corpus %s: %v", path, err)
	}
	if len(corpus.Cases) == 0 {
		t.Fatalf("informational probe corpus %s must not be empty", path)
	}
	return corpus
}

func loadLabeledProbeCorpusForTest(t *testing.T, path string) LabeledProbeCorpus {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read labeled probe corpus %s: %v", path, err)
	}
	var corpus LabeledProbeCorpus
	if err := common.Unmarshal(data, &corpus); err != nil {
		t.Fatalf("parse labeled probe corpus %s: %v", path, err)
	}
	if len(corpus.Cases) == 0 {
		t.Fatalf("labeled probe corpus %s must not be empty", path)
	}
	return corpus
}

func evaluateLabeledProbeCorpus(t *testing.T, corpus LabeledProbeCorpus) map[CheckKey]goldenAccuracyMetric {
	t.Helper()
	metrics, err := evaluateLabeledProbeCorpusCases(corpus)
	if err != nil {
		t.Fatal(err)
	}
	return metrics
}
