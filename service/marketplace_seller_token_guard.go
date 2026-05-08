package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ca0fgh/hermestoken/model"
	"gorm.io/gorm"
)

func validateMarketplaceCredentialAPIKeyHosting(apiKey string) error {
	for _, key := range marketplaceCredentialAPIKeyCandidates(apiKey) {
		tokenKey := normalizeMarketplaceCredentialTokenKey(key)
		if tokenKey == "" {
			continue
		}
		token, err := model.GetTokenByKey(tokenKey, true)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return fmt.Errorf("failed to validate marketplace hosted token: %w", err)
		}
		if err := validateMarketplaceHostedTokenRoutes(token); err != nil {
			return err
		}
	}
	return nil
}

func marketplaceCredentialAPIKeyCandidates(apiKey string) []string {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil
	}
	return strings.Split(strings.Trim(apiKey, "\n"), "\n")
}

func normalizeMarketplaceCredentialTokenKey(apiKey string) string {
	apiKey = strings.TrimSpace(apiKey)
	if strings.HasPrefix(apiKey, "Bearer ") || strings.HasPrefix(apiKey, "bearer ") {
		apiKey = strings.TrimSpace(apiKey[7:])
	}
	apiKey = strings.TrimPrefix(apiKey, "sk-")
	parts := strings.Split(apiKey, "-")
	return strings.TrimSpace(parts[0])
}

func validateMarketplaceHostedTokenRoutes(token *model.Token) error {
	if token == nil {
		return nil
	}
	if len(token.MarketplaceFixedOrderIDList()) > 0 {
		return errors.New("token bound to marketplace fixed order cannot be hosted in marketplace")
	}
	if token.MarketplacePoolFiltersEnabled {
		return errors.New("token bound to marketplace order pool cannot be hosted in marketplace")
	}
	return nil
}
