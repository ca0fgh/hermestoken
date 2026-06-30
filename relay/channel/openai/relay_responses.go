package openai

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/dto"
	"github.com/ca0fgh/hermestoken/logger"
	relaycommon "github.com/ca0fgh/hermestoken/relay/common"
	"github.com/ca0fgh/hermestoken/relay/helper"
	"github.com/ca0fgh/hermestoken/service"
	"github.com/ca0fgh/hermestoken/types"

	"github.com/gin-gonic/gin"
)

func OaiResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.HermesTokenError) {
	defer service.CloseResponseBodyGracefully(resp)

	// read response body
	var responsesResponse dto.OpenAIResponsesResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	err = common.Unmarshal(responseBody, &responsesResponse)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := responsesResponse.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	if responsesResponse.HasImageGenerationCall() {
		c.Set("image_generation_call", true)
		c.Set("image_generation_call_quality", responsesResponse.GetQuality())
		c.Set("image_generation_call_size", responsesResponse.GetSize())
	}

	// 写入新的 response body
	service.IOCopyBytesGracefully(c, resp, responseBody)

	// compute usage
	usage := dto.Usage{}
	if responsesResponse.Usage != nil {
		usage.PromptTokens = responsesResponse.Usage.InputTokens
		usage.CompletionTokens = responsesResponse.Usage.OutputTokens
		usage.TotalTokens = responsesResponse.Usage.TotalTokens
		if responsesResponse.Usage.InputTokensDetails != nil {
			usage.PromptTokensDetails.CachedTokens = responsesResponse.Usage.InputTokensDetails.CachedTokens
		}
	}
	if info == nil || info.ResponsesUsageInfo == nil || info.ResponsesUsageInfo.BuiltInTools == nil {
		return &usage, nil
	}
	// 解析 Tools 用量
	for _, tool := range responsesResponse.Tools {
		buildToolinfo, ok := info.ResponsesUsageInfo.BuiltInTools[common.Interface2String(tool["type"])]
		if !ok || buildToolinfo == nil {
			logger.LogError(c, fmt.Sprintf("BuiltInTools not found for tool type: %v", tool["type"]))
			continue
		}
		buildToolinfo.CallCount++
	}
	return &usage, nil
}

func OaiResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.HermesTokenError) {
	if resp == nil || resp.Body == nil {
		logger.LogError(c, "invalid response or response body")
		return nil, types.NewError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse)
	}

	defer service.CloseResponseBodyGracefully(resp)

	var usage = &dto.Usage{}
	var responseTextBuilder strings.Builder
	// 上游在流中显式上报失败（response.failed / response.error / error 事件）时记录，
	// 用于在扫描结束后合成可重试的渠道错误，触发故障转移。
	var upstreamStreamErr *types.HermesTokenError

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {

		// 检查当前数据是否包含 completed 状态和 usage 信息
		var streamResponse dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(data, &streamResponse); err != nil {
			logger.LogError(c, "failed to unmarshal stream response: "+err.Error())
			sr.Error(err)
			return
		}
		sendResponsesStreamData(c, streamResponse, data)
		switch streamResponse.Type {
		case "response.failed", "response.error", "error":
			// 上游显式失败事件：HTTP 仍是 200，但本次响应不可用。构造可重试的渠道错误
			// （channel: 前缀 → IsChannelError，绕过亲和跳过重试），扫描结束后抛出以切换渠道。
			if upstreamStreamErr == nil {
				detail := streamResponse.Type
				// 优先取上游真实错误原因（嵌套 response.error 或裸 error 事件顶层 message/code），
				// 取不到才退回事件类型名，避免日志退化成无信息的 "(error)"。
				if reason := streamResponse.ErrorDetail(); reason != "" {
					detail = fmt.Sprintf("%s: %s", streamResponse.Type, reason)
				}
				upstreamStreamErr = types.NewErrorWithStatusCode(
					fmt.Errorf("upstream responses stream failed (%s)", detail),
					types.ErrorCodeChannelEmptyResponse, http.StatusServiceUnavailable)
			}
		case "response.incomplete":
			// 不完整但已生成部分内容（如命中 max_output_tokens）：仍带 usage，按完成态计费，
			// 与 response.completed 共用下方 usage 提取逻辑。
			fallthrough
		case "response.completed":
			if streamResponse.Response != nil {
				if streamResponse.Response.Usage != nil {
					if streamResponse.Response.Usage.InputTokens != 0 {
						usage.PromptTokens = streamResponse.Response.Usage.InputTokens
					}
					if streamResponse.Response.Usage.OutputTokens != 0 {
						usage.CompletionTokens = streamResponse.Response.Usage.OutputTokens
					}
					if streamResponse.Response.Usage.TotalTokens != 0 {
						usage.TotalTokens = streamResponse.Response.Usage.TotalTokens
					}
					if streamResponse.Response.Usage.InputTokensDetails != nil {
						usage.PromptTokensDetails.CachedTokens = streamResponse.Response.Usage.InputTokensDetails.CachedTokens
					}
				}
				if streamResponse.Response.HasImageGenerationCall() {
					c.Set("image_generation_call", true)
					c.Set("image_generation_call_quality", streamResponse.Response.GetQuality())
					c.Set("image_generation_call_size", streamResponse.Response.GetSize())
				}
			}
		case "response.output_text.delta":
			// 处理输出文本
			responseTextBuilder.WriteString(streamResponse.Delta)
		case dto.ResponsesOutputTypeItemDone:
			// 函数调用处理
			if streamResponse.Item != nil {
				switch streamResponse.Item.Type {
				case dto.BuildInCallWebSearchCall:
					if info != nil && info.ResponsesUsageInfo != nil && info.ResponsesUsageInfo.BuiltInTools != nil {
						if webSearchTool, exists := info.ResponsesUsageInfo.BuiltInTools[dto.BuildInToolWebSearchPreview]; exists && webSearchTool != nil {
							webSearchTool.CallCount++
						}
					}
				}
			}
		}
	})

	if usage.CompletionTokens == 0 {
		// 计算输出文本的 token 数量
		tempStr := responseTextBuilder.String()
		if len(tempStr) > 0 {
			// 非正常结束，使用输出文本的 token 数量
			completionTokens := service.CountTextToken(tempStr, info.UpstreamModelName)
			usage.CompletionTokens = completionTokens
		}
	}

	if usage.PromptTokens == 0 && usage.CompletionTokens != 0 {
		usage.PromptTokens = info.GetEstimatePromptTokens()
	}

	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens

	// 上游显式失败事件：直接抛出可重试渠道错误，切换到健康渠道。
	if upstreamStreamErr != nil {
		logger.LogError(c, "responses stream upstream failure: "+upstreamStreamErr.Error())
		return usage, upstreamStreamErr
	}

	// 退化空响应失败转移：流已正常结束（eof/done）但既无 usage 也无任何输出文本——
	// 上游接受请求、通常只推了 response.created 等前导元数据后即关闭，未产出任何答案。
	// 这种「假成功」当前会被记成 0-token 消费日志且不切渠道，用户侧表现为静默失败。
	// 此处合成可重试的渠道错误，复用既有重试链切换到健康渠道。
	// 严格门控以确保绝不影响成功响应：
	//   1. 总 token 为 0（既无 usage 事件也无任何输出文本——成功响应必有其一）；
	//   2. 非 client_gone（客户端断开时重试无意义且会写已死连接，保持原行为=记 0 退款）；
	//   3. RELAY_RESPONSES_EMPTY_FAILOVER 开关（默认开，可作运维急停）。
	if common.ResponsesEmptyStreamFailover && usage.TotalTokens == 0 {
		clientGone := info != nil && info.StreamStatus != nil &&
			info.StreamStatus.EndReason() == relaycommon.StreamEndReasonClientGone
		if !clientGone {
			endReason := relaycommon.StreamEndReasonNone
			if info != nil && info.StreamStatus != nil {
				endReason = info.StreamStatus.EndReason()
			}
			emptyErr := types.NewErrorWithStatusCode(
				fmt.Errorf("upstream returned empty responses stream (no usage, no output text; end_reason=%s)", endReason),
				types.ErrorCodeChannelEmptyResponse, http.StatusServiceUnavailable)
			logger.LogError(c, "responses stream empty, failing over: "+emptyErr.Error())
			return usage, emptyErr
		}
	}

	return usage, nil
}
