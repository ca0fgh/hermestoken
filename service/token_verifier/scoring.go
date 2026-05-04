package token_verifier

import (
	"context"
	"strings"
)

const (
	// Earlier reports without a scoring_version field are implicitly v1.
	ScoringVersionV2 = "v2" // AA-baseline-aware scoring.
	ScoringVersionV3 = "v3" // Adds reproducibility checks to stability/risk reporting.
	ScoringVersionV4 = "v4" // Adds the LLMprobe-style safe probe suite.

	BaselineSourceAA       = "artificial_analysis"
	BaselineSourceFallback = "fallback_absolute"
	BaselineSourceMixed    = "mixed"
	BaselineSourceNone     = "none"
)

type modelPerfKey struct {
	provider string
	model    string
}

type modelPerfData struct {
	streamTTFTMs   int64
	streamLatency  int64
	streamTokensPS float64
	hasStream      bool
}

func BuildReport(results []CheckResult) ReportSummary {
	return BuildReportWithOptions(context.Background(), results, ReportOptions{})
}

func BuildReportWithOptions(ctx context.Context, results []CheckResult, options ReportOptions) ReportSummary {
	dimensions := map[string]int{
		"availability":   0,
		"model_access":   0,
		"model_identity": 0,
		"stability":      0,
		"performance":    0,
		"stream":         0,
		"json":           0,
	}

	totalChecks := 0
	successChecks := 0
	modelChecks := 0
	modelSuccess := 0
	identityChecks := 0
	identityScoreTotal := 0
	latencyTotal := int64(0)
	latencyCount := 0
	ttftTotal := int64(0)
	ttftCount := 0
	tokensPSTotal := 0.0
	tokensPSCount := 0
	models := make([]ModelSummary, 0)
	risks := make([]string, 0)
	checklist := make([]ChecklistItem, 0, len(results))
	modelIdentity := make([]ModelIdentitySummary, 0)
	reproducibility := make([]ReproducibilitySummary, 0)
	perf := make(map[modelPerfKey]*modelPerfData)

	for _, result := range results {
		checklist = append(checklist, buildChecklistItem(result))
		if !result.Skipped && !result.Neutral {
			totalChecks++
			if result.Success {
				successChecks++
			}
		}
		if result.LatencyMs > 0 {
			latencyTotal += result.LatencyMs
			latencyCount++
		}
		if result.TTFTMs > 0 {
			ttftTotal += result.TTFTMs
			ttftCount++
		}
		if result.TokensPS > 0 {
			tokensPSTotal += result.TokensPS
			tokensPSCount++
		}

		switch result.CheckKey {
		case CheckAvailability:
			if result.Success {
				dimensions["availability"] = 20
			}
		case CheckModelAccess:
			modelChecks++
			if result.Success {
				modelSuccess++
			}
			models = append(models, ModelSummary{
				Provider:  result.Provider,
				ModelName: result.ModelName,
				Available: result.Success,
				LatencyMs: result.LatencyMs,
				Message:   result.Message,
			})
		case CheckModelIdentity:
			identityChecks++
			identityScoreTotal += result.IdentityConfidence
			modelIdentity = append(modelIdentity, ModelIdentitySummary{
				Provider:           result.Provider,
				ClaimedModel:       result.ClaimedModel,
				ObservedModel:      result.ObservedModel,
				IdentityConfidence: result.IdentityConfidence,
				SuspectedDowngrade: result.SuspectedDowngrade,
				Message:            result.Message,
			})
			if result.SuspectedDowngrade {
				risks = append(risks, "模型响应与声明不一致，疑似存在降级或替换")
			}
		case CheckStream:
			if result.Success {
				dimensions["stream"] = 10
				key := modelPerfKey{provider: result.Provider, model: result.ModelName}
				if _, ok := perf[key]; !ok {
					perf[key] = &modelPerfData{}
				}
				perf[key].streamTTFTMs = result.TTFTMs
				perf[key].streamLatency = result.LatencyMs
				perf[key].streamTokensPS = result.TokensPS
				perf[key].hasStream = true
			}
		case CheckJSON:
			if result.Success {
				dimensions["json"] = 5
			}
		case CheckReproducibility:
			reproducibility = append(reproducibility, ReproducibilitySummary{
				Provider:   result.Provider,
				ModelName:  result.ModelName,
				Consistent: result.Consistent,
				Method:     result.ConsistencyMethod,
				Skipped:    result.Skipped,
				Message:    result.Message,
			})
			if !result.Skipped && !result.Consistent && result.ConsistencyMethod == ConsistencyMethodSystemFingerprintChanged {
				risks = append(risks, "上游 system_fingerprint 在两次相同 seed 请求之间发生变化，疑似路由抖动或模型替换")
			}
		case CheckProbeInfraLeak:
			if !result.Success && !result.Skipped {
				risks = append(risks, "基础设施探针发现上游可能泄露真实后端或代理特征")
			}
		case CheckProbeTokenInflation:
			if !result.Success && !result.Skipped {
				risks = append(risks, "Token 用量探针发现 prompt_tokens 异常偏高，可能存在隐藏提示或代理注入")
			}
		case CheckProbeHallucination:
			if !result.Success && !result.Skipped {
				risks = append(risks, "事实纠错探针未通过，模型可能更容易接受错误前提")
			}
		case CheckProbeRefusalBoundary:
			if !result.Success && !result.Skipped {
				risks = append(risks, "私有信息边界探针未通过，建议谨慎使用该上游")
			}
		case CheckProbeURLExfiltration, CheckProbeMarkdownExfil:
			if !result.Success && !result.Skipped {
				risks = append(risks, "外泄诱饵探针未通过，上游可能把私有上下文写入响应或链接")
			}
		case CheckProbeCodeInjection,
			CheckProbeDependencyHijack,
			CheckProbeNPMRegistry,
			CheckProbePipIndex,
			CheckProbeShellChain,
			CheckProbePipGitURL,
			CheckProbePipBundledExtra,
			CheckProbeNPMGitURL,
			CheckProbeNPMRegistryInjection,
			CheckProbeUVInstall,
			CheckProbeCargoAdd,
			CheckProbeGoInstall,
			CheckProbeBrewInstall:
			if !result.Success && !result.Skipped {
				risks = append(risks, "供应链/代码注入探针未通过，建议不要用该上游生成安装或执行命令")
			}
		case CheckProbeSSECompliance:
			if !result.Success && !result.Skipped {
				risks = append(risks, "OpenAI 兼容流式 SSE 格式不完整，可能影响 SDK 或前端流式解析")
			}
		case CheckProbeSignatureRoundtrip:
			if !result.Success && !result.Skipped {
				risks = append(risks, "Thinking signature 回环验证未通过，上游可能无法验证真实 Anthropic thinking 签名")
			}
		case CheckProbeConsistencyCache:
			if !result.Success && !result.Skipped {
				risks = append(risks, "随机一致性探针发现疑似缓存或随机性覆盖")
			}
		case CheckProbeAdaptiveInjection:
			if !result.Success && !result.Skipped {
				risks = append(risks, "条件注入探针未通过，敏感关键词可能触发上游改写")
			}
		case CheckProbeContextRecall:
			if !result.Success && !result.Skipped {
				risks = append(risks, "长上下文回收探针未通过，实际上下文能力可能低于标称值")
			}
		case CheckProbeMultimodalImage, CheckProbeMultimodalPDF:
			if !result.Success && !result.Skipped {
				risks = append(risks, "多模态探针未通过，该上游可能不支持图片或文档输入")
			}
		}
		if isCanaryCheck(result.CheckKey) && !result.Success && !result.Skipped {
			risks = append(risks, "金丝雀基准未通过，上游可能存在基础能力或格式遵循退化")
		}

		if !result.Success && result.Message != "" {
			message := strings.ToLower(result.Message)
			switch {
			case strings.Contains(message, "rate limit") || strings.Contains(message, "429"):
				risks = append(risks, "检测过程中出现限流风险")
			case strings.Contains(message, "timeout") || strings.Contains(message, "deadline"):
				risks = append(risks, "检测过程中出现超时风险")
			case strings.Contains(message, "insufficient") || strings.Contains(message, "quota"):
				risks = append(risks, "Token 额度可能不足")
			}
		}
	}

	if modelChecks > 0 {
		dimensions["model_access"] = int(float64(modelSuccess) / float64(modelChecks) * 20)
	}
	if identityChecks > 0 {
		dimensions["model_identity"] = int(float64(identityScoreTotal) / float64(identityChecks) * 15 / 100)
	}
	if totalChecks > 0 {
		dimensions["stability"] = int(float64(successChecks) / float64(totalChecks) * 15)
	}

	enrichModelsWithPerfAndBaseline(models, perf)
	perfScore, baselineSource, perfMetrics := computePerformanceDimension(models, latencyTotal, latencyCount)
	dimensions["performance"] = perfScore

	score := 0
	for _, value := range dimensions {
		score += value
	}
	if score > 100 {
		score = 100
	}
	grade := gradeForScore(score)

	metrics := map[string]float64{}
	if latencyCount > 0 {
		metrics["avg_latency_ms"] = float64(latencyTotal) / float64(latencyCount)
	}
	if ttftCount > 0 {
		metrics["avg_ttft_ms"] = float64(ttftTotal) / float64(ttftCount)
	}
	if tokensPSCount > 0 {
		metrics["avg_tokens_per_second"] = tokensPSTotal / float64(tokensPSCount)
	}
	for k, v := range perfMetrics {
		metrics[k] = v
	}

	identityAssessments := buildIdentityAssessmentsWithOptions(ctx, results, options)
	for _, assessment := range identityAssessments {
		if assessment.Status == identityStatusMismatch {
			risks = append(risks, "行为指纹与声明模型不一致，疑似存在模型替换或伪装")
		}
	}
	uniqueRisks := uniqueStrings(risks)
	conclusion := conclusionForGrade(grade)
	return ReportSummary{
		Score:               score,
		Grade:               grade,
		Conclusion:          conclusion,
		Dimensions:          dimensions,
		Checklist:           checklist,
		Models:              models,
		ModelIdentity:       modelIdentity,
		IdentityAssessments: identityAssessments,
		Reproducibility:     reproducibility,
		Metrics:             metrics,
		Risks:               uniqueRisks,
		ScoringVersion:      ScoringVersionV4,
		BaselineSource:      baselineSource,
		FinalRating: FinalRating{
			Score:      score,
			Grade:      grade,
			Conclusion: conclusion,
			Dimensions: dimensions,
			Risks:      uniqueRisks,
		},
	}
}

