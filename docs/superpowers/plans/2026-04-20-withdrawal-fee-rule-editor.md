# Withdrawal Fee Rule Editor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the withdrawal fee JSON textarea with a reusable inline rule-table editor, unify fee-range semantics to left-open/right-closed, and surface clearer fee explanations plus preview behavior in the user withdrawal flow.

**Architecture:** Keep `WithdrawalFeeRules` as the persisted option value, but introduce a frontend editor model plus helper functions that convert between table rows and the current JSON structure. Mirror the same range semantics in Go model code so admin validation, user preview, and submission-time fee calculation all agree on the same matching behavior.

**Tech Stack:** React + Semi UI + the current helper/source-test pattern (`node --test`), Go + GORM model tests, the current wallet withdrawal UI, and OptionMap config plumbing.

---

## File Structure

### Shared frontend helper layer

- Modify: `web/src/helpers/withdrawal.js`
  - Add editor-model normalization, serialization, validation, human-readable summaries, sample preview generation, and update preview matching semantics to left-open/right-closed.
- Create: `web/tests/withdrawal-fee-rule-editor.test.mjs`
  - Source assertions for helper exports, editor warnings, and inline-editor copy.

### Admin withdrawal settings UI

- Modify: `web/src/pages/Setting/Payment/SettingsWithdrawal.jsx`
  - Replace raw JSON textarea usage with a structured editor container and persist serialized rules.
- Create: `web/src/components/settings/withdrawal/WithdrawalFeeRulesEditor.jsx`
  - Rule table, add/delete/move controls, validation summary, and sample preview section.
- Create: `web/src/components/settings/withdrawal/WithdrawalFeeRuleInlineForm.jsx`
  - Single expanded-row form for editing one rule inline.
- Modify: `web/tests/withdrawal-settings.test.mjs`
  - Assert that settings page now uses the new editor instead of long JSON help text.

### Backend rule parsing and fee calculation

- Modify: `model/user_withdrawal_setting.go`
  - Tighten rule validation for empty ranges and keep overlap checks compatible with touching boundaries.
- Modify: `model/user_withdrawal.go`
  - Add a shared amount-matching helper using left-open/right-closed semantics and use it in fee calculation.
- Modify: `model/user_withdrawal_test.go`
  - Add regression tests for exact boundary hits, empty-range rejection, and unmatched amounts.

### User withdrawal UX and copy

- Modify: `web/src/components/topup/modals/WithdrawalApplyModal.jsx`
  - Show natural-language fee rules plus a clearer “matched rule / fee / net amount” summary.
- Modify: `web/src/components/topup/index.jsx`
  - Block submission when the amount matches no fee rule and wire the new helper outputs into the modal.
- Modify: `web/tests/wallet-withdrawal.test.mjs`
  - Assert the unmatched-rule guard and richer rule-preview copy.
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh-CN.json`
- Modify: `web/src/i18n/locales/zh-TW.json`
- Modify: `web/src/i18n/locales/ja.json`
- Modify: `web/src/i18n/locales/fr.json`
- Modify: `web/src/i18n/locales/ru.json`
- Modify: `web/src/i18n/locales/vi.json`
- Modify: `web/tests/withdrawal-locales.test.mjs`
  - Add the new admin-editor and unmatched-rule copy keys to locale coverage.

---

### Task 1: Build the Shared Withdrawal Rule Helper Layer

**Files:**
- Modify: `web/src/helpers/withdrawal.js`
- Test: `web/tests/withdrawal-fee-rule-editor.test.mjs`

- [ ] **Step 1: Write the failing helper/source test**

```js
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';

const readSource = (relativePath) =>
  fs.readFileSync(
    path.join('/Users/money/project/subproject/hermestoken/web', relativePath),
    'utf8',
  );

test('withdrawal helper exposes rule editor primitives', () => {
  const source = readSource('src/helpers/withdrawal.js');
  assert.match(source, /export const normalizeWithdrawalFeeEditorRules/);
  assert.match(source, /export const validateWithdrawalFeeEditorRules/);
  assert.match(source, /export const serializeWithdrawalFeeEditorRules/);
  assert.match(source, /export const describeWithdrawalFeeRule/);
  assert.match(source, /export const buildWithdrawalFeeSamples/);
  assert.match(source, /0 < 金额 <=/);
});
```

- [ ] **Step 2: Run the helper test to verify it fails**

Run: `cd /Users/money/project/subproject/hermestoken/web && node --test tests/withdrawal-fee-rule-editor.test.mjs`
Expected: FAIL with undefined exports such as `normalizeWithdrawalFeeEditorRules` and absent range-copy assertions.

- [ ] **Step 3: Implement the helper functions and updated preview semantics**

```js
const DEFAULT_RULE = {
  id: '',
  min_amount: 0,
  max_amount: '',
  fee_type: 'fixed',
  fee_value: 0,
  min_fee: 0,
  max_fee: 0,
  enabled: true,
  sort_order: 1,
};

const toEditorAmount = (value, fallback = '') =>
  value === '' || value === null || value === undefined
    ? fallback
    : Number(value);

