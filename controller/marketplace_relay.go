package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/constant"
	"github.com/ca0fgh/hermestoken/dto"
	"github.com/ca0fgh/hermestoken/logger"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/pkg/billingexpr"
	relaycommon "github.com/ca0fgh/hermestoken/relay/common"
	"github.com/ca0fgh/hermestoken/relay/helper"
	"github.com/ca0fgh/hermestoken/service"
	"github.com/ca0fgh/hermestoken/setting"
	"github.com/ca0fgh/hermestoken/types"
	"github.com/gin-gonic/gin"
)

const marketplaceFixedOrderHeader = "X-Marketplace-Fixed-Order-Id"

func MarketplaceUnifiedRelay(c *gin.Context, relayFormat types.RelayFormat) {
	requestID := c.GetString(common.RequestIdKey)
	request, err := helper.GetAndValidateRequest(c, relayFormat)
	if err != nil {
		if common.IsRequestBodyTooLargeError(err) || errors.Is(err, common.ErrRequestBodyTooLarge) {
			abortMarketplaceUnifiedRelay(c, requestID, types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry()))
		} else {
			abortMarketplaceUnifiedRelay(c, requestID, types.NewError(err, types.ErrorCodeInvalidRequest))
		}
		return
	}
	modelName := marketplaceRelayRequestModelName(request)
	if modelName == "" {
		abortMarketplaceUnifiedRelay(c, requestID, types.NewError(errors.New("model is required"), types.ErrorCodeInvalidRequest, types.ErrOptionWithStatusCode(http.StatusBadRequest)))
		return
	}
	if !resetMarketplaceRelayBody(c, requestID) {
		return
	}

	if strings.TrimSpace(c.GetHeader(marketplaceFixedOrderHeader)) != "" {
		if !marketplaceUnifiedRouteEnabled(c, model.MarketplaceRouteFixedOrder) {
			abortMarketplaceUnifiedRelay(c, requestID, types.NewError(errors.New("marketplace fixed order route is disabled for this token"), types.ErrorCodeInvalidRequest, types.ErrOptionWithStatusCode(http.StatusForbidden), types.ErrOptionWithSkipRetry()))
			return
		}
		if marketplaceUnifiedFixedOrderAvailable(c, modelName, requestID) {
			MarketplaceFixedOrderRelay(c, relayFormat)
		}
		return
	}

	for _, route := range marketplaceUnifiedRouteOrder(c) {
		switch route {
		case model.MarketplaceRouteFixedOrder:
			if marketplaceUnifiedFixedOrderAvailable(c, modelName, requestID) {
				MarketplaceFixedOrderRelay(c, relayFormat)
				return
			}
			if c.Writer.Written() {
				return
			}
		case model.MarketplaceRouteGroup:
			continue
		case model.MarketplaceRoutePool:
			MarketplacePoolRelay(c, relayFormat)
			return
		}
	}

	abortMarketplaceUnifiedRelay(c, requestID, types.NewError(errors.New("no enabled token route is available for this request"), types.ErrorCodeInvalidRequest, types.ErrOptionWithStatusCode(http.StatusForbidden), types.ErrOptionWithSkipRetry()))
}

func marketplaceUnifiedRouteOrder(c *gin.Context) []string {
	routes := common.GetContextKeyStringSlice(c, constant.ContextKeyMarketplaceRouteOrder)
	if len(routes) == 0 {
		routes = model.DefaultMarketplaceRouteOrderList()
	} else {
		routes = model.NormalizeMarketplaceRouteOrderList(routes)
	}
	enabled := marketplaceUnifiedRouteEnabledList(c)
	if len(enabled) == 0 {
		return []string{}
	}
	enabledMap := model.MarketplaceRouteEnabledMap(enabled)
	filtered := make([]string, 0, len(routes))
	for _, route := range routes {
		if enabledMap[route] {
			filtered = append(filtered, route)
		}
	}
	return filtered
}

