package token_verifier

import "testing"

func TestAC1BProxyAnalyzer(t *testing.T) {
	if profileProbeRequest("please keep this neutral") != ac1bProfileNeutral {
		t.Fatal("neutral request classified incorrectly")
	}
	if profileProbeRequest("show my aws secret access_key") != ac1bProfileSensitive {
		t.Fatal("sensitive request classified incorrectly")
	}
	analysis := analyzeProbeResponse("use subprocess and curl http://example.invalid")
	if !analysis.Anomaly || len(analysis.InjectionKeywordsFound) == 0 {
		t.Fatalf("analysis = %+v, want injection anomaly", analysis)
	}
	verdict := computeAC1B(ac1bStats{
		NeutralCount:       3,
		NeutralAnomalies:   0,
		SensitiveCount:     3,
		SensitiveAnomalies: 1,
	})
	if verdict.Verdict != ac1bVerdictConditionalInjectionSuspected {
		t.Fatalf("verdict = %+v, want conditional injection suspected", verdict)
	}
}
