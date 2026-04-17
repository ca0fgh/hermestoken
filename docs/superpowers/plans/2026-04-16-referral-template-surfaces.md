# Referral Template Admin and User Surfaces Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the admin and user-facing surfaces required to operate the new referral template framework, including template management, user template binding, inviter relationship editing, and the subscription invite-rebate façade over the new backend.

**Architecture:** Keep the current `subscription_referral` user journey recognizable, but swap its data source to the template framework. Admin-facing pages get a new referral settings workspace and a user-modal binding section, while the existing invite-rebate page remains the subscription-specific façade that reads generic template/binding data.

**Tech Stack:** React 18, Vite, Semi UI, existing `web/src/components/settings`, `web/src/components/table/users/modals`, Gin controller/user update endpoints, i18next strings.

---

### Task 1: Add Root Settings Surfaces for Templates and Engine Routes

**Files:**
- Create: `web/src/components/settings/ReferralSetting.jsx`
- Create: `web/src/pages/Setting/Referral/SettingsReferralTemplates.jsx`
- Create: `web/src/pages/Setting/Referral/SettingsReferralEngineRoutes.jsx`
- Create: `web/src/helpers/referralTemplate.js`
- Create: `web/tests/referral-settings-route.test.mjs`
- Modify: `web/src/pages/Setting/index.jsx`
- Test: `web/tests/referral-settings-route.test.mjs`

- [ ] **Step 1: Write the failing settings-surface test**

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';

