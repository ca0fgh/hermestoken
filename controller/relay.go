package controller

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/constant"
	"github.com/ca0fgh/hermestoken/dto"
	"github.com/ca0fgh/hermestoken/logger"
	"github.com/ca0fgh/hermestoken/middleware"
	"github.com/ca0fgh/hermestoken/model"
	perfmetrics "github.com/ca0fgh/hermestoken/pkg/perf_metrics"
	"github.com/ca0fgh/hermestoken/relay"
	relaycommon "github.com/ca0fgh/hermestoken/relay/common"
	relayconstant "github.com/ca0fgh/hermestoken/relay/constant"
	"github.com/ca0fgh/hermestoken/relay/helper"
	"github.com/ca0fgh/hermestoken/service"
	"github.com/ca0fgh/hermestoken/setting"
	"github.com/ca0fgh/hermestoken/setting/operation_setting"
	"github.com/ca0fgh/hermestoken/types"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/samber/lo"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func relayHandler(c *gin.Context, info *relaycommon.RelayInfo) *types.HermesTokenError {
	var err *types.HermesTokenError
	switch info.RelayMode {
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		err = relay.ImageHelper(c, info)
	case relayconstant.RelayModeAudioSpeech:
		fallthrough
	case relayconstant.RelayModeAudioTranslation:
		fallthrough
	case relayconstant.RelayModeAudioTranscription:
		err = relay.AudioHelper(c, info)
	case relayconstant.RelayModeRerank:
		err = relay.RerankHelper(c, info)
	case relayconstant.RelayModeEmbeddings:
		err = relay.EmbeddingHelper(c, info)
	case relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact:
		err = relay.ResponsesHelper(c, info)
	default:
		err = relay.TextHelper(c, info)
	}
	return err
}

func geminiRelayHandler(c *gin.Context, info *relaycommon.RelayInfo) *types.HermesTokenError {
	var err *types.HermesTokenError
	if strings.Contains(c.Request.URL.Path, "embed") {
		err = relay.GeminiEmbeddingHandler(c, info)
	} else {
		err = relay.GeminiHelper(c, info)
	}
	return err
}

