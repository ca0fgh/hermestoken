package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	UserWithdrawalStatusPending  = "pending"
	UserWithdrawalStatusApproved = "approved"
	UserWithdrawalStatusPaid     = "paid"
	UserWithdrawalStatusRejected = "rejected"

	WithdrawalChannelAlipay = "alipay"

	WithdrawalFeeTypeFixed = "fixed"
	WithdrawalFeeTypeRatio = "ratio"
)

type UserWithdrawal struct {
	Id                     int     `json:"id"`
	UserId                 int     `json:"user_id" gorm:"index;not null"`
	Username               string  `json:"username,omitempty" gorm:"column:username;->;-:migration"`
	TradeNo                string  `json:"trade_no" gorm:"uniqueIndex;type:varchar(64);not null"`
	Channel                string  `json:"channel" gorm:"type:varchar(32);not null;default:'alipay'"`
	Currency               string  `json:"currency" gorm:"type:varchar(16);not null"`
	ExchangeRateSnapshot   float64 `json:"exchange_rate_snapshot" gorm:"type:decimal(18,6);not null;default:1"`
	AvailableQuotaSnapshot int     `json:"available_quota_snapshot" gorm:"not null;default:0"`
	FrozenQuotaSnapshot    int     `json:"frozen_quota_snapshot" gorm:"not null;default:0"`
	ApplyAmount            float64 `json:"apply_amount" gorm:"type:decimal(18,2);not null;default:0"`
	FeeAmount              float64 `json:"fee_amount" gorm:"type:decimal(18,2);not null;default:0"`
	NetAmount              float64 `json:"net_amount" gorm:"type:decimal(18,2);not null;default:0"`
	ApplyQuota             int     `json:"apply_quota" gorm:"not null;default:0"`
	FeeQuota               int     `json:"fee_quota" gorm:"not null;default:0"`
	NetQuota               int     `json:"net_quota" gorm:"not null;default:0"`
	AlipayAccount          string  `json:"alipay_account" gorm:"type:varchar(128);not null"`
	AlipayRealName         string  `json:"alipay_real_name" gorm:"type:varchar(64);default:''"`
	Status                 string  `json:"status" gorm:"type:varchar(32);index;not null;default:'pending'"`
	FeeRuleSnapshotJSON    string  `json:"fee_rule_snapshot_json" gorm:"type:text"`
	ReviewAdminId          int     `json:"review_admin_id" gorm:"index;not null;default:0"`
	RejectedAdminId        int     `json:"rejected_admin_id" gorm:"index;not null;default:0"`
	PaidAdminId            int     `json:"paid_admin_id" gorm:"index;not null;default:0"`
	ReviewNote             string  `json:"review_note" gorm:"type:text"`
	RejectionNote          string  `json:"rejection_note" gorm:"type:text"`
	PayReceiptNo           string  `json:"pay_receipt_no" gorm:"type:varchar(128);default:''"`
	PayReceiptUrl          string  `json:"pay_receipt_url" gorm:"type:text"`
	PaidNote               string  `json:"paid_note" gorm:"type:text"`
	ReviewedAt             int64   `json:"reviewed_at" gorm:"bigint"`
	PaidAt                 int64   `json:"paid_at" gorm:"bigint"`
	CreatedAt              int64   `json:"created_at" gorm:"bigint"`
	UpdatedAt              int64   `json:"updated_at" gorm:"bigint"`
}

type CreateUserWithdrawalParams struct {
	UserID         int
	Amount         float64
	AlipayAccount  string
	AlipayRealName string
}

type WithdrawalFeeRule struct {
	MinAmount float64 `json:"min_amount"`
	MaxAmount float64 `json:"max_amount"`
	FeeType   string  `json:"fee_type"`
	FeeValue  float64 `json:"fee_value"`
	MinFee    float64 `json:"min_fee"`
	MaxFee    float64 `json:"max_fee"`
	Enabled   bool    `json:"enabled"`
	SortOrder int     `json:"sort_order"`
}

