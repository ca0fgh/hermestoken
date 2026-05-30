package service

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/setting/system_setting"

	"golang.org/x/net/proxy"
)

var (
	// httpClient is the STREAMING relay client: its transport carries
	// ResponseHeaderTimeout for fast hung-upstream failover. Streaming upstreams
	// emit headers immediately, so this never truncates a long stream.
	httpClient *http.Client
	// nonStreamHTTPClient is the NON-STREAM relay client: it has NO
	// ResponseHeaderTimeout (a non-stream upstream withholds headers until the whole
	// completion is generated) and is instead bounded by an overall per-attempt
	// Timeout (RelayNonStreamTimeout) so a truly hung channel still fails over.
	nonStreamHTTPClient *http.Client
	proxyClientLock     sync.Mutex
	proxyClients        = make(map[string]*http.Client)
)

func checkRedirect(req *http.Request, via []*http.Request) error {
	fetchSetting := system_setting.GetFetchSetting()
	urlStr := req.URL.String()
	if err := common.ValidateURLWithFetchSetting(urlStr, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, fetchSetting.ApplyIPFilterForDomain); err != nil {
		return fmt.Errorf("redirect to %s blocked: %v", urlStr, err)
	}
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	return nil
}

// applyRelayTransportTimeouts sets the per-attempt response-header timeout on a
// STREAMING relay transport. ResponseHeaderTimeout bounds only the wait for response
// headers, not body/stream reads, so it fails over fast on a hung upstream
// without ever truncating a long streaming response. 0 leaves it unbounded.
//
// It must NOT be used for non-stream transports: a non-stream upstream sends its
// headers only after generation completes, so a header timeout there aborts every
// legitimately slow completion. Non-stream uses an overall client.Timeout instead
// (see nonStreamClientTimeout).
func applyRelayTransportTimeouts(transport *http.Transport) {
	if common.RelayResponseHeaderTimeout > 0 {
		transport.ResponseHeaderTimeout = time.Duration(common.RelayResponseHeaderTimeout) * time.Second
	}
}

// streamClientTimeout returns the overall Timeout for the streaming client.
// 0 means unbounded — required so long streams are never truncated.
func streamClientTimeout() time.Duration {
	if common.RelayTimeout > 0 {
		return time.Duration(common.RelayTimeout) * time.Second
	}
	return 0
}

// nonStreamClientTimeout returns the overall per-attempt Timeout for the non-stream
// client. Prefer RelayNonStreamTimeout; if unset (0) fall back to RelayTimeout; if
// that is also 0, unbounded.
func nonStreamClientTimeout() time.Duration {
	if common.RelayNonStreamTimeout > 0 {
		return time.Duration(common.RelayNonStreamTimeout) * time.Second
	}
	if common.RelayTimeout > 0 {
		return time.Duration(common.RelayTimeout) * time.Second
	}
	return 0
}

// newRelayTransport builds a relay transport. The response-header timeout is
// applied only for streaming; non-stream transports never carry it.
func newRelayTransport(stream bool) *http.Transport {
	transport := &http.Transport{
		MaxIdleConns:        common.RelayMaxIdleConns,
		MaxIdleConnsPerHost: common.RelayMaxIdleConnsPerHost,
		ForceAttemptHTTP2:   true,
		Proxy:               http.ProxyFromEnvironment, // Support HTTP_PROXY, HTTPS_PROXY, NO_PROXY env vars
	}
	if stream {
		applyRelayTransportTimeouts(transport)
	}
	if common.TLSInsecureSkipVerify {
		transport.TLSClientConfig = common.InsecureTLSConfig
	}
	return transport
}

func InitHttpClient() {
	httpClient = &http.Client{
		Transport:     newRelayTransport(true),
		Timeout:       streamClientTimeout(),
		CheckRedirect: checkRedirect,
	}
	nonStreamHTTPClient = &http.Client{
		Transport:     newRelayTransport(false),
		Timeout:       nonStreamClientTimeout(),
		CheckRedirect: checkRedirect,
	}
}

// GetHttpClient returns the streaming relay client (carries the response-header
// timeout). Existing non-relay callers keep their current behavior.
func GetHttpClient() *http.Client {
	return httpClient
}

// GetNonStreamHttpClient returns the non-stream relay client (no response-header
// timeout, bounded by an overall per-attempt timeout). Like GetHttpClient, this is
// nil until InitHttpClient runs at startup; intentionally NOT falling back to the
// streaming client, since that would silently reattach the response-header timeout
// to non-stream traffic and reintroduce the cascade bug.
func GetNonStreamHttpClient() *http.Client {
	return nonStreamHTTPClient
}

