# 交接说明：彻底解决流式 0-token 超时（ping 保活 + 渠道 failover）

> 实现交接稿（**完整方案**）。背景见会话记录 / 项目 memory `prod-deployment-hermestoken`。
> 新会话里先 `Read` 本文件，再按「实现要点」动手。目标：从干净上下文低成本、高质量落地。

## 一、要解决的线上问题（已坐实）

codex CLI 流量（`/v1/responses`、`reasoning_effort=xhigh`、~42 万缓存 prompt）打到很慢的
`gpt-限时渠道`（channel 51，成功请求首字也要 27~41s）。上游首字到达前迟迟无数据，
**客户端先到达自身 ~120s 超时而断开** → `StreamScannerHandler` 走 `client_gone`、
`received=0`、无 usage 事件 → 记 0 token、退回预扣额度、`content="上游没有返回计费信息…"`。
现象：消费日志出现「0 输入 / 0 输出 / 128~143s」的失败记录（计费本身是对的，是请求失败）。

复现用例（已存在，逐字复现线上日志）：
`relay/helper/stream_scanner_client_gone_repro_test.go`
（`ClientGoneRecordsZeroUsage`、`ClientGoneHungUpstreamHitsDrainTimeout`）。

## 二、关键认知（务必先理解，否则方案会做歪）

锁死 failover 的**不是「发了 200 头」**，而是「**已经把第一段真实 `data:` 内容转发给客户端**」。

- `ping`（SSE 注释行 `: PING\n\n`，见 `relay/helper` 的 `PingData`）**不带语义内容**。
- 因此可以：**立刻**给客户端发 200 + event-stream 头并开始 ping 保活；**背后**照样切换上游渠道——
  只要还没转发过真实 `data:`，客户端只是先收到几个 ping，**完全无感**。
- 结论：**ping 保活与渠道 failover 可以同时使用**（此前认为二者互斥是错的）。

## 三、完全彻底方案

```
1. 一进流式分支：向客户端提交 200 + event-stream 头，启动 ping 保活。
   —— ping ticker + 客户端 writer 必须存活于【整个重试循环之上】，贯穿所有 attempt。
   —— 客户端因此永不空闲超时，client_gone 根除。

2. for attempt := 0; attempt <= RetryTimes; attempt++ {
       channel := 选下一个候选渠道（沿用现有渠道选择/排除已试列表）
       resp, err := 向 channel 发上游请求
       if err != nil { 软性标记 channel; continue }            // 连不上，直接切

       firstChunk, ferr := waitFirstSSEDataChunk(resp.Body, T_firstChunk)
       // 等待期间 ping 持续保活；只要还【没向客户端转发过真实 data】，就可安全切换
       if ferr != nil {                                        // 首字超时 / 上游中途断开 / 上游错误
           软性标记 channel（带冷却，勿 hard-disable）
           resp.Body.Close()
           continue                                            // 透明 failover 到下一渠道
       }

       // 拿到首字 → 此刻才算 committed
       body := io.MultiReader(bytes.NewReader(firstChunk), resp.Body)
       resp.Body = io.NopCloser(body)
       StreamScannerHandler(c, resp, info, dataHandler)        // 复用现有扫描/转发
       return                                                  // 之后中途卡住只结束流，不再切
   }

3. 所有渠道耗尽 → 向客户端发一个明确的 SSE error 事件并结束。
   —— 不再是静默 0-token；客户端全程被 ping 保活，能收到这个错误。
```

效果（三个问题一次根除）：
1. 客户端被 ping 保活 → 不再首字前断开 → **不再有静默 0-token**；
2. 真故障/慢渠道 → **透明 failover**；
3. 全部失败 → **返回明确错误**，而非 0-token 假「消费」。

## 四、实现要点（架构）

当前 `relay/helper/stream_scanner.go::StreamScannerHandler`（L46）把
**「客户端提交+ping」和「单次上游扫描」耦合在一起**，且 `SetEventStreamHeaders` 在
扫描前 L119 提交。需要拆成两层：

- **外层（客户端侧，跨 attempt 存活）**：提交 200 头、跑 ping ticker、维护 writeMutex、
  在所有 attempt 结束后做收尾（[DONE] 或 error 事件）。
- **内层（单次 attempt）**：连上游 → `waitFirstSSEDataChunk` 首字闸门 →（成功）把
  首块 + 剩余 body 交给扫描转发逻辑。失败则返回「可重试」信号给外层循环。

