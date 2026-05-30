package service

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ca0fgh/hermestoken/common"
)

// Reproduction + fix verification for the production incident (2026-05-27):
// non-streaming claude-sonnet-4-6 requests failed with
//
//	do request failed: Post "https://.../v1/messages": http2: timeout awaiting response headers
//
// cascading through every channel (54->60->57->...) returning 500 / 0 tokens
// after ~4 minutes, while streaming traffic on the same channels succeeded.
//
// Root cause: the SHARED relay http client set Transport.ResponseHeaderTimeout =
// RELAY_RESPONSE_HEADER_TIMEOUT (30s) and was used for BOTH stream and non-stream.
// A non-stream upstream withholds response headers until generation finishes, so
// any non-stream completion slower than 30s was aborted -> failed over -> cascade.
//
// Fix: stream and non-stream now use SEPARATE clients (service.http_client.go +
// relay/channel/api_request.go). The stream client keeps the header timeout (fast
// failover; streaming headers arrive instantly). The non-stream client has NO
// header timeout and is bounded only by an overall per-attempt timeout
// (RELAY_NONSTREAM_TIMEOUT), so a legitimately slow completion returns normally
// while a truly hung channel still fails over.
//
// Timeouts are scaled down here (prod 30s/300s) purely for test speed; the
// mechanism is identical. The tests drive the real production client builder.

const (
	reproHeaderTimeoutSecs    = 1
	reproNonStreamTimeoutSecs = 10
)

// initRelayClients sets the relay timeout globals and builds the real shared
// stream + non-stream clients exactly the way main() does, restoring originals
// after the test.
func initRelayClients(t *testing.T, headerTimeoutSecs, nonStreamTimeoutSecs int) {
	t.Helper()
	origHeader := common.RelayResponseHeaderTimeout
	origNonStream := common.RelayNonStreamTimeout
	origStream := httpClient
	origNS := nonStreamHTTPClient
	common.RelayResponseHeaderTimeout = headerTimeoutSecs
	common.RelayNonStreamTimeout = nonStreamTimeoutSecs
	t.Cleanup(func() {
		common.RelayResponseHeaderTimeout = origHeader
		common.RelayNonStreamTimeout = origNonStream
		httpClient = origStream
		nonStreamHTTPClient = origNS
	})
	InitHttpClient()
}