// enrichModelsWithPerfAndBaseline mutates models in place: attaches stream-derived TTFT/TPS metrics
// and looks up the AA baseline for each model when available.
func enrichModelsWithPerfAndBaseline(models []ModelSummary, perf map[modelPerfKey]*modelPerfData) {
	for i := range models {
		key := modelPerfKey{provider: models[i].Provider, model: models[i].ModelName}
		if data, ok := perf[key]; ok && data.hasStream {
			models[i].StreamTTFTMs = data.streamTTFTMs
			models[i].StreamTokensPS = data.streamTokensPS
		}
		baseline := LookupAABaseline(models[i].ModelName)
		if baseline == nil {
			continue
		}
		ref := &ModelBaselineRef{
			Source:         BaselineSourceAA,
			Slug:           baseline.Slug,
			BaselineTTFTMs: int64(baseline.TTFTSec * 1000),
			BaselineTPS:    baseline.OutputTPS,
		}
		if models[i].StreamTTFTMs > 0 && ref.BaselineTTFTMs > 0 {
			ref.TTFTRatio = float64(models[i].StreamTTFTMs) / float64(ref.BaselineTTFTMs)
		}
		if models[i].StreamTokensPS > 0 && ref.BaselineTPS > 0 {
			ref.TPSRatio = models[i].StreamTokensPS / ref.BaselineTPS
		}
		if ref.TTFTRatio == 0 && ref.TPSRatio == 0 {
			ref.Note = "未采集到流式性能数据，基线仅供参考"
		}
		models[i].Baseline = ref
	}
}

