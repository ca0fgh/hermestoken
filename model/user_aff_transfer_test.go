package model

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

func setupAffTransferTest(t *testing.T) *User {
	t.Helper()

	db := setupWithdrawalModelDB(t)
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() {
		common.QuotaPerUnit = originalQuotaPerUnit
	})

	user := seedWithdrawalUser(t, db, "aff_transfer_user", 1000)
	user.AffQuota = 300
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("failed to seed aff quota: %v", err)
	}

	return user
}

func TestTransferAffQuotaToQuotaMovesQuotaAtomically(t *testing.T) {
	user := setupAffTransferTest(t)

	if err := user.TransferAffQuotaToQuota(150); err != nil {
		t.Fatalf("transfer aff quota: %v", err)
	}

	var stored User
	if err := DB.First(&stored, user.Id).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if stored.AffQuota != 150 {
		t.Fatalf("aff_quota = %d, want 150", stored.AffQuota)
	}
	if stored.Quota != 1150 {
		t.Fatalf("quota = %d, want 1150", stored.Quota)
	}
}

func TestTransferAffQuotaToQuotaRejectsInsufficientQuotaWithoutPartialUpdate(t *testing.T) {
	user := setupAffTransferTest(t)

	if err := user.TransferAffQuotaToQuota(400); err == nil {
		t.Fatal("expected insufficient aff quota error")
	}

	var stored User
	if err := DB.First(&stored, user.Id).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if stored.AffQuota != 300 {
		t.Fatalf("aff_quota = %d, want 300", stored.AffQuota)
	}
	if stored.Quota != 1000 {
		t.Fatalf("quota = %d, want 1000", stored.Quota)
	}
}

func TestTransferAffQuotaToQuotaRepeatedRequestsCannotOverTransfer(t *testing.T) {
	user := setupAffTransferTest(t)

	if err := user.TransferAffQuotaToQuota(200); err != nil {
		t.Fatalf("first transfer aff quota: %v", err)
	}
	if err := user.TransferAffQuotaToQuota(200); err == nil {
		t.Fatal("expected second transfer to fail")
	}

	var stored User
	if err := DB.First(&stored, user.Id).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if stored.AffQuota != 100 {
		t.Fatalf("aff_quota = %d, want 100", stored.AffQuota)
	}
	if stored.Quota != 1200 {
		t.Fatalf("quota = %d, want 1200", stored.Quota)
	}
}

func TestTransferAffQuotaToQuotaConcurrentRequestsCannotDoubleSpend(t *testing.T) {
	user := setupAffTransferTest(t)

	sqlDB, err := DB.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(2)

	const callbackName = "aff_transfer_concurrent_update_barrier"
	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	var callbackHits atomic.Int32

	if err := DB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Table != "users" {
			return
		}
		if callbackHits.Add(1) > 2 {
			return
		}
		ready <- struct{}{}
		<-release
	}); err != nil {
		t.Fatalf("register update callback: %v", err)
	}
	t.Cleanup(func() {
		_ = DB.Callback().Update().Remove(callbackName)
	})

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- (&User{Id: user.Id}).TransferAffQuotaToQuota(200)
		}()
	}

	for i := 0; i < 2; i++ {
		select {
		case <-ready:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("timed out waiting for concurrent transfer updates")
		}
	}
	close(release)
	wg.Wait()
	close(errs)

	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("successful transfers = %d, want 1", successes)
	}

	var stored User
	if err := DB.First(&stored, user.Id).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if stored.AffQuota != 100 {
		t.Fatalf("aff_quota = %d, want 100", stored.AffQuota)
	}
	if stored.Quota != 1200 {
		t.Fatalf("quota = %d, want 1200", stored.Quota)
	}
}
