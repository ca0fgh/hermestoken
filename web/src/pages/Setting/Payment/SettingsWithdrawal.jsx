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

import React, { useEffect, useRef, useState } from 'react';
import { Banner, Button, Form, Spin, Typography } from '@douyinfe/semi-ui';
import {
  API,
  showError,
  showSuccess,
  toBoolean,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';
import WithdrawalFeeRulesEditor from '../../../components/settings/withdrawal/WithdrawalFeeRulesEditor';
import {
  normalizeWithdrawalFeeEditorRules,
  parsePersistedWithdrawalFeeRules,
  serializeWithdrawalFeeEditorRules,
  validateWithdrawalFeeEditorRules,
} from '../../../helpers/withdrawal';

const { Text } = Typography;

export default function SettingsWithdrawal({ options, refresh }) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [withdrawalFeeRulesInvalidState, setWithdrawalFeeRulesInvalidState] =
    useState(null);
  const [
    hasReplacedInvalidWithdrawalFeeRules,
    setHasReplacedInvalidWithdrawalFeeRules,
  ] = useState(false);
  const [inputs, setInputs] = useState({
    WithdrawalEnabled: false,
    WithdrawalMinAmount: 10,
    WithdrawalInstruction: '',
    WithdrawalFeeRules: [],
  });
  const formApiRef = useRef(null);

  useEffect(() => {
    if (options && formApiRef.current) {
      const parsedFeeRules = parsePersistedWithdrawalFeeRules(
        options.WithdrawalFeeRules || '[]',
      );
      const currentInputs = {
        WithdrawalEnabled: toBoolean(options.WithdrawalEnabled),
        WithdrawalMinAmount:
          options.WithdrawalMinAmount !== undefined
            ? parseFloat(options.WithdrawalMinAmount || 10)
            : 10,
        WithdrawalInstruction: options.WithdrawalInstruction || '',
        WithdrawalFeeRules: parsedFeeRules.rules,
      };
      setWithdrawalFeeRulesInvalidState(
        parsedFeeRules.errors.length > 0
          ? {
              rawValue: parsedFeeRules.rawValue,
              errors: parsedFeeRules.errors,
            }
          : null,
      );
      setHasReplacedInvalidWithdrawalFeeRules(false);
      setInputs(currentInputs);
      formApiRef.current.setValues(currentInputs);
    }
  }, [options]);

  const handleFormChange = (values) => {
    setInputs((current) => ({
      ...current,
      ...values,
    }));
  };

  const handleFeeRulesChange = (rules) => {
    if (withdrawalFeeRulesInvalidState) {
      setHasReplacedInvalidWithdrawalFeeRules(true);
      setWithdrawalFeeRulesInvalidState(null);
    }

    setInputs((current) => ({
      ...current,
      WithdrawalFeeRules: normalizeWithdrawalFeeEditorRules(rules),
    }));
  };

  const submit = async () => {
    if (
      withdrawalFeeRulesInvalidState &&
      !hasReplacedInvalidWithdrawalFeeRules
    ) {
      showError(t('提现手续费规则配置已损坏，请先修复或替换后再保存。'));
      return;
    }

    const validation = validateWithdrawalFeeEditorRules(
      inputs.WithdrawalFeeRules,
      t,
    );
    if (validation.errors.length > 0) {
      showError(validation.errors[0]);
      return;
    }

    setLoading(true);
    try {
      const requestQueue = [
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
          value:
            withdrawalFeeRulesInvalidState?.rawValue ??
            serializeWithdrawalFeeEditorRules(inputs.WithdrawalFeeRules),
        }),
      ];

      const results = await Promise.all(requestQueue);
      const errorResult = results.find((res) => !res?.data?.success);
      if (errorResult) {
        showError(errorResult.data.message || t('更新失败'));
        return;
      }

      showSuccess(t('更新成功'));
      refresh && refresh();
    } catch (error) {
      showError(t('更新失败'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Spin spinning={loading}>
      <Form
        initValues={inputs}
        onValueChange={handleFormChange}
        getFormApi={(api) => (formApiRef.current = api)}
      >
        <Form.Section text={t('提现设置')}>
          <Form.Switch
            field='WithdrawalEnabled'
            label={t('启用提现')}
            extraText={t('开启后，用户可在钱包管理中发起提现申请')}
          />
          <Form.InputNumber
            field='WithdrawalMinAmount'
            label={t('最低提现金额')}
            min={0}
            style={{ width: '100%' }}
            extraText={t('按当前提现币种金额计算')}
          />
          <Form.TextArea
            field='WithdrawalInstruction'
            label={t('提现说明')}
            autosize={{ minRows: 3 }}
            extraText={t('展示在用户提现弹窗中，例如到账时效与线下处理说明')}
          />

          <div style={{ marginBottom: 24 }}>
            <Text strong>{t('提现手续费规则')}</Text>
            <Text type='tertiary' style={{ display: 'block', marginTop: 4 }}>
              {t('使用可视化编辑器维护区间、收费方式和预览结果，保存时会自动转换为系统配置格式。')}
            </Text>
            {withdrawalFeeRulesInvalidState && (
              <Banner
                type='danger'
                closeIcon={null}
                style={{ marginTop: 12 }}
                description={t('检测到已保存的提现手续费规则配置无效。当前不会自动覆盖原始配置；请修复规则后重新保存，或恢复默认示例并重新配置。')}
              />
            )}
            {/* invalid persisted WithdrawalFeeRules before normalizing into editor state */}
            <WithdrawalFeeRulesEditor
              value={inputs.WithdrawalFeeRules}
              onChange={handleFeeRulesChange}
            />
          </div>

          <Button onClick={submit}>{t('保存提现设置')}</Button>
        </Form.Section>
      </Form>
    </Spin>
  );
}
