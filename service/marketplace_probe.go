package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/ca0fgh/hermestoken/constant"
	"github.com/ca0fgh/hermestoken/model"
	tokenverifier "github.com/ca0fgh/hermestoken/service/token_verifier"
	"gorm.io/gorm"
)

const (
	marketplaceCredentialProbeQueueSize  = 1000
	marketplaceCredentialProbeWorkerSize = 2
	marketplaceProbeErrorMaxLength       = 240
)

type marketplaceCredentialDirectProbeFunc func(context.Context, tokenverifier.DirectProbeRequest) (*tokenverifier.DirectProbeResponse, error)

var (
	marketplaceCredentialProbeQueue      chan int
	marketplaceCredentialProbeQueueMutex sync.RWMutex
	marketplaceCredentialProbeWorkerOnce sync.Once
	runMarketplaceCredentialDirectProbe  marketplaceCredentialDirectProbeFunc = tokenverifier.RunDirectProbe
)

func StartMarketplaceCredentialProbeWorker() {
	marketplaceCredentialProbeWorkerOnce.Do(func() {
		queue := make(chan int, marketplaceCredentialProbeQueueSize)
		marketplaceCredentialProbeQueueMutex.Lock()
		marketplaceCredentialProbeQueue = queue
		marketplaceCredentialProbeQueueMutex.Unlock()

		for i := 0; i < marketplaceCredentialProbeWorkerSize; i++ {
			go marketplaceCredentialProbeWorker(queue)
		}
		go enqueuePendingMarketplaceCredentialProbes()
	})
}

func EnqueueMarketplaceCredentialProbe(credentialID int) {
	if credentialID <= 0 {
		return
	}
	marketplaceCredentialProbeQueueMutex.RLock()
	queue := marketplaceCredentialProbeQueue
	marketplaceCredentialProbeQueueMutex.RUnlock()
	if queue == nil {
		return
	}
	select {
	case queue <- credentialID:
	default:
		go func() {
			queue <- credentialID
		}()
	}
}

func RunMarketplaceCredentialProbe(ctx context.Context, credentialID int) error {
	if credentialID <= 0 {
		return errors.New("marketplace credential id is required")
	}

	var credential model.MarketplaceCredential
	if err := model.DB.First(&credential, credentialID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return errors.New("marketplace credential not found")
		}
		return err
	}

	secret, err := GetMarketplaceCredentialSecret()
	if err != nil {
		_ = markMarketplaceCredentialProbeFailed(credential, "", "", err)
		return err
	}
	apiKey, err := DecryptMarketplaceAPIKey(credential.EncryptedAPIKey, secret)
	if err != nil {
		_ = markMarketplaceCredentialProbeFailed(credential, "", "", err)
		return err
	}

	provider := marketplaceProbeProviderForCredential(credential)
	profile := marketplaceProbeProfileForProvider(provider)
	clientProfile := marketplaceProbeClientProfileForProvider(provider)
	modelName, err := marketplaceProbeModelForCredential(credential, provider)
	if err != nil {
		_ = markMarketplaceCredentialProbeFailed(credential, apiKey, credential.BaseURL, err)
		return marketplaceCredentialProbeSanitizedError(err, apiKey, credential.BaseURL)
	}
	baseURL, err := marketplaceProbeBaseURLForCredential(credential, apiKey)
	if err != nil {
		_ = markMarketplaceCredentialProbeFailed(credential, apiKey, credential.BaseURL, err)
		return marketplaceCredentialProbeSanitizedError(err, apiKey, credential.BaseURL)
	}

	if err := markMarketplaceCredentialProbeRunning(credential.ID, provider, profile, modelName, clientProfile); err != nil {
		return err
	}

	probeCtx, cancel := marketplaceCredentialProbeContext(ctx, profile)
	defer cancel()
	result, err := runMarketplaceCredentialDirectProbe(probeCtx, tokenverifier.DirectProbeRequest{
		BaseURL:       baseURL,
		APIKey:        apiKey,
		Provider:      provider,
		Model:         modelName,
		ProbeProfile:  profile,
		ClientProfile: clientProfile,
	})
	if err != nil {
		_ = markMarketplaceCredentialProbeFailed(credential, apiKey, baseURL, err)
		return marketplaceCredentialProbeSanitizedError(err, apiKey, baseURL)
	}
	if result == nil {
		err := errors.New("marketplace probe returned empty result")
		_ = markMarketplaceCredentialProbeFailed(credential, apiKey, baseURL, err)
		return marketplaceCredentialProbeSanitizedError(err, apiKey, baseURL)
	}

	return markMarketplaceCredentialProbeCompleted(credential.ID, marketplaceProbeCompletion{
		Status:         marketplaceProbeStatusForReport(result.Report),
		Score:          marketplaceProbeReportScore(result.Report),
		ScoreMax:       marketplaceProbeReportScoreMax(result.Report),
		Grade:          marketplaceProbeReportGrade(result.Report),
		Provider:       provider,
		Profile:        profile,
		Model:          modelName,
		ClientProfile:  clientProfile,
		ScoringVersion: result.Report.ScoringVersion,
	})
}