func Relay(c *gin.Context, relayFormat types.RelayFormat) {

	requestId := c.GetString(common.RequestIdKey)
	//group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	//originalModel := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)
	if common.GetContextKeyBool(c, constant.ContextKeyMarketplaceUnifiedRelay) {
		MarketplaceUnifiedRelay(c, relayFormat)
		return
	}

	var (
		hermesTokenError *types.HermesTokenError
		ws               *websocket.Conn
	)
	defer func() { logRetryChannels(c, hermesTokenError == nil) }()

	if relayFormat == types.RelayFormatOpenAIRealtime {
		var err error
		ws, err = upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			helper.WssError(c, ws, types.NewError(err, types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry()).ToOpenAIError())
			return
		}
		defer ws.Close()
	}

	defer func() {
		if hermesTokenError != nil {
			logger.LogError(c, fmt.Sprintf("relay error: %s", hermesTokenError.Error()))
			hermesTokenError.SetMessage(common.MessageWithRequestId(hermesTokenError.Error(), requestId))
			switch relayFormat {
			case types.RelayFormatOpenAIRealtime:
				helper.WssError(c, ws, hermesTokenError.ToOpenAIError())
			case types.RelayFormatClaude:
				c.JSON(hermesTokenError.StatusCode, gin.H{
					"type":  "error",
					"error": hermesTokenError.ToClaudeError(),
				})
			default:
				c.JSON(hermesTokenError.StatusCode, gin.H{
					"error": hermesTokenError.ToOpenAIError(),
				})
			}
		}
	}()

	request, err := helper.GetAndValidateRequest(c, relayFormat)
	if err != nil {
		// Map "request body too large" to 413 so clients can handle it correctly
		if common.IsRequestBodyTooLargeError(err) || errors.Is(err, common.ErrRequestBodyTooLarge) {
			hermesTokenError = types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
		} else {
			hermesTokenError = types.NewError(err, types.ErrorCodeInvalidRequest)
		}
		return
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, relayFormat, request, ws)
	if err != nil {
		hermesTokenError = types.NewError(err, types.ErrorCodeGenRelayInfoFailed)
		return
	}

	needSensitiveCheck := setting.ShouldCheckPromptSensitive()
	needCountToken := constant.CountToken
	// Avoid building huge CombineText (strings.Join) when token counting and sensitive check are both disabled.
	var meta *types.TokenCountMeta
	if needSensitiveCheck || needCountToken {
		meta = request.GetTokenCountMeta()
	} else {
		meta = fastTokenCountMetaForPricing(request)
	}

	if needSensitiveCheck && meta != nil {
		contains, words := service.CheckSensitiveText(meta.CombineText)
		if contains {
			logger.LogWarn(c, fmt.Sprintf("user sensitive words detected: %s", strings.Join(words, ", ")))
			hermesTokenError = types.NewError(err, types.ErrorCodeSensitiveWordsDetected)
			return
		}
	}

	tokens, err := service.EstimateRequestToken(c, meta, relayInfo)
	if err != nil {
		hermesTokenError = types.NewError(err, types.ErrorCodeCountTokenFailed)
		return
	}

	relayInfo.SetEstimatePromptTokens(tokens)

	priceData, err := helper.ModelPriceHelper(c, relayInfo, tokens, meta)
	if err != nil {
		hermesTokenError = types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithStatusCode(http.StatusBadRequest))
		return
	}

	// common.SetContextKey(c, constant.ContextKeyTokenCountMeta, meta)

	if priceData.FreeModel {
		logger.LogInfo(c, fmt.Sprintf("模型 %s 免费，跳过预扣费", relayInfo.OriginModelName))
	} else {
		hermesTokenError = service.PreConsumeBilling(c, priceData.QuotaToPreConsume, relayInfo)
		if hermesTokenError != nil {
			return
		}
	}

	defer func() {
		// Only return quota if downstream failed and quota was actually pre-consumed
		if hermesTokenError != nil {
			hermesTokenError = service.NormalizeViolationFeeError(hermesTokenError)
			if relayInfo.Billing != nil {
				relayInfo.Billing.Refund(c)
			}
			service.ChargeViolationFeeIfNeeded(c, relayInfo, hermesTokenError)
		}
	}()

	retryParam := &service.RetryParam{
		Ctx:        c,
		TokenGroup: relayInfo.TokenGroup,
		ModelName:  relayInfo.OriginModelName,
		Retry:      common.GetPointer(0),
	}
	if err := retryParam.SeedSelectedChannel(retrySeedGroup(c, relayInfo), c.GetInt("channel_id")); err != nil {
		logger.LogError(c, fmt.Sprintf("seed retry channel state failed: %v", err))
	}
	relayInfo.RetryIndex = 0
	relayInfo.LastError = nil

	for ; retryParam.GetRetry() <= common.RetryTimes; retryParam.IncreaseRetry() {
		relayInfo.RetryIndex = retryParam.GetRetry()
		channel, channelErr := getChannel(c, relayInfo, retryParam)
		if channelErr != nil {
			logger.LogError(c, channelErr.Error())
			// 渠道耗尽属于终态失败：此前可重试的尝试按设计未落 type=5（见 processChannelError），
			// 这里用最后一次真实上游错误补记一条带完整 use_channel 重试链的错误日志，避免真失败漏记。
			if relayInfo.LastError != nil {
				setRetryAdminInfo(c, retryAdminInfo{
					Index:     retryParam.GetRetry(),
					Remaining: 0,
					WillRetry: false,
					Reason:    retryReasonSelectedChannelUnavailable,
				})
				processChannelError(c, c.GetInt("channel_id"), relayInfo.LastError)
			}
			hermesTokenError = channelErr
			break
		}

		addUsedChannel(c, channel.Id)
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			// Ensure consistent 413 for oversized bodies even when error occurs later (e.g., retry path)
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				hermesTokenError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
			} else {
				hermesTokenError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		// 每次尝试前重置流式状态：relayInfo 在重试间复用，而 StreamScannerHandler 仅在
		// StreamStatus==nil 时新建，故须在此显式重置，避免上一次尝试（尤其首字超时失败转移）的
		// StreamStatus（已 ended，后续 SetEndReason 均无效）与已转发计数串入本次尝试，导致结束
		// 原因 / 计费日志错乱（例如失败转移后重试成功，却仍记为 first_chunk_timeout）。
		relayInfo.StreamStatus = nil
		relayInfo.ReceivedResponseCount = 0
		// 首字超时失败转移：对「非亲和」流式请求设置阈值（默认关闭→恒为 0，零行为变更）。
		relayInfo.FirstChunkTimeoutSeconds = firstChunkTimeoutSecondsForRequest(c, relayInfo)

		switch relayFormat {
		case types.RelayFormatOpenAIRealtime:
			hermesTokenError = relay.WssHelper(c, relayInfo)
		case types.RelayFormatClaude:
			hermesTokenError = relay.ClaudeHelper(c, relayInfo)
		case types.RelayFormatGemini:
			hermesTokenError = geminiRelayHandler(c, relayInfo)
		default:
			hermesTokenError = relayHandler(c, relayInfo)
		}

		// 首字超时失败转移：StreamScannerHandler 在「等待首个 data: 超时且尚未转发任何输出」时
		// 把流标记为 FirstChunkTimeout（handler 通常仍返回 nil）。此处合成可重试 504，复用既有重试
		// 机制切换渠道。仅在 ReceivedResponseCount==0（没出过输出）时合成，满足「只重试没出过输出的」；
		// 亲和请求因 FirstChunkTimeoutSeconds=0 永不进入此分支，codex 渠道亲和与 prompt 缓存不受影响。
		// 用 503（在 500-503 自动重试区间内）：504/524 被显式列为「永不重试」(alwaysSkipRetryStatusCodes)，
		// 且 channel_id 在常规文本中继路径并不置入 ctx，故必须用落在重试区间的状态码才能可靠触发重试。
		if hermesTokenError == nil && relayInfo.IsStream && relayInfo.StreamStatus != nil &&
			relayInfo.StreamStatus.EndReason() == relaycommon.StreamEndReasonFirstChunkTimeout &&
			relayInfo.ReceivedResponseCount == 0 {
			hermesTokenError = types.NewErrorWithStatusCode(
				fmt.Errorf("first chunk timeout after %ds, failing over to another channel", relayInfo.FirstChunkTimeoutSeconds),
				types.ErrorCodeBadResponse, http.StatusServiceUnavailable)
			logger.LogError(c, fmt.Sprintf("first-chunk-timeout failover: channel=%d, %s", channel.Id, hermesTokenError.Error()))
		}

		if hermesTokenError == nil {
			relayInfo.LastError = nil
			return
		}

		hermesTokenError = service.NormalizeViolationFeeError(hermesTokenError)
		relayInfo.LastError = hermesTokenError

		retryRemaining := common.RetryTimes - retryParam.GetRetry()
		willRetry, retryReason := shouldRetryWithReason(c, hermesTokenError, retryRemaining)
		setRetryAdminInfo(c, retryAdminInfo{
			Index:     retryParam.GetRetry(),
			Remaining: retryRemaining,
			WillRetry: willRetry,
			Reason:    retryReason,
		})

		processChannelError(c, channel.Id, hermesTokenError)

		if !willRetry {
			break
		}
	}

	if hermesTokenError != nil {
		gopool.Go(func() {
			perfmetrics.RecordRelaySample(relayInfo, false, 0)
		})
	}
}

