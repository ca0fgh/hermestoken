import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';

const repoRoot = new URL('../../', import.meta.url);
const fromRepoRoot = (relativePath) => new URL(relativePath, repoRoot);

test('invite rebate page still talks to the subscription facade endpoints', () => {
  const source = fs.readFileSync(
    fromRepoRoot('web/classic/src/components/invite-rebate/InviteRebatePage.jsx'),
    'utf8',
  );
  assert.match(source, /\/api\/user\/referral\/subscription/);
});

test('legacy self override endpoints are removed from router and controller surface', () => {
  const routerSource = fs.readFileSync(fromRepoRoot('router/api-router.go'), 'utf8');
  const controllerSource = fs.readFileSync(
    fromRepoRoot('controller/subscription_referral.go'),
    'utf8',
  );

  assert.doesNotMatch(
    routerSource,
    /selfRoute\.PUT\("\/referral\/subscription"/,
  );
  assert.doesNotMatch(
    routerSource,
    /selfRoute\.DELETE\("\/referral\/subscription"/,
  );
  assert.doesNotMatch(controllerSource, /func UpdateSubscriptionReferralSelf/);
  assert.doesNotMatch(controllerSource, /func DeleteSubscriptionReferralSelf/);
});