落地两种做法（任选，MultiReader 改动更小）：
- (A) 在 handler（`relay/responses_handler.go` 等，已有 `RetryTimes` 重试循环）里：先做
  ping+提交头，再在 attempt 循环里做首字闸门；成功后用 MultiReader 还原 body 调
  `StreamScannerHandler`，并让其**跳过重复提交头/重复 ping**（加 `alreadyCommitted`/`disablePing` 入参）。
- (B) 给 `StreamScannerHandler` 增加 `deferDataUntilFirstChunk` 模式，把首字闸门内置。

推荐 (A)：对 `StreamScannerHandler` 侵入最小，重试编排集中在 handler。

### 新增 helper：`waitFirstSSEDataChunk(body io.Reader, timeout) (firstChunk []byte, err error)`
（建议放 `relay/helper/`）
- goroutine 内读到第一行 `data: …`（跳过注释/空行/非 data 行，逻辑同
  `stream_scanner.go:243-254`；`[DONE]` 视为「无内容」算失败/EOF）；
- `select { case 首字 → 返回原始字节; case <-time.After(timeout) → 超时错误; }`；
- **务必**用 `io.MultiReader(bytes.NewReader(firstChunk), body)` 把已读字节拼回，避免丢首块；
- 读 goroutine 在超时返回后仍可能阻塞在上游 Read —— 关 `resp.Body` 让其解除（参考现有
  5s drain 逻辑 `stream_scanner.go:101-115`）。

## 五、文件锚点（本会话已确认）

- `relay/helper/stream_scanner.go`：`StreamScannerHandler` L46；`SetEventStreamHeaders` 提交 L119；
  ping goroutine L127-186（`PingData`、`PingIntervalEnabled/Seconds`、`DefaultPingInterval=10s`）；
  client_gone 分支 L234 / L288；5s drain L101-115；data-interval `getStreamingTimeout()`（默认 300s，
  逐 chunk 重置，**与首字超时是两个独立概念**）。
- `relay/helper/common.go:41`：`SetEventStreamHeaders`（内部 `flush()`）。
- `relay/common/stream_status.go`：`StreamEndReason*`（含 `ClientGone`/`Timeout`），`IsNormalEnd()`。
- `relay/common/relay_info.go`：`FirstResponseTime`、`SetFirstResponseTime()` L658、`ReceivedResponseCount`。
- `relay/responses_handler.go`（及 `compatible_handler.go` 等）：已有 retry/`RetryTimes`，failover 编排接这里。
- `service/text_quota.go:366`：「上游没有返回计费信息」兜底——保留为「全部渠道失败」时的计费。

## 六、配置（新增）

- `T_firstChunk`：首字超时秒数。建议默认 **60~90s**（要 > 合法最坏首字 ~41s，避免误切健康渠道）。
  最好可**按分组/模型覆盖**（普通模型设短以快切真故障；codex/xhigh 重负载设长）。MVP 可先全局值。
- 复用现有 `RetryTimes`（建议 2~3）。
- ping：确认 `PingIntervalEnabled` 在本路径开启、间隔合理（如 10s）。

## 七、边界与残留

- **首字到达后**上游再卡住：已转发真实 data，**不能**再 failover —— 维持现有 timeout/client_gone 结束逻辑。
- 若 codex 是**硬性总超时**（不看字节、到点就掐）：ping 也无法突破该上限，但这是客户端侧硬限制，
  服务端已做到极致。可观察上线后 client_gone 是否显著下降来验证。
- `gpt-限时渠道` 若所有渠道同样慢：failover 仍可能逐个超时，但因 ping 保活 + 最终明确报错，
  不再是静默 0-token；`RetryTimes` 别过大、渠道用软性冷却标记以免误伤。

## 八、测试

复用 `relay/helper/stream_scanner_client_gone_repro_test.go`，并新增覆盖：
- 首字超时 → 切到第二个渠道（第二渠道给首字 → 成功）；客户端收到完整流，**200 头只提交一次**，
  期间有 ping，无重复 ping/头。
- 所有渠道首字都超时 → 客户端收到 SSE error 事件结束，`ReceivedResponseCount==0`，渠道被软标记。
- 首字已到 → 之后上游卡住 → **不**切渠道，按现有逻辑结束（验证边界）。
- `waitFirstSSEDataChunk` 单测：正常首字 / 超时 / 上游提前 EOF / 跳过注释行；MultiReader 不丢首块。
- 跑：`go test ./relay/helper/ ./relay/ -run '...' -count=1 -race`。

## 九、顺带（独立小修）

