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
import { Banner, Button, Tag, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import WithdrawalFeeRuleInlineForm from './WithdrawalFeeRuleInlineForm';
import {
  describeWithdrawalFeeRule,
  getWithdrawalFeeTypeLabel,
  normalizeWithdrawalFeeEditorRules,
  reindexWithdrawalFeeEditorRules,
  validateWithdrawalFeeEditorRules,
} from '../../../helpers/withdrawal';

const { Text } = Typography;

const createLocalRuleId = () =>
  `withdrawal-fee-rule-${Date.now()}-${Math.random().toString(16).slice(2, 8)}`;

const createBlankRule = (sortOrder) => ({
  id: createLocalRuleId(),
  minAmount: 0,
  maxAmount: '',
  feeType: 'fixed',
  feeValue: 0,
  minFee: 0,
  maxFee: '',
  enabled: true,
  sortOrder,
});

const normalizeRuleForCompare = (rule) =>
  JSON.stringify(
    normalizeWithdrawalFeeEditorRules(rule ? [rule] : []).map(
      ({ id, ...normalizedRule }) => normalizedRule,
    ),
  );

export default function WithdrawalFeeRulesEditor({
  value,
  onChange,
  onDraftStateChange,
}) {
  const { t } = useTranslation();
  const [editingRuleId, setEditingRuleId] = useState('');
  const [draftRule, setDraftRule] = useState(null);
  const [draftBaseline, setDraftBaseline] = useState(null);

  const rules = useMemo(
    () => normalizeWithdrawalFeeEditorRules(value),
    [value],
  );
  const feedback = useMemo(
    () => validateWithdrawalFeeEditorRules(rules, t),
    [rules, t],
  );
  const hasDraftChanges = useMemo(() => {
    if (!editingRuleId || !draftRule || !draftBaseline) {
      return false;
    }

    return (
      normalizeRuleForCompare(draftRule) !==
      normalizeRuleForCompare(draftBaseline)
    );
  }, [draftBaseline, draftRule, editingRuleId]);

  useEffect(() => {
    onDraftStateChange?.(Boolean(editingRuleId));
  }, [editingRuleId, onDraftStateChange]);

  const commitRules = (nextRules) => {
    onChange?.(reindexWithdrawalFeeEditorRules(nextRules));
  };

  const startDraft = (nextEditingRuleId, nextDraftRule) => {
    setEditingRuleId(nextEditingRuleId);
    setDraftRule(nextDraftRule);
    setDraftBaseline(nextDraftRule);
  };

  const clearDraft = () => {
    setEditingRuleId('');
    setDraftRule(null);
    setDraftBaseline(null);
  };

  const handleDraftReplacement = (replaceDraft) => {
    if (
      hasDraftChanges &&
      !window.confirm(t('当前有未保存的规则修改，确定要放弃并继续吗？'))
    ) {
      return;
    }

    replaceDraft();
  };

  const handleAddRule = () => {
    handleDraftReplacement(() => {
      startDraft('new', createBlankRule(rules.length + 1));
    });
  };

  const handleEditRule = (rule) => {
    if (editingRuleId === rule.id) {
      return;
    }

    handleDraftReplacement(() => {
      startDraft(rule.id, rule);
    });
  };

  const handleSaveRule = (nextRule) => {
    const nextRules =
      editingRuleId === 'new'
        ? [...rules, nextRule]
        : rules.map((rule) => (rule.id === editingRuleId ? nextRule : rule));

    commitRules(nextRules);
    clearDraft();
  };

  const handleCancelEdit = () => {
    clearDraft();
  };

  const handleDeleteRule = (ruleId) => {
    commitRules(rules.filter((rule) => rule.id !== ruleId));
    if (editingRuleId === ruleId) {
      handleCancelEdit();
    }
  };

  const handleMoveRule = (index, offset) => {
    const targetIndex = index + offset;
    if (targetIndex < 0 || targetIndex >= rules.length) {
      return;
    }

    const nextRules = [...rules];
    const [movedRule] = nextRules.splice(index, 1);
    nextRules.splice(targetIndex, 0, movedRule);
    commitRules(nextRules);
  };

  return (
    <div style={{ marginTop: 8 }}>
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          gap: 12,
          flexWrap: 'wrap',
          marginBottom: 12,
        }}
      >
        <div>
          <Text type='tertiary' style={{ display: 'block' }}>
            {t('按列表顺序匹配金额区间，系统会使用第一条启用且命中的规则。')}
          </Text>
        </div>
        <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          <Button onClick={handleAddRule}>{t('新增规则')}</Button>
        </div>
      </div>

      {feedback.errors.length > 0 && (
        <Banner
          type='danger'
          closeIcon={null}
          description={
            <div>
              {feedback.errors.map((message) => (
                <div key={message}>{message}</div>
              ))}
            </div>
          }
          style={{ marginBottom: 12 }}
        />
      )}

      {feedback.warnings.length > 0 && (
        <Banner
          type='warning'
          closeIcon={null}
          description={
            <div>
              {feedback.warnings.map((message) => (
                <div key={message}>{message}</div>
              ))}
            </div>
          }
          style={{ marginBottom: 12 }}
        />
      )}

      <div
        style={{
          border: '1px solid var(--semi-color-border)',
          borderRadius: 12,
          overflow: 'hidden',
        }}
      >
        <div style={{ overflowX: 'auto' }}>
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: '96px minmax(0, 1fr) 280px',
              gap: 12,
              padding: '12px 16px',
              background: 'var(--semi-color-fill-0)',
              borderBottom: '1px solid var(--semi-color-border)',
              fontSize: 12,
              color: 'var(--semi-color-text-2)',
            }}
          >
            <div>{t('顺序')}</div>
            <div>{t('规则概览')}</div>
            <div>{t('操作')}</div>
          </div>

          {rules.length === 0 && editingRuleId !== 'new' && (
            <div style={{ padding: 20 }}>
              <Text type='tertiary'>
                {t('暂未配置提现手续费规则，可点击“新增规则”。')}
              </Text>
            </div>
          )}

          {editingRuleId === 'new' && draftRule && (
            <div
              style={{
                padding: 16,
                borderBottom:
                  rules.length > 0
                    ? '1px solid var(--semi-color-border)'
                    : 'none',
              }}
            >
              <Text strong style={{ display: 'block', marginBottom: 12 }}>
                {t('新增规则')}
              </Text>
              <WithdrawalFeeRuleInlineForm
                rule={draftRule}
                onDraftChange={setDraftRule}
                onCancel={handleCancelEdit}
                onSave={handleSaveRule}
                saveText={t('保存')}
              />
            </div>
          )}

          {rules.map((rule, index) => {
            const isEditing = editingRuleId === rule.id;

            return (
              <div
                key={rule.id}
                style={{
                  padding: 16,
                  borderBottom:
                    index === rules.length - 1
                      ? 'none'
                      : '1px solid var(--semi-color-border)',
                  background: isEditing
                    ? 'var(--semi-color-fill-0)'
                    : 'transparent',
                }}
              >
                <div
                  style={{
                    display: 'grid',
                    gridTemplateColumns: '96px minmax(0, 1fr) 280px',
                    gap: 12,
                    alignItems: 'start',
                  }}
                >
                  <div>
                    <Tag color='white'>{`#${index + 1}`}</Tag>
                  </div>
                  <div>
                    <Text strong>{describeWithdrawalFeeRule(rule, t)}</Text>
                    <div
                      style={{
                        marginTop: 8,
                        display: 'flex',
                        gap: 8,
                        flexWrap: 'wrap',
                      }}
                    >
                      <Tag color={rule.enabled ? 'green' : 'grey'}>
                        {rule.enabled ? t('已启用') : t('已停用')}
                      </Tag>
                      <Tag
                        color={
                          rule.feeType === 'ratio' ? 'light-blue' : 'orange'
                        }
                      >
                        {getWithdrawalFeeTypeLabel(rule.feeType, t)}
                      </Tag>
                    </div>
                  </div>
                  <div
                    style={{
                      display: 'flex',
                      gap: 8,
                      flexWrap: 'wrap',
                      justifyContent: 'flex-end',
                    }}
                  >
                    <Button
                      size='small'
                      disabled={index === 0}
                      onClick={() => handleMoveRule(index, -1)}
                    >
                      {t('上移')}
                    </Button>
                    <Button
                      size='small'
                      disabled={index === rules.length - 1}
                      onClick={() => handleMoveRule(index, 1)}
                    >
                      {t('下移')}
                    </Button>
                    <Button size='small' onClick={() => handleEditRule(rule)}>
                      {t('编辑')}
                    </Button>
                    <Button
                      size='small'
                      type='danger'
                      onClick={() => handleDeleteRule(rule.id)}
                    >
                      {t('删除')}
                    </Button>
                  </div>
                </div>

                {isEditing && draftRule && (
                  <div style={{ marginTop: 16 }}>
                    <WithdrawalFeeRuleInlineForm
                      rule={draftRule}
                      onDraftChange={setDraftRule}
                      onCancel={handleCancelEdit}
                      onSave={handleSaveRule}
                      saveText={t('保存')}
                    />
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
