package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	CryptoNetworkTronTRC20 = "tron_trc20"
	CryptoNetworkBSCERC20  = "bsc_erc20"

	CryptoTokenUSDT = "USDT"

	CryptoPaymentStatusPending   = "pending"
	CryptoPaymentStatusDetected  = "detected"
	CryptoPaymentStatusConfirmed = "confirmed"
	CryptoPaymentStatusSuccess   = "success"
	CryptoPaymentStatusExpired   = "expired"
	CryptoPaymentStatusUnderpaid = "underpaid"
	CryptoPaymentStatusOverpaid  = "overpaid"
	CryptoPaymentStatusAmbiguous = "ambiguous"
	CryptoPaymentStatusLatePaid  = "late_paid"
	CryptoPaymentStatusFailed    = "failed"

	CryptoTransactionStatusSeen      = "seen"
	CryptoTransactionStatusConfirmed = "confirmed"
	CryptoTransactionStatusIgnored   = "ignored"
	CryptoTransactionStatusOrphaned  = "orphaned"
)

var (
	ErrCryptoInvalidAmount       = errors.New("invalid crypto payment amount")
	ErrCryptoInvalidSuffix       = errors.New("invalid crypto payment suffix")
	ErrCryptoOrderNotFound       = errors.New("crypto payment order not found")
	ErrCryptoOrderStatusInvalid  = errors.New("crypto payment order status invalid")
	ErrCryptoTransactionMismatch = errors.New("crypto transaction evidence mismatch")
	ErrCryptoAmountCollision     = errors.New("crypto payment amount collision")
)

type CryptoPaymentOrder struct {
	Id                    int    `json:"id"`
	TopUpId               int    `json:"topup_id" gorm:"uniqueIndex"`
	TradeNo               string `json:"trade_no" gorm:"uniqueIndex;type:varchar(255)"`
	UserId                int    `json:"user_id" gorm:"index"`
	Network               string `json:"network" gorm:"type:varchar(32);index"`
	TokenSymbol           string `json:"token_symbol" gorm:"type:varchar(16)"`
	TokenContract         string `json:"token_contract" gorm:"type:varchar(128);index"`
	TokenDecimals         int    `json:"token_decimals"`
	ReceiveAddress        string `json:"receive_address" gorm:"type:varchar(128);index"`
	BaseAmount            string `json:"base_amount" gorm:"type:varchar(64)"`
	PayAmount             string `json:"pay_amount" gorm:"type:varchar(64)"`
	PayAmountBaseUnits    string `json:"pay_amount_base_units" gorm:"type:varchar(128);index"`
	UniqueSuffix          int    `json:"unique_suffix"`
	ExpiresAt             int64  `json:"expires_at" gorm:"index"`
	RequiredConfirmations int    `json:"required_confirmations"`
	Status                string `json:"status" gorm:"type:varchar(32);index"`
	MatchedTxHash         string `json:"matched_tx_hash" gorm:"type:varchar(128);index"`
	MatchedLogIndex       int    `json:"matched_log_index" gorm:"default:-1"`
	DetectedAt            int64  `json:"detected_at"`
	ConfirmedAt           int64  `json:"confirmed_at"`
	CompletedAt           int64  `json:"completed_at"`
	CreateTime            int64  `json:"create_time"`
	UpdateTime            int64  `json:"update_time"`
}

type CryptoPaymentTransaction struct {
	Id              int    `json:"id"`
	Network         string `json:"network" gorm:"type:varchar(32);uniqueIndex:idx_crypto_tx_event"`
	TxHash          string `json:"tx_hash" gorm:"type:varchar(128);uniqueIndex:idx_crypto_tx_event"`
	LogIndex        int    `json:"log_index" gorm:"uniqueIndex:idx_crypto_tx_event"`
	BlockNumber     int64  `json:"block_number" gorm:"index"`
	BlockTimestamp  int64  `json:"block_timestamp"`
	FromAddress     string `json:"from_address" gorm:"type:varchar(128);index"`
	ToAddress       string `json:"to_address" gorm:"type:varchar(128);index"`
	TokenContract   string `json:"token_contract" gorm:"type:varchar(128);index"`
	TokenSymbol     string `json:"token_symbol" gorm:"type:varchar(16)"`
	TokenDecimals   int    `json:"token_decimals"`
	Amount          string `json:"amount" gorm:"type:varchar(64)"`
	AmountBaseUnits string `json:"amount_base_units" gorm:"type:varchar(128);index"`
	Confirmations   int64  `json:"confirmations"`
	Status          string `json:"status" gorm:"type:varchar(32);index"`
	MatchedOrderId  int    `json:"matched_order_id" gorm:"index"`
	RawPayload      string `json:"raw_payload" gorm:"type:text"`
	CreateTime      int64  `json:"create_time"`
	UpdateTime      int64  `json:"update_time"`
}

