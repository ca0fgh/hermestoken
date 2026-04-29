package constant

import (
	"net/http"
	"strings"
)

const (
	RelayModeUnknown = iota
	RelayModeChatCompletions
	RelayModeCompletions
	RelayModeEmbeddings
	RelayModeModerations
	RelayModeImagesGenerations
	RelayModeImagesEdits
	RelayModeEdits

	RelayModeMidjourneyImagine
	RelayModeMidjourneyDescribe
	RelayModeMidjourneyBlend
	RelayModeMidjourneyChange
	RelayModeMidjourneySimpleChange
	RelayModeMidjourneyNotify
	RelayModeMidjourneyTaskFetch
	RelayModeMidjourneyTaskImageSeed
	RelayModeMidjourneyTaskFetchByCondition
	RelayModeMidjourneyAction
	RelayModeMidjourneyModal
	RelayModeMidjourneyShorten
	RelayModeSwapFace
	RelayModeMidjourneyUpload
	RelayModeMidjourneyVideo
	RelayModeMidjourneyEdits

	RelayModeAudioSpeech        // tts
	RelayModeAudioTranscription // whisper
	RelayModeAudioTranslation   // whisper

	RelayModeSunoFetch
	RelayModeSunoFetchByID
	RelayModeSunoSubmit

	RelayModeVideoFetchByID
	RelayModeVideoSubmit

	RelayModeRerank

	RelayModeResponses

	RelayModeRealtime

	RelayModeGemini

	RelayModeResponsesCompact
)

func Path2RelayMode(path string) int {
	if canonicalPath, ok := CanonicalOpenAIPath(path); ok {
		path = canonicalPath
	}
	relayMode := RelayModeUnknown
	if relayPathHasPrefix(path, "/v1/chat/completions") || relayPathHasPrefix(path, "/pg/chat/completions") {
		relayMode = RelayModeChatCompletions
	} else if relayPathHasPrefix(path, "/v1/completions") {
		relayMode = RelayModeCompletions
	} else if relayPathHasPrefix(path, "/v1/embeddings") {
		relayMode = RelayModeEmbeddings
	} else if strings.HasSuffix(path, "embeddings") {
		relayMode = RelayModeEmbeddings
	} else if relayPathHasPrefix(path, "/v1/moderations") {
		relayMode = RelayModeModerations
	} else if relayPathHasPrefix(path, "/v1/images/generations") {
		relayMode = RelayModeImagesGenerations
	} else if relayPathHasPrefix(path, "/v1/images/edits") {
		relayMode = RelayModeImagesEdits
	} else if relayPathHasPrefix(path, "/v1/edits") {
		relayMode = RelayModeEdits
	} else if relayPathHasPrefix(path, "/v1/responses/compact") {
		relayMode = RelayModeResponsesCompact
	} else if relayPathHasPrefix(path, "/v1/responses") {
		relayMode = RelayModeResponses
	} else if relayPathHasPrefix(path, "/v1/audio/speech") {
		relayMode = RelayModeAudioSpeech
	} else if relayPathHasPrefix(path, "/v1/audio/transcriptions") {
		relayMode = RelayModeAudioTranscription
	} else if relayPathHasPrefix(path, "/v1/audio/translations") {
		relayMode = RelayModeAudioTranslation
	} else if relayPathHasPrefix(path, "/v1/rerank") {
		relayMode = RelayModeRerank
	} else if relayPathHasPrefix(path, "/v1/realtime") {
		relayMode = RelayModeRealtime
	} else if relayPathHasPrefix(path, "/v1beta/models") || relayPathHasPrefix(path, "/v1/models") {
		relayMode = RelayModeGemini
	} else if videoMode := Path2RelayVideo("", path); videoMode != RelayModeUnknown {
		relayMode = videoMode
	} else if strings.HasPrefix(path, "/mj") {
		relayMode = Path2RelayModeMidjourney(path)
	}
	return relayMode
}

func CanonicalOpenAIPath(path string) (string, bool) {
	if idx := strings.Index(path, "?"); idx >= 0 {
		path = path[:idx]
	}
	if path == "" || strings.HasPrefix(path, "/v1/") || strings.HasPrefix(path, "/v1beta/") {
		return "", false
	}
	rootPaths := []string{
		"/chat/completions",
		"/completions",
		"/responses/compact",
		"/responses",
		"/embeddings",
		"/moderations",
		"/images/generations",
		"/images/edits",
		"/images/variations",
		"/edits",
		"/audio/speech",
		"/audio/transcriptions",
		"/audio/translations",
		"/rerank",
		"/models",
		"/engines",
		"/files",
		"/fine-tunes",
	}
	for _, rootPath := range rootPaths {
		if relayPathHasPrefix(path, rootPath) {
			return "/v1" + path, true
		}
	}
	return "", false
}

