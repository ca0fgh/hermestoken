package service

import (
	"sort"
	"strings"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/dto"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/setting/billing_setting"
	"github.com/ca0fgh/hermestoken/setting/ratio_setting"
)

type MarketplacePricePreview struct {
	Model      string                `json:"model"`
	Official   MarketplacePricePoint `json:"official"`
	Buyer      MarketplacePricePoint `json:"buyer"`
	Multiplier float64               `json:"multiplier"`
}

type MarketplacePricePoint struct {
	QuotaType              string   `json:"quota_type"`
	BillingMode            string   `json:"billing_mode,omitempty"`
	BillingExpr            string   `json:"billing_expr,omitempty"`
	ModelPrice             float64  `json:"model_price,omitempty"`
	ModelRatio             float64  `json:"model_ratio,omitempty"`
	CompletionRatio        float64  `json:"completion_ratio,omitempty"`
	CacheRatio             *float64 `json:"cache_ratio,omitempty"`
	CreateCacheRatio       *float64 `json:"create_cache_ratio,omitempty"`
	InputPricePerMTok      float64  `json:"input_price_per_mtok,omitempty"`
	OutputPricePerMTok     float64  `json:"output_price_per_mtok,omitempty"`
	CacheReadPricePerMTok  *float64 `json:"cache_read_price_per_mtok,omitempty"`
	CacheWritePricePerMTok *float64 `json:"cache_write_price_per_mtok,omitempty"`
	TaskPerRequestPrice    float64  `json:"task_per_request_price,omitempty"`
	TaskPerSecondPrice     float64  `json:"task_per_second_price,omitempty"`
	AppliedMultiplier      float64  `json:"applied_multiplier,omitempty"`
	Configured             bool     `json:"configured"`
}

type MarketplacePricingItem struct {
	ModelName              string   `json:"model_name"`
	QuotaType              string   `json:"quota_type"`
	BillingMode            string   `json:"billing_mode,omitempty"`
	BillingExpr            string   `json:"billing_expr,omitempty"`
	ModelPrice             float64  `json:"model_price,omitempty"`
	ModelRatio             float64  `json:"model_ratio,omitempty"`
	CompletionRatio        float64  `json:"completion_ratio,omitempty"`
	CacheRatio             *float64 `json:"cache_ratio,omitempty"`
	CreateCacheRatio       *float64 `json:"create_cache_ratio,omitempty"`
	InputPricePerMTok      float64  `json:"input_price_per_mtok,omitempty"`
	OutputPricePerMTok     float64  `json:"output_price_per_mtok,omitempty"`
	CacheReadPricePerMTok  *float64 `json:"cache_read_price_per_mtok,omitempty"`
	CacheWritePricePerMTok *float64 `json:"cache_write_price_per_mtok,omitempty"`
	TaskPerRequestPrice    float64  `json:"task_per_request_price,omitempty"`
	TaskPerSecondPrice     float64  `json:"task_per_second_price,omitempty"`
	Configured             bool     `json:"configured"`
}

type marketplaceOfficialPriceSnapshot struct {
	Model    string                `json:"model"`
	Official MarketplacePricePoint `json:"official"`
}

type marketplaceBuyerPriceSnapshot struct {
	Model      string                `json:"model"`
	Buyer      MarketplacePricePoint `json:"buyer"`
	Multiplier float64               `json:"multiplier"`
}

func marketplacePricePreviewForCredential(credential model.MarketplaceCredential) []MarketplacePricePreview {
	modelNames := strings.Split(credential.Models, ",")
	previews := make([]MarketplacePricePreview, 0, len(modelNames))
	for _, modelName := range modelNames {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			continue
		}
		official := marketplaceOfficialPricePoint(modelName)
		previews = append(previews, MarketplacePricePreview{
			Model:      modelName,
			Official:   official,
			Buyer:      marketplaceBuyerPricePoint(official, credential.Multiplier),
			Multiplier: credential.Multiplier,
		})
	}
	return previews
}

func marketplacePricePreviewForModel(credential model.MarketplaceCredential, modelName string) MarketplacePricePreview {
	official := marketplaceOfficialPricePoint(modelName)
	return MarketplacePricePreview{
		Model:      modelName,
		Official:   official,
		Buyer:      marketplaceBuyerPricePoint(official, credential.Multiplier),
		Multiplier: credential.Multiplier,
	}
}

