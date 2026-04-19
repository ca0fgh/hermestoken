package controller

import (
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
)

type genericMessageResponse struct {
	Message string `json:"message"`
}

func decodeGenericMessageResponse(t *testing.T, recorderBody []byte) genericMessageResponse {
	t.Helper()

	var response genericMessageResponse
	if err := common.Unmarshal(recorderBody, &response); err != nil {
		t.Fatalf("failed to decode generic response: %v", err)
	}
	return response
}

func TestRequestCreemPayRejectsMissingWebhookSecret(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionPaymentUser(t, db, 1, "creem-no-secret@example.com", "creem_no_secret", "")

	originalAPIKey := setting.CreemApiKey
	originalProducts := setting.CreemProducts
	originalWebhookSecret := setting.CreemWebhookSecret
	originalTestMode := setting.CreemTestMode
	setting.CreemApiKey = "creem_test_key"
	setting.CreemProducts = `[{"productId":"prod_1","name":"P1","price":9.9,"currency":"USD","quota":1000}]`
	setting.CreemWebhookSecret = ""
	setting.CreemTestMode = false
	t.Cleanup(func() {
		setting.CreemApiKey = originalAPIKey
		setting.CreemProducts = originalProducts
		setting.CreemWebhookSecret = originalWebhookSecret
		setting.CreemTestMode = originalTestMode
	})

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/creem/pay", map[string]any{
		"product_id":     "prod_1",
		"payment_method": "creem",
	}, user.Id)

	RequestCreemPay(ctx)

	response := decodeGenericMessageResponse(t, recorder.Body.Bytes())
	if response.Message != "error" {
		t.Fatalf("expected error message, got %s", response.Message)
	}

	var count int64
	if err := db.Model(&model.TopUp{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count topups: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no topup rows to be created, found %d", count)
	}
}

func TestRequestStripePayStoresComputedMoneyWithoutStripePriceID(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionPaymentUser(t, db, 1, "stripe-dynamic@example.com", "stripe_dynamic", "")

	originalSecret := setting.StripeApiSecret
	originalWebhookSecret := setting.StripeWebhookSecret
	originalPriceID := setting.StripePriceId
	originalUnitPrice := setting.StripeUnitPrice
	setting.StripeApiSecret = "sk_test_dynamic"
	setting.StripeWebhookSecret = "whsec_dynamic"
	setting.StripePriceId = ""
	setting.StripeUnitPrice = 2.5
	t.Cleanup(func() {
		setting.StripeApiSecret = originalSecret
		setting.StripeWebhookSecret = originalWebhookSecret
		setting.StripePriceId = originalPriceID
		setting.StripeUnitPrice = originalUnitPrice
	})

	originalGenerator := stripeCheckoutLinkGenerator
	var captured struct {
		referenceID string
		amount      int64
		payMoney    float64
	}
	stripeCheckoutLinkGenerator = func(referenceID string, customerID string, email string, amount int64, payMoney float64, successURL string, cancelURL string) (string, error) {
		captured.referenceID = referenceID
		captured.amount = amount
		captured.payMoney = payMoney
		return "https://stripe.example/pay", nil
	}
	t.Cleanup(func() {
		stripeCheckoutLinkGenerator = originalGenerator
	})

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/stripe/pay", map[string]any{
		"amount":         10,
		"payment_method": "stripe",
	}, user.Id)

	RequestStripePay(ctx)

	response := decodeGenericMessageResponse(t, recorder.Body.Bytes())
	if response.Message != "success" {
		t.Fatalf("expected success message, got %s", response.Message)
	}
	if captured.payMoney != 25 {
		t.Fatalf("expected pay money 25, got %.2f", captured.payMoney)
	}

	var topUp model.TopUp
	if err := db.Where("trade_no = ?", captured.referenceID).First(&topUp).Error; err != nil {
		t.Fatalf("failed to load created stripe topup: %v", err)
	}
	if topUp.Amount != 10 {
		t.Fatalf("expected stored amount 10, got %d", topUp.Amount)
	}
	if topUp.Money != 25 {
		t.Fatalf("expected stored money 25, got %.2f", topUp.Money)
	}
	if topUp.PaymentMethod != PaymentMethodStripe {
		t.Fatalf("expected payment method %s, got %s", PaymentMethodStripe, topUp.PaymentMethod)
	}
}

