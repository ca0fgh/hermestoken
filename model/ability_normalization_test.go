package model

import (
	"testing"

	"github.com/ca0fgh/hermestoken/common"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestUpdateAbilitiesNormalizesModelsAndGroups(t *testing.T) {
	originalDB := DB
	originalSQLite := common.UsingSQLite
	originalMySQL := common.UsingMySQL
	originalPostgres := common.UsingPostgreSQL
	defer func() {
		DB = originalDB
		common.UsingSQLite = originalSQLite
		common.UsingMySQL = originalMySQL
		common.UsingPostgreSQL = originalPostgres
	}()

	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	if err := db.AutoMigrate(&Ability{}); err != nil {
		t.Fatalf("failed to migrate abilities table: %v", err)
	}

	DB = db
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	InitColumnMetadata()

	priority := int64(0)
	weight := uint(0)
	channel := &Channel{
		Id:       11,
		Group:    " cc-opus4.6-福利渠道 , default , cc-opus4.6-福利渠道 ",
		Models:   " claude-opus-4-6 , claude-sonnet-4-6, claude-opus-4-6 ",
		Status:   common.ChannelStatusEnabled,
		Priority: &priority,
		Weight:   &weight,
	}

	if err := channel.UpdateAbilities(nil); err != nil {
		t.Fatalf("UpdateAbilities returned error: %v", err)
	}

	var abilities []Ability
	if err := DB.Where("channel_id = ?", channel.Id).Find(&abilities).Error; err != nil {
		t.Fatalf("failed to load abilities: %v", err)
	}

	if len(abilities) != 4 {
		t.Fatalf("expected 4 normalized abilities, got %d", len(abilities))
	}

	got := make(map[string]struct{}, len(abilities))
	for _, ability := range abilities {
		got[ability.Group+"|"+ability.Model] = struct{}{}
	}

	expected := []string{
		"cc-opus4.6-福利渠道|claude-opus-4-6",
		"cc-opus4.6-福利渠道|claude-sonnet-4-6",
		"default|claude-opus-4-6",
		"default|claude-sonnet-4-6",
	}
	for _, key := range expected {
		if _, ok := got[key]; !ok {
			t.Fatalf("missing normalized ability %q, got %#v", key, got)
		}
	}
}
