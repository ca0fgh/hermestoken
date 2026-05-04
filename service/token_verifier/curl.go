package token_verifier

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptrace"
	"strings"
	"time"
)

type CurlExecutor struct {
	Timeout time.Duration
}

type CurlResponse struct {
	StatusCode    int
	Headers       map[string]string
	Body          []byte
	LatencyMs     int64
	TTFTMs        int64
	DownloadBytes int64
}

func NewCurlExecutor(timeout time.Duration) *CurlExecutor {
	if timeout <= 0 {
		timeout = defaultVerifierHTTPTimeout
	}
	return &CurlExecutor{Timeout: timeout}
}

func (e *CurlExecutor) Do(ctx context.Context, method string, targetURL string, headers map[string]string, body []byte) (*CurlResponse, error) {
	if strings.TrimSpace(targetURL) == "" {
		return nil, errors.New("http target url is empty")
	}
	timeout := e.Timeout
	if timeout <= 0 {
		timeout = defaultVerifierHTTPTimeout
	}
	if ctx == nil {
		ctx = context.Background()
	}

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	request, err := http.NewRequestWithContext(ctx, method, targetURL, bodyReader)
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		if strings.TrimSpace(key) == "" {
			continue
		}
		if strings.ContainsAny(key, "\r\n") || strings.ContainsAny(value, "\r\n") {
			return nil, errors.New("http header contains invalid newline")
		}
		request.Header.Set(key, value)
	}
	request.ContentLength = int64(len(body))

	startedAt := time.Now()
	firstByteAt := time.Time{}
	trace := &httptrace.ClientTrace{
		GotFirstResponseByte: func() {
			if firstByteAt.IsZero() {
				firstByteAt = time.Now()
			}
		},
	}
	request = request.WithContext(httptrace.WithClientTrace(request.Context(), trace))
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if firstByteAt.IsZero() {
		firstByteAt = time.Now()
	}
	responseHeaders := make(map[string]string, len(response.Header))
	for key, values := range response.Header {
		if len(values) == 0 {
			continue
		}
		responseHeaders[strings.ToLower(key)] = strings.Join(values, ",")
	}
	return &CurlResponse{
		StatusCode:    response.StatusCode,
		Headers:       responseHeaders,
		Body:          responseBody,
		LatencyMs:     time.Since(startedAt).Milliseconds(),
		TTFTMs:        firstByteAt.Sub(startedAt).Milliseconds(),
		DownloadBytes: int64(len(responseBody)),
	}, nil
}
