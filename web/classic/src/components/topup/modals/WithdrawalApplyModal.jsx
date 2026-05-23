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

import React from 'react';
import {
  Modal,
  Typography,
  Input,
  InputNumber,
  Select,
  Tag,
} from '@douyinfe/semi-ui';
import { HandCoins } from 'lucide-react';
import {
  WITHDRAWAL_USDT_NETWORK_OPTIONS,
  buildWithdrawalFeeRuleDescriptions,
  describeWithdrawalFeeRuleForUser,
  formatWithdrawalAmount,
  getWithdrawalBalanceAmounts,
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
  withdrawalChannel,
  setWithdrawalChannel,
  alipayAccount,
  setAlipayAccount,
  alipayRealName,
  setAlipayRealName,
  usdtNetwork,
  setUSDTNetwork,
  usdtAddress,
  setUSDTAddress,
  preview,
}) => {
  const symbol = config?.currencySymbol || '¥';
  const isUSDTWithdrawal = withdrawalChannel === 'usdt';
  const channelOptions = [
    { label: t('支付宝'), value: 'alipay' },
    { label: t('USDT'), value: 'usdt' },
  ];
  const usdtNetworkOptions = WITHDRAWAL_USDT_NETWORK_OPTIONS.map((option) => ({
    ...option,
    label: t(option.label),
  }));
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
    ? describeWithdrawalFeeRuleForUser(
        preview.matchedRule,
        t,
        ruleDescriptionOptions,
      )
    : t('未命中任何手续费规则');
  const blockMessage = preview?.isValid
    ? ''
    : t(
        preview?.blockReason ||
          '当前提现金额未命中任何手续费规则，请调整金额或联系管理员',
      );
  const balances = getWithdrawalBalanceAmounts(config);
  const maxWithdrawAmountText = formatWithdrawalAmount(
    balances.rechargeAmount,
    symbol,
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
            min={balances.minAmount || 0}
            max={balances.rechargeAmount || 0}
            value={amount}
            onChange={(value) => setAmount(value || 0)}
            style={{ width: '100%' }}
          />
          <Text type='tertiary' size='small'>
            {t('最多可提 {{amount}}', { amount: maxWithdrawAmountText })}
          </Text>
        </div>

        <div className='rounded-xl border border-[var(--semi-color-border)] p-3 bg-[var(--semi-color-fill-0)]'>
          <div className='grid grid-cols-1 sm:grid-cols-3 gap-3'>
            <div>
              <div className='text-xs text-[var(--semi-color-text-2)] mb-1'>
                {t('可用总余额')}
              </div>
              <div className='font-medium'>
                {formatWithdrawalAmount(balances.totalAmount, symbol)}
              </div>
            </div>
            <div>
              <div className='text-xs text-[var(--semi-color-text-2)] mb-1'>
                {t('充值余额（可提现）')}
              </div>
              <div className='font-medium'>
                {formatWithdrawalAmount(balances.rechargeAmount, symbol)}
              </div>
            </div>
            <div>
              <div className='text-xs text-[var(--semi-color-text-2)] mb-1'>
                {t('兑换码余额（不可提现）')}
              </div>
              <div className='font-medium'>
                {formatWithdrawalAmount(balances.redemptionAmount, symbol)}
              </div>
            </div>
          </div>
        </div>

        <div>
          <Text strong className='block mb-2'>
            {t('提现方式')}
          </Text>
          <Select
            value={withdrawalChannel}
            onChange={setWithdrawalChannel}
            optionList={channelOptions}
            style={{ width: '100%' }}
          />
        </div>

        {isUSDTWithdrawal ? (
          <>
            <div>
              <Text strong className='block mb-2'>
                {t('USDT 网络')}
              </Text>
              <Select
                value={usdtNetwork}
                onChange={setUSDTNetwork}
                optionList={usdtNetworkOptions}
                style={{ width: '100%' }}
              />
            </div>

            <div>
              <Text strong className='block mb-2'>
                {t('USDT 收款地址')}
              </Text>
              <Input
                value={usdtAddress}
                onChange={setUSDTAddress}
                placeholder={t('请输入 USDT 收款地址')}
              />
              <Text type='tertiary' size='small'>
                {t('请确认收款地址与所选 USDT 网络一致。')}
              </Text>
            </div>
          </>
        ) : (
          <>
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
          </>
        )}

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
            <Tag color='blue'>{t('已命中手续费规则')}</Tag>
          ) : (
            <div className='space-y-2'>
              <Tag color='red'>{t('未命中任何手续费规则')}</Tag>
              <Text type='danger'>{blockMessage}</Text>
            </div>
          )}
        </div>

        <div className='text-sm text-[var(--semi-color-text-2)]'>
          {config?.instruction ||
            (isUSDTWithdrawal
              ? t('管理员将在线审核并线下 USDT 打款')
              : t('管理员将在线审核并线下支付宝打款'))}
        </div>
      </div>
    </Modal>
  );
};

export default WithdrawalApplyModal;
