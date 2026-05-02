package token_verifier

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
)

const (
	aaModelsEndpoint = "https://artificialanalysis.ai/api/v2/data/llms/models"
	aaCacheRedisKey  = "token_verifier:aa_baseline"
	aaCacheTTL       = 14 * 24 * time.Hour
	aaHTTPTimeout    = 20 * time.Second
)

type AABaselineModel struct {
	ID        string  `json:"id"`
	Slug      string  `json:"slug"`
	Name      string  `json:"name"`
	TTFTSec   float64 `json:"median_time_to_first_token_seconds"`
	OutputTPS float64 `json:"median_output_tokens_per_second"`
}

type AABaselineSnapshot struct {
	FetchedAt time.Time                   `json:"fetched_at"`
	Models    map[string]*AABaselineModel `json:"models"`
}

var (
	aaSnapshotMu     sync.RWMutex
	aaSnapshotCached *AABaselineSnapshot
)

func aaApiKey() string {
	return strings.TrimSpace(os.Getenv("AA_API_KEY"))
}

// AABaselineEnabled reports whether AA-baseline performance scoring is active.
// Defaults to true when AA_API_KEY is set; can be force-disabled via AA_BASELINE_ENABLED=false.
func AABaselineEnabled() bool {
	if aaApiKey() == "" {
		return false
	}
	explicit := strings.TrimSpace(os.Getenv("AA_BASELINE_ENABLED"))
	if explicit == "" {
		return true
	}
	enabled, err := strconv.ParseBool(explicit)
	if err != nil {
		return true
	}
	return enabled
}

// AARefreshInterval returns the cron interval for syncing AA snapshots.
// Default 24h; configurable via AA_REFRESH_INTERVAL_HOURS.
func AARefreshInterval() time.Duration {
	raw := strings.TrimSpace(os.Getenv("AA_REFRESH_INTERVAL_HOURS"))
	if raw == "" {
		return 24 * time.Hour
	}
	hours, err := strconv.Atoi(raw)
	if err != nil || hours <= 0 {
		return 24 * time.Hour
	}
	return time.Duration(hours) * time.Hour
}

// FetchAABaseline pulls the latest LLM perf snapshot from Artificial Analysis.
func FetchAABaseline(ctx context.Context) (*AABaselineSnapshot, error) {
	key := aaApiKey()
	if key == "" {
		return nil, errors.New("AA_API_KEY not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, aaModelsEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", key)
	req.Header.Set("Accept", "application/json")
	client := &http.Client{Timeout: aaHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("AA request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("AA read body failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("AA returned http %d: %s", resp.StatusCode, truncate(string(body), 256))
	}
	return parseAABaselineResponse(body)
}

func parseAABaselineResponse(body []byte) (*AABaselineSnapshot, error) {
	var raw struct {
		Status int `json:"status"`
		Data   []struct {
			ID                            string  `json:"id"`
			Slug                          string  `json:"slug"`
			Name                          string  `json:"name"`
			MedianOutputTokensPerSecond   float64 `json:"median_output_tokens_per_second"`
			MedianTimeToFirstTokenSeconds float64 `json:"median_time_to_first_token_seconds"`
		} `json:"data"`
	}
	if err := common.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode AA response: %w", err)
	}
	snap := &AABaselineSnapshot{
		FetchedAt: time.Now(),
		Models:    make(map[string]*AABaselineModel, len(raw.Data)*2),
	}
	for _, m := range raw.Data {
		if strings.TrimSpace(m.Slug) == "" && strings.TrimSpace(m.Name) == "" {
			continue
		}
		bm := &AABaselineModel{
			ID:        m.ID,
			Slug:      m.Slug,
			Name:      m.Name,
			TTFTSec:   m.MedianTimeToFirstTokenSeconds,
			OutputTPS: m.MedianOutputTokensPerSecond,
		}
		for _, key := range aaIndexKeys(m.Slug, m.Name) {
			if _, exists := snap.Models[key]; !exists {
				snap.Models[key] = bm
			}
		}
	}
	return snap, nil
}

// aaIndexKeys returns the normalized keys we use to look up a baseline by user-supplied model name.
// Reuses canonicalModelName so the lookup tolerates date suffixes, "-latest"/"-preview" tails, and underscore variants.
func aaIndexKeys(slug, name string) []string {
	seen := make(map[string]struct{}, 4)
	out := make([]string, 0, 4)
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		normalized := canonicalModelName(s)
		if normalized == "" {
			return
		}
		if _, ok := seen[normalized]; ok {
			return
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	add(slug)
	add(name)
	return out
}

// LookupAABaseline returns the cached baseline entry matching the given measured model name,
// or nil when the feature is disabled, the cache is empty, or no match exists.
func LookupAABaseline(modelName string) *AABaselineModel {
	if !AABaselineEnabled() {
		return nil
	}
	snap := getCachedSnapshot()
	if snap == nil {
		return nil
	}
	key := canonicalModelName(modelName)
	if key == "" {
		return nil
	}
	return snap.Models[key]
}

// CachedAASnapshotInfo exposes a lightweight summary for diagnostics endpoints (no payload dump).
func CachedAASnapshotInfo() (fetchedAt time.Time, modelCount int, present bool) {
	snap := getCachedSnapshot()
	if snap == nil {
		return time.Time{}, 0, false
	}
	return snap.FetchedAt, len(snap.Models), true
}

func getCachedSnapshot() *AABaselineSnapshot {
	aaSnapshotMu.RLock()
	snap := aaSnapshotCached
	aaSnapshotMu.RUnlock()
	if snap != nil {
		return snap
	}
	if !common.RedisEnabled {
		return nil
	}
	raw, err := common.RedisGet(aaCacheRedisKey)
	if err != nil || raw == "" {
		return nil
	}
	var loaded AABaselineSnapshot
	if err := common.UnmarshalJsonStr(raw, &loaded); err != nil {
		return nil
	}
	aaSnapshotMu.Lock()
	aaSnapshotCached = &loaded
	aaSnapshotMu.Unlock()
	return &loaded
}

// StoreAABaselineSnapshot caches a snapshot in process memory and (when enabled) Redis.
func StoreAABaselineSnapshot(snap *AABaselineSnapshot) error {
	if snap == nil {
		return errors.New("nil snapshot")
	}
	aaSnapshotMu.Lock()
	aaSnapshotCached = snap
	aaSnapshotMu.Unlock()
	if !common.RedisEnabled {
		return nil
	}
	data, err := common.Marshal(snap)
	if err != nil {
		return err
	}
	return common.RedisSet(aaCacheRedisKey, string(data), aaCacheTTL)
}

// SetAABaselineSnapshotForTest replaces the in-memory cache. Test-only.
func SetAABaselineSnapshotForTest(snap *AABaselineSnapshot) {
	aaSnapshotMu.Lock()
	defer aaSnapshotMu.Unlock()
	aaSnapshotCached = snap
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