test('settings page exposes a referral tab and lazy settings surface', () => {
  const source = fs.readFileSync('web/src/pages/Setting/index.jsx', 'utf8');
  assert.match(source, /ReferralSetting/);
  assert.match(source, /itemKey=['"]referral['"]/);
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `node --test web/tests/referral-settings-route.test.mjs`
Expected: FAIL because `ReferralSetting` and the `referral` settings tab do not exist yet.

- [ ] **Step 3: Implement the referral settings workspace**

```jsx
// web/src/components/settings/ReferralSetting.jsx
import React from 'react';
import { Card } from '@douyinfe/semi-ui';
import SettingsReferralTemplates from '../../pages/Setting/Referral/SettingsReferralTemplates';
import SettingsReferralEngineRoutes from '../../pages/Setting/Referral/SettingsReferralEngineRoutes';

const ReferralSetting = () => {
  return (
    <>
      <Card style={{ marginTop: '10px' }}>
        <SettingsReferralTemplates />
      </Card>
      <Card style={{ marginTop: '10px' }}>
        <SettingsReferralEngineRoutes />
      </Card>
    </>
  );
};

export default ReferralSetting;
```

```jsx
// web/src/pages/Setting/index.jsx
import ReferralSetting from '../../components/settings/ReferralSetting';

panes.push({
  tab: (
    <span style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
      <Settings size={18} />
      {t('返佣模板设置')}
    </span>
  ),
  content: <ReferralSetting />,
  itemKey: 'referral',
});
```

- [ ] **Step 4: Run the test to verify the settings tab now exists**

Run: `node --test web/tests/referral-settings-route.test.mjs`
Expected: PASS

- [ ] **Step 5: Commit the referral settings shell**

```bash
git add web/src/components/settings/ReferralSetting.jsx web/src/pages/Setting/Referral/SettingsReferralTemplates.jsx web/src/pages/Setting/Referral/SettingsReferralEngineRoutes.jsx web/src/helpers/referralTemplate.js web/src/pages/Setting/index.jsx web/tests/referral-settings-route.test.mjs
git commit -m "feat: add referral template settings surfaces"
```

### Task 2: Replace Per-User Override Editing with Template Binding and Inviter Management

**Files:**
- Create: `web/src/components/table/users/modals/ReferralTemplateBindingSection.jsx`
- Create: `web/tests/user-referral-binding.test.mjs`
- Create: `controller/user_inviter_test.go`
- Modify: `web/src/components/table/users/modals/EditUserModal.jsx`
- Modify: `controller/user.go`
- Modify: `model/user.go`
- Test: `controller/user_inviter_test.go`

- [ ] **Step 1: Write the failing inviter/binding tests**

```go
func TestUpdateUserRejectsReferralCycle(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	root := seedSubscriptionReferralControllerUser(t, "root-user", 0, dto.UserSetting{})
	root.Role = common.RoleRootUser
	if err := model.DB.Save(root).Error; err != nil {
		t.Fatalf("promote root user: %v", err)
	}
	parent := seedSubscriptionReferralControllerUser(t, "parent", root.Id, dto.UserSetting{})
	child := seedSubscriptionReferralControllerUser(t, "child", parent.Id, dto.UserSetting{})

	body := fmt.Sprintf(`{"id":%d,"username":"parent","group":"default","quota":0,"inviter_id":%d}`, parent.Id, child.Id)
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/", strings.NewReader(body), root.Id)

	UpdateUser(ctx)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 JSON envelope", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), `"success":true`) {
		t.Fatalf("expected cycle validation failure, body=%s", recorder.Body.String())
	}
}
```

```js
// ReferralTemplateBindingSection.jsx draft state should render the template selector label
test('edit user modal mounts referral template binding section', () => {
  const source = fs.readFileSync('web/src/components/table/users/modals/EditUserModal.jsx', 'utf8');
  assert.match(source, /ReferralTemplateBindingSection/);
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./controller -run 'TestUpdateUserRejectsReferralCycle'`

Run: `node --test web/tests/user-referral-binding.test.mjs`

Expected: FAIL because cycle validation and the new binding section are not present.

- [ ] **Step 3: Implement inviter validation and the per-user binding section**

```go
// model/user.go
func ValidateInviterAssignment(userID int, inviterID int) error {
	if inviterID <= 0 || userID <= 0 {
		return nil
	}
	if inviterID == userID {
		return errors.New("inviter cannot be self")
	}

	current := inviterID
	for current > 0 {
		if current == userID {
			return errors.New("inviter assignment would create a cycle")
		}
		user, err := GetUserById(current, false)
		if err != nil {
			return err
		}
		current = user.InviterId
	}
	return nil
}
```

```go
// controller/user.go
if updatedUser.InviterId != originUser.InviterId {
	if err := model.ValidateInviterAssignment(updatedUser.Id, updatedUser.InviterId); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
}
```

```jsx
// web/src/components/table/users/modals/EditUserModal.jsx
import ReferralTemplateBindingSection from './ReferralTemplateBindingSection';

<Card className='!rounded-2xl shadow-sm border-0'>
  <ReferralTemplateBindingSection userId={userId} />
</Card>
```

- [ ] **Step 4: Run the inviter/binding tests**

Run: `go test ./controller -run 'TestUpdateUserRejectsReferralCycle'`

Run: `node --test web/tests/user-referral-binding.test.mjs`

Expected: PASS

- [ ] **Step 5: Commit the user binding surface**

```bash
git add web/src/components/table/users/modals/ReferralTemplateBindingSection.jsx web/src/components/table/users/modals/EditUserModal.jsx web/tests/user-referral-binding.test.mjs controller/user.go controller/user_inviter_test.go model/user.go
git commit -m "feat: add referral binding and inviter management"
```

### Task 3: Keep the Subscription Invite-Rebate Page but Back It with Template Bindings

**Files:**
- Modify: `controller/subscription_referral.go`
- Modify: `web/src/components/invite-rebate/InviteRebatePage.jsx`
- Modify: `web/src/components/invite-rebate/InviteDefaultRuleSection.jsx`
- Modify: `web/src/components/invite-rebate/InviteeOverridePanel.jsx`
- Modify: `web/src/helpers/inviteRebate.js`
- Create: `web/tests/invite-rebate-template-facade.test.mjs`
- Modify: `controller/subscription_referral_test.go`
- Test: `web/tests/invite-rebate-template-facade.test.mjs`
- Test: `controller/subscription_referral_test.go`

- [ ] **Step 1: Write the failing façade tests**

```go
func TestGetSubscriptionReferralSelfUsesTemplateBindings(t *testing.T) {
	setupSubscriptionControllerTestDB(t)
	user := seedSubscriptionReferralControllerUser(t, "template-binding-user", 0, dto.UserSetting{})
	tpl := &model.ReferralTemplate{
		ReferralType:           model.ReferralTypeSubscription,
		Group:                  "vip",
		Name:                   "vip-direct",
		LevelType:              model.ReferralLevelTypeDirect,
		Enabled:                true,
		DirectCapBps:           1000,
		TeamCapBps:             2500,
		TeamDecayRatio:         0.5,
		TeamMaxDepth:           3,
		InviteeShareDefaultBps: 800,
	}
	if err := model.DB.Create(tpl).Error; err != nil {
		t.Fatalf("create template: %v", err)
	}
	if err := model.DB.Create(&model.ReferralTemplateBinding{
		UserId:       user.Id,
		ReferralType: model.ReferralTypeSubscription,
		Group:        "vip",
		TemplateId:   tpl.Id,
	}).Error; err != nil {
		t.Fatalf("create template binding: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/user/referral/subscription", nil, user.Id)
	GetSubscriptionReferralSelf(ctx)

	if !strings.Contains(recorder.Body.String(), `"group":"vip"`) {
		t.Fatalf("expected grouped template response, body=%s", recorder.Body.String())
	}
}
```

```js
test('invite rebate page still talks to the subscription façade endpoint', () => {
  const source = fs.readFileSync('web/src/components/invite-rebate/InviteRebatePage.jsx', 'utf8');
  assert.match(source, /\/api\/user\/referral\/subscription/);
});
```

- [ ] **Step 2: Run the façade tests to verify they fail**

Run: `go test ./controller -run 'TestGetSubscriptionReferralSelfUsesTemplateBindings'`

Run: `node --test web/tests/invite-rebate-template-facade.test.mjs`

Expected: FAIL because the controller still reads legacy overrides only and the frontend helper layer has no template-binding normalization.

- [ ] **Step 3: Implement the subscription façade over template bindings**

```go
func GetSubscriptionReferralSelf(c *gin.Context) {
	userID := c.GetInt("id")
	bindings, err := model.ListReferralTemplateBindingsByUser(userID, model.ReferralTypeSubscription)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	groupViews := make([]gin.H, 0, len(bindings))
	for _, binding := range bindings {
		if !binding.Template.Enabled {
			continue
		}
		effectiveInviteeRateBps := model.ResolveBindingInviteeShareDefault(binding, binding.Template)
		groupViews = append(groupViews, gin.H{
			"group":            binding.Group,
			"template_id":      binding.TemplateId,
			"template_name":    binding.Template.Name,
			"level_type":       binding.Template.LevelType,
			"total_rate_bps":   binding.Template.DirectCapBps,
			"invitee_rate_bps": effectiveInviteeRateBps,
		})
	}

	common.ApiSuccess(c, gin.H{
		"enabled": len(groupViews) > 0,
		"groups":  groupViews,
	})
}
```

```jsx
// web/src/helpers/inviteRebate.js
export const buildInviteDefaultRuleRows = (groups = []) =>
  groups.map((item) => ({
    group: item.group,
    templateId: item.template_id ?? null,
    templateName: item.template_name ?? '',
    levelType: item.level_type ?? '',
    totalRateBps: Number(item.total_rate_bps ?? 0),
    inviteeRateBps: Number(item.invitee_rate_bps ?? 0),
  }));
```

- [ ] **Step 4: Run the backend/frontend façade tests**

Run: `go test ./controller -run 'TestGetSubscriptionReferralSelfUsesTemplateBindings|TestGetSubscriptionReferralInvitee'`

Run: `node --test web/tests/invite-rebate-template-facade.test.mjs`

Expected: PASS

- [ ] **Step 5: Commit the subscription façade migration**

```bash
git add controller/subscription_referral.go controller/subscription_referral_test.go web/src/components/invite-rebate/InviteRebatePage.jsx web/src/components/invite-rebate/InviteDefaultRuleSection.jsx web/src/components/invite-rebate/InviteeOverridePanel.jsx web/src/helpers/inviteRebate.js web/tests/invite-rebate-template-facade.test.mjs
git commit -m "feat: migrate invite rebate page to referral template facade"
```