type MarkUserWithdrawalPaidParams struct {
	PayReceiptNo  string
	PayReceiptURL string
	PaidNote      string
}

type AdminWithdrawalFilter struct {
	Status        string
	Keyword       string
	UserID        int
	Username      string
	AlipayAccount string
	DateFrom      int64
	DateTo        int64
}

type UserWithdrawalConfigView struct {
	Enabled           bool                `json:"enabled"`
	MinAmount         float64             `json:"min_amount"`
	Instruction       string              `json:"instruction"`
	FeeRules          []WithdrawalFeeRule `json:"fee_rules"`
	HasOpenWithdrawal bool                `json:"has_open_withdrawal"`
	Currency          string              `json:"currency"`
	CurrencySymbol    string              `json:"currency_symbol"`
	QuotaDisplayType  string              `json:"quota_display_type"`
	ExchangeRate      float64             `json:"exchange_rate"`
	AvailableQuota    int                 `json:"available_quota"`
	FrozenQuota       int                 `json:"frozen_quota"`
	AvailableAmount   float64             `json:"available_amount"`
	FrozenAmount      float64             `json:"frozen_amount"`
}

func (w *UserWithdrawal) normalize() {
	w.TradeNo = strings.TrimSpace(w.TradeNo)
	w.Channel = strings.TrimSpace(w.Channel)
	w.Currency = strings.TrimSpace(w.Currency)
	w.AlipayAccount = strings.TrimSpace(w.AlipayAccount)
	w.AlipayRealName = strings.TrimSpace(w.AlipayRealName)
	w.Status = strings.TrimSpace(w.Status)
	w.ReviewNote = strings.TrimSpace(w.ReviewNote)
	w.RejectionNote = strings.TrimSpace(w.RejectionNote)
	w.PayReceiptNo = strings.TrimSpace(w.PayReceiptNo)
	w.PayReceiptUrl = strings.TrimSpace(w.PayReceiptUrl)
	w.PaidNote = strings.TrimSpace(w.PaidNote)
}

func (w *UserWithdrawal) Validate() error {
	w.normalize()
	if w.UserId <= 0 {
		return errors.New("user id is required")
	}
	if w.TradeNo == "" {
		return errors.New("trade no is required")
	}
	if w.Channel == "" {
		return errors.New("channel is required")
	}
	if w.Channel != WithdrawalChannelAlipay {
		return errors.New("invalid withdrawal channel")
	}
	if w.Currency == "" {
		return errors.New("currency is required")
	}
	if w.AlipayAccount == "" {
		return errors.New("alipay account is required")
	}
	if w.AlipayRealName == "" {
		return errors.New("alipay real name is required")
	}
	switch w.Status {
	case UserWithdrawalStatusPending, UserWithdrawalStatusApproved, UserWithdrawalStatusPaid, UserWithdrawalStatusRejected:
	default:
		return errors.New("invalid withdrawal status")
	}
	if w.ApplyAmount < 0 || w.FeeAmount < 0 || w.NetAmount < 0 {
		return errors.New("invalid withdrawal amount")
	}
	if w.ApplyQuota < 0 || w.FeeQuota < 0 || w.NetQuota < 0 {
		return errors.New("invalid withdrawal quota")
	}
	return nil
}

func (w *UserWithdrawal) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	w.CreatedAt = now
	w.UpdatedAt = now
	if err := w.Validate(); err != nil {
		return err
	}
	return nil
}

func (w *UserWithdrawal) BeforeUpdate(tx *gorm.DB) error {
	w.UpdatedAt = common.GetTimestamp()
	return w.Validate()
}