const (
	contextKeyRetryAdminInfo = "retry_admin_info"

	retryReasonNoError                    = "no_error"
	retryReasonSkipRetryError             = "skip_retry_error"
	retryReasonBudgetExhausted            = "retry_budget_exhausted"
	retryReasonSpecificChannel            = "specific_channel"
	retryReasonChannelError               = "channel_error"
	retryReasonSelectedChannelUnavailable = "selected_channel_unavailable"
	retryReasonChannelAffinitySkipRetry   = "channel_affinity_skip_retry"
	retryReasonSuccessfulStatusCode       = "successful_status_code"
	retryReasonInvalidStatusCode          = "invalid_status_code"
	retryReasonAlwaysSkipErrorCode        = "always_skip_error_code"
	retryReasonStatusCode                 = "status_code"
	retryReasonStatusCodeNotRetryable     = "status_code_not_retryable"
	retryReasonLockedChannel              = "locked_channel"
	retryReasonTaskLocalError             = "task_local_error"
)

type retryAdminInfo struct {
	Index     int
	Remaining int
	WillRetry bool
	Reason    string
}

func retrySeedGroup(c *gin.Context, relayInfo *relaycommon.RelayInfo) string {
	if relayInfo == nil {
		return ""
	}
	if relayInfo.TokenGroup == "auto" {
		if autoGroup := common.GetContextKeyString(c, constant.ContextKeyAutoGroup); autoGroup != "" {
			return autoGroup
		}
	}
	return relayInfo.UsingGroup
}

