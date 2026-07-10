package helper

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ca0fgh/hermestoken/constant"
	relaycommon "github.com/ca0fgh/hermestoken/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 本地复现线上现象：gpt-5.4 流式请求记录 0 input / 0 output、耗时 ~130s。
//
// 线上链路（容器 hermestoken-prod 日志）：
//
//	[ERR] stream ended: reason=client_gone end_error="context canceled", received=0
//	[ERR] timeout waiting for goroutines to exit            ← 5s 后
//	[INFO] record consume log: prompt_tokens:0, completion_tokens:0,
//	       use_time_seconds:129, frt:-1000,
//	       content:"上游没有返回计费信息，无法扣费（可能是上游超时）"
//
// 成因：codex CLI（reasoning_effort=xhigh、超大缓存 prompt）打到很慢的
// gpt-限时渠道（channel 51，成功请求首字都要 27~41s）。上游在首字到达前
// 迟迟不吐数据，客户端先到达自身超时而断开 → StreamScannerHandler 走
// StreamEndReasonClientGone 分支，ReceivedResponseCount=0（没有任何 usage
// 事件）→ 计费 0 token。use_time≈130s 是客户端超时，不是服务端超时。
//
// 其中的 5s 空档已被上游 #5710 修复：cleanup 现在会关闭 resp.Body，扫描
// goroutine 的 Read 立即返回，wg.Wait 不再空等到 5s 上限。下方第二个用例
// 由「复现该空档」改为「守住修复」。

// newClientGoneScannerCtx 构造一个 request context 可被取消（模拟客户端断开）
// 的 gin.Context，以及一个阻塞的上游响应体（模拟慢/挂起的上游）。
func newClientGoneScannerCtx(t *testing.T) (*gin.Context, context.CancelFunc, *io.PipeWriter, *http.Response, *relaycommon.RelayInfo) {
	t.Helper()

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30 // 远大于用例时长，避免服务端 streaming 超时先触发
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	pr, pw := io.Pipe() // 上游：测试期间不写入 → Read 一直阻塞，模拟挂起的上游
	t.Cleanup(func() { _ = pw.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)

	resp := &http.Response{Body: pr}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	return c, cancel, pw, resp, info
}

// 现象一：客户端在首字到达前断开 → end_reason=client_gone、received=0 → 0 token。
func TestStreamScannerHandler_ClientGoneRecordsZeroUsage(t *testing.T) {
	c, cancel, pw, resp, info := newClientGoneScannerCtx(t)

	done := make(chan struct{})
	go func() {
		StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {})
		close(done)
	}()

	time.Sleep(200 * time.Millisecond) // 等扫描 goroutine 阻塞在上游 Read 上
	cancel()                           // 客户端断开（对应线上 context canceled）

	// 断开后解开上游，让扫描 goroutine 退出，避免被 5s drain 超时拖慢本用例
	time.Sleep(100 * time.Millisecond)
	_ = pw.Close()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("handler did not return after client disconnect")
	}

	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonClientGone, info.StreamStatus.EndReason(),
		"客户端断开应判定为 client_gone")
	assert.False(t, info.StreamStatus.IsNormalEnd(),
		"client_gone 非正常结束 → status=error")
	assert.Equal(t, 0, info.ReceivedResponseCount,
		"首字前断开，未收到任何上游事件 → 0 token，复现线上 0 输入 0 输出")
}

// 现象二（已修复，此用例守住修复）：客户端断开但上游仍阻塞在 Read（慢上游）时，
// cleanup 关闭 resp.Body 会立即解开扫描 goroutine 的 Read，handler 迅速返回，
// 不再空等到 5s drain 上限（线上那条 "timeout waiting for goroutines to exit"）。
func TestStreamScannerHandler_ClientGoneHungUpstreamReturnsWithoutDrainTimeout(t *testing.T) {
	c, cancel, _, resp, info := newClientGoneScannerCtx(t)
	// 注意：本用例不主动关闭 pw（由 t.Cleanup 兜底）——上游始终不吐数据也不关闭，
	// 唯一能解开扫描 goroutine 的就是 cleanup 里的 resp.Body.Close()。

	done := make(chan struct{})
	start := time.Now()
	go func() {
		StreamScannerHandler(c, resp, info, func(data string, sr *StreamResult) {})
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)
	cancel() // 客户端断开；上游 Read 仍阻塞

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("handler hung > 10s: cleanup 未关闭 resp.Body?")
	}
	elapsed := time.Since(start)

	require.NotNil(t, info.StreamStatus)
	assert.Equal(t, relaycommon.StreamEndReasonClientGone, info.StreamStatus.EndReason())
	assert.Less(t, elapsed, 2*time.Second,
		"客户端断开后应立即返回，不得再命中 5s goroutine drain 超时")
}
