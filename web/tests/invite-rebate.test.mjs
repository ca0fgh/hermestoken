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
*/

import assert from 'node:assert/strict';
import test from 'node:test';
import {
  buildInviteDefaultRuleRows,
  buildReceivedContributionDetailCards,
  buildReceivedInviteeRuleRows,
  buildInviteeContributionLedgerRows,
  buildInviteeContributionSummary,
  buildInviteeOverrideDraftPercentMap,
  buildInviteeContributionDetailCards,
  buildInviteeOverrideRows,
  normalizeInviteeContributionPage,
} from '../classic/src/helpers/inviteRebate.js';

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

test('buildInviteDefaultRuleRows maps grouped template defaults into read-only helper rows', () => {
  assert.deepEqual(
    buildInviteDefaultRuleRows([
      {
        group: 'default',
        template_name: 'default-direct-template',
        level_type: 'direct',
        total_rate_bps: 4500,
        invitee_rate_bps: 500,
      },
      {
        group: 'vip',
        template_name: 'vip-team-template',
        level_type: 'team',
        totalRateBps: 3000,
        inviteeRateBps: 0,
      },
    ]),
    [
      {
        id: 'subscription:default',
        group: 'default',
        templateName: 'default-direct-template',
        levelType: 'direct',
        effectiveTotalRateBps: 4500,
        effectiveInviteeRateBps: 500,
        effectiveInviterRateBps: 4000,
      },
      {
        id: 'subscription:vip',
        group: 'vip',
        templateName: 'vip-team-template',
        levelType: 'team',
        effectiveTotalRateBps: 3000,
        effectiveInviteeRateBps: 0,
        effectiveInviterRateBps: 3000,
      },
    ],
  );
});

test('buildReceivedInviteeRuleRows maps inviter-assigned invitee rebate scopes into read-only rows', () => {
  assert.deepEqual(
    buildReceivedInviteeRuleRows({
      received_inviter: {
        id: 9,
        username: 'parent-user',
      },
      received_groups: [
        {
          group: 'vip',
          template_name: 'vip-direct-template',
          level_type: 'direct',
          total_rate_bps: 1200,
          effective_invitee_rate_bps: 500,
          effective_inviter_rate_bps: 700,
          has_override: true,
        },
      ],
    }),
    [
      {
        id: 'received:vip',
        inviterId: 9,
        inviterUsername: 'parent-user',
        group: 'vip',
        templateName: 'vip-direct-template',
        levelType: 'direct',
        effectiveTotalRateBps: 1200,
        effectiveInviteeRateBps: 500,
        effectiveInviterRateBps: 700,
        hasOverride: true,
      },
    ],
  );
});

test('buildReceivedContributionDetailCards groups invitee rebate ledger by order for the invitee view', () => {
  assert.deepEqual(
    buildReceivedContributionDetailCards({
      received_contribution_details: [
        {
          batch_id: 8,
          trade_no: 'trade-team-1',
          group: 'vip',
          reward_component: 'invitee_reward',
          source_reward_component: 'team_direct_reward',
          role_type: 'team',
          effective_reward_quota: 300,
          status: 'credited',
          settled_at: 1711111111,
        },
        {
          batch_id: 7,
          trade_no: 'trade-direct-1',
          group: 'default',
          reward_component: 'invitee_reward',
          source_reward_component: 'direct_reward',
          role_type: 'direct',
          effective_reward_quota: 150,
          status: 'credited',
          settled_at: 1711111000,
        },
      ],
    }),
    [
      {
        id: '8:trade-team-1',
        batchId: 8,
        tradeNo: 'trade-team-1',
        group: 'vip',
        status: 'credited',
        settledAt: 1711111111,
        ownRewardQuota: 0,
        inviteeRewardQuota: 300,
        items: [
          {
            id: '8:trade-team-1:0',
            rewardComponent: 'invitee_reward',
            sourceRewardComponent: 'team_direct_reward',
            sourceComponentLabel: '团队直返',
            roleType: 'team',
            effectiveRewardQuota: 300,
            status: 'credited',
            componentLabel: '收到返佣',
            roleLabel: '团队',
            isInviteeShare: true,
          },
        ],
      },
      {
        id: '7:trade-direct-1',
        batchId: 7,
        tradeNo: 'trade-direct-1',
        group: 'default',
        status: 'credited',
        settledAt: 1711111000,
        ownRewardQuota: 0,
        inviteeRewardQuota: 150,
        items: [
          {
            id: '7:trade-direct-1:0',
            rewardComponent: 'invitee_reward',
            sourceRewardComponent: 'direct_reward',
            sourceComponentLabel: '直推返佣',
            roleType: 'direct',
            effectiveRewardQuota: 150,
            status: 'credited',
            componentLabel: '收到返佣',
            roleLabel: '直推',
            isInviteeShare: true,
          },
        ],
      },
    ],
  );
});