func marketplaceUnifiedRouteEnabled(c *gin.Context, route string) bool {
	return model.MarketplaceRouteEnabledMap(marketplaceUnifiedRouteEnabledList(c))[route]
}

func marketplaceUnifiedRouteEnabledList(c *gin.Context) []string {
	value, ok := common.GetContextKey(c, constant.ContextKeyMarketplaceRouteEnabled)
	if !ok {
		return model.DefaultMarketplaceRouteOrderList()
	}
	routes, ok := value.([]string)
	if !ok {
		return model.DefaultMarketplaceRouteOrderList()
	}
	return model.NormalizeMarketplaceRouteEnabledList(routes)
}

func marketplaceUnifiedFixedOrderAvailable(c *gin.Context, modelName string, requestID string) bool {
	fixedOrderIDs, headerSet, err := marketplaceFixedOrderIDsFromRequest(c)
	if headerSet {
		if err != nil {
			abortMarketplaceUnifiedRelay(c, requestID, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithStatusCode(http.StatusBadRequest), types.ErrOptionWithSkipRetry()))
			return false
		}
		return true
	}
	if err != nil || len(fixedOrderIDs) == 0 {
		return false
	}
	order, selectErr := service.SelectBuyerMarketplaceFixedOrderForTokenBindings(service.MarketplaceFixedOrderBindingSelectInput{
		BuyerUserID:   c.GetInt("id"),
		FixedOrderIDs: fixedOrderIDs,
		Model:         modelName,
	})
	return selectErr == nil && order != nil
}

func abortMarketplaceUnifiedRelay(c *gin.Context, requestID string, hermesTokenError *types.HermesTokenError) {
	if hermesTokenError == nil {
		return
	}
	logger.LogError(c, fmt.Sprintf("marketplace unified relay error: %s", hermesTokenError.Error()))
	hermesTokenError.SetMessage(common.MessageWithRequestId(hermesTokenError.Error(), requestID))
	c.JSON(hermesTokenError.StatusCode, gin.H{
		"error": hermesTokenError.ToOpenAIError(),
	})
}

func resetMarketplaceRelayBody(c *gin.Context, requestID string) bool {
	bodyStorage, err := common.GetBodyStorage(c)
	if err == nil {
		c.Request.Body = io.NopCloser(bodyStorage)
		return true
	}
	if common.IsRequestBodyTooLargeError(err) || errors.Is(err, common.ErrRequestBodyTooLarge) {
		abortMarketplaceUnifiedRelay(c, requestID, types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry()))
	} else {
		abortMarketplaceUnifiedRelay(c, requestID, types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry()))
	}
	return false
}

