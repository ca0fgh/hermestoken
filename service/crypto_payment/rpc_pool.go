package crypto_payment

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ca0fgh/hermestoken/common"
)

// endpointUnavailableError marks a failure of the endpoint itself — a dial error,
// a timeout, an HTTP status — as opposed to a well-formed answer we happen not to
// like.
//
// Only the first kind is worth asking a different provider. A JSON-RPC error
// object such as "block range is too large" is the provider talking to us about
// our request, and retrying that against another endpoint would paper over the
// span negotiation scanEVMRange depends on.
type endpointUnavailableError struct{ err error }

func (e endpointUnavailableError) Error() string { return e.err.Error() }

func (e endpointUnavailableError) Unwrap() error { return e.err }

func endpointUnavailable(err error) error {
	if err == nil {
		return nil
	}
	return endpointUnavailableError{err: err}
}

func isEndpointUnavailable(err error) bool {
	var unavailable endpointUnavailableError
	return errors.As(err, &unavailable)
}

// endpointPool holds every RPC endpoint configured for one network and hands out
// the one currently believed good.
//
// One endpoint is a single point of failure for a receive path. Production ran
// Solana on a lone Ankr URL that turned out to be pointed at devnet, and the only
// replacement the key could reach was a public node with no SLA. A list means a
// provider can rate-limit, 403, or disappear without the chain going blind, and it
// means an endpoint caught serving the wrong chain can simply be dropped.
// reauditionInterval bounds how long a fallback may hold "current" before the
// operator's first-listed endpoint is tried again. Without it, one transient blip
// on the paid provider demotes the pool onto a free fallback permanently — BSC ran
// five days on bsc-dataseed, whose eth_getLogs always answers "limit exceeded",
// because nothing ever went back to ask the recovered primary.
const reauditionInterval = 10 * time.Minute

type endpointPool struct {
	mu        sync.Mutex
	network   string
	endpoints []string
	current   int
	// demotedAt is when current last moved off the primary. Zero while the pool is
	// on the primary.
	demotedAt time.Time
}

func newEndpointPool(network string, raw string) *endpointPool {
	return &endpointPool{network: network, endpoints: splitEndpoints(raw)}
}

// splitEndpoints accepts a comma, semicolon, or whitespace separated list so an
// operator can paste several URLs into the single option field the UI offers.
func splitEndpoints(raw string) []string {
	fields := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	endpoints := make([]string, 0, len(fields))
	seen := make(map[string]bool)
	for _, field := range fields {
		endpoint := strings.TrimRight(strings.TrimSpace(field), "/")
		if endpoint == "" || seen[endpoint] {
			continue
		}
		seen[endpoint] = true
		endpoints = append(endpoints, endpoint)
	}
	return endpoints
}

func (p *endpointPool) size() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.endpoints)
}

func (p *endpointPool) all() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.endpoints...)
}

// do runs attempt against the current endpoint and, when that endpoint is
// unavailable rather than merely disagreeing, against each remaining one.
//
// Whichever endpoint answers becomes the new current, so a healthy provider is not
// rediscovered on every single call.
func (p *endpointPool) do(attempt func(endpoint string) error) error {
	endpoints, start := p.snapshot()
	if len(endpoints) == 0 {
		return fmt.Errorf("%s has no usable RPC endpoint configured", p.network)
	}
	var lastErr error
	var lastEndpoint string
	for offset := 0; offset < len(endpoints); offset++ {
		endpoint := endpoints[(start+offset)%len(endpoints)]
		err := attempt(endpoint)
		if err == nil || !isEndpointUnavailable(err) {
			// A provider that returned a JSON-RPC error still answered; it is not the
			// thing that is broken, so it stays current.
			p.promote(endpoint)
			return err
		}
		lastErr = err
		lastEndpoint = endpoint
		if offset+1 < len(endpoints) {
			common.SysLog(fmt.Sprintf("crypto scanner %s RPC endpoint %s is unavailable (%s), trying the next one",
				p.network, redactEndpoint(endpoint), redactEndpointInText(err.Error(), endpoint)))
		}
	}
	return fmt.Errorf("every %s RPC endpoint failed, last error: %s",
		p.network, redactEndpointInText(lastErr.Error(), lastEndpoint))
}

// evict drops an endpoint that proved it is not serving the chain we configured.
// A devnet URL in a mainnet pool never becomes right, and leaving it in rotation
// means every request has a chance of quietly reading a different chain.
func (p *endpointPool) evict(endpoint string, reason string) {
	p.mu.Lock()
	remaining := make([]string, 0, len(p.endpoints))
	removed := false
	for _, candidate := range p.endpoints {
		if candidate == endpoint {
			removed = true
			continue
		}
		remaining = append(remaining, candidate)
	}
	p.endpoints = remaining
	if p.current >= len(remaining) {
		p.current = 0
	}
	left := len(remaining)
	p.mu.Unlock()

	if !removed {
		return
	}
	common.SysLog(fmt.Sprintf("crypto scanner %s dropped RPC endpoint %s: %s (%d endpoint(s) left)",
		p.network, redactEndpoint(endpoint), reason, left))
}

func (p *endpointPool) snapshot() ([]string, int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.current != 0 && time.Since(p.demotedAt) >= reauditionInterval {
		p.current = 0
		p.demotedAt = time.Time{}
	}
	return append([]string(nil), p.endpoints...), p.current
}

func (p *endpointPool) promote(endpoint string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for index, candidate := range p.endpoints {
		if candidate == endpoint {
			if index != 0 && p.current == 0 {
				p.demotedAt = time.Now()
			}
			p.current = index
			return
		}
	}
}

// resetToPrimary forgets which endpoint was current and starts the next call from
// the operator's first choice. A scanner whose cursor has stopped moving calls
// this: whatever pinned it — a fallback that answers everything except the one
// request that matters, an error phrased in words no classifier lists — starting
// over from the top is the reset that cannot be argued with.
func (p *endpointPool) resetToPrimary() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.current = 0
	p.demotedAt = time.Time{}
}

// redactEndpointInText scrubs the endpoint URL out of an error message. Go's HTTP
// client quotes the full request URL in its errors ("Post \"https://...\": ..."),
// so redacting only the label while printing err.Error() verbatim still leaked the
// Ankr API key into the system log — which is exactly how it was found leaked in
// production on 2026-07-28.
func redactEndpointInText(text string, endpoint string) string {
	if endpoint == "" {
		return text
	}
	return strings.ReplaceAll(text, endpoint, redactEndpoint(endpoint))
}

// redactEndpoint keeps the provider recognizable in logs without printing the API
// key that providers embed in the URL path.
func redactEndpoint(endpoint string) string {
	scheme := ""
	rest := endpoint
	if index := strings.Index(endpoint, "://"); index >= 0 {
		scheme = endpoint[:index+3]
		rest = endpoint[index+3:]
	}
	host := rest
	if index := strings.Index(rest, "/"); index >= 0 {
		host = rest[:index]
		return scheme + host + "/***"
	}
	return scheme + host
}