func GetUserWithdrawalConfigView(userID int) (UserWithdrawalConfigView, error) {
	user, err := GetUserById(userID, true)
	if err != nil {
		return UserWithdrawalConfigView{}, err
	}

	setting := GetUserWithdrawalSetting()
	currency := GetUserWithdrawalCurrencyConfig()
	hasOpen, err := userHasOpenWithdrawals(userID)
	if err != nil {
		return UserWithdrawalConfigView{}, err
	}

	return UserWithdrawalConfigView{
		Enabled:           setting.Enabled,
		MinAmount:         setting.MinAmount,
		Instruction:       setting.Instruction,
		FeeRules:          setting.FeeRules,
		HasOpenWithdrawal: hasOpen,
		Currency:          currency.Currency,
		CurrencySymbol:    currency.Symbol,
		QuotaDisplayType:  currency.Type,
		ExchangeRate:      currency.UsdToCurrencyRate,
		AvailableQuota:    user.Quota,
		FrozenQuota:       user.WithdrawFrozenQuota,
		AvailableAmount:   quotaToCurrencyAmount(user.Quota, currency),
		FrozenAmount:      quotaToCurrencyAmount(user.WithdrawFrozenQuota, currency),
	}, nil
}

func CreateUserWithdrawal(params *CreateUserWithdrawalParams) (*UserWithdrawal, error) {
	if params == nil {
		return nil, errors.New("withdrawal params are required")
	}

	setting := GetUserWithdrawalSetting()
	if !setting.Enabled {
		return nil, errors.New("withdrawal is disabled")
	}

	amount := decimal.NewFromFloat(params.Amount).Round(2)
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, errors.New("invalid withdrawal amount")
	}
	if amount.LessThan(decimal.NewFromFloat(setting.MinAmount)) {
		return nil, fmt.Errorf("withdrawal amount must be at least %.2f", setting.MinAmount)
	}

	currency := GetUserWithdrawalCurrencyConfig()
	applyQuota, err := currencyAmountToQuota(amount.InexactFloat64(), currency)
	if err != nil {
		return nil, err
	}
	if applyQuota <= 0 {
		return nil, errors.New("invalid withdrawal quota")
	}

	matchedRule, feeAmount, err := calculateWithdrawalFeeAmount(amount, setting.FeeRules)
	if err != nil {
		return nil, err
	}
	netAmount := amount.Sub(feeAmount).Round(2)
	if !netAmount.GreaterThan(decimal.Zero) {
		return nil, errors.New("net withdrawal amount must be greater than zero")
	}

	feeQuota, err := currencyAmountToQuota(feeAmount.InexactFloat64(), currency)
	if err != nil {
		return nil, err
	}
	netQuota, err := currencyAmountToQuota(netAmount.InexactFloat64(), currency)
	if err != nil {
		return nil, err
	}

	var created UserWithdrawal
	err = DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&user, params.UserID).Error; err != nil {
			return err
		}

		openCount, err := countOpenUserWithdrawalsTx(tx, params.UserID)
		if err != nil {
			return err
		}
		if openCount > 0 {
			return errors.New("existing withdrawal is still pending")
		}

		if user.Quota < applyQuota {
			return errors.New("insufficient wallet balance")
		}

		tradeNo, err := generateUserWithdrawalTradeNo(tx)
		if err != nil {
			return err
		}

		ruleSnapshotJSON := ""
		if matchedRule != nil {
			ruleSnapshotJSON = common.GetJsonString(matchedRule)
		}

		created = UserWithdrawal{
			UserId:                 params.UserID,
			TradeNo:                tradeNo,
			Channel:                WithdrawalChannelAlipay,
			Currency:               currency.Currency,
			ExchangeRateSnapshot:   currency.UsdToCurrencyRate,
			AvailableQuotaSnapshot: user.Quota,
			FrozenQuotaSnapshot:    user.WithdrawFrozenQuota + applyQuota,
			ApplyAmount:            amount.InexactFloat64(),
			FeeAmount:              feeAmount.InexactFloat64(),
			NetAmount:              netAmount.InexactFloat64(),
			ApplyQuota:             applyQuota,
			FeeQuota:               feeQuota,
			NetQuota:               netQuota,
			AlipayAccount:          strings.TrimSpace(params.AlipayAccount),
			AlipayRealName:         strings.TrimSpace(params.AlipayRealName),
			Status:                 UserWithdrawalStatusPending,
			FeeRuleSnapshotJSON:    ruleSnapshotJSON,
		}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}

		if err := tx.Model(&User{}).Where("id = ?", params.UserID).Updates(map[string]any{
			"quota":                 gorm.Expr("quota - ?", applyQuota),
			"withdraw_frozen_quota": gorm.Expr("withdraw_frozen_quota + ?", applyQuota),
		}).Error; err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	if common.RedisEnabled {
		if user, userErr := GetUserById(params.UserID, true); userErr == nil {
			_ = updateUserCache(*user)
		}
	}

	RecordLog(params.UserID, LogTypeSystem, fmt.Sprintf("提交提现申请 %s，申请金额 %s，手续费 %.2f，实际到账 %.2f", created.TradeNo, logger.LogQuota(created.ApplyQuota), created.FeeAmount, created.NetAmount))
	return &created, nil
}

