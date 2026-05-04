package token_verifier

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
)

func sortedStringKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func firstRegexSubmatch(text string, pattern string) string {
	match := regexp.MustCompile(pattern).FindStringSubmatch(text)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func endpointWithSuffix(baseURL string, suffix string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if base == "" {
		return suffix
	}
	if strings.HasSuffix(base, suffix) {
		return base
	}
	parsed, err := url.Parse(base)
	if err == nil && strings.HasSuffix(parsed.Path, suffix) {
		return base
	}
	return base + suffix
}