var upgrader = websocket.Upgrader{
	Subprotocols: []string{"realtime"}, // WS 握手支持的协议，如果有使用 Sec-WebSocket-Protocol，则必须在此声明对应的 Protocol TODO add other protocol
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许跨域
	},
}

func addUsedChannel(c *gin.Context, channelId int) {
	useChannel := c.GetStringSlice("use_channel")
	useChannel = append(useChannel, fmt.Sprintf("%d", channelId))
	c.Set("use_channel", useChannel)
}

func formatRetryChannels(useChannel []string) string {
	if len(useChannel) <= 1 {
		return ""
	}
	channels := make([]string, 0, len(useChannel))
	for _, channel := range useChannel {
		channel = strings.TrimSpace(channel)
		if channel == "" {
			continue
		}
		channels = append(channels, channel)
	}
	if len(channels) <= 1 {
		return ""
	}
	return strings.Join(channels, "->")
}

// logRetryChannels 在请求结束时汇总重试链路。recovered 表示最终是否命中可用渠道。
// 区分"重试成功"与"重试失败"，避免把"失败后已恢复"的请求误读为彻底失败。
func logRetryChannels(c *gin.Context, recovered bool) {
	if c == nil {
		return
	}
	retryChain := formatRetryChannels(c.GetStringSlice("use_channel"))
	if retryChain == "" {
		return
	}
	if recovered {
		logger.LogInfo(c, fmt.Sprintf("重试成功：%s（已切换到可用渠道，请求最终成功）", retryChain))
	} else {
		logger.LogWarn(c, fmt.Sprintf("重试失败：%s（所有渠道均返回错误，请求最终失败）", retryChain))
	}
}

func fastTokenCountMetaForPricing(request dto.Request) *types.TokenCountMeta {
	if request == nil {
		return &types.TokenCountMeta{}
	}
	meta := &types.TokenCountMeta{
		TokenType: types.TokenTypeTokenizer,
	}
	switch r := request.(type) {
	case *dto.GeneralOpenAIRequest:
		maxCompletionTokens := lo.FromPtrOr(r.MaxCompletionTokens, uint(0))
		maxTokens := lo.FromPtrOr(r.MaxTokens, uint(0))
		if maxCompletionTokens > maxTokens {
			meta.MaxTokens = int(maxCompletionTokens)
		} else {
			meta.MaxTokens = int(maxTokens)
		}
	case *dto.OpenAIResponsesRequest:
		meta.MaxTokens = int(lo.FromPtrOr(r.MaxOutputTokens, uint(0)))
	case *dto.ClaudeRequest:
		meta.MaxTokens = int(lo.FromPtr(r.MaxTokens))
	case *dto.ImageRequest:
		// Pricing for image requests depends on ImagePriceRatio; safe to compute even when CountToken is disabled.
		return r.GetTokenCountMeta()
	default:
		// Best-effort: leave CombineText empty to avoid large allocations.
	}
	return meta
}

