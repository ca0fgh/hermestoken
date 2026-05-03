package token_verifier

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const curlMetaMarker = "\n__TOKEN_VERIFIER_CURL_META__"

type CurlExecutor struct {
	Timeout time.Duration
}

type CurlResponse struct {
	StatusCode    int
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
		return nil, errors.New("curl target url is empty")
	}
	timeout := e.Timeout
	if timeout <= 0 {
		timeout = defaultVerifierHTTPTimeout
	}
	args := []string{
		"-sS",
		"--no-buffer",
		"--max-time", strconv.FormatFloat(timeout.Seconds(), 'f', 0, 64),
		"-X", method,
	}
	for key, value := range headers {
		if strings.TrimSpace(key) == "" {
			continue
		}
		args = append(args, "-H", key+": "+value)
	}
	if body != nil {
		args = append(args, "--data-binary", "@-")
	}
	args = append(args,
		"-w", curlMetaMarker+"%{http_code}|%{time_total}|%{time_starttransfer}|%{size_download}",
		targetURL,
	)

	cmd := exec.CommandContext(ctx, "curl", args...)
	if body != nil {
		cmd.Stdin = bytes.NewReader(body)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("curl failed: %s", message)
	}
	return parseCurlOutput(stdout.Bytes())
}

func parseCurlOutput(output []byte) (*CurlResponse, error) {
	idx := bytes.LastIndex(output, []byte(curlMetaMarker))
	if idx < 0 {
		return nil, errors.New("curl output missing metadata")
	}
	body := bytes.TrimSuffix(output[:idx], []byte("\n"))
	meta := strings.TrimSpace(string(output[idx+len(curlMetaMarker):]))
	parts := strings.Split(meta, "|")
	if len(parts) != 4 {
		return nil, fmt.Errorf("invalid curl metadata: %s", meta)
	}
	statusCode, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid curl http code: %w", err)
	}
	totalSeconds, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid curl total time: %w", err)
	}
	ttftSeconds, err := strconv.ParseFloat(parts[2], 64)
	if err != nil {
		return nil, fmt.Errorf("invalid curl start transfer time: %w", err)
	}
	downloadBytes, _ := strconv.ParseInt(parts[3], 10, 64)
	return &CurlResponse{
		StatusCode:    statusCode,
		Body:          append([]byte(nil), body...),
		LatencyMs:     int64(totalSeconds * 1000),
		TTFTMs:        int64(ttftSeconds * 1000),
		DownloadBytes: downloadBytes,
	}, nil
}