type CryptoScannerState struct {
	Network            string `json:"network" gorm:"primaryKey;type:varchar(32)"`
	LastScannedBlock   int64  `json:"last_scanned_block"`
	LastFinalizedBlock int64  `json:"last_finalized_block"`
	UpdatedAt          int64  `json:"updated_at"`
}

func CryptoTopUpInitialStatus() string {
	return common.TopUpStatusPending
}

func IsActiveCryptoOrderStatus(status string) bool {
	switch status {
	case CryptoPaymentStatusPending, CryptoPaymentStatusDetected, CryptoPaymentStatusConfirmed:
		return true
	default:
		return false
	}
}

func (o *CryptoPaymentOrder) IsExpired(now time.Time) bool {
	if o == nil || o.ExpiresAt <= 0 {
		return false
	}
	return now.Unix() > o.ExpiresAt
}

func CryptoPayAmountFromSuffix(baseAmount decimal.Decimal, tokenDecimals int, suffix int) (string, string, error) {
	if baseAmount.LessThanOrEqual(decimal.Zero) || tokenDecimals < 6 {
		return "", "", ErrCryptoInvalidAmount
	}
	if suffix < 1 || suffix > 9999 {
		return "", "", ErrCryptoInvalidSuffix
	}
	payAmount := baseAmount.Add(decimal.NewFromInt(int64(suffix)).Div(decimal.NewFromInt(1_000_000)))
	payDisplay := payAmount.StringFixed(6)
	unitMultiplier := decimal.NewFromInt(10).Pow(decimal.NewFromInt(int64(tokenDecimals)))
	baseUnits := payAmount.Mul(unitMultiplier).Round(0)
	return payDisplay, baseUnits.StringFixed(0), nil
}

func NormalizeCryptoNetwork(network string) string {
	return strings.ToLower(strings.TrimSpace(network))
}

func cryptoNow() int64 {
	return time.Now().Unix()
}

func cryptoRefCol(column string) string {
	if common.UsingPostgreSQL {
		return fmt.Sprintf("\"%s\"", column)
	}
	return fmt.Sprintf("`%s`", column)
}

type CreateCryptoTopUpOrderInput struct {
	UserID                int
	Network               string
	Amount                int64
	ReceiveAddress        string
	TokenContract         string
	TokenDecimals         int
	RequiredConfirmations int
	ExpireMinutes         int
	SuffixMax             int
	Now                   time.Time
	SuffixGenerator       func(max int) int
}