func RequestSellerMarketplaceCredentialProbe(sellerUserID int, credentialID int) (*model.MarketplaceCredential, error) {
	return RequestSellerMarketplaceCredentialProbeWithModel(sellerUserID, credentialID, "")
}

func RequestSellerMarketplaceCredentialProbeWithModel(sellerUserID int, credentialID int, requestedModel string) (*model.MarketplaceCredential, error) {
	if err := validateMarketplaceEnabled(); err != nil {
		return nil, err
	}
	if sellerUserID <= 0 {
		return nil, errors.New("seller user id is required")
	}
	if credentialID <= 0 {
		return nil, errors.New("marketplace credential id is required")
	}

	requestedModel = strings.TrimSpace(requestedModel)
	if err := validateMarketplaceProbeRequestedModel(requestedModel); err != nil {
		return nil, err
	}

	var credential model.MarketplaceCredential
	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := marketplaceForUpdate(tx).
			Where("id = ? AND seller_user_id = ?", credentialID, sellerUserID).
			First(&credential).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("marketplace credential not found")
			}
			return err
		}
		if marketplaceCredentialProbeInProgress(credential.ProbeStatus) {
			return errors.New("marketplace credential probe is already in progress")
		}
		if requestedModel != "" && !marketplaceCredentialModelsContain(credential.Models, requestedModel) {
			return errors.New("marketplace credential does not include requested probe model")
		}
		setMarketplaceCredentialProbePending(&credential)
		if requestedModel != "" {
			credential.ProbeModel = requestedModel
		}
		return tx.Save(&credential).Error
	}); err != nil {
		return nil, err
	}

	EnqueueMarketplaceCredentialProbe(credential.ID)
	return &credential, nil
}

func validateMarketplaceProbeRequestedModel(requestedModel string) error {
	if requestedModel == "" {
		return nil
	}
	if len(requestedModel) > 191 {
		return errors.New("marketplace credential probe model is too long")
	}
	if strings.ContainsAny(requestedModel, "\r\n,，") {
		return errors.New("marketplace credential probe supports only one model")
	}
	return nil
}

func setMarketplaceCredentialProbePending(credential *model.MarketplaceCredential) {
	if credential == nil {
		return
	}
	credential.ProbeStatus = model.MarketplaceProbeStatusPending
	credential.ProbeScore = 0
	credential.ProbeScoreMax = 0
	credential.ProbeGrade = ""
	credential.ProbeCheckedAt = 0
	credential.ProbeError = ""
	credential.ProbeProvider = ""
	credential.ProbeProfile = ""
	credential.ProbeModel = ""
	credential.ProbeClientProfile = ""
	credential.ProbeScoringVersion = ""
}

func marketplaceCredentialProbeInProgress(status string) bool {
	switch strings.TrimSpace(status) {
	case model.MarketplaceProbeStatusPending, model.MarketplaceProbeStatusRunning:
		return true
	default:
		return false
	}
}

func marketplaceCredentialProbeTargetChanged(changedFields []string, keyReplaced bool) bool {
	if keyReplaced {
		return true
	}
	for _, field := range changedFields {
		switch field {
		case "models",
			"openai_organization",
			"test_model",
			"base_url",
			"other",
			"model_mapping",
			"status_code_mapping",
			"setting",
			"param_override",
			"header_override",
			"settings":
			return true
		}
	}
	return false
}

