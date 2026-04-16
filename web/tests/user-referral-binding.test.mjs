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
