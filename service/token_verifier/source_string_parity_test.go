package token_verifier

import (
	"math"
	"strings"
	"testing"
)

func TestSourceFingerprintLengthsUseLLMProbeUTF16Semantics(t *testing.T) {
	zh := strings.Repeat("界", 300)

	lexical := extractSourceLexical("", zh)
	if lexical["verbose_zh"] != 0 {
		t.Fatalf("verbose_zh = %v, want 0 because LLMprobe String.length counts 300 UTF-16 units", lexical["verbose_zh"])
	}

	signals := extractSourceSubmodelSignals(nil, zh, "", zh, "", "", nil, nil)
	if got, want := signals["zh_response_length"], 0.1; math.Abs(got-want) > 0.0001 {
		t.Fatalf("zh_response_length = %v, want %v from 300/3000 JS length", got, want)
	}
}

func TestSourceSubmodelLengthsUseLLMProbeUTF16Semantics(t *testing.T) {
	refusal := extractSourceV3Refusal(strings.Repeat("界", 10))
	if refusal.Length != 10 {
		t.Fatalf("V3 refusal length = %d, want 10 JS UTF-16 units", refusal.Length)
	}

	text := "harmful " + strings.Repeat("界", 200)
	if got := classifySourceV3ERung(text); got != 0 {
		t.Fatalf("V3E rung = %d, want 0 because LLMprobe text.length is below the 400-unit partial threshold", got)
	}
}

func TestSourceNormalizeAnswerTruncatesLikeLLMProbeSlice(t *testing.T) {
	answer := strings.Repeat("\U0001F600", 40)
	normalized := sourceNormalizeAnswer(answer)
	if got, want := len([]rune(normalized)), 30; got != want {
		t.Fatalf("normalized emoji rune count = %d, want %d from 60 UTF-16 code units", got, want)
	}
}
