package service

import (
	"errors"
	"net/http"
	"testing"

	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/types"
)

func TestShouldFallbackToWalletAfterSubscriptionError(t *testing.T) {
	t.Run("allows wallet fallback when there is no active subscription", func(t *testing.T) {
		err := types.NewErrorWithStatusCode(
			errors.New("订阅额度不足或未配置订阅: no active subscription"),
			types.ErrorCodeInsufficientUserQuota,
			http.StatusForbidden,
		)

		if !shouldFallbackToWalletAfterSubscriptionError(err) {
			t.Fatal("expected no-active-subscription error to allow wallet fallback")
		}
	})

	t.Run("blocks wallet fallback after quota exhaustion downgraded the user group", func(t *testing.T) {
		err := types.NewErrorWithStatusCode(
			errors.Join(errors.New("订阅额度不足或未配置订阅"), model.ErrSubscriptionQuotaInsufficient),
			types.ErrorCodeInsufficientUserQuota,
			http.StatusForbidden,
		)

		if shouldFallbackToWalletAfterSubscriptionError(err) {
			t.Fatal("expected quota-exhausted subscription error to block wallet fallback")
		}
	})
}