func MarketplaceFixedOrderRelay(c *gin.Context, relayFormat types.RelayFormat) {
	requestID := c.GetString(common.RequestIdKey)
	var hermesTokenError *types.HermesTokenError
	var relayInfo *relaycommon.RelayInfo

	defer func() {
		if hermesTokenError == nil {
			return
		}
		logger.LogError(c, fmt.Sprintf("marketplace fixed order relay error: %s", hermesTokenError.Error()))
		hermesTokenError = service.NormalizeViolationFeeError(hermesTokenError)
		if relayInfo != nil && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
		hermesTokenError.SetMessage(common.MessageWithRequestId(hermesTokenError.Error(), requestID))
		c.JSON(hermesTokenError.StatusCode, gin.H{
			"error": hermesTokenError.ToOpenAIError(),
		})
	}()

	if !marketplaceUnifiedRouteEnabled(c, model.MarketplaceRouteFixedOrder) {
		hermesTokenError = types.NewError(errors.New("marketplace fixed order route is disabled for this token"), types.ErrorCodeInvalidRequest, types.ErrOptionWithStatusCode(http.StatusForbidden), types.ErrOptionWithSkipRetry())
		return
	}

	originalPath := c.Request.URL.Path
	normalizeMarketplaceRelayPath(c)

	request, err := helper.GetAndValidateRequest(c, relayFormat)
	if err != nil {
		if common.IsRequestBodyTooLargeError(err) || errors.Is(err, common.ErrRequestBodyTooLarge) {
			hermesTokenError = types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
		} else {
			hermesTokenError = types.NewError(err, types.ErrorCodeInvalidRequest)
		}
		return
	}

	modelName := marketplaceRelayRequestModelName(request)
	if modelName == "" {
		hermesTokenError = types.NewError(errors.New("model is required"), types.ErrorCodeInvalidRequest, types.ErrOptionWithStatusCode(http.StatusBadRequest))
		return
	}
	common.SetContextKey(c, constant.ContextKeyOriginalModel, modelName)

	relayInfo, err = relaycommon.GenRelayInfo(c, relayFormat, request, nil)
	if err != nil {
		hermesTokenError = types.NewError(err, types.ErrorCodeGenRelayInfoFailed)
		return
	}
	relayInfo.RequestURLPath = strings.TrimPrefix(relayInfo.RequestURLPath, "/marketplace")
	if relayInfo.RequestURLPath == "" {
		relayInfo.RequestURLPath = c.Request.URL.String()
	}

	needSensitiveCheck := setting.ShouldCheckPromptSensitive()
	needCountToken := constant.CountToken
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
			hermesTokenError = types.NewError(errors.New("sensitive words detected"), types.ErrorCodeSensitiveWordsDetected)
			return
		}
	}
	tokens, err := service.EstimateRequestToken(c, meta, relayInfo)
	if err != nil {
		hermesTokenError = types.NewError(err, types.ErrorCodeCountTokenFailed)
		return
	}
	relayInfo.SetEstimatePromptTokens(tokens)

	fixedOrderIDs, headerSet, err := marketplaceFixedOrderIDsFromRequest(c)
	if err != nil {
		hermesTokenError = types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithStatusCode(http.StatusBadRequest), types.ErrOptionWithSkipRetry())
		return
	}
	var order *model.MarketplaceFixedOrder
	if headerSet {
		order, err = service.GetBuyerMarketplaceFixedOrder(c.GetInt("id"), fixedOrderIDs[0])
		if err != nil {
			hermesTokenError = types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithStatusCode(http.StatusBadRequest), types.ErrOptionWithSkipRetry())
			return
		}
	} else {
		order, err = service.SelectBuyerMarketplaceFixedOrderForTokenBindings(service.MarketplaceFixedOrderBindingSelectInput{
			BuyerUserID:   c.GetInt("id"),
			FixedOrderIDs: fixedOrderIDs,
			Model:         modelName,
		})
		if err != nil {
			hermesTokenError = types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithStatusCode(http.StatusBadRequest), types.ErrOptionWithSkipRetry())
			return
		}
	}

	priceData, err := helper.ModelPriceHelper(c, relayInfo, tokens, meta)
	if err != nil {
		hermesTokenError = types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithStatusCode(http.StatusBadRequest))
		return
	}
	applyMarketplaceMultiplier(&priceData, relayInfo, tokens, meta, order.MultiplierSnapshot)

	preparation, err := service.PrepareMarketplaceFixedOrderRelay(service.MarketplaceFixedOrderRelayInput{
		BuyerUserID:    c.GetInt("id"),
		FixedOrderID:   order.ID,
		Model:          modelName,
		EstimatedQuota: priceData.QuotaToPreConsume,
		RequestID:      relayInfo.RequestId,
	})
	if err != nil {
		hermesTokenError = types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithStatusCode(http.StatusBadRequest), types.ErrOptionWithSkipRetry())
		return
	}

	setupMarketplaceFixedOrderRelayChannelContext(c, preparation)
	relayInfo.Billing = preparation.Session
	relayInfo.PriceData = priceData
	relayInfo.BillingSource = "marketplace_fixed_order"
	c.Set("marketplace_relay", true)
	c.Set("marketplace_fixed_order_id", order.ID)
	c.Set("marketplace_original_path", originalPath)

	bodyStorage, bodyErr := common.GetBodyStorage(c)
	if bodyErr != nil {
		if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
			hermesTokenError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
		} else {
			hermesTokenError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		return
	}
	c.Request.Body = io.NopCloser(bodyStorage)

	hermesTokenError = relayHandler(c, relayInfo)
	if hermesTokenError == nil && relayInfo.Billing != nil && relayInfo.Billing.NeedsRefund() {
		relayInfo.Billing.Refund(c)
	}
}

