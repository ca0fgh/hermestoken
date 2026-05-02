package model

import (
	"errors"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	TokenVerificationStatusPending = "pending"
	TokenVerificationStatusRunning = "running"
	TokenVerificationStatusSuccess = "success"
	TokenVerificationStatusFailed  = "failed"
)

type TokenVerificationTask struct {
	ID         int64  `json:"id" gorm:"primaryKey"`
	UserID     int    `json:"user_id" gorm:"index"`
	TokenID    int    `json:"token_id" gorm:"index"`
	TokenName  string `json:"token_name" gorm:"type:varchar(191)"`
	Models     string `json:"models" gorm:"type:text"`
	Providers  string `json:"providers" gorm:"type:text"`
	Status     string `json:"status" gorm:"type:varchar(20);index"`
	Score      int    `json:"score"`
	Grade      string `json:"grade" gorm:"type:varchar(10)"`
	FailReason string `json:"fail_reason" gorm:"type:text"`
	CreatedAt  int64  `json:"created_at" gorm:"index"`
	StartedAt  int64  `json:"started_at"`
	FinishedAt int64  `json:"finished_at"`
}

func (t *TokenVerificationTask) SetModels(models []string) {
	data, _ := common.Marshal(models)
	t.Models = string(data)
}

func (t *TokenVerificationTask) GetModels() []string {
	if t == nil || t.Models == "" {
		return nil
	}
	var models []string
	if err := common.UnmarshalJsonStr(t.Models, &models); err != nil {
		return nil
	}
	return models
}

func (t *TokenVerificationTask) SetProviders(providers []string) {
	data, _ := common.Marshal(providers)
	t.Providers = string(data)
}

func (t *TokenVerificationTask) GetProviders() []string {
	if t == nil || t.Providers == "" {
		return nil
	}
	var providers []string
	if err := common.UnmarshalJsonStr(t.Providers, &providers); err != nil {
		return nil
	}
	return providers
}

type TokenVerificationResult struct {
	ID                 int64   `json:"id" gorm:"primaryKey"`
	TaskID             int64   `json:"task_id" gorm:"index"`
	Provider           string  `json:"provider" gorm:"type:varchar(32);index"`
	CheckKey           string  `json:"check_key" gorm:"type:varchar(64);index"`
	ModelName          string  `json:"model_name" gorm:"type:varchar(191);index"`
	ClaimedModel       string  `json:"claimed_model" gorm:"type:varchar(191);index"`
	ObservedModel      string  `json:"observed_model" gorm:"type:varchar(191);index"`
	IdentityConfidence int     `json:"identity_confidence"`
	SuspectedDowngrade bool    `json:"suspected_downgrade"`
	Success            bool    `json:"success"`
	Score              int     `json:"score"`
	LatencyMs          int64   `json:"latency_ms"`
	TTFTMs             int64   `json:"ttft_ms"`
	TokensPS           float64 `json:"tokens_ps" gorm:"type:decimal(18,6);not null;default:0"`
	ErrorCode          string  `json:"error_code" gorm:"type:varchar(64)"`
	Message            string  `json:"message" gorm:"type:text"`
	Raw                string  `json:"raw" gorm:"type:json"`
	CreatedAt          int64   `json:"created_at" gorm:"index"`
}

type TokenVerificationReport struct {
	ID             int64  `json:"id" gorm:"primaryKey"`
	TaskID         int64  `json:"task_id" gorm:"uniqueIndex"`
	UserID         int    `json:"user_id" gorm:"index"`
	Summary        string `json:"summary" gorm:"type:json"`
	ScoringVersion string `json:"scoring_version" gorm:"type:varchar(16);index"`
	BaselineSource string `json:"baseline_source" gorm:"type:varchar(32)"`
	CreatedAt      int64  `json:"created_at" gorm:"index"`
}

