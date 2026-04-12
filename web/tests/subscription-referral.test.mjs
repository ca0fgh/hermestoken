/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import test from 'node:test';
import assert from 'node:assert/strict';
import {
  buildAdminReferralRows,
  buildAdminReferralFormValues,
  buildGroupedReferralSummaries,
  clampInviteeRateBps,
  buildReferralRateSummary,
  formatRateBpsPercent,
  normalizeAdminReferralPayload,
  parseAdminReferralSettings,
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

test('normalizeAdminReferralPayload keeps BPS integers within 0-10000', () => {
  assert.deepEqual(
    normalizeAdminReferralPayload({ enabled: true, totalRateBps: 12000 }),
    {
      enabled: true,
      totalRateBps: 10000,
    },
  );
});

test('parseAdminReferralSettings normalizes grouped admin API payload for local form state', () => {
  assert.deepEqual(
    parseAdminReferralSettings({
      enabled: 1,
      group_rates: {
        default: '4500',
        vip: 3000,
      },
    }),
    {
      enabled: true,
      groupRates: {
        default: 4500,
        vip: 3000,
      },
    },
  );
});

test('buildAdminReferralRows maps group names and rates into table rows', () => {
  assert.deepEqual(
    buildAdminReferralRows(['default', 'vip'], {
      default: 4500,
      vip: 3000,
    }),
    [
      {
        group: 'default',
        enabled: true,
        totalRateBps: 4500,
        totalRatePercent: 45,
      },
      {
        group: 'vip',
        enabled: true,
        totalRateBps: 3000,
        totalRatePercent: 30,
      },
    ],
  );
});

test('buildGroupedReferralSummaries derives inviter rate per group', () => {
  assert.deepEqual(
    buildGroupedReferralSummaries([
      {
        group: 'default',
        total_rate_bps: 4500,
        invitee_rate_bps: 500,
      },
      {
        group: 'vip',
        total_rate_bps: 3000,
        invitee_rate_bps: 0,
      },
    ]),
    [
      {
        group: 'default',
        totalRateBps: 4500,
        inviteeRateBps: 500,
        inviterRateBps: 4000,
      },
      {
        group: 'vip',
        totalRateBps: 3000,
        inviteeRateBps: 0,
        inviterRateBps: 3000,
      },
    ],
  );
});

test('buildAdminReferralFormValues maps settings to Semi Form field names', () => {
  assert.deepEqual(
    buildAdminReferralFormValues({
      enabled: true,
      totalRatePercent: 45,
    }),
    {
      SubscriptionReferralEnabled: true,
      SubscriptionReferralGlobalRateBps: 45,
    },
  );
});
