package common

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type StreamEndReason string

const (
	StreamEndReasonNone        StreamEndReason = ""
	StreamEndReasonDone        StreamEndReason = "done"
	StreamEndReasonTimeout     StreamEndReason = "timeout"
	StreamEndReasonClientGone  StreamEndReason = "client_gone"
	StreamEndReasonScannerErr  StreamEndReason = "scanner_error"
	StreamEndReasonHandlerStop StreamEndReason = "handler_stop"
	StreamEndReasonEOF         StreamEndReason = "eof"
	StreamEndReasonPanic       StreamEndReason = "panic"
	StreamEndReasonPingFail    StreamEndReason = "ping_fail"
	// StreamEndReasonFirstChunkTimeout：等待上游首个 data: 超过阈值且尚未转发任何输出。
	// 非正常结束（IsNormalEnd 为 false）；控制器据此对「非亲和」请求合成可重试错误以切换渠道。
	StreamEndReasonFirstChunkTimeout StreamEndReason = "first_chunk_timeout"
)

const maxStreamErrorEntries = 20

type StreamErrorEntry struct {
	Message   string
	Timestamp time.Time
}

// StreamStatus 在 StreamScannerHandler 的多个 goroutine（scanner / ping /
// dataHandler / 主循环）之间共享。所有字段一律私有，并由单个 mu 保护读写——
// 既有实现把 EndReason/EndError 暴露为公有字段，写入虽用 sync.Once 串行化，但
// 主循环对 IsNormalEnd()/Summary() 的读取与这些写入无 happens-before 关系，
// 在 -race 下表现为数据竞争（线上 8 个计费渠道适配器共用此原语）。改为全程
// 持锁访问后，"首个 EndReason 胜出" 由 ended 标志在锁内保证，读写完全同步。
type StreamStatus struct {
	mu         sync.Mutex
	ended      bool
	endReason  StreamEndReason
	endError   error
	errors     []StreamErrorEntry
	errorCount int
}

func NewStreamStatus() *StreamStatus {
	return &StreamStatus{}
}

// SetEndReason 记录流的结束原因，首个写入者胜出，后续调用被忽略（锁内幂等）。
func (s *StreamStatus) SetEndReason(reason StreamEndReason, err error) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ended {
		return
	}
	s.ended = true
	s.endReason = reason
	s.endError = err
}

// EndReason 返回已固化的结束原因（持锁读，可在其他 goroutine 仍可能写入时安全调用）。
func (s *StreamStatus) EndReason() StreamEndReason {
	if s == nil {
		return StreamEndReasonNone
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.endReason
}

// EndError 返回结束时携带的错误（持锁读）。
func (s *StreamStatus) EndError() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.endError
}

func (s *StreamStatus) RecordError(msg string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errorCount++
	if len(s.errors) < maxStreamErrorEntries {
		s.errors = append(s.errors, StreamErrorEntry{
			Message:   msg,
			Timestamp: time.Now(),
		})
	}
}

func (s *StreamStatus) HasErrors() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.errorCount > 0
}

// TotalErrorCount 返回累计软错误次数（可超过 maxStreamErrorEntries）。
func (s *StreamStatus) TotalErrorCount() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.errorCount
}

// ErrorMessages 返回已存储的软错误消息副本（最多 maxStreamErrorEntries 条）。
func (s *StreamStatus) ErrorMessages() []string {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	messages := make([]string, 0, len(s.errors))
	for _, e := range s.errors {
		messages = append(messages, e.Message)
	}
	return messages
}

func (s *StreamStatus) IsNormalEnd() bool {
	if s == nil {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.endReason == StreamEndReasonDone ||
		s.endReason == StreamEndReasonEOF ||
		s.endReason == StreamEndReasonHandlerStop
}

func (s *StreamStatus) Summary() string {
	if s == nil {
		return "StreamStatus<nil>"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	b := &strings.Builder{}
	fmt.Fprintf(b, "reason=%s", s.endReason)
	if s.endError != nil {
		fmt.Fprintf(b, " end_error=%q", s.endError.Error())
	}
	if s.errorCount > 0 {
		fmt.Fprintf(b, " soft_errors=%d", s.errorCount)
	}
	return b.String()
}