func normalizeMarketplaceRelayPath(c *gin.Context) {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return
	}
	if strings.HasPrefix(c.Request.URL.Path, "/marketplace") {
		c.Request.URL.Path = strings.TrimPrefix(c.Request.URL.Path, "/marketplace")
	}
}

func marketplaceFixedOrderIDFromHeader(c *gin.Context) (int, error) {
	raw := strings.TrimSpace(c.GetHeader(marketplaceFixedOrderHeader))
	if raw == "" {
		return 0, fmt.Errorf("%s is required", marketplaceFixedOrderHeader)
	}
	id, err := strconv.Atoi(raw)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", marketplaceFixedOrderHeader)
	}
	return id, nil
}

func marketplaceFixedOrderIDsFromRequest(c *gin.Context) ([]int, bool, error) {
	raw := strings.TrimSpace(c.GetHeader(marketplaceFixedOrderHeader))
	if raw != "" {
		id, err := marketplaceFixedOrderIDFromHeader(c)
		if err != nil {
			return nil, true, err
		}
		return []int{id}, true, nil
	}
	if value, ok := c.Get("token_marketplace_fixed_order_ids"); ok {
		if ids, ok := value.([]int); ok && len(ids) > 0 {
			return ids, false, nil
		}
	}
	if id := c.GetInt("token_marketplace_fixed_order_id"); id > 0 {
		return []int{id}, false, nil
	}
	return nil, false, fmt.Errorf("%s is required or bind a marketplace fixed order to the token", marketplaceFixedOrderHeader)
}

func marketplaceRelayRequestModelName(request dto.Request) string {
	switch r := request.(type) {
	case *dto.GeneralOpenAIRequest:
		return strings.TrimSpace(r.Model)
	case *dto.OpenAIResponsesRequest:
		return strings.TrimSpace(r.Model)
	case *dto.OpenAIResponsesCompactionRequest:
		return strings.TrimSpace(r.Model)
	case *dto.EmbeddingRequest:
		return strings.TrimSpace(r.Model)
	case *dto.ImageRequest:
		return strings.TrimSpace(r.Model)
	case *dto.AudioRequest:
		return strings.TrimSpace(r.Model)
	case *dto.RerankRequest:
		return strings.TrimSpace(r.Model)
	default:
		return ""
	}
}

func applyMarketplaceMultiplier(priceData *types.PriceData, relayInfo *relaycommon.RelayInfo, promptTokens int, meta *types.TokenCountMeta, multiplier float64) {
	if priceData == nil {
		return
	}
	if multiplier < 0 {
		multiplier = 0
	}
	priceData.GroupRatioInfo = types.GroupRatioInfo{
		GroupRatio:        multiplier,
		GroupSpecialRatio: multiplier,
		HasSpecialRatio:   true,
	}
	if relayInfo != nil && relayInfo.TieredBillingSnapshot != nil {
		snapshot := relayInfo.TieredBillingSnapshot
		preConsumedQuota := billingexpr.QuotaRound(snapshot.EstimatedQuotaBeforeGroup * multiplier)
		snapshot.GroupRatio = multiplier
		snapshot.EstimatedQuotaAfterGroup = preConsumedQuota
		priceData.QuotaToPreConsume = preConsumedQuota
		return
	}
	if priceData.FreeModel {
		priceData.QuotaToPreConsume = 0
		return
	}
	if priceData.UsePrice {
		priceData.QuotaToPreConsume = int(priceData.ModelPrice * common.QuotaPerUnit * multiplier)
		return
	}
	preConsumedTokens := common.Max(promptTokens, common.PreConsumedQuota)
	if meta != nil && meta.MaxTokens != 0 {
		preConsumedTokens += meta.MaxTokens
	}
	priceData.QuotaToPreConsume = int(float64(preConsumedTokens) * priceData.ModelRatio * multiplier)
}

