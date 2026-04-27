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
		Quota:  300,
		Other:  `{}`,
	}).Error; err != nil {
		t.Fatalf("failed to seed wallet consume log: %v", err)
	}
	if err := db.Create(&model.Log{
		UserId: user.Id,
		Type:   model.LogTypeConsume,
		Quota:  900,
		Other:  `{"billing_source":"subscription"}`,
	}).Error; err != nil {
		t.Fatalf("failed to seed subscription consume log: %v", err)
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
	if item.WalletAmountUsed != 300 {
		t.Fatalf("wallet amount used = %d, want 300", item.WalletAmountUsed)
	}
	if item.SubscriptionAmountTotal != 1000 || item.SubscriptionAmountUsed != 250 {
		t.Fatalf("subscription fields = used:%d total:%d, want 250/1000", item.SubscriptionAmountUsed, item.SubscriptionAmountTotal)
	}
	if item.SubscriptionQuotaUnlimited {
		t.Fatal("subscription unlimited = true, want false")
	}
}
