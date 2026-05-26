package helper

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/constant"
	"github.com/ca0fgh/hermestoken/logger"
	relaycommon "github.com/ca0fgh/hermestoken/relay/common"
	"github.com/ca0fgh/hermestoken/setting/operation_setting"

	"github.com/bytedance/gopkg/util/gopool"

	"github.com/gin-gonic/gin"
)

const (
	InitialScannerBufferSize    = 64 << 10 // 64KB (64*1024)
	DefaultMaxScannerBufferSize = 64 << 20 // 64MB (64*1024*1024) default SSE buffer size
	DefaultPingInterval         = 10 * time.Second
	DefaultStreamingTimeout     = 300 * time.Second
)

func getScannerBufferSize() int {
	if constant.StreamScannerMaxBufferMB > 0 {
		return constant.StreamScannerMaxBufferMB << 20
	}
	return DefaultMaxScannerBufferSize
}

func getStreamingTimeout() time.Duration {
	streamingTimeout := time.Duration(constant.StreamingTimeout) * time.Second
	if streamingTimeout <= 0 {
		return DefaultStreamingTimeout
	}
	return streamingTimeout
}

func StreamScannerHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, dataHandler func(data string, sr *StreamResult)) {

	if resp == nil || dataHandler == nil {
		return
	}

	if info.StreamStatus == nil {
		info.StreamStatus = relaycommon.NewStreamStatus()
	}

	// 确保响应体总是被关闭
	defer func() {
		if resp.Body != nil {
			resp.Body.Close()
		}
	}()

	streamingTimeout := getStreamingTimeout()

	var (
		// stopChan：只关闭、不发送的广播停止信号（配合 stopOnce 幂等、无竞争），
		// 取代原「bool 缓冲通道 + SafeSendBool + close」写法，消除 close 与并发
		// send/recv 的数据竞争（既有 -race 问题）。
		stopChan = make(chan struct{})
		stopOnce sync.Once
		scanner    = bufio.NewScanner(resp.Body)
		ticker     = time.NewTicker(streamingTimeout)
		pingTicker *time.Ticker
		// firstChunkTimer：首字超时失败转移定时器（仅 info.FirstChunkTimeoutSeconds>0 时创建）。
		firstChunkTimer *time.Timer
		writeMutex      sync.Mutex // Mutex to protect concurrent writes
		wg         sync.WaitGroup // 用于等待所有 goroutine 退出
		// firstTokenSeen：首字是否已到。供「首字保活」判断使用，用原子量避免与
		// 扫描 goroutine 写 info.FirstResponseTime 产生数据竞争。
		firstTokenSeen atomic.Bool
	)

	// stop 幂等广播停止：关闭 stopChan 唤醒所有等待者，多次调用安全（避免 close 竞争）。
	stop := func() { stopOnce.Do(func() { close(stopChan) }) }

	generalSettings := operation_setting.GetGeneralSetting()
	pingInterval := time.Duration(generalSettings.PingIntervalSeconds) * time.Second
	if pingInterval <= 0 {
		pingInterval = DefaultPingInterval
	}
	// 常规 ping：贯穿整条流，受设置开关控制（保持原有语义）。
	pingEnabled := generalSettings.PingIntervalEnabled && !info.DisablePing
	// 首字保活：即使常规 ping 关闭，也要在「等待上游首个 token」期间持续向客户端
	// 发送 SSE 注释保活，防止客户端在慢上游首字到达前因空闲超时而断开——这是线上
	// gpt-5.4 流式 0-token / client_gone 的根因（上游首字 27~41s+，客户端 ~120s 先断）。
	// 仅在未显式 DisablePing 时启用；一旦收到首字，若常规 ping 未开启则自动停止，
	// 对现有流式行为保持最小侵入。SSE 注释不携带语义，亦不破坏 prompt-cache 渠道亲和。
	keepaliveEnabled := pingEnabled || !info.DisablePing

	if keepaliveEnabled {
		pingTicker = time.NewTicker(pingInterval)
	}

	// 首字超时失败转移：仅当控制器对「非亲和」请求设置了正阈值时启用。期间保活照常运行，
	// 使客户端在等待 failover 判定前不至于空闲断开；超时触发时尚未转发任何输出，可安全切换渠道。
	if info.FirstChunkTimeoutSeconds > 0 {
		firstChunkTimer = time.NewTimer(time.Duration(info.FirstChunkTimeoutSeconds) * time.Second)
	}

	logger.LogDebug(c, "relay timeout seconds: %d", common.RelayTimeout)
	logger.LogDebug(c, "relay max idle conns: %d", common.RelayMaxIdleConns)
	logger.LogDebug(c, "relay max idle conns per host: %d", common.RelayMaxIdleConnsPerHost)
	logger.LogDebug(c, "streaming timeout seconds: %d", int64(streamingTimeout.Seconds()))
	logger.LogDebug(c, "ping interval seconds: %d", int64(pingInterval.Seconds()))

	// 改进资源清理，确保所有 goroutine 正确退出
	defer func() {
		// 通知所有 goroutine 停止
		stop()

		ticker.Stop()
		if pingTicker != nil {
			pingTicker.Stop()
		}
		if firstChunkTimer != nil {
			firstChunkTimer.Stop()
		}

		// 等待所有 goroutine 退出，最多等待5秒
		done := make(chan struct{})
		gopool.Go(func() {
			wg.Wait()
			close(done)
		})

		select {
		case <-done:
			// 所有 goroutine 已退出，此时读取 ReceivedResponseCount 无竞争
			// （扫描 goroutine 在 SetFirstResponseTime 处递增该计数）。
			if info.StreamStatus.IsNormalEnd() && !info.StreamStatus.HasErrors() {
				logger.LogInfo(c, fmt.Sprintf("stream ended: %s", info.StreamStatus.Summary()))
			} else {
				logger.LogError(c, fmt.Sprintf("stream ended: %s, received=%d", info.StreamStatus.Summary(), info.ReceivedResponseCount))
			}
		case <-time.After(5 * time.Second):
			// goroutine 卡住未退出：避免与仍在运行的 goroutine 竞争读取计数，
			// 仅记录异常摘要（StreamStatus.Summary 已加锁，可安全调用）。
			logger.LogError(c, fmt.Sprintf("timeout waiting for goroutines to exit: %s", info.StreamStatus.Summary()))
		}
	}()

	scanner.Buffer(make([]byte, InitialScannerBufferSize), getScannerBufferSize())
	scanner.Split(bufio.ScanLines)
	SetEventStreamHeaders(c)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = context.WithValue(ctx, "stop_chan", stop)

	// Handle ping data sending with improved error handling
	if keepaliveEnabled && pingTicker != nil {
		wg.Add(1)
		gopool.Go(func() {
			defer func() {
				wg.Done()
				if r := recover(); r != nil {
					logger.LogError(c, fmt.Sprintf("ping goroutine panic: %v", r))
					info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonPanic, fmt.Errorf("ping panic: %v", r))
					stop()
				}
				logger.LogDebug(c, "ping goroutine exited")
			}()

			// 添加超时保护，防止 goroutine 无限运行
			maxPingDuration := 30 * time.Minute // 最大 ping 持续时间
			pingTimeout := time.NewTimer(maxPingDuration)
			defer pingTimeout.Stop()

			for {
				select {
				case <-pingTicker.C:
					// 首字保活模式：常规 ping 关闭时，仅在首字到达前保活；
					// 一旦收到上游首字就停止，避免改变常规流式行为。
					if !pingEnabled && firstTokenSeen.Load() {
						logger.LogDebug(c, "first-token keepalive done: first response received")
						return
					}
					// 使用超时机制防止写操作阻塞
					done := make(chan error, 1)
					gopool.Go(func() {
						writeMutex.Lock()
						defer writeMutex.Unlock()
						done <- PingData(c)
					})

					select {
					case err := <-done:
						if err != nil {
							logger.LogError(c, "ping data error: "+err.Error())
							info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonPingFail, err)
							return
						}
						logger.LogDebug(c, "ping data sent")
					case <-time.After(10 * time.Second):
						logger.LogError(c, "ping data send timeout")
						info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonPingFail, fmt.Errorf("ping send timeout"))
						return
					case <-ctx.Done():
						return
					case <-stopChan:
						return
					}
				case <-ctx.Done():
					return
				case <-stopChan:
					return
				case <-c.Request.Context().Done():
					// 监听客户端断开连接
					return
				case <-pingTimeout.C:
					logger.LogError(c, "ping goroutine max duration reached")
					return
				}
			}
		})
	}

	dataChan := make(chan string, 10)

	wg.Add(1)
	gopool.Go(func() {
		defer func() {
			wg.Done()
			if r := recover(); r != nil {
				logger.LogError(c, fmt.Sprintf("data handler goroutine panic: %v", r))
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonPanic, fmt.Errorf("handler panic: %v", r))
			}
			stop()
		}()
		sr := newStreamResult(info.StreamStatus)
		for data := range dataChan {
			sr.reset()
			writeMutex.Lock()
			dataHandler(data, sr)
			writeMutex.Unlock()
			if sr.IsStopped() {
				return
			}
		}
	})

	// Scanner goroutine with improved error handling
	wg.Add(1)
	common.RelayCtxGo(ctx, func() {
		defer func() {
			close(dataChan)
			wg.Done()
			if r := recover(); r != nil {
				logger.LogError(c, fmt.Sprintf("scanner goroutine panic: %v", r))
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonPanic, fmt.Errorf("scanner panic: %v", r))
			}
			stop()
			logger.LogDebug(c, "scanner goroutine exited")
		}()

		for scanner.Scan() {
			// 检查是否需要停止
			select {
			case <-stopChan:
				return
			case <-ctx.Done():
				return
			case <-c.Request.Context().Done():
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, c.Request.Context().Err())
				return
			default:
			}

			ticker.Reset(streamingTimeout)
			data := scanner.Text()
			logger.LogDebug(c, "stream scanner data: %s", data)

			if len(data) < 6 {
				continue
			}
			if data[:5] != "data:" && data[:6] != "[DONE]" {
				continue
			}
			data = data[5:]
			data = strings.TrimSpace(data)
			if data == "" {
				continue
			}
			if !strings.HasPrefix(data, "[DONE]") {
				info.SetFirstResponseTime()
				firstTokenSeen.Store(true)
				info.ReceivedResponseCount++

				select {
				case dataChan <- data:
				case <-ctx.Done():
					return
				case <-stopChan:
					return
				}
			} else {
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonDone, nil)
				logger.LogDebug(c, "received [DONE], stopping scanner")
				return
			}
		}

		if err := scanner.Err(); err != nil {
			if err != io.EOF {
				logger.LogError(c, "scanner error: "+err.Error())
				info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonScannerErr, err)
			}
		}
		info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonEOF, nil)
	})

	// 主循环等待完成或超时。
	// 首字超时分支仅在 info.FirstChunkTimeoutSeconds>0（控制器对非亲和请求设置）且上游首个
	// token 尚未到达时生效；若首字已到，则该定时器已陈旧，忽略并继续等待数据流自然结束。
waitLoop:
	for {
		select {
		case <-ticker.C:
			info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonTimeout, nil)
			break waitLoop
		case <-stopChan:
			// EndReason already set by the goroutine that triggered stopChan
			break waitLoop
		case <-c.Request.Context().Done():
			info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, c.Request.Context().Err())
			break waitLoop
		case <-firstChunkTimerChan(firstChunkTimer):
			if firstTokenSeen.Load() {
				continue
			}
			info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonFirstChunkTimeout,
				fmt.Errorf("no first chunk within %ds", info.FirstChunkTimeoutSeconds))
			break waitLoop
		}
	}

	// "stream ended" 摘要日志已移入上方的 cleanup defer，在 wg.Wait() 之后打印——
	// 确保读取 info.ReceivedResponseCount 时扫描 goroutine 已退出，消除数据竞争。
}

// firstChunkTimerChan 返回首字超时定时器的通道；定时器为 nil（未启用）时返回 nil 通道，
// 在 select 中永远阻塞，相当于该分支不存在——使首字超时成为零行为变更的可选特性。
func firstChunkTimerChan(t *time.Timer) <-chan time.Time {
	if t == nil {
		return nil
	}
	return t.C
}
