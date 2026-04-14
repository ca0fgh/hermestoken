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

import assert from 'node:assert/strict';
import test from 'node:test';
import {
  buildInviteDefaultRuleRows,
  buildInviteeOverrideRows,
  normalizeInviteeContributionPage,
} from '../src/helpers/inviteRebate.js';

test('normalizeInviteeContributionPage coerces invitee page fields and preserves items arrays', () => {
  const items = [{ id: 12, username: 'alice' }];

  assert.deepEqual(
    normalizeInviteeContributionPage({
      page: '2',
      page_size: '20',
      total: '8',
      invitee_count: '5',
      total_contribution_quota: '150000',
      items,
    }),
    {
      page: 2,
      page_size: 20,
      total: 8,
      invitee_count: 5,
      total_contribution_quota: 150000,
      items,
    },
  );

  assert.deepEqual(normalizeInviteeContributionPage(), {
    page: 1,
    page_size: 0,
    total: 0,
    invitee_count: 0,
    total_contribution_quota: 0,
    items: [],
  });
});

test('buildInviteDefaultRuleRows maps grouped inviter defaults into editable helper rows', () => {
  assert.deepEqual(
    buildInviteDefaultRuleRows([
      {
        group: 'default',
        total_rate_bps: 4500,
        invitee_rate_bps: 500,
      },
      {
        group: 'vip',
        totalRateBps: 3000,
        inviteeRateBps: 0,
      },
    ]),
    [
      {
        id: 'subscription:default',
        type: 'subscription',
        group: 'default',
        inputPercent: 5,
        effectiveTotalRateBps: 4500,
        hasOverride: true,
        isDraft: false,
      },
      {
        id: 'subscription:vip',
        type: 'subscription',
        group: 'vip',
        inputPercent: 0,
        effectiveTotalRateBps: 3000,
        hasOverride: true,
        isDraft: false,
      },
    ],
  );
});

test('buildInviteeOverrideRows maps invitee detail payload into override-aware rows', () => {
  assert.deepEqual(
    buildInviteeOverrideRows({
      available_groups: ['default', 'vip'],
      default_invitee_rate_bps_by_group: {
        default: 500,
        vip: 900,
      },
      effective_total_rate_bps_by_group: {
        default: 4500,
        vip: 3000,
      },
      overrides: [
        {
          group: 'vip',
          invitee_rate_bps: 1200,
        },
      ],
    }),
    [
      {
        id: 'subscription:default',
        type: 'subscription',
        group: 'default',
        inputPercent: 5,
        effectiveTotalRateBps: 4500,
        defaultInviteeRateBps: 500,
        hasOverride: false,
        isDraft: false,
      },
      {
        id: 'subscription:vip',
        type: 'subscription',
        group: 'vip',
        inputPercent: 12,
        effectiveTotalRateBps: 3000,
        defaultInviteeRateBps: 900,
        hasOverride: true,
        isDraft: false,
      },
    ],
  );
});
