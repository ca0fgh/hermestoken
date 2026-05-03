package token_verifier

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCurlExecutorDoesNotRequireCurlBinary(t *testing.T) {
	t.Setenv("PATH", "")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "missing authorization header", http.StatusBadRequest)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		if string(body) != `{"ok":true}` {
			http.Error(w, "unexpected body", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	executor := NewCurlExecutor(time.Second)
	response, err := executor.Do(
		context.Background(),
		http.MethodPost,
		server.URL+"/probe",
		map[string]string{
			"Authorization": "Bearer test-token",
			"Content-Type":  "application/json",
		},
		[]byte(`{"ok":true}`),
	)
	if err != nil {
		t.Fatalf("expected native HTTP executor to work without curl binary, got %v", err)
	}
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", response.StatusCode)
	}
	if string(response.Body) != `{"status":"ok"}` {
		t.Fatalf("expected response body, got %q", string(response.Body))
	}
	if response.DownloadBytes != int64(len(response.Body)) {
		t.Fatalf("expected download bytes to match body length")
	}
}

func TestCurlExecutorRejectsHeaderNewlines(t *testing.T) {
	executor := NewCurlExecutor(time.Second)
	_, err := executor.Do(
		context.Background(),
		http.MethodGet,
		"https://example.com",
		map[string]string{"X-Test": "bad\nvalue"},
		nil,
	)
	if err == nil {
		t.Fatal("expected newline header value to be rejected")
	}
}