线上 `perf_metrics` 表缺失（`SQLSTATE 42P01`，日志刷屏 `failed to flush perf metric bucket`）——
补建表迁移 / 在 AutoMigrate 注册该表。与 failover 改动无关，可一起做。

---

# 十、实现进展（IMPLEMENTATION STATUS，2026-05-26）

## ✅ 已实现并验证（本会话落地）

**首字保活（first-token keepalive）** — `relay/helper/stream_scanner.go`：
- 即使常规 ping 关闭（`PingIntervalEnabled=false`），只要未显式 `DisablePing`，就在「等待上游
  首个 token」期间持续向客户端发 SSE 注释保活；**收到首字后**（常规 ping 关时）自动停止；
  `DisablePing=true` 时完全不保活。
- **关键价值：不依赖 prod 的 ping 设置** —— 修复是否生效不取决于运维是否开了 ping。
- 这是对线上 0-token / `client_gone` 的**直接、安全、对的核心修复**（针对亲和 codex 流量：
  只保活、不 failover，不破 prompt 缓存）。
- 同时修复了实现过程中引入的一个数据竞争：keepalive 判断改用局部 `atomic.Bool`
  （`firstTokenSeen`），不再跨 goroutine 读 `info.FirstResponseTime`。
- 验证：`go build` ✅、`go vet` ✅；新增测试
  `relay/helper/stream_scanner_first_token_keepalive_test.go`
  （`FirstTokenKeepaliveWhenPingDisabled`、`NoKeepaliveWhenDisablePing`）+ 既有 ping/复现用例
  **功能全部 PASS**（不带 `-race`）。

## 🔧 既有并发竞争（pre-existing，非本次引入）——全部已修并通过 `-race` ✅

`StreamScannerHandler` 在 `-race` 下暴露**多处既有数据竞争**（项目 CI 未跑 `-race`，长期未被发现）。
**本次已全部修复**，`go test -race ./relay/common/ ./relay/helper/` 现已**全绿**：

**（1）stopChan close/send 竞争 —— 已修 ✅**
- 原 `make(chan bool,3)` + `common.SafeSendBool`（裸 `ch<-`）+ cleanup defer `close(stopChan)`，
  `close` 与并发 send/recv 竞争。
- 修法：改为只关闭、不发送的广播信号 `make(chan struct{})` + `sync.Once` 守护的 `stop()`；
  `common/gopool.go` 的 panic handler 改为从 ctx 取出幂等 `func()` 调用。

**（2）StreamStatus 字段多 goroutine 写竞争 —— 已修 ✅**
- `StreamStatus.EndReason`/`EndError` 原为公有字段，写虽用 `sync.Once` 串行化，但主循环对
  `IsNormalEnd()`/`Summary()` 的读取与这些写入无 happens-before 关系 → `-race` 下竞争。
- 修法：`relay/common/stream_status.go` 全部字段改私有，**统一由单个 `mu` 保护读写**；
  "首个 EndReason 胜出" 由锁内 `ended` 标志保证；新增 `EndReason()`/`EndError()`/`ErrorMessages()`
  访问器；外部唯一读取点 `service/log_info_generate.go` 同步改为走访问器。

**（3）`info.ReceivedResponseCount` 读写竞争（本会话新发现的真实生产竞争）—— 已修 ✅**
- 收尾的 "stream ended" 日志在 `select` 返回后、`wg.Wait()` 之前读取 `info.ReceivedResponseCount`，
  而扫描 goroutine 仍可能在递增它（timeout / client_gone 路径尤甚）。
- 修法：把该日志移入 cleanup defer 中、`wg.Wait()` 完成之后再打印，确保读取时写入方已退出；
  卡死兜底分支只调用已加锁的 `Summary()`、不读裸计数。

**（4）测试隔离：t.Parallel + 全局 config 竞争 —— 已修 ✅**
- `setupStreamTest`/`runScannerWithPingSetting` 会改写全局 `constant.StreamingTimeout`、
  `operation_setting`，而生产代码 `getStreamingTimeout()` 在 handler 运行期读取同一全局；并行用例
  与彼此的 setup 竞争。
- 修法：移除 `relay/helper/stream_scanner_test.go` 中全部 `t.Parallel()`（该包用例都会触达
  全局），使包内串行执行；`relay/common` 的 StreamStatus 单元测试因已全程加锁，保留并行且 `-race` 通过。

## ✅ 失败转移（failover）——已实现，默认关闭（opt-in），并通过 `-race`