export const normalizeWithdrawalFeeEditorRules = (feeRules = []) =>
  feeRules.map((rule, index) => ({
    ...DEFAULT_RULE,
    ...rule,
    id: rule?.id || `rule-${index + 1}`,
    max_amount:
      Number(rule?.max_amount || 0) > 0 ? Number(rule.max_amount) : '',
    sort_order: index + 1,
  }));

export const matchesWithdrawalFeeRuleAmount = (amount, rule) => {
  const numericAmount = Number(amount || 0);
  const min = Number(rule?.min_amount || 0);
  const max = Number(rule?.max_amount || 0);
  if (!Number.isFinite(numericAmount) || numericAmount <= 0) return false;
  if (min === 0) {
    if (numericAmount <= 0) return false;
  } else if (numericAmount <= min) {
    return false;
  }
  if (max > 0 && numericAmount > max) return false;
  return true;
};

export const validateWithdrawalFeeEditorRules = (rules = []) => {
  const errors = [];
  const warnings = [];
  const enabledRules = rules
    .filter((rule) => rule?.enabled !== false)
    .map((rule, index) => ({
      ...rule,
      min_amount: Number(rule?.min_amount || 0),
      max_amount: toEditorAmount(rule?.max_amount, ''),
      sort_order: index + 1,
    }));

  enabledRules.forEach((rule, index) => {
    if (rule.max_amount !== '' && Number(rule.max_amount) <= rule.min_amount) {
      errors.push(`第 ${index + 1} 条规则的结束金额必须大于起始金额`);
    }
  });

  for (let i = 1; i < enabledRules.length; i += 1) {
    const previous = enabledRules[i - 1];
    const current = enabledRules[i];
    if (previous.max_amount === '') {
      errors.push(`第 ${i} 条规则已无上限，后面不能继续配置规则`);
      break;
    }
    if (current.min_amount < previous.max_amount) {
      errors.push(`第 ${i} 条规则和第 ${i + 1} 条规则的金额区间发生重叠`);
    }
    if (current.min_amount > previous.max_amount) {
      warnings.push(
        `第 ${i} 条规则和第 ${i + 1} 条规则之间存在未覆盖金额区间`,
      );
    }
  }

  return { errors, warnings };
};

export const serializeWithdrawalFeeEditorRules = (rules = []) =>
  JSON.stringify(
    rules.map((rule, index) => ({
      min_amount: Number(rule.min_amount || 0),
      max_amount: rule.max_amount === '' ? 0 : Number(rule.max_amount || 0),
      fee_type: rule.fee_type,
      fee_value: Number(rule.fee_value || 0),
      min_fee: Number(rule.min_fee || 0),
      max_fee: Number(rule.max_fee || 0),
      enabled: rule.enabled !== false,
      sort_order: index + 1,
    })),
  );

export const describeWithdrawalFeeRule = (rule) => {
  const min = Number(rule?.min_amount || 0);
  const max = Number(rule?.max_amount || 0);
  if (max > 0) {
    return min === 0 ? `0 < 金额 <= ${max}` : `${min} < 金额 <= ${max}`;
  }
  return `金额 > ${min}`;
};

export const buildWithdrawalFeeSamples = (rules = []) =>
  [50, 100, 300, 1000, 3000].map((amount) => ({
    amount,
    preview: calculateWithdrawalPreview(amount, rules),
  }));
```

- [ ] **Step 4: Run the helper test to verify it passes**

Run: `cd /Users/money/project/subproject/hermestoken/web && node --test tests/withdrawal-fee-rule-editor.test.mjs`
Expected: PASS with `ok 1 - withdrawal helper exposes rule editor primitives`.

- [ ] **Step 5: Commit the helper layer**

```bash
cd /Users/money/project/subproject/hermestoken
git add web/src/helpers/withdrawal.js web/tests/withdrawal-fee-rule-editor.test.mjs
git commit -m "feat: add withdrawal fee rule editor helpers"
```

### Task 2: Replace the JSON Textarea with an Inline Rule Editor

**Files:**
- Create: `web/src/components/settings/withdrawal/WithdrawalFeeRulesEditor.jsx`
- Create: `web/src/components/settings/withdrawal/WithdrawalFeeRuleInlineForm.jsx`
- Modify: `web/src/pages/Setting/Payment/SettingsWithdrawal.jsx`
- Test: `web/tests/withdrawal-settings.test.mjs`

- [ ] **Step 1: Extend the settings source test so the old JSON-only UI fails**

```js
test('payment settings render the inline withdrawal fee rule editor', () => {
  const paymentSettingSource = readSource(
    'src/components/settings/PaymentSetting.jsx',
  );
  const settingsWithdrawalSource = readSource(
    'src/pages/Setting/Payment/SettingsWithdrawal.jsx',
  );
  const editorSource = readSource(
    'src/components/settings/withdrawal/WithdrawalFeeRulesEditor.jsx',
  );

  assert.match(paymentSettingSource, /SettingsWithdrawal/);
  assert.match(settingsWithdrawalSource, /WithdrawalFeeRulesEditor/);
  assert.doesNotMatch(settingsWithdrawalSource, /匹配第一条 enabled=true 且金额区间命中的规则/);
  assert.match(editorSource, /新增规则/);
  assert.match(editorSource, /上移/);
  assert.match(editorSource, /下移/);
  assert.match(editorSource, /恢复默认示例/);
});
```

- [ ] **Step 2: Run the settings test to verify it fails**

Run: `cd /Users/money/project/subproject/hermestoken/web && node --test tests/withdrawal-settings.test.mjs`
Expected: FAIL because `WithdrawalFeeRulesEditor` does not exist and the old long JSON hint text is still present.

- [ ] **Step 3: Build the inline editor components and wire them into settings**

```jsx
// web/src/components/settings/withdrawal/WithdrawalFeeRulesEditor.jsx
import React, { useMemo, useState } from 'react';
import { Button, Tag, Typography } from '@douyinfe/semi-ui';
import {
  buildWithdrawalFeeSamples,
  describeWithdrawalFeeRule,
  normalizeWithdrawalFeeEditorRules,
  validateWithdrawalFeeEditorRules,
} from '../../../helpers/withdrawal';
import WithdrawalFeeRuleInlineForm from './WithdrawalFeeRuleInlineForm';

