package token_verifier

type CheckKey string

const (
	ProviderOpenAI    = "openai"
	ProviderAnthropic = "anthropic"

	CheckProbeInstructionFollow       CheckKey = "probe_instruction_follow"
	CheckProbeMathLogic               CheckKey = "probe_math_logic"
	CheckProbeJSONOutput              CheckKey = "probe_json_output"
	CheckProbeSymbolExact             CheckKey = "probe_symbol_exact"
	CheckProbeHallucination           CheckKey = "probe_hallucination"
	CheckProbeInfraLeak               CheckKey = "probe_infra_leak"
	CheckProbeTokenInflation          CheckKey = "probe_token_inflation"
	CheckProbeResponseAugment         CheckKey = "probe_response_augmentation"
	CheckProbeChannelSignature        CheckKey = "probe_channel_signature"
	CheckProbeSignatureRoundtrip      CheckKey = "probe_signature_roundtrip"
	CheckProbeSSECompliance           CheckKey = "probe_sse_compliance"
	CheckProbeURLExfiltration         CheckKey = "probe_url_exfiltration"
	CheckProbeMarkdownExfil           CheckKey = "probe_markdown_exfiltration"
	CheckProbeCodeInjection           CheckKey = "probe_code_injection"
	CheckProbeDependencyHijack        CheckKey = "probe_dependency_hijack"
	CheckProbeNPMRegistry             CheckKey = "probe_npm_registry"
	CheckProbePipIndex                CheckKey = "probe_pip_index"
	CheckProbeShellChain              CheckKey = "probe_shell_chain"
	CheckProbeNeedleTiny              CheckKey = "probe_needle_tiny"
	CheckProbeLetterCount             CheckKey = "probe_letter_count"
	CheckProbeZHReasoning             CheckKey = "probe_zh_reasoning"
	CheckProbeCodeGeneration          CheckKey = "probe_code_generation"
	CheckProbeENReasoning             CheckKey = "probe_en_reasoning"
	CheckProbeCensorship              CheckKey = "probe_censorship"
	CheckProbePromptInjection         CheckKey = "probe_prompt_injection"
	CheckProbePromptInjectionHard     CheckKey = "probe_prompt_injection_hard"
	CheckProbeBedrockProbe            CheckKey = "probe_bedrock_probe"
	CheckProbeIdentityLeak            CheckKey = "probe_identity_leak"
	CheckProbePipGitURL               CheckKey = "probe_pip_git_url"
	CheckProbePipBundledExtra         CheckKey = "probe_pip_bundled_extra"
	CheckProbeNPMGitURL               CheckKey = "probe_npm_git_url"
	CheckProbeNPMRegistryInjection    CheckKey = "probe_npm_registry_injection"
	CheckProbeUVInstall               CheckKey = "probe_uv_install"
	CheckProbeCargoAdd                CheckKey = "probe_cargo_add"
	CheckProbeGoInstall               CheckKey = "probe_go_install"
	CheckProbeBrewInstall             CheckKey = "probe_brew_install"
	CheckProbeKnowledgeCutoff         CheckKey = "probe_knowledge_cutoff"
	CheckProbeCacheDetection          CheckKey = "probe_cache_detection"
	CheckProbeThinkingBlock           CheckKey = "probe_thinking_block"
	CheckProbeConsistencyCache        CheckKey = "probe_consistency_cache"
	CheckProbeAdaptiveInjection       CheckKey = "probe_adaptive_injection"
	CheckProbeContextLength           CheckKey = "probe_context_length"
	CheckProbeMultimodalImage         CheckKey = "probe_multimodal_image"
	CheckProbeMultimodalPDF           CheckKey = "probe_multimodal_pdf"
	CheckProbeIdentityStyleEN         CheckKey = "probe_identity_style_en"
	CheckProbeIdentityStyleZHTW       CheckKey = "probe_identity_style_zh_tw"
	CheckProbeIdentityReasoningShape  CheckKey = "probe_identity_reasoning_shape"
	CheckProbeIdentitySelfKnowledge   CheckKey = "probe_identity_self_knowledge"
	CheckProbeIdentityListFormat      CheckKey = "probe_identity_list_format"
	CheckProbeIdentityRefusalPattern  CheckKey = "probe_identity_refusal_pattern"
	CheckProbeIdentityJSONDiscipline  CheckKey = "probe_identity_json_discipline"
	CheckProbeIdentityCapabilityClaim CheckKey = "probe_identity_capability_claim"
	CheckProbeLingKRNum               CheckKey = "probe_ling_kr_num"
	CheckProbeLingJPPM                CheckKey = "probe_ling_jp_pm"
	CheckProbeLingFRPM                CheckKey = "probe_ling_fr_pm"
	CheckProbeLingRUPres              CheckKey = "probe_ling_ru_pres"
	CheckProbeTokenCountNum           CheckKey = "probe_token_count_num"
	CheckProbeTokenSplitWord          CheckKey = "probe_token_split_word"
	CheckProbeTokenSelfKnowledge      CheckKey = "probe_token_self_knowledge"
	CheckProbeCodeReverseList         CheckKey = "probe_code_reverse_list"
	CheckProbeCodeCommentLang         CheckKey = "probe_code_comment_lang"
	CheckProbeCodeErrorStyle          CheckKey = "probe_code_error_style"
	CheckProbeMetaContextLen          CheckKey = "probe_meta_context_len"
	CheckProbeMetaThinkingMode        CheckKey = "probe_meta_thinking_mode"
	CheckProbeMetaCreator             CheckKey = "probe_meta_creator"
	CheckProbeLingUKPM                CheckKey = "probe_ling_uk_pm"
	CheckProbeLingKRCrisis            CheckKey = "probe_ling_kr_crisis"
	CheckProbeLingDEChan              CheckKey = "probe_ling_de_chan"
	CheckProbeCompPyFloat             CheckKey = "probe_comp_py_float"
	CheckProbeCompLargeExp            CheckKey = "probe_comp_large_exp"
	CheckProbeTowerHanoi              CheckKey = "probe_tower_hanoi"
	CheckProbeReverseWords            CheckKey = "probe_reverse_words"
	CheckProbePhotosynthesis          CheckKey = "probe_photosynthesis"
	CheckProbePerfBulkEcho            CheckKey = "probe_perf_bulk_echo"
	CheckProbeTokenZWJ                CheckKey = "probe_token_zwj"
	CheckProbeSubmodelCutoff          CheckKey = "probe_submodel_cutoff"
	CheckProbeSubmodelCapability      CheckKey = "probe_submodel_capability"
	CheckProbeSubmodelRefusal         CheckKey = "probe_submodel_refusal"
	CheckProbePIFingerprint           CheckKey = "probe_pi_fingerprint"
	CheckProbeRefusalL1               CheckKey = "probe_refusal_l1"
	CheckProbeRefusalL2               CheckKey = "probe_refusal_l2"
	CheckProbeRefusalL3               CheckKey = "probe_refusal_l3"
	CheckProbeRefusalL4               CheckKey = "probe_refusal_l4"
	CheckProbeRefusalL5               CheckKey = "probe_refusal_l5"
	CheckProbeRefusalL6               CheckKey = "probe_refusal_l6"
	CheckProbeRefusalL7               CheckKey = "probe_refusal_l7"
	CheckProbeRefusalL8               CheckKey = "probe_refusal_l8"
	CheckProbeFmtBullets              CheckKey = "probe_format_bullets"
	CheckProbeFmtExplainDepth         CheckKey = "probe_format_explain_depth"
	CheckProbeFmtCodeLangTag          CheckKey = "probe_format_code_lang_tag"
	CheckProbeUncertaintyEstimate     CheckKey = "probe_uncertainty_estimate"

	CheckCanaryMathMul        CheckKey = "canary_math_mul"
	CheckCanaryMathPow        CheckKey = "canary_math_pow"
	CheckCanaryMathMod        CheckKey = "canary_math_mod"
	CheckCanaryLogicSyllogism CheckKey = "canary_logic_syllogism"
	CheckCanaryRecallCapital  CheckKey = "canary_recall_capital"
	CheckCanaryRecallSymbol   CheckKey = "canary_recall_symbol"
	CheckCanaryFormatEcho     CheckKey = "canary_format_echo"
	CheckCanaryFormatJSON     CheckKey = "canary_format_json"
	CheckCanaryCodeReverse    CheckKey = "canary_code_reverse"
	CheckCanaryRecallMoonYear CheckKey = "canary_recall_moon_year"
)

