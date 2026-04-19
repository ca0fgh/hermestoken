package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

func TestUserCreateWithdrawal(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	if err := db.AutoMigrate(&model.UserWithdrawal{}); err != nil {
		t.Fatalf("failed to migrate user withdrawals: %v", err)
	}

	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })

	user := seedSubscriptionReferralControllerUser(t, "withdraw-controller-user", 0, dto.UserSetting{})
	if err := db.Model(&model.User{}).Where("id = ?", user.Id).Update("quota", 100000).Error; err != nil {
		t.Fatalf("failed to seed user quota: %v", err)
	}
	common.OptionMap[model.WithdrawalEnabledOptionKey] = "true"
	common.OptionMap[model.WithdrawalMinAmountOptionKey] = "10"
	common.OptionMap[model.WithdrawalInstructionOptionKey] = "manual payout"
	common.OptionMap[model.WithdrawalFeeRulesOptionKey] = `[{"min_amount":10,"max_amount":0,"fee_type":"fixed","fee_value":2,"enabled":true,"sort_order":1}]`

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/withdrawals", map[string]any{
		"amount":           100,
		"alipay_account":   "alice@example.com",
		"alipay_real_name": "Alice",
	}, user.Id)
	CreateUserWithdrawal(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success, got message: %s", response.Message)
	}
}

func TestAdminApproveRejectAndMarkPaidWithdrawal(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	if err := db.AutoMigrate(&model.UserWithdrawal{}); err != nil {
		t.Fatalf("failed to migrate user withdrawals: %v", err)
	}

	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })

	admin := seedSubscriptionReferralControllerUser(t, "withdraw-admin-user", 0, dto.UserSetting{})
	if err := db.Model(&model.User{}).Where("id = ?", admin.Id).Update("role", common.RoleRootUser).Error; err != nil {
		t.Fatalf("failed to promote admin: %v", err)
	}
	user := seedSubscriptionReferralControllerUser(t, "withdraw-target-user", 0, dto.UserSetting{})
	if err := db.Model(&model.User{}).Where("id = ?", user.Id).Update("quota", 100000).Error; err != nil {
		t.Fatalf("failed to seed withdrawal user quota: %v", err)
	}
	common.OptionMap[model.WithdrawalEnabledOptionKey] = "true"
	common.OptionMap[model.WithdrawalMinAmountOptionKey] = "10"
	common.OptionMap[model.WithdrawalFeeRulesOptionKey] = `[]`

	withdrawal, err := model.CreateUserWithdrawal(&model.CreateUserWithdrawalParams{
		UserID:         user.Id,
		Amount:         100,
		AlipayAccount:  "alice@example.com",
		AlipayRealName: "Alice",
	})
	if err != nil {
		t.Fatalf("failed to create withdrawal: %v", err)
	}

	approveCtx, approveRecorder := newAuthenticatedContext(t, http.MethodPost, fmt.Sprintf("/api/admin/withdrawals/%d/approve", withdrawal.Id), map[string]any{
		"review_note": "ok",
	}, admin.Id)
	approveCtx.Params = ginParams("id", withdrawal.Id)
	AdminApproveWithdrawal(approveCtx)
	approveResponse := decodeAPIResponse(t, approveRecorder)
	if !approveResponse.Success {
		t.Fatalf("expected approve success, got message: %s", approveResponse.Message)
	}

	rejectCtx, rejectRecorder := newAuthenticatedContext(t, http.MethodPost, fmt.Sprintf("/api/admin/withdrawals/%d/reject", withdrawal.Id), map[string]any{
		"rejection_note": "manual reject",
	}, admin.Id)
	rejectCtx.Params = ginParams("id", withdrawal.Id)
	AdminRejectWithdrawal(rejectCtx)
	rejectResponse := decodeAPIResponse(t, rejectRecorder)
	if !rejectResponse.Success {
		t.Fatalf("expected reject success, got message: %s", rejectResponse.Message)
	}

	secondWithdrawal, err := model.CreateUserWithdrawal(&model.CreateUserWithdrawalParams{
		UserID:         user.Id,
		Amount:         50,
		AlipayAccount:  "alice@example.com",
		AlipayRealName: "Alice",
	})
	if err != nil {
		t.Fatalf("failed to create second withdrawal: %v", err)
	}
	if err := model.ApproveUserWithdrawal(secondWithdrawal.Id, admin.Id, "ok"); err != nil {
		t.Fatalf("failed to approve second withdrawal: %v", err)
	}

	paidCtx, paidRecorder := newAuthenticatedContext(t, http.MethodPost, fmt.Sprintf("/api/admin/withdrawals/%d/mark-paid", secondWithdrawal.Id), map[string]any{
		"pay_receipt_no":  "ALI123",
		"pay_receipt_url": "https://example.com/receipt.png",
		"paid_note":       "done",
	}, admin.Id)
	paidCtx.Params = ginParams("id", secondWithdrawal.Id)
	AdminMarkWithdrawalPaid(paidCtx)
	paidResponse := decodeAPIResponse(t, paidRecorder)
	if !paidResponse.Success {
		t.Fatalf("expected mark-paid success, got message: %s", paidResponse.Message)
	}
}

func TestAdminListWithdrawalsKeywordMatchesUserID(t *testing.T) {
	db := setupSubscriptionControllerTestDB(t)
	if err := db.AutoMigrate(&model.UserWithdrawal{}); err != nil {
		t.Fatalf("failed to migrate user withdrawals: %v", err)
	}

	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })

	admin := seedSubscriptionReferralControllerUser(t, "withdraw-search-admin", 0, dto.UserSetting{})
	if err := db.Model(&model.User{}).Where("id = ?", admin.Id).Update("role", common.RoleRootUser).Error; err != nil {
		t.Fatalf("failed to promote admin: %v", err)
	}
	user := seedSubscriptionReferralControllerUser(t, "withdraw-search-user", 0, dto.UserSetting{})
	if err := db.Model(&model.User{}).Where("id = ?", user.Id).Update("quota", 100000).Error; err != nil {
		t.Fatalf("failed to seed withdrawal user quota: %v", err)
	}

	common.OptionMap[model.WithdrawalEnabledOptionKey] = "true"
	common.OptionMap[model.WithdrawalMinAmountOptionKey] = "10"
	common.OptionMap[model.WithdrawalFeeRulesOptionKey] = `[]`

	if _, err := model.CreateUserWithdrawal(&model.CreateUserWithdrawalParams{
		UserID:         user.Id,
		Amount:         100,
		AlipayAccount:  "search-user@example.com",
		AlipayRealName: "Search User",
	}); err != nil {
		t.Fatalf("failed to create withdrawal: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(
		t,
		http.MethodGet,
		fmt.Sprintf("/api/admin/withdrawals?keyword=%d&p=1&page_size=10", user.Id),
		nil,
		admin.Id,
	)
	AdminListWithdrawals(ctx)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected success, got message: %s", response.Message)
	}

	var page struct {
		Items []model.UserWithdrawal `json:"items"`
	}
	if err := common.Unmarshal(response.Data, &page); err != nil {
		t.Fatalf("failed to decode withdrawal page: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("expected 1 matched withdrawal, got %d", len(page.Items))
	}
	if page.Items[0].UserId != user.Id {
		t.Fatalf("expected user_id %d, got %d", user.Id, page.Items[0].UserId)
	}
}

func ginParams(key string, id int) gin.Params {
	return gin.Params{{Key: key, Value: strconv.Itoa(id)}}
}
