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
import { formatWithdrawalAmount } from '../../helpers';

const { Text } = Typography;

const WithdrawalMetric = ({ label, value }) => (
  <div className='rounded-xl border border-[var(--semi-color-border)] p-4 bg-[var(--semi-color-fill-0)]'>
    <div className='text-xs text-[var(--semi-color-text-2)] mb-2'>{label}</div>
    <div className='text-xl font-semibold'>{value}</div>
  </div>
);

const WithdrawalCard = ({ t, config, onApply, onOpenHistory }) => {
  const currencySymbol = config?.currencySymbol || '¥';
  const disabled = !config?.enabled || config?.hasOpenWithdrawal;

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
              {t('从主余额发起提现申请，管理员审核后线下支付宝打款。')}
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
            label={t('可提现余额')}
            value={formatWithdrawalAmount(
              config?.availableAmount,
              currencySymbol,
            )}
          />
          <WithdrawalMetric
            label={t('冻结中余额')}
            value={formatWithdrawalAmount(config?.frozenAmount, currencySymbol)}
          />
          <WithdrawalMetric
            label={t('最低提现金额')}
            value={formatWithdrawalAmount(config?.minAmount, currencySymbol)}
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
