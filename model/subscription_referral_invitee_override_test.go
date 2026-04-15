package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupSubscriptionReferralInviteeOverrideMigrationDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := DB
	originalLogDB := LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalRedisEnabled := common.RedisEnabled
	originalBatchUpdateEnabled := common.BatchUpdateEnabled
	originalCommonGroupCol := commonGroupCol

	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	common.BatchUpdateEnabled = false
	commonGroupCol = "`group`"

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	DB = db
	LOG_DB = db

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		DB = originalDB
		LOG_DB = originalLogDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.RedisEnabled = originalRedisEnabled
		common.BatchUpdateEnabled = originalBatchUpdateEnabled
		commonGroupCol = originalCommonGroupCol
	})

	return db
}

func TestUpsertSubscriptionReferralInviteeOverrideStoresGroupedRate(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	if err := db.AutoMigrate(&SubscriptionReferralInviteeOverride{}); err != nil {
		t.Fatalf("failed to migrate invitee override table: %v", err)
	}

	inviter := seedReferralUser(t, db, "invitee-override-inviter", 0, dto.UserSetting{})
	invitee := seedReferralUser(t, db, "invitee-override-target", inviter.Id, dto.UserSetting{})

	defaultOverride, err := UpsertSubscriptionReferralInviteeOverride(inviter.Id, invitee.Id, " default ", 1400)
	if err != nil {
		t.Fatalf("UpsertSubscriptionReferralInviteeOverride(default) error = %v", err)
	}
	if defaultOverride.Group != "default" {
		t.Fatalf("default override Group = %q, want %q", defaultOverride.Group, "default")
	}
	if defaultOverride.InviteeRateBps != 1400 {
		t.Fatalf("default override InviteeRateBps = %d, want 1400", defaultOverride.InviteeRateBps)
	}

	if _, err := UpsertSubscriptionReferralInviteeOverride(inviter.Id, invitee.Id, "vip", 900); err != nil {
		t.Fatalf("UpsertSubscriptionReferralInviteeOverride(vip) error = %v", err)
	}

	overrides, err := ListSubscriptionReferralInviteeOverrides(inviter.Id, invitee.Id)
	if err != nil {
		t.Fatalf("ListSubscriptionReferralInviteeOverrides() error = %v", err)
	}
	if len(overrides) != 2 {
		t.Fatalf("override count = %d, want 2", len(overrides))
	}
	if overrides[0].Group != "default" || overrides[1].Group != "vip" {
		t.Fatalf("override groups = [%q %q], want [default vip]", overrides[0].Group, overrides[1].Group)
	}

	if err := DeleteSubscriptionReferralInviteeOverride(inviter.Id, invitee.Id, "default"); err != nil {
		t.Fatalf("DeleteSubscriptionReferralInviteeOverride(default) error = %v", err)
	}

	overrides, err = ListSubscriptionReferralInviteeOverrides(inviter.Id, invitee.Id)
	if err != nil {
		t.Fatalf("ListSubscriptionReferralInviteeOverrides(after delete) error = %v", err)
	}
	if len(overrides) != 1 || overrides[0].Group != "vip" {
		t.Fatalf("overrides after delete = %+v, want only vip", overrides)
	}
}

func TestSubscriptionReferralInviteeOverrideRejectsNonOwnedInvitee(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	if err := db.AutoMigrate(&SubscriptionReferralInviteeOverride{}); err != nil {
		t.Fatalf("failed to migrate invitee override table: %v", err)
	}

	inviter := seedReferralUser(t, db, "ownership-inviter", 0, dto.UserSetting{})
	otherInviter := seedReferralUser(t, db, "ownership-other-inviter", 0, dto.UserSetting{})
	nonOwnedInvitee := seedReferralUser(t, db, "ownership-invitee", otherInviter.Id, dto.UserSetting{})

	_, err := UpsertSubscriptionReferralInviteeOverride(inviter.Id, nonOwnedInvitee.Id, "vip", 800)
	if err == nil || !strings.Contains(err.Error(), "does not belong to inviter") {
		t.Fatalf("UpsertSubscriptionReferralInviteeOverride() error = %v, want ownership validation error", err)
	}

	if err := DeleteSubscriptionReferralInviteeOverride(inviter.Id, nonOwnedInvitee.Id, "vip"); err == nil || !strings.Contains(err.Error(), "does not belong to inviter") {
		t.Fatalf("DeleteSubscriptionReferralInviteeOverride() error = %v, want ownership validation error", err)
	}

	if _, err := ListSubscriptionReferralInviteeOverrides(inviter.Id, nonOwnedInvitee.Id); err == nil || !strings.Contains(err.Error(), "does not belong to inviter") {
		t.Fatalf("ListSubscriptionReferralInviteeOverrides() error = %v, want ownership validation error", err)
	}
}

