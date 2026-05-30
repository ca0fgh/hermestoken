package common

import (
	"strings"
	"testing"
)

func TestDefaultFooterHTMLIncludesQQCommunityLinkOnly(t *testing.T) {
	if !strings.Contains(DefaultFooterHTML, "https://qm.qq.com/q/SE1YlMkygq") {
		t.Fatalf("default footer should include QQ community link")
	}
	if strings.Contains(DefaultFooterHTML, "https://t.me/") || strings.Contains(DefaultFooterHTML, "Telegram") {
		t.Fatalf("default footer should not include Telegram community link")
	}
	if !strings.Contains(DefaultFooterHTML, "data:image/svg+xml;base64,") {
		t.Fatalf("default footer should embed svg icons")
	}
}