func setupMarketplaceFixedOrderRelayChannelContext(c *gin.Context, preparation *service.MarketplaceFixedOrderRelayPreparation) {
	credential := preparation.Credential
	setupMarketplaceRelayChannelContext(c, credential, preparation.APIKey)
}

func MarketplacePoolRelay(c *gin.Context, relayFormat types.RelayFormat) {
	requestID := c.GetString(common.RequestIdKey)
	var hermesTokenError *types.HermesTokenError
	var relayInfo *relaycommon.RelayInfo

	defer func() {
		if hermesTokenError == nil {
			return
		}
		logger.LogError(c, fmt.Sprintf("marketplace pool relay error: %s", hermesTokenError.Error()))
		hermesTokenError = service.NormalizeViolationFeeError(hermesTokenError)
		if relayInfo != nil && relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
		hermesTokenError.SetMessage(common.MessageWithRequestId(hermesTokenError.Error(), requestID))
		c.JSON(hermesTokenError.StatusCode, gin.H{
			"error": hermesTokenError.ToOpenAIError(),
		})
	}()

	if !marketplaceUnifiedRouteEnabled(c, model.MarketplaceRoutePool) {
		hermesTokenError = types.NewError(errors.New("marketplace pool route is disabled for this token"), types.ErrorCodeInvalidRequest, types.ErrOptionWithStatusCode(http.StatusForbidden), types.ErrOptionWithSkipRetry())
		return
	}

	originalPath := c.Request.URL.Path
	normalizeMarketplacePoolRelayPath(c)

	request, err := helper.GetAndValidateRequest(c, relayFormat)
	if err != nil {
		if common.IsRequestBodyTooLargeError(err) || errors.Is(err, common.ErrRequestBodyTooLarge) {
			hermesTokenError = types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
		} else {
			hermesTokenError = types.NewError(err, types.ErrorCodeInvalidRequest)
		}
		return
	}

	modelName := marketplaceRelayRequestModelName(request)
	if modelName == "" {
		hermesTokenError = types.NewError(errors.New("model is required"), types.ErrorCodeInvalidRequest, types.ErrOptionWithStatusCode(http.StatusBadRequest))
		return
	}
	common.SetContextKey(c, constant.ContextKeyOriginalModel, modelName)

	relayInfo, err = relaycommon.GenRelayInfo(c, relayFormat, request, nil)
	if err != nil {
		hermesTokenError = types.NewError(err, types.ErrorCodeGenRelayInfoFailed)
		return
	}
	relayInfo.RequestURLPath = strings.TrimPrefix(relayInfo.RequestURLPath, "/marketplace/pool")
	if relayInfo.RequestURLPath == "" {
		relayInfo.RequestURLPath = c.Request.URL.String()
	}

	needSensitiveCheck := setting.ShouldCheckPromptSensitive()
	needCountToken := constant.CountToken
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
			hermesTokenError = types.NewError(errors.New("sensitive words detected"), types.ErrorCodeSensitiveWordsDetected)
			return
		}
	}
	tokens, err := service.EstimateRequestToken(c, meta, relayInfo)
	if err != nil {
		hermesTokenError = types.NewError(err, types.ErrorCodeCountTokenFailed)
		return
	}
	relayInfo.SetEstimatePromptTokens(tokens)

	poolInput, err := marketplacePoolRelayInputFromRequest(c, c.GetInt("id"), modelName, relayInfo.RequestId)
	if err != nil {
		hermesTokenError = types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithStatusCode(http.StatusBadRequest), types.ErrOptionWithSkipRetry())
		return
	}
	preparation, err := service.PrepareMarketplacePoolRelay(poolInput)
	if err != nil {
		hermesTokenError = types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithStatusCode(http.StatusBadRequest), types.ErrOptionWithSkipRetry())
		return
	}
	setupMarketplacePoolRelayChannelContext(c, preparation)

	priceData, err := helper.ModelPriceHelper(c, relayInfo, tokens, meta)
	if err != nil {
		preparation.Session.Release(c)
		hermesTokenError = types.NewError(err, types.ErrorCodeModelPriceError, types.ErrOptionWithStatusCode(http.StatusBadRequest))
		return
	}
	applyMarketplaceMultiplier(&priceData, relayInfo, tokens, meta, preparation.Credential.Multiplier)

	if calculator := marketplacePoolBuyerChargeCalculator(preparation.Session); calculator != nil {
		priceData.QuotaToPreConsume = calculator.BuyerChargeForQuota(priceData.QuotaToPreConsume)
	}
	relayInfo.PriceData = priceData

	if priceData.FreeModel {
		relayInfo.Billing = service.NewMarketplacePoolBillingSession(nil, preparation.Session)
	} else {
		hermesTokenError = service.PreConsumeBilling(c, priceData.QuotaToPreConsume, relayInfo)
		if hermesTokenError != nil {
			preparation.Session.Release(c)
			return
		}
		relayInfo.Billing = service.NewMarketplacePoolBillingSession(relayInfo.Billing, preparation.Session)
	}
	relayInfo.BillingSource = "marketplace_pool"
	c.Set("marketplace_relay", true)
	c.Set("marketplace_pool_credential_id", preparation.Credential.ID)
	c.Set("marketplace_original_path", originalPath)

	bodyStorage, bodyErr := common.GetBodyStorage(c)
	if bodyErr != nil {
		if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
			hermesTokenError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
		} else {
			hermesTokenError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		return
	}
	c.Request.Body = io.NopCloser(bodyStorage)

	hermesTokenError = relayHandler(c, relayInfo)
	if hermesTokenError == nil && relayInfo.Billing != nil && relayInfo.Billing.NeedsRefund() {
		relayInfo.Billing.Refund(c)
	}
}

