import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';

test('invite rebate page still talks to the subscription facade endpoints', () => {
  const source = fs.readFileSync(
    'web/src/components/invite-rebate/InviteRebatePage.jsx',
    'utf8',
  );
  assert.match(source, /\/api\/user\/referral\/subscription/);
});
