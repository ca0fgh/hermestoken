package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/dto"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/pkg/billingexpr"
	relaycommon "github.com/ca0fgh/hermestoken/relay/common"
	"github.com/ca0fgh/hermestoken/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestSettleTestQuotaUsesTieredBilling(t *testing.T) {
	info := &relaycommon.RelayInfo{
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode:   "tiered_expr",
			ExprString:    `param("stream") == true ? tier("stream", p * 3) : tier("base", p * 2)`,
			ExprHash:      billingexpr.ExprHashString(`param("stream") == true ? tier("stream", p * 3) : tier("base", p * 2)`),
			GroupRatio:    1,
			EstimatedTier: "stream",
			QuotaPerUnit:  common.QuotaPerUnit,
			ExprVersion:   1,
		},
		BillingRequestInput: &billingexpr.RequestInput{
			Body: []byte(`{"stream":true}`),
		},
	}

	quota, result := settleTestQuota(info, types.PriceData{
		ModelRatio:      1,
		CompletionRatio: 2,
	}, &dto.Usage{
		PromptTokens: 1000,
	})

	require.Equal(t, 1500, quota)
	require.NotNil(t, result)
	require.Equal(t, "stream", result.MatchedTier)
}

func TestBuildTestLogOtherInjectsTieredInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	info := &relaycommon.RelayInfo{
		TieredBillingSnapshot: &billingexpr.BillingSnapshot{
			BillingMode: "tiered_expr",
			ExprString:  `tier("base", p * 2)`,
		},
		ChannelMeta: &relaycommon.ChannelMeta{},
	}
	priceData := types.PriceData{
		GroupRatioInfo: types.GroupRatioInfo{GroupRatio: 1},
	}
	usage := &dto.Usage{
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 12,
		},
	}

	other := buildTestLogOther(ctx, info, priceData, usage, &billingexpr.TieredResult{
		MatchedTier: "base",
	})

	require.Equal(t, "tiered_expr", other["billing_mode"])
	require.Equal(t, "base", other["matched_tier"])
	require.NotEmpty(t, other["expr_b64"])
}

func TestResolveChannelTestModelRequiresConfiguredModel(t *testing.T) {
	modelName, err := resolveChannelTestModel(&model.Channel{}, "")

	require.Error(t, err)
	require.Empty(t, modelName)
	require.Contains(t, err.Error(), "请先指定测试模型")
}

func TestResolveChannelTestModelUsesFirstConfiguredModel(t *testing.T) {
	modelName, err := resolveChannelTestModel(&model.Channel{
		Models: "claude-opus-4-7,claude-opus-4-6",
	}, "")

	require.NoError(t, err)
	require.Equal(t, "claude-opus-4-7", modelName)
}
