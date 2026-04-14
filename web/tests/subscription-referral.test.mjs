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
import { readFileSync } from 'node:fs';
import {
  buildAdminOverrideRows,
  buildAdminOverrideGroupOptions,
  buildAdminReferralRows,
  buildAdminReferralFormValues,
  buildGroupedReferralSummaries,
  buildInvitationDraftPercentInputs,
  clampInviteeRateBps,
  buildReferralRateSummary,
  formatRateBpsPercent,
  normalizeAdminReferralPayload,
  parseAdminReferralSettings,
  createAdminOverrideDraftRow,
} from '../src/helpers/subscriptionReferral.js';

test('buildAdminOverrideRows maps grouped overrides into extensible rows', () => {
  const rows = buildAdminOverrideRows([
    {
      group: 'alpha',
      effective_total_rate_bps: 4000,
      has_override: true,
      override_rate_bps: 3000,
    },
    {
      group: 'beta',
      effectiveTotalRateBps: 2500,
      hasOverride: false,
      overrideRateBps: null,
    },
  ]);

  assert.deepEqual(rows, [
    {
      id: 'subscription:alpha',
      type: 'subscription',
      group: 'alpha',
      effectiveTotalRateBps: 4000,
      hasOverride: true,
      overrideRateBps: 3000,
      overrideRatePercent: 30,
      inputPercent: 30,
      isDraft: false,
    },
    {
      id: 'subscription:beta',
      type: 'subscription',
      group: 'beta',
      effectiveTotalRateBps: 2500,
      hasOverride: false,
      overrideRateBps: null,
      overrideRatePercent: 25,
      inputPercent: 25,
      isDraft: false,
    },
  ]);
});

test('buildAdminOverrideGroupOptions retains legacy group even when missing from catalog', () => {
  const rows = [
    { id: 'subscription:alpha', type: 'subscription', group: 'alpha' },
    { id: 'subscription:legacy', type: 'subscription', group: 'legacy' },
  ];
  const options = buildAdminOverrideGroupOptions(
    ['alpha', 'beta'],
    rows,
    rows[1],
  );

  assert.deepEqual(options, [
    { label: 'alpha', value: 'alpha', disabled: true },
    { label: 'beta', value: 'beta', disabled: false },
    { label: 'legacy', value: 'legacy', disabled: false },
  ]);
});

test('createAdminOverrideDraftRow creates a new draft entry with defaults', () => {
  const draft = createAdminOverrideDraftRow();

  assert.match(draft.id, /^draft:/);
  assert.equal(draft.type, 'subscription');
  assert.equal(draft.group, '');
  assert.equal(draft.overrideRatePercent, 0);
  assert.equal(draft.isDraft, true);
  assert.equal(draft.hasOverride, false);
  assert.equal(draft.inputPercent, 0);
});

test('buildAdminOverrideGroupOptions disables already used groups for the same type', () => {
  const rows = [
    { id: 'subscription:alpha', type: 'subscription', group: 'alpha' },
    { id: 'subscription:beta', type: 'subscription', group: 'beta' },
  ];
  const options = buildAdminOverrideGroupOptions(
    ['alpha', 'beta', 'gamma'],
    rows,
    rows[0],
  );

  assert.deepEqual(options, [
    { label: 'alpha', value: 'alpha', disabled: false },
    { label: 'beta', value: 'beta', disabled: true },
    { label: 'gamma', value: 'gamma', disabled: false },
  ]);
});

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
      groups: ['default', 'vip'],
      group_rates: {
        default: '4500',
        vip: 3000,
      },
    }),
    {
      enabled: true,
      groups: ['default', 'vip'],
      groupRates: {
        default: 4500,
        vip: 3000,
      },
    },
  );
});

test('parseAdminReferralSettings does not synthesize default from legacy total_rate_bps', () => {
  assert.deepEqual(
    parseAdminReferralSettings({
      enabled: true,
      total_rate_bps: 4500,
    }),
    {
      enabled: true,
      groups: [],
      groupRates: {},
    },
  );
});

test('buildAdminReferralRows does not invent default rows when group list is empty', () => {
  assert.deepEqual(buildAdminReferralRows([], {}), []);
});