const { Text } = Typography;

const createDefaultRule = (sortOrder) => ({
  id: `rule-${Date.now()}-${sortOrder}`,
  min_amount: 0,
  max_amount: '',
  fee_type: 'fixed',
  fee_value: 0,
  min_fee: 0,
  max_fee: 0,
  enabled: true,
  sort_order: sortOrder,
});

export default function WithdrawalFeeRulesEditor({ value = [], onChange }) {
  const [editingRuleId, setEditingRuleId] = useState('');
  const rules = useMemo(
    () => normalizeWithdrawalFeeEditorRules(value),
    [value],
  );
  const feedback = useMemo(
    () => validateWithdrawalFeeEditorRules(rules),
    [rules],
  );

  const replaceRule = (ruleId, nextRule) =>
    onChange(rules.map((rule) => (rule.id === ruleId ? nextRule : rule)));

  const moveRule = (ruleId, direction) => {
    const index = rules.findIndex((rule) => rule.id === ruleId);
    const nextIndex = index + direction;
    if (index < 0 || nextIndex < 0 || nextIndex >= rules.length) return;
    const reordered = [...rules];
    [reordered[index], reordered[nextIndex]] = [
      reordered[nextIndex],
      reordered[index],
    ];
    onChange(reordered);
  };

  return (
    <div className='space-y-4'>
      <div className='flex items-center justify-between'>
        <Text strong>提现手续费规则</Text>
        <div className='flex gap-2'>
          <Button onClick={() => onChange([...rules, createDefaultRule(rules.length + 1)])}>
            新增规则
          </Button>
          <Button theme='borderless' onClick={() => onChange(normalizeWithdrawalFeeEditorRules([
            { min_amount: 0, max_amount: 100, fee_type: 'fixed', fee_value: 5, enabled: true, sort_order: 1 },
            { min_amount: 100, max_amount: 500, fee_type: 'ratio', fee_value: 3, enabled: true, sort_order: 2 },
            { min_amount: 500, max_amount: 2000, fee_type: 'ratio', fee_value: 2, enabled: true, sort_order: 3 },
            { min_amount: 2000, max_amount: '', fee_type: 'ratio', fee_value: 1, enabled: true, sort_order: 4 },
          ]))}>
            恢复默认示例
          </Button>
        </div>
      </div>
      {rules.map((rule, index) => (
        <div key={rule.id} className='rounded-xl border border-[var(--semi-color-border)]'>
          <div className='flex items-center justify-between px-4 py-3'>
            <div className='space-y-1'>
              <div>{index + 1}. {describeWithdrawalFeeRule(rule)}</div>
              <div className='text-sm text-[var(--semi-color-text-2)]'>
                {rule.fee_type === 'fixed' ? `固定手续费 ${rule.fee_value}` : `按 ${rule.fee_value}% 收费`}
              </div>
            </div>
            <div className='flex items-center gap-2'>
              <Tag color={rule.enabled ? 'green' : 'grey'}>
                {rule.enabled ? '启用' : '停用'}
              </Tag>
              <Button size='small' theme='borderless' onClick={() => moveRule(rule.id, -1)}>上移</Button>
              <Button size='small' theme='borderless' onClick={() => moveRule(rule.id, 1)}>下移</Button>
              <Button size='small' onClick={() => setEditingRuleId(rule.id)}>编辑</Button>
            </div>
          </div>
          {editingRuleId === rule.id ? (
            <WithdrawalFeeRuleInlineForm
              rule={rule}
              onCancel={() => setEditingRuleId('')}
              onSave={(nextRule) => {
                replaceRule(rule.id, nextRule);
                setEditingRuleId('');
              }}
            />
          ) : null}
        </div>
      ))}
      {feedback.errors.map((message) => <Tag key={message} color='red'>{message}</Tag>)}
      {feedback.warnings.map((message) => <Tag key={message} color='orange'>{message}</Tag>)}
      {buildWithdrawalFeeSamples(rules).map(({ amount, preview }) => (
        <div key={amount} className='text-sm text-[var(--semi-color-text-2)]'>
          {amount} -> 手续费 {preview?.feeAmount ?? 0}
        </div>
      ))}
    </div>
  );
}
```

```jsx
// web/src/components/settings/withdrawal/WithdrawalFeeRuleInlineForm.jsx
import React, { useState } from 'react';
import { Button, Form } from '@douyinfe/semi-ui';