func ListMarketplacePricingItems() []MarketplacePricingItem {
	modelNames := make(map[string]struct{})
	pricingByModel := make(map[string]model.Pricing)
	addModelName := func(modelName string) {
		modelName = strings.TrimSpace(modelName)
		if modelName != "" {
			modelNames[modelName] = struct{}{}
		}
	}

	for modelName := range ratio_setting.GetModelPriceCopy() {
		addModelName(modelName)
	}
	for modelName := range ratio_setting.GetModelRatioCopy() {
		addModelName(modelName)
	}
	for modelName := range ratio_setting.GetTaskModelPricingCopy() {
		addModelName(modelName)
	}
	for modelName := range ratio_setting.GetCompletionRatioCopy() {
		addModelName(modelName)
	}
	for modelName := range ratio_setting.GetCacheRatioCopy() {
		addModelName(modelName)
	}
	for modelName := range ratio_setting.GetCreateCacheRatioCopy() {
		addModelName(modelName)
	}
	for modelName := range ratio_setting.GetImageRatioCopy() {
		addModelName(modelName)
	}
	for modelName := range ratio_setting.GetAudioRatioCopy() {
		addModelName(modelName)
	}
	for modelName := range ratio_setting.GetAudioCompletionRatioCopy() {
		addModelName(modelName)
	}
	billingExprs := billing_setting.GetBillingExprCopy()
	for modelName, mode := range billing_setting.GetBillingModeCopy() {
		if mode == billing_setting.BillingModeTieredExpr && strings.TrimSpace(billingExprs[modelName]) != "" {
			addModelName(modelName)
		}
	}
	if model.DB != nil {
		for _, modelName := range model.GetEnabledModels() {
			addModelName(modelName)
		}
		for _, pricing := range model.GetPricing() {
			addModelName(pricing.ModelName)
			pricingByModel[pricing.ModelName] = pricing
		}
	}

	names := make([]string, 0, len(modelNames))
	for modelName := range modelNames {
		names = append(names, modelName)
	}
	sort.Strings(names)

	items := make([]MarketplacePricingItem, 0, len(names))
	for _, modelName := range names {
		point := marketplaceOfficialPricePoint(modelName)
		if !point.Configured {
			if pricing, ok := pricingByModel[modelName]; ok {
				items = append(items, marketplacePricingItemFromCatalogPricing(pricing))
				continue
			}
		}
		items = append(items, marketplacePricingItemFromPoint(modelName, point))
	}
	return items
}

func ListPricedOpenAIModels(openAIModelsByID map[string]dto.OpenAIModels) []dto.OpenAIModels {
	modelNames := make(map[string]struct{})
	addModelName := func(modelName string) {
		modelName = strings.TrimSpace(modelName)
		if modelName != "" {
			modelNames[modelName] = struct{}{}
		}
	}

	for modelName := range ratio_setting.GetModelPriceCopy() {
		addModelName(modelName)
	}
	for modelName := range ratio_setting.GetModelRatioCopy() {
		addModelName(modelName)
	}
	for modelName, pricing := range ratio_setting.GetTaskModelPricingCopy() {
		if pricing.Enabled() {
			addModelName(modelName)
		}
	}
	for modelName := range ratio_setting.GetCompletionRatioCopy() {
		addModelName(modelName)
	}
	for modelName := range ratio_setting.GetCacheRatioCopy() {
		addModelName(modelName)
	}
	for modelName := range ratio_setting.GetCreateCacheRatioCopy() {
		addModelName(modelName)
	}
	for modelName := range ratio_setting.GetImageRatioCopy() {
		addModelName(modelName)
	}
	for modelName := range ratio_setting.GetAudioRatioCopy() {
		addModelName(modelName)
	}
	for modelName := range ratio_setting.GetAudioCompletionRatioCopy() {
		addModelName(modelName)
	}
	billingExprs := billing_setting.GetBillingExprCopy()
	for modelName, mode := range billing_setting.GetBillingModeCopy() {
		if mode == billing_setting.BillingModeTieredExpr && strings.TrimSpace(billingExprs[modelName]) != "" {
			addModelName(modelName)
		}
	}
	if model.DB != nil {
		for _, pricing := range model.GetPricing() {
			addModelName(pricing.ModelName)
		}
	}

	modelList := make([]string, 0, len(modelNames))
	for modelName := range modelNames {
		modelList = append(modelList, modelName)
	}
	sort.Strings(modelList)

	pricedModels := make([]dto.OpenAIModels, 0, len(modelList))
	for _, modelName := range modelList {
		pricedModel, ok := openAIModelsByID[modelName]
		if !ok {
			pricedModel = dto.OpenAIModels{
				Id:      modelName,
				Object:  "model",
				Created: 1626777600,
				OwnedBy: "custom",
			}
		}
		pricedModel.SupportedEndpointTypes = model.GetModelSupportEndpointTypes(modelName)
		pricedModels = append(pricedModels, ApplyMarketplacePricingFields(pricedModel))
	}
	return pricedModels
}

