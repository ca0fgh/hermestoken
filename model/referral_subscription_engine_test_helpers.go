package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/common"
)

type seedTemplateEngineFixtureInput struct {
	Group                 string
	ImmediateInviterLevel string
	AncestorLevels        []string
}

type templateEngineFixture struct {
	PayerUser        *User
	ImmediateInviter *User
	Ancestors        []*User
}

func seedTemplateEngineFixture(t *testing.T, input seedTemplateEngineFixtureInput) *templateEngineFixture {
	t.Helper()

	db := setupReferralTemplateDB(t)
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("failed to migrate users: %v", err)
	}

	payer := &User{
		Id:       501,
		Username: "template_payer",
		Password: "password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		AffCode:  "template_payer_code",
	}
	if err := db.Create(payer).Error; err != nil {
		t.Fatalf("failed to create payer: %v", err)
	}

	immediateInviter := &User{
		Id:       401,
		Username: "template_immediate_inviter",
		Password: "password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		AffCode:  "template_immediate_inviter_code",
	}
	if err := db.Create(immediateInviter).Error; err != nil {
		t.Fatalf("failed to create immediate inviter: %v", err)
	}

	payer.InviterId = immediateInviter.Id
	if err := db.Model(&User{}).Where("id = ?", payer.Id).Update("inviter_id", payer.InviterId).Error; err != nil {
		t.Fatalf("failed to update payer inviter id: %v", err)
	}

	createTemplateBindingForUser(t, immediateInviter.Id, input.Group, input.ImmediateInviterLevel)

	fixture := &templateEngineFixture{
		PayerUser:        payer,
		ImmediateInviter: immediateInviter,
		Ancestors:        make([]*User, 0, len(input.AncestorLevels)),
	}

	parentID := immediateInviter.Id
	nextID := 301
	for idx, levelType := range input.AncestorLevels {
		ancestor := &User{
			Id:       nextID,
			Username: fmt.Sprintf("template_ancestor_%d", idx+1),
			Password: "password",
			Role:     common.RoleCommonUser,
			Status:   common.UserStatusEnabled,
			AffCode:  fmt.Sprintf("template_ancestor_%d_code", idx+1),
		}
		if err := db.Create(ancestor).Error; err != nil {
			t.Fatalf("failed to create ancestor: %v", err)
		}

		if err := db.Model(&User{}).Where("id = ?", parentID).Update("inviter_id", ancestor.Id).Error; err != nil {
			t.Fatalf("failed to update inviter chain: %v", err)
		}

		createTemplateBindingForUser(t, ancestor.Id, input.Group, levelType)
		fixture.Ancestors = append(fixture.Ancestors, ancestor)
		parentID = ancestor.Id
		nextID--
	}

	return fixture
}

func createTemplateBindingForUser(t *testing.T, userID int, group string, levelType string) {
	t.Helper()

	template := &ReferralTemplate{
		Name:                   fmt.Sprintf("%s_template_%d", levelType, userID),
		ReferralType:           ReferralTypeSubscription,
		Group:                  group,
		LevelType:              levelType,
		Enabled:                true,
		DirectCapBps:           1000,
		TeamCapBps:             2500,
		TeamDecayRatio:         0.5,
		TeamMaxDepth:           5,
		InviteeShareDefaultBps: 800,
	}
	if err := CreateReferralTemplate(template); err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	binding := &ReferralTemplateBinding{
		UserId:       userID,
		ReferralType: ReferralTypeSubscription,
		Group:        group,
		TemplateId:   template.Id,
		CreatedBy:    userID,
		UpdatedBy:    userID,
	}
	if _, err := UpsertReferralTemplateBinding(binding); err != nil {
		t.Fatalf("failed to create template binding: %v", err)
	}
}
