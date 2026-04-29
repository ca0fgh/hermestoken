package model

import (
	"database/sql/driver"
	"fmt"
	"strings"

	"github.com/ca0fgh/hermestoken/common"
)

const (
	MarketplaceRouteFixedOrder = "fixed_order"
	MarketplaceRouteGroup      = "group"
	MarketplaceRoutePool       = "pool"
)

type MarketplaceRouteOrder string

type MarketplaceRouteEnabled string

func DefaultMarketplaceRouteOrderList() []string {
	return []string{MarketplaceRouteFixedOrder, MarketplaceRouteGroup, MarketplaceRoutePool}
}

func NewMarketplaceRouteOrder(routes []string) MarketplaceRouteOrder {
	return MarketplaceRouteOrder(strings.Join(NormalizeMarketplaceRouteOrderList(routes), ","))
}

func NewMarketplaceRouteEnabled(routes []string) MarketplaceRouteEnabled {
	normalized := NormalizeMarketplaceRouteEnabledList(routes)
	if len(normalized) == 0 {
		return MarketplaceRouteEnabled("none")
	}
	return MarketplaceRouteEnabled(strings.Join(normalized, ","))
}

func NormalizeMarketplaceRouteOrderList(routes []string) []string {
	seen := make(map[string]bool, len(routes))
	normalized := make([]string, 0, len(DefaultMarketplaceRouteOrderList()))
	for _, route := range routes {
		route = normalizeMarketplaceRouteName(route)
		if route == "" || seen[route] {
			continue
		}
		seen[route] = true
		normalized = append(normalized, route)
	}
	for _, route := range DefaultMarketplaceRouteOrderList() {
		if seen[route] {
			continue
		}
		normalized = append(normalized, route)
	}
	return normalized
}

func NormalizeMarketplaceRouteEnabledList(routes []string) []string {
	seen := make(map[string]bool, len(routes))
	normalized := make([]string, 0, len(DefaultMarketplaceRouteOrderList()))
	for _, route := range routes {
		route = normalizeMarketplaceRouteName(route)
		if route == "" || seen[route] {
			continue
		}
		seen[route] = true
		normalized = append(normalized, route)
	}
	return normalized
}

func (order MarketplaceRouteOrder) Routes() []string {
	raw := strings.TrimSpace(string(order))
	if raw == "" {
		return DefaultMarketplaceRouteOrderList()
	}
	return NormalizeMarketplaceRouteOrderList(strings.Split(raw, ","))
}

func (enabled MarketplaceRouteEnabled) Routes() []string {
	raw := strings.TrimSpace(string(enabled))
	if raw == "" {
		return DefaultMarketplaceRouteOrderList()
	}
	if raw == "none" {
		return []string{}
	}
	return NormalizeMarketplaceRouteEnabledList(strings.Split(raw, ","))
}

func (order MarketplaceRouteOrder) Value() (driver.Value, error) {
	return strings.Join(order.Routes(), ","), nil
}

func (enabled MarketplaceRouteEnabled) Value() (driver.Value, error) {
	routes := enabled.Routes()
	if len(routes) == 0 {
		return "none", nil
	}
	return strings.Join(routes, ","), nil
}

func (order *MarketplaceRouteOrder) Scan(value any) error {
	if order == nil {
		return nil
	}
	switch v := value.(type) {
	case nil:
		*order = NewMarketplaceRouteOrder(nil)
	case string:
		*order = NewMarketplaceRouteOrder(strings.Split(v, ","))
	case []byte:
		*order = NewMarketplaceRouteOrder(strings.Split(string(v), ","))
	default:
		return fmt.Errorf("unsupported marketplace route order type %T", value)
	}
	return nil
}

func (enabled *MarketplaceRouteEnabled) Scan(value any) error {
	if enabled == nil {
		return nil
	}
	switch v := value.(type) {
	case nil:
		*enabled = MarketplaceRouteEnabled("")
	case string:
		*enabled = scanMarketplaceRouteEnabledString(v)
	case []byte:
		*enabled = scanMarketplaceRouteEnabledString(string(v))
	default:
		return fmt.Errorf("unsupported marketplace route enabled type %T", value)
	}
	return nil
}

func (order MarketplaceRouteOrder) MarshalJSON() ([]byte, error) {
	return common.Marshal(order.Routes())
}

func (enabled MarketplaceRouteEnabled) MarshalJSON() ([]byte, error) {
	return common.Marshal(enabled.Routes())
}

func (order *MarketplaceRouteOrder) UnmarshalJSON(data []byte) error {
	if order == nil {
		return nil
	}
	var list []string
	if err := common.Unmarshal(data, &list); err == nil {
		*order = NewMarketplaceRouteOrder(list)
		return nil
	}
	var raw string
	if err := common.Unmarshal(data, &raw); err == nil {
		*order = NewMarketplaceRouteOrder(strings.Split(raw, ","))
		return nil
	}
	if string(data) == "null" {
		*order = NewMarketplaceRouteOrder(nil)
		return nil
	}
	return fmt.Errorf("invalid marketplace route order")
}

func (enabled *MarketplaceRouteEnabled) UnmarshalJSON(data []byte) error {
	if enabled == nil {
		return nil
	}
	var list []string
	if err := common.Unmarshal(data, &list); err == nil {
		*enabled = NewMarketplaceRouteEnabled(list)
		return nil
	}
	var raw string
	if err := common.Unmarshal(data, &raw); err == nil {
		*enabled = scanMarketplaceRouteEnabledString(raw)
		return nil
	}
	if string(data) == "null" {
		*enabled = MarketplaceRouteEnabled("")
		return nil
	}
	return fmt.Errorf("invalid marketplace route enabled")
}

func (token *Token) MarketplaceRouteOrderList() []string {
	if token == nil {
		return DefaultMarketplaceRouteOrderList()
	}
	return token.MarketplaceRouteOrder.Routes()
}

func (token *Token) MarketplaceRouteEnabledList() []string {
	if token == nil {
		return DefaultMarketplaceRouteOrderList()
	}
	return token.MarketplaceRouteEnabled.Routes()
}

func MarketplaceRouteEnabledMap(routes []string) map[string]bool {
	enabled := make(map[string]bool, len(routes))
	for _, route := range NormalizeMarketplaceRouteEnabledList(routes) {
		enabled[route] = true
	}
	return enabled
}

func scanMarketplaceRouteEnabledString(raw string) MarketplaceRouteEnabled {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return MarketplaceRouteEnabled("")
	}
	if raw == "none" {
		return MarketplaceRouteEnabled("none")
	}
	return NewMarketplaceRouteEnabled(strings.Split(raw, ","))
}

func normalizeMarketplaceRouteName(route string) string {
	switch strings.TrimSpace(route) {
	case MarketplaceRouteFixedOrder, "marketplace_fixed_order", "fixed", "order":
		return MarketplaceRouteFixedOrder
	case MarketplaceRouteGroup, "normal_group", "ordinary_group", "channel":
		return MarketplaceRouteGroup
	case MarketplaceRoutePool, "marketplace_pool", "order_pool":
		return MarketplaceRoutePool
	default:
		return ""
	}
}