// slowNonStreamUpstream simulates a non-streaming LLM channel: it "generates" for
// genDuration, then sends 200 + the full JSON body in one shot. This is exactly
// how claude-sonnet-4-6 (stream:false) behaves: no headers until the answer is done.
func slowNonStreamUpstream(genDuration time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(genDuration) // upstream is still generating; no headers yet
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"msg_repro","type":"message","role":"assistant",` +
			`"content":[{"type":"text","text":"done"}],"usage":{"input_tokens":10,"output_tokens":5}}`))
	}))
}

// streamingUpstream simulates a streaming channel: 200 + headers immediately,
// then SSE chunks dribble out slowly.
func streamingUpstream(perChunk time.Duration, chunks int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK) // headers out immediately
		fl, _ := w.(http.Flusher)
		if fl != nil {
			fl.Flush()
		}
		for i := 0; i < chunks; i++ {
			time.Sleep(perChunk)
			_, _ = fmt.Fprintf(w, "data: {\"delta\":{\"text\":\"chunk-%d\"}}\n\n", i)
			if fl != nil {
				fl.Flush()
			}
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
}

func postMessages(client *http.Client, url string) (*http.Response, error) {
	body := strings.NewReader(`{"model":"claude-sonnet-4-6","stream":false,"max_tokens":1024,` +
		`"messages":[{"role":"user","content":"hi"}]}`)
	req, err := http.NewRequest(http.MethodPost, url+"/v1/messages", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return client.Do(req)
}

// THE BUG: the streaming client (header timeout) aborts a slow non-stream
// completion at the header-timeout boundary with the exact prod error. This is why
// routing non-stream onto the stream client cascaded across all channels — and why
// the fix routes non-stream onto a separate client.
func TestStreamClient_CutsSlowNonStream_isTheBug(t *testing.T) {
	upstream := slowNonStreamUpstream(3 * time.Second) // generation > 1s header timeout
	defer upstream.Close()

	initRelayClients(t, reproHeaderTimeoutSecs, reproNonStreamTimeoutSecs)
	streamClient := GetHttpClient()

	start := time.Now()
	resp, err := postMessages(streamClient, upstream.URL)
	elapsed := time.Since(start)

	if err == nil {
		resp.Body.Close()
		t.Fatalf("stream client should abort slow non-stream at the header timeout, got 200 OK")
	}
	if !strings.Contains(err.Error(), "timeout awaiting response headers") {
		t.Fatalf("expected prod signature %q, got: %v", "timeout awaiting response headers", err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("expected abort at ~1s header timeout, took %v", elapsed)
	}
	t.Logf("reproduced prod error on stream client after %v: %v", elapsed.Round(time.Millisecond), err)
}

// THE FIX: the non-stream client has no header timeout, so the same slow
// completion now returns 200 + full body instead of being cut. No more cascade.
func TestNonStreamClient_ToleratesSlowGeneration_isTheFix(t *testing.T) {
	upstream := slowNonStreamUpstream(3 * time.Second) // > 1s header timeout, < 10s overall
	defer upstream.Close()

	initRelayClients(t, reproHeaderTimeoutSecs, reproNonStreamTimeoutSecs)
	nonStreamClient := GetNonStreamHttpClient()

	if nonStreamClient == GetHttpClient() {
		t.Fatal("non-stream client must be a distinct client from the stream client")
	}

	start := time.Now()
	resp, err := postMessages(nonStreamClient, upstream.URL)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("non-stream client must tolerate slow generation (no header timeout), got: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("body read should succeed, got: %v", err)
	}
	if !strings.Contains(string(body), "msg_repro") {
		t.Fatalf("expected full upstream body, got %q", string(body))
	}
	if elapsed < 3*time.Second {
		t.Fatalf("expected to wait out the ~3s generation, returned in %v", elapsed)
	}
	t.Logf("fix verified: slow non-stream completion returned 200 after %v", elapsed.Round(time.Millisecond))
}

// The non-stream client still bounds a TRULY hung channel via the overall
// per-attempt timeout, so failover is preserved — it just no longer mistakes a
// slow-but-working completion for a hung one.
func TestNonStreamClient_StillBoundsHungChannel(t *testing.T) {
	upstream := slowNonStreamUpstream(5 * time.Second) // never returns within the 1s overall bound
	defer upstream.Close()

	initRelayClients(t, reproHeaderTimeoutSecs, 1) // non-stream overall timeout = 1s
	nonStreamClient := GetNonStreamHttpClient()

	start := time.Now()
	resp, err := postMessages(nonStreamClient, upstream.URL)
	elapsed := time.Since(start)
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected the overall non-stream timeout to abort a hung channel")
	}
	// client.Timeout surfaces as a deadline/timeout error -> doRequest wraps it as a
	// retryable do_request_failed -> next channel. Just assert it failed fast (~1s).
	if elapsed > 2*time.Second {
		t.Fatalf("expected fail-fast at the ~1s overall timeout, took %v", elapsed)
	}
	t.Logf("hung-channel failover preserved: aborted after %v: %v", elapsed.Round(time.Millisecond), err)
}

// Control: streaming on the stream client is unaffected even though total stream
// time far exceeds the header timeout — this is why prod streaming kept working.
func TestStreamingUpstream_NotAffectedByHeaderTimeout(t *testing.T) {
	upstream := streamingUpstream(400*time.Millisecond, 6) // 2.4s total > 1s header timeout
	defer upstream.Close()

	initRelayClients(t, reproHeaderTimeoutSecs, reproNonStreamTimeoutSecs)
	streamClient := GetHttpClient()

	resp, err := postMessages(streamClient, upstream.URL)
	if err != nil {
		t.Fatalf("streaming sends headers immediately; must not hit header timeout, got: %v", err)
	}
	defer resp.Body.Close()

	var chunks int
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		if strings.HasPrefix(sc.Text(), "data: {") {
			chunks++
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("streaming body read should be unbounded, got: %v", err)
	}
	if chunks != 6 {
		t.Fatalf("expected 6 streamed chunks, got %d", chunks)
	}
}
