package controller

import (
	"strconv"
	"strings"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/gin-gonic/gin"
)

type createUserWithdrawalRequest struct {
	Amount         float64 `json:"amount"`
	Channel        string  `json:"channel"`
	AlipayAccount  string  `json:"alipay_account"`
	AlipayRealName string  `json:"alipay_real_name"`
	USDTNetwork    string  `json:"usdt_network"`
	USDTAddress    string  `json:"usdt_address"`
}

type approveUserWithdrawalRequest struct {
	ReviewNote string `json:"review_note"`
}

type rejectUserWithdrawalRequest struct {
	RejectionNote string `json:"rejection_note"`
}

type markUserWithdrawalPaidRequest struct {
	PayReceiptNo  string `json:"pay_receipt_no"`
	PayReceiptURL string `json:"pay_receipt_url"`
	PaidNote      string `json:"paid_note"`
}

func GetUserWithdrawalConfig(c *gin.Context) {
	userID := c.GetInt("id")
	view, err := model.GetUserWithdrawalConfigView(userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

func CreateUserWithdrawal(c *gin.Context) {
	var req createUserWithdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "无效的提现参数")
		return
	}

	req.AlipayAccount = strings.TrimSpace(req.AlipayAccount)
	req.AlipayRealName = strings.TrimSpace(req.AlipayRealName)
	req.USDTNetwork = model.NormalizeWithdrawalUSDTNetwork(req.USDTNetwork)
	req.USDTAddress = strings.TrimSpace(req.USDTAddress)
	channel := model.NormalizeWithdrawalChannel(req.Channel)

	switch channel {
	case model.WithdrawalChannelAlipay:
		if req.AlipayAccount == "" {
			common.ApiErrorMsg(c, "支付宝账号不能为空")
			return
		}
		if req.AlipayRealName == "" {
			common.ApiErrorMsg(c, "支付宝姓名不能为空")
			return
		}
		if len(req.AlipayAccount) > 128 {
			common.ApiErrorMsg(c, "支付宝账号过长")
			return
		}
		if len(req.AlipayRealName) > 64 {
			common.ApiErrorMsg(c, "支付宝姓名过长")
			return
		}
	case model.WithdrawalChannelUSDT:
		if req.USDTNetwork == "" {
			common.ApiErrorMsg(c, "USDT 网络不能为空")
			return
		}
		if !model.IsSupportedUSDTWithdrawalNetwork(req.USDTNetwork) {
			common.ApiErrorMsg(c, "无效的 USDT 网络")
			return
		}
		if req.USDTAddress == "" {
			common.ApiErrorMsg(c, "USDT 收款地址不能为空")
			return
		}
		if len(req.USDTAddress) > 128 {
			common.ApiErrorMsg(c, "USDT 收款地址过长")
			return
		}
		if !model.IsValidUSDTWithdrawalAddress(req.USDTNetwork, req.USDTAddress) {
			common.ApiErrorMsg(c, "USDT 收款地址无效")
			return
		}
	default:
		common.ApiErrorMsg(c, "无效的提现方式")
		return
	}

	order, err := model.CreateUserWithdrawal(&model.CreateUserWithdrawalParams{
		UserID:         c.GetInt("id"),
		Channel:        channel,
		Amount:         req.Amount,
		AlipayAccount:  req.AlipayAccount,
		AlipayRealName: req.AlipayRealName,
		USDTNetwork:    req.USDTNetwork,
		USDTAddress:    req.USDTAddress,
	})
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, order)
}

func ListUserWithdrawals(c *gin.Context) {
	userID := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.ListUserWithdrawals(userID, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func GetUserWithdrawal(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的提现单ID")
		return
	}
	item, err := model.GetUserWithdrawalByID(id, c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func AdminListWithdrawals(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userID, err := model.ParseUserIDFilter(c.Query("user_id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的用户ID")
		return
	}
	dateFrom, _ := strconv.ParseInt(strings.TrimSpace(c.Query("date_from")), 10, 64)
	dateTo, _ := strconv.ParseInt(strings.TrimSpace(c.Query("date_to")), 10, 64)

	items, total, err := model.ListAdminWithdrawals(model.AdminWithdrawalFilter{
		Status:        c.Query("status"),
		Keyword:       c.Query("keyword"),
		UserID:        userID,
		Username:      c.Query("username"),
		AlipayAccount: c.Query("alipay_account"),
		DateFrom:      dateFrom,
		DateTo:        dateTo,
	}, pageInfo)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func AdminGetWithdrawal(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的提现单ID")
		return
	}
	item, err := model.GetAdminWithdrawalByID(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, item)
}

func AdminApproveWithdrawal(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的提现单ID")
		return
	}
	var req approveUserWithdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "无效的审核参数")
		return
	}
	if err := model.ApproveUserWithdrawal(id, c.GetInt("id"), req.ReviewNote); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, gin.H{"id": id, "status": model.UserWithdrawalStatusApproved})
}

func AdminRejectWithdrawal(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的提现单ID")
		return
	}
	var req rejectUserWithdrawalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "无效的驳回参数")
		return
	}
	if err := model.RejectUserWithdrawal(id, c.GetInt("id"), req.RejectionNote); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, gin.H{"id": id, "status": model.UserWithdrawalStatusRejected})
}

func AdminMarkWithdrawalPaid(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "无效的提现单ID")
		return
	}
	var req markUserWithdrawalPaidRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "无效的打款参数")
		return
	}
	if err := model.MarkUserWithdrawalPaid(id, c.GetInt("id"), model.MarkUserWithdrawalPaidParams{
		PayReceiptNo:  req.PayReceiptNo,
		PayReceiptURL: req.PayReceiptURL,
		PaidNote:      req.PaidNote,
	}); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, gin.H{"id": id, "status": model.UserWithdrawalStatusPaid})
}
