package service

import (
	"errors"
	"strings"

	"github.com/ca0fgh/hermestoken/model"
)

type MarketplaceTokenModelListInput struct {
	BuyerUserID int
	UserGroup   string
	Token       *model.Token
}

func ListMarketplaceTokenModels(input MarketplaceTokenModelListInput) ([]string, error) {
	if input.BuyerUserID <= 0 {
		return nil, errors.New("buyer user id is required")
	}
	if input.Token == nil {
		return []string{}, nil
	}
	enabledRoutes := model.MarketplaceRouteEnabledMap(input.Token.MarketplaceRouteEnabledList())
	models := make([]string, 0)

	for _, route := range input.Token.MarketplaceRouteOrderList() {
		if !enabledRoutes[route] {
			continue
		}
		switch route {
		case model.MarketplaceRouteFixedOrder:
			var err error
			models, err = appendMarketplaceTokenFixedOrderModels(models, input)
			if err != nil {
				return nil, err
			}
		case model.MarketplaceRouteGroup:
			models = appendMarketplaceModelNames(models, marketplaceTokenGroupModels(input))
		case model.MarketplaceRoutePool:
			var err error
			models, err = appendMarketplaceTokenPoolModels(models, input)
			if err != nil {
				return nil, err
			}
		}
	}

	return models, nil
}

func appendMarketplaceTokenFixedOrderModels(models []string, input MarketplaceTokenModelListInput) ([]string, error) {
	fixedOrderModels, err := ListBuyerMarketplaceFixedOrderTokenModels(MarketplaceFixedOrderBindingSelectInput{
		BuyerUserID:   input.BuyerUserID,
		FixedOrderIDs: input.Token.MarketplaceFixedOrderIDList(),
	})
	if err != nil {
		return nil, err
	}
	return appendMarketplaceModelNames(models, fixedOrderModels), nil
}

func appendMarketplaceTokenPoolModels(models []string, input MarketplaceTokenModelListInput) ([]string, error) {
	filter := MarketplaceOrderListInput{BuyerUserID: input.BuyerUserID}
	if input.Token.MarketplacePoolFiltersEnabled {
		filter = marketplaceOrderListInputFromPoolFilters(input.BuyerUserID, input.Token.MarketplacePoolFilters.Values())
	}
	poolModels, err := ListMarketplacePoolRelayModels(filter)
	if err != nil {
		return nil, err
	}
	return appendMarketplaceModelNames(models, poolModels), nil
}

func marketplaceOrderListInputFromPoolFilters(buyerUserID int, values model.MarketplacePoolFilterValues) MarketplaceOrderListInput {
	return MarketplaceOrderListInput{
		BuyerUserID:         buyerUserID,
		VendorType:          values.VendorType,
		Model:               values.Model,
		QuotaMode:           values.QuotaMode,
		TimeMode:            values.TimeMode,
		MinQuotaLimit:       values.MinQuotaLimit,
		MaxQuotaLimit:       values.MaxQuotaLimit,
		MinTimeLimitSeconds: values.MinTimeLimitSeconds,
		MaxTimeLimitSeconds: values.MaxTimeLimitSeconds,
		MinMultiplier:       values.MinMultiplier,
		MaxMultiplier:       values.MaxMultiplier,
		MinConcurrencyLimit: values.MinConcurrencyLimit,
		MaxConcurrencyLimit: values.MaxConcurrencyLimit,
	}
}

func marketplaceTokenGroupModels(input MarketplaceTokenModelListInput) []string {
	group := strings.TrimSpace(input.Token.Group)
	if group == "" {
		group = strings.TrimSpace(input.UserGroup)
	}
	if group == "" {
		return []string{}
	}
	if group == "auto" {
		models := make([]string, 0)
		for _, autoGroup := range GetUserAutoGroupForUser(input.BuyerUserID, input.UserGroup) {
			models = appendMarketplaceModelNames(models, model.GetGroupEnabledModels(autoGroup))
		}
		return models
	}
	return model.GetGroupEnabledModels(group)
}

func appendMarketplaceModels(models []string, rawModels string) []string {
	return appendMarketplaceModelNames(models, strings.Split(rawModels, ","))
}

func appendMarketplaceModelNames(models []string, names []string) []string {
	seen := make(map[string]struct{}, len(models)+len(names))
	for _, modelName := range models {
		seen[modelName] = struct{}{}
	}
	for _, modelName := range names {
		modelName = strings.TrimSpace(modelName)
		if modelName == "" {
			continue
		}
		if _, ok := seen[modelName]; ok {
			continue
		}
		seen[modelName] = struct{}{}
		models = append(models, modelName)
	}
	return models
}