func normalizeMarketplacePoolRelayPath(c *gin.Context) {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return
	}
	if strings.HasPrefix(c.Request.URL.Path, "/marketplace/pool") {
		c.Request.URL.Path = strings.TrimPrefix(c.Request.URL.Path, "/marketplace/pool")
	}
}

func setupMarketplacePoolRelayChannelContext(c *gin.Context, preparation *service.MarketplacePoolRelayPreparation) {
	setupMarketplaceRelayChannelContext(c, preparation.Credential, preparation.APIKey)
}

func marketplacePoolBuyerChargeCalculator(session *service.MarketplacePoolRelaySession) relaycommon.BuyerChargeCalculator {
	if session == nil {
		return nil
	}
	return session
}

func marketplacePoolRelayInputFromRequest(c *gin.Context, buyerUserID int, modelName string, requestID string) (service.MarketplacePoolRelayInput, error) {
	input := service.MarketplacePoolRelayInput{
		BuyerUserID:         buyerUserID,
		VendorType:          marketplaceRelayIntQuery(c, "vendor_type"),
		Model:               strings.TrimSpace(modelName),
		QuotaMode:           c.Query("quota_mode"),
		TimeMode:            c.Query("time_mode"),
		MinQuotaLimit:       marketplaceRelayInt64Query(c, "min_quota_limit"),
		MaxQuotaLimit:       marketplaceRelayInt64Query(c, "max_quota_limit"),
		MinTimeLimitSeconds: marketplaceRelayInt64Query(c, "min_time_limit_seconds"),
		MaxTimeLimitSeconds: marketplaceRelayInt64Query(c, "max_time_limit_seconds"),
		MinMultiplier:       marketplaceRelayFloatQuery(c, "min_multiplier"),
		MaxMultiplier:       marketplaceRelayFloatQuery(c, "max_multiplier"),
		MinConcurrencyLimit: marketplaceRelayIntQuery(c, "min_concurrency_limit"),
		MaxConcurrencyLimit: marketplaceRelayIntQuery(c, "max_concurrency_limit"),
		RequestID:           requestID,
	}
	if !common.GetContextKeyBool(c, constant.ContextKeyMarketplacePoolFiltersEnabled) {
		return input, nil
	}
	filters, ok := common.GetContextKeyType[model.MarketplacePoolFilters](c, constant.ContextKeyMarketplacePoolFilters)
	if !ok {
		return input, nil
	}
	values := filters.Values()
	savedModel := strings.TrimSpace(values.Model)
	if savedModel != "" && savedModel != strings.TrimSpace(modelName) {
		return input, fmt.Errorf("request model %s does not match saved marketplace pool conditions model %s", modelName, savedModel)
	}

	input.VendorType = values.VendorType
	if savedModel != "" {
		input.Model = savedModel
	}
	input.QuotaMode = values.QuotaMode
	input.TimeMode = values.TimeMode
	input.MinQuotaLimit = values.MinQuotaLimit
	input.MaxQuotaLimit = values.MaxQuotaLimit
	input.MinTimeLimitSeconds = values.MinTimeLimitSeconds
	input.MaxTimeLimitSeconds = values.MaxTimeLimitSeconds
	input.MinMultiplier = values.MinMultiplier
	input.MaxMultiplier = values.MaxMultiplier
	input.MinConcurrencyLimit = values.MinConcurrencyLimit
	input.MaxConcurrencyLimit = values.MaxConcurrencyLimit
	return input, nil
}