func TestListSubscriptionReferralInviteeOverrideCountsBatchesInvitees(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)
	if err := db.AutoMigrate(&SubscriptionReferralInviteeOverride{}); err != nil {
		t.Fatalf("failed to migrate invitee override table: %v", err)
	}

	inviter := seedReferralUser(t, db, "batch-count-inviter", 0, dto.UserSetting{})
	firstInvitee := seedReferralUser(t, db, "batch-count-first", inviter.Id, dto.UserSetting{})
	secondInvitee := seedReferralUser(t, db, "batch-count-second", inviter.Id, dto.UserSetting{})
	thirdInvitee := seedReferralUser(t, db, "batch-count-third", inviter.Id, dto.UserSetting{})

	if _, err := UpsertSubscriptionReferralInviteeOverride(inviter.Id, firstInvitee.Id, "default", 100); err != nil {
		t.Fatalf("failed to create first invitee default override: %v", err)
	}
	if _, err := UpsertSubscriptionReferralInviteeOverride(inviter.Id, firstInvitee.Id, "vip", 200); err != nil {
		t.Fatalf("failed to create first invitee vip override: %v", err)
	}
	if _, err := UpsertSubscriptionReferralInviteeOverride(inviter.Id, secondInvitee.Id, "vip", 300); err != nil {
		t.Fatalf("failed to create second invitee vip override: %v", err)
	}

	counts, err := ListSubscriptionReferralInviteeOverrideCounts(inviter.Id, []int{firstInvitee.Id, secondInvitee.Id, secondInvitee.Id, thirdInvitee.Id})
	if err != nil {
		t.Fatalf("ListSubscriptionReferralInviteeOverrideCounts() error = %v", err)
	}
	if counts[firstInvitee.Id] != 2 {
		t.Fatalf("first invitee count = %d, want 2", counts[firstInvitee.Id])
	}
	if counts[secondInvitee.Id] != 1 {
		t.Fatalf("second invitee count = %d, want 1", counts[secondInvitee.Id])
	}
	if counts[thirdInvitee.Id] != 0 {
		t.Fatalf("third invitee count = %d, want 0", counts[thirdInvitee.Id])
	}
}

