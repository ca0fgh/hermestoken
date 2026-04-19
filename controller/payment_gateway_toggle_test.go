package controller

import (
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

type topupInfoResponse struct {
	EnableOnlineTopup bool                `json:"enable_online_topup"`
	EnableStripeTopup bool                `json:"enable_stripe_topup"`
	EnableCreemTopup  bool                `json:"enable_creem_topup"`
	PayMethods        []map[string]string `json:"pay_methods"`
}

func mustUpdateOptionValue(t *testing.T, key string, value string) {
	t.Helper()

	if err := model.UpdateOption(key, value); err != nil {
		t.Fatalf("failed to update option %s: %v", key, err)
	}
}

func containsPaymentMethod(methods []map[string]string, methodType string) bool {
	for _, method := range methods {
		if method["type"] == methodType {
			return true
		}
	}
	return false
}

func withConfiguredTopupGateways(t *testing.T) {
	t.Helper()

	originalPayAddress := operation_setting.PayAddress
	originalEpayID := operation_setting.EpayId
	originalEpayKey := operation_setting.EpayKey
	originalPayMethods := append([]map[string]string(nil), operation_setting.PayMethods...)
	originalStripeSecret := setting.StripeApiSecret
	originalStripeWebhookSecret := setting.StripeWebhookSecret
	originalCreemAPIKey := setting.CreemApiKey
	originalCreemProducts := setting.CreemProducts
	originalCreemWebhookSecret := setting.CreemWebhookSecret
	originalCreemTestMode := setting.CreemTestMode

	operation_setting.PayAddress = "https://epay.example.com"
	operation_setting.EpayId = "epay_merchant"
	operation_setting.EpayKey = "epay_secret"
	operation_setting.PayMethods = []map[string]string{
		{
			"name": "支付宝",
			"type": "alipay",
		},
	}
	setting.StripeApiSecret = "sk_test_toggle"
	setting.StripeWebhookSecret = "whsec_toggle"
	setting.CreemApiKey = "creem_toggle_key"
	setting.CreemProducts = `[{"productId":"prod_toggle","name":"Toggle Product","price":9.9,"currency":"USD","quota":1000}]`
	setting.CreemWebhookSecret = "creem_toggle_secret"
	setting.CreemTestMode = false

	t.Cleanup(func() {
		operation_setting.PayAddress = originalPayAddress
		operation_setting.EpayId = originalEpayID
		operation_setting.EpayKey = originalEpayKey
		operation_setting.PayMethods = originalPayMethods
		setting.StripeApiSecret = originalStripeSecret
		setting.StripeWebhookSecret = originalStripeWebhookSecret
		setting.CreemApiKey = originalCreemAPIKey
		setting.CreemProducts = originalCreemProducts
		setting.CreemWebhookSecret = originalCreemWebhookSecret
		setting.CreemTestMode = originalCreemTestMode
	})
}

func TestGetTopUpInfoHonorsPaymentGatewaySwitches(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	withConfiguredTopupGateways(t)

	mustUpdateOptionValue(t, "EpayEnabled", "false")
	mustUpdateOptionValue(t, "StripeEnabled", "false")
	mustUpdateOptionValue(t, "CreemEnabled", "false")

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/topup/info", nil, 1)
	GetTopUpInfo(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got %s", response.Message)
	}

	var data topupInfoResponse
	if err := common.Unmarshal(response.Data, &data); err != nil {
		t.Fatalf("failed to decode topup info response: %v", err)
	}

	if data.EnableOnlineTopup {
		t.Fatal("expected epay topup to be disabled by switch")
	}
	if data.EnableStripeTopup {
		t.Fatal("expected stripe topup to be disabled by switch")
	}
	if data.EnableCreemTopup {
		t.Fatal("expected creem topup to be disabled by switch")
	}
	if containsPaymentMethod(data.PayMethods, "stripe") {
		t.Fatal("expected stripe pay method to be hidden when stripe switch is off")
	}

	mustUpdateOptionValue(t, "EpayEnabled", "true")
	mustUpdateOptionValue(t, "StripeEnabled", "true")
	mustUpdateOptionValue(t, "CreemEnabled", "true")

	ctx, recorder = newAuthenticatedContext(t, http.MethodGet, "/api/user/topup/info", nil, 1)
	GetTopUpInfo(ctx)

	response = decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response after re-enabling, got %s", response.Message)
	}
	if err := common.Unmarshal(response.Data, &data); err != nil {
		t.Fatalf("failed to decode topup info response after re-enable: %v", err)
	}

	if !data.EnableOnlineTopup {
		t.Fatal("expected epay topup to be enabled after switch restore")
	}
	if !data.EnableStripeTopup {
		t.Fatal("expected stripe topup to be enabled after switch restore")
	}
	if !data.EnableCreemTopup {
		t.Fatal("expected creem topup to be enabled after switch restore")
	}
	if !containsPaymentMethod(data.PayMethods, "stripe") {
		t.Fatal("expected stripe pay method to be visible when stripe switch is on")
	}
}