func getChannel(c *gin.Context, info *relaycommon.RelayInfo, retryParam *service.RetryParam) (*model.Channel, *types.HermesTokenError) {
	if info.ChannelMeta == nil {
		return &model.Channel{
			Id:   c.GetInt("channel_id"),
			Type: c.GetInt("channel_type"),
			Name: c.GetString("channel_name"),
		}, nil
	}
	channel, selectGroup, err := service.CacheGetRandomSatisfiedChannel(retryParam)

	info.PriceData.GroupRatioInfo = helper.HandleGroupRatio(c, info)

	if err != nil {
		return nil, types.NewError(fmt.Errorf("获取分组 %s 下模型 %s 的可用渠道失败（retry）: %s", selectGroup, info.OriginModelName, err.Error()), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	if channel == nil {
		return nil, types.NewError(fmt.Errorf("分组 %s 下模型 %s 的可用渠道不存在（retry）", selectGroup, info.OriginModelName), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}

	hermesTokenError := middleware.SetupContextForSelectedChannel(c, channel, info.OriginModelName)
	if hermesTokenError != nil {
		return nil, hermesTokenError
	}
	return channel, nil
}

// firstChunkTimeoutSecondsForRequest 返回本次流式请求的首字超时阈值（秒）；0 表示不启用失败转移。
// 仅当全局开关开启、且为流式、且非渠道亲和（如 codex 钉 channel 51 不转移以保护 prompt 缓存）、
// 且未指定固定渠道时返回正值。默认关闭——开关关闭时恒返回 0，生产行为完全不变。
func firstChunkTimeoutSecondsForRequest(c *gin.Context, info *relaycommon.RelayInfo) int {
	if info == nil || !info.IsStream {
		return 0
	}
	gs := operation_setting.GetGeneralSetting()
	if !gs.FirstChunkTimeoutEnabled {
		return 0
	}
	// 渠道亲和（prompt_cache_key 钉渠道）：超时也不得切换，否则破坏上游 prompt 缓存。
	if service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
		return 0
	}
	// 用户显式指定固定渠道：不转移。
	if _, ok := c.Get("specific_channel_id"); ok {
		return 0
	}
	secs := gs.FirstChunkTimeoutSeconds
	if secs <= 0 {
		secs = 60
	}
	return secs
}

func shouldRetry(c *gin.Context, openaiErr *types.HermesTokenError, retryTimes int) bool {
	shouldRetry, _ := shouldRetryWithReason(c, openaiErr, retryTimes)
	return shouldRetry
}

func shouldRetryWithReason(c *gin.Context, openaiErr *types.HermesTokenError, retryTimes int) (bool, string) {
	if openaiErr == nil {
		return false, retryReasonNoError
	}
	if types.IsSkipRetryError(openaiErr) {
		return false, retryReasonSkipRetryError
	}
	if retryTimes <= 0 {
		return false, retryReasonBudgetExhausted
	}
	if _, ok := c.Get("specific_channel_id"); ok {
		return false, retryReasonSpecificChannel
	}
	if types.IsChannelError(openaiErr) {
		service.EnableRetryAfterChannelAffinityFailure(c)
		return true, retryReasonChannelError
	}
	if c.GetInt("channel_id") > 0 {
		service.EnableRetryAfterChannelAffinityFailure(c)
		return true, retryReasonSelectedChannelUnavailable
	}
	if !service.EnableRetryAfterChannelAffinityFailure(c) && service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
		return false, retryReasonChannelAffinitySkipRetry
	}
	code := openaiErr.StatusCode
	if code >= 200 && code < 300 {
		return false, retryReasonSuccessfulStatusCode
	}
	if code < 100 || code > 599 {
		return true, retryReasonInvalidStatusCode
	}
	if operation_setting.IsAlwaysSkipRetryCode(openaiErr.GetErrorCode()) {
		return false, retryReasonAlwaysSkipErrorCode
	}
	if operation_setting.ShouldRetryByStatusCode(code) {
		return true, retryReasonStatusCode
	}
	return false, retryReasonStatusCodeNotRetryable
}

func setRetryAdminInfo(c *gin.Context, info retryAdminInfo) {
	if c == nil {
		return
	}
	c.Set(contextKeyRetryAdminInfo, info)
}

// willRetryFromAdminInfo 读取本次尝试前由 setRetryAdminInfo 写入的重试意图。
// 缺失时（如非重试路径或直接调用）默认 false，即按终态错误处理。
func willRetryFromAdminInfo(c *gin.Context) bool {
	if c == nil {
		return false
	}
	value, exists := c.Get(contextKeyRetryAdminInfo)
	if !exists {
		return false
	}
	info, ok := value.(retryAdminInfo)
	if !ok {
		return false
	}
	return info.WillRetry
}

func appendRetryAdminInfo(c *gin.Context, adminInfo map[string]interface{}) {
	if c == nil || adminInfo == nil {
		return
	}
	value, exists := c.Get(contextKeyRetryAdminInfo)
	if !exists {
		return
	}
	info, ok := value.(retryAdminInfo)
	if !ok {
		return
	}
	adminInfo["retry_index"] = info.Index
	adminInfo["retry_remaining"] = info.Remaining
	adminInfo["retry_will_retry"] = info.WillRetry
	adminInfo["retry_final_attempt"] = !info.WillRetry
	if info.Reason != "" {
		adminInfo["retry_reason"] = info.Reason
	}
}

func processChannelError(c *gin.Context, channelId int, err *types.HermesTokenError) {
	// 单次渠道失败：若后续仍会重试，按可恢复事件记 WARN 应用日志即返回，不落 type=5 错误日志——
	// 否则一个最终成功（如 11->34->61）的请求会在后台留下多条红色"错误"，与真·终态失败混淆。
	// 只有重试耗尽 / 不可重试的终态失败才落库（见下方 RecordErrorLog）。渠道健康可看 WARN 应用日志，
	// 终态那条 type=5 也带完整 use_channel 重试链。终态失败的兜底落库见 Relay 循环中 getChannel 失败分支。
	if willRetryFromAdminInfo(c) {
		logger.LogWarn(c, fmt.Sprintf("channel error (channel #%d, status code: %d), will retry next channel: %s", channelId, err.StatusCode, err.Error()))
		return
	}
	logger.LogError(c, fmt.Sprintf("channel error (channel #%d, status code: %d), retries exhausted: %s", channelId, err.StatusCode, err.Error()))
	modelName := c.GetString("original_model")
	if constant.ErrorLogEnabled && types.IsRecordErrorLog(err) {
		// 保存错误日志到mysql中
		userId := c.GetInt("id")
		tokenName := c.GetString("token_name")
		tokenId := c.GetInt("token_id")
		userGroup := c.GetString("group")
		channelTestLogGroup := c.GetString(contextKeyChannelTestLogGroup)
		if channelTestLogGroup != "" {
			userGroup = channelTestLogGroup
		}
		other := make(map[string]interface{})
		if c.Request != nil && c.Request.URL != nil {
			other["request_path"] = c.Request.URL.Path
		}
		other["error_type"] = err.GetErrorType()
		other["error_code"] = err.GetErrorCode()
		other["status_code"] = err.StatusCode
		other["channel_id"] = channelId
		other["channel_name"] = c.GetString("channel_name")
		other["channel_type"] = c.GetInt("channel_type")
		adminInfo := make(map[string]interface{})
		adminInfo["use_channel"] = c.GetStringSlice("use_channel")
		isMultiKey := common.GetContextKeyBool(c, constant.ContextKeyChannelIsMultiKey)
		if isMultiKey {
			adminInfo["is_multi_key"] = true
			adminInfo["multi_key_index"] = common.GetContextKeyInt(c, constant.ContextKeyChannelMultiKeyIndex)
		}
		appendRetryAdminInfo(c, adminInfo)
		service.AppendChannelAffinityAdminInfo(c, adminInfo)
		other["admin_info"] = adminInfo
		if c.GetBool(contextKeyChannelTest) {
			addChannelTestLogInfo(other, c.GetStringSlice(contextKeyChannelTestGroups), c.GetString("group"))
		}
		startTime := common.GetContextKeyTime(c, constant.ContextKeyRequestStartTime)
		if startTime.IsZero() {
			startTime = time.Now()
		}
		useTimeSeconds := int(time.Since(startTime).Seconds())
		model.RecordErrorLog(c, userId, channelId, modelName, tokenName, err.MaskSensitiveErrorWithStatusCode(), tokenId, useTimeSeconds, common.GetContextKeyBool(c, constant.ContextKeyIsStream), userGroup, other)
	}

}

func RelayMidjourney(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatMjProxy, nil, nil)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"description": fmt.Sprintf("failed to generate relay info: %s", err.Error()),
			"type":        "upstream_error",
			"code":        4,
		})
		return
	}

	var mjErr *dto.MidjourneyResponse
	switch relayInfo.RelayMode {
	case relayconstant.RelayModeMidjourneyNotify:
		mjErr = relay.RelayMidjourneyNotify(c)
	case relayconstant.RelayModeMidjourneyTaskFetch, relayconstant.RelayModeMidjourneyTaskFetchByCondition:
		mjErr = relay.RelayMidjourneyTask(c, relayInfo.RelayMode)
	case relayconstant.RelayModeMidjourneyTaskImageSeed:
		mjErr = relay.RelayMidjourneyTaskImageSeed(c)
	case relayconstant.RelayModeSwapFace:
		mjErr = relay.RelaySwapFace(c, relayInfo)
	default:
		mjErr = relay.RelayMidjourneySubmit(c, relayInfo)
	}
	//err = relayMidjourneySubmit(c, relayMode)
	log.Println(mjErr)
	if mjErr != nil {
		statusCode := http.StatusBadRequest
		if mjErr.Code == 30 {
			mjErr.Result = "当前分组负载已饱和，请稍后再试，或升级账户以提升服务质量。"
			statusCode = http.StatusTooManyRequests
		}
		c.JSON(statusCode, gin.H{
			"description": fmt.Sprintf("%s %s", mjErr.Description, mjErr.Result),
			"type":        "upstream_error",
			"code":        mjErr.Code,
		})
		channelId := c.GetInt("channel_id")
		logger.LogError(c, fmt.Sprintf("relay error (channel #%d, status code %d): %s", channelId, statusCode, fmt.Sprintf("%s %s", mjErr.Description, mjErr.Result)))
	}
}