export default function WithdrawalFeeRuleInlineForm({
  rule,
  onCancel,
  onSave,
}) {
  const [draft, setDraft] = useState(rule);

  return (
    <div className='border-t border-[var(--semi-color-border)] px-4 py-4 bg-[var(--semi-color-fill-0)]'>
      <Form onValueChange={(values) => setDraft((current) => ({ ...current, ...values }))}>
        <div className='grid grid-cols-2 gap-3'>
          <Form.InputNumber field='min_amount' label='起始金额' initValue={draft.min_amount} />
          <Form.InputNumber field='max_amount' label='结束金额' initValue={draft.max_amount} />
          <Form.Select
            field='fee_type'
            label='收费方式'
            initValue={draft.fee_type}
            optionList={[
              { label: '固定手续费', value: 'fixed' },
              { label: '按比例', value: 'ratio' },
            ]}
          />
          <Form.InputNumber
            field='fee_value'
            label={draft.fee_type === 'fixed' ? '固定手续费' : '费率'}
            initValue={draft.fee_value}
          />
          {draft.fee_type === 'ratio' ? (
            <>
              <Form.InputNumber field='min_fee' label='最低手续费' initValue={draft.min_fee} />
              <Form.InputNumber field='max_fee' label='最高手续费' initValue={draft.max_fee} />
            </>
          ) : null}
          <Form.Switch field='enabled' label='启用状态' initValue={draft.enabled} />
        </div>
      </Form>
      <div className='mt-4 flex justify-end gap-2'>
        <Button theme='borderless' onClick={onCancel}>取消</Button>
        <Button onClick={() => onSave(draft)}>保存</Button>
      </div>
    </div>
  );
}
```

```jsx
// web/src/pages/Setting/Payment/SettingsWithdrawal.jsx
import WithdrawalFeeRulesEditor from '../../../components/settings/withdrawal/WithdrawalFeeRulesEditor';
import {
  normalizeWithdrawalFeeEditorRules,
  serializeWithdrawalFeeEditorRules,
  validateWithdrawalFeeEditorRules,
} from '../../../helpers/withdrawal';

const currentInputs = {
  WithdrawalEnabled: toBoolean(options.WithdrawalEnabled),
  WithdrawalMinAmount: parseFloat(options.WithdrawalMinAmount || 10),
  WithdrawalInstruction: options.WithdrawalInstruction || '',
  WithdrawalFeeRules: normalizeWithdrawalFeeEditorRules(
    JSON.parse(options.WithdrawalFeeRules || '[]'),
  ),
};

const submit = async () => {
  const feedback = validateWithdrawalFeeEditorRules(inputs.WithdrawalFeeRules);
  if (feedback.errors.length > 0) {
    showError(feedback.errors[0]);
    return;
  }

  const results = await Promise.all([
    API.put('/api/option/', {
      key: 'WithdrawalEnabled',
      value: Boolean(inputs.WithdrawalEnabled),
    }),
    API.put('/api/option/', {
      key: 'WithdrawalMinAmount',
      value: String(inputs.WithdrawalMinAmount || 0),
    }),
    API.put('/api/option/', {
      key: 'WithdrawalInstruction',
      value: inputs.WithdrawalInstruction || '',
    }),
    API.put('/api/option/', {
      key: 'WithdrawalFeeRules',
      value: serializeWithdrawalFeeEditorRules(inputs.WithdrawalFeeRules),
    }),
  ]);
  const errorResult = results.find((res) => !res?.data?.success);
  if (errorResult) {
    showError(errorResult.data.message || t('更新失败'));
    return;
  }
  showSuccess(t('更新成功'));
};

<WithdrawalFeeRulesEditor
  value={inputs.WithdrawalFeeRules}
  onChange={(rules) =>
    setInputs((current) => ({ ...current, WithdrawalFeeRules: rules }))
  }
/>
```

- [ ] **Step 4: Run the settings tests to verify they pass**

Run: `cd /Users/money/project/subproject/hermestoken/web && node --test tests/withdrawal-settings.test.mjs tests/withdrawal-fee-rule-editor.test.mjs`
Expected: PASS with both tests green and no assertion for the removed JSON help block.

- [ ] **Step 5: Commit the settings editor UI**

```bash
cd /Users/money/project/subproject/hermestoken
git add \
  web/src/components/settings/withdrawal/WithdrawalFeeRulesEditor.jsx \
  web/src/components/settings/withdrawal/WithdrawalFeeRuleInlineForm.jsx \
  web/src/pages/Setting/Payment/SettingsWithdrawal.jsx \
  web/tests/withdrawal-settings.test.mjs
