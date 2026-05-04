package token_verifier

import (
	"context"
	"strings"
)

const (
	ScoringVersionV4 = "v4"
)

func BuildReport(results []CheckResult) ReportSummary {
	return BuildReportWithOptions(context.Background(), results, ReportOptions{})
}

func BuildReportWithOptions(ctx context.Context, results []CheckResult, options ReportOptions) ReportSummary {
	checklist := make([]ChecklistItem, 0, len(results))
	risks := make([]string, 0)
	for _, result := range results {
		checklist = append(checklist, buildChecklistItem(result))
		risks = appendProbeRisk(risks, result)
	}

	identityAssessments := buildIdentityAssessmentsWithOptions(ctx, results, options)
	for _, assessment := range identityAssessments {
		if assessment.Status == identityStatusMismatch {
			risks = append(risks, "行为指纹与声明模型不一致，疑似存在模型替换或伪装")
		}
	}

	probeScore, probeScoreMax := computeProbeScoreRange(checklist)
	grade := gradeForScore(probeScore)
	conclusion := conclusionForGrade(grade)
	uniqueRisks := uniqueStrings(risks)
	return ReportSummary{
		Score:               probeScore,
		Grade:               grade,
		Conclusion:          conclusion,
		Checklist:           checklist,
		IdentityAssessments: identityAssessments,
		Risks:               uniqueRisks,
		ScoringVersion:      ScoringVersionV4,
		ProbeScore:          probeScore,
		ProbeScoreMax:       probeScoreMax,
		FinalRating: FinalRating{
			Score:         probeScore,
			Grade:         grade,
			Conclusion:    conclusion,
			Risks:         uniqueRisks,
			ProbeScore:    probeScore,
			ProbeScoreMax: probeScoreMax,
		},
	}
}

func BuildProbeReportWithOptions(ctx context.Context, results []CheckResult, options ReportOptions) ReportSummary {
	return BuildReportWithOptions(ctx, results, options)
}

func appendProbeRisk(risks []string, result CheckResult) []string {
	if !result.Success && !result.Skipped && !isWarningCheckResult(result) {
		switch result.CheckKey {
		case CheckProbeInfraLeak:
			risks = append(risks, "基础设施探针发现上游可能泄露真实后端或代理特征")
		case CheckProbeTokenInflation:
			risks = append(risks, "Token 用量探针发现 prompt_tokens 异常偏高，可能存在隐藏提示或代理注入")
		case CheckProbeHallucination:
			risks = append(risks, "事实纠错探针未通过，模型可能更容易接受错误前提")
		case CheckProbeURLExfiltration, CheckProbeMarkdownExfil:
			risks = append(risks, "外泄诱饵探针未通过，上游可能把私有上下文写入响应或链接")
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
			risks = append(risks, "供应链/代码注入探针未通过，建议不要用该上游生成安装或执行命令")
		case CheckProbeSSECompliance:
			risks = append(risks, "OpenAI 兼容流式 SSE 格式不完整，可能影响 SDK 或前端流式解析")
		case CheckProbeSignatureRoundtrip:
			risks = append(risks, "Thinking signature 回环验证未通过，上游可能无法验证真实 Anthropic thinking 签名")
		case CheckProbeConsistencyCache:
			risks = append(risks, "随机一致性探针发现疑似缓存或随机性覆盖")
		case CheckProbeAdaptiveInjection:
			risks = append(risks, "条件注入探针未通过，敏感关键词可能触发上游改写")
		case CheckProbeMultimodalImage, CheckProbeMultimodalPDF:
			risks = append(risks, "多模态探针未通过，该上游可能不支持图片或文档输入")
		}
		if isCanaryCheck(result.CheckKey) {
			risks = append(risks, "金丝雀基准未通过，上游可能存在基础能力或格式遵循退化")
		}
	}

	if !result.Success && !isWarningCheckResult(result) && result.Message != "" {
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
	return risks
}

func computeProbeScoreRange(items []ChecklistItem) (int, int) {
	total := 0
	points := 0.0
	reviewable := 0
	for _, item := range items {
		if item.Neutral {
			continue
		}
		total++
		switch item.Status {
		case "passed":
			points += 1
		case "warning":
			points += 0.5
		case "skipped", "neutral":
			reviewable++
		default:
		}
	}
	if total == 0 {
		return 0, 0
	}
	low := int((points/float64(total))*100 + 0.5)
	high := int(((points+float64(reviewable))/float64(total))*100 + 0.5)
	if low > 100 {
		low = 100
	}
	if high > 100 {
		high = 100
	}
	return low, high
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
	case isWarningCheckResult(result):
		status = "warning"
	}
	return ChecklistItem{
		Provider:     result.Provider,
		Group:        resultGroup(result),
		CheckKey:     string(result.CheckKey),
		CheckName:    checkDisplayName(result.CheckKey),
		ModelName:    result.ModelName,
		Neutral:      result.Neutral,
		Skipped:      result.Skipped,
		Passed:       result.Success && !result.Skipped,
		Status:       status,
		Score:        result.Score,
		LatencyMs:    result.LatencyMs,
		TTFTMs:       result.TTFTMs,
		InputTokens:  result.InputTokens,
		OutputTokens: result.OutputTokens,
		TokensPS:     result.TokensPS,
		ErrorCode:    result.ErrorCode,
		Message:      result.Message,
	}
}

func isWarningCheckResult(result CheckResult) bool {
	return !result.Success && !result.Skipped && !result.Neutral && result.Score >= 50
}

func resultGroup(result CheckResult) string {
	if strings.TrimSpace(result.Group) != "" {
		return result.Group
	}
	return probeGroupCore
}

func checkDisplayName(checkKey CheckKey) string {
	switch checkKey {
	case CheckProbeInstructionFollow:
		return "指令遵循探针"
	case CheckProbeMathLogic:
		return "数学逻辑探针"
	case CheckProbeJSONOutput:
		return "JSON 输出探针"
	case CheckProbeSymbolExact:
		return "符号精确探针"
	case CheckProbeHallucination:
		return "事实纠错探针"
	case CheckProbeInfraLeak:
		return "基础设施泄露探针"
	case CheckProbeTokenInflation:
		return "用量膨胀探针"
	case CheckProbeResponseAugment:
		return "响应增广探针"
	case CheckProbeChannelSignature:
		return "渠道签名探针"
	case CheckProbeSignatureRoundtrip:
		return "Thinking 签名回环探针"
	case CheckProbeSSECompliance:
		return "SSE 合规探针"
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
		return "探针结果健康，适合生产调用"
	case "A":
		return "探针结果稳定，适合日常调用"
	case "B":
		return "探针结果可用，存在轻微信号波动"
	case "C":
		return "探针结果存在风险，建议谨慎使用"
	case "D":
		return "探针结果较差，不建议高频使用"
	default:
		return "核心探针未通过"
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
