import test from 'node:test';
import assert from 'node:assert/strict';

import {
  buildGroupOption,
  buildGroupOptions,
} from '../src/helpers/groupOptions.js';

test('buildGroupOption keeps group name as visible label when description is empty', () => {
  const option = buildGroupOption('claude-code-oups4.6', {
    desc: '',
    ratio: 1,
  });

  assert.equal(option.label, 'claude-code-oups4.6');
  assert.equal(option.value, 'claude-code-oups4.6');
  assert.equal(option.desc, '');
  assert.equal(option.optionDescription, '');
  assert.equal(option.ratio, 1);
});

test('buildGroupOption truncates long descriptions only for dropdown secondary text', () => {
  const option = buildGroupOption(
    'vip',
    {
      desc: '这是一个非常长的描述，用来确认下拉框里的副标题会被截断',
      ratio: 0.8,
    },
    { truncateDescAt: 8 },
  );

  assert.equal(option.label, 'vip');
  assert.equal(
    option.fullLabel,
    '这是一个非常长的描述，用来确认下拉框里的副标题会被截断',
  );
  assert.equal(option.optionDescription, '这是一个非常长的...');
  assert.equal(option.ratio, 0.8);
});

test('buildGroupOptions moves the current user group to the front', () => {
  const options = buildGroupOptions(
    {
      default: { desc: '默认分组', ratio: 1 },
      vip: { desc: 'VIP 分组', ratio: 0.8 },
    },
    { userGroup: 'vip' },
  );

  assert.deepEqual(
    options.map((option) => option.value),
    ['vip', 'default'],
  );
});
