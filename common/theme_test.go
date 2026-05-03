package common

import "testing"

func TestDefaultThemeIsClassic(t *testing.T) {
	if got := GetTheme(); got != "classic" {
		t.Fatalf("theme = %q, want classic", got)
	}
}
