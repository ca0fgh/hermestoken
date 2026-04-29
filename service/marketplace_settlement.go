package service

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/logger"
	"github.com/ca0fgh/hermestoken/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type MarketplaceSettlementListInput struct {
	SellerUserID int
	Status       string
	SourceType   string
	CredentialID int
}

type MarketplaceIncomeSummary struct {
	PendingIncome     int64 `json:"pending_income"`
	AvailableIncome   int64 `json:"available_income"`
	BlockedIncome     int64 `json:"blocked_income"`
	ReversedIncome    int64 `json:"reversed_income"`
	WithdrawnIncome   int64 `json:"withdrawn_income"`
	TotalSellerIncome int64 `json:"total_seller_income"`
	SettlementCount   int64 `json:"settlement_count"`
}

type MarketplaceSettlementReleaseResult struct {
	ReleasedCount  int   `json:"released_count"`
	ReleasedIncome int64 `json:"released_income"`
}

type marketplaceSettlementReleaseSideEffect struct {
	Created      bool
	SellerUserID int
	SellerIncome int64
	SourceType   string
	CredentialID int
}

func ListSellerMarketplaceSettlements(input MarketplaceSettlementListInput, startIdx int, pageSize int) ([]model.MarketplaceSettlement, int64, error) {
	if err := validateMarketplaceSettlementListInput(input); err != nil {
		return nil, 0, err
	}
	query := applyMarketplaceSettlementListFilter(model.DB.Model(&model.MarketplaceSettlement{}), input)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var items []model.MarketplaceSettlement
	if err := query.Order("id desc").Limit(pageSize).Offset(startIdx).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func GetSellerMarketplaceIncomeSummary(sellerUserID int) (MarketplaceIncomeSummary, error) {
	if err := validateMarketplaceEnabled(); err != nil {
		return MarketplaceIncomeSummary{}, err
	}
	if sellerUserID <= 0 {
		return MarketplaceIncomeSummary{}, errors.New("seller user id is required")
	}

	summary := MarketplaceIncomeSummary{}
	query := model.DB.Model(&model.MarketplaceSettlement{}).Where("seller_user_id = ?", sellerUserID)
	if err := query.Count(&summary.SettlementCount).Error; err != nil {
		return MarketplaceIncomeSummary{}, err
	}

	var rows []struct {
		Status string
		Income int64
	}
	if err := query.Select("status, COALESCE(SUM(seller_income), 0) AS income").Group("status").Scan(&rows).Error; err != nil {
		return MarketplaceIncomeSummary{}, err
	}
	for _, row := range rows {
		summary.TotalSellerIncome += row.Income
		switch row.Status {
		case model.MarketplaceSettlementStatusPending:
			summary.PendingIncome = row.Income
		case model.MarketplaceSettlementStatusAvailable:
			summary.AvailableIncome = row.Income
		case model.MarketplaceSettlementStatusBlocked:
			summary.BlockedIncome = row.Income
		case model.MarketplaceSettlementStatusReversed:
			summary.ReversedIncome = row.Income
		case model.MarketplaceSettlementStatusWithdrawn:
			summary.WithdrawnIncome = row.Income
		}
	}
	return summary, nil
}

func ReleaseSellerAvailableMarketplaceSettlements(sellerUserID int, now int64, limit int) (MarketplaceSettlementReleaseResult, error) {
	if err := validateMarketplaceEnabled(); err != nil {
		return MarketplaceSettlementReleaseResult{}, err
	}
	if sellerUserID <= 0 {
		return MarketplaceSettlementReleaseResult{}, errors.New("seller user id is required")
	}
	if now <= 0 {
		now = common.GetTimestamp()
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	var result MarketplaceSettlementReleaseResult
	err := model.DB.Transaction(func(tx *gorm.DB) error {
		var settlements []model.MarketplaceSettlement
		if err := marketplaceForUpdate(tx).
			Where("seller_user_id = ? AND status = ? AND available_at <= ?", sellerUserID, model.MarketplaceSettlementStatusPending, now).
			Order("id asc").
			Limit(limit).
			Find(&settlements).Error; err != nil {
			return err
		}
		if len(settlements) == 0 {
			return nil
		}

		ids := make([]int, 0, len(settlements))
		var releasedIncome int64
		for _, settlement := range settlements {
			if settlement.SellerIncome <= 0 {
				continue
			}
			ids = append(ids, settlement.ID)
			releasedIncome += settlement.SellerIncome
		}
		if len(ids) == 0 || releasedIncome <= 0 {
			return nil
		}
		maxInt := int64(^uint(0) >> 1)
		if releasedIncome > maxInt {
			return errors.New("released marketplace income exceeds supported user quota range")
		}

		var seller model.User
		if err := marketplaceForUpdate(tx).First(&seller, sellerUserID).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.MarketplaceSettlement{}).
			Where("id IN ? AND seller_user_id = ? AND status = ?", ids, sellerUserID, model.MarketplaceSettlementStatusPending).
			Update("status", model.MarketplaceSettlementStatusAvailable).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.User{}).
			Where("id = ?", sellerUserID).
			Update("quota", gorm.Expr("quota + ?", int(releasedIncome))).Error; err != nil {
			return err
		}

		result.ReleasedCount = len(ids)
		result.ReleasedIncome = releasedIncome
		return nil
	})
	if err != nil {
		return MarketplaceSettlementReleaseResult{}, err
	}
	if result.ReleasedIncome > 0 {
		_, _ = model.GetUserQuota(sellerUserID, true)
		model.RecordLog(sellerUserID, model.LogTypeSystem, fmt.Sprintf("市场收益释放 %s，已转入可用余额", logger.LogQuota(int(result.ReleasedIncome))))
	}
	return result, nil
}