func RelayNotImplemented(c *gin.Context) {
	err := types.OpenAIError{
		Message: "API not implemented",
		Type:    "hermestoken_error",
		Param:   "",
		Code:    "api_not_implemented",
	}
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": err,
	})
}

func RelayNotFound(c *gin.Context) {
	err := types.OpenAIError{
		Message: fmt.Sprintf("Invalid URL (%s %s)", c.Request.Method, c.Request.URL.Path),
		Type:    "invalid_request_error",
		Param:   "",
		Code:    "",
	}
	c.JSON(http.StatusNotFound, gin.H{
		"error": err,
	})
}

func RelayTaskFetch(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &dto.TaskError{
			Code:       "gen_relay_info_failed",
			Message:    err.Error(),
			StatusCode: http.StatusInternalServerError,
		})
		return
	}
	if taskErr := relay.RelayTaskFetch(c, relayInfo.RelayMode); taskErr != nil {
		respondTaskError(c, taskErr)
	}
}

func RelayTask(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &dto.TaskError{
			Code:       "gen_relay_info_failed",
			Message:    err.Error(),
			StatusCode: http.StatusInternalServerError,
		})
		return
	}

	if taskErr := relay.ResolveOriginTask(c, relayInfo); taskErr != nil {
		respondTaskError(c, taskErr)
		return
	}

	var result *relay.TaskSubmitResult
	var taskErr *dto.TaskError
	defer func() {
		if taskErr != nil && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
	}()

	retryParam := &service.RetryParam{
		Ctx:        c,
		TokenGroup: relayInfo.TokenGroup,
		ModelName:  relayInfo.OriginModelName,
		Retry:      common.GetPointer(0),
	}
	if err := retryParam.SeedSelectedChannel(retrySeedGroup(c, relayInfo), c.GetInt("channel_id")); err != nil {
		logger.LogError(c, fmt.Sprintf("seed retry channel state failed: %v", err))
	}

	for ; retryParam.GetRetry() <= common.RetryTimes; retryParam.IncreaseRetry() {
		var channel *model.Channel
		lockedChannel := false

		if lockedCh, ok := relayInfo.LockedChannel.(*model.Channel); ok && lockedCh != nil {
			channel = lockedCh
			lockedChannel = true
			if retryParam.GetRetry() > 0 {
				if setupErr := middleware.SetupContextForSelectedChannel(c, channel, relayInfo.OriginModelName); setupErr != nil {
					taskErr = service.TaskErrorWrapperLocal(setupErr.Err, "setup_locked_channel_failed", http.StatusInternalServerError)
					break
				}
			}
		} else {
			var channelErr *types.HermesTokenError
			channel, channelErr = getChannel(c, relayInfo, retryParam)
			if channelErr != nil {
				logger.LogError(c, channelErr.Error())
				// 渠道耗尽属于终态失败：此前可重试的尝试按设计未落 type=5（见 processChannelError），
				// 这里用最后一次真实上游错误补记一条带完整 use_channel 重试链的错误日志，避免真失败漏记。
				if taskErr != nil && !taskErr.LocalError {
					setRetryAdminInfo(c, retryAdminInfo{
						Index:     retryParam.GetRetry(),
						Remaining: 0,
						WillRetry: false,
						Reason:    retryReasonSelectedChannelUnavailable,
					})
					processChannelError(c, c.GetInt("channel_id"), types.NewOpenAIError(taskErr.Error, types.ErrorCodeBadResponseStatusCode, taskErr.StatusCode))
				}
				taskErr = service.TaskErrorWrapperLocal(channelErr.Err, "get_channel_failed", http.StatusInternalServerError)
				break
			}
		}

		addUsedChannel(c, channel.Id)
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusRequestEntityTooLarge)
			} else {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusBadRequest)
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		result, taskErr = relay.RelayTaskSubmit(c, relayInfo)
		if taskErr == nil {
			break
		}

		retryRemaining := common.RetryTimes - retryParam.GetRetry()
		willRetry, retryReason := shouldRetryTaskRelayWithReason(c, channel.Id, lockedChannel, taskErr, retryRemaining)
		setRetryAdminInfo(c, retryAdminInfo{
			Index:     retryParam.GetRetry(),
			Remaining: retryRemaining,
			WillRetry: willRetry,
			Reason:    retryReason,
		})

		if !taskErr.LocalError {
			processChannelError(c, channel.Id, types.NewOpenAIError(taskErr.Error, types.ErrorCodeBadResponseStatusCode, taskErr.StatusCode))
		}

		if !willRetry {
			break
		}
	}

	logRetryChannels(c, taskErr == nil)

	// ── 成功：结算 + 日志 + 插入任务 ──
	if taskErr == nil {
		if settleErr := service.SettleBilling(c, relayInfo, result.Quota); settleErr != nil {
			common.SysError("settle task billing error: " + settleErr.Error())
		}
		service.LogTaskConsumption(c, relayInfo)

		task := model.InitTask(result.Platform, relayInfo)
		task.PrivateData.UpstreamTaskID = result.UpstreamTaskID
		task.PrivateData.BillingSource = relayInfo.BillingSource
		task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
		task.PrivateData.TokenId = relayInfo.TokenId
		task.PrivateData.BillingContext = &model.TaskBillingContext{
			ModelPrice:      relayInfo.PriceData.ModelPrice,
			GroupRatio:      relayInfo.PriceData.GroupRatioInfo.GroupRatio,
			ModelRatio:      relayInfo.PriceData.ModelRatio,
			OtherRatios:     relayInfo.PriceData.OtherRatios,
			OriginModelName: relayInfo.OriginModelName,
			PerCallBilling:  common.StringsContains(constant.TaskPricePatches, relayInfo.OriginModelName) || relayInfo.PriceData.UsePrice,
			TaskPerRequest:  relayInfo.PriceData.TaskPerRequestPrice,
			TaskPerSecond:   relayInfo.PriceData.TaskPerSecondPrice,
		}
		task.Quota = result.Quota
		task.Data = result.TaskData
		task.Action = relayInfo.Action
		if insertErr := task.Insert(); insertErr != nil {
			common.SysError("insert task error: " + insertErr.Error())
		}
	}

	if taskErr != nil {
		respondTaskError(c, taskErr)
	}
}