// GetHttpClientWithProxy returns the default streaming client or a proxy-enabled one.
func GetHttpClientWithProxy(proxyURL string) (*http.Client, error) {
	if proxyURL == "" {
		return GetHttpClient(), nil
	}
	return NewProxyHttpClient(proxyURL)
}

// ResetProxyClientCache 清空代理客户端缓存，确保下次使用时重新初始化
func ResetProxyClientCache() {
	proxyClientLock.Lock()
	defer proxyClientLock.Unlock()
	for _, client := range proxyClients {
		if transport, ok := client.Transport.(*http.Transport); ok && transport != nil {
			transport.CloseIdleConnections()
		}
	}
	proxyClients = make(map[string]*http.Client)
}

// NewProxyHttpClient 创建支持代理的 HTTP 客户端（流式：带响应头超时）。
func NewProxyHttpClient(proxyURL string) (*http.Client, error) {
	return newProxyHttpClient(proxyURL, true)
}

// NewNonStreamProxyHttpClient 创建支持代理的非流式 HTTP 客户端（无响应头超时，
// 由整体超时兜底），用于非流式中继请求，避免慢但正常的补全被头超时误切。
func NewNonStreamProxyHttpClient(proxyURL string) (*http.Client, error) {
	return newProxyHttpClient(proxyURL, false)
}

func newProxyHttpClient(proxyURL string, stream bool) (*http.Client, error) {
	if proxyURL == "" {
		if stream {
			if client := GetHttpClient(); client != nil {
				return client, nil
			}
			return http.DefaultClient, nil
		}
		if client := GetNonStreamHttpClient(); client != nil {
			return client, nil
		}
		return http.DefaultClient, nil
	}

	// Stream and non-stream proxy clients differ in their timeout configuration,
	// so they are cached under distinct keys.
	cacheKey := proxyURL
	if !stream {
		cacheKey = "nonstream|" + proxyURL
	}

	proxyClientLock.Lock()
	if client, ok := proxyClients[cacheKey]; ok {
		proxyClientLock.Unlock()
		return client, nil
	}
	proxyClientLock.Unlock()

	parsedURL, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}

	clientTimeout := streamClientTimeout()
	if !stream {
		clientTimeout = nonStreamClientTimeout()
	}

	switch parsedURL.Scheme {
	case "http", "https":
		transport := &http.Transport{
			MaxIdleConns:        common.RelayMaxIdleConns,
			MaxIdleConnsPerHost: common.RelayMaxIdleConnsPerHost,
			ForceAttemptHTTP2:   true,
			Proxy:               http.ProxyURL(parsedURL),
		}
		if stream {
			applyRelayTransportTimeouts(transport)
		}
		if common.TLSInsecureSkipVerify {
			transport.TLSClientConfig = common.InsecureTLSConfig
		}
		client := &http.Client{
			Transport:     transport,
			CheckRedirect: checkRedirect,
			Timeout:       clientTimeout,
		}
		proxyClientLock.Lock()
		proxyClients[cacheKey] = client
		proxyClientLock.Unlock()
		return client, nil

	case "socks5", "socks5h":
		// 获取认证信息
		var auth *proxy.Auth
		if parsedURL.User != nil {
			auth = &proxy.Auth{
				User:     parsedURL.User.Username(),
				Password: "",
			}
			if password, ok := parsedURL.User.Password(); ok {
				auth.Password = password
			}
		}

		// 创建 SOCKS5 代理拨号器
		// proxy.SOCKS5 使用 tcp 参数，所有 TCP 连接包括 DNS 查询都将通过代理进行。行为与 socks5h 相同
		dialer, err := proxy.SOCKS5("tcp", parsedURL.Host, auth, proxy.Direct)
		if err != nil {
			return nil, err
		}

		transport := &http.Transport{
			MaxIdleConns:        common.RelayMaxIdleConns,
			MaxIdleConnsPerHost: common.RelayMaxIdleConnsPerHost,
			ForceAttemptHTTP2:   true,
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
		}
		if stream {
			applyRelayTransportTimeouts(transport)
		}
		if common.TLSInsecureSkipVerify {
			transport.TLSClientConfig = common.InsecureTLSConfig
		}

		client := &http.Client{Transport: transport, CheckRedirect: checkRedirect, Timeout: clientTimeout}
		proxyClientLock.Lock()
		proxyClients[cacheKey] = client
		proxyClientLock.Unlock()
		return client, nil

	default:
		return nil, fmt.Errorf("unsupported proxy scheme: %s, must be http, https, socks5 or socks5h", parsedURL.Scheme)
	}
}
