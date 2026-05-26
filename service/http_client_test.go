package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ca0fgh/hermestoken/common"
)

// newHeaderTimeoutClient builds an isolated client whose transport has the relay
// response-header timeout applied from the (temporarily overridden) global.
func newHeaderTimeoutClient(t *testing.T, headerTimeoutSecs int) *http.Client {
	t.Helper()
	orig := common.RelayResponseHeaderTimeout
	common.RelayResponseHeaderTimeout = headerTimeoutSecs
	t.Cleanup(func() { common.RelayResponseHeaderTimeout = orig })
	tr := &http.Transport{}
	applyRelayTransportTimeouts(tr)
	return &http.Client{Transport: tr}
}

func TestApplyRelayTransportTimeouts(t *testing.T) {
	orig := common.RelayResponseHeaderTimeout
	t.Cleanup(func() { common.RelayResponseHeaderTimeout = orig })

	common.RelayResponseHeaderTimeout = 0
	tr := &http.Transport{}
	applyRelayTransportTimeouts(tr)
	if tr.ResponseHeaderTimeout != 0 {
		t.Fatalf("expected unbounded (0) when disabled, got %v", tr.ResponseHeaderTimeout)
	}

	common.RelayResponseHeaderTimeout = 7
	tr2 := &http.Transport{}
	applyRelayTransportTimeouts(tr2)
	if tr2.ResponseHeaderTimeout != 7*time.Second {
		t.Fatalf("expected 7s, got %v", tr2.ResponseHeaderTimeout)
	}
}

// A hung upstream (no headers within the timeout) must fail fast so the retry
// loop can move to the next channel, instead of blocking ~60s for the upstream.
func TestResponseHeaderTimeout_FailsFastOnSlowHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // delay headers beyond the 1s timeout
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()

	client := newHeaderTimeoutClient(t, 1)
	start := time.Now()
	resp, err := client.Get(server.URL)
	elapsed := time.Since(start)
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected a response-header timeout error, got success")
	}
	if elapsed > 1500*time.Millisecond {
		t.Fatalf("expected fail-fast (~1s), took %v", elapsed)
	}
}

// Headers arrive immediately, then the body streams slowly. ResponseHeaderTimeout
// must NOT abort this — proving long streaming responses are never truncated.
func TestResponseHeaderTimeout_DoesNotTruncateSlowBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		time.Sleep(2 * time.Second) // longer than the 1s header timeout
		_, _ = w.Write([]byte("late-body"))
	}))
	defer server.Close()

	client := newHeaderTimeoutClient(t, 1)
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("headers arrived in time; request must succeed, got %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("body read should be unbounded, got %v", err)
	}
	if string(body) != "late-body" {
		t.Fatalf("expected full body, got %q", string(body))
	}
}

// When disabled (0), slow headers are tolerated (current production behavior).
func TestResponseHeaderTimeout_DisabledWaits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newHeaderTimeoutClient(t, 0) // disabled
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("disabled timeout should wait, got %v", err)
	}
	resp.Body.Close()
}
