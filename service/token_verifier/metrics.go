package token_verifier

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// All token verification metrics live on the default prometheus registry.
// Names follow {namespace}_{subsystem}_{metric}_{unit}_{type-suffix}.
const (
	metricsNamespace = "hermestoken"
	metricsSubsystem = "token_verification"
)

var (
	tasksTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystem,
			Name:      "tasks_total",
			Help:      "Total number of token verification tasks completed, labelled by final task status.",
		},
		[]string{"status"}, // success | failed
	)

	tasksByGradeTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystem,
			Name:      "tasks_by_grade_total",
			Help:      "Total successful tasks bucketed by final grade letter.",
		},
		[]string{"grade"}, // S | A | B | C | D | Fail
	)

	taskDurationSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystem,
			Name:      "task_duration_seconds",
			Help:      "Token verification task end-to-end duration.",
			Buckets:   []float64{1, 5, 10, 20, 30, 45, 60, 90, 120, 180, 300},
		},
		[]string{"baseline_source"}, // artificial_analysis | mixed | fallback_absolute | none
	)

	downgradeDetectedTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystem,
			Name:      "downgrade_detected_total",
			Help:      "Number of (provider, model) pairs flagged with suspected_downgrade=true across all completed tasks.",
		},
	)

	aaRefreshTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystem,
			Name:      "aa_refresh_total",
			Help:      "Number of Artificial Analysis baseline refresh attempts.",
		},
		[]string{"result"}, // success | failed
	)

	aaTTFTRatio = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystem,
			Name:      "aa_ttft_ratio",
			Help:      "Distribution of measured_ttft / aa_baseline_ttft across completed tasks (lower is better).",
			Buckets:   []float64{0.5, 0.75, 1.0, 1.15, 1.5, 2.0, 3.0, 5.0, 10.0},
		},
	)

	aaTPSRatio = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystem,
			Name:      "aa_tps_ratio",
			Help:      "Distribution of measured_tokens_per_second / aa_baseline_tps (higher is better).",
			Buckets:   []float64{0.1, 0.3, 0.5, 0.7, 0.85, 1.0, 1.25, 1.5, 2.0},
		},
	)

	aaBaselineModels = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystem,
			Name:      "aa_baseline_models",
			Help:      "Number of models in the cached AA baseline snapshot. -1 when no snapshot available.",
		},
		func() float64 {
			snap := getCachedSnapshot()
			if snap == nil {
				return -1
			}
			return float64(len(snap.Models))
		},
	)

	aaBaselineAgeSeconds = prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystem,
			Name:      "aa_baseline_age_seconds",
			Help:      "Seconds since the AA baseline snapshot was last refreshed. -1 when no snapshot available.",
		},
		func() float64 {
			snap := getCachedSnapshot()
			if snap == nil {
				return -1
			}
			return time.Since(snap.FetchedAt).Seconds()
		},
	)
)

func init() {
	prometheus.MustRegister(
		tasksTotal,
		tasksByGradeTotal,
		taskDurationSeconds,
		downgradeDetectedTotal,
		aaRefreshTotal,
		aaTTFTRatio,
		aaTPSRatio,
		aaBaselineModels,
		aaBaselineAgeSeconds,
	)
}

// observeTaskCompleted records the outcome of a single token verification task.
// status is "success" or "failed"; grade is empty for failed tasks.
// duration is the elapsed wall time. report may be nil for failed tasks.
func observeTaskCompleted(status string, duration time.Duration, report *ReportSummary) {
	tasksTotal.WithLabelValues(status).Inc()

	baselineSource := BaselineSourceNone
	if report != nil {
		if report.BaselineSource != "" {
			baselineSource = report.BaselineSource
		}
		if status == "success" && report.Grade != "" {
			tasksByGradeTotal.WithLabelValues(report.Grade).Inc()
		}
		for _, mi := range report.ModelIdentity {
			if mi.SuspectedDowngrade {
				downgradeDetectedTotal.Inc()
			}
		}
		for _, m := range report.Models {
			if m.Baseline == nil {
				continue
			}
			if m.Baseline.TTFTRatio > 0 {
				aaTTFTRatio.Observe(m.Baseline.TTFTRatio)
			}
			if m.Baseline.TPSRatio > 0 {
				aaTPSRatio.Observe(m.Baseline.TPSRatio)
			}
		}
	}
	taskDurationSeconds.WithLabelValues(baselineSource).Observe(duration.Seconds())
}

// observeAARefresh records the outcome of one Artificial Analysis sync attempt.
func observeAARefresh(result string) {
	aaRefreshTotal.WithLabelValues(result).Inc()
}
