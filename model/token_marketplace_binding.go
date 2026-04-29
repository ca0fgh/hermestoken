package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type MarketplaceFixedOrderIDs string

func NewMarketplaceFixedOrderIDs(ids []int) MarketplaceFixedOrderIDs {
	normalized := normalizeMarketplaceFixedOrderIDList(ids)
	if len(normalized) == 0 {
		return ""
	}
	parts := make([]string, 0, len(normalized))
	for _, id := range normalized {
		parts = append(parts, strconv.Itoa(id))
	}
	return MarketplaceFixedOrderIDs(strings.Join(parts, ","))
}

func (ids MarketplaceFixedOrderIDs) Values() []int {
	return parseMarketplaceFixedOrderIDs(string(ids))
}

func (ids MarketplaceFixedOrderIDs) Value() (driver.Value, error) {
	return string(ids), nil
}

func (ids *MarketplaceFixedOrderIDs) Scan(value any) error {
	if ids == nil {
		return nil
	}
	switch v := value.(type) {
	case nil:
		*ids = ""
	case string:
		*ids = MarketplaceFixedOrderIDs(v)
	case []byte:
		*ids = MarketplaceFixedOrderIDs(string(v))
	default:
		return fmt.Errorf("unsupported marketplace fixed order ids type %T", value)
	}
	return nil
}

func (ids MarketplaceFixedOrderIDs) MarshalJSON() ([]byte, error) {
	return json.Marshal(ids.Values())
}

func (ids *MarketplaceFixedOrderIDs) UnmarshalJSON(data []byte) error {
	if ids == nil {
		return nil
	}
	var list []int
	if err := json.Unmarshal(data, &list); err == nil {
		*ids = NewMarketplaceFixedOrderIDs(list)
		return nil
	}
	var raw string
	if err := json.Unmarshal(data, &raw); err == nil {
		*ids = NewMarketplaceFixedOrderIDs(parseMarketplaceFixedOrderIDs(raw))
		return nil
	}
	if string(data) == "null" {
		*ids = ""
		return nil
	}
	return fmt.Errorf("invalid marketplace fixed order ids")
}

func (token *Token) MarketplaceFixedOrderIDList() []int {
	if token == nil {
		return nil
	}
	ids := token.MarketplaceFixedOrderIDs.Values()
	if token.MarketplaceFixedOrderID > 0 {
		ids = append([]int{token.MarketplaceFixedOrderID}, ids...)
	}
	return normalizeMarketplaceFixedOrderIDList(ids)
}

func (token *Token) SetMarketplaceFixedOrderIDList(ids []int) {
	if token == nil {
		return
	}
	normalized := normalizeMarketplaceFixedOrderIDList(ids)
	token.MarketplaceFixedOrderIDs = NewMarketplaceFixedOrderIDs(normalized)
	if len(normalized) == 0 {
		token.MarketplaceFixedOrderID = 0
		return
	}
	token.MarketplaceFixedOrderID = normalized[0]
}

func normalizeMarketplaceFixedOrderIDList(ids []int) []int {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int]struct{}, len(ids))
	normalized := make([]int, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized
}

func parseMarketplaceFixedOrderIDs(raw string) []int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if strings.HasPrefix(raw, "[") {
		var ids []int
		if err := json.Unmarshal([]byte(raw), &ids); err == nil {
			return normalizeMarketplaceFixedOrderIDList(ids)
		}
	}
	parts := strings.Split(raw, ",")
	ids := make([]int, 0, len(parts))
	for _, part := range parts {
		id, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return normalizeMarketplaceFixedOrderIDList(ids)
}
