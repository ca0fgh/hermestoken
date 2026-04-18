package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

const (
	demoUserPrefix  = "rebate_demo_"
	demoTradePrefix = "demo_referral_"
	demoPlanPrefix  = "邀请返佣演示-"
)

type demoBindingSeed struct {
	Group                  string
	Name                   string
	LevelType              string
	DirectCapBps           int
	TeamCapBps             int
	InviteeShareDefaultBps int
}

type demoOrderSeed struct {
	Group string
	Money float64
	Code  string
}

type demoInviteeSeed struct {
	Username    string
	DisplayName string
	Orders      []demoOrderSeed
	Overrides   map[string]int
}

func main() {
	dbPath := flag.String("db", os.Getenv("SQLITE_PATH"), "sqlite db path")
	quotaPerUnit := flag.Float64("quota-per-unit", 100, "quota per unit snapshot used for demo settlement")
	flag.Parse()

	if strings.TrimSpace(*dbPath) == "" {
		fmt.Fprintln(os.Stderr, "missing sqlite db path, pass --db or set SQLITE_PATH")
		os.Exit(1)
	}

	if err := seedInviteRebateDemo(*dbPath, *quotaPerUnit); err != nil {
		fmt.Fprintf(os.Stderr, "seed invite rebate demo failed: %v\n", err)
		os.Exit(1)
	}
}

func seedInviteRebateDemo(dbPath string, quotaPerUnit float64) error {
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.QuotaPerUnit = quotaPerUnit
	model.InitColumnMetadata()

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return err
	}
	model.DB = db
	model.LOG_DB = db

	root, err := model.GetUserById(1, false)
	if err != nil {
		return fmt.Errorf("load root user: %w", err)
	}

	if err := cleanupDemoData(db, root.Id); err != nil {
		return err
	}

	bindings := []demoBindingSeed{
		{
			Group:                  "default",
			Name:                   "邀请返佣演示-default-直推模板",
			LevelType:              model.ReferralLevelTypeDirect,
			DirectCapBps:           4500,
			TeamCapBps:             0,
			InviteeShareDefaultBps: 1000,
		},
		{
			Group:                  "vip",
			Name:                   "邀请返佣演示-vip-团队模板",
			LevelType:              model.ReferralLevelTypeTeam,
			DirectCapBps:           0,
			TeamCapBps:             3200,
			InviteeShareDefaultBps: 800,
		},
		{
			Group:                  "svip",
			Name:                   "邀请返佣演示-svip-直推模板",
			LevelType:              model.ReferralLevelTypeDirect,
			DirectCapBps:           6000,
			TeamCapBps:             0,
			InviteeShareDefaultBps: 1500,
		},
	}

	for _, bindingSeed := range bindings {
		template, err := ensureDemoTemplate(db, root.Id, bindingSeed)
		if err != nil {
			return err
		}
		if _, err := model.UpsertReferralTemplateBinding(&model.ReferralTemplateBinding{
			UserId:       root.Id,
			ReferralType: model.ReferralTypeSubscription,
			Group:        bindingSeed.Group,
			TemplateId:   template.Id,
			CreatedBy:    root.Id,
			UpdatedBy:    root.Id,
		}); err != nil {
			return fmt.Errorf("upsert root template binding for %s: %w", bindingSeed.Group, err)
		}
	}

	planByGroup := make(map[string]*model.SubscriptionPlan, len(bindings))
	for _, bindingSeed := range bindings {
		plan, err := ensureDemoPlan(db, bindingSeed.Group)
		if err != nil {
			return err
		}
		planByGroup[bindingSeed.Group] = plan
	}

	invitees := []demoInviteeSeed{
		{
			Username:    demoUserPrefix + "aurora",
			DisplayName: "Aurora",
			Orders: []demoOrderSeed{
				{Group: "default", Money: 29.9, Code: "aurora-default-1"},
				{Group: "svip", Money: 59.9, Code: "aurora-svip-1"},
			},
			Overrides: map[string]int{
				"default": 1800,
				"svip":    2200,
			},
		},
		{
			Username:    demoUserPrefix + "blake",
			DisplayName: "Blake",
			Orders: []demoOrderSeed{
				{Group: "vip", Money: 36.9, Code: "blake-vip-1"},
			},
			Overrides: map[string]int{},
		},
		{
			Username:    demoUserPrefix + "cindy",
			DisplayName: "Cindy",
			Orders: []demoOrderSeed{
				{Group: "default", Money: 18.8, Code: "cindy-default-1"},
				{Group: "vip", Money: 15.5, Code: "cindy-vip-1"},
			},
			Overrides: map[string]int{
				"vip": 1200,
			},
		},
		{
			Username:    demoUserPrefix + "derek",
			DisplayName: "Derek",
			Orders: []demoOrderSeed{
				{Group: "vip", Money: 9.9, Code: "derek-vip-1"},
			},
			Overrides: map[string]int{},
		},
	}

	createdInvitees := make([]*model.User, 0, len(invitees))
	for _, inviteeSeed := range invitees {
		invitee, err := ensureDemoInvitee(db, root.Id, inviteeSeed)
		if err != nil {
			return err
		}
		createdInvitees = append(createdInvitees, invitee)
		for _, orderSeed := range inviteeSeed.Orders {
			plan := planByGroup[orderSeed.Group]
			if plan == nil {
				return fmt.Errorf("missing plan for group %s", orderSeed.Group)
			}
			if err := createAndCompleteDemoOrder(invitee.Id, plan, orderSeed); err != nil {
				return err
			}
		}
		for group, inviteeShareBps := range inviteeSeed.Overrides {
			if _, err := model.UpsertReferralInviteeShareOverride(
				root.Id,
				invitee.Id,
				model.ReferralTypeSubscription,
				group,
				inviteeShareBps,
				root.Id,
			); err != nil {
				return fmt.Errorf("upsert invitee override for %s/%s: %w", invitee.Username, group, err)
			}
		}
	}

	if err := db.Model(&model.User{}).
		Where("id = ?", root.Id).
		Updates(map[string]any{
			"aff_count": len(createdInvitees),
		}).Error; err != nil {
		return fmt.Errorf("update root aff_count: %w", err)
	}

	fmt.Printf("seeded invite rebate demo data into %s\n", dbPath)
	fmt.Printf("root user: %s (id=%d)\n", root.Username, root.Id)
	for _, invitee := range createdInvitees {
		fmt.Printf("- %s (id=%d, group=%s)\n", invitee.Username, invitee.Id, invitee.Group)
	}
	return nil
}