func relayPathHasPrefix(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func Path2RelayVideo(method, path string) int {
	if idx := strings.Index(path, "?"); idx >= 0 {
		path = path[:idx]
	}
	method = strings.ToUpper(strings.TrimSpace(method))

	if isVideoSubmitPath(path) {
		if method == "" || method == http.MethodPost {
			return RelayModeVideoSubmit
		}
		return RelayModeUnknown
	}

	if isVideoFetchPath(path) {
		if method == "" || method == http.MethodGet {
			return RelayModeVideoFetchByID
		}
		return RelayModeUnknown
	}

	return RelayModeUnknown
}

func isVideoSubmitPath(path string) bool {
	if strings.HasPrefix(path, "/v1/videos/") && strings.HasSuffix(path, "/remix") {
		return true
	}
	if path == "/v1/videos" || path == "/v1/video/generations" {
		return true
	}
	if path == "/kling/v1/videos/text2video" || path == "/kling/v1/videos/image2video" {
		return true
	}
	return false
}

func isVideoFetchPath(path string) bool {
	if strings.HasPrefix(path, "/v1/videos/") && !strings.HasSuffix(path, "/remix") {
		return true
	}
	if strings.HasPrefix(path, "/v1/video/generations/") {
		return true
	}
	if strings.HasPrefix(path, "/kling/v1/videos/text2video/") ||
		strings.HasPrefix(path, "/kling/v1/videos/image2video/") {
		return true
	}

	return false
}

func Path2RelayModeMidjourney(path string) int {
	relayMode := RelayModeUnknown
	if strings.HasSuffix(path, "/mj/submit/action") {
		// midjourney plus
		relayMode = RelayModeMidjourneyAction
	} else if strings.HasSuffix(path, "/mj/submit/modal") {
		// midjourney plus
		relayMode = RelayModeMidjourneyModal
	} else if strings.HasSuffix(path, "/mj/submit/shorten") {
		// midjourney plus
		relayMode = RelayModeMidjourneyShorten
	} else if strings.HasSuffix(path, "/mj/insight-face/swap") {
		// midjourney plus
		relayMode = RelayModeSwapFace
	} else if strings.HasSuffix(path, "/submit/upload-discord-images") {
		// midjourney plus
		relayMode = RelayModeMidjourneyUpload
	} else if strings.HasSuffix(path, "/mj/submit/imagine") {
		relayMode = RelayModeMidjourneyImagine
	} else if strings.HasSuffix(path, "/mj/submit/video") {
		relayMode = RelayModeMidjourneyVideo
	} else if strings.HasSuffix(path, "/mj/submit/edits") {
		relayMode = RelayModeMidjourneyEdits
	} else if strings.HasSuffix(path, "/mj/submit/blend") {
		relayMode = RelayModeMidjourneyBlend
	} else if strings.HasSuffix(path, "/mj/submit/describe") {
		relayMode = RelayModeMidjourneyDescribe
	} else if strings.HasSuffix(path, "/mj/notify") {
		relayMode = RelayModeMidjourneyNotify
	} else if strings.HasSuffix(path, "/mj/submit/change") {
		relayMode = RelayModeMidjourneyChange
	} else if strings.HasSuffix(path, "/mj/submit/simple-change") {
		relayMode = RelayModeMidjourneyChange
	} else if strings.HasSuffix(path, "/fetch") {
		relayMode = RelayModeMidjourneyTaskFetch
	} else if strings.HasSuffix(path, "/image-seed") {
		relayMode = RelayModeMidjourneyTaskImageSeed
	} else if strings.HasSuffix(path, "/list-by-condition") {
		relayMode = RelayModeMidjourneyTaskFetchByCondition
	}
	return relayMode
}

func Path2RelaySuno(method, path string) int {
	relayMode := RelayModeUnknown
	if method == http.MethodPost && strings.HasSuffix(path, "/fetch") {
		relayMode = RelayModeSunoFetch
	} else if method == http.MethodGet && strings.Contains(path, "/fetch/") {
		relayMode = RelayModeSunoFetchByID
	} else if strings.Contains(path, "/submit/") {
		relayMode = RelayModeSunoSubmit
	}
	return relayMode
}