func marketplaceCredentialProbeWorker(queue <-chan int) {
	for credentialID := range queue {
		if err := RunMarketplaceCredentialProbe(context.Background(), credentialID); err != nil {
			common.SysLog(fmt.Sprintf("marketplace credential probe failed for credential %d: %s", credentialID, sanitizeMarketplaceProbeMessage(err.Error(), "", "")))
		}
	}
}

func enqueuePendingMarketplaceCredentialProbes() {
	if model.DB == nil || !model.DB.Migrator().HasTable(&model.MarketplaceCredential{}) {
		return
	}

	var ids []int
	if err := model.DB.Model(&model.MarketplaceCredential{}).
		Where("probe_status IN ?", []string{model.MarketplaceProbeStatusPending, model.MarketplaceProbeStatusRunning}).
		Limit(marketplaceCredentialProbeQueueSize).
		Pluck("id", &ids).Error; err != nil {
		common.SysLog("failed to load pending marketplace credential probes: " + err.Error())
		return
	}
	for _, id := range ids {
		EnqueueMarketplaceCredentialProbe(id)
	}
}

func marketplaceProbeProviderForCredential(credential model.MarketplaceCredential) string {
	if credential.VendorType == constant.ChannelTypeAnthropic {
		return tokenverifier.ProviderAnthropic
	}
	return tokenverifier.ProviderOpenAI
}

func marketplaceProbeProfileForProvider(provider string) string {
	return tokenverifier.ProbeProfileFull
}

func marketplaceProbeClientProfileForProvider(provider string) string {
	if provider == tokenverifier.ProviderAnthropic {
		return tokenverifier.ClientProfileClaudeCode
	}
	return ""
}

func marketplaceProbeModelForCredential(credential model.MarketplaceCredential, provider string) (string, error) {
	if marketplaceCredentialProbeInProgress(credential.ProbeStatus) {
		if modelName := strings.TrimSpace(credential.ProbeModel); modelName != "" {
			return modelName, nil
		}
	}
	if modelName := strings.TrimSpace(credential.TestModel); modelName != "" {
		return modelName, nil
	}
	for _, modelName := range strings.Split(credential.Models, ",") {
		modelName = strings.TrimSpace(modelName)
		if modelName != "" {
			return modelName, nil
		}
	}
	if provider == tokenverifier.ProviderAnthropic {
		return "", errors.New("marketplace credential has no Anthropic model for probe")
	}
	return "", errors.New("marketplace credential has no model for probe")
}

func marketplaceCredentialModelsContain(models string, requestedModel string) bool {
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" {
		return false
	}
	for _, modelName := range strings.Split(models, ",") {
		if strings.TrimSpace(modelName) == requestedModel {
			return true
		}
	}
	return false
}

func marketplaceProbeBaseURLForCredential(credential model.MarketplaceCredential, apiKey string) (string, error) {
	channel := MarketplaceChannelFromCredential(&credential, apiKey)
	if channel == nil {
		return "", errors.New("marketplace credential channel is empty")
	}
	baseURL := strings.TrimSpace(channel.GetBaseURL())
	if err := tokenverifier.ValidateBaseURL(baseURL); err != nil {
		return "", err
	}
	return strings.TrimRight(baseURL, "/"), nil
}

func marketplaceCredentialProbeContext(parent context.Context, profile string) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	if _, ok := parent.Deadline(); ok {
		return context.WithCancel(parent)
	}
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case tokenverifier.ProbeProfileFull:
		return context.WithTimeout(parent, 6*time.Minute)
	case tokenverifier.ProbeProfileDeep:
		return context.WithTimeout(parent, 3*time.Minute)
	default:
		return context.WithTimeout(parent, 90*time.Second)
	}
}