git commit -m "feat: add inline withdrawal fee rule editor"
```

### Task 3: Align Backend Rule Validation and Fee Matching with the New Semantics

**Files:**
- Modify: `model/user_withdrawal_setting.go`
- Modify: `model/user_withdrawal.go`
- Modify: `model/user_withdrawal_test.go`

- [ ] **Step 1: Add failing Go tests for the new range semantics**

```go
func TestCalculateWithdrawalFeeAmountUsesLeftOpenRightClosedRanges(t *testing.T) {
	rules, err := ParseWithdrawalFeeRules(`[{"min_amount":0,"max_amount":100,"fee_type":"fixed","fee_value":5,"enabled":true,"sort_order":1},{"min_amount":100,"max_amount":500,"fee_type":"ratio","fee_value":3,"enabled":true,"sort_order":2}]`)
	if err != nil {
		t.Fatalf("ParseWithdrawalFeeRules returned error: %v", err)
	}

	matched, feeAmount, err := calculateWithdrawalFeeAmount(decimal.NewFromInt(100), rules)
	if err != nil {
		t.Fatalf("calculateWithdrawalFeeAmount returned error: %v", err)
	}
	if matched == nil || matched.MinAmount != 0 || matched.MaxAmount != 100 {
		t.Fatalf("matched rule = %#v, want first rule for amount 100", matched)
	}
	if !feeAmount.Equal(decimal.NewFromInt(5)) {
		t.Fatalf("feeAmount = %s, want 5", feeAmount.String())
	}
}

func TestParseWithdrawalFeeRulesRejectsEmptyRanges(t *testing.T) {
	_, err := ParseWithdrawalFeeRules(`[{"min_amount":100,"max_amount":100,"fee_type":"fixed","fee_value":5,"enabled":true,"sort_order":1}]`)
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "range") {
		t.Fatalf("ParseWithdrawalFeeRules error = %v, want empty range validation", err)
	}
}
```

- [ ] **Step 2: Run the focused Go tests to verify they fail**

Run: `cd /Users/money/project/subproject/hermestoken && go test ./model -run 'Test(CalculateWithdrawalFeeAmountUsesLeftOpenRightClosedRanges|ParseWithdrawalFeeRulesRejectsEmptyRanges)'`
Expected: FAIL because amount `100` still matches the second rule under the old `[min, max)` logic and `min_amount == max_amount` is currently accepted.

- [ ] **Step 3: Update model validation and matching helpers**

```go
func normalizeWithdrawalFeeRules(rules []WithdrawalFeeRule) ([]WithdrawalFeeRule, error) {
	normalized := make([]WithdrawalFeeRule, 0, len(rules))
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if rule.MinAmount < 0 || rule.MaxAmount < 0 {
			return nil, fmt.Errorf("invalid withdrawal fee rule range")
		}
		if rule.MaxAmount > 0 && rule.MaxAmount <= rule.MinAmount {
			return nil, fmt.Errorf("invalid withdrawal fee rule range")
		}
		normalized = append(normalized, rule)
	}

	sort.SliceStable(normalized, func(i, j int) bool {
		if normalized[i].SortOrder == normalized[j].SortOrder {
			return normalized[i].MinAmount < normalized[j].MinAmount
		}
		return normalized[i].SortOrder < normalized[j].SortOrder
	})

	for i := 1; i < len(normalized); i++ {
		prev := normalized[i-1]
		current := normalized[i]
		if prev.MaxAmount == 0 {
			return nil, fmt.Errorf("withdrawal fee rules overlap")
		}
		if current.MinAmount < prev.MaxAmount {
			return nil, fmt.Errorf("withdrawal fee rules overlap")
		}
	}

	return normalized, nil
}

func matchesWithdrawalFeeRuleAmount(amount decimal.Decimal, rule WithdrawalFeeRule) bool {
	if amount.LessThanOrEqual(decimal.Zero) {
		return false
	}

	minAmount := decimal.NewFromFloat(rule.MinAmount)
	if rule.MinAmount == 0 {
		if amount.LessThanOrEqual(decimal.Zero) {
			return false
		}
	} else if !amount.GreaterThan(minAmount) {
		return false
	}

	if rule.MaxAmount > 0 && amount.GreaterThan(decimal.NewFromFloat(rule.MaxAmount)) {
		return false
	}

	return true
}