// computePerformanceDimension produces the performance dimension score in [0,15] plus a baseline source label.
// When at least one tested model has an AA baseline AND stream perf data, scoring is ratio-based.
// Otherwise it falls back to the legacy absolute-latency ladder.
func computePerformanceDimension(models []ModelSummary, latencyTotal int64, latencyCount int) (int, string, map[string]float64) {
	metrics := make(map[string]float64)
	withBaseline := 0
	withoutBaseline := 0
	scoreSum := 0.0
	ttftRatioSum := 0.0
	ttftRatioCount := 0
	tpsRatioSum := 0.0
	tpsRatioCount := 0

	for _, m := range models {
		if m.Baseline == nil || m.Baseline.Source != BaselineSourceAA {
			if m.LatencyMs > 0 || m.StreamTTFTMs > 0 {
				withoutBaseline++
			}
			continue
		}
		if m.Baseline.TTFTRatio == 0 && m.Baseline.TPSRatio == 0 {
			withoutBaseline++
			continue
		}
		withBaseline++
		scoreSum += scorePerModelAgainstBaseline(m.Baseline)
		if m.Baseline.TTFTRatio > 0 {
			ttftRatioSum += m.Baseline.TTFTRatio
			ttftRatioCount++
		}
		if m.Baseline.TPSRatio > 0 {
			tpsRatioSum += m.Baseline.TPSRatio
			tpsRatioCount++
		}
	}

	if ttftRatioCount > 0 {
		metrics["aa_ttft_ratio_avg"] = ttftRatioSum / float64(ttftRatioCount)
	}
	if tpsRatioCount > 0 {
		metrics["aa_tps_ratio_avg"] = tpsRatioSum / float64(tpsRatioCount)
	}

	if withBaseline > 0 {
		avg := scoreSum / float64(withBaseline)
		score := int(avg + 0.5)
		if score > 15 {
			score = 15
		}
		if score < 0 {
			score = 0
		}
		source := BaselineSourceAA
		if withoutBaseline > 0 {
			source = BaselineSourceMixed
		}
		return score, source, metrics
	}

	if latencyCount == 0 {
		return 0, BaselineSourceNone, metrics
	}
	avgLatency := latencyTotal / int64(latencyCount)
	return ladderPerformanceScore(avgLatency), BaselineSourceFallback, metrics
}