func createReleasedMarketplaceSettlementTx(tx *gorm.DB, settlement *model.MarketplaceSettlement) (bool, error) {
	if settlement == nil {
		return false, nil
	}
	if settlement.BuyerCharge <= 0 {
		return false, nil
	}
	if settlement.SellerUserID <= 0 {
		return false, errors.New("seller user id is required")
	}
	if strings.TrimSpace(settlement.RequestID) == "" {
		return false, errors.New("marketplace settlement request id is required")
	}
	if settlement.SellerIncome < 0 {
		return false, errors.New("marketplace seller income must not be negative")
	}
	if settlement.AvailableAt <= 0 {
		settlement.AvailableAt = common.GetTimestamp()
	}
	settlement.Status = model.MarketplaceSettlementStatusAvailable

	if settlement.SellerIncome > 0 {
		maxInt := int64(^uint(0) >> 1)
		if settlement.SellerIncome > maxInt {
			return false, errors.New("released marketplace income exceeds supported user quota range")
		}
	}

	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "request_id"}},
		DoNothing: true,
	}).Create(settlement)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	if settlement.SellerIncome <= 0 {
		return true, nil
	}

	update := tx.Model(&model.User{}).
		Where("id = ?", settlement.SellerUserID).
		Update("quota", gorm.Expr("quota + ?", int(settlement.SellerIncome)))
	if update.Error != nil {
		return false, update.Error
	}
	if update.RowsAffected != 1 {
		return false, errors.New("seller user not found")
	}
	return true, nil
}

func newMarketplaceSettlementReleaseSideEffect(settlement *model.MarketplaceSettlement, created bool) marketplaceSettlementReleaseSideEffect {
	if settlement == nil {
		return marketplaceSettlementReleaseSideEffect{}
	}
	return marketplaceSettlementReleaseSideEffect{
		Created:      created,
		SellerUserID: settlement.SellerUserID,
		SellerIncome: settlement.SellerIncome,
		SourceType:   settlement.SourceType,
		CredentialID: settlement.CredentialID,
	}
}

func applyMarketplaceSettlementReleaseSideEffect(effect marketplaceSettlementReleaseSideEffect) {
	if !effect.Created || effect.SellerUserID <= 0 || effect.SellerIncome <= 0 {
		return
	}
	_, _ = model.GetUserQuota(effect.SellerUserID, true)
	model.RecordLog(effect.SellerUserID, model.LogTypeSystem, marketplaceSettlementReleaseLogContent(effect))
}

func marketplaceSettlementReleaseLogContent(effect marketplaceSettlementReleaseSideEffect) string {
	prefix := "市场收益实时结算"
	switch effect.SourceType {
	case marketplaceSettlementSourceFixedOrderFill:
		prefix = "市场买断收益实时结算"
	case marketplaceSettlementSourceFixedOrderFinal:
		prefix = "市场买断剩余额度结算"
	case marketplaceSettlementSourcePoolFill:
		prefix = "市场订单池收益实时结算"
	}
	if effect.CredentialID > 0 {
		return fmt.Sprintf("%s %s，托管Key %d", prefix, logger.LogQuota(int(effect.SellerIncome)), effect.CredentialID)
	}
	return fmt.Sprintf("%s %s", prefix, logger.LogQuota(int(effect.SellerIncome)))
}

func validateMarketplaceSettlementListInput(input MarketplaceSettlementListInput) error {
	if err := validateMarketplaceEnabled(); err != nil {
		return err
	}
	if input.SellerUserID <= 0 {
		return errors.New("seller user id is required")
	}
	if strings.TrimSpace(input.Status) != "" && !isMarketplaceSettlementStatus(input.Status) {
		return fmt.Errorf("unsupported marketplace settlement status %s", input.Status)
	}
	if strings.TrimSpace(input.SourceType) != "" {
		switch input.SourceType {
		case marketplaceSettlementSourceFixedOrderFill, marketplaceSettlementSourceFixedOrderFinal, marketplaceSettlementSourcePoolFill:
		default:
			return fmt.Errorf("unsupported marketplace settlement source type %s", input.SourceType)
		}
	}
	return nil
}

func applyMarketplaceSettlementListFilter(query *gorm.DB, input MarketplaceSettlementListInput) *gorm.DB {
	query = query.Where("seller_user_id = ?", input.SellerUserID)
	if strings.TrimSpace(input.Status) != "" {
		query = query.Where("status = ?", input.Status)
	}
	if strings.TrimSpace(input.SourceType) != "" {
		query = query.Where("source_type = ?", input.SourceType)
	}
	if input.CredentialID > 0 {
		query = query.Where("credential_id = ?", input.CredentialID)
	}
	return query
}

func isMarketplaceSettlementStatus(status string) bool {
	switch status {
	case model.MarketplaceSettlementStatusPending,
		model.MarketplaceSettlementStatusAvailable,
		model.MarketplaceSettlementStatusWithdrawn,
		model.MarketplaceSettlementStatusBlocked,
		model.MarketplaceSettlementStatusReversed:
		return true
	default:
		return false
	}
}