// respondTaskError 统一输出 Task 错误响应（含 429 限流提示改写）
func respondTaskError(c *gin.Context, taskErr *dto.TaskError) {
	if taskErr.StatusCode == http.StatusTooManyRequests {
		taskErr.Message = "当前分组上游负载已饱和，请稍后再试"
	}
	c.JSON(taskErr.StatusCode, taskErr)
}

func shouldRetryTaskRelay(c *gin.Context, channelId int, taskErr *dto.TaskError, retryTimes int) bool {
	shouldRetry, _ := shouldRetryTaskRelayWithReason(c, channelId, false, taskErr, retryTimes)
	return shouldRetry
}

func shouldRetryTaskRelayWithReason(c *gin.Context, channelId int, lockedChannel bool, taskErr *dto.TaskError, retryTimes int) (bool, string) {
	if taskErr == nil {
		return false, retryReasonNoError
	}
	if taskErr.LocalError {
		return false, retryReasonTaskLocalError
	}
	if retryTimes <= 0 {
		return false, retryReasonBudgetExhausted
	}
	if _, ok := c.Get("specific_channel_id"); ok {
		return false, retryReasonSpecificChannel
	}
	if lockedChannel {
		return false, retryReasonLockedChannel
	}
	if channelId > 0 {
		service.EnableRetryAfterChannelAffinityFailure(c)
		return true, retryReasonSelectedChannelUnavailable
	}
	if service.ShouldSkipRetryAfterChannelAffinityFailure(c) {
		return false, retryReasonChannelAffinitySkipRetry
	}
	if taskErr.StatusCode >= 200 && taskErr.StatusCode < 300 {
		return false, retryReasonSuccessfulStatusCode
	}
	if taskErr.StatusCode < 100 || taskErr.StatusCode > 599 {
		return true, retryReasonInvalidStatusCode
	}
	if operation_setting.ShouldRetryByStatusCode(taskErr.StatusCode) {
		return true, retryReasonStatusCode
	}
	return false, retryReasonStatusCodeNotRetryable
}
