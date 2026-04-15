import test from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

import {
  canMergeGroups,
  buildLegacyGroupOptionPayload,
  buildGroupUpsertPayload,
  buildMergePayload,
} from '../src/helpers/pricingGroupAdmin.js';

const groupRatioSettingsPath = new URL(
  '../src/pages/Setting/Ratio/GroupRatioSettings.jsx',
  import.meta.url,
);
const groupTablePath = new URL(
  '../src/pages/Setting/Ratio/components/GroupTable.jsx',
  import.meta.url,
);

test('buildGroupUpsertPayload keeps group_key immutable during edits', () => {
  assert.deepEqual(
    buildGroupUpsertPayload(
      { group_key: 'cc-opus4.6', display_name: '旧名' },
      {
        group_key: 'cc-hacked',
        display_name: '新名',
        billing_ratio: 0.8,
        user_selectable: true,
      },
    ),
    {
      group_key: 'cc-opus4.6',
      display_name: '新名',
      billing_ratio: 0.8,
      user_selectable: true,
      description: '',
      sort_order: 0,
      status: 1,
    },
  );
});

test('buildMergePayload trims merge keys', () => {
  assert.deepEqual(
    buildMergePayload({
      source_group_key: '  cc-legacy  ',
      target_group_key: '  cc-opus4.6  ',
    }),
    {
      source_group_key: 'cc-legacy',
      target_group_key: 'cc-opus4.6',
    },
  );
});

test('canMergeGroups only allows two distinct non-empty keys', () => {
  assert.equal(
    canMergeGroups({ source_group_key: 'cc-legacy', target_group_key: 'cc-opus4.6' }),
    true,
  );
  assert.equal(
    canMergeGroups({ source_group_key: 'cc-legacy', target_group_key: 'cc-legacy' }),
    false,
  );
  assert.equal(
    canMergeGroups({ source_group_key: '', target_group_key: 'cc-opus4.6' }),
    false,
  );
});

test('buildLegacyGroupOptionPayload excludes archived groups from legacy sync', () => {
  assert.deepEqual(
    buildLegacyGroupOptionPayload([
      {
        group_key: 'default',
        display_name: 'Default',
        billing_ratio: 1,
        user_selectable: false,
        status: 1,
      },
      {
        group_key: 'premium',
        display_name: 'Premium',
        billing_ratio: 0.8,
        user_selectable: true,
        status: 2,
      },
      {
        group_key: 'legacy-archived',
        display_name: 'Legacy Archived',
        billing_ratio: 3,
        user_selectable: true,
        status: 3,
      },
    ]),
    {
      GroupRatio: JSON.stringify(
        {
          default: 1,
          premium: 0.8,
        },
        null,
        2,
      ),
      UserUsableGroups: JSON.stringify(
        {
          premium: 'Premium',
        },
        null,
        2,
      ),
    },
  );
});

test('GroupRatioSettings exposes a merge action in visual mode', async () => {
  const source = await readFile(groupRatioSettingsPath, 'utf8');

  assert.match(source, /mergePricingGroups/);
  assert.match(source, /canMergeGroups/);
  assert.match(source, /分组合并/);
  assert.match(source, /source_group_key/);
  assert.match(source, /target_group_key/);
  assert.match(source, /disabled=\{!isMergeReady\}/);
  assert.match(source, /group\.status !== 3/);
  assert.match(source, /将会把.*合并到/);
  assert.match(source, /legacy 引用一致性/);
  assert.match(source, /listPricingGroupConsistencyReport/);
  assert.match(source, /activeGroupNames/);
});

test('canonical GroupTable uses archive wording instead of hard delete wording', async () => {
  const source = await readFile(groupTablePath, 'utf8');

  assert.match(source, /确认归档该分组/);
  assert.match(source, /排序/);
  assert.match(source, /状态/);
});
