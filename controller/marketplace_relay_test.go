package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/constant"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarketplaceFixedOrderIDsFromRequestUsesHeaderWhenPresent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/marketplace/v1/chat/completions", nil)
	ctx.Request.Header.Set(marketplaceFixedOrderHeader, "42")
	ctx.Set("token_marketplace_fixed_order_ids", []int{7, 8})

	ids, headerSet, err := marketplaceFixedOrderIDsFromRequest(ctx)

	require.NoError(t, err)
	assert.True(t, headerSet)
	assert.Equal(t, []int{42}, ids)
}

func TestMarketplaceFixedOrderIDsFromRequestUsesTokenBindings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/marketplace/v1/chat/completions", nil)
	ctx.Set("token_marketplace_fixed_order_ids", []int{7, 8})

	ids, headerSet, err := marketplaceFixedOrderIDsFromRequest(ctx)

	require.NoError(t, err)
	assert.False(t, headerSet)
	assert.Equal(t, []int{7, 8}, ids)
}

func TestMarketplaceFixedOrderIDsFromRequestRequiresHeaderOrTokenBinding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/marketplace/v1/chat/completions", nil)

	ids, headerSet, err := marketplaceFixedOrderIDsFromRequest(ctx)

	assert.Empty(t, ids)
	assert.False(t, headerSet)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bind a marketplace fixed order to the token")
}

func TestMarketplaceUnifiedRouteOrderFiltersDisabledRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	common.SetContextKey(ctx, constant.ContextKeyMarketplaceRouteOrder, []string{
		model.MarketplaceRouteFixedOrder,
		model.MarketplaceRouteGroup,
		model.MarketplaceRoutePool,
	})
	common.SetContextKey(ctx, constant.ContextKeyMarketplaceRouteEnabled, []string{
		model.MarketplaceRouteGroup,
		model.MarketplaceRoutePool,
	})

	assert.Equal(t, []string{
		model.MarketplaceRouteGroup,
		model.MarketplaceRoutePool,
	}, marketplaceUnifiedRouteOrder(ctx))
}

func TestMarketplaceFixedOrderRelayRejectsDisabledTokenRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/marketplace/v1/chat/completions", nil)
	common.SetContextKey(ctx, constant.ContextKeyMarketplaceRouteEnabled, []string{
		model.MarketplaceRouteGroup,
		model.MarketplaceRoutePool,
	})

	MarketplaceFixedOrderRelay(ctx, types.RelayFormatOpenAI)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "marketplace fixed order route is disabled")
}

func TestMarketplacePoolRelayRejectsDisabledTokenRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/marketplace/pool/v1/chat/completions", nil)
	common.SetContextKey(ctx, constant.ContextKeyMarketplaceRouteEnabled, []string{
		model.MarketplaceRouteFixedOrder,
		model.MarketplaceRouteGroup,
	})

	MarketplacePoolRelay(ctx, types.RelayFormatOpenAI)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "marketplace pool route is disabled")
}
