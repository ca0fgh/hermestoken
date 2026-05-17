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
import { Card, Button, Typography, Tag } from '@douyinfe/semi-ui';
import { WalletCards, Landmark, FileText } from 'lucide-react';
import {
  formatWithdrawalAmount,
  getWithdrawalBalanceAmounts,
} from '../../helpers';

const { Text } = Typography;

const getWithdrawalMetricBadgeClassName = (color) => {
  switch (color) {
    case 'green':
      return 'bg-[var(--semi-color-success-light-default)] text-[var(--semi-color-success)]';
    case 'grey':
      return 'bg-[var(--semi-color-fill-1)] text-[var(--semi-color-text-1)]';
    default:
      return 'bg-[var(--semi-color-primary-light-default)] text-[var(--semi-color-primary)]';
  }
};

const WithdrawalMetricBadge = ({ children, color }) => (
  <span
    className={`inline-flex items-center rounded-md px-2 py-1 text-xs font-medium leading-none whitespace-nowrap ${getWithdrawalMetricBadgeClassName(color)}`}
  >
    {children}
  </span>
);

const WithdrawalMetric = ({ label, value, badge, badgeColor = 'blue' }) => (
  <div className='rounded-xl border border-[var(--semi-color-border)] p-4 bg-[var(--semi-color-fill-0)] min-h-[132px] flex flex-col justify-between gap-5'>
    <div className='flex flex-col items-start gap-2'>
      <div className='text-base font-medium leading-tight text-[var(--semi-color-text-2)] whitespace-nowrap'>
        {label}
      </div>
      {badge ? (
        <WithdrawalMetricBadge color={badgeColor}>{badge}</WithdrawalMetricBadge>
      ) : null}
    </div>
    <div className='text-3xl font-semibold leading-tight tabular-nums whitespace-nowrap'>
      {value}
    </div>
  </div>
);

const WithdrawalMetaItem = ({ label, value }) => (
  <div className='rounded-lg border border-[var(--semi-color-border)] px-4 py-3 bg-[var(--semi-color-fill-0)] flex items-center justify-between gap-3'>
    <span className='truncate text-sm text-[var(--semi-color-text-2)]'>
      {label}
    </span>
    <span className='text-base font-semibold tabular-nums whitespace-nowrap'>
      {value}
    </span>
  </div>
);

const WithdrawalCard = ({ t, config, onApply, onOpenHistory }) => {
  const currencySymbol = config?.currencySymbol || '¥';
  const disabled = !config?.enabled || config?.hasOpenWithdrawal;
  const balances = getWithdrawalBalanceAmounts(config);

  return (
    <Card className='!rounded-2xl shadow-sm border-0'>
      <div className='flex flex-col gap-4'>
        <div className='flex items-start justify-between gap-4'>
          <div>
            <div className='flex items-center gap-2'>
              <WalletCards size={18} />
              <Text className='text-lg font-medium'>{t('余额提现')}</Text>
            </div>
            <Text type='tertiary' className='block mt-1'>
              {t('充值余额可申请提现，兑换码余额不可提现。')}
            </Text>
          </div>
          {config?.hasOpenWithdrawal ? (
            <Tag color='orange'>{t('存在未完结提现单')}</Tag>
          ) : config?.enabled ? (
            <Tag color='green'>{t('提现已开启')}</Tag>
          ) : (
            <Tag color='grey'>{t('提现未开启')}</Tag>
          )}
        </div>

        <div className='grid grid-cols-1 md:grid-cols-3 gap-4'>
          <WithdrawalMetric
            label={t('可用总余额')}
            value={formatWithdrawalAmount(balances.totalAmount, currencySymbol)}
          />
          <WithdrawalMetric
            label={t('充值余额')}
            badge={t('可提现')}
            badgeColor='green'
            value={formatWithdrawalAmount(
              balances.rechargeAmount,
              currencySymbol,
            )}
          />
          <WithdrawalMetric
            label={t('兑换码余额')}
            badge={t('不可提现')}
            badgeColor='grey'
            value={formatWithdrawalAmount(
              balances.redemptionAmount,
              currencySymbol,
            )}
          />
        </div>

        <div className='grid grid-cols-1 sm:grid-cols-2 gap-3'>
          <WithdrawalMetaItem
            label={t('冻结中余额')}
            value={formatWithdrawalAmount(balances.frozenAmount, currencySymbol)}
          />
          <WithdrawalMetaItem
            label={t('最低提现金额')}
            value={formatWithdrawalAmount(balances.minAmount, currencySymbol)}
          />
        </div>

        <div className='flex flex-col md:flex-row gap-3 items-start md:items-center justify-between'>
          <div className='flex flex-wrap gap-3 text-sm text-[var(--semi-color-text-2)]'>
            <span className='flex items-center gap-1'>
              <Landmark size={14} />
              {t('提现币种')} {config?.currency}
            </span>
            <span className='flex items-center gap-1'>
              <FileText size={14} />
              {config?.instruction || t('管理员将根据提现说明线下处理打款')}
            </span>
          </div>
          <div className='flex gap-2'>
            <Button onClick={onOpenHistory}>{t('提现记录')}</Button>
            <Button type='primary' disabled={disabled} onClick={onApply}>
              {t('申请提现')}
            </Button>
          </div>
        </div>
      </div>
    </Card>
  );
};

export default WithdrawalCard;
