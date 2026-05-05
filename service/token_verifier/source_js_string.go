package token_verifier

func sourceJSStringLength(text string) int {
	units := 0
	for _, r := range text {
		if r > 0xFFFF {
			units += 2
		} else {
			units++
		}
	}
	return units
}

func sourceJSStringPrefix(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	units := 0
	end := 0
	for i, r := range text {
		width := 1
		if r > 0xFFFF {
			width = 2
		}
		if units+width > limit {
			break
		}
		units += width
		end = i + len(string(r))
	}
	if end == 0 && text != "" {
		return ""
	}
	return text[:end]
}

func sourceJSStringSuffix(text string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(text)
	units := 0
	start := len(runes)
	for i := len(runes) - 1; i >= 0; i-- {
		width := 1
		if runes[i] > 0xFFFF {
			width = 2
		}
		if units+width > limit {
			break
		}
		units += width
		start = i
	}
	return string(runes[start:])
}