func CreateCryptoTopUpOrder(input CreateCryptoTopUpOrderInput) (*CryptoPaymentOrder, error) {
	if input.UserID <= 0 || input.Amount <= 0 || input.TokenDecimals < 6 || strings.TrimSpace(input.ReceiveAddress) == "" || strings.TrimSpace(input.TokenContract) == "" {
		return nil, ErrCryptoInvalidAmount
	}
	if input.ExpireMinutes <= 0 {
		input.ExpireMinutes = 10
	}
	if input.RequiredConfirmations <= 0 {
		input.RequiredConfirmations = 20
	}
	if input.SuffixMax <= 0 || input.SuffixMax > 9999 {
		input.SuffixMax = 9999
	}
	if input.Now.IsZero() {
		input.Now = time.Now()
	}
	if input.SuffixGenerator == nil {
		input.SuffixGenerator = func(max int) int {
			return common.GetRandomInt(max) + 1
		}
	}

	var created CryptoPaymentOrder
	err := DB.Transaction(func(tx *gorm.DB) error {
		for attempt := 0; attempt < 20; attempt++ {
			suffix := input.SuffixGenerator(input.SuffixMax)
			payAmount, payBaseUnits, amountErr := CryptoPayAmountFromSuffix(decimal.NewFromInt(input.Amount), input.TokenDecimals, suffix)
			if amountErr != nil {
				return amountErr
			}

			var count int64
			if err := tx.Model(&CryptoPaymentOrder{}).
				Where("network = ? AND receive_address = ? AND pay_amount_base_units = ? AND expires_at >= ? AND status IN ?",
					NormalizeCryptoNetwork(input.Network), strings.TrimSpace(input.ReceiveAddress), payBaseUnits, input.Now.Unix(), []string{CryptoPaymentStatusPending, CryptoPaymentStatusDetected, CryptoPaymentStatusConfirmed}).
				Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				continue
			}

			tradeNo := fmt.Sprintf("CRYPTO-%d-%d-%s", input.UserID, input.Now.UnixMilli(), common.GetRandomString(6))
			payMoney, parseErr := decimal.NewFromString(payAmount)
			if parseErr != nil {
				return parseErr
			}
			topUp := &TopUp{
				UserId:        input.UserID,
				Amount:        input.Amount,
				Money:         payMoney.InexactFloat64(),
				TradeNo:       tradeNo,
				PaymentMethod: PaymentMethodCryptoUSDT,
				Currency:      CryptoTokenUSDT,
				CreateTime:    input.Now.Unix(),
				Status:        common.TopUpStatusPending,
			}
			if err := tx.Create(topUp).Error; err != nil {
				return err
			}

			created = CryptoPaymentOrder{
				TopUpId:               topUp.Id,
				TradeNo:               tradeNo,
				UserId:                input.UserID,
				Network:               NormalizeCryptoNetwork(input.Network),
				TokenSymbol:           CryptoTokenUSDT,
				TokenContract:         strings.TrimSpace(input.TokenContract),
				TokenDecimals:         input.TokenDecimals,
				ReceiveAddress:        strings.TrimSpace(input.ReceiveAddress),
				BaseAmount:            decimal.NewFromInt(input.Amount).StringFixed(6),
				PayAmount:             payAmount,
				PayAmountBaseUnits:    payBaseUnits,
				UniqueSuffix:          suffix,
				ExpiresAt:             input.Now.Add(time.Duration(input.ExpireMinutes) * time.Minute).Unix(),
				RequiredConfirmations: input.RequiredConfirmations,
				Status:                CryptoPaymentStatusPending,
				MatchedLogIndex:       -1,
				CreateTime:            input.Now.Unix(),
				UpdateTime:            input.Now.Unix(),
			}
			return tx.Create(&created).Error
		}
		return ErrCryptoAmountCollision
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}

func GetCryptoPaymentOrderByTradeNo(tradeNo string) *CryptoPaymentOrder {
	if strings.TrimSpace(tradeNo) == "" {
		return nil
	}
	var order CryptoPaymentOrder
	if err := DB.Where("trade_no = ?", strings.TrimSpace(tradeNo)).First(&order).Error; err != nil {
		return nil
	}
	return &order
}

func GetCryptoOrderConfirmations(orderID int) int64 {
	if orderID <= 0 {
		return 0
	}
	var tx CryptoPaymentTransaction
	if err := DB.Where("matched_order_id = ?", orderID).Order("id desc").First(&tx).Error; err != nil {
		return 0
	}
	return tx.Confirmations
}

func ExpireCryptoPaymentOrderIfNeeded(order *CryptoPaymentOrder, now time.Time) (*CryptoPaymentOrder, error) {
	if order == nil || order.Status != CryptoPaymentStatusPending || !order.IsExpired(now) {
		return order, nil
	}
	order.Status = CryptoPaymentStatusExpired
	order.UpdateTime = now.Unix()
	if err := DB.Save(order).Error; err != nil {
		return nil, err
	}
	return order, nil
}