func cleanupDemoData(db *gorm.DB, rootUserID int) error {
	var demoUsers []model.User
	if err := db.Where("username LIKE ?", demoUserPrefix+"%").Find(&demoUsers).Error; err != nil {
		return fmt.Errorf("load demo users: %w", err)
	}

	demoUserIDs := make([]int, 0, len(demoUsers))
	for _, user := range demoUsers {
		demoUserIDs = append(demoUserIDs, user.Id)
	}

	var demoBatches []model.ReferralSettlementBatch
	if err := db.Where("source_trade_no LIKE ?", demoTradePrefix+"%").Find(&demoBatches).Error; err != nil {
		return fmt.Errorf("load demo batches: %w", err)
	}
	demoBatchIDs := make([]int, 0, len(demoBatches))
	for _, batch := range demoBatches {
		demoBatchIDs = append(demoBatchIDs, batch.Id)
	}

	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.User{}).
			Where("id = ?", rootUserID).
			Updates(map[string]any{
				"aff_quota":   0,
				"aff_history": 0,
				"aff_count":   0,
			}).Error; err != nil {
			return err
		}

		if len(demoBatchIDs) > 0 {
			if err := tx.Where("batch_id IN ?", demoBatchIDs).Delete(&model.ReferralSettlementRecord{}).Error; err != nil {
				return err
			}
			if err := tx.Where("id IN ?", demoBatchIDs).Delete(&model.ReferralSettlementBatch{}).Error; err != nil {
				return err
			}
		}

		if len(demoUserIDs) > 0 {
			if err := tx.Where("invitee_user_id IN ?", demoUserIDs).Delete(&model.ReferralInviteeShareOverride{}).Error; err != nil {
				return err
			}
			if err := tx.Where("user_id IN ?", demoUserIDs).Delete(&model.UserSubscription{}).Error; err != nil {
				return err
			}
		}

		if err := tx.Where("trade_no LIKE ?", demoTradePrefix+"%").Delete(&model.TopUp{}).Error; err != nil {
			return err
		}
		if err := tx.Where("trade_no LIKE ?", demoTradePrefix+"%").Delete(&model.SubscriptionOrder{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where("title LIKE ?", demoPlanPrefix+"%").Delete(&model.SubscriptionPlan{}).Error; err != nil {
			return err
		}
		if len(demoUserIDs) > 0 {
			if err := tx.Unscoped().Where("id IN ?", demoUserIDs).Delete(&model.User{}).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func ensureDemoTemplate(db *gorm.DB, operatorID int, seed demoBindingSeed) (*model.ReferralTemplate, error) {
	var template model.ReferralTemplate
	err := db.Where("name = ?", seed.Name).First(&template).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		template = model.ReferralTemplate{
			ReferralType:           model.ReferralTypeSubscription,
			Group:                  seed.Group,
			Name:                   seed.Name,
			LevelType:              seed.LevelType,
			Enabled:                true,
			DirectCapBps:           seed.DirectCapBps,
			TeamCapBps:             seed.TeamCapBps,
			InviteeShareDefaultBps: seed.InviteeShareDefaultBps,
			CreatedBy:              operatorID,
			UpdatedBy:              operatorID,
		}
		if err := model.CreateReferralTemplate(&template); err != nil {
			return nil, fmt.Errorf("create template %s: %w", seed.Name, err)
		}
		return &template, nil
	}
	if err != nil {
		return nil, err
	}

	template.ReferralType = model.ReferralTypeSubscription
	template.Group = seed.Group
	template.LevelType = seed.LevelType
	template.Enabled = true
	template.DirectCapBps = seed.DirectCapBps
	template.TeamCapBps = seed.TeamCapBps
	template.InviteeShareDefaultBps = seed.InviteeShareDefaultBps
	template.UpdatedBy = operatorID
	if err := model.UpdateReferralTemplate(&template); err != nil {
		return nil, fmt.Errorf("update template %s: %w", seed.Name, err)
	}
	return &template, nil
}

func ensureDemoPlan(db *gorm.DB, group string) (*model.SubscriptionPlan, error) {
	title := demoPlanPrefix + group + "-月付"
	var plan model.SubscriptionPlan
	err := db.Where("title = ?", title).First(&plan).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		plan = model.SubscriptionPlan{
			Title:         title,
			Subtitle:      "邀请返佣页面演示数据",
			PriceAmount:   9.9,
			Currency:      "USD",
			DurationUnit:  model.SubscriptionDurationMonth,
			DurationValue: 1,
			Enabled:       true,
			UpgradeGroup:  group,
		}
		if err := db.Create(&plan).Error; err != nil {
			return nil, fmt.Errorf("create plan %s: %w", title, err)
		}
		model.InvalidateSubscriptionPlanCache(plan.Id)
		return &plan, nil
	}
	if err != nil {
		return nil, err
	}

	plan.Subtitle = "邀请返佣页面演示数据"
	plan.Enabled = true
	plan.UpgradeGroup = group
	if err := db.Save(&plan).Error; err != nil {
		return nil, fmt.Errorf("update plan %s: %w", title, err)
	}
	model.InvalidateSubscriptionPlanCache(plan.Id)
	return &plan, nil
}

func ensureDemoInvitee(db *gorm.DB, inviterID int, seed demoInviteeSeed) (*model.User, error) {
	var invitee model.User
	err := db.Where("username = ?", seed.Username).First(&invitee).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		invitee = model.User{
			Username:    seed.Username,
			DisplayName: seed.DisplayName,
			Password:    "password123",
			Role:        common.RoleCommonUser,
			Status:      common.UserStatusEnabled,
			Group:       "default",
			AffCode:     seed.Username + "_code",
			InviterId:   inviterID,
		}
		if err := db.Create(&invitee).Error; err != nil {
			return nil, fmt.Errorf("create invitee %s: %w", seed.Username, err)
		}
		return &invitee, nil
	}
	if err != nil {
		return nil, err
	}

	invitee.DisplayName = seed.DisplayName
	invitee.InviterId = inviterID
	invitee.Status = common.UserStatusEnabled
	invitee.Role = common.RoleCommonUser
	if err := db.Save(&invitee).Error; err != nil {
		return nil, fmt.Errorf("update invitee %s: %w", seed.Username, err)
	}
	return &invitee, nil
}

func createAndCompleteDemoOrder(userID int, plan *model.SubscriptionPlan, orderSeed demoOrderSeed) error {
	if plan == nil {
		return errors.New("plan is required")
	}

	order := &model.SubscriptionOrder{
		UserId:        userID,
		PlanId:        plan.Id,
		Money:         orderSeed.Money,
		TradeNo:       demoTradePrefix + orderSeed.Code,
		PaymentMethod: "demo",
		Status:        common.TopUpStatusPending,
		CreateTime:    common.GetTimestamp(),
	}
	if err := order.Insert(); err != nil {
		return fmt.Errorf("create order %s: %w", order.TradeNo, err)
	}
	if err := model.CompleteSubscriptionOrder(order.TradeNo, `{"demo":true}`); err != nil {
		return fmt.Errorf("complete order %s: %w", order.TradeNo, err)
	}
	return nil
}