func ApproveUserWithdrawal(id int, adminID int, note string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var withdrawal UserWithdrawal
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&withdrawal, id).Error; err != nil {
			return err
		}
		if withdrawal.Status != UserWithdrawalStatusPending {
			return errors.New("withdrawal is not pending")
		}
		now := common.GetTimestamp()
		withdrawal.Status = UserWithdrawalStatusApproved
		withdrawal.ReviewAdminId = adminID
		withdrawal.ReviewNote = strings.TrimSpace(note)
		withdrawal.ReviewedAt = now
		return tx.Save(&withdrawal).Error
	})
}

func RejectUserWithdrawal(id int, adminID int, note string) error {
	trimmedNote := strings.TrimSpace(note)
	if trimmedNote == "" {
		return errors.New("rejection note is required")
	}

	var userID int
	var applyQuota int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var withdrawal UserWithdrawal
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&withdrawal, id).Error; err != nil {
			return err
		}
		if withdrawal.Status != UserWithdrawalStatusPending && withdrawal.Status != UserWithdrawalStatusApproved {
			return errors.New("withdrawal cannot be rejected")
		}

		var user User
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&user, withdrawal.UserId).Error; err != nil {
			return err
		}

		if err := tx.Model(&User{}).Where("id = ?", user.Id).Updates(map[string]any{
			"quota":                 gorm.Expr("quota + ?", withdrawal.ApplyQuota),
			"withdraw_frozen_quota": gorm.Expr("withdraw_frozen_quota - ?", withdrawal.ApplyQuota),
		}).Error; err != nil {
			return err
		}

		now := common.GetTimestamp()
		withdrawal.Status = UserWithdrawalStatusRejected
		withdrawal.RejectedAdminId = adminID
		withdrawal.RejectionNote = trimmedNote
		if withdrawal.ReviewedAt == 0 {
			withdrawal.ReviewedAt = now
		}
		if err := tx.Save(&withdrawal).Error; err != nil {
			return err
		}

		userID = user.Id
		applyQuota = withdrawal.ApplyQuota
		return nil
	})
	if err != nil {
		return err
	}

	if common.RedisEnabled {
		if user, userErr := GetUserById(userID, true); userErr == nil {
			_ = updateUserCache(*user)
		}
	}
	RecordLog(userID, LogTypeManage, fmt.Sprintf("管理员 %d 驳回提现申请 %d，退回额度 %s", adminID, id, logger.LogQuota(applyQuota)))
	return nil
}

