package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/model"
	"github.com/ca0fgh/hermestoken/service"
	"github.com/ca0fgh/hermestoken/setting"
	"github.com/gin-gonic/gin"
)

type marketplaceCredentialCreateRequest struct {
	VendorType         int      `json:"vendor_type"`
	APIKey             string   `json:"api_key"`
	OpenAIOrganization string   `json:"openai_organization"`
	TestModel          string   `json:"test_model"`
	BaseURL            string   `json:"base_url"`
	Other              string   `json:"other"`
	ModelMapping       string   `json:"model_mapping"`
	StatusCodeMapping  string   `json:"status_code_mapping"`
	Setting            string   `json:"setting"`
	ParamOverride      string   `json:"param_override"`
	HeaderOverride     string   `json:"header_override"`
	OtherSettings      string   `json:"settings"`
	Models             []string `json:"models"`
	QuotaMode          string   `json:"quota_mode"`
	QuotaLimit         int64    `json:"quota_limit"`
	TimeMode           string   `json:"time_mode"`
	TimeLimitSeconds   int64    `json:"time_limit_seconds"`
	Multiplier         float64  `json:"multiplier"`
	ConcurrencyLimit   *int     `json:"concurrency_limit"`
}

type marketplaceCredentialUpdateRequest struct {
	APIKey             string    `json:"api_key"`
	OpenAIOrganization *string   `json:"openai_organization"`
	TestModel          *string   `json:"test_model"`
	BaseURL            *string   `json:"base_url"`
	Other              *string   `json:"other"`
	ModelMapping       *string   `json:"model_mapping"`
	StatusCodeMapping  *string   `json:"status_code_mapping"`
	Setting            *string   `json:"setting"`
	ParamOverride      *string   `json:"param_override"`
	HeaderOverride     *string   `json:"header_override"`
	OtherSettings      *string   `json:"settings"`
	Models             *[]string `json:"models"`
	QuotaMode          *string   `json:"quota_mode"`
	QuotaLimit         *int64    `json:"quota_limit"`
	TimeMode           *string   `json:"time_mode"`
	TimeLimitSeconds   *int64    `json:"time_limit_seconds"`
	Multiplier         *float64  `json:"multiplier"`
	ConcurrencyLimit   *int      `json:"concurrency_limit"`
}

type marketplaceCredentialFetchModelsRequest struct {
	VendorType         int    `json:"vendor_type"`
	Type               int    `json:"type"`
	APIKey             string `json:"api_key"`
	Key                string `json:"key"`
	OpenAIOrganization string `json:"openai_organization"`
	TestModel          string `json:"test_model"`
	BaseURL            string `json:"base_url"`
	Other              string `json:"other"`
	ModelMapping       string `json:"model_mapping"`
	StatusCodeMapping  string `json:"status_code_mapping"`
	Setting            string `json:"setting"`
	ParamOverride      string `json:"param_override"`
	HeaderOverride     string `json:"header_override"`
	OtherSettings      string `json:"settings"`
}

func SellerListMarketplacePricedModels(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    service.ListPricedOpenAIModels(openAIModelsMap),
		"object":  "list",
	})
}

