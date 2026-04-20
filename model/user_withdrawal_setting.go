package model

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

const (
	WithdrawalEnabledOptionKey     = "WithdrawalEnabled"
	WithdrawalMinAmountOptionKey   = "WithdrawalMinAmount"
	WithdrawalInstructionOptionKey = "WithdrawalInstruction"
	WithdrawalFeeRulesOptionKey    = "WithdrawalFeeRules"
)

type UserWithdrawalSetting struct {
	Enabled     bool                `json:"enabled"`
	MinAmount   float64             `json:"min_amount"`
	Instruction string              `json:"instruction"`
	FeeRules    []WithdrawalFeeRule `json:"fee_rules"`
}

type UserWithdrawalCurrencyConfig struct {
	Type              string  `json:"type"`
	Currency          string  `json:"currency"`
	Symbol            string  `json:"symbol"`
	UsdToCurrencyRate float64 `json:"usd_to_currency_rate"`
}

func GetUserWithdrawalSetting() UserWithdrawalSetting {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()

	setting := UserWithdrawalSetting{
		Enabled:     strings.TrimSpace(common.OptionMap[WithdrawalEnabledOptionKey]) == "true",
		Instruction: strings.TrimSpace(common.OptionMap[WithdrawalInstructionOptionKey]),
		FeeRules:    []WithdrawalFeeRule{},
	}

	if minAmount, err := strconv.ParseFloat(strings.TrimSpace(common.OptionMap[WithdrawalMinAmountOptionKey]), 64); err == nil {
		setting.MinAmount = minAmount
	}
	if rules, err := ParseWithdrawalFeeRules(common.OptionMap[WithdrawalFeeRulesOptionKey]); err == nil {
		setting.FeeRules = rules
	}
	return setting
}

func ValidateUserWithdrawalSetting(setting UserWithdrawalSetting) error {
	if setting.MinAmount < 0 {
		return fmt.Errorf("invalid withdrawal min amount")
	}
	_, err := normalizeWithdrawalFeeRules(setting.FeeRules)
	return err
}

func ParseWithdrawalFeeRules(raw string) ([]WithdrawalFeeRule, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return []WithdrawalFeeRule{}, nil
	}

	var rules []WithdrawalFeeRule
	if err := common.UnmarshalJsonStr(trimmed, &rules); err != nil {
		return nil, err
	}
	return normalizeWithdrawalFeeRules(rules)
}

func normalizeWithdrawalFeeRules(rules []WithdrawalFeeRule) ([]WithdrawalFeeRule, error) {
	normalized := make([]WithdrawalFeeRule, 0, len(rules))
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if rule.MinAmount < 0 {
			return nil, fmt.Errorf("invalid withdrawal fee rule min amount")
		}
		if rule.MaxAmount < 0 {
			return nil, fmt.Errorf("invalid withdrawal fee rule max amount")
		}
		if rule.MaxAmount > 0 && rule.MaxAmount <= rule.MinAmount {
			return nil, fmt.Errorf("invalid withdrawal fee rule range")
		}
		switch strings.TrimSpace(rule.FeeType) {
		case WithdrawalFeeTypeFixed, WithdrawalFeeTypeRatio:
		default:
			return nil, fmt.Errorf("invalid withdrawal fee type")
		}
		if rule.FeeValue < 0 {
			return nil, fmt.Errorf("invalid withdrawal fee value")
		}
		if strings.TrimSpace(rule.FeeType) == WithdrawalFeeTypeRatio && rule.FeeValue > 100 {
			return nil, fmt.Errorf("invalid withdrawal ratio fee value")
		}
		if rule.MinFee < 0 || rule.MaxFee < 0 {
			return nil, fmt.Errorf("invalid withdrawal fee clamp")
		}
		normalized = append(normalized, rule)
	}

	sort.SliceStable(normalized, func(i, j int) bool {
		if normalized[i].SortOrder == normalized[j].SortOrder {
			if normalized[i].MinAmount == normalized[j].MinAmount {
				return normalized[i].MaxAmount < normalized[j].MaxAmount
			}
			return normalized[i].MinAmount < normalized[j].MinAmount
		}
		return normalized[i].SortOrder < normalized[j].SortOrder
	})

	for i := 1; i < len(normalized); i++ {
		prev := normalized[i-1]
		current := normalized[i]
		if prev.MaxAmount == 0 {
			return nil, fmt.Errorf("withdrawal fee rules overlap")
		}
		if current.MinAmount < prev.MaxAmount {
			return nil, fmt.Errorf("withdrawal fee rules overlap")
		}
	}

	return normalized, nil
}

func GetUserWithdrawalCurrencyConfig() UserWithdrawalCurrencyConfig {
	displayType := operation_setting.GetQuotaDisplayType()
	switch displayType {
	case operation_setting.QuotaDisplayTypeUSD:
		return UserWithdrawalCurrencyConfig{
			Type:              displayType,
			Currency:          "USD",
			Symbol:            "$",
			UsdToCurrencyRate: 1,
		}
	case operation_setting.QuotaDisplayTypeCustom:
		rate := operation_setting.GetUsdToCurrencyRate(operation_setting.USDExchangeRate)
		if rate <= 0 {
			rate = 1
		}
		symbol := operation_setting.GetGeneralSetting().CustomCurrencySymbol
		if strings.TrimSpace(symbol) == "" {
			symbol = "¤"
		}
		return UserWithdrawalCurrencyConfig{
			Type:              displayType,
			Currency:          "CUSTOM",
			Symbol:            symbol,
			UsdToCurrencyRate: rate,
		}
	case operation_setting.QuotaDisplayTypeCNY, operation_setting.QuotaDisplayTypeTokens:
		rate := operation_setting.USDExchangeRate
		if rate <= 0 {
			rate = 1
		}
		return UserWithdrawalCurrencyConfig{
			Type:              operation_setting.QuotaDisplayTypeCNY,
			Currency:          "CNY",
			Symbol:            "¥",
			UsdToCurrencyRate: rate,
		}
	default:
		return UserWithdrawalCurrencyConfig{
			Type:              "USD",
			Currency:          "USD",
			Symbol:            "$",
			UsdToCurrencyRate: 1,
		}
	}
}