func MarketplacePricingFieldsForModel(modelName string) MarketplacePricingItem {
	point := marketplaceOfficialPricePoint(modelName)
	if !point.Configured && model.DB != nil {
		for _, pricing := range model.GetPricing() {
			if pricing.ModelName == modelName {
				return marketplacePricingItemFromCatalogPricing(pricing)
			}
		}
	}
	return marketplacePricingItemFromPoint(modelName, point)
}

func ApplyMarketplacePricingFields(openAIModel dto.OpenAIModels) dto.OpenAIModels {
	pricing := MarketplacePricingFieldsForModel(openAIModel.Id)
	openAIModel.ModelName = pricing.ModelName
	openAIModel.QuotaType = pricing.QuotaType
	openAIModel.BillingMode = pricing.BillingMode
	openAIModel.BillingExpr = pricing.BillingExpr
	openAIModel.ModelPrice = pricing.ModelPrice
	openAIModel.ModelRatio = pricing.ModelRatio
	openAIModel.CompletionRatio = pricing.CompletionRatio
	openAIModel.CacheRatio = pricing.CacheRatio
	openAIModel.CreateCacheRatio = pricing.CreateCacheRatio
	openAIModel.InputPricePerMTok = pricing.InputPricePerMTok
	openAIModel.OutputPricePerMTok = pricing.OutputPricePerMTok
	openAIModel.CacheReadPricePerMTok = pricing.CacheReadPricePerMTok
	openAIModel.CacheWritePricePerMTok = pricing.CacheWritePricePerMTok
	openAIModel.TaskPerRequestPrice = pricing.TaskPerRequestPrice
	openAIModel.TaskPerSecondPrice = pricing.TaskPerSecondPrice
	openAIModel.Configured = pricing.Configured
	return openAIModel
}

func marketplaceOfficialPricePoint(modelName string) MarketplacePricePoint {
	if billing_setting.GetBillingMode(modelName) == billing_setting.BillingModeTieredExpr {
		if expr, ok := billing_setting.GetBillingExpr(modelName); ok && strings.TrimSpace(expr) != "" {
			return MarketplacePricePoint{
				QuotaType:   "tiered_expr",
				BillingMode: billing_setting.BillingModeTieredExpr,
				BillingExpr: expr,
				Configured:  true,
			}
		}
	}
	if taskPricing, ok := ratio_setting.GetTaskModelPricing(modelName); ok {
		return MarketplacePricePoint{
			QuotaType:           "per_second",
			BillingMode:         "per_second",
			TaskPerRequestPrice: taskPricing.PerRequest,
			TaskPerSecondPrice:  taskPricing.PerSecond,
			Configured:          true,
		}
	}
	if modelPrice, ok := ratio_setting.GetModelPrice(modelName, false); ok {
		return MarketplacePricePoint{
			QuotaType:   "price",
			BillingMode: "per_request",
			ModelPrice:  modelPrice,
			Configured:  true,
		}
	}
	modelRatio, ok, _ := ratio_setting.GetModelRatio(modelName)
	point := MarketplacePricePoint{
		QuotaType:   "ratio",
		BillingMode: "metered",
		ModelRatio:  modelRatio,
		Configured:  ok,
	}
	if ok {
		enrichMarketplaceMeteredPricePoint(modelName, &point)
	}
	return point
}

func marketplaceBuyerPricePoint(official MarketplacePricePoint, multiplier float64) MarketplacePricePoint {
	buyer := official
	buyer.AppliedMultiplier = multiplier
	buyer.ModelPrice = official.ModelPrice * multiplier
	buyer.ModelRatio = official.ModelRatio * multiplier
	buyer.InputPricePerMTok = official.InputPricePerMTok * multiplier
	buyer.OutputPricePerMTok = official.OutputPricePerMTok * multiplier
	buyer.CacheReadPricePerMTok = multiplyFloat64Pointer(official.CacheReadPricePerMTok, multiplier)
	buyer.CacheWritePricePerMTok = multiplyFloat64Pointer(official.CacheWritePricePerMTok, multiplier)
	buyer.TaskPerRequestPrice = official.TaskPerRequestPrice * multiplier
	buyer.TaskPerSecondPrice = official.TaskPerSecondPrice * multiplier
	return buyer
}

func marketplacePricingItemFromPoint(modelName string, point MarketplacePricePoint) MarketplacePricingItem {
	return MarketplacePricingItem{
		ModelName:              modelName,
		QuotaType:              point.QuotaType,
		BillingMode:            point.BillingMode,
		BillingExpr:            point.BillingExpr,
		ModelPrice:             point.ModelPrice,
		ModelRatio:             point.ModelRatio,
		CompletionRatio:        point.CompletionRatio,
		CacheRatio:             point.CacheRatio,
		CreateCacheRatio:       point.CreateCacheRatio,
		InputPricePerMTok:      point.InputPricePerMTok,
		OutputPricePerMTok:     point.OutputPricePerMTok,
		CacheReadPricePerMTok:  point.CacheReadPricePerMTok,
		CacheWritePricePerMTok: point.CacheWritePricePerMTok,
		TaskPerRequestPrice:    point.TaskPerRequestPrice,
		TaskPerSecondPrice:     point.TaskPerSecondPrice,
		Configured:             point.Configured,
	}
}

