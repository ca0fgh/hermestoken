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

import React from 'react';
import { Modal, Typography, Input, InputNumber, Tag } from '@douyinfe/semi-ui';
import { HandCoins } from 'lucide-react';
import {
  buildWithdrawalFeeRuleDescriptions,
  describeWithdrawalFeeRuleForUser,
  formatWithdrawalAmount,
} from '../../../helpers';

const { Text } = Typography;

const WithdrawalApplyModal = ({
  t,
  visible,
  onCancel,
  onSubmit,
  submitting,
  config,
  amount,
  setAmount,
  alipayAccount,
  setAlipayAccount,
  alipayRealName,
  setAlipayRealName,
  preview,
}) => {
  const symbol = config?.currencySymbol || '¥';
  const ruleDescriptionOptions = {
    currency: config?.currency,
    currencySymbol: symbol,
  };
  const feeRuleDescriptions = buildWithdrawalFeeRuleDescriptions(
    config?.feeRules || [],
    t,
    ruleDescriptionOptions,
  );
  const matchedRuleText = preview?.matchedRule
    ? describeWithdrawalFeeRuleForUser(preview.matchedRule, t, ruleDescriptionOptions)
    : t('未命中任何手续费规则');
  const blockMessage = preview?.isValid
    ? ''
    : t(
        preview?.blockReason ||
          '当前提现金额未命中任何手续费规则，请调整金额或联系管理员',
      );

  return (
    <Modal
      title={
        <div className='flex items-center gap-2'>
          <HandCoins size={18} />
          <span>{t('申请提现')}</span>
        </div>
      }
      visible={visible}
      onCancel={onCancel}
      onOk={onSubmit}
      confirmLoading={submitting}
      maskClosable={false}
      centered
    >
      <div className='space-y-4'>
        <div>
          <Text strong className='block mb-2'>
            {t('提现金额')}
          </Text>
          <InputNumber
            min={config?.minAmount || 0}
            max={config?.availableAmount || 0}
            value={amount}
            onChange={(value) => setAmount(value || 0)}
            style={{ width: '100%' }}
          />
        </div>

        <div>
          <Text strong className='block mb-2'>
            {t('支付宝账号')}
          </Text>
          <Input
            value={alipayAccount}
            onChange={setAlipayAccount}
            placeholder={t('请输入支付宝账号')}
          />
        </div>

        <div>
          <Text strong className='block mb-2'>
            {t('支付宝姓名')}
          </Text>
          <Input
            value={alipayRealName}
            onChange={setAlipayRealName}
            placeholder={t('请输入支付宝姓名')}
          />
        </div>

        <div className='rounded-xl border border-[var(--semi-color-border)] p-4 bg-[var(--semi-color-fill-0)] space-y-3'>
          <Text strong>{t('规则说明')}</Text>
          {feeRuleDescriptions.length > 0 ? (
            <div className='space-y-2'>
              {feeRuleDescriptions.map((description) => (
                <div
                  key={description}
                  className='text-sm text-[var(--semi-color-text-1)]'
                >
                  {description}
                </div>
              ))}
            </div>
          ) : (
            <Text type='tertiary'>{t('未命中任何手续费规则')}</Text>
          )}
        </div>

        <div className='rounded-xl border border-[var(--semi-color-border)] p-4 bg-[var(--semi-color-fill-0)] space-y-2'>
          <div className='flex justify-between items-center'>
            <Text type='tertiary'>{t('命中规则')}</Text>
            <Text className='text-right'>{matchedRuleText}</Text>
          </div>
          <div className='flex justify-between items-center'>
            <Text type='tertiary'>{t('手续费')}</Text>
            <Text>
              {preview?.isValid
                ? formatWithdrawalAmount(preview?.feeAmount, symbol)
                : '--'}
            </Text>
          </div>
          <div className='flex justify-between items-center'>
            <Text type='tertiary'>{t('实际到账')}</Text>
            <Text strong>
              {preview?.isValid
                ? formatWithdrawalAmount(preview?.netAmount, symbol)
                : '--'}
            </Text>
          </div>
          {preview?.isValid ? (
            <Tag color='blue'>
              {t('已命中手续费规则')}
            </Tag>
          ) : (
            <div className='space-y-2'>
              <Tag color='red'>{t('未命中任何手续费规则')}</Tag>
              <Text type='danger'>{blockMessage}</Text>
            </div>
          )}
        </div>

        <div className='text-sm text-[var(--semi-color-text-2)]'>
          {config?.instruction || t('管理员将在线审核并线下支付宝打款')}
        </div>
      </div>
    </Modal>
  );
};

export default WithdrawalApplyModal;