func calculateWithdrawalFeeAmount(amount decimal.Decimal, rules []WithdrawalFeeRule) (*WithdrawalFeeRule, decimal.Decimal, error) {
	for i := range rules {
		rule := rules[i]
		if !matchesWithdrawalFeeRuleAmount(amount, rule) {
			continue
		}
		feeAmount := decimal.Zero
		switch rule.FeeType {
		case WithdrawalFeeTypeFixed:
			feeAmount = decimal.NewFromFloat(rule.FeeValue)
		case WithdrawalFeeTypeRatio:
			feeAmount = amount.Mul(decimal.NewFromFloat(rule.FeeValue)).Div(decimal.NewFromInt(100))
			if rule.MinFee > 0 && feeAmount.LessThan(decimal.NewFromFloat(rule.MinFee)) {
				feeAmount = decimal.NewFromFloat(rule.MinFee)
			}
			if rule.MaxFee > 0 && feeAmount.GreaterThan(decimal.NewFromFloat(rule.MaxFee)) {
				feeAmount = decimal.NewFromFloat(rule.MaxFee)
			}
		default:
			return nil, decimal.Zero, errors.New("invalid withdrawal fee type")
		}
		return &rule, feeAmount.Round(2), nil
	}

	return nil, decimal.Zero, nil
}
```

- [ ] **Step 4: Run the targeted Go tests to verify they pass**

Run: `cd /Users/money/project/subproject/hermestoken && go test ./model -run 'Test(CalculateWithdrawalFeeAmountUsesLeftOpenRightClosedRanges|ParseWithdrawalFeeRulesRejectsEmptyRanges|ParseWithdrawalFeeRulesRejectsOverlappingRanges)'`
Expected: PASS with all three tests green.

- [ ] **Step 5: Commit the backend rule-engine changes**

```bash
cd /Users/money/project/subproject/hermestoken
git add model/user_withdrawal_setting.go model/user_withdrawal.go model/user_withdrawal_test.go
git commit -m "feat: align withdrawal fee rules with inline editor semantics"
```

### Task 4: Surface Natural-Language Rules and Block Unmatched User Withdrawals

**Files:**
- Modify: `web/src/components/topup/modals/WithdrawalApplyModal.jsx`
- Modify: `web/src/components/topup/index.jsx`
- Modify: `web/tests/wallet-withdrawal.test.mjs`
- Modify: `web/src/i18n/locales/en.json`
- Modify: `web/src/i18n/locales/zh-CN.json`
- Modify: `web/src/i18n/locales/zh-TW.json`
- Modify: `web/src/i18n/locales/ja.json`
- Modify: `web/src/i18n/locales/fr.json`
- Modify: `web/src/i18n/locales/ru.json`
- Modify: `web/src/i18n/locales/vi.json`
- Modify: `web/tests/withdrawal-locales.test.mjs`

- [ ] **Step 1: Write the failing wallet/locales assertions**

```js
test('wallet withdrawal flow blocks unmatched fee-rule amounts and shows readable rule summaries', () => {
  const source = readSource('src/components/topup/index.jsx');
  const modalSource = readSource('src/components/topup/modals/WithdrawalApplyModal.jsx');

  assert.match(source, /当前提现金额未命中任何手续费规则，请调整金额或联系管理员/);
  assert.match(modalSource, /规则说明/);
  assert.match(modalSource, /命中规则/);
  assert.match(modalSource, /100 元及以下：固定手续费 5 元/);
});

test('withdrawal locales include fee editor and unmatched rule copy', () => {
  const keys = ['规则说明', '命中规则', '当前提现金额未命中任何手续费规则，请调整金额或联系管理员'];
  for (const locale of ['en', 'zh-CN', 'zh-TW', 'ja', 'fr', 'ru', 'vi']) {
    const raw = fs.readFileSync(path.join(root, `${locale}.json`), 'utf8');
    for (const key of keys) {
      assert.match(raw, new RegExp(`"${key}"`));
    }
  }
});
```

- [ ] **Step 2: Run the wallet/locales tests to verify they fail**

Run: `cd /Users/money/project/subproject/hermestoken/web && node --test tests/wallet-withdrawal.test.mjs tests/withdrawal-locales.test.mjs`
Expected: FAIL because the unmatched-rule message and rule-summary copy do not exist yet.

- [ ] **Step 3: Update the modal, submission guard, and locale copy**

```jsx
// web/src/components/topup/modals/WithdrawalApplyModal.jsx
import {
  describeWithdrawalFeeRule,
  formatWithdrawalAmount,
} from '../../../helpers';

const readableRules = (config?.feeRules || [])
  .filter((rule) => rule?.enabled !== false)
  .map((rule) => {
    if (rule.fee_type === 'fixed') {
      return `${describeWithdrawalFeeRule(rule).replace('金额', t('金额'))}：${t('固定手续费')} ${formatWithdrawalAmount(rule.fee_value, symbol)}`;
    }
    return `${describeWithdrawalFeeRule(rule).replace('金额', t('金额'))}：${t('按')} ${rule.fee_value}% ${t('收费')}`;
  });

<div className='rounded-xl border border-[var(--semi-color-border)] p-4 space-y-3'>
  <Text strong>{t('规则说明')}</Text>
  {readableRules.map((item) => (
    <div key={item} className='text-sm text-[var(--semi-color-text-2)]'>
      {item}
    </div>
  ))}
</div>
<div className='rounded-xl border border-[var(--semi-color-border)] p-4 bg-[var(--semi-color-fill-0)] space-y-2'>
  <div className='flex justify-between items-center'>
    <Text type='tertiary'>{t('命中规则')}</Text>
    <Text>{preview?.matchedRule ? describeWithdrawalFeeRule(preview.matchedRule) : t('未命中')}</Text>
  </div>
  <div className='flex justify-between items-center'>
    <Text type='tertiary'>{t('手续费')}</Text>
    <Text>{formatWithdrawalAmount(preview?.feeAmount, symbol)}</Text>
  </div>
  <div className='flex justify-between items-center'>
    <Text type='tertiary'>{t('实际到账')}</Text>
    <Text strong>{formatWithdrawalAmount(preview?.netAmount, symbol)}</Text>
  </div>
</div>
```

```jsx
// web/src/components/topup/index.jsx
const preview = calculateWithdrawalPreview(
  withdrawalAmount,
  withdrawalConfig?.feeRules || [],
);

