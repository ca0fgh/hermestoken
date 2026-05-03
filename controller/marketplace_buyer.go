package controller

import (
	"errors"
	"strconv"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/service"
	"github.com/gin-gonic/gin"
)

type marketplaceFixedOrderCreateRequest struct {
	CredentialID       int     `json:"credential_id"`
	PurchasedQuota     int64   `json:"purchased_quota"`
	PurchasedAmountUSD float64 `json:"purchased_amount_usd"`
}

type marketplaceFixedOrderBindTokenRequest struct {
	TokenID       int   `json:"token_id"`
	FixedOrderIDs []int `json:"fixed_order_ids"`
}

type marketplaceFixedOrderBindTokensRequest struct {
	TokenIDs []int `json:"token_ids"`
}

type marketplacePoolFiltersSaveRequest struct {
	TokenID int                               `json:"token_id"`
	Filters model.MarketplacePoolFilterValues `json:"filters"`
}

func BuyerListMarketplaceOrders(c *gin.Context) {
	userID := c.GetInt("id")
	filter, err := marketplaceOrderListFilter(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	filter.BuyerUserID = userID

	pageInfo := common.GetPageQuery(c)
	orders, total, err := service.ListMarketplaceOrders(filter, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(orders)
	common.ApiSuccess(c, pageInfo)
}

func BuyerGetMarketplaceOrderFilterRanges(c *gin.Context) {
	userID := c.GetInt("id")
	filter, err := marketplaceOrderListFilter(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	filter.BuyerUserID = userID

	ranges, err := service.GetMarketplaceOrderFilterRanges(filter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, ranges)
}

func BuyerCreateMarketplaceFixedOrder(c *gin.Context) {
	userID := c.GetInt("id")
	var req marketplaceFixedOrderCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	order, err := service.CreateMarketplaceFixedOrder(service.MarketplaceFixedOrderCreateInput{
		BuyerUserID:        userID,
		CredentialID:       req.CredentialID,
		PurchasedQuota:     req.PurchasedQuota,
		PurchasedAmountUSD: req.PurchasedAmountUSD,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, order)
}

func BuyerListMarketplaceFixedOrders(c *gin.Context) {
	userID := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	orders, total, err := service.ListBuyerMarketplaceFixedOrders(userID, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(orders)
	common.ApiSuccess(c, pageInfo)
}

func BuyerGetMarketplaceFixedOrder(c *gin.Context) {
	userID := c.GetInt("id")
	orderID, err := marketplaceIDParam(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	order, err := service.GetBuyerMarketplaceFixedOrder(userID, orderID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, order)
}

func BuyerBindMarketplaceFixedOrderToken(c *gin.Context) {
	userID := c.GetInt("id")
	orderID, err := marketplaceOptionalIDParam(c)
	var req marketplaceFixedOrderBindTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.TokenID <= 0 {
		common.ApiError(c, errors.New("token_id is required"))
		return
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	fixedOrderIDs := req.FixedOrderIDs
	if orderID > 0 && len(fixedOrderIDs) == 0 {
		fixedOrderIDs = []int{orderID}
	}
	fixedOrderIDs, err = service.ValidateBuyerMarketplaceFixedOrderBindings(userID, fixedOrderIDs)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	token, err := model.GetTokenByIds(req.TokenID, userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token.SetMarketplaceFixedOrderIDList(fixedOrderIDs)
	if err := token.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildMaskedTokenResponse(token))
}

func BuyerBindMarketplaceFixedOrderTokens(c *gin.Context) {
	userID := c.GetInt("id")
	orderID, err := marketplaceIDParam(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	var req marketplaceFixedOrderBindTokensRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}

	result, err := service.SetBuyerMarketplaceFixedOrderTokenBindings(userID, orderID, req.TokenIDs)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"fixed_order_id": result.FixedOrderID,
		"token_ids":      result.TokenIDs,
		"tokens":         buildMaskedTokenResponses(result.Tokens),
	})
}

func BuyerListMarketplacePoolModels(c *gin.Context) {
	userID := c.GetInt("id")
	filter, err := marketplaceOrderListFilter(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	filter.BuyerUserID = userID
	models, err := service.ListMarketplacePoolModels(filter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, models)
}

func BuyerListMarketplacePoolCandidates(c *gin.Context) {
	userID := c.GetInt("id")
	filter, err := marketplaceOrderListFilter(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	filter.BuyerUserID = userID

	pageInfo := common.GetPageQuery(c)
	candidates, total, err := service.ListMarketplacePoolCandidates(filter, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(candidates)
	common.ApiSuccess(c, pageInfo)
}

func BuyerSaveMarketplacePoolFilters(c *gin.Context) {
	userID := c.GetInt("id")
	var req marketplacePoolFiltersSaveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.TokenID <= 0 {
		common.ApiError(c, errors.New("token_id is required"))
		return
	}

	token, err := model.GetTokenByIds(req.TokenID, userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token.MarketplacePoolFiltersEnabled = true
	token.MarketplacePoolFilters = model.NewMarketplacePoolFilters(req.Filters)
	if err := token.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildMaskedTokenResponse(token))
}

func MarketplaceListPricing(c *gin.Context) {
	common.ApiSuccess(c, service.ListMarketplacePricingItems())
}

func marketplaceOrderListFilter(c *gin.Context) (service.MarketplaceOrderListInput, error) {
	var filter service.MarketplaceOrderListInput
	if c.Query("vendor_type") != "" {
		vendorType, err := strconv.Atoi(c.Query("vendor_type"))
		if err != nil {
			return filter, err
		}
		filter.VendorType = vendorType
	}
	filter.Model = c.Query("model")
	filter.QuotaMode = c.Query("quota_mode")
	filter.TimeMode = c.Query("time_mode")
	if c.Query("min_quota_limit") != "" {
		minQuotaLimit, err := strconv.ParseInt(c.Query("min_quota_limit"), 10, 64)
		if err != nil {
			return filter, err
		}
		filter.MinQuotaLimit = minQuotaLimit
	}
	if c.Query("max_quota_limit") != "" {
		maxQuotaLimit, err := strconv.ParseInt(c.Query("max_quota_limit"), 10, 64)
		if err != nil {
			return filter, err
		}
		filter.MaxQuotaLimit = maxQuotaLimit
	}
	if c.Query("min_time_limit_seconds") != "" {
		minTimeLimitSeconds, err := strconv.ParseInt(c.Query("min_time_limit_seconds"), 10, 64)
		if err != nil {
			return filter, err
		}
		filter.MinTimeLimitSeconds = minTimeLimitSeconds
	}
	if c.Query("max_time_limit_seconds") != "" {
		maxTimeLimitSeconds, err := strconv.ParseInt(c.Query("max_time_limit_seconds"), 10, 64)
		if err != nil {
			return filter, err
		}
		filter.MaxTimeLimitSeconds = maxTimeLimitSeconds
	}
	if c.Query("min_multiplier") != "" {
		minMultiplier, err := strconv.ParseFloat(c.Query("min_multiplier"), 64)
		if err != nil {
			return filter, err
		}
		filter.MinMultiplier = minMultiplier
	}
	if c.Query("max_multiplier") != "" {
		maxMultiplier, err := strconv.ParseFloat(c.Query("max_multiplier"), 64)
		if err != nil {
			return filter, err
		}
		filter.MaxMultiplier = maxMultiplier
	}
	if c.Query("min_concurrency_limit") != "" {
		minConcurrencyLimit, err := strconv.Atoi(c.Query("min_concurrency_limit"))
		if err != nil {
			return filter, err
		}
		filter.MinConcurrencyLimit = minConcurrencyLimit
	}
	if c.Query("max_concurrency_limit") != "" {
		maxConcurrencyLimit, err := strconv.Atoi(c.Query("max_concurrency_limit"))
		if err != nil {
			return filter, err
		}
		filter.MaxConcurrencyLimit = maxConcurrencyLimit
	}
	return filter, nil
}

func marketplaceIDParam(c *gin.Context) (int, error) {
	return strconv.Atoi(c.Param("id"))
}

func marketplaceOptionalIDParam(c *gin.Context) (int, error) {
	raw := c.Param("id")
	if raw == "" {
		return 0, nil
	}
	return strconv.Atoi(raw)
}
