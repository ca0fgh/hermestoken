import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';

test('edit user modal mounts referral template binding section', () => {
  const source = fs.readFileSync(
    'web/src/components/table/users/modals/EditUserModal.jsx',
    'utf8',
  );
  assert.match(source, /ReferralTemplateBindingSection/);
  assert.doesNotMatch(source, /SubscriptionReferralOverrideSection/);
});

test('referral template binding section uses explicit state updates instead of mutating response objects', () => {
  const source = fs.readFileSync(
    'web/src/components/table/users/modals/ReferralTemplateBindingSection.jsx',
    'utf8',
  );
  assert.match(source, /updateRow/);
  assert.match(source, /新增绑定/);
  assert.doesNotMatch(source, /import \{[^}]*\bInput\b/);
  assert.doesNotMatch(source, /view\.binding\.template_id\s*=/);
  assert.doesNotMatch(source, /view\.binding\.invitee_share_override_bps\s*=/);
  assert.doesNotMatch(source, /默认分账比例/);
  assert.doesNotMatch(source, /{t\('分组'\)}/);
});