const submitWithdrawal = async () => {
  if (!preview?.matchedRule) {
    showError(t('当前提现金额未命中任何手续费规则，请调整金额或联系管理员'));
    return;
  }
  if (withdrawalAmount < (withdrawalConfig?.minAmount || 0)) {
    showError(t('提现金额不能低于最低提现金额'));
    return;
  }
  if (!withdrawalAlipayAccount.trim()) {
    showError(t('支付宝账号不能为空'));
    return;
  }
  if (!withdrawalAlipayRealName.trim()) {
    showError(t('支付宝姓名不能为空'));
    return;
  }

  setWithdrawalSubmitting(true);
  try {
    const res = await API.post('/api/user/withdrawals', {
      amount: Number(withdrawalAmount),
      alipay_account: withdrawalAlipayAccount.trim(),
      alipay_real_name: withdrawalAlipayRealName.trim(),
    });
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('提现申请已提交'));
      setOpenWithdrawalApply(false);
      await Promise.all([loadSelf(), loadWithdrawalConfig()]);
      return;
    }
    showError(message || t('提现申请失败'));
  } catch (error) {
    showError(t('提现申请失败'));
  } finally {
    setWithdrawalSubmitting(false);
  }
};
```

- [ ] **Step 4: Run the wallet/locales tests to verify they pass**

Run: `cd /Users/money/project/subproject/hermestoken/web && node --test tests/wallet-withdrawal.test.mjs tests/withdrawal-locales.test.mjs tests/withdrawal-settings.test.mjs tests/withdrawal-fee-rule-editor.test.mjs`
Expected: PASS with all four tests green; the wallet test should now find the unmatched-rule guard and readable rule section.

- [ ] **Step 5: Commit the user-facing fee-preview updates**

```bash
cd /Users/money/project/subproject/hermestoken
git add \
  web/src/components/topup/modals/WithdrawalApplyModal.jsx \
  web/src/components/topup/index.jsx \
  web/tests/wallet-withdrawal.test.mjs \
  web/src/i18n/locales/en.json \
  web/src/i18n/locales/zh-CN.json \
  web/src/i18n/locales/zh-TW.json \
  web/src/i18n/locales/ja.json \
  web/src/i18n/locales/fr.json \
  web/src/i18n/locales/ru.json \
  web/src/i18n/locales/vi.json \
  web/tests/withdrawal-locales.test.mjs
