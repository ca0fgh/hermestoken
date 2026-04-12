import test from 'node:test';
import assert from 'node:assert/strict';
import {
  clampInviteeRateBps,
  buildReferralRateSummary,
  formatRateBpsPercent,
} from '../src/helpers/subscriptionReferral.js';

test('clampInviteeRateBps caps invitee rate to total rate', () => {
  assert.equal(clampInviteeRateBps(2600, 2000), 2000);
  assert.equal(clampInviteeRateBps(800, 2000), 800);
});

test('buildReferralRateSummary derives inviter rate from total minus invitee', () => {
  assert.deepEqual(buildReferralRateSummary(2000, 500), {
    totalRateBps: 2000,
    inviteeRateBps: 500,
    inviterRateBps: 1500,
  });
});

test('formatRateBpsPercent formats basis points as percent strings', () => {
  assert.equal(formatRateBpsPercent(2000), '20%');
  assert.equal(formatRateBpsPercent(375), '3.75%');
});
