import test from 'node:test';
import assert from 'node:assert/strict';

import {
  buildReferralTypeOptions,
  buildReferralLevelTypeOptions,
  formatReferralGroupLabel,
  formatReferralLevelTypeLabel,
  formatReferralTemplateOptionLabel,
  formatReferralTypeLabel,
} from '../src/helpers/referralLabels.js';

const fakeT = (key) =>
  (
    {
      '订阅返佣': '订阅返佣',
      '直推模板（direct）': '直推模板（direct）',
      '团队模板（team）': '团队模板（team）',
      '所有分组': '所有分组',
    }[key] || key
  );

test('referral label helpers localize referral type, level type, and empty groups', () => {
  assert.equal(formatReferralTypeLabel('subscription_referral', fakeT), '订阅返佣');
  assert.equal(formatReferralLevelTypeLabel('direct', fakeT), '直推模板（direct）');
  assert.equal(formatReferralLevelTypeLabel('team', fakeT), '团队模板（team）');
  assert.equal(formatReferralGroupLabel('', fakeT), '所有分组');
  assert.equal(formatReferralGroupLabel('vip', fakeT), 'vip');
});

test('referral option builders expose localized labels', () => {
  assert.deepEqual(buildReferralTypeOptions(fakeT), [
    { label: '订阅返佣', value: 'subscription_referral' },
  ]);
  assert.deepEqual(buildReferralLevelTypeOptions(fakeT), [
    { label: '直推模板（direct）', value: 'direct' },
    { label: '团队模板（team）', value: 'team' },
  ]);
});

test('referral template option label prefers template name and avoids redundant scope suffixes', () => {
  assert.equal(
    formatReferralTemplateOptionLabel(
      {
        name: 'default-直推模板',
        group: 'default',
        level_type: 'direct',
      },
      fakeT,
    ),
    'default-直推模板',
  );

  assert.equal(
    formatReferralTemplateOptionLabel(
      {
        name: '',
        group: 'vip',
        level_type: 'team',
      },
      fakeT,
    ),
    'vip · 团队模板（team）',
  );
});