git commit -m "feat: improve withdrawal fee rule messaging"
```

### Task 5: Run End-to-End Regression for the Whole Fee Rule Rewrite

**Files:**
- Modify: `web/src/pages/Setting/Payment/SettingsWithdrawal.jsx`
- Modify: `web/src/helpers/withdrawal.js`
- Modify: `model/user_withdrawal.go`
- Modify: `model/user_withdrawal_setting.go`
- Modify: `web/src/components/topup/modals/WithdrawalApplyModal.jsx`
- Modify: `web/src/components/topup/index.jsx`

- [ ] **Step 1: Add the final focused regression checks before shipping**

```go
func TestCreateUserWithdrawalRejectsAmountWithoutMatchingFeeRule(t *testing.T) {
	db := setupWithdrawalModelDB(t)
	originalQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 100
	t.Cleanup(func() { common.QuotaPerUnit = originalQuotaPerUnit })

	user := seedWithdrawalUser(t, db, "withdraw-gap-user", 100000)
	common.OptionMap = map[string]string{
		WithdrawalEnabledOptionKey:     "true",
		WithdrawalMinAmountOptionKey:   "10",
		WithdrawalInstructionOptionKey: "manual payout",
		WithdrawalFeeRulesOptionKey:    `[{"min_amount":0,"max_amount":100,"fee_type":"fixed","fee_value":5,"enabled":true,"sort_order":1},{"min_amount":200,"max_amount":0,"fee_type":"ratio","fee_value":1,"enabled":true,"sort_order":2}]`,
	}

	_, err := CreateUserWithdrawal(&CreateUserWithdrawalParams{
		UserID:         user.Id,
		Amount:         150,
		AlipayAccount:  "gap@example.com",
		AlipayRealName: "Gap User",
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "match") {
		t.Fatalf("CreateUserWithdrawal error = %v, want no-matching-rule failure", err)
	}
}
```

- [ ] **Step 2: Run the combined regression suite and verify the new test fails first**

Run: `cd /Users/money/project/subproject/hermestoken && go test ./model -run 'Test(CreateUserWithdrawalRejectsAmountWithoutMatchingFeeRule|CalculateWithdrawalFeeAmountUsesLeftOpenRightClosedRanges)' && cd web && node --test tests/withdrawal-fee-rule-editor.test.mjs tests/withdrawal-settings.test.mjs tests/wallet-withdrawal.test.mjs tests/withdrawal-locales.test.mjs`
Expected: the Go test fails first until `CreateUserWithdrawal` returns a “no matching rule” error for gap amounts.

- [ ] **Step 3: Finish the remaining implementation glue**

```go
func CreateUserWithdrawal(params *CreateUserWithdrawalParams) (*UserWithdrawal, error) {
	if params == nil {
		return nil, errors.New("withdrawal params are required")
	}

	setting := GetUserWithdrawalSetting()
	if !setting.Enabled {
		return nil, errors.New("withdrawal is disabled")
	}

	amount := decimal.NewFromFloat(params.Amount).Round(2)
	if amount.LessThanOrEqual(decimal.Zero) {
		return nil, errors.New("invalid withdrawal amount")
	}
	if amount.LessThan(decimal.NewFromFloat(setting.MinAmount)) {
		return nil, fmt.Errorf("withdrawal amount must be at least %.2f", setting.MinAmount)
	}

	currency := GetUserWithdrawalCurrencyConfig()
	applyQuota, err := currencyAmountToQuota(amount.InexactFloat64(), currency)
	if err != nil {
		return nil, err
	}
	if applyQuota <= 0 {
		return nil, errors.New("invalid withdrawal quota")
	}

	matchedRule, feeAmount, err := calculateWithdrawalFeeAmount(amount, setting.FeeRules)
	if err != nil {
		return nil, err
	}
	if matchedRule == nil {
		return nil, errors.New("withdrawal amount does not match any fee rule")
	}
	netAmount := amount.Sub(feeAmount).Round(2)
	if !netAmount.GreaterThan(decimal.Zero) {
		return nil, errors.New("net withdrawal amount must be greater than zero")
	}

	feeQuota, err := currencyAmountToQuota(feeAmount.InexactFloat64(), currency)
	if err != nil {
		return nil, err
	}
	netQuota, err := currencyAmountToQuota(netAmount.InexactFloat64(), currency)
	if err != nil {
		return nil, err
	}

	var created UserWithdrawal
	err = DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.Set("gorm:query_option", "FOR UPDATE").First(&user, params.UserID).Error; err != nil {
			return err
		}
		if user.Quota < applyQuota {
			return errors.New("insufficient wallet balance")
		}

		tradeNo, err := generateUserWithdrawalTradeNo(tx)
		if err != nil {
			return err
		}
		ruleSnapshotJSON := ""
		if matchedRule != nil {
			ruleSnapshotJSON = common.GetJsonString(matchedRule)
		}

		created = UserWithdrawal{
			UserId:              params.UserID,
			TradeNo:             tradeNo,
			Channel:             WithdrawalChannelAlipay,
			Currency:            currency.Currency,
			ApplyAmount:         amount.InexactFloat64(),
			FeeAmount:           feeAmount.InexactFloat64(),
			NetAmount:           netAmount.InexactFloat64(),
			ApplyQuota:          applyQuota,
			FeeQuota:            feeQuota,
			NetQuota:            netQuota,
			AlipayAccount:       strings.TrimSpace(params.AlipayAccount),
			AlipayRealName:      strings.TrimSpace(params.AlipayRealName),
			Status:              UserWithdrawalStatusPending,
			FeeRuleSnapshotJSON: ruleSnapshotJSON,
		}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}

		return tx.Model(&User{}).Where("id = ?", params.UserID).Updates(map[string]any{
			"quota":                 gorm.Expr("quota - ?", applyQuota),
			"withdraw_frozen_quota": gorm.Expr("withdraw_frozen_quota + ?", applyQuota),
		}).Error
	})
	if err != nil {
		return nil, err
	}
	return &created, nil
}
```

```jsx
// web/src/pages/Setting/Payment/SettingsWithdrawal.jsx
const [inputs, setInputs] = useState({
  WithdrawalEnabled: false,
  WithdrawalMinAmount: 10,
  WithdrawalInstruction: '',
  WithdrawalFeeRules: [],
});

const submit = async () => {
  const feedback = validateWithdrawalFeeEditorRules(inputs.WithdrawalFeeRules);
  if (feedback.errors.length > 0) {
    showError(feedback.errors[0]);
    return;
  }

  const requestQueue = [
    API.put('/api/option/', {
      key: 'WithdrawalFeeRules',
      value: serializeWithdrawalFeeEditorRules(inputs.WithdrawalFeeRules),
    }),
  ];
  await Promise.all(requestQueue);
};
```

- [ ] **Step 4: Run the full regression suite to verify everything passes**

Run: `cd /Users/money/project/subproject/hermestoken && go test ./model -run 'Test(CreateUserWithdrawalRejectsAmountWithoutMatchingFeeRule|CalculateWithdrawalFeeAmountUsesLeftOpenRightClosedRanges|ParseWithdrawalFeeRulesRejectsEmptyRanges|ParseWithdrawalFeeRulesRejectsOverlappingRanges)' && cd web && node --test tests/withdrawal-fee-rule-editor.test.mjs tests/withdrawal-settings.test.mjs tests/wallet-withdrawal.test.mjs tests/withdrawal-locales.test.mjs`
Expected: PASS for the focused Go suite and PASS for all four frontend source-test files.

- [ ] **Step 5: Commit the final integration pass**

```bash
cd /Users/money/project/subproject/hermestoken
git add \
  web/src/pages/Setting/Payment/SettingsWithdrawal.jsx \
  web/src/helpers/withdrawal.js \
  model/user_withdrawal.go \
  model/user_withdrawal_setting.go \
  model/user_withdrawal_test.go \
  web/src/components/topup/modals/WithdrawalApplyModal.jsx \
  web/src/components/topup/index.jsx
git commit -m "feat: finish withdrawal fee rule editor rollout"
```