func MarkUserWithdrawalPaid(id int, adminID int, params MarkUserWithdrawalPaidParams) error {
	var userID int
	var applyQuota int
	err := DB.Transaction(func(tx *gorm.DB) error {
		var withdrawal UserWithdrawal
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&withdrawal, id).Error; err != nil {
			return err
		}
		if withdrawal.Status != UserWithdrawalStatusApproved {
			return errors.New("withdrawal is not approved")
		}

		var user User
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&user, withdrawal.UserId).Error; err != nil {
			return err
		}

		if err := tx.Model(&User{}).Where("id = ?", user.Id).Update("withdraw_frozen_quota", gorm.Expr("withdraw_frozen_quota - ?", withdrawal.ApplyQuota)).Error; err != nil {
			return err
		}

		now := common.GetTimestamp()
		withdrawal.Status = UserWithdrawalStatusPaid
		withdrawal.PaidAdminId = adminID
		withdrawal.PayReceiptNo = strings.TrimSpace(params.PayReceiptNo)
		withdrawal.PayReceiptUrl = strings.TrimSpace(params.PayReceiptURL)
		withdrawal.PaidNote = strings.TrimSpace(params.PaidNote)
		withdrawal.PaidAt = now
		if err := tx.Save(&withdrawal).Error; err != nil {
			return err
		}

		userID = user.Id
		applyQuota = withdrawal.ApplyQuota
		return nil
	})
	if err != nil {
		return err
	}

	if common.RedisEnabled {
		if user, userErr := GetUserById(userID, true); userErr == nil {
			_ = updateUserCache(*user)
		}
	}
	RecordLog(userID, LogTypeManage, fmt.Sprintf("管理员 %d 确认提现申请 %d 已打款，出账额度 %s", adminID, id, logger.LogQuota(applyQuota)))
	return nil
}

