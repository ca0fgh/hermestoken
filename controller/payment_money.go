package controller

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"github.com/shopspring/decimal"
)

var zeroDecimalPaymentCurrencies = map[string]bool{
	"IDR": true,
	"JPY": true,
	"KRW": true,
	"VND": true,
}

func isZeroDecimalPaymentCurrency(currency string) bool {
	_, ok := zeroDecimalPaymentCurrencies[strings.ToUpper(strings.TrimSpace(currency))]
	return ok
}

func moneyStringFromMinorUnits(amountMinor int64, currency string) string {
	if isZeroDecimalPaymentCurrency(currency) {
		return decimal.NewFromInt(amountMinor).StringFixed(0)
	}
	return decimal.NewFromInt(amountMinor).
		Div(decimal.NewFromInt(100)).
		Round(2).
		StringFixed(2)
}

func minorUnitsFromMoney(amount float64, currency string) (int64, error) {
	scale := int64(100)
	if isZeroDecimalPaymentCurrency(currency) {
		scale = 1
	}
	minor := decimal.NewFromFloat(amount).
		Mul(decimal.NewFromInt(scale)).
		Round(0)
	if minor.LessThanOrEqual(decimal.Zero) {
		return 0, fmt.Errorf("invalid amount: %.6f", amount)
	}
	return minor.IntPart(), nil
}

func shouldAcknowledgePaymentValidationError(err error) bool {
	return errors.Is(err, model.ErrSubscriptionOrderNotFound) ||
		errors.Is(err, model.ErrSubscriptionOrderStatusInvalid) ||
		errors.Is(err, model.ErrSubscriptionOrderPaymentMethodMismatch) ||
		errors.Is(err, model.ErrSubscriptionOrderAmountMismatch) ||
		errors.Is(err, model.ErrSubscriptionOrderProductMismatch) ||
		errors.Is(err, model.ErrTopUpPaymentMethodMismatch) ||
		errors.Is(err, model.ErrTopUpAmountMismatch) ||
		errors.Is(err, model.ErrTopUpCurrencyMismatch) ||
		errors.Is(err, model.ErrTopUpProductMismatch)
}