func TestTopupRequestsRejectDisabledPaymentGateways(t *testing.T) {
	t.Run("epay", func(t *testing.T) {
		db := setupSubscriptionControllerTestDB(t)
		seedSubscriptionPaymentUser(t, db, 1, "epay-toggle@example.com", "epay_toggle", "")
		originalPayMethods := append([]map[string]string(nil), operation_setting.PayMethods...)
		operation_setting.PayMethods = []map[string]string{
			{
				"name": "支付宝",
				"type": "alipay",
			},
		}
		t.Cleanup(func() {
			operation_setting.PayMethods = originalPayMethods
		})
		mustUpdateOptionValue(t, "EpayEnabled", "false")

		ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/topup", map[string]any{
			"amount":         10,
			"payment_method": "alipay",
		}, 1)

		RequestEpay(ctx)

		response := decodeGenericMessageResponse(t, recorder.Body.Bytes())
		if response.Message != "error" {
			t.Fatalf("expected error response when epay is disabled, got %s", recorder.Body.String())
		}
		if !strings.Contains(recorder.Body.String(), "易支付") {
			t.Fatalf("expected disabled epay message, got %s", recorder.Body.String())
		}

		var count int64
		if err := db.Model(&model.TopUp{}).Count(&count).Error; err != nil {
			t.Fatalf("failed to count topups: %v", err)
		}
		if count != 0 {
			t.Fatalf("expected no topup rows when epay is disabled, found %d", count)
		}
	})

	t.Run("epay amount quote", func(t *testing.T) {
		setupSubscriptionControllerTestDB(t)
		mustUpdateOptionValue(t, "EpayEnabled", "false")

		ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/amount", map[string]any{
			"amount": 10,
		}, 1)

		RequestAmount(ctx)

		response := decodeGenericMessageResponse(t, recorder.Body.Bytes())
		if response.Message != "error" || !strings.Contains(recorder.Body.String(), "易支付") {
			t.Fatalf("expected disabled epay quote message, got %s", recorder.Body.String())
		}
	})

	t.Run("stripe", func(t *testing.T) {
		db := setupSubscriptionControllerTestDB(t)
		seedSubscriptionPaymentUser(t, db, 1, "stripe-toggle@example.com", "stripe_toggle", "")
		mustUpdateOptionValue(t, "StripeEnabled", "false")

		ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/stripe/pay", map[string]any{
			"amount":         10,
			"payment_method": "stripe",
		}, 1)

		RequestStripePay(ctx)

		response := decodeGenericMessageResponse(t, recorder.Body.Bytes())
		if response.Message != "error" || !strings.Contains(recorder.Body.String(), "Stripe") {
			t.Fatalf("expected disabled stripe message, got %s", recorder.Body.String())
		}

		var count int64
		if err := db.Model(&model.TopUp{}).Count(&count).Error; err != nil {
			t.Fatalf("failed to count topups: %v", err)
		}
		if count != 0 {
			t.Fatalf("expected no topup rows when stripe is disabled, found %d", count)
		}
	})

	t.Run("creem", func(t *testing.T) {
		db := setupSubscriptionControllerTestDB(t)
		seedSubscriptionPaymentUser(t, db, 1, "creem-toggle@example.com", "creem_toggle", "")
		mustUpdateOptionValue(t, "CreemEnabled", "false")

		ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/creem/pay", map[string]any{
			"product_id":     "prod_toggle",
			"payment_method": "creem",
		}, 1)

		RequestCreemPay(ctx)

		response := decodeGenericMessageResponse(t, recorder.Body.Bytes())
		if response.Message != "error" || !strings.Contains(recorder.Body.String(), "Creem") {
			t.Fatalf("expected disabled creem message, got %s", recorder.Body.String())
		}

		var count int64
		if err := db.Model(&model.TopUp{}).Count(&count).Error; err != nil {
			t.Fatalf("failed to count topups: %v", err)
		}
		if count != 0 {
			t.Fatalf("expected no topup rows when creem is disabled, found %d", count)
		}
	})
}