type CheckResult struct {
	Provider            string         `json:"provider"`
	Group               string         `json:"group,omitempty"`
	CheckKey            CheckKey       `json:"check_key"`
	ModelName           string         `json:"model_name,omitempty"`
	Neutral             bool           `json:"neutral,omitempty"`
	Skipped             bool           `json:"skipped,omitempty"`
	Success             bool           `json:"success"`
	Score               int            `json:"score"`
	LatencyMs           int64          `json:"latency_ms,omitempty"`
	TTFTMs              int64          `json:"ttft_ms,omitempty"`
	TokensPS            float64        `json:"tokens_ps,omitempty"`
	ErrorCode           string         `json:"error_code,omitempty"`
	Message             string         `json:"message,omitempty"`
	Raw                 map[string]any `json:"raw,omitempty"`
	PrivateResponseText string         `json:"-"`
}

type ChecklistItem struct {
	Provider  string  `json:"provider"`
	Group     string  `json:"group,omitempty"`
	CheckKey  string  `json:"check_key"`
	CheckName string  `json:"check_name"`
	ModelName string  `json:"model_name,omitempty"`
	Neutral   bool    `json:"neutral,omitempty"`
	Skipped   bool    `json:"skipped,omitempty"`
	Passed    bool    `json:"passed"`
	Status    string  `json:"status"`
	Score     int     `json:"score"`
	LatencyMs int64   `json:"latency_ms,omitempty"`
	TTFTMs    int64   `json:"ttft_ms,omitempty"`
	TokensPS  float64 `json:"tokens_ps,omitempty"`
	ErrorCode string  `json:"error_code,omitempty"`
	Message   string  `json:"message,omitempty"`
}