func markMarketplaceCredentialProbeRunning(credentialID int, provider string, profile string, modelName string, clientProfile string) error {
	return model.DB.Model(&model.MarketplaceCredential{}).
		Where("id = ?", credentialID).
		Updates(map[string]any{
			"probe_status":          model.MarketplaceProbeStatusRunning,
			"probe_error":           "",
			"probe_provider":        provider,
			"probe_profile":         profile,
			"probe_model":           modelName,
			"probe_client_profile":  clientProfile,
			"probe_scoring_version": "",
		}).Error
}

type marketplaceProbeCompletion struct {
	Status         string
	Score          int
	ScoreMax       int
	Grade          string
	Provider       string
	Profile        string
	Model          string
	ClientProfile  string
	ScoringVersion string
}

func markMarketplaceCredentialProbeCompleted(credentialID int, completion marketplaceProbeCompletion) error {
	return model.DB.Model(&model.MarketplaceCredential{}).
		Where("id = ?", credentialID).
		Updates(map[string]any{
			"probe_status":          completion.Status,
			"probe_score":           completion.Score,
			"probe_score_max":       completion.ScoreMax,
			"probe_grade":           completion.Grade,
			"probe_checked_at":      time.Now().Unix(),
			"probe_error":           "",
			"probe_provider":        completion.Provider,
			"probe_profile":         completion.Profile,
			"probe_model":           completion.Model,
			"probe_client_profile":  completion.ClientProfile,
			"probe_scoring_version": completion.ScoringVersion,
		}).Error
}

func markMarketplaceCredentialProbeFailed(credential model.MarketplaceCredential, apiKey string, baseURL string, probeErr error) error {
	message := ""
	if probeErr != nil {
		message = probeErr.Error()
	}
	return model.DB.Model(&model.MarketplaceCredential{}).
		Where("id = ?", credential.ID).
		Updates(map[string]any{
			"probe_status":     model.MarketplaceProbeStatusFailed,
			"probe_checked_at": time.Now().Unix(),
			"probe_error":      sanitizeMarketplaceProbeMessage(message, apiKey, baseURL),
		}).Error
}

func marketplaceProbeStatusForReport(report tokenverifier.ReportSummary) string {
	score := marketplaceProbeReportScore(report)
	switch {
	case score >= 80:
		return model.MarketplaceProbeStatusPassed
	case score >= 50:
		return model.MarketplaceProbeStatusWarning
	default:
		return model.MarketplaceProbeStatusFailed
	}
}

func marketplaceProbeReportScore(report tokenverifier.ReportSummary) int {
	if report.ProbeScore != 0 || report.ProbeScoreMax > 0 || report.Score == 0 {
		return clampMarketplaceProbeScore(report.ProbeScore)
	}
	return clampMarketplaceProbeScore(report.Score)
}

func marketplaceProbeReportScoreMax(report tokenverifier.ReportSummary) int {
	score := marketplaceProbeReportScore(report)
	if report.ProbeScoreMax <= 0 {
		return score
	}
	return clampMarketplaceProbeScore(report.ProbeScoreMax)
}

func marketplaceProbeReportGrade(report tokenverifier.ReportSummary) string {
	if grade := strings.TrimSpace(report.Grade); grade != "" {
		return truncateMarketplaceProbeValue(grade, 16)
	}
	return truncateMarketplaceProbeValue(strings.TrimSpace(report.FinalRating.Grade), 16)
}

func clampMarketplaceProbeScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func sanitizeMarketplaceProbeMessage(message string, apiKey string, baseURL string) string {
	message = strings.TrimSpace(message)
	if message == "" {
		return ""
	}
	for _, secret := range []string{apiKey, baseURL, strings.TrimRight(baseURL, "/")} {
		secret = strings.TrimSpace(secret)
		if secret == "" {
			continue
		}
		message = strings.ReplaceAll(message, secret, "[redacted]")
	}
	return truncateMarketplaceProbeValue(message, marketplaceProbeErrorMaxLength)
}

func marketplaceCredentialProbeSanitizedError(err error, apiKey string, baseURL string) error {
	if err == nil {
		return nil
	}
	message := sanitizeMarketplaceProbeMessage(err.Error(), apiKey, baseURL)
	if message == "" {
		return err
	}
	return errors.New(message)
}

func truncateMarketplaceProbeValue(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return value[:limit]
	}
	return value[:limit-3] + "..."
}