func TestSubscriptionRequestStripePayRejectsPriceMismatch(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionPaymentUser(t, db, 1, "stripe-sub-mismatch@example.com", "stripe_sub_mismatch", "")
	plan := seedSubscriptionPlan(t, db, "stripe-price-mismatch-plan")
	mustUpdateSubscriptionPlan(t, db, plan.Id, map[string]any{
		"price_amount":    30.0,
		"stripe_price_id": "price_mismatch",
	})

	withSubscriptionStripeSettings(t)

	originalResolver := subscriptionStripeUnitAmountResolver
	subscriptionStripeUnitAmountResolver = func(priceID string) (int64, error) {
		return 1000, nil
	}
	t.Cleanup(func() {
		subscriptionStripeUnitAmountResolver = originalResolver
	})

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/subscription/stripe/pay", map[string]any{
		"plan_id":  plan.Id,
		"quantity": 1,
	}, user.Id)

	SubscriptionRequestStripePay(ctx)

	response := decodeGenericMessageResponse(t, recorder.Body.Bytes())
	if response.Message == "success" {
		t.Fatalf("expected mismatch to be rejected, got success body %s", recorder.Body.String())
	}

	var count int64
	if err := db.Model(&model.SubscriptionOrder{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count subscription orders: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected no subscription order to be created, found %d", count)
	}
}

func TestRechargeCreemAndWaffoRejectAmountMismatch(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionPaymentUser(t, db, 1, "provider-mismatch@example.com", "provider_mismatch", "")

	creemTopUp := &model.TopUp{
		UserId:            user.Id,
		Amount:            500,
		Money:             9.9,
		TradeNo:           "creem_mismatch_trade",
		PaymentMethod:     PaymentMethodCreem,
		Currency:          "USD",
		ProviderProductID: "prod_expected",
		CreateTime:        time.Now().Unix(),
		Status:            common.TopUpStatusPending,
	}
	mustCreateTopUp(t, creemTopUp)

	waffoTopUp := &model.TopUp{
		UserId:        user.Id,
		Amount:        10,
		Money:         15.5,
		TradeNo:       "waffo_mismatch_trade",
		PaymentMethod: "waffo",
		Currency:      "USD",
		CreateTime:    time.Now().Unix(),
		Status:        common.TopUpStatusPending,
	}
	mustCreateTopUp(t, waffoTopUp)

	if err := model.RechargeCreem(creemTopUp.TradeNo, user.Email, "0.01", "USD", "prod_expected"); err == nil {
		t.Fatal("expected creem amount mismatch to be rejected")
	}
	if err := model.RechargeWaffo(waffoTopUp.TradeNo, "0.01", "USD"); err == nil {
		t.Fatal("expected waffo amount mismatch to be rejected")
	}

	if loadTopUpByTradeNo(t, creemTopUp.TradeNo).Status != common.TopUpStatusPending {
		t.Fatal("expected creem topup to remain pending")
	}
	if loadTopUpByTradeNo(t, waffoTopUp.TradeNo).Status != common.TopUpStatusPending {
		t.Fatal("expected waffo topup to remain pending")
	}
}

func TestRechargeCreemRejectsCurrencyAndProductMismatch(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionPaymentUser(t, db, 1, "creem-product-mismatch@example.com", "creem_product_mismatch", "")

	topUp := &model.TopUp{
		UserId:            user.Id,
		Amount:            1000,
		Money:             9.9,
		TradeNo:           "creem_product_currency_mismatch",
		PaymentMethod:     PaymentMethodCreem,
		Currency:          "USD",
		ProviderProductID: "prod_expected",
		CreateTime:        time.Now().Unix(),
		Status:            common.TopUpStatusPending,
	}
	mustCreateTopUp(t, topUp)

	if err := model.RechargeCreem(topUp.TradeNo, user.Email, "9.90", "EUR", "prod_expected"); err == nil {
		t.Fatal("expected creem currency mismatch to be rejected")
	}
	if err := model.RechargeCreem(topUp.TradeNo, user.Email, "9.90", "USD", "prod_wrong"); err == nil {
		t.Fatal("expected creem product mismatch to be rejected")
	}

	if loadTopUpByTradeNo(t, topUp.TradeNo).Status != common.TopUpStatusPending {
		t.Fatal("expected creem topup to remain pending")
	}
}

func TestRechargeWaffoRejectsCurrencyMismatch(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionPaymentUser(t, db, 1, "waffo-currency-mismatch@example.com", "waffo_currency_mismatch", "")

	topUp := &model.TopUp{
		UserId:        user.Id,
		Amount:        100,
		Money:         15.5,
		TradeNo:       "waffo_currency_mismatch",
		PaymentMethod: "waffo",
		Currency:      "USD",
		CreateTime:    time.Now().Unix(),
		Status:        common.TopUpStatusPending,
	}
	mustCreateTopUp(t, topUp)

	if err := model.RechargeWaffo(topUp.TradeNo, "15.50", "HKD"); err == nil {
		t.Fatal("expected waffo currency mismatch to be rejected")
	}

	if loadTopUpByTradeNo(t, topUp.TradeNo).Status != common.TopUpStatusPending {
		t.Fatal("expected waffo topup to remain pending")
	}
}