func setupMarketplaceRelayChannelContext(c *gin.Context, credential *model.MarketplaceCredential, apiKey string) {
	channel := service.MarketplaceChannelFromCredential(credential, apiKey)
	common.SetContextKey(c, constant.ContextKeyChannelId, 0)
	common.SetContextKey(c, constant.ContextKeyChannelName, channel.Name)
	common.SetContextKey(c, constant.ContextKeyChannelType, channel.Type)
	common.SetContextKey(c, constant.ContextKeyChannelCreateTime, channel.CreatedTime)
	common.SetContextKey(c, constant.ContextKeyChannelSetting, channel.GetSetting())
	common.SetContextKey(c, constant.ContextKeyChannelOtherSetting, channel.GetOtherSettings())
	common.SetContextKey(c, constant.ContextKeyChannelParamOverride, channel.GetParamOverride())
	common.SetContextKey(c, constant.ContextKeyChannelHeaderOverride, channel.GetHeaderOverride())
	if channel.OpenAIOrganization != nil && *channel.OpenAIOrganization != "" {
		common.SetContextKey(c, constant.ContextKeyChannelOrganization, *channel.OpenAIOrganization)
	}
	common.SetContextKey(c, constant.ContextKeyChannelModelMapping, channel.GetModelMapping())
	common.SetContextKey(c, constant.ContextKeyChannelStatusCodeMapping, channel.GetStatusCodeMapping())
	common.SetContextKey(c, constant.ContextKeyChannelIsMultiKey, false)
	common.SetContextKey(c, constant.ContextKeyChannelMultiKeyIndex, 0)
	common.SetContextKey(c, constant.ContextKeyChannelKey, apiKey)
	common.SetContextKey(c, constant.ContextKeyChannelBaseUrl, channel.GetBaseURL())
	common.SetContextKey(c, constant.ContextKeySystemPromptOverride, false)
	switch channel.Type {
	case constant.ChannelTypeAzure:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeVertexAi:
		c.Set("region", channel.Other)
	case constant.ChannelTypeXunfei:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeGemini:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeAli:
		c.Set("plugin", channel.Other)
	case constant.ChannelCloudflare:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeMokaAI:
		c.Set("api_version", channel.Other)
	case constant.ChannelTypeCoze:
		c.Set("bot_id", channel.Other)
	}
}

func marketplaceRelayIntQuery(c *gin.Context, key string) int {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return parsed
}

func marketplaceRelayInt64Query(c *gin.Context, key string) int64 {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func marketplaceRelayFloatQuery(c *gin.Context, key string) float64 {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}
