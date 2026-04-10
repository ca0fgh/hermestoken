package router

import "testing"

func TestResolveFrontendRouteMode(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name               string
		frontendBaseURL    string
		isMasterNode       bool
		hasEmbeddedAssets  bool
		expectedMode       frontendRouteMode
		expectedBaseURL    string
		expectedWasIgnored bool
	}{
		{
			name:              "embedded frontend when assets exist",
			hasEmbeddedAssets: true,
			expectedMode:      frontendRouteModeEmbedded,
		},
		{
			name:               "redirect when external frontend configured",
			frontendBaseURL:    "https://ui.example.com/",
			hasEmbeddedAssets:  true,
			expectedMode:       frontendRouteModeRedirect,
			expectedBaseURL:    "https://ui.example.com",
			expectedWasIgnored: false,
		},
		{
			name:               "master node ignores external frontend and uses embedded assets",
			frontendBaseURL:    "https://ui.example.com/",
			isMasterNode:       true,
			hasEmbeddedAssets:  true,
			expectedMode:       frontendRouteModeEmbedded,
			expectedBaseURL:    "",
			expectedWasIgnored: true,
		},
		{
			name:               "disabled when no embedded assets or external frontend",
			expectedMode:       frontendRouteModeDisabled,
			expectedBaseURL:    "",
			expectedWasIgnored: false,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			mode, baseURL, wasIgnored := resolveFrontendRouteMode(testCase.frontendBaseURL, testCase.isMasterNode, testCase.hasEmbeddedAssets)

			if mode != testCase.expectedMode {
				t.Fatalf("expected mode %v, got %v", testCase.expectedMode, mode)
			}
			if baseURL != testCase.expectedBaseURL {
				t.Fatalf("expected baseURL %q, got %q", testCase.expectedBaseURL, baseURL)
			}
			if wasIgnored != testCase.expectedWasIgnored {
				t.Fatalf("expected ignored=%t, got %t", testCase.expectedWasIgnored, wasIgnored)
			}
		})
	}
}
