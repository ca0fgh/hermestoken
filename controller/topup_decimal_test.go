package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
)

type amountQuoteResponse struct {
	Message string `json:"message"`
	Data    string `json:"data"`
}

func TestRequestAmountAcceptsCentLevelTopup(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	seedSubscriptionPaymentUser(t, db, 1, "cent-topup@example.com", "cent_topup", "")

	originalMinTopUp := operation_setting.MinTopUp
	originalPrice := operation_setting.Price
	originalPayAddress := operation_setting.PayAddress
	originalEpayID := operation_setting.EpayId
	originalEpayKey := operation_setting.EpayKey
	originalPayMethods := append([]map[string]string(nil), operation_setting.PayMethods...)
	t.Cleanup(func() {
		operation_setting.MinTopUp = originalMinTopUp
		operation_setting.Price = originalPrice
		operation_setting.PayAddress = originalPayAddress
		operation_setting.EpayId = originalEpayID
		operation_setting.EpayKey = originalEpayKey
		operation_setting.PayMethods = originalPayMethods
	})

	operation_setting.MinTopUp = 0.01
	operation_setting.Price = 1
	operation_setting.PayAddress = "https://epay.example.com"
	operation_setting.EpayId = "epay_merchant"
	operation_setting.EpayKey = "epay_secret"
	operation_setting.PayMethods = []map[string]string{{"name": "支付宝", "type": "alipay"}}
	mustUpdateOptionValue(t, "EpayEnabled", "true")

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/amount", map[string]any{
		"amount": 0.01,
	}, 1)

	RequestAmount(ctx)

	var response amountQuoteResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode amount quote response: %v", err)
	}
	if response.Message != "success" {
		t.Fatalf("expected cent-level topup quote to succeed, got %s", recorder.Body.String())
	}
	if response.Data != "0.01" {
		t.Fatalf("expected 0.01 * 1 quote to be 0.01, got %#v", response.Data)
	}
}
