package helper

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ca0fgh/hermestoken/constant"
	relaycommon "github.com/ca0fgh/hermestoken/relay/common"
	"github.com/ca0fgh/hermestoken/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// 验证「首字保活」修复：即使常规 ping 关闭（PingIntervalEnabled=false），
// 只要未显式 DisablePing，等待上游首个 token 期间仍应持续向客户端发 SSE 注释保活，
// 防止客户端在慢上游首字到达前因空闲超时而断开（线上 0-token / client_gone 根因）。
//
// 该行为不依赖 prod 的 ping 设置，因此修复对线上生效与否不取决于运维是否开了 ping。

func newSlowFirstTokenUpstream(firstTokenDelay time.Duration) *http.Response {
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		time.Sleep(firstTokenDelay) // 上游迟迟不吐首字（期间应有保活）
		fmt.Fprint(pw, "data: {\"id\":1}\n")
		fmt.Fprint(pw, "data: [DONE]\n")
	}()
	return &http.Response{Body: pr}
}

func runScannerWithPingSetting(t *testing.T, info *relaycommon.RelayInfo, pingEnabled bool) string {
	t.Helper()

	setting := operation_setting.GetGeneralSetting()
	oldEnabled, oldSeconds := setting.PingIntervalEnabled, setting.PingIntervalSeconds
	setting.PingIntervalEnabled = pingEnabled
	setting.PingIntervalSeconds = 1
	t.Cleanup(func() {
		setting.PingIntervalEnabled = oldEnabled
		setting.PingIntervalSeconds = oldSeconds
	})

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	resp := newSlowFirstTokenUpstream(2500 * time.Millisecond)

	var processed atomic.Int64
	done := make(chan struct{})
	go func() {
		StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) { processed.Add(1) })
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		t.Fatal("StreamScannerHandler 未在预期时间内返回")
	}
	assert.Equal(t, int64(1), processed.Load(), "首字最终应被正常处理")
	return recorder.Body.String()
}

// 常规 ping 关闭、未 DisablePing：等待首字期间仍应保活（>=1 个 PING）。
func TestStreamScannerHandler_FirstTokenKeepaliveWhenPingDisabled(t *testing.T) {
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	body := runScannerWithPingSetting(t, info, false /* pingEnabled */)

	pingCount := strings.Count(body, ": PING")
	assert.GreaterOrEqual(t, pingCount, 1,
		"常规 ping 关闭时，等待上游首字期间仍应保活以避免客户端断开；实际 PING 数=%d", pingCount)
}

// 显式 DisablePing：任何情况下都不应保活（尊重显式关闭）。
func TestStreamScannerHandler_NoKeepaliveWhenDisablePing(t *testing.T) {
	info := &relaycommon.RelayInfo{DisablePing: true, ChannelMeta: &relaycommon.ChannelMeta{}}
	body := runScannerWithPingSetting(t, info, false /* pingEnabled */)

	pingCount := strings.Count(body, ": PING")
	assert.Equal(t, 0, pingCount, "显式 DisablePing 时不应发送任何保活")
}
