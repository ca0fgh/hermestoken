package helper

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ca0fgh/hermestoken/constant"
	relaycommon "github.com/ca0fgh/hermestoken/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// 验证「首字超时失败转移」机制（#6）在 StreamScannerHandler 层的行为：
//   - FirstChunkTimeoutSeconds>0 且上游迟迟不吐首字 → 标记 FirstChunkTimeout、未转发任何输出；
//   - FirstChunkTimeoutSeconds==0（默认）→ 机制完全禁用，零行为变更；
//   - 首字早于阈值到达 → 不触发，正常结束。
//
// 控制器据 EndReason==FirstChunkTimeout && ReceivedResponseCount==0 合成可重试 504 切换渠道；
// 亲和请求由控制器置阈值为 0，永不进入此路径（保护 codex prompt 缓存）。

func runFirstChunkScanner(t *testing.T, firstChunkTimeoutSecs int, firstTokenDelay time.Duration) (*relaycommon.RelayInfo, int64) {
	t.Helper()

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		time.Sleep(firstTokenDelay) // 上游迟迟不吐首字
		fmt.Fprint(pw, "data: {\"id\":1}\n")
		fmt.Fprint(pw, "data: [DONE]\n")
	}()
	resp := &http.Response{Body: pr}

	info := &relaycommon.RelayInfo{
		IsStream:                 true,
		DisablePing:              true, // 关掉保活，聚焦首字超时判定
		FirstChunkTimeoutSeconds: firstChunkTimeoutSecs,
		ChannelMeta:              &relaycommon.ChannelMeta{},
	}

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
	return info, processed.Load()
}

// 阈值=1s、上游 2s 才吐首字 → 触发首字超时，未转发任何输出。
func TestStreamScannerHandler_FirstChunkTimeout_Fires(t *testing.T) {
	info, processed := runFirstChunkScanner(t, 1, 2*time.Second)

	assert.Equal(t, relaycommon.StreamEndReasonFirstChunkTimeout, info.StreamStatus.EndReason(),
		"等待首字超过阈值应标记 FirstChunkTimeout")
	assert.Equal(t, 0, info.ReceivedResponseCount, "首字超时时不应转发任何输出")
	assert.Equal(t, int64(0), processed, "首字超时时 dataHandler 不应被调用")
	assert.False(t, info.StreamStatus.IsNormalEnd(), "首字超时是非正常结束")
}

// 阈值=0（默认禁用）、上游 1s 吐首字 → 机制禁用，正常完成，零行为变更。
func TestStreamScannerHandler_FirstChunkTimeout_DisabledByDefault(t *testing.T) {
	info, processed := runFirstChunkScanner(t, 0, 1*time.Second)

	assert.NotEqual(t, relaycommon.StreamEndReasonFirstChunkTimeout, info.StreamStatus.EndReason(),
		"默认禁用时不应出现 FirstChunkTimeout")
	assert.Equal(t, int64(1), processed, "默认禁用时应正常处理首字")
}

// 阈值=5s、上游 1s 即吐首字 → 首字早于阈值，不触发，正常完成。
func TestStreamScannerHandler_FirstChunkTimeout_FirstTokenBeforeThreshold(t *testing.T) {
	info, processed := runFirstChunkScanner(t, 5, 1*time.Second)

	assert.NotEqual(t, relaycommon.StreamEndReasonFirstChunkTimeout, info.StreamStatus.EndReason(),
		"首字早于阈值到达时不应触发首字超时")
	assert.Equal(t, int64(1), processed, "首字早到应正常处理")
}