func TestListSubscriptionReferralInviteeContributionSummariesUsesNetInviterReward(t *testing.T) {
	db := setupSubscriptionReferralSettlementDB(t)

	inviter := seedReferralUser(t, db, "contribution-inviter", 0, dto.UserSetting{})
	alice := seedReferralUser(t, db, "alice-contrib", inviter.Id, dto.UserSetting{})
	bob := seedReferralUser(t, db, "bob-contrib", inviter.Id, dto.UserSetting{})
	charlie := seedReferralUser(t, db, "charlie-contrib", inviter.Id, dto.UserSetting{})
	if err := db.Model(&User{}).Where("id = ?", alice.Id).Update("group", "starter").Error; err != nil {
		t.Fatalf("failed to update alice group: %v", err)
	}
	if err := db.Model(&User{}).Where("id = ?", bob.Id).Update("group", "vip").Error; err != nil {
		t.Fatalf("failed to update bob group: %v", err)
	}
	if err := db.Model(&User{}).Where("id = ?", charlie.Id).Update("group", "enterprise").Error; err != nil {
		t.Fatalf("failed to update charlie group: %v", err)
	}

	records := []SubscriptionReferralRecord{
		{OrderId: 1, OrderTradeNo: "trade-alice-1", ReferralGroup: "vip", PayerUserId: alice.Id, InviterUserId: inviter.Id, BeneficiaryUserId: inviter.Id, BeneficiaryRole: SubscriptionReferralBeneficiaryRoleInviter, RewardQuota: 500, ReversedQuota: 100, DebtQuota: 40, Status: SubscriptionReferralStatusPartialRevert},
		{OrderId: 2, OrderTradeNo: "trade-alice-2", ReferralGroup: "vip", PayerUserId: alice.Id, InviterUserId: inviter.Id, BeneficiaryUserId: inviter.Id, BeneficiaryRole: SubscriptionReferralBeneficiaryRoleInviter, RewardQuota: 200, ReversedQuota: 0, DebtQuota: 0, Status: SubscriptionReferralStatusCredited},
		{OrderId: 3, OrderTradeNo: "trade-bob-1", ReferralGroup: "vip", PayerUserId: bob.Id, InviterUserId: inviter.Id, BeneficiaryUserId: inviter.Id, BeneficiaryRole: SubscriptionReferralBeneficiaryRoleInviter, RewardQuota: 700, ReversedQuota: 50, DebtQuota: 50, Status: SubscriptionReferralStatusPartialRevert},
		{OrderId: 4, OrderTradeNo: "trade-ignore-invitee", ReferralGroup: "vip", PayerUserId: alice.Id, InviterUserId: inviter.Id, BeneficiaryUserId: alice.Id, BeneficiaryRole: SubscriptionReferralBeneficiaryRoleInvitee, RewardQuota: 999, ReversedQuota: 0, DebtQuota: 0, Status: SubscriptionReferralStatusCredited},
	}
	for i := range records {
		if err := db.Create(&records[i]).Error; err != nil {
			t.Fatalf("failed to seed referral record %d: %v", i, err)
		}
	}

	pageInfo := &common.PageInfo{Page: 1, PageSize: 3}
	summaries, total, contributionTotal, err := ListSubscriptionReferralInviteeContributionSummaries(inviter.Id, "", pageInfo)
	if err != nil {
		t.Fatalf("ListSubscriptionReferralInviteeContributionSummaries() error = %v", err)
	}
	if total != 3 {
		t.Fatalf("total summaries = %d, want 3", total)
	}
	if contributionTotal != 1160 {
		t.Fatalf("contributionTotal = %d, want 1160", contributionTotal)
	}
	if len(summaries) != 3 {
		t.Fatalf("summary count = %d, want 3", len(summaries))
	}
	if summaries[0].InviteeUserId != alice.Id || summaries[0].ContributionQuota != 560 {
		t.Fatalf("first summary = %+v, want invitee %d contribution 560", summaries[0], alice.Id)
	}
	if summaries[0].InviteeGroup != "starter" {
		t.Fatalf("first summary group = %q, want starter", summaries[0].InviteeGroup)
	}
	if summaries[1].InviteeUserId != bob.Id || summaries[1].ContributionQuota != 600 {
		t.Fatalf("second summary = %+v, want invitee %d contribution 600", summaries[1], bob.Id)
	}
	if summaries[1].InviteeGroup != "vip" {
		t.Fatalf("second summary group = %q, want vip", summaries[1].InviteeGroup)
	}
	if summaries[2].InviteeUserId != charlie.Id || summaries[2].ContributionQuota != 0 {
		t.Fatalf("third summary = %+v, want invitee %d contribution 0", summaries[2], charlie.Id)
	}
	if summaries[2].InviteeGroup != "enterprise" {
		t.Fatalf("third summary group = %q, want enterprise", summaries[2].InviteeGroup)
	}

	filteredByUsername, filteredTotal, filteredContributionTotal, err := ListSubscriptionReferralInviteeContributionSummaries(inviter.Id, "bob", &common.PageInfo{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListSubscriptionReferralInviteeContributionSummaries(username) error = %v", err)
	}
	if filteredTotal != 1 || filteredContributionTotal != 600 || len(filteredByUsername) != 1 || filteredByUsername[0].InviteeUserId != bob.Id {
		t.Fatalf("filtered by username = %+v total=%d contributionTotal=%d, want only bob/600", filteredByUsername, filteredTotal, filteredContributionTotal)
	}

	filteredByID, filteredIDTotal, filteredIDContributionTotal, err := ListSubscriptionReferralInviteeContributionSummaries(inviter.Id, fmt.Sprintf("%d", charlie.Id), &common.PageInfo{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListSubscriptionReferralInviteeContributionSummaries(id) error = %v", err)
	}
	if filteredIDTotal != 1 || filteredIDContributionTotal != 0 || len(filteredByID) != 1 || filteredByID[0].InviteeUserId != charlie.Id {
		t.Fatalf("filtered by id = %+v total=%d contributionTotal=%d, want only charlie/0", filteredByID, filteredIDTotal, filteredIDContributionTotal)
	}

	deletedInvitee := seedReferralUser(t, db, "deleted-contrib", inviter.Id, dto.UserSetting{})
	if err := db.Delete(deletedInvitee).Error; err != nil {
		t.Fatalf("failed to soft-delete invitee: %v", err)
	}

	summariesAfterDelete, totalAfterDelete, _, err := ListSubscriptionReferralInviteeContributionSummaries(inviter.Id, "", &common.PageInfo{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListSubscriptionReferralInviteeContributionSummaries(after delete) error = %v", err)
	}
	if totalAfterDelete != 3 {
		t.Fatalf("total summaries after delete = %d, want 3", totalAfterDelete)
	}
	for _, summary := range summariesAfterDelete {
		if summary.InviteeUserId == deletedInvitee.Id {
			t.Fatalf("deleted invitee %d unexpectedly present in summaries: %+v", deletedInvitee.Id, summariesAfterDelete)
		}
	}
}

func TestMigrateDBCreatesSubscriptionReferralInviteeOverrideTable(t *testing.T) {
	db := setupSubscriptionReferralInviteeOverrideMigrationDB(t)

	if err := migrateDB(); err != nil {
		t.Fatalf("migrateDB() error = %v", err)
	}
	if !db.Migrator().HasTable(&SubscriptionReferralInviteeOverride{}) {
		t.Fatal("expected subscription_referral_invitee_overrides table to exist after migrateDB()")
	}
}
