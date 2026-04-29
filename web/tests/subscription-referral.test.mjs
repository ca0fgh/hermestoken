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

import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import {
  buildGroupedReferralSummaries,
  buildInvitationDraftPercentInputs,
  clampInviteeRateBps,
  buildReferralRateSummary,
  formatRateBpsPercent,
} from '../classic/src/helpers/subscriptionReferral.js';

test('clampInviteeRateBps caps invitee rate to total rate', () => {
  assert.equal(clampInviteeRateBps(2600, 2000), 2000);
  assert.equal(clampInviteeRateBps(800, 2000), 800);
});

test('buildReferralRateSummary derives inviter rate from total minus invitee', () => {
  assert.deepEqual(buildReferralRateSummary(2000, 500), {
    group: '',
    totalRateBps: 2000,
    inviteeRateBps: 500,
    inviterRateBps: 1500,
  });
});

test('formatRateBpsPercent formats basis points as percent strings', () => {
  assert.equal(formatRateBpsPercent(2000), '20%');
  assert.equal(formatRateBpsPercent(375), '3.75%');
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

test('buildGroupedReferralSummaries returns no cards when grouped payload is empty', () => {
  assert.deepEqual(buildGroupedReferralSummaries([]), []);
});

test('buildGroupedReferralSummaries preserves group names for invitation cards', () => {
  assert.deepEqual(
    buildGroupedReferralSummaries([
      {
        group: 'default',
        total_rate_bps: 4500,
        invitee_rate_bps: 500,
      },
    ]),
    [
      {
        group: 'default',
        totalRateBps: 4500,
        inviteeRateBps: 500,
        inviterRateBps: 4000,
      },
    ],
  );
});

test('buildInvitationDraftPercentInputs preserves existing drafts when persisted groups refresh', () => {
  assert.deepEqual(
    buildInvitationDraftPercentInputs(
      {
        default: 3.5,
        vip: 12.34,
      },
      [
        {
          group: 'default',
          totalRateBps: 4500,
          inviteeRateBps: 400,
        },
        {
          group: 'vip',
          totalRateBps: 3000,
          inviteeRateBps: 500,
        },
      ],
    ),
    {
      default: 3.5,
      vip: 12.34,
    },
  );
});

test('buildInvitationDraftPercentInputs preserves the saving group draft when persisted data has not changed', () => {
  assert.deepEqual(
    buildInvitationDraftPercentInputs(
      {
        default: 7.25,
        vip: 12.34,
      },
      [
        {
          group: 'default',
          totalRateBps: 4500,
          inviteeRateBps: 500,
        },
        {
          group: 'vip',
          totalRateBps: 3000,
          inviteeRateBps: 500,
        },
      ],
      'default',
    ),
    {
      default: 7.25,
      vip: 12.34,
    },
  );
});

test('InvitationCard does not reference removed referralSavingGroup state', () => {
  const invitationCardSource = readFileSync(
    new URL('../classic/src/components/topup/InvitationCard.jsx', import.meta.url),
    'utf8',
  );

  assert.equal(invitationCardSource.includes('referralSavingGroup'), false);
});

test('InvitationCard no longer imports grouped subscription referral helpers', () => {
  const invitationCardSource = readFileSync(
    new URL('../classic/src/components/topup/InvitationCard.jsx', import.meta.url),
    'utf8',
  );

  assert.doesNotMatch(
    invitationCardSource,
    /import\s*{[\s\S]*\brateBpsToPercentNumber\b[\s\S]*}\s*from\s*['"]\.\.\/\.\.\/helpers\/subscriptionReferral['"]/,
  );
});

test('InvitationCard no longer renders grouped subscription referral controls in wallet view', () => {
  const invitationCardSource = readFileSync(
    new URL('../classic/src/components/topup/InvitationCard.jsx', import.meta.url),
    'utf8',
  );

  assert.doesNotMatch(invitationCardSource, /visibleReferralGroups/);
  assert.doesNotMatch(invitationCardSource, /订阅返佣分配/);
  assert.doesNotMatch(invitationCardSource, /保存返佣设置/);
});

test('wallet invitation card keeps invite stats and link but removes subscription referral management', () => {
  const invitationCardSource = readFileSync(
    new URL('../classic/src/components/topup/InvitationCard.jsx', import.meta.url),
    'utf8',
  );

  assert.match(invitationCardSource, /t\('邀请奖励'\)/);
  assert.match(invitationCardSource, /t\('收益统计'\)/);
  assert.match(invitationCardSource, /t\('邀请链接'\)/);
  assert.doesNotMatch(invitationCardSource, /t\('订阅返佣分配'\)/);
  assert.doesNotMatch(invitationCardSource, /referralGroups/);
  assert.doesNotMatch(invitationCardSource, /onSaveReferralConfig/);
});

test('TopUp no longer fetches or wires wallet subscription referral management state', () => {
  const topUpSource = readFileSync(
    new URL('../classic/src/components/topup/index.jsx', import.meta.url),
    'utf8',
  );

  assert.doesNotMatch(
    topUpSource,
    /API\.get\('\/api\/user\/referral\/subscription'\)/,
  );
  assert.doesNotMatch(topUpSource, /const \[referralGroups, setReferralGroups\]/);
  assert.doesNotMatch(
    topUpSource,
    /const updateSubscriptionReferralSelf = async \(group, inviteeRateBps\) =>/,
  );
  assert.doesNotMatch(topUpSource, /referralGroups=\{referralGroups\}/);
  assert.doesNotMatch(
    topUpSource,
    /onSaveReferralConfig=\{updateSubscriptionReferralSelf\}/,
  );
});

test('OperationSetting no longer mounts the global subscription referral settings card', () => {
  const operationSettingSource = readFileSync(
    new URL('../classic/src/components/settings/OperationSetting.jsx', import.meta.url),
    'utf8',
  );

  assert.equal(
    operationSettingSource.includes('SettingsSubscriptionReferral'),
    false,
  );
  assert.equal(
    operationSettingSource.includes('SubscriptionReferralEnabled'),
    false,
  );
});

test('topup wallet view no longer keeps grouped referral summary refresh logic', () => {
  const topupSource = readFileSync(
    new URL('../classic/src/components/topup/index.jsx', import.meta.url),
    'utf8',
  );

  assert.doesNotMatch(topupSource, /getSubscriptionReferralSelf/);
  assert.equal(topupSource.includes('data.total_rate_bps'), false);
  assert.equal(topupSource.includes('data.invitee_rate_bps'), false);
});

test('invite rebate UI no longer describes inviter-authorized splits as platform default rules', () => {
  const defaultRuleSource = readFileSync(
    new URL(
      '../classic/src/components/invite-rebate/InviteDefaultRuleSection.jsx',
      import.meta.url,
    ),
    'utf8',
  );
  const inviteeOverrideSource = readFileSync(
    new URL(
      '../classic/src/components/invite-rebate/InviteeOverridePanel.jsx',
      import.meta.url,
    ),
    'utf8',
  );

  assert.equal(defaultRuleSource.includes("默认返佣规则"), false);
  assert.equal(defaultRuleSource.includes("当前默认总返佣率"), false);
  assert.equal(inviteeOverrideSource.includes("未设置独立返佣时，使用默认规则"), false);
  assert.equal(inviteeOverrideSource.includes("当前默认返佣率"), false);
  assert.equal(inviteeOverrideSource.includes("当前默认总返佣率"), false);
});

test('option map no longer exposes legacy global subscription referral settings', () => {
  const optionSource = readFileSync(
    new URL('../../model/option.go', import.meta.url),
    'utf8',
  );

  assert.equal(optionSource.includes('OptionMap["SubscriptionReferralEnabled"]'), false);
  assert.equal(
    optionSource.includes('OptionMap["SubscriptionReferralGlobalRateBps"]'),
    false,
  );
  assert.equal(
    optionSource.includes('OptionMap["SubscriptionReferralGroupRates"]'),
    false,
  );
});