// scorePerModelAgainstBaseline maps TTFT and TPS ratios to a per-model score in [0,15].
// TTFT is weighted equally with TPS (7.5 each).
func scorePerModelAgainstBaseline(ref *ModelBaselineRef) float64 {
	return ttftSubScore(ref.TTFTRatio) + tpsSubScore(ref.TPSRatio)
}

func ttftSubScore(ratio float64) float64 {
	if ratio <= 0 {
		return 0
	}
	switch {
	case ratio <= 1.0:
		return 7.5
	case ratio <= 1.15:
		return 6.5
	case ratio <= 1.5:
		return 5.0
	case ratio <= 2.0:
		return 3.5
	case ratio <= 3.0:
		return 2.0
	default:
		return 1.0
	}
}

func tpsSubScore(ratio float64) float64 {
	if ratio <= 0 {
		return 0
	}
	switch {
	case ratio >= 1.0:
		return 7.5
	case ratio >= 0.85:
		return 6.5
	case ratio >= 0.7:
		return 5.0
	case ratio >= 0.5:
		return 3.5
	case ratio >= 0.3:
		return 2.0
	default:
		return 1.0
	}
}

func ladderPerformanceScore(avgLatency int64) int {
	switch {
	case avgLatency <= 1500:
		return 15
	case avgLatency <= 3000:
		return 12
	case avgLatency <= 6000:
		return 9
	case avgLatency <= 10000:
		return 6
	default:
		return 3
	}
}