func CreateTokenVerificationTask(userID int, token *Token, models []string, providers []string) (*TokenVerificationTask, error) {
	if userID <= 0 || token == nil || token.Id <= 0 {
		return nil, errors.New("invalid token verification task")
	}
	now := time.Now().Unix()
	task := &TokenVerificationTask{
		UserID:    userID,
		TokenID:   token.Id,
		TokenName: token.Name,
		Status:    TokenVerificationStatusPending,
		CreatedAt: now,
	}
	task.SetModels(models)
	task.SetProviders(providers)
	if err := DB.Create(task).Error; err != nil {
		return nil, err
	}
	return task, nil
}

func GetTokenVerificationTaskByID(id int64) (*TokenVerificationTask, error) {
	if id <= 0 {
		return nil, errors.New("invalid task id")
	}
	var task TokenVerificationTask
	if err := DB.First(&task, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func GetUserTokenVerificationTaskByID(userID int, id int64) (*TokenVerificationTask, error) {
	if userID <= 0 || id <= 0 {
		return nil, errors.New("invalid task id")
	}
	var task TokenVerificationTask
	if err := DB.First(&task, "id = ? and user_id = ?", id, userID).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func ListUserTokenVerificationTasks(userID int, startIdx int, num int) ([]*TokenVerificationTask, error) {
	var tasks []*TokenVerificationTask
	err := DB.Where("user_id = ?", userID).Order("id desc").Limit(num).Offset(startIdx).Find(&tasks).Error
	return tasks, err
}

func CountUserTokenVerificationTasks(userID int) (int64, error) {
	var count int64
	err := DB.Model(&TokenVerificationTask{}).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

func UpdateTokenVerificationTaskRunning(taskID int64) error {
	return DB.Model(&TokenVerificationTask{}).
		Where("id = ? and status = ?", taskID, TokenVerificationStatusPending).
		Updates(map[string]any{
			"status":     TokenVerificationStatusRunning,
			"started_at": time.Now().Unix(),
		}).Error
}

func CompleteTokenVerificationTask(taskID int64, score int, grade string) error {
	return DB.Model(&TokenVerificationTask{}).
		Where("id = ?", taskID).
		Updates(map[string]any{
			"status":      TokenVerificationStatusSuccess,
			"score":       score,
			"grade":       grade,
			"fail_reason": "",
			"finished_at": time.Now().Unix(),
		}).Error
}

func FailTokenVerificationTask(taskID int64, reason string) error {
	return DB.Model(&TokenVerificationTask{}).
		Where("id = ?", taskID).
		Updates(map[string]any{
			"status":      TokenVerificationStatusFailed,
			"fail_reason": reason,
			"finished_at": time.Now().Unix(),
		}).Error
}

func AddTokenVerificationResults(results []*TokenVerificationResult) error {
	if len(results) == 0 {
		return nil
	}
	now := time.Now().Unix()
	for _, result := range results {
		if result.CreatedAt == 0 {
			result.CreatedAt = now
		}
	}
	return DB.Create(&results).Error
}

func GetTokenVerificationResults(taskID int64) ([]*TokenVerificationResult, error) {
	var results []*TokenVerificationResult
	err := DB.Where("task_id = ?", taskID).Order("id asc").Find(&results).Error
	return results, err
}

func UpsertTokenVerificationReport(report *TokenVerificationReport) error {
	if report == nil {
		return errors.New("empty report")
	}
	if report.CreatedAt == 0 {
		report.CreatedAt = time.Now().Unix()
	}
	var existing TokenVerificationReport
	err := DB.First(&existing, "task_id = ?", report.TaskID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return DB.Create(report).Error
	}
	if err != nil {
		return err
	}
	return DB.Model(&TokenVerificationReport{}).
		Where("task_id = ?", report.TaskID).
		Updates(map[string]any{
			"summary":         report.Summary,
			"scoring_version": report.ScoringVersion,
			"baseline_source": report.BaselineSource,
			"created_at":      report.CreatedAt,
		}).Error
}

func GetTokenVerificationReportByTaskID(taskID int64) (*TokenVerificationReport, error) {
	var report TokenVerificationReport
	if err := DB.First(&report, "task_id = ?", taskID).Error; err != nil {
		return nil, err
	}
	return &report, nil
}
