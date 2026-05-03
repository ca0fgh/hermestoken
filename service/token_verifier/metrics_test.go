package token_verifier

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	dto "github.com/prometheus/client_model/go"
)

// histogramSampleCount returns the cumulative observation count of a single
// Histogram (not HistogramVec). testutil.CollectAndCount counts series, not
// samples; we want the sample total to assert "this histogram saw N pushes".
func histogramSampleCount(t *testing.T, h prometheus.Histogram) uint64 {
	t.Helper()
	var m dto.Metric
	if err := h.Write(&m); err != nil {
		t.Fatalf("histogram.Write: %v", err)
	}
	if m.Histogram == nil || m.Histogram.SampleCount == nil {
		return 0
	}
	return *m.Histogram.SampleCount
}

func TestObserveTaskCompletedSuccess(t *testing.T) {
	resetVecsForTest()

	beforeDowngrade := testutil.ToFloat64(downgradeDetectedTotal)
	beforeTTFT := histogramSampleCount(t, aaTTFTRatio)
	beforeTPS := histogramSampleCount(t, aaTPSRatio)

	report := &ReportSummary{
		Score:          88,
		Grade:          "A",
		BaselineSource: BaselineSourceAA,
		Models: []ModelSummary{
			{
				Provider:  "openai",
				ModelName: "gpt-4o",
				Baseline: &ModelBaselineRef{
					Source:    BaselineSourceAA,
					TTFTRatio: 1.2,
					TPSRatio:  0.85,
				},
			},
		},
		ModelIdentity: []ModelIdentitySummary{
			{Provider: "openai", ClaimedModel: "gpt-4o", IdentityConfidence: 95, SuspectedDowngrade: false},
		},
	}
	observeTaskCompleted("success", 12*time.Second, report)

	if got := testutil.ToFloat64(tasksTotal.WithLabelValues("success")); got != 1 {
		t.Errorf("tasks_total{status=success}=%v, want 1", got)
	}
	if got := testutil.ToFloat64(tasksByGradeTotal.WithLabelValues("A")); got != 1 {
		t.Errorf("tasks_by_grade_total{grade=A}=%v, want 1", got)
	}
	if got := testutil.ToFloat64(downgradeDetectedTotal); got != beforeDowngrade {
		t.Errorf("downgrade_detected_total moved from %v to %v on a healthy report", beforeDowngrade, got)
	}
	if got := histogramSampleCount(t, aaTTFTRatio); got != beforeTTFT+1 {
		t.Errorf("aa_ttft_ratio samples: got %d, want %d", got, beforeTTFT+1)
	}
	if got := histogramSampleCount(t, aaTPSRatio); got != beforeTPS+1 {
		t.Errorf("aa_tps_ratio samples: got %d, want %d", got, beforeTPS+1)
	}
}

func TestObserveTaskCompletedFailedNoReport(t *testing.T) {
	resetVecsForTest()

	observeTaskCompleted("failed", 3*time.Second, nil)

	if got := testutil.ToFloat64(tasksTotal.WithLabelValues("failed")); got != 1 {
		t.Errorf("tasks_total{status=failed}=%v, want 1", got)
	}
	if got := testutil.CollectAndCount(tasksByGradeTotal); got != 0 {
		t.Errorf("tasks_by_grade_total should have 0 series after failed task with no report, got %d", got)
	}
}

func TestObserveTaskCompletedRecordsDowngradeAcrossModels(t *testing.T) {
	resetVecsForTest()

	before := testutil.ToFloat64(downgradeDetectedTotal)

	report := &ReportSummary{
		Score:          50,
		Grade:          "C",
		BaselineSource: BaselineSourceMixed,
		ModelIdentity: []ModelIdentitySummary{
			{Provider: "openai", ClaimedModel: "gpt-4o", SuspectedDowngrade: true},
			{Provider: "openai", ClaimedModel: "gpt-4.1", SuspectedDowngrade: true},
			{Provider: "openai", ClaimedModel: "gpt-3.5", SuspectedDowngrade: false},
		},
	}
	observeTaskCompleted("success", 20*time.Second, report)

	if got := testutil.ToFloat64(downgradeDetectedTotal); got != before+2 {
		t.Errorf("downgrade_detected_total: got %v, want %v (one per flagged identity entry)", got, before+2)
	}
}

func TestObserveAARefresh(t *testing.T) {
	resetVecsForTest()

	observeAARefresh("success")
	observeAARefresh("success")
	observeAARefresh("failed")

	if got := testutil.ToFloat64(aaRefreshTotal.WithLabelValues("success")); got != 2 {
		t.Errorf("aa_refresh_total{result=success}=%v, want 2", got)
	}
	if got := testutil.ToFloat64(aaRefreshTotal.WithLabelValues("failed")); got != 1 {
		t.Errorf("aa_refresh_total{result=failed}=%v, want 1", got)
	}
}

// resetVecsForTest resets only the *Vec collectors which support Reset().
// Plain Counter and Histograms cannot be reset; tests handle them via deltas.
func resetVecsForTest() {
	tasksTotal.Reset()
	tasksByGradeTotal.Reset()
	taskDurationSeconds.Reset()
	aaRefreshTotal.Reset()
}
