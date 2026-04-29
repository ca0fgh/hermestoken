package model

import (
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/stretchr/testify/require"
)

func TestMarketplaceRouteOrderDefaultsAndNormalizes(t *testing.T) {
	require.Equal(t, []string{
		MarketplaceRouteFixedOrder,
		MarketplaceRouteGroup,
		MarketplaceRoutePool,
	}, DefaultMarketplaceRouteOrderList())

	require.Equal(t, []string{
		MarketplaceRouteGroup,
		MarketplaceRoutePool,
		MarketplaceRouteFixedOrder,
	}, NormalizeMarketplaceRouteOrderList([]string{
		"normal_group",
		"pool",
		"pool",
		"fixed",
		"unknown",
	}))
}

func TestMarketplaceRouteEnabledDefaultsAndAllowsEmptySet(t *testing.T) {
	require.Equal(t, []string{
		MarketplaceRouteFixedOrder,
		MarketplaceRouteGroup,
		MarketplaceRoutePool,
	}, MarketplaceRouteEnabled("").Routes())

	require.Equal(t, []string{
		MarketplaceRouteGroup,
		MarketplaceRoutePool,
	}, NewMarketplaceRouteEnabled([]string{
		"normal_group",
		"pool",
		"pool",
		"unknown",
	}).Routes())

	require.Empty(t, NewMarketplaceRouteEnabled(nil).Routes())
}

func TestMarketplaceRouteOrderJSONAcceptsArrayAndString(t *testing.T) {
	var fromArray MarketplaceRouteOrder
	require.NoError(t, common.Unmarshal([]byte(`["pool","group"]`), &fromArray))
	require.Equal(t, []string{
		MarketplaceRoutePool,
		MarketplaceRouteGroup,
		MarketplaceRouteFixedOrder,
	}, fromArray.Routes())

	var fromString MarketplaceRouteOrder
	require.NoError(t, common.Unmarshal([]byte(`"group,fixed_order"`), &fromString))
	require.Equal(t, []string{
		MarketplaceRouteGroup,
		MarketplaceRouteFixedOrder,
		MarketplaceRoutePool,
	}, fromString.Routes())
}

func TestMarketplaceRouteEnabledJSONAcceptsArrayAndEmptySet(t *testing.T) {
	var enabled MarketplaceRouteEnabled
	require.NoError(t, common.Unmarshal([]byte(`["pool","group"]`), &enabled))
	require.Equal(t, []string{
		MarketplaceRoutePool,
		MarketplaceRouteGroup,
	}, enabled.Routes())

	require.NoError(t, common.Unmarshal([]byte(`[]`), &enabled))
	require.Empty(t, enabled.Routes())
}
