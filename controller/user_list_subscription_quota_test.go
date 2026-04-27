package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
)

func TestGetAllUsersReturnsWalletAndSubscriptionQuotaSeparately(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	now := common.GetTimestamp()

	user := seedSubscriptionReferralControllerUser(t, "quota-list-user", 0, dto.UserSetting{})
	user.Quota = 300
	user.UsedQuota = 1200
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("failed to seed wallet quota: %v", err)
	}

	plan := seedSubscriptionPlan(t, db, "quota-list-plan")
	if err := db.Create(&model.UserSubscription{
		UserId:      user.Id,
		PlanId:      plan.Id,
		AmountTotal: 1000,
		AmountUsed:  250,
		StartTime:   now - 100,
		EndTime:     now + 100,
		Status:      "active",
		Source:      "order",
	}).Error; err != nil {
		t.Fatalf("failed to seed active subscription: %v", err)
	}
	if err := db.Create(&model.Log{
		UserId: user.Id,
		Type:   model.LogTypeConsume,
		Quota:  400,
		Other:  `{}`,
	}).Error; err != nil {
		t.Fatalf("failed to seed wallet consume log: %v", err)
	}
	if err := db.Create(&model.Log{
		UserId: user.Id,
		Type:   model.LogTypeRefund,
		Quota:  100,
		Other:  `{}`,
	}).Error; err != nil {
		t.Fatalf("failed to seed wallet refund log: %v", err)
	}
	if err := db.Create(&model.Log{
		UserId: user.Id,
		Type:   model.LogTypeConsume,
		Quota:  900,
		Other:  `{"billing_source": "subscription"}`,
	}).Error; err != nil {
		t.Fatalf("failed to seed subscription consume log: %v", err)
	}
	if err := db.Create(&model.Log{
		UserId: user.Id,
		Type:   model.LogTypeRefund,
		Quota:  500,
		Other:  `{"billing_source": "subscription"}`,
	}).Error; err != nil {
		t.Fatalf("failed to seed subscription refund log: %v", err)
	}
	if err := db.Create(&model.SubscriptionOrder{
		UserId:        user.Id,
		PlanId:        plan.Id,
		Money:         2.00,
		PaymentMoney:  2.00,
		TradeNo:       "wallet-success-order",
		PaymentMethod: model.PaymentMethodWallet,
		Status:        common.TopUpStatusSuccess,
		CreateTime:    now - 50,
		CompleteTime:  now - 40,
		Quantity:      1,
	}).Error; err != nil {
		t.Fatalf("failed to seed wallet subscription order: %v", err)
	}
	if err := db.Create(&model.SubscriptionOrder{
		UserId:        user.Id,
		PlanId:        plan.Id,
		Money:         99.00,
		PaymentMoney:  1.50,
		TradeNo:       "wallet-payment-money-order",
		PaymentMethod: model.PaymentMethodWallet,
		Status:        common.TopUpStatusSuccess,
		CreateTime:    now - 45,
		CompleteTime:  now - 35,
		Quantity:      1,
	}).Error; err != nil {
		t.Fatalf("failed to seed payment-money wallet subscription order: %v", err)
	}
	if err := db.Create(&model.SubscriptionOrder{
		UserId:        user.Id,
		PlanId:        plan.Id,
		Money:         9.00,
		PaymentMoney:  9.00,
		TradeNo:       "wallet-pending-order",
		PaymentMethod: model.PaymentMethodWallet,
		Status:        common.TopUpStatusPending,
		CreateTime:    now - 30,
		Quantity:      1,
	}).Error; err != nil {
		t.Fatalf("failed to seed pending wallet subscription order: %v", err)
	}
	if err := db.Create(&model.SubscriptionOrder{
		UserId:        user.Id,
		PlanId:        plan.Id,
		Money:         8.00,
		PaymentMoney:  8.00,
		TradeNo:       "stripe-success-order",
		PaymentMethod: model.PaymentMethodStripe,
		Status:        common.TopUpStatusSuccess,
		CreateTime:    now - 20,
		CompleteTime:  now - 10,
		Quantity:      1,
	}).Error; err != nil {
		t.Fatalf("failed to seed non-wallet subscription order: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/?p=1&page_size=10", nil, user.Id)
	GetAllUsers(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success response, got message: %s", response.Message)
	}

	var page struct {
		Items []struct {
			Id                         int   `json:"id"`
			Quota                      int   `json:"quota"`
			UsedQuota                  int   `json:"used_quota"`
			WalletAmountUsed           int64 `json:"wallet_amount_used"`
			SubscriptionAmountTotal    int64 `json:"subscription_amount_total"`
			SubscriptionAmountUsed     int64 `json:"subscription_amount_used"`
			SubscriptionQuotaUnlimited bool  `json:"subscription_quota_unlimited"`
		} `json:"items"`
	}
	if err := common.Unmarshal(response.Data, &page); err != nil {
		t.Fatalf("failed to decode user page: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("items length = %d, want 1", len(page.Items))
	}
	item := page.Items[0]
	if item.Quota != 300 || item.UsedQuota != 1200 {
		t.Fatalf("wallet fields = quota:%d used:%d, want 300/1200", item.Quota, item.UsedQuota)
	}
	expectedWalletUsed := int64(300) + int64(3.50*common.QuotaPerUnit)
	if item.WalletAmountUsed != expectedWalletUsed {
		t.Fatalf("wallet amount used = %d, want %d", item.WalletAmountUsed, expectedWalletUsed)
	}
	if item.SubscriptionAmountTotal != 1000 || item.SubscriptionAmountUsed != 250 {
		t.Fatalf("subscription fields = used:%d total:%d, want 250/1000", item.SubscriptionAmountUsed, item.SubscriptionAmountTotal)
	}
	if item.SubscriptionQuotaUnlimited {
		t.Fatal("subscription unlimited = true, want false")
	}
}