func ListUserWithdrawals(userID int, pageInfo *common.PageInfo) ([]*UserWithdrawal, int64, error) {
	var items []*UserWithdrawal
	var total int64
	query := DB.Model(&UserWithdrawal{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("id DESC").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&items).Error
	return items, total, err
}

func ListAdminWithdrawals(filter AdminWithdrawalFilter, pageInfo *common.PageInfo) ([]*UserWithdrawal, int64, error) {
	var items []*UserWithdrawal
	var total int64
	query := DB.Table("user_withdrawals").
		Select("user_withdrawals.*, users.username AS username").
		Joins("LEFT JOIN users ON users.id = user_withdrawals.user_id")

	if trimmedStatus := strings.TrimSpace(filter.Status); trimmedStatus != "" {
		query = query.Where("user_withdrawals.status = ?", trimmedStatus)
	}
	if filter.UserID > 0 {
		query = query.Where("user_withdrawals.user_id = ?", filter.UserID)
	}
	if trimmedUsername := strings.TrimSpace(filter.Username); trimmedUsername != "" {
		query = query.Where("users.username LIKE ?", "%"+trimmedUsername+"%")
	}
	if trimmedAlipayAccount := strings.TrimSpace(filter.AlipayAccount); trimmedAlipayAccount != "" {
		query = query.Where("user_withdrawals.alipay_account LIKE ?", "%"+trimmedAlipayAccount+"%")
	}
	if trimmedKeyword := strings.TrimSpace(filter.Keyword); trimmedKeyword != "" {
		like := "%" + trimmedKeyword + "%"
		if keywordUserID, err := strconv.Atoi(trimmedKeyword); err == nil && keywordUserID > 0 {
			query = query.Where(
				"user_withdrawals.trade_no LIKE ? OR user_withdrawals.alipay_account LIKE ? OR users.username LIKE ? OR user_withdrawals.user_id = ?",
				like,
				like,
				like,
				keywordUserID,
			)
		} else {
			query = query.Where(
				"user_withdrawals.trade_no LIKE ? OR user_withdrawals.alipay_account LIKE ? OR users.username LIKE ?",
				like,
				like,
				like,
			)
		}
	}
	if filter.DateFrom > 0 {
		query = query.Where("user_withdrawals.created_at >= ?", filter.DateFrom)
	}
	if filter.DateTo > 0 {
		query = query.Where("user_withdrawals.created_at <= ?", filter.DateTo)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Order("user_withdrawals.id DESC").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Scan(&items).Error
	return items, total, err
}

func GetUserWithdrawalByID(id int, userID int) (*UserWithdrawal, error) {
	var item UserWithdrawal
	if err := DB.Where("id = ? AND user_id = ?", id, userID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func GetAdminWithdrawalByID(id int) (*UserWithdrawal, error) {
	var item UserWithdrawal
	if err := DB.Table("user_withdrawals").
		Select("user_withdrawals.*, users.username AS username").
		Joins("LEFT JOIN users ON users.id = user_withdrawals.user_id").
		Where("user_withdrawals.id = ?", id).
		Scan(&item).Error; err != nil {
		return nil, err
	}
	if item.Id == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &item, nil
}

func userHasOpenWithdrawals(userID int) (bool, error) {
	count, err := countOpenUserWithdrawalsTx(DB, userID)
	return count > 0, err
}

func countOpenUserWithdrawalsTx(tx *gorm.DB, userID int) (int64, error) {
	var count int64
	err := tx.Model(&UserWithdrawal{}).
		Where("user_id = ? AND status IN ?", userID, []string{UserWithdrawalStatusPending, UserWithdrawalStatusApproved}).
		Count(&count).Error
	return count, err
}

func generateUserWithdrawalTradeNo(tx *gorm.DB) (string, error) {
	for i := 0; i < 5; i++ {
		tradeNo := fmt.Sprintf("WDR%s%s", time.Now().Format("20060102150405"), common.GetRandomString(6))
		var count int64
		if err := tx.Model(&UserWithdrawal{}).Where("trade_no = ?", tradeNo).Count(&count).Error; err != nil {
			return "", err
		}
		if count == 0 {
			return tradeNo, nil
		}
	}
	return "", errors.New("failed to generate withdrawal trade no")
}

func calculateWithdrawalFeeAmount(amount decimal.Decimal, rules []WithdrawalFeeRule) (*WithdrawalFeeRule, decimal.Decimal, error) {
	for i := range rules {
		rule := rules[i]
		if amount.LessThan(decimal.NewFromFloat(rule.MinAmount)) {
			continue
		}
		if rule.MaxAmount > 0 && !amount.LessThan(decimal.NewFromFloat(rule.MaxAmount)) {
			continue
		}

		feeAmount := decimal.Zero
		switch rule.FeeType {
		case WithdrawalFeeTypeFixed:
			feeAmount = decimal.NewFromFloat(rule.FeeValue)
		case WithdrawalFeeTypeRatio:
			feeAmount = amount.Mul(decimal.NewFromFloat(rule.FeeValue)).Div(decimal.NewFromInt(100))
			if rule.MinFee > 0 && feeAmount.LessThan(decimal.NewFromFloat(rule.MinFee)) {
				feeAmount = decimal.NewFromFloat(rule.MinFee)
			}
			if rule.MaxFee > 0 && feeAmount.GreaterThan(decimal.NewFromFloat(rule.MaxFee)) {
				feeAmount = decimal.NewFromFloat(rule.MaxFee)
			}
		default:
			return nil, decimal.Zero, errors.New("invalid withdrawal fee type")
		}
		feeAmount = feeAmount.Round(2)
		return &rule, feeAmount, nil
	}

	return nil, decimal.Zero, nil
}

func currencyAmountToQuota(amount float64, currency UserWithdrawalCurrencyConfig) (int, error) {
	if amount < 0 {
		return 0, errors.New("invalid amount")
	}
	if common.QuotaPerUnit <= 0 {
		return 0, errors.New("invalid quota per unit")
	}
	if currency.UsdToCurrencyRate <= 0 {
		return 0, errors.New("invalid withdrawal exchange rate")
	}

	usdAmount := decimal.NewFromFloat(amount).Div(decimal.NewFromFloat(currency.UsdToCurrencyRate))
	quota := usdAmount.Mul(decimal.NewFromFloat(common.QuotaPerUnit)).Round(0)
	return int(quota.IntPart()), nil
}

func quotaToCurrencyAmount(quota int, currency UserWithdrawalCurrencyConfig) float64 {
	if quota <= 0 || common.QuotaPerUnit <= 0 {
		return 0
	}
	if currency.UsdToCurrencyRate <= 0 {
		currency.UsdToCurrencyRate = 1
	}

	usdAmount := decimal.NewFromInt(int64(quota)).Div(decimal.NewFromFloat(common.QuotaPerUnit))
	return usdAmount.Mul(decimal.NewFromFloat(currency.UsdToCurrencyRate)).Round(2).InexactFloat64()
}

func ParseUserIDFilter(raw string) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	return strconv.Atoi(trimmed)
}
