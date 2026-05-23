package model

import (
	"os"
	"strings"
	"testing"
)

func TestModelMainHasNoHardcodedDefaultRootPassword(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}

	for _, forbidden := range []string{
		"password is 123456",
		`Password2Hash("123456")`,
	} {
		if strings.Contains(string(source), forbidden) {
			t.Fatalf("model/main.go must not contain hardcoded default root password marker %q", forbidden)
		}
	}
}
