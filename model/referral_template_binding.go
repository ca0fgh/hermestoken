package model

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/ca0fgh/hermestoken/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ReferralTemplateBinding struct {
	Id           int    `json:"id"`
	UserId       int    `json:"user_id" gorm:"type:int;not null;uniqueIndex:idx_referral_template_binding_scope"`
	ReferralType string `json:"referral_type" gorm:"type:varchar(64);not null;uniqueIndex:idx_referral_template_binding_scope"`
	Group        string `json:"group" gorm:"type:varchar(64);not null;default:'';uniqueIndex:idx_referral_template_binding_scope"`
	TemplateId   int    `json:"template_id" gorm:"type:int;not null;index"`
	CreatedBy    int    `json:"created_by" gorm:"type:int;not null;default:0"`
	UpdatedBy    int    `json:"updated_by" gorm:"type:int;not null;default:0"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt    int64  `json:"updated_at" gorm:"bigint"`
}

type ReferralTemplateBindingView struct {
	Binding  ReferralTemplateBinding `json:"binding"`
	Template ReferralTemplate        `json:"template"`
}

type ReferralTemplateBindingBundleView struct {
	BindingIDs []int `json:"binding_ids"`
	ReferralTemplateBundle
}

func (b *ReferralTemplateBinding) normalize() {
	b.ReferralType = strings.TrimSpace(b.ReferralType)
	b.Group = strings.TrimSpace(b.Group)
}

func (b *ReferralTemplateBinding) ValidateAgainstTemplate(template *ReferralTemplate) error {
	b.normalize()
	if template == nil {
		return errors.New("template is required")
	}

	templateReferralType := strings.TrimSpace(template.ReferralType)
	templateGroup := strings.TrimSpace(template.Group)
	if templateReferralType == "" {
		return fmt.Errorf("template %d referral type is required", template.Id)
	}
	if templateGroup == "" {
		return fmt.Errorf("template %d group is required", template.Id)
	}
	b.ReferralType = templateReferralType
	b.Group = templateGroup
	return nil
}

func (b *ReferralTemplateBinding) validateWithTemplateID(tx *gorm.DB) error {
	b.normalize()
	if b.TemplateId <= 0 {
		return errors.New("template_id is required")
	}
	var template ReferralTemplate
	if err := tx.First(&template, b.TemplateId).Error; err != nil {
		return err
	}
	return b.ValidateAgainstTemplate(&template)
}

func (b *ReferralTemplateBinding) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	b.CreatedAt = now
	b.UpdatedAt = now
	return b.validateWithTemplateID(tx)
}

func (b *ReferralTemplateBinding) BeforeUpdate(tx *gorm.DB) error {
	b.UpdatedAt = common.GetTimestamp()
	return b.validateWithTemplateID(tx)
}

func HasActiveReferralTemplateBinding(userID int, referralType string, group string) (bool, *ReferralTemplateBinding, error) {
	if userID <= 0 {
		return false, nil, errors.New("invalid user id")
	}

	var binding ReferralTemplateBinding
	err := DB.Where(
		"user_id = ? AND referral_type = ? AND "+commonGroupCol+" = ?",
		userID,
		strings.TrimSpace(referralType),
		strings.TrimSpace(group),
	).First(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}

	template, err := GetReferralTemplateByID(binding.TemplateId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, &binding, nil
		}
		return false, nil, err
	}
	if !template.Enabled {
		return false, &binding, nil
	}
	return true, &binding, nil
}

func ListReferralTemplateBindingsByUser(userID int, referralType string) ([]ReferralTemplateBindingView, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}

	var bindings []ReferralTemplateBinding
	query := DB.Where("user_id = ?", userID).Order(commonGroupCol + " ASC")
	if trimmedReferralType := strings.TrimSpace(referralType); trimmedReferralType != "" {
		query = query.Where("referral_type = ?", trimmedReferralType)
	}
	if err := query.Find(&bindings).Error; err != nil {
		return nil, err
	}

	views := make([]ReferralTemplateBindingView, 0, len(bindings))
	for _, binding := range bindings {
		template, err := GetReferralTemplateByID(binding.TemplateId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			return nil, err
		}
		views = append(views, ReferralTemplateBindingView{
			Binding:  binding,
			Template: *template,
		})
	}
	return views, nil
}

func HasAnyActiveReferralTemplateBindingByUser(userID int, referralType string) (bool, error) {
	views, err := ListReferralTemplateBindingsByUser(userID, referralType)
	if err != nil {
		return false, err
	}
	for _, view := range views {
		if view.Template.Enabled {
			return true, nil
		}
	}
	return false, nil
}

func ListReferralTemplateBindingBundlesByUser(userID int, referralType string) ([]ReferralTemplateBindingBundleView, error) {
	views, err := ListReferralTemplateBindingsByUser(userID, referralType)
	if err != nil {
		return nil, err
	}
	if len(views) == 0 {
		return []ReferralTemplateBindingBundleView{}, nil
	}

	bundles, err := ListReferralTemplateBundles(referralType)
	if err != nil {
		return nil, err
	}
	bundleByKey := make(map[string]ReferralTemplateBundle, len(bundles))
	for _, bundle := range bundles {
		bundleByKey[bundle.BundleKey] = bundle
	}

	bundleOrder := make([]string, 0, len(views))
	bundleViews := make(map[string]*ReferralTemplateBindingBundleView, len(views))
	for _, view := range views {
		bundleKey := referralTemplateBundleKeyForRow(view.Template)
		bundle, exists := bundleByKey[bundleKey]
		if !exists {
			bundle = ReferralTemplateBundle{
				BundleKey:              bundleKey,
				TemplateIDs:            []int{view.Template.Id},
				ReferralType:           view.Template.ReferralType,
				Groups:                 []string{view.Template.Group},
				Name:                   view.Template.Name,
				LevelType:              view.Template.LevelType,
				Enabled:                view.Template.Enabled,
				DirectCapBps:           view.Template.DirectCapBps,
				TeamCapBps:             view.Template.TeamCapBps,
				InviteeShareDefaultBps: view.Template.InviteeShareDefaultBps,
				CreatedAt:              view.Template.CreatedAt,
				UpdatedAt:              view.Template.UpdatedAt,
			}
		}

		bundleView, exists := bundleViews[bundleKey]
		if !exists {
			bundleView = &ReferralTemplateBindingBundleView{
				BindingIDs: []int{},
				ReferralTemplateBundle: ReferralTemplateBundle{
					BundleKey:              bundle.BundleKey,
					TemplateIDs:            []int{},
					ReferralType:           bundle.ReferralType,
					Groups:                 []string{},
					Name:                   bundle.Name,
					LevelType:              bundle.LevelType,
					Enabled:                bundle.Enabled,
					DirectCapBps:           bundle.DirectCapBps,
					TeamCapBps:             bundle.TeamCapBps,
					InviteeShareDefaultBps: bundle.InviteeShareDefaultBps,
					CreatedAt:              bundle.CreatedAt,
					UpdatedAt:              bundle.UpdatedAt,
				},
			}
			bundleViews[bundleKey] = bundleView
			bundleOrder = append(bundleOrder, bundleKey)
		}

		if view.Binding.Id > 0 {
			bundleView.BindingIDs = append(bundleView.BindingIDs, view.Binding.Id)
		}
		if !containsInt(bundleView.TemplateIDs, view.Template.Id) {
			bundleView.TemplateIDs = append(bundleView.TemplateIDs, view.Template.Id)
		}
		boundGroup := strings.TrimSpace(view.Binding.Group)
		if boundGroup == "" {
			boundGroup = strings.TrimSpace(view.Template.Group)
		}
		if boundGroup != "" && !containsString(bundleView.Groups, boundGroup) {
			bundleView.Groups = append(bundleView.Groups, boundGroup)
		}
	}

	items := make([]ReferralTemplateBindingBundleView, 0, len(bundleOrder))
	for _, bundleKey := range bundleOrder {
		bundleView := bundleViews[bundleKey]
		sort.Ints(bundleView.BindingIDs)
		items = append(items, *bundleView)
	}
	return items, nil
}

func containsInt(items []int, target int) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func ResolveBindingInviteeShareDefault(view ReferralTemplateBindingView) int {
	return NormalizeSubscriptionReferralRateBps(view.Template.InviteeShareDefaultBps)
}

func GetReferralTemplateBindingViewByUserAndScope(userID int, referralType string, group string) (*ReferralTemplateBindingView, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}

	var binding ReferralTemplateBinding
	err := DB.Where(
		"user_id = ? AND referral_type = ? AND "+commonGroupCol+" = ?",
		userID,
		strings.TrimSpace(referralType),
		strings.TrimSpace(group),
	).First(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	template, err := GetReferralTemplateByID(binding.TemplateId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	view := &ReferralTemplateBindingView{
		Binding:  binding,
		Template: *template,
	}
	return view, nil
}

func normalizeBindingIDs(bindingIDs []int) []int {
	uniqueIDs := make(map[int]struct{}, len(bindingIDs))
	normalized := make([]int, 0, len(bindingIDs))
	for _, bindingID := range bindingIDs {
		if bindingID <= 0 {
			continue
		}
		if _, exists := uniqueIDs[bindingID]; exists {
			continue
		}
		uniqueIDs[bindingID] = struct{}{}
		normalized = append(normalized, bindingID)
	}
	sort.Ints(normalized)
	return normalized
}

func loadReferralTemplateBundleRowsTx(tx *gorm.DB, templateID int) ([]ReferralTemplate, error) {
	if tx == nil {
		tx = DB
	}
	if templateID <= 0 {
		return nil, gorm.ErrRecordNotFound
	}

	var template ReferralTemplate
	if err := tx.First(&template, templateID).Error; err != nil {
		return nil, err
	}

	bundleKey := strings.TrimSpace(template.BundleKey)
	if bundleKey == "" || isSyntheticReferralTemplateBundleKey(bundleKey) {
		return []ReferralTemplate{template}, nil
	}

	var bundleRows []ReferralTemplate
	if err := tx.Where("bundle_key = ?", bundleKey).Order(commonGroupCol + " ASC, id ASC").Find(&bundleRows).Error; err != nil {
		return nil, err
	}
	if len(bundleRows) == 0 {
		return []ReferralTemplate{template}, nil
	}
	return bundleRows, nil
}

func subscriptionReferralTemplateCapBps(template ReferralTemplate) int {
	if template.LevelType == ReferralLevelTypeTeam {
		return template.TeamCapBps
	}
	return template.DirectCapBps
}

func findLowestEnabledSubscriptionReferralTemplateIDTx(tx *gorm.DB) (int, error) {
	if tx == nil {
		tx = DB
	}
	if tx == nil {
		return 0, errors.New("database is not initialized")
	}

	var rows []ReferralTemplate
	if err := tx.Where(
		"referral_type = ? AND enabled = ?",
		ReferralTypeSubscription,
		true,
	).Order("id ASC").Find(&rows).Error; err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, gorm.ErrRecordNotFound
	}

	type bundleCandidate struct {
		templateID int
		capBps     int
	}

	candidatesByBundleKey := make(map[string]bundleCandidate, len(rows))
	bundleKeys := make([]string, 0, len(rows))
	for _, row := range rows {
		bundleKey := referralTemplateBundleKeyForRow(row)
		candidate, exists := candidatesByBundleKey[bundleKey]
		rowCapBps := subscriptionReferralTemplateCapBps(row)
		if !exists {
			candidatesByBundleKey[bundleKey] = bundleCandidate{
				templateID: row.Id,
				capBps:     rowCapBps,
			}
			bundleKeys = append(bundleKeys, bundleKey)
			continue
		}
		if rowCapBps < candidate.capBps || (rowCapBps == candidate.capBps && row.Id < candidate.templateID) {
			candidatesByBundleKey[bundleKey] = bundleCandidate{
				templateID: row.Id,
				capBps:     rowCapBps,
			}
		}
	}

	sort.Slice(bundleKeys, func(i, j int) bool {
		left := candidatesByBundleKey[bundleKeys[i]]
		right := candidatesByBundleKey[bundleKeys[j]]
		if left.capBps != right.capBps {
			return left.capBps < right.capBps
		}
		return left.templateID < right.templateID
	})
	return candidatesByBundleKey[bundleKeys[0]].templateID, nil
}

func hasAnyActiveReferralTemplateBindingByUserTx(tx *gorm.DB, userID int, referralType string) (bool, error) {
	if tx == nil {
		tx = DB
	}
	if tx == nil {
		return false, errors.New("database is not initialized")
	}
	if userID <= 0 {
		return false, errors.New("invalid user id")
	}

	var count int64
	if err := tx.Model(&ReferralTemplateBinding{}).
		Joins("JOIN referral_templates ON referral_templates.id = referral_template_bindings.template_id").
		Where(
			"referral_template_bindings.user_id = ? AND referral_template_bindings.referral_type = ? AND referral_templates.referral_type = ? AND referral_templates.enabled = ?",
			userID,
			strings.TrimSpace(referralType),
			strings.TrimSpace(referralType),
			true,
		).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func upsertReferralTemplateBindingBundleForUserTx(tx *gorm.DB, userID int, referralType string, templateID int, replaceBindingIDs []int, operatorID int) ([]ReferralTemplateBinding, error) {
	if tx == nil {
		tx = DB
	}
	if tx == nil {
		return nil, errors.New("database is not initialized")
	}
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	if templateID <= 0 {
		return nil, errors.New("template_id is required")
	}

	bundleRows, err := loadReferralTemplateBundleRowsTx(tx, templateID)
	if err != nil {
		return nil, err
	}
	if len(bundleRows) == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	effectiveReferralType := strings.TrimSpace(referralType)
	if effectiveReferralType == "" {
		effectiveReferralType = strings.TrimSpace(bundleRows[0].ReferralType)
	}

	normalizedReplaceBindingIDs := normalizeBindingIDs(replaceBindingIDs)
	if len(normalizedReplaceBindingIDs) > 0 {
		if err := tx.Where(
			"id IN ? AND user_id = ? AND referral_type = ?",
			normalizedReplaceBindingIDs,
			userID,
			effectiveReferralType,
		).Delete(&ReferralTemplateBinding{}).Error; err != nil {
			return nil, err
		}
	}

	savedBindings := make([]ReferralTemplateBinding, 0, len(bundleRows))
	for _, row := range bundleRows {
		binding := ReferralTemplateBinding{
			UserId:       userID,
			ReferralType: effectiveReferralType,
			TemplateId:   row.Id,
			CreatedBy:    operatorID,
			UpdatedBy:    operatorID,
		}
		if err := binding.validateWithTemplateID(tx); err != nil {
			return nil, err
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "referral_type"},
				{Name: "group"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"template_id": binding.TemplateId,
				"updated_by":  binding.UpdatedBy,
				"updated_at":  common.GetTimestamp(),
			}),
		}).Create(&binding).Error; err != nil {
			return nil, err
		}
		if err := tx.Where(
			"user_id = ? AND referral_type = ? AND "+commonGroupCol+" = ?",
			binding.UserId,
			binding.ReferralType,
			binding.Group,
		).First(&binding).Error; err != nil {
			return nil, err
		}
		savedBindings = append(savedBindings, binding)
	}

	sort.Slice(savedBindings, func(i, j int) bool {
		return savedBindings[i].Group < savedBindings[j].Group
	})
	return savedBindings, nil
}

func AssignLowestSubscriptionReferralTemplateForInvitedUser(tx *gorm.DB, inviteeUserID int, inviterUserID int) error {
	if inviteeUserID <= 0 {
		return errors.New("invalid invitee user id")
	}
	if inviterUserID <= 0 {
		return nil
	}
	if !GetSubscriptionReferralGlobalSetting().AutoAssignInviteeTemplate {
		return nil
	}
	if tx == nil {
		tx = DB
	}
	if tx == nil {
		return errors.New("database is not initialized")
	}

	hasActiveBinding, err := hasAnyActiveReferralTemplateBindingByUserTx(tx, inviterUserID, ReferralTypeSubscription)
	if err != nil {
		return err
	}
	if !hasActiveBinding {
		return nil
	}

	templateID, err := findLowestEnabledSubscriptionReferralTemplateIDTx(tx)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	_, err = upsertReferralTemplateBindingBundleForUserTx(tx, inviteeUserID, ReferralTypeSubscription, templateID, nil, 0)
	return err
}

func UpsertReferralTemplateBinding(binding *ReferralTemplateBinding) (*ReferralTemplateBinding, error) {
	if binding == nil {
		return nil, errors.New("binding is required")
	}

	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := binding.validateWithTemplateID(tx); err != nil {
			return err
		}

		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "referral_type"},
				{Name: "group"},
			},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"template_id": binding.TemplateId,
				"updated_by":  binding.UpdatedBy,
				"updated_at":  common.GetTimestamp(),
			}),
		}).Create(binding).Error; err != nil {
			return err
		}

		return tx.Where(
			"user_id = ? AND referral_type = ? AND "+commonGroupCol+" = ?",
			binding.UserId,
			binding.ReferralType,
			binding.Group,
		).First(binding).Error
	})
	if err != nil {
		return nil, err
	}
	return binding, nil
}

func UpsertReferralTemplateBindingBundleForUser(userID int, referralType string, templateID int, replaceBindingIDs []int, operatorID int) ([]ReferralTemplateBinding, error) {
	if userID <= 0 {
		return nil, errors.New("invalid user id")
	}
	if templateID <= 0 {
		return nil, errors.New("template_id is required")
	}

	savedBindings := make([]ReferralTemplateBinding, 0)
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		savedBindings, err = upsertReferralTemplateBindingBundleForUserTx(tx, userID, referralType, templateID, replaceBindingIDs, operatorID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return savedBindings, nil
}

func syncReferralTemplateBundleBindingsForExistingUsersTx(tx *gorm.DB, retainedTemplateIDs []int, bundleRows []ReferralTemplate, operatorID int) error {
	if tx == nil {
		return errors.New("transaction is required")
	}

	normalizedTemplateIDs := normalizeBindingIDs(retainedTemplateIDs)
	if len(normalizedTemplateIDs) == 0 || len(bundleRows) == 0 {
		return nil
	}

	owners := make([]struct {
		UserId       int    `gorm:"column:user_id"`
		ReferralType string `gorm:"column:referral_type"`
	}, 0)
	if err := tx.Model(&ReferralTemplateBinding{}).
		Select("DISTINCT user_id, referral_type").
		Where("template_id IN ?", normalizedTemplateIDs).
		Scan(&owners).Error; err != nil {
		return err
	}

	for _, owner := range owners {
		if owner.UserId <= 0 {
			continue
		}
		for _, row := range bundleRows {
			if row.Id <= 0 {
				continue
			}
			binding := ReferralTemplateBinding{
				UserId:       owner.UserId,
				ReferralType: strings.TrimSpace(owner.ReferralType),
				TemplateId:   row.Id,
				CreatedBy:    operatorID,
				UpdatedBy:    operatorID,
			}
			if err := binding.validateWithTemplateID(tx); err != nil {
				return err
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{
					{Name: "user_id"},
					{Name: "referral_type"},
					{Name: "group"},
				},
				DoUpdates: clause.Assignments(map[string]interface{}{
					"template_id": binding.TemplateId,
					"updated_by":  binding.UpdatedBy,
					"updated_at":  common.GetTimestamp(),
				}),
			}).Create(&binding).Error; err != nil {
				return err
			}
		}
	}

	return nil
}