func marketplacePricingItemFromCatalogPricing(pricing model.Pricing) MarketplacePricingItem {
	if pricing.BillingMode == billing_setting.BillingModeTieredExpr && strings.TrimSpace(pricing.BillingExpr) != "" {
		return marketplacePricingItemFromPoint(pricing.ModelName, MarketplacePricePoint{
			QuotaType:   "tiered_expr",
			BillingMode: billing_setting.BillingModeTieredExpr,
			BillingExpr: pricing.BillingExpr,
			Configured:  true,
		})
	}
	if pricing.QuotaType == 1 {
		return marketplacePricingItemFromPoint(pricing.ModelName, MarketplacePricePoint{
			QuotaType:   "price",
			BillingMode: "per_request",
			ModelPrice:  pricing.ModelPrice,
			Configured:  true,
		})
	}

	point := MarketplacePricePoint{
		QuotaType:         "ratio",
		BillingMode:       "metered",
		ModelRatio:        pricing.ModelRatio,
		CompletionRatio:   pricing.CompletionRatio,
		InputPricePerMTok: marketplaceRatioToUSDPerMTok(pricing.ModelRatio),
		Configured:        true,
	}
	point.OutputPricePerMTok = point.InputPricePerMTok * point.CompletionRatio
	if pricing.CacheRatio != nil {
		point.CacheRatio = pricing.CacheRatio
		cacheReadPrice := point.InputPricePerMTok * *pricing.CacheRatio
		point.CacheReadPricePerMTok = &cacheReadPrice
	}
	if pricing.CreateCacheRatio != nil {
		point.CreateCacheRatio = pricing.CreateCacheRatio
		cacheWritePrice := point.InputPricePerMTok * *pricing.CreateCacheRatio
		point.CacheWritePricePerMTok = &cacheWritePrice
	}
	return marketplacePricingItemFromPoint(pricing.ModelName, point)
}

func enrichMarketplaceMeteredPricePoint(modelName string, point *MarketplacePricePoint) {
	point.CompletionRatio = ratio_setting.GetCompletionRatio(modelName)
	point.InputPricePerMTok = marketplaceRatioToUSDPerMTok(point.ModelRatio)
	point.OutputPricePerMTok = point.InputPricePerMTok * point.CompletionRatio

	if cacheRatio, ok := ratio_setting.GetCacheRatio(modelName); ok {
		point.CacheRatio = &cacheRatio
		cacheReadPrice := point.InputPricePerMTok * cacheRatio
		point.CacheReadPricePerMTok = &cacheReadPrice
	}
	if createCacheRatio, ok := ratio_setting.GetCreateCacheRatio(modelName); ok {
		point.CreateCacheRatio = &createCacheRatio
		cacheWritePrice := point.InputPricePerMTok * createCacheRatio
		point.CacheWritePricePerMTok = &cacheWritePrice
	}
}

func marketplaceRatioToUSDPerMTok(ratio float64) float64 {
	return ratio * 1_000_000 / common.QuotaPerUnit
}

func multiplyFloat64Pointer(value *float64, multiplier float64) *float64 {
	if value == nil {
		return nil
	}
	result := *value * multiplier
	return &result
}

func marshalMarketplaceOfficialPriceSnapshot(previews []MarketplacePricePreview) string {
	snapshots := make([]marketplaceOfficialPriceSnapshot, 0, len(previews))
	for _, preview := range previews {
		snapshots = append(snapshots, marketplaceOfficialPriceSnapshot{
			Model:    preview.Model,
			Official: preview.Official,
		})
	}
	return marshalMarketplacePriceSnapshot(snapshots)
}

func marshalMarketplaceBuyerPriceSnapshot(previews []MarketplacePricePreview) string {
	snapshots := make([]marketplaceBuyerPriceSnapshot, 0, len(previews))
	for _, preview := range previews {
		snapshots = append(snapshots, marketplaceBuyerPriceSnapshot{
			Model:      preview.Model,
			Buyer:      preview.Buyer,
			Multiplier: preview.Multiplier,
		})
	}
	return marshalMarketplacePriceSnapshot(snapshots)
}

func marshalMarketplacePriceSnapshot(value any) string {
	payload, err := common.Marshal(value)
	if err != nil {
		return "[]"
	}
	return string(payload)
}