test('buildInviteeOverrideRows maps template scope payload into override-aware rows', () => {
  assert.deepEqual(
    buildInviteeOverrideRows({
      scopes: [
        {
          group: 'default',
          template_name: 'default-direct-template',
          level_type: 'direct',
          total_rate_bps: 4500,
          default_invitee_rate_bps: 500,
          effective_invitee_rate_bps: 500,
          has_override: false,
        },
        {
          group: 'vip',
          template_name: 'vip-team-template',
          level_type: 'team',
          total_rate_bps: 3000,
          default_invitee_rate_bps: 900,
          override_invitee_rate_bps: 1200,
          effective_invitee_rate_bps: 1200,
          has_override: true,
        },
      ],
    }),
    [
      {
        id: 'subscription:default',
        group: 'default',
        templateName: 'default-direct-template',
        levelType: 'direct',
        inputPercent: 5,
        effectiveTotalRateBps: 4500,
        defaultInviteeRateBps: 500,
        effectiveInviteeRateBps: 500,
        hasOverride: false,
      },
      {
        id: 'subscription:vip',
        group: 'vip',
        templateName: 'vip-team-template',
        levelType: 'team',
        inputPercent: 12,
        effectiveTotalRateBps: 3000,
        defaultInviteeRateBps: 900,
        effectiveInviteeRateBps: 1200,
        hasOverride: true,
      },
    ],
  );
});

test('buildInviteeOverrideDraftPercentMap syncs per-group drafts from the latest rows', () => {
  assert.deepEqual(
    buildInviteeOverrideDraftPercentMap([
      {
        group: 'default',
        inputPercent: 30,
      },
      {
        group: 'vip',
        inputPercent: 12,
      },
    ]),
    {
      default: 30,
      vip: 12,
    },
  );

  assert.deepEqual(buildInviteeOverrideDraftPercentMap(), {});
});

test('buildInviteeContributionSummary aggregates order, direct, team, and invitee-share totals', () => {
  assert.deepEqual(
    buildInviteeContributionSummary([
      {
        id: '1:trade-direct',
        ownRewardQuota: 1200,
        inviteeRewardQuota: 100,
        items: [
          { roleType: 'direct', isInviteeShare: false },
          { roleType: 'direct', isInviteeShare: true },
        ],
      },
      {
        id: '2:trade-team',
        ownRewardQuota: 600,
        inviteeRewardQuota: 50,
        items: [
          { roleType: 'team', isInviteeShare: false },
          { roleType: 'team', isInviteeShare: true },
          { roleType: 'team', isInviteeShare: false },
        ],
      },
    ]),
    {
      orderCount: 2,
      ownRewardQuota: 1800,
      inviteeRewardQuota: 150,
      directDetailCount: 1,
      teamDetailCount: 2,
      inviteeShareCount: 2,
    },
  );
});