满足你的要求：**渠道首字超时就切换别的渠道重试，但只重试「没出过输出」的**，且**不破坏渠道亲和**。
设计为 **default-off 特性开关**：开关关闭时恒不启用，生产行为逐字节不变；开启后才生效。

落地点（6 处，已全部完成并 `-race` 验证）：
1. **配置**（`setting/operation_setting/general_setting.go`）：新增 `FirstChunkTimeoutEnabled`（默认
   `false`）、`FirstChunkTimeoutSeconds`（默认 60）。
2. **结束原因**（`relay/common/stream_status.go`）：新增 `StreamEndReasonFirstChunkTimeout`
   （`IsNormalEnd()` 为 false，按失败处理）。
3. **RelayInfo 字段**（`relay/common/relay_info.go`）：`FirstChunkTimeoutSeconds int`，0=禁用。
4. **扫描器**（`relay/helper/stream_scanner.go`）：`info.FirstChunkTimeoutSeconds>0` 时起一个首字定时器；
   主等待循环新增分支——首字未到即触发 → 标记 FirstChunkTimeout 结束（首字已到则忽略陈旧定时器继续等待）。
   期间保活照常运行，客户端在 failover 判定前不会空闲断开。`firstChunkTimerChan(nil)` 返回 nil 通道，
   使该分支在禁用时等价于不存在 → 零行为变更。
5. **控制器**（`controller/relay.go`）：
   - 每次尝试前**重置** `relayInfo.StreamStatus=nil`、`ReceivedResponseCount=0`（relayInfo 跨重试复用，
     StreamScannerHandler 仅在 StreamStatus==nil 时新建，不重置会让上次的 ended 状态/计数串入本次，
     导致失败转移后重试成功却仍记为 first_chunk_timeout）；
   - `firstChunkTimeoutSecondsForRequest()` 计算阈值，**仅当**开关开启 && 流式 &&
     `!ShouldSkipRetryAfterChannelAffinityFailure(c)`（亲和如 codex 钉 channel 51 → 返回 0，永不转移）
     && 未指定 `specific_channel_id` 时返回正值；
   - relay 返回后，若 `EndReason==FirstChunkTimeout && ReceivedResponseCount==0`，合成可重试
     **503**（`ErrorCodeBadResponse`），**复用既有重试机制**：503 落在自动重试区间 500-503，经
     `ShouldRetryByStatusCode` 放行；**不能用 504**——504/524 在 `alwaysSkipRetryStatusCodes` 中被
     显式列为永不重试，且 `channel_id` 在常规文本中继路径并不置入 ctx。不改动 `shouldRetryWithReason`。
6. **测试**：
   - `relay/helper/stream_scanner_first_chunk_timeout_test.go`：触发 / 默认禁用 / 首字早到三例；
   - `controller/relay_first_chunk_failover_test.go`：合成 503 可重试 + 504 不可重试（反证）、默认关闭恒为 0、
     亲和/指定渠道/非流式门控均为 0。
   - 全部 `-race` 通过；`controller` 包既有测试亦 `-race` 通过（重试循环改动无回归）。

**为何安全**：亲和请求阈值恒为 0 → 永不产生 FirstChunkTimeout → 不会 failover，codex prompt 缓存不受影响；
默认关闭 → 未开启前生产零变更；合成错误走既有重试路径 → 不触碰亲和状态机。
**启用方式**：将 `general_setting.first_chunk_timeout_enabled` 置为 true（建议先在低峰观察）。

## ❌ codex 超时类型（仍需 prod 实测，物理上非服务端能闭环）

- **空闲型超时** → keepalive（已实现）+ 默认 300s `StreamingTimeout` 已基本根治；
- **硬性总超时** → 服务端再优化也突破不了，**必须**同时调大 codex 客户端超时。
- 注：非亲和流量现在还多了一层兜底——开启 failover 后，慢渠道首字超时会切到更快渠道，
  对「硬性总超时」也有缓解（但 codex 因亲和不参与 failover，仍以 keepalive 兜底）。
- 验证：用 codex 发一个慢请求，看持续保活能否撑过 ~120s 并最终完成。

## 一句话结论

线上 0-token/`client_gone` 的服务端可控部分由 keepalive 修复；`StreamScannerHandler` **全部既有并发竞争
已修复并通过 `-race`**；`perf_metrics` 缺表已修（§九）；**首字超时失败转移已实现（默认关闭、亲和安全、`-race` 通过）**。
唯一非纯服务端能闭环的是 codex 客户端「硬性总超时」——只能由 prod 实测确认并在客户端侧调大。