func TestSubscriptionRequestsRejectDisabledPaymentGateways(t *testing.T) {
	t.Run("epay", func(t *testing.T) {
		db := setupSubscriptionControllerTestDB(t)
		seedSubscriptionPaymentUser(t, db, 1, "sub-epay-toggle@example.com", "sub_epay_toggle", "")
		plan := seedSubscriptionPlan(t, db, "sub-epay-toggle")
		originalPayMethods := append([]map[string]string(nil), operation_setting.PayMethods...)
		operation_setting.PayMethods = []map[string]string{
			{
				"name": "支付宝",
				"type": "alipay",
			},
		}
		t.Cleanup(func() {
			operation_setting.PayMethods = originalPayMethods
		})
		mustUpdateOptionValue(t, "EpayEnabled", "false")

		ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/subscription/epay/pay", map[string]any{
			"plan_id":        plan.Id,
			"quantity":       1,
			"payment_method": "alipay",
		}, 1)

		SubscriptionRequestEpay(ctx)

		response := decodeAPIResponse(t, recorder)
		if response.Success || response.Message == "" || !strings.Contains(response.Message, "易支付") {
			t.Fatalf("expected disabled epay subscription message, got %+v", response)
		}

		var count int64
		if err := db.Model(&model.SubscriptionOrder{}).Count(&count).Error; err != nil {
			t.Fatalf("failed to count subscription orders: %v", err)
		}
		if count != 0 {
			t.Fatalf("expected no subscription orders when epay is disabled, found %d", count)
		}
	})

	t.Run("stripe", func(t *testing.T) {
		db := setupSubscriptionControllerTestDB(t)
		seedSubscriptionPaymentUser(t, db, 1, "sub-stripe-toggle@example.com", "sub_stripe_toggle", "")
		plan := seedSubscriptionPlan(t, db, "sub-stripe-toggle")
		mustUpdateSubscriptionPlan(t, db, plan.Id, map[string]interface{}{
			"stripe_price_id": "price_toggle",
		})
		mustUpdateOptionValue(t, "StripeEnabled", "false")

		ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/subscription/stripe/pay", map[string]any{
			"plan_id":  plan.Id,
			"quantity": 1,
		}, 1)

		SubscriptionRequestStripePay(ctx)

		response := decodeAPIResponse(t, recorder)
		if response.Success || response.Message == "" || !strings.Contains(response.Message, "Stripe") {
			t.Fatalf("expected disabled stripe subscription message, got %+v", response)
		}

		var count int64
		if err := db.Model(&model.SubscriptionOrder{}).Count(&count).Error; err != nil {
			t.Fatalf("failed to count subscription orders: %v", err)
		}
		if count != 0 {
			t.Fatalf("expected no subscription orders when stripe is disabled, found %d", count)
		}
	})

	t.Run("creem", func(t *testing.T) {
		db := setupSubscriptionControllerTestDB(t)
		seedSubscriptionPaymentUser(t, db, 1, "sub-creem-toggle@example.com", "sub_creem_toggle", "")
		plan := seedSubscriptionPlan(t, db, "sub-creem-toggle")
		mustUpdateSubscriptionPlan(t, db, plan.Id, map[string]interface{}{
			"creem_product_id": "prod_toggle",
		})
		mustUpdateOptionValue(t, "CreemEnabled", "false")

		ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/subscription/creem/pay", map[string]any{
			"plan_id":  plan.Id,
			"quantity": 1,
		}, 1)

		SubscriptionRequestCreemPay(ctx)

		response := decodeAPIResponse(t, recorder)
		if response.Success || response.Message == "" || !strings.Contains(response.Message, "Creem") {
			t.Fatalf("expected disabled creem subscription message, got %+v", response)
		}

		var count int64
		if err := db.Model(&model.SubscriptionOrder{}).Count(&count).Error; err != nil {
			t.Fatalf("failed to count subscription orders: %v", err)
		}
		if count != 0 {
			t.Fatalf("expected no subscription orders when creem is disabled, found %d", count)
		}
	})
}