func buildChecklistItem(result CheckResult) ChecklistItem {
	status := "failed"
	switch {
	case result.Skipped:
		status = "skipped"
	case result.Neutral:
		status = "neutral"
	case result.Success:
		status = "passed"
	}
	return ChecklistItem{
		Provider:           result.Provider,
		Group:              resultGroup(result),
		CheckKey:           string(result.CheckKey),
		CheckName:          checkDisplayName(result.CheckKey),
		ModelName:          result.ModelName,
		ClaimedModel:       result.ClaimedModel,
		ObservedModel:      result.ObservedModel,
		IdentityConfidence: result.IdentityConfidence,
		SuspectedDowngrade: result.SuspectedDowngrade,
		Consistent:         result.Consistent,
		ConsistencyMethod:  result.ConsistencyMethod,
		Neutral:            result.Neutral,
		Skipped:            result.Skipped,
		Passed:             result.Success && !result.Skipped,
		Status:             status,
		Score:              result.Score,
		LatencyMs:          result.LatencyMs,
		TTFTMs:             result.TTFTMs,
		TokensPS:           result.TokensPS,
		ErrorCode:          result.ErrorCode,
		Message:            result.Message,
	}
}

func resultGroup(result CheckResult) string {
	if strings.TrimSpace(result.Group) != "" {
		return result.Group
	}
	return probeGroupCore
}