func SellerCreateMarketplaceCredential(c *gin.Context) {
	userID := c.GetInt("id")
	var req marketplaceCredentialCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	credential, err := service.CreateSellerMarketplaceCredential(service.MarketplaceCredentialCreateInput{
		SellerUserID:       userID,
		VendorType:         req.VendorType,
		APIKey:             req.APIKey,
		OpenAIOrganization: req.OpenAIOrganization,
		TestModel:          req.TestModel,
		BaseURL:            req.BaseURL,
		Other:              req.Other,
		ModelMapping:       req.ModelMapping,
		StatusCodeMapping:  req.StatusCodeMapping,
		Setting:            req.Setting,
		ParamOverride:      req.ParamOverride,
		HeaderOverride:     req.HeaderOverride,
		OtherSettings:      req.OtherSettings,
		Models:             req.Models,
		QuotaMode:          req.QuotaMode,
		QuotaLimit:         req.QuotaLimit,
		TimeMode:           req.TimeMode,
		TimeLimitSeconds:   req.TimeLimitSeconds,
		Multiplier:         req.Multiplier,
		ConcurrencyLimit:   req.ConcurrencyLimit,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, credential)
}

func SellerFetchMarketplaceCredentialModels(c *gin.Context) {
	var req marketplaceCredentialFetchModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if !setting.MarketplaceEnabled {
		common.ApiError(c, errors.New("marketplace is not enabled"))
		return
	}
	vendorType := req.VendorType
	if vendorType == 0 {
		vendorType = req.Type
	}
	if !setting.IsMarketplaceVendorTypeEnabled(vendorType) {
		common.ApiError(c, errors.New("marketplace vendor type is not enabled"))
		return
	}
	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(req.Key)
	}
	apiKey = strings.TrimSpace(strings.Split(apiKey, "\n")[0])
	if apiKey == "" {
		common.ApiError(c, errors.New("api key is required"))
		return
	}

	channel := service.MarketplaceChannelFromCredential(&model.MarketplaceCredential{
		VendorType:         vendorType,
		OpenAIOrganization: req.OpenAIOrganization,
		TestModel:          req.TestModel,
		BaseURL:            req.BaseURL,
		Other:              req.Other,
		ModelMapping:       req.ModelMapping,
		StatusCodeMapping:  req.StatusCodeMapping,
		Setting:            req.Setting,
		ParamOverride:      req.ParamOverride,
		HeaderOverride:     req.HeaderOverride,
		OtherSettings:      req.OtherSettings,
	}, apiKey)

	models, err := fetchChannelUpstreamModelIDs(channel)
	if err != nil {
		message := strings.ReplaceAll(err.Error(), apiKey, "[redacted]")
		c.JSON(200, gin.H{
			"success": false,
			"message": fmt.Sprintf("获取模型列表失败: %s", message),
		})
		return
	}
	common.ApiSuccess(c, models)
}

