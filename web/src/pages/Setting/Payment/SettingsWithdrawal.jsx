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
import { Button, Form, Spin } from '@douyinfe/semi-ui';
import {
  API,
  showError,
  showSuccess,
  toBoolean,
  verifyJSON,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';

export default function SettingsWithdrawal({ options, refresh }) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState({
    WithdrawalEnabled: false,
    WithdrawalMinAmount: 10,
    WithdrawalInstruction: '',
    WithdrawalFeeRules: '[]',
  });
  const formApiRef = useRef(null);

  useEffect(() => {
    if (options && formApiRef.current) {
      const currentInputs = {
        WithdrawalEnabled: toBoolean(options.WithdrawalEnabled),
        WithdrawalMinAmount:
          options.WithdrawalMinAmount !== undefined
            ? parseFloat(options.WithdrawalMinAmount || 10)
            : 10,
        WithdrawalInstruction: options.WithdrawalInstruction || '',
        WithdrawalFeeRules: (() => {
          const raw = options.WithdrawalFeeRules || '[]';
          try {
            return JSON.stringify(JSON.parse(raw), null, 2);
          } catch {
            return raw;
          }
        })(),
      };
      setInputs(currentInputs);
      formApiRef.current.setValues(currentInputs);
    }
  }, [options]);

  const handleFormChange = (values) => {
    setInputs(values);
  };

  const submit = async () => {
    if (!verifyJSON(inputs.WithdrawalFeeRules || '[]')) {
      showError(t('提现手续费规则不是合法的 JSON'));
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
          value: JSON.stringify(JSON.parse(inputs.WithdrawalFeeRules || '[]')),
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
          <Form.TextArea
            field='WithdrawalFeeRules'
            label={t('提现手续费规则')}
            autosize={{ minRows: 8 }}
            extraText={
              <div className='space-y-1 text-xs leading-6 text-[var(--semi-color-text-2)]'>
                <div>
                  {t(
                    '规则为 JSON 数组；系统会按 sort_order 从小到大，匹配第一条 enabled=true 且金额区间命中的规则。',
                  )}
                </div>
                <div>
                  {t('min_amount')}: {t('本条规则的最小提现金额，包含当前值。')}
                </div>
                <div>
                  {t('max_amount')}:{' '}
                  {t('本条规则的最大提现金额；填 0 表示无上限；大于 0 时不包含当前值。')}
                </div>
                <div>
                  {t('fee_type')}:{' '}
                  {t(
                    'fixed 表示固定手续费；ratio 表示按提现金额百分比计算手续费。',
                  )}
                </div>
                <div>
                  {t('fee_value')}:{' '}
                  {t(
                    'fixed 时表示直接收多少钱；ratio 时表示百分比，例如 2 表示 2%。',
                  )}
                </div>
                <div>
                  {t('min_fee')}:{' '}
                  {t('仅在 ratio 下生效；按比例算出的手续费低于它时，按这个最小值收。')}
                </div>
                <div>
                  {t('max_fee')}:{' '}
                  {t(
                    '仅在 ratio 下生效；按比例算出的手续费高于它时，按这个最大值收；填 0 表示不设上限。',
                  )}
                </div>
                <div>
                  {t('enabled')}: {t('true 表示启用，false 表示停用。')}
                </div>
                <div>
                  {t('sort_order')}: {t('匹配顺序，值越小越先匹配。')}
                </div>
                <div>
                  {t(
                    '示例：{\"min_amount\":50,\"max_amount\":0,\"fee_type\":\"ratio\",\"fee_value\":2,\"min_fee\":1,\"max_fee\":10,\"enabled\":true,\"sort_order\":2} 表示 50 及以上按 2% 收费，最低 1，最高 10。',
                  )}
                </div>
              </div>
            }
          />
          <Button onClick={submit}>{t('保存提现设置')}</Button>
        </Form.Section>
      </Form>
    </Spin>
  );
}