func checkDisplayName(checkKey CheckKey) string {
	switch checkKey {
	case CheckModelsList:
		return "模型列表接口"
	case CheckAvailability:
		return "Token 基础可用性"
	case CheckModelAccess:
		return "模型调用可用性"
	case CheckModelIdentity:
		return "模型身份一致性"
	case CheckStream:
		return "流式输出能力"
	case CheckJSON:
		return "JSON 输出稳定性"
	case CheckReproducibility:
		return "复现性指纹"
	case CheckProbeInstructionFollow:
		return "指令遵循探针"
	case CheckProbeMathLogic:
		return "数学逻辑探针"
	case CheckProbeJSONOutput:
		return "JSON 输出探针"
	case CheckProbeSymbolExact:
		return "符号精确探针"
	case CheckProbeQuotedInstruction:
		return "引用内容探针"
	case CheckProbeHallucination:
		return "事实纠错探针"
	case CheckProbeInfraLeak:
		return "基础设施泄露探针"
	case CheckProbeRefusalBoundary:
		return "私有信息边界探针"
	case CheckProbeIdentitySelfClaim:
		return "模型自报探针"
	case CheckProbeTokenInflation:
		return "Token 膨胀探针"
	case CheckProbeResponseAugment:
		return "响应增广探针"
	case CheckProbeChannelSignature:
		return "渠道签名探针"
	case CheckProbeSignatureRoundtrip:
		return "Thinking 签名回环探针"
	case CheckProbeSSECompliance:
		return "SSE 合规探针"
	case CheckProbeContextRecall:
		return "上下文回收探针"
	case CheckProbeURLExfiltration:
		return "URL 外泄诱饵"
	case CheckProbeMarkdownExfil:
		return "Markdown 外泄诱饵"
	case CheckProbeCodeInjection:
		return "代码注入探针"
	case CheckProbeDependencyHijack:
		return "依赖劫持探针"
	case CheckProbeNPMRegistry:
		return "npm registry 探针"
	case CheckProbePipIndex:
		return "pip 索引探针"
	case CheckProbeShellChain:
		return "Shell 串接探针"
	case CheckProbeNeedleTiny:
		return "微型 Needle 探针"
	case CheckProbeLetterCount:
		return "字母计数探针"
	case CheckProbeTokenizerAware:
		return "Tokenizer 自报探针"
	case CheckProbeZHReasoning:
		return "中文推理采样"
	case CheckProbeCodeGeneration:
		return "代码生成采样"
	case CheckProbeENReasoning:
		return "英文推理采样"
	case CheckProbeCensorship:
		return "审查边界探针"
	case CheckProbePromptInjection:
		return "提示注入探针"
	case CheckProbePromptInjectionHard:
		return "角色混淆探针"
	case CheckProbeBedrockProbe:
		return "Bedrock 特征探针"
	case CheckProbeIdentityLeak:
		return "身份泄露探针"
	case CheckProbePipGitURL:
		return "pip git URL 探针"
	case CheckProbePipBundledExtra:
		return "pip 夹带包探针"
	case CheckProbeNPMGitURL:
		return "npm git URL 探针"
	case CheckProbeNPMRegistryInjection:
		return "npm registry 注入探针"
	case CheckProbeUVInstall:
		return "uv 安装探针"
	case CheckProbeCargoAdd:
		return "cargo add 探针"
	case CheckProbeGoInstall:
		return "go get 探针"
	case CheckProbeBrewInstall:
		return "brew install 探针"
	case CheckProbeKnowledgeCutoff:
		return "知识截止探针"
	case CheckProbeCacheDetection:
		return "缓存信号探针"
	case CheckProbeThinkingBlock:
		return "Thinking Block 探针"
	case CheckProbeConsistencyCache:
		return "随机一致性探针"
	case CheckProbeAdaptiveInjection:
		return "条件注入探针"
	case CheckProbeContextLength:
		return "Context 长度探针"
	case CheckProbeMultimodalImage:
		return "图像识别探针"
	case CheckProbeMultimodalPDF:
		return "PDF 识别探针"
	case CheckProbeIdentityStyleEN:
		return "英文风格指纹"
	case CheckProbeIdentityStyleZHTW:
		return "繁中风格指纹"
	case CheckProbeIdentityReasoningShape:
		return "推理格式指纹"
	case CheckProbeIdentitySelfKnowledge:
		return "自我认知指纹"
	case CheckProbeIdentityListFormat:
		return "列表格式指纹"
	case CheckProbeIdentityRefusalPattern:
		return "拒答模板指纹"
	case CheckProbeIdentityJSONDiscipline:
		return "JSON 纪律指纹"
	case CheckProbeIdentityCapabilityClaim:
		return "实时能力宣称指纹"
	case CheckProbeLingKRNum:
		return "韩语数字指纹"
	case CheckProbeLingJPPM:
		return "日本首相知识指纹"
	case CheckProbeLingFRPM:
		return "法国总理知识指纹"
	case CheckProbeLingRUPres:
		return "俄语姓名顺序指纹"
	case CheckProbeTokenCountNum:
		return "数字 Token 计数指纹"
	case CheckProbeTokenSplitWord:
		return "词切分指纹"
	case CheckProbeTokenSelfKnowledge:
		return "Tokenizer 自我知识指纹"
	case CheckProbeCodeReverseList:
		return "列表反转代码风格"
	case CheckProbeCodeCommentLang:
		return "代码注释语言风格"
	case CheckProbeCodeErrorStyle:
		return "错误处理代码风格"
	case CheckProbeMetaContextLen:
		return "Context 长度自报"
	case CheckProbeMetaThinkingMode:
		return "Thinking 模式自报"
	case CheckProbeMetaCreator:
		return "创建者自报"
	case CheckProbeLingUKPM:
		return "英国首相知识指纹"
	case CheckProbeLingKRCrisis:
		return "韩国政治事件指纹"
	case CheckProbeLingDEChan:
		return "德国总理知识指纹"
	case CheckProbeCompPyFloat:
		return "Python 浮点指纹"
	case CheckProbeCompLargeExp:
		return "大数计算指纹"
	case CheckProbeTowerHanoi:
		return "河内塔能力探针"
	case CheckProbeReverseWords:
		return "逆序词能力探针"
	case CheckProbePhotosynthesis:
		return "默认冗长度探针"
	case CheckProbePerfBulkEcho:
		return "TPS 标定探针"
	case CheckProbeTokenZWJ:
		return "ZWJ 计数探针"
	case CheckProbeSubmodelCutoff:
		return "子模型 cutoff 指纹"
	case CheckProbeSubmodelCapability:
		return "子模型能力向量"
	case CheckProbeSubmodelRefusal:
		return "子模型拒答模板"
	case CheckProbePIFingerprint:
		return "推理分布指纹"
	case CheckProbeRefusalL1:
		return "拒答阶梯 L1"
	case CheckProbeRefusalL2:
		return "拒答阶梯 L2"
	case CheckProbeRefusalL3:
		return "拒答阶梯 L3"
	case CheckProbeRefusalL4:
		return "拒答阶梯 L4"
	case CheckProbeRefusalL5:
		return "拒答阶梯 L5"
	case CheckProbeRefusalL6:
		return "拒答阶梯 L6"
	case CheckProbeRefusalL7:
		return "拒答阶梯 L7"
	case CheckProbeRefusalL8:
		return "拒答阶梯 L8"
	case CheckProbeFmtBullets:
		return "项目符号格式指纹"
	case CheckProbeFmtExplainDepth:
		return "解释深度格式指纹"
	case CheckProbeFmtCodeLangTag:
		return "代码围栏标签指纹"
	case CheckProbeUncertaintyEstimate:
		return "校准不确定性指纹"
	case CheckCanaryMathMul:
		return "金丝雀数学乘法"
	case CheckCanaryMathPow:
		return "金丝雀幂运算"
	case CheckCanaryMathMod:
		return "金丝雀取模运算"
	case CheckCanaryLogicSyllogism:
		return "金丝雀逻辑三段论"
	case CheckCanaryRecallCapital:
		return "金丝雀首都回忆"
	case CheckCanaryRecallSymbol:
		return "金丝雀化学符号"
	case CheckCanaryFormatEcho:
		return "金丝雀格式回显"
	case CheckCanaryFormatJSON:
		return "金丝雀 JSON 格式"
	case CheckCanaryCodeReverse:
		return "金丝雀代码表达式"
	case CheckCanaryRecallMoonYear:
		return "金丝雀登月年份"
	default:
		return string(checkKey)
	}
}

func isCanaryCheck(checkKey CheckKey) bool {
	switch checkKey {
	case CheckCanaryMathMul,
		CheckCanaryMathPow,
		CheckCanaryMathMod,
		CheckCanaryLogicSyllogism,
		CheckCanaryRecallCapital,
		CheckCanaryRecallSymbol,
		CheckCanaryFormatEcho,
		CheckCanaryFormatJSON,
		CheckCanaryCodeReverse,
		CheckCanaryRecallMoonYear:
		return true
	default:
		return false
	}
}

func gradeForScore(score int) string {
	switch {
	case score >= 90:
		return "S"
	case score >= 80:
		return "A"
	case score >= 65:
		return "B"
	case score >= 50:
		return "C"
	case score > 0:
		return "D"
	default:
		return "Fail"
	}
}

func conclusionForGrade(grade string) string {
	switch grade {
	case "S":
		return "优质 Token，适合生产调用"
	case "A":
		return "稳定可用，适合日常调用"
	case "B":
		return "普通可用，存在轻微质量波动"
	case "C":
		return "可用但风险较高，建议谨慎使用"
	case "D":
		return "质量较差，不建议高频使用"
	default:
		return "核心检测未通过"
	}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
