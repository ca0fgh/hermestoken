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

import React, { useEffect, useMemo, useState } from 'react';
import { Button, Input, InputNumber, Select, Switch, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { getWithdrawalFeeTypeLabel } from '../../../helpers/withdrawal';

const { Text } = Typography;

const parseRequiredNumber = (value, fallback = 0) => {
  const numericValue = Number(value);
  return Number.isFinite(numericValue) ? numericValue : fallback;
};

const isOptionalNumberBlank = (value) =>
  value === '' || value === null || value === undefined;

const parseOptionalNumber = (value) => {
  if (isOptionalNumberBlank(value)) {
    return '';
  }

  const textValue = String(value).trim();
  if (textValue === '') {
    return '';
  }

  const numericValue = Number(textValue);
  return Number.isFinite(numericValue) ? numericValue : null;
};

const buildDraftFromRule = (rule) => ({
  id: rule?.id || '',
  minAmount: parseRequiredNumber(rule?.minAmount, 0),
  maxAmount:
    rule?.maxAmount === '' || rule?.maxAmount === null || rule?.maxAmount === undefined
      ? ''
      : String(rule.maxAmount),
  feeType: rule?.feeType === 'ratio' ? 'ratio' : 'fixed',
  feeValue: parseRequiredNumber(rule?.feeValue, 0),
  minFee: parseRequiredNumber(rule?.minFee, 0),
  maxFee:
    rule?.maxFee === '' || rule?.maxFee === null || rule?.maxFee === undefined
      ? ''
      : String(rule.maxFee),
  enabled: rule?.enabled !== false,
  sortOrder: parseRequiredNumber(rule?.sortOrder, 1),
});

export default function WithdrawalFeeRuleInlineForm({
  rule,
  onCancel,
  onDraftChange,
  onSave,
  saveText,
}) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState(() => buildDraftFromRule(rule));

  useEffect(() => {
    setDraft(buildDraftFromRule(rule));
  }, [rule]);

  const updateDraft = (updater) => {
    setDraft((current) => {
      const nextDraft =
        typeof updater === 'function' ? updater(current) : updater;
      onDraftChange?.(nextDraft);
      return nextDraft;
    });
  };

  const helperText = useMemo(() => {
    if (draft.feeType === 'ratio') {
      return t('费率按百分比填写，例如 2 表示按提现金额的 2% 收费。');
    }

    return t('固定金额表示每笔提现直接收取的手续费。');
  }, [draft.feeType, t]);

  const optionalFieldErrors = useMemo(() => {
    const errors = {};

    if (parseOptionalNumber(draft.maxAmount) === null) {
      errors.maxAmount = t('结束金额必须是有效数字');
    }

    if (
      draft.feeType === 'ratio' &&
      parseOptionalNumber(draft.maxFee) === null
    ) {
      errors.maxFee = t('最高手续费必须是有效数字');
    }

    return errors;
  }, [draft.feeType, draft.maxAmount, draft.maxFee, t]);

  const hasOptionalFieldErrors = Object.keys(optionalFieldErrors).length > 0;

  const handleSave = () => {
    if (hasOptionalFieldErrors) {
      return;
    }

    onSave({
      id: draft.id,
      minAmount: parseRequiredNumber(draft.minAmount, 0),
      maxAmount: parseOptionalNumber(draft.maxAmount),
      feeType: draft.feeType,
      feeValue: parseRequiredNumber(draft.feeValue, 0),
      minFee: draft.feeType === 'ratio' ? parseRequiredNumber(draft.minFee, 0) : 0,
      maxFee: draft.feeType === 'ratio' ? parseOptionalNumber(draft.maxFee) : '',
      enabled: draft.enabled,
      sortOrder: parseRequiredNumber(draft.sortOrder, 1),
    });
  };

  return (
    <div
      style={{
        border: '1px solid var(--semi-color-border)',
        borderRadius: 12,
        padding: 16,
        background: 'var(--semi-color-fill-0)',
      }}
    >
      <div
        style={{
          display: 'grid',
          gap: 12,
          gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
        }}
      >
        <div>
          <Text strong>{t('起始金额')}</Text>
          <InputNumber
            min={0}
            style={{ width: '100%', marginTop: 6 }}
            value={draft.minAmount}
            onChange={(value) =>
              updateDraft((current) => ({
                ...current,
                minAmount: parseRequiredNumber(value, 0),
              }))
            }
          />
        </div>
        <div>
          <Text strong>{t('结束金额')}</Text>
          <Input
            style={{ width: '100%', marginTop: 6 }}
            value={draft.maxAmount}
            onChange={(value) =>
              updateDraft((current) => ({
                ...current,
                maxAmount: value,
              }))
            }
            placeholder={t('留空表示无上限')}
          />
          {optionalFieldErrors.maxAmount && (
            <Text
              type='danger'
              size='small'
              style={{ display: 'block', marginTop: 6 }}
            >
              {optionalFieldErrors.maxAmount}
            </Text>
          )}
        </div>
        <div>
          <Text strong>{t('收费方式')}</Text>
          <Select
            style={{ width: '100%', marginTop: 6 }}
            value={draft.feeType}
            onChange={(value) =>
              updateDraft((current) => ({
                ...current,
                feeType: value,
              }))
            }
          >
            <Select.Option value='fixed'>
              {getWithdrawalFeeTypeLabel('fixed', t)}
            </Select.Option>
            <Select.Option value='ratio'>
              {getWithdrawalFeeTypeLabel('ratio', t)}
            </Select.Option>
          </Select>
        </div>
        <div>
          <Text strong>{draft.feeType === 'ratio' ? t('费率') : t('固定金额')}</Text>
          <InputNumber
            min={0}
            max={draft.feeType === 'ratio' ? 100 : undefined}
            style={{ width: '100%', marginTop: 6 }}
            value={draft.feeValue}
            onChange={(value) =>
              updateDraft((current) => ({
                ...current,
                feeValue: parseRequiredNumber(value, 0),
              }))
            }
          />
          <Text type='tertiary' size='small' style={{ display: 'block', marginTop: 6 }}>
            {helperText}
          </Text>
        </div>
        {draft.feeType === 'ratio' && (
          <>
            <div>
              <Text strong>{t('最低手续费')}</Text>
              <InputNumber
                min={0}
                style={{ width: '100%', marginTop: 6 }}
                value={draft.minFee}
                onChange={(value) =>
                  updateDraft((current) => ({
                    ...current,
                    minFee: parseRequiredNumber(value, 0),
                  }))
                }
              />
            </div>
            <div>
              <Text strong>{t('最高手续费')}</Text>
              <Input
                style={{ width: '100%', marginTop: 6 }}
                value={draft.maxFee}
                onChange={(value) =>
                  updateDraft((current) => ({
                    ...current,
                    maxFee: value,
                  }))
                }
                placeholder={t('留空表示不设上限')}
              />
              {optionalFieldErrors.maxFee && (
                <Text
                  type='danger'
                  size='small'
                  style={{ display: 'block', marginTop: 6 }}
                >
                  {optionalFieldErrors.maxFee}
                </Text>
              )}
            </div>
          </>
        )}
        <div>
          <Text strong>{t('启用状态')}</Text>
          <div style={{ marginTop: 10 }}>
            <Switch
              checked={draft.enabled}
              onChange={(checked) =>
                updateDraft((current) => ({
                  ...current,
                  enabled: checked,
                }))
              }
            />
            <Text type='tertiary' size='small' style={{ marginLeft: 8 }}>
              {draft.enabled ? t('当前规则已启用') : t('当前规则已停用')}
            </Text>
          </div>
        </div>
      </div>

      <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 16 }}>
        <Button onClick={onCancel}>{t('取消')}</Button>
        <Button
          theme='solid'
          onClick={handleSave}
          disabled={hasOptionalFieldErrors}
        >
          {saveText || t('保存')}
        </Button>
      </div>
    </div>
  );
}