func SellerListMarketplaceCredentials(c *gin.Context) {
	userID := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	credentials, total, err := service.ListSellerMarketplaceCredentialItems(userID, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(credentials)
	common.ApiSuccess(c, pageInfo)
}

func SellerGetMarketplaceCredential(c *gin.Context) {
	userID := c.GetInt("id")
	credentialID, err := marketplaceCredentialIDParam(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	credential, err := service.GetSellerMarketplaceCredential(userID, credentialID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, credential)
}

func SellerUpdateMarketplaceCredential(c *gin.Context) {
	userID := c.GetInt("id")
	credentialID, err := marketplaceCredentialIDParam(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var req marketplaceCredentialUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	credential, err := service.UpdateSellerMarketplaceCredential(service.MarketplaceCredentialUpdateInput{
		SellerUserID:       userID,
		CredentialID:       credentialID,
		APIKey:             req.APIKey,
		OpenAIOrganization: req.OpenAIOrganization,
		TestModel:          req.TestModel,
		BaseURL:            req.BaseURL,
		Other:              req.Other,
		ModelMapping:       req.ModelMapping,
		StatusCodeMapping:  req.StatusCodeMapping,
		Setting:            req.Setting,
		ParamOverride:      req.ParamOverride,
		HeaderOverride:     req.HeaderOverride,
		OtherSettings:      req.OtherSettings,
		Models:             req.Models,
		QuotaMode:          req.QuotaMode,
		QuotaLimit:         req.QuotaLimit,
		TimeMode:           req.TimeMode,
		TimeLimitSeconds:   req.TimeLimitSeconds,
		Multiplier:         req.Multiplier,
		ConcurrencyLimit:   req.ConcurrencyLimit,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, credential)
}

func SellerProbeMarketplaceCredential(c *gin.Context) {
	userID := c.GetInt("id")
	credentialID, err := marketplaceCredentialIDParam(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	credential, err := service.RequestSellerMarketplaceCredentialProbeWithModel(userID, credentialID, c.Query("model"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, credential)
}

func SellerDeleteMarketplaceCredential(c *gin.Context) {
	userID := c.GetInt("id")
	credentialID, err := marketplaceCredentialIDParam(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err := service.DeleteSellerMarketplaceCredential(userID, credentialID); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"deleted": true})
}

func SellerListMarketplaceCredential(c *gin.Context) {
	setSellerMarketplaceCredentialListed(c, true)
}

func SellerUnlistMarketplaceCredential(c *gin.Context) {
	setSellerMarketplaceCredentialListed(c, false)
}

func SellerEnableMarketplaceCredential(c *gin.Context) {
	setSellerMarketplaceCredentialEnabled(c, true)
}

func SellerDisableMarketplaceCredential(c *gin.Context) {
	setSellerMarketplaceCredentialEnabled(c, false)
}

func SellerTestMarketplaceCredential(c *gin.Context) {
	userID := c.GetInt("id")
	credentialID, err := marketplaceCredentialIDParam(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	channel, err := service.BuildSellerMarketplaceChannel(userID, credentialID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	testModel := c.Query("model")
	endpointType := c.Query("endpoint_type")
	isStream, _ := strconv.ParseBool(c.Query("stream"))
	start := time.Now()
	result := testChannel(channel, testModel, endpointType, isStream)
	responseTimeMS := time.Since(start).Milliseconds()
	if responseTimeMS <= 0 {
		responseTimeMS = 1
	}
	reason := "credential live test passed"
	if result.localErr != nil {
		reason = result.localErr.Error()
	}
	if result.hermesTokenError != nil {
		reason = result.hermesTokenError.Error()
	}
	updated, markErr := service.ApplySellerMarketplaceCredentialTestResult(service.MarketplaceCredentialTestResultInput{
		SellerUserID:   userID,
		CredentialID:   credentialID,
		Success:        result.localErr == nil && result.hermesTokenError == nil && !result.skipped,
		Skipped:        result.skipped,
		ResponseTimeMS: responseTimeMS,
		Reason:         reason,
	})
	if markErr != nil {
		common.ApiError(c, markErr)
		return
	}
	consumedTime := float64(responseTimeMS) / 1000.0
	if result.localErr != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": result.localErr.Error(),
			"time":    0.0,
			"data":    updated,
		})
		return
	}
	if result.hermesTokenError != nil {
		c.JSON(http.StatusOK, gin.H{
			"success":    false,
			"message":    result.hermesTokenError.Error(),
			"time":       consumedTime,
			"error_code": result.hermesTokenError.GetErrorCode(),
			"data":       updated,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"time":    consumedTime,
		"data":    updated,
	})
}

func SellerGetMarketplaceIncome(c *gin.Context) {
	userID := c.GetInt("id")
	summary, err := service.GetSellerMarketplaceIncomeSummary(userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, summary)
}

func SellerListMarketplaceSettlements(c *gin.Context) {
	userID := c.GetInt("id")
	credentialID := 0
	if c.Query("credential_id") != "" {
		parsedCredentialID, err := strconv.Atoi(c.Query("credential_id"))
		if err != nil {
			common.ApiError(c, err)
			return
		}
		credentialID = parsedCredentialID
	}

	pageInfo := common.GetPageQuery(c)
	items, total, err := service.ListSellerMarketplaceSettlements(service.MarketplaceSettlementListInput{
		SellerUserID: userID,
		Status:       c.Query("status"),
		SourceType:   c.Query("source_type"),
		CredentialID: credentialID,
	}, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

func SellerReleaseMarketplaceSettlements(c *gin.Context) {
	userID := c.GetInt("id")
	result, err := service.ReleaseSellerAvailableMarketplaceSettlements(userID, common.GetTimestamp(), 100)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

func setSellerMarketplaceCredentialListed(c *gin.Context, listed bool) {
	userID := c.GetInt("id")
	credentialID, err := marketplaceCredentialIDParam(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	credential, err := service.SetSellerMarketplaceCredentialListed(userID, credentialID, listed)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, credential)
}

func setSellerMarketplaceCredentialEnabled(c *gin.Context, enabled bool) {
	userID := c.GetInt("id")
	credentialID, err := marketplaceCredentialIDParam(c)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	credential, err := service.SetSellerMarketplaceCredentialEnabled(userID, credentialID, enabled)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, credential)
}

func marketplaceCredentialIDParam(c *gin.Context) (int, error) {
	return strconv.Atoi(c.Param("id"))
}