type IdentityCandidateSummary struct {
	Family  string   `json:"family"`
	Model   string   `json:"model,omitempty"`
	Score   float64  `json:"score"`
	Reasons []string `json:"reasons,omitempty"`
}

type SubmodelAssessmentSummary struct {
	Method      string                     `json:"method"`
	Family      string                     `json:"family,omitempty"`
	ModelID     string                     `json:"model_id,omitempty"`
	DisplayName string                     `json:"display_name,omitempty"`
	Score       float64                    `json:"score"`
	Abstained   bool                       `json:"abstained,omitempty"`
	Candidates  []IdentityCandidateSummary `json:"candidates,omitempty"`
	Evidence    []string                   `json:"evidence,omitempty"`
}

type IdentityVerdictSummary struct {
	Status         string   `json:"status"`
	TrueFamily     string   `json:"true_family,omitempty"`
	TrueModel      string   `json:"true_model,omitempty"`
	SpoofMethod    string   `json:"spoof_method,omitempty"`
	ConfidenceBand string   `json:"confidence_band"`
	Reasoning      []string `json:"reasoning,omitempty"`
}

type IdentityAssessmentSummary struct {
	Provider            string                      `json:"provider"`
	ModelName           string                      `json:"model_name"`
	ClaimedModel        string                      `json:"claimed_model,omitempty"`
	Status              string                      `json:"status"`
	Confidence          float64                     `json:"confidence"`
	PredictedFamily     string                      `json:"predicted_family,omitempty"`
	PredictedModel      string                      `json:"predicted_model,omitempty"`
	Method              string                      `json:"method,omitempty"`
	Candidates          []IdentityCandidateSummary  `json:"candidates,omitempty"`
	SubmodelAssessments []SubmodelAssessmentSummary `json:"submodel_assessments,omitempty"`
	Verdict             *IdentityVerdictSummary     `json:"verdict,omitempty"`
	RiskFlags           []string                    `json:"risk_flags,omitempty"`
	Evidence            []string                    `json:"evidence,omitempty"`
}

type FinalRating struct {
	Score         int      `json:"score"`
	Grade         string   `json:"grade"`
	Conclusion    string   `json:"conclusion"`
	Risks         []string `json:"risks"`
	ProbeScore    int      `json:"probe_score"`
	ProbeScoreMax int      `json:"probe_score_max"`
}

type ReportSummary struct {
	Score               int                         `json:"score"`
	Grade               string                      `json:"grade"`
	Conclusion          string                      `json:"conclusion"`
	Checklist           []ChecklistItem             `json:"checklist"`
	IdentityAssessments []IdentityAssessmentSummary `json:"identity_assessments,omitempty"`
	Risks               []string                    `json:"risks"`
	FinalRating         FinalRating                 `json:"final_rating"`
	ScoringVersion      string                      `json:"scoring_version"`
	ProbeScore          int                         `json:"probe_score"`
	ProbeScoreMax       int                         `json:"probe_score_max"`
}

type DirectProbeRequest struct {
	BaseURL      string
	APIKey       string
	Provider     string
	Model        string
	ProbeProfile string
}

type DirectProbeResponse struct {
	BaseURL      string        `json:"base_url"`
	Provider     string        `json:"provider"`
	Model        string        `json:"model"`
	ProbeProfile string        `json:"probe_profile"`
	Results      []CheckResult `json:"results"`
	Report       ReportSummary `json:"report"`
}
