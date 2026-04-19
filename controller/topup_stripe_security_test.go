package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"
)

func buildStripeCheckoutCompletedPayload(referenceID string, amountTotal int64, customerID string) []byte {
	return []byte(fmt.Sprintf(`{
  "id":"evt_test_checkout_completed",
  "object":"event",
  "api_version":"%s",
  "type":"checkout.session.completed",
  "data":{
    "object":{
      "id":"cs_test_checkout_completed",
      "object":"checkout.session",
      "client_reference_id":"%s",
      "status":"complete",
      "amount_total":%d,
      "currency":"usd",
      "customer":"%s"
    }
  }
}`, stripe.APIVersion, referenceID, amountTotal, customerID))
}

func newStripeWebhookTestContext(payload []byte, signature string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/stripe/webhook", strings.NewReader(string(payload)))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set("Stripe-Signature", signature)
	return ctx, recorder
}

func mustCreateTopUp(t *testing.T, dbTopUp *model.TopUp) {
	t.Helper()
	if err := model.DB.Create(dbTopUp).Error; err != nil {
		t.Fatalf("failed to create topup: %v", err)
	}
}

func loadTopUpByTradeNo(t *testing.T, tradeNo string) *model.TopUp {
	t.Helper()
	topUp := model.GetTopUpByTradeNo(tradeNo)
	if topUp == nil {
		t.Fatalf("expected topup %s to exist", tradeNo)
	}
	return topUp
}

func loadUserByID(t *testing.T, userID int) *model.User {
	t.Helper()
	user, err := model.GetUserById(userID, false)
	if err != nil {
		t.Fatalf("failed to load user %d: %v", userID, err)
	}
	return user
}

func TestStripeWebhookRejectsSignedPayloadWhenWebhookSecretMissing(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionPaymentUser(t, db, 1, "stripe-missing-secret@example.com", "stripe_missing_secret", "")
	topUp := &model.TopUp{
		UserId:        user.Id,
		Amount:        99999,
		Money:         99999,
		TradeNo:       "USR1NOEMPTYSECRET",
		PaymentMethod: "alipay",
		CreateTime:    time.Now().Unix(),
		Status:        common.TopUpStatusPending,
	}
	mustCreateTopUp(t, topUp)

	originalWebhookSecret := setting.StripeWebhookSecret
	setting.StripeWebhookSecret = ""
	t.Cleanup(func() {
		setting.StripeWebhookSecret = originalWebhookSecret
	})

	payload := buildStripeCheckoutCompletedPayload(topUp.TradeNo, 1, "cus_empty_secret")
	signedPayload := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: payload,
		Secret:  "",
	})

	ctx, recorder := newStripeWebhookTestContext(payload, signedPayload.Header)
	StripeWebhook(ctx)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected webhook to be rejected with %d when secret is missing, got %d", http.StatusServiceUnavailable, recorder.Code)
	}

	reloadedTopUp := loadTopUpByTradeNo(t, topUp.TradeNo)
	if reloadedTopUp.Status != common.TopUpStatusPending {
		t.Fatalf("expected topup to remain pending, got %s", reloadedTopUp.Status)
	}

	reloadedUser := loadUserByID(t, user.Id)
	if reloadedUser.Quota != 0 {
		t.Fatalf("expected user quota to stay 0, got %d", reloadedUser.Quota)
	}
}

func TestStripeWebhookDoesNotCompleteNonStripeTopUp(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionPaymentUser(t, db, 1, "stripe-cross-provider@example.com", "stripe_cross_provider", "")
	topUp := &model.TopUp{
		UserId:        user.Id,
		Amount:        99999,
		Money:         99999,
		TradeNo:       "USR1NOCROSSPROVIDER",
		PaymentMethod: "alipay",
		CreateTime:    time.Now().Unix(),
		Status:        common.TopUpStatusPending,
	}
	mustCreateTopUp(t, topUp)

	originalWebhookSecret := setting.StripeWebhookSecret
	setting.StripeWebhookSecret = "whsec_cross_provider"
	t.Cleanup(func() {
		setting.StripeWebhookSecret = originalWebhookSecret
	})

	payload := buildStripeCheckoutCompletedPayload(topUp.TradeNo, 1, "cus_cross_provider")
	signedPayload := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: payload,
		Secret:  setting.StripeWebhookSecret,
	})

	ctx, recorder := newStripeWebhookTestContext(payload, signedPayload.Header)
	StripeWebhook(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected webhook handler to return %d, got %d", http.StatusOK, recorder.Code)
	}

	reloadedTopUp := loadTopUpByTradeNo(t, topUp.TradeNo)
	if reloadedTopUp.Status != common.TopUpStatusPending {
		t.Fatalf("expected non-stripe topup to remain pending, got %s", reloadedTopUp.Status)
	}

	reloadedUser := loadUserByID(t, user.Id)
	if reloadedUser.Quota != 0 {
		t.Fatalf("expected user quota to stay 0, got %d", reloadedUser.Quota)
	}
}

func TestStripeWebhookDoesNotCompleteStripeTopUpWhenAmountMismatches(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionPaymentUser(t, db, 1, "stripe-amount-mismatch@example.com", "stripe_amount_mismatch", "")
	topUp := &model.TopUp{
		UserId:        user.Id,
		Amount:        100,
		Money:         100,
		TradeNo:       "ref_amount_mismatch",
		PaymentMethod: PaymentMethodStripe,
		CreateTime:    time.Now().Unix(),
		Status:        common.TopUpStatusPending,
	}
	mustCreateTopUp(t, topUp)

	originalWebhookSecret := setting.StripeWebhookSecret
	setting.StripeWebhookSecret = "whsec_amount_mismatch"
	t.Cleanup(func() {
		setting.StripeWebhookSecret = originalWebhookSecret
	})

	payload := buildStripeCheckoutCompletedPayload(topUp.TradeNo, 1, "cus_amount_mismatch")
	signedPayload := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: payload,
		Secret:  setting.StripeWebhookSecret,
	})

	ctx, recorder := newStripeWebhookTestContext(payload, signedPayload.Header)
	StripeWebhook(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected webhook handler to return %d, got %d", http.StatusOK, recorder.Code)
	}

	reloadedTopUp := loadTopUpByTradeNo(t, topUp.TradeNo)
	if reloadedTopUp.Status != common.TopUpStatusPending {
		t.Fatalf("expected mismatched-amount topup to remain pending, got %s", reloadedTopUp.Status)
	}

	reloadedUser := loadUserByID(t, user.Id)
	if reloadedUser.Quota != 0 {
		t.Fatalf("expected user quota to stay 0, got %d", reloadedUser.Quota)
	}
}
