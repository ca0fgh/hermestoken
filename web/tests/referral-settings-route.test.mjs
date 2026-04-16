import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';

test('settings page exposes a referral tab and settings surface', () => {
  const source = fs.readFileSync('web/src/pages/Setting/index.jsx', 'utf8');
  assert.match(source, /ReferralSetting/);
  assert.match(source, /itemKey:\s*['"]referral['"]/);
});
