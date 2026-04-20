import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const testDir = path.dirname(fileURLToPath(import.meta.url));
const webRoot = path.resolve(testDir, '..');
const helperPath = path.join(webRoot, 'src/helpers/withdrawal.js');
const helperUrl = pathToFileURL(helperPath).href;

const readHelperSource = () => fs.readFileSync(helperPath, 'utf8');
const loadHelpers = () => import(`${helperUrl}?t=${Date.now()}`);

test('withdrawal helper test resolves files from repo-relative paths', () => {
  const source = fs.readFileSync(fileURLToPath(import.meta.url), 'utf8');

  assert.equal(webRoot, path.resolve(testDir, '..'));
  assert.doesNotMatch(source, /\/Users\/money\/project/);
});

test('withdrawal helper source exports fee rule editor helpers and range copy', () => {
  const source = readHelperSource();

  assert.match(source, /export const normalizeWithdrawalFeeEditorRules\s*=/);
  assert.match(source, /export const validateWithdrawalFeeEditorRules\s*=/);
  assert.match(source, /export const serializeWithdrawalFeeEditorRules\s*=/);
  assert.match(source, /export const describeWithdrawalFeeRule\s*=/);
  assert.match(source, /export const buildWithdrawalFeeSamples\s*=/);
  assert.match(source, /0 < 金额 <=/);
});

test('fee rule editor helpers normalize validate serialize and describe rules', async () => {
  const {
    normalizeWithdrawalFeeEditorRules,
    validateWithdrawalFeeEditorRules,
    serializeWithdrawalFeeEditorRules,
    describeWithdrawalFeeRule,
  } = await loadHelpers();

  const normalized = normalizeWithdrawalFeeEditorRules([
    {
      min_amount: 100,
      max_amount: 500,
      fee_type: 'ratio',
      fee_value: 3,
      min_fee: 1,
      max_fee: 10,
      enabled: true,
      sort_order: 2,
    },
    {
      min_amount: 0,
      max_amount: 100,
      fee_type: 'fixed',
      fee_value: 5,
      enabled: true,
      sort_order: 1,
    },
    {
      min_amount: 500,
      max_amount: 0,
      fee_type: 'ratio',
      fee_value: 1,
      min_fee: 0,
      max_fee: 0,
      enabled: false,
      sort_order: 3,
    },
  ]);

  assert.deepEqual(
    normalized.map((rule) => ({
      minAmount: rule.minAmount,
      maxAmount: rule.maxAmount,
      feeType: rule.feeType,
      feeValue: rule.feeValue,
      enabled: rule.enabled,
      sortOrder: rule.sortOrder,
    })),
    [
      {
        minAmount: 0,
        maxAmount: 100,
        feeType: 'fixed',
        feeValue: 5,
        enabled: true,
        sortOrder: 1,
      },
      {
        minAmount: 100,
        maxAmount: 500,
        feeType: 'ratio',
        feeValue: 3,
        enabled: true,
        sortOrder: 2,
      },
      {
        minAmount: 500,
        maxAmount: '',
        feeType: 'ratio',
        feeValue: 1,
        enabled: false,
        sortOrder: 3,
      },
    ],
  );

  assert.equal(describeWithdrawalFeeRule(normalized[0]), '0 < 金额 <= 100：固定手续费 5');
  assert.equal(describeWithdrawalFeeRule(normalized[1]), '100 < 金额 <= 500：按 3% 收费，最低 1，最高 10');
  assert.equal(describeWithdrawalFeeRule(normalized[2]), '金额 > 500：按 1% 收费');

  assert.deepEqual(
    validateWithdrawalFeeEditorRules(normalized),
    {
      errors: [],
      warnings: ['第 3 条规则已停用，用户侧不会命中它'],
    },
  );

  assert.equal(
    serializeWithdrawalFeeEditorRules(normalized),
    JSON.stringify([
      {
        min_amount: 0,
        max_amount: 100,
        fee_type: 'fixed',
        fee_value: 5,
        min_fee: 0,
        max_fee: 0,
        enabled: true,
        sort_order: 1,
      },
      {
        min_amount: 100,
        max_amount: 500,
        fee_type: 'ratio',
        fee_value: 3,
        min_fee: 1,
        max_fee: 10,
        enabled: true,
        sort_order: 2,
      },
      {
        min_amount: 500,
        max_amount: 0,
        fee_type: 'ratio',
        fee_value: 1,
        min_fee: 0,
        max_fee: 0,
        enabled: false,
        sort_order: 3,
      },
    ]),
  );
});

