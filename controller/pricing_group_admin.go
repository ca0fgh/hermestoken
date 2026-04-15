package controller

import (
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type pricingGroupUpsertRequest struct {
	GroupKey       string  `json:"group_key"`
	DisplayName    string  `json:"display_name"`
	Description    string  `json:"description"`
	BillingRatio   float64 `json:"billing_ratio"`
	UserSelectable bool    `json:"user_selectable"`
	Status         int     `json:"status"`
	SortOrder      int     `json:"sort_order"`
}

type pricingGroupArchiveRequest struct {
	GroupKey string `json:"group_key"`
}

type pricingGroupMergeRequest struct {
	SourceGroupKey string `json:"source_group_key"`
	TargetGroupKey string `json:"target_group_key"`
}

func AdminListPricingGroups(c *gin.Context) {
	var groups []model.PricingGroup
	if err := model.DB.Order("sort_order asc").Order("id asc").Find(&groups).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, groups)
}

func AdminCreatePricingGroup(c *gin.Context) {
	var req pricingGroupUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	groupKey := strings.TrimSpace(req.GroupKey)
	if groupKey == "" {
		common.ApiErrorMsg(c, "group_key 不能为空")
		return
	}

	group := &model.PricingGroup{
		GroupKey:       groupKey,
		DisplayName:    strings.TrimSpace(req.DisplayName),
		Description:    strings.TrimSpace(req.Description),
		BillingRatio:   req.BillingRatio,
		UserSelectable: req.UserSelectable,
		Status:         req.Status,
		SortOrder:      req.SortOrder,
	}
	if group.DisplayName == "" {
		group.DisplayName = group.GroupKey
	}
	if group.Status == 0 {
		group.Status = model.PricingGroupStatusActive
	}
	if group.BillingRatio <= 0 {
		group.BillingRatio = 1
	}

	if err := model.DB.Create(group).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, group)
}

func AdminUpdatePricingGroup(c *gin.Context) {
	groupKey := strings.TrimSpace(c.Param("group_key"))
	if groupKey == "" {
		common.ApiErrorMsg(c, "group_key 不能为空")
		return
	}

	var group model.PricingGroup
	if err := model.DB.Where("group_key = ?", groupKey).First(&group).Error; err != nil {
		common.ApiError(c, err)
		return
	}

	var req pricingGroupUpsertRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	if displayName := strings.TrimSpace(req.DisplayName); displayName != "" {
		group.DisplayName = displayName
	}
	group.Description = strings.TrimSpace(req.Description)
	if req.BillingRatio > 0 {
		group.BillingRatio = req.BillingRatio
	}
	group.UserSelectable = req.UserSelectable
	group.SortOrder = req.SortOrder
	if req.Status != 0 {
		group.Status = req.Status
	}

	if err := model.DB.Save(&group).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, group)
}

func ArchivePricingGroup(c *gin.Context) {
	var req pricingGroupArchiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	groupKey := strings.TrimSpace(req.GroupKey)
	if groupKey == "" {
		common.ApiErrorMsg(c, "group_key 不能为空")
		return
	}

	var group model.PricingGroup
	if err := model.DB.Where("group_key = ?", groupKey).First(&group).Error; err != nil {
		common.ApiError(c, err)
		return
	}

	group.Status = model.PricingGroupStatusArchived
	group.UserSelectable = false
	if err := model.DB.Save(&group).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, group)
}

func MergePricingGroup(c *gin.Context) {
	var req pricingGroupMergeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	sourceKey := strings.TrimSpace(req.SourceGroupKey)
	targetKey := strings.TrimSpace(req.TargetGroupKey)
	if sourceKey == "" || targetKey == "" {
		common.ApiErrorMsg(c, "source_group_key 和 target_group_key 不能为空")
		return
	}
	if sourceKey == targetKey {
		common.ApiErrorMsg(c, "source_group_key 和 target_group_key 不能相同")
		return
	}

	var source model.PricingGroup
	if err := model.DB.Where("group_key = ?", sourceKey).First(&source).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	var target model.PricingGroup
	if err := model.DB.Where("group_key = ?", targetKey).First(&target).Error; err != nil {
		common.ApiError(c, err)
		return
	}

	if err := model.DB.Transaction(func(tx *gorm.DB) error {
		if err := mergePricingGroupReferencesTx(tx, sourceKey, targetKey); err != nil {
			return err
		}
		if err := mergeCanonicalPricingGroupRulesTx(tx, source.Id, target.Id); err != nil {
			return err
		}
		if err := tx.Where("alias_key = ?", sourceKey).Delete(&model.PricingGroupAlias{}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.PricingGroupAlias{
			AliasKey: sourceKey,
			GroupId:  target.Id,
			Reason:   "merged into " + targetKey,
		}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&source).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"source_group_key": sourceKey,
		"target_group_key": targetKey,
	})
}