test('buildInviteeContributionLedgerRows flattens grouped cards into readable ledger rows', () => {
  assert.deepEqual(
    buildInviteeContributionLedgerRows([
      {
        id: '8:trade-team-1',
        tradeNo: 'trade-team-1',
        group: 'vip',
        status: 'credited',
        settledAt: 1711111111,
        items: [
          {
            id: '8:trade-team-1:0',
            roleType: 'team',
            roleLabel: '团队',
            componentLabel: '团队直返',
            effectiveRewardQuota: 700,
            status: 'credited',
          },
          {
            id: '8:trade-team-1:1',
            roleType: 'team',
            roleLabel: '团队',
            componentLabel: '返给对方',
            effectiveRewardQuota: 300,
            status: 'credited',
          },
        ],
      },
    ]),
    [
      {
        id: '8:trade-team-1:8:trade-team-1:0',
        tradeNo: 'trade-team-1',
        group: 'vip',
        settledAt: 1711111111,
        status: 'credited',
        roleType: 'team',
        roleLabel: '团队',
        componentLabel: '团队直返',
        sourceComponentLabel: '',
        effectiveRewardQuota: 700,
        isInviteeShare: false,
      },
      {
        id: '8:trade-team-1:8:trade-team-1:1',
        tradeNo: 'trade-team-1',
        group: 'vip',
        settledAt: 1711111111,
        status: 'credited',
        roleType: 'team',
        roleLabel: '团队',
        componentLabel: '返给对方',
        sourceComponentLabel: '',
        effectiveRewardQuota: 300,
        isInviteeShare: true,
      },
    ],
  );
});

test('buildInviteeContributionDetailCards groups records by order and maps role/component metadata', () => {
  assert.deepEqual(
    buildInviteeContributionDetailCards({
      contribution_details: [
        {
          batch_id: 8,
          trade_no: 'trade-team-1',
          group: 'vip',
          reward_component: 'team_direct_reward',
          source_reward_component: null,
          role_type: 'team',
          effective_reward_quota: 700,
          status: 'credited',
          settled_at: 1711111111,
        },
        {
          batch_id: 8,
          trade_no: 'trade-team-1',
          group: 'vip',
          reward_component: 'invitee_reward',
          source_reward_component: 'team_direct_reward',
          role_type: 'team',
          effective_reward_quota: 300,
          status: 'credited',
          settled_at: 1711111111,
        },
        {
          batch_id: 7,
          trade_no: 'trade-direct-1',
          group: 'default',
          reward_component: 'direct_reward',
          source_reward_component: null,
          role_type: 'direct',
          effective_reward_quota: 500,
          status: 'credited',
          settled_at: 1711111000,
        },
      ],
    }),
    [
      {
        id: '8:trade-team-1',
        batchId: 8,
        tradeNo: 'trade-team-1',
        group: 'vip',
        status: 'credited',
        settledAt: 1711111111,
        ownRewardQuota: 700,
        inviteeRewardQuota: 300,
        items: [
          {
            id: '8:trade-team-1:0',
            rewardComponent: 'team_direct_reward',
            sourceRewardComponent: '',
            sourceComponentLabel: '',
            roleType: 'team',
            effectiveRewardQuota: 700,
            status: 'credited',
            componentLabel: '团队直返',
            roleLabel: '团队',
            isInviteeShare: false,
          },
          {
            id: '8:trade-team-1:1',
            rewardComponent: 'invitee_reward',
            sourceRewardComponent: 'team_direct_reward',
            sourceComponentLabel: '团队直返',
            roleType: 'team',
            effectiveRewardQuota: 300,
            status: 'credited',
            componentLabel: '返给对方',
            roleLabel: '团队',
            isInviteeShare: true,
          },
        ],
      },
      {
        id: '7:trade-direct-1',
        batchId: 7,
        tradeNo: 'trade-direct-1',
        group: 'default',
        status: 'credited',
        settledAt: 1711111000,
        ownRewardQuota: 500,
        inviteeRewardQuota: 0,
        items: [
          {
            id: '7:trade-direct-1:0',
            rewardComponent: 'direct_reward',
            sourceRewardComponent: '',
            sourceComponentLabel: '',
            roleType: 'direct',
            effectiveRewardQuota: 500,
            status: 'credited',
            componentLabel: '直推返佣',
            roleLabel: '直推',
            isInviteeShare: false,
          },
        ],
      },
    ],
  );
});