test('validation reports overlap errors and gap warnings', async () => {
  const { validateWithdrawalFeeEditorRules } = await loadHelpers();

  const feedback = validateWithdrawalFeeEditorRules([
    {
      minAmount: 0,
      maxAmount: 100,
      feeType: 'fixed',
      feeValue: 5,
      enabled: true,
      sortOrder: 1,
    },
    {
      minAmount: 80,
      maxAmount: 200,
      feeType: 'fixed',
      feeValue: 6,
      enabled: true,
      sortOrder: 2,
    },
    {
      minAmount: 300,
      maxAmount: '',
      feeType: 'fixed',
      feeValue: 7,
      enabled: true,
      sortOrder: 3,
    },
  ]);

  assert.match(feedback.errors.join('\n'), /第 1 条规则和第 2 条规则的金额区间发生重叠/);
  assert.match(feedback.warnings.join('\n'), /第 2 条规则和第 3 条规则之间存在金额区间断层/);
});

test('disabled rules are ignored by cross-rule validation', async () => {
  const { validateWithdrawalFeeEditorRules } = await loadHelpers();

  const feedback = validateWithdrawalFeeEditorRules([
    {
      minAmount: 0,
      maxAmount: 100,
      feeType: 'fixed',
      feeValue: 5,
      enabled: true,
      sortOrder: 1,
    },
    {
      minAmount: 50,
      maxAmount: '',
      feeType: 'fixed',
      feeValue: 6,
      enabled: false,
      sortOrder: 2,
    },
    {
      minAmount: 100,
      maxAmount: 300,
      feeType: 'fixed',
      feeValue: 7,
      enabled: true,
      sortOrder: 3,
    },
  ]);

  assert.deepEqual(feedback.errors, []);
  assert.deepEqual(feedback.warnings, ['第 2 条规则已停用，用户侧不会命中它']);
});

test('sample previews and withdrawal preview use left-open right-closed matching', async () => {
  const {
    buildWithdrawalFeeSamples,
    calculateWithdrawalPreview,
  } = await loadHelpers();

  const rules = [
    {
      min_amount: 0,
      max_amount: 100,
      fee_type: 'fixed',
      fee_value: 5,
      min_fee: 0,
      max_fee: 0,
      enabled: true,
      sort_order: 1,
    },
    {
      min_amount: 100,
      max_amount: 500,
      fee_type: 'ratio',
      fee_value: 3,
      min_fee: 1,
      max_fee: 10,
      enabled: true,
      sort_order: 2,
    },
    {
      min_amount: 500,
      max_amount: 0,
      fee_type: 'ratio',
      fee_value: 1,
      min_fee: 0,
      max_fee: 0,
      enabled: true,
      sort_order: 3,
    },
  ];

  assert.deepEqual(calculateWithdrawalPreview(0, rules), {
    feeAmount: 0,
    netAmount: 0,
    matchedRule: null,
    isValid: false,
    blockReason: '提现金额必须大于 0',
  });
  assert.equal(calculateWithdrawalPreview(100, rules).matchedRule?.sort_order, 1);
  assert.equal(calculateWithdrawalPreview(100.01, rules).matchedRule?.sort_order, 2);
  assert.equal(calculateWithdrawalPreview(500, rules).matchedRule?.sort_order, 2);
  assert.equal(calculateWithdrawalPreview(500.01, rules).matchedRule?.sort_order, 3);

  assert.deepEqual(calculateWithdrawalPreview(99, [
    {
      min_amount: 100,
      max_amount: 500,
      fee_type: 'fixed',
      fee_value: 5,
      enabled: true,
      sort_order: 1,
    },
  ]), {
    feeAmount: null,
    netAmount: null,
    matchedRule: null,
    isValid: false,
    blockReason: '当前提现金额未命中任何手续费规则，请调整金额或联系管理员',
  });

  const samples = buildWithdrawalFeeSamples(rules);
  assert.deepEqual(
    samples.map((sample) => sample.amount),
    [50, 100, 300, 500, 1000],
  );
  assert.deepEqual(
    samples.map((sample) => sample.ruleText),
    [
      '0 < 金额 <= 100：固定手续费 5',
      '0 < 金额 <= 100：固定手续费 5',
      '100 < 金额 <= 500：按 3% 收费，最低 1，最高 10',
      '100 < 金额 <= 500：按 3% 收费，最低 1，最高 10',
      '金额 > 500：按 1% 收费',
    ],
  );
  assert.deepEqual(
    samples.map((sample) => sample.feeAmount),
    [5, 5, 9, 10, 10],
  );
});