func mergePricingGroupReferencesTx(tx *gorm.DB, sourceKey, targetKey string) error {
	if tx.Migrator().HasTable(&model.User{}) {
		if err := tx.Model(&model.User{}).
			Where(&model.User{GroupKey: sourceKey}).
			Or(&model.User{Group: sourceKey}).
			Updates(map[string]any{"group_key": targetKey, "group": targetKey}).Error; err != nil {
			return err
		}
	}
	if tx.Migrator().HasTable(&model.Token{}) {
		if err := tx.Model(&model.Token{}).
			Where(&model.Token{GroupKey: sourceKey}).
			Or(&model.Token{Group: sourceKey}).
			Updates(map[string]any{"group_key": targetKey, "group": targetKey}).Error; err != nil {
			return err
		}
	}
	if tx.Migrator().HasTable(&model.SubscriptionPlan{}) {
		if err := tx.Model(&model.SubscriptionPlan{}).
			Where(&model.SubscriptionPlan{UpgradeGroupKey: sourceKey}).
			Or(&model.SubscriptionPlan{UpgradeGroup: sourceKey}).
			Updates(map[string]any{"upgrade_group_key": targetKey, "upgrade_group": targetKey}).Error; err != nil {
			return err
		}
	}
	if tx.Migrator().HasTable(&model.UserSubscription{}) {
		if err := tx.Model(&model.UserSubscription{}).
			Where(&model.UserSubscription{UpgradeGroupKeySnapshot: sourceKey}).
			Or(&model.UserSubscription{UpgradeGroup: sourceKey}).
			Updates(map[string]any{
				"upgrade_group_key_snapshot":  targetKey,
				"upgrade_group":               targetKey,
				"upgrade_group_name_snapshot": targetKey,
			}).Error; err != nil {
			return err
		}
	}
	if tx.Migrator().HasTable(&model.SubscriptionReferralOverride{}) {
		if err := tx.Model(&model.SubscriptionReferralOverride{}).
			Where(&model.SubscriptionReferralOverride{Group: sourceKey}).
			Update("group", targetKey).Error; err != nil {
			return err
		}
	}
	if tx.Migrator().HasTable(&model.SubscriptionReferralRecord{}) {
		if err := tx.Model(&model.SubscriptionReferralRecord{}).
			Where(&model.SubscriptionReferralRecord{ReferralGroup: sourceKey}).
			Update("referral_group", targetKey).Error; err != nil {
			return err
		}
	}
	if tx.Migrator().HasTable(&model.Channel{}) {
		if err := tx.Model(&model.Channel{}).
			Where(&model.Channel{Group: sourceKey}).
			Update("group", targetKey).Error; err != nil {
			return err
		}
	}
	if tx.Migrator().HasTable(&model.Ability{}) {
		if err := tx.Model(&model.Ability{}).
			Where(&model.Ability{Group: sourceKey}).
			Update("group", targetKey).Error; err != nil {
			return err
		}
	}
	return nil
}

func mergeCanonicalPricingGroupRulesTx(tx *gorm.DB, sourceID, targetID int) error {
	if sourceID == targetID {
		return errors.New("source and target pricing group ids cannot match")
	}

	if tx.Migrator().HasTable(&model.PricingGroupRatioOverride{}) {
		var overrides []model.PricingGroupRatioOverride
		if err := tx.Where("source_group_id = ? OR target_group_id = ?", sourceID, sourceID).Find(&overrides).Error; err != nil {
			return err
		}
		for _, override := range overrides {
			if err := tx.Delete(&override).Error; err != nil {
				return err
			}
			next := override
			if next.SourceGroupId == sourceID {
				next.SourceGroupId = targetID
			}
			if next.TargetGroupId == sourceID {
				next.TargetGroupId = targetID
			}
			if next.SourceGroupId == next.TargetGroupId {
				continue
			}
			var existing model.PricingGroupRatioOverride
			err := tx.Where("source_group_id = ? AND target_group_id = ?", next.SourceGroupId, next.TargetGroupId).First(&existing).Error
			switch {
			case errors.Is(err, gorm.ErrRecordNotFound):
				next.Id = 0
				if err := tx.Create(&next).Error; err != nil {
					return err
				}
			case err == nil:
				existing.Ratio = next.Ratio
				if err := tx.Save(&existing).Error; err != nil {
					return err
				}
			default:
				return err
			}
		}
	}

	if tx.Migrator().HasTable(&model.PricingGroupVisibilityRule{}) {
		var rules []model.PricingGroupVisibilityRule
		if err := tx.Where("subject_group_id = ? OR target_group_id = ?", sourceID, sourceID).Find(&rules).Error; err != nil {
			return err
		}
		for _, rule := range rules {
			next := rule
			if next.SubjectGroupId == sourceID {
				next.SubjectGroupId = targetID
			}
			if next.TargetGroupId == sourceID {
				next.TargetGroupId = targetID
			}
			if err := tx.Model(&rule).Updates(map[string]any{
				"subject_group_id": next.SubjectGroupId,
				"target_group_id":  next.TargetGroupId,
			}).Error; err != nil {
				return err
			}
		}
	}

	if tx.Migrator().HasTable(&model.PricingGroupAutoPriority{}) {
		var priorities []model.PricingGroupAutoPriority
		if err := tx.Where("group_id = ?", sourceID).Find(&priorities).Error; err != nil {
			return err
		}
		for _, priority := range priorities {
			var existing model.PricingGroupAutoPriority
			err := tx.Where("group_id = ?", targetID).First(&existing).Error
			if err == nil {
				if err := tx.Delete(&priority).Error; err != nil {
					return err
				}
				continue
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err := tx.Model(&priority).Update("group_id", targetID).Error; err != nil {
				return err
			}
		}
	}

	return nil
}
