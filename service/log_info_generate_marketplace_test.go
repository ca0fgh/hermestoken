package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	relaycommon "github.com/ca0fgh/hermestoken/relay/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGenerateTextOtherInfoIncludesMarketplaceFixedOrderMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("marketplace_relay", true)
	ctx.Set("marketplace_fixed_order_id", 42)
	ctx.Set("marketplace_original_path", "/marketplace/v1/chat/completions")

	other := GenerateTextOtherInfo(ctx, &relaycommon.RelayInfo{
		BillingSource: "marketplace_fixed_order",
		ChannelMeta:   &relaycommon.ChannelMeta{},
	}, 1, 1, 1, 0, 1, 0, 1)

	assert.Equal(t, "marketplace_fixed_order", other["billing_source"])
	assert.Equal(t, true, other["marketplace_relay"])
	assert.Equal(t, 42, other["marketplace_fixed_order_id"])
	assert.Equal(t, "/marketplace/v1/chat/completions", other["marketplace_original_path"])
}

func TestGenerateTextOtherInfoIncludesMarketplacePoolMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("marketplace_relay", true)
	ctx.Set("marketplace_pool_credential_id", 7)
	ctx.Set("marketplace_original_path", "/marketplace/pool/v1/chat/completions")

	other := GenerateTextOtherInfo(ctx, &relaycommon.RelayInfo{
		BillingSource: "marketplace_pool",
		ChannelMeta:   &relaycommon.ChannelMeta{},
	}, 1, 1, 1, 0, 1, 0, 1)

	assert.Equal(t, "marketplace_pool", other["billing_source"])
	assert.Equal(t, true, other["marketplace_relay"])
	assert.Equal(t, 7, other["marketplace_pool_credential_id"])
	assert.Equal(t, "/marketplace/pool/v1/chat/completions", other["marketplace_original_path"])
}