test('buildAdminOverrideRows returns no rows when grouped override payload is empty', () => {
  assert.deepEqual(buildAdminOverrideRows([]), []);
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

test('buildAdminReferralRows sorts rows alphabetically and includes rates missing from explicit group list', () => {
  assert.deepEqual(
    buildAdminReferralRows(['vip'], {
      zeta: 1200,
      default: 4500,
      vip: 3000,
    }),
    [
      {
        group: 'vip',
        enabled: true,
        totalRateBps: 3000,
        totalRatePercent: 30,
      },
    ],
  );
});

test('parseAdminReferralSettings does not synthesize groups from group_rates when payload.groups is empty', () => {
  assert.deepEqual(
    parseAdminReferralSettings({
      enabled: true,
      group_rates: {
        vip: 3000,
        default: 4500,
      },
    }),
    {
      enabled: true,
      groups: [],
      groupRates: {
        default: 4500,
        vip: 3000,
      },
    },
  );
});

test('buildAdminReferralRows ignores rates for groups not present in the provided group list', () => {
  assert.deepEqual(
    buildAdminReferralRows([], {
      vip: 3000,
      default: 4500,
    }),
    [],
  );
});

test('plan-backed settings groups render rows even when missing from group_rates', () => {
  const parsedSettings = parseAdminReferralSettings({
    enabled: true,
    groups: ['default', 'retired'],
    group_rates: {
      default: 4500,
    },
  });

  assert.deepEqual(
    buildAdminReferralRows(parsedSettings.groups, parsedSettings.groupRates),
    [
      {
        group: 'default',
        enabled: true,
        totalRateBps: 4500,
        totalRatePercent: 45,
      },
      {
        group: 'retired',
        enabled: false,
        totalRateBps: 0,
        totalRatePercent: 0,
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

test('buildGroupedReferralSummaries returns no cards when grouped payload is empty', () => {
  assert.deepEqual(buildGroupedReferralSummaries([]), []);
});

test('buildAdminOverrideRows normalizes grouped override API payload', () => {
  assert.deepEqual(
    buildAdminOverrideRows([
      {
        group: 'default',
        effective_total_rate_bps: 4500,
        has_override: false,
        override_rate_bps: null,
      },
      {
        group: 'vip',
        effective_total_rate_bps: 3000,
        has_override: true,
        override_rate_bps: 2500,
      },
    ]),
    [
      {
        id: 'subscription:default',
        type: 'subscription',
        group: 'default',
        effectiveTotalRateBps: 4500,
        hasOverride: false,
        overrideRateBps: null,
        overrideRatePercent: 45,
        inputPercent: 45,
        isDraft: false,
      },
      {
        id: 'subscription:vip',
        type: 'subscription',
        group: 'vip',
        effectiveTotalRateBps: 3000,
        hasOverride: true,
        overrideRateBps: 2500,
        overrideRatePercent: 25,
        inputPercent: 25,
        isDraft: false,
      },
    ],
  );
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
    new URL('../src/components/topup/InvitationCard.jsx', import.meta.url),
    'utf8',
  );

  assert.equal(invitationCardSource.includes('referralSavingGroup'), false);
});

test('InvitationCard imports rateBpsToPercentNumber for grouped invitee max validation', () => {
  const invitationCardSource = readFileSync(
    new URL('../src/components/topup/InvitationCard.jsx', import.meta.url),
    'utf8',
  );

  assert.match(
    invitationCardSource,
    /import\s*{[\s\S]*\brateBpsToPercentNumber\b[\s\S]*}\s*from\s*['"]\.\.\/\.\.\/helpers\/subscriptionReferral['"]/,
  );
});

test('InvitationCard only renders the subscription referral card when grouped rebates exist', () => {
  const invitationCardSource = readFileSync(
    new URL('../src/components/topup/InvitationCard.jsx', import.meta.url),
    'utf8',
  );

  assert.match(
    invitationCardSource,
    /const visibleReferralGroups = \(referralGroups \|\| \[\]\)\.filter/,
  );
  assert.match(
    invitationCardSource,
    /\{visibleReferralGroups\.length > 0 && \(/,
  );
});

test('OperationSetting no longer mounts the global subscription referral settings card', () => {
  const operationSettingSource = readFileSync(
    new URL('../src/components/settings/OperationSetting.jsx', import.meta.url),
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

test('SubscriptionReferralOverrideSection keeps the extensible override list workflow', () => {
  const overrideSource = readFileSync(
    new URL(
      '../src/components/table/users/modals/SubscriptionReferralOverrideSection.jsx',
      import.meta.url,
    ),
    'utf8',
  );

  assert.match(
    overrideSource,
    /buildAdminOverrideRows\(\s*Array\.isArray\(next\.groups\) \? next\.groups : \[\],\s*\)/,
  );
  assert.match(
    overrideSource,
    /API\.get\(`\/api\/subscription\/admin\/referral\/users\/\$\{userId\}`\)/,
  );
  assert.match(overrideSource, /API\.get\(['"`]\/api\/group\/['"`]\)/);
  assert.match(overrideSource, /createAdminOverrideDraftRow\(\)/);
  assert.match(overrideSource, /buildAdminOverrideGroupOptions\(/);
  assert.match(overrideSource, /t\('新增覆盖'\)/);
  assert.equal(
    overrideSource.includes('/api/subscription/admin/referral/settings'),
    false,
  );
});

test('topup self save refreshes grouped referral summaries instead of synthesizing top-level fallbacks', () => {
  const topupSource = readFileSync(
    new URL('../src/components/topup/index.jsx', import.meta.url),
    'utf8',
  );

  assert.match(topupSource, /await getSubscriptionReferralSelf\(\);/);
  assert.equal(topupSource.includes('data.total_rate_bps'), false);
  assert.equal(topupSource.includes('data.invitee_rate_bps'), false);
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
