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
import { Button, Space, Typography } from '@douyinfe/semi-ui';
import { timestamp2string } from '../../../helpers';
import {
  formatWithdrawalAmount,
  getWithdrawalChannelLabel,
  getWithdrawalCurrencySymbol,
  getWithdrawalPayoutAccount,
  getWithdrawalPayoutNote,
  getWithdrawalStatusMeta,
} from '../../../helpers/withdrawal';

const { Text } = Typography;

export const getWithdrawalsColumns = ({
  t,
  openApproveModal,
  openRejectModal,
  openPaidModal,
}) => [
  {
    title: t('提现单号'),
    dataIndex: 'trade_no',
    key: 'trade_no',
    render: (value) => <Text copyable>{value}</Text>,
  },
  {
    title: t('用户'),
    dataIndex: 'username',
    key: 'username',
    render: (value, record) => value || `#${record.user_id}`,
  },
  {
    title: t('提现方式'),
    dataIndex: 'channel',
    key: 'channel',
    render: (value) => getWithdrawalChannelLabel(value, t),
  },
  {
    title: t('收款账号'),
    key: 'payout_account',
    render: (_, record) => {
      const account = getWithdrawalPayoutAccount(record);
      return <Text copyable={{ content: account }}>{account || '--'}</Text>;
    },
  },
  {
    title: t('收款备注'),
    key: 'payout_note',
    render: (_, record) => getWithdrawalPayoutNote(record, t),
  },
  {
    title: t('申请金额'),
    dataIndex: 'apply_amount',
    key: 'apply_amount',
    render: (value, record) =>
      formatWithdrawalAmount(
        value,
        getWithdrawalCurrencySymbol(record?.currency),
      ),
  },
  {
    title: t('手续费'),
    dataIndex: 'fee_amount',
    key: 'fee_amount',
    render: (value, record) =>
      formatWithdrawalAmount(
        value,
        getWithdrawalCurrencySymbol(record?.currency),
      ),
  },
  {
    title: t('实际到账'),
    dataIndex: 'net_amount',
    key: 'net_amount',
    render: (value, record) =>
      formatWithdrawalAmount(
        value,
        getWithdrawalCurrencySymbol(record?.currency),
      ),
  },
  {
    title: t('状态'),
    dataIndex: 'status',
    key: 'status',
    render: (value) => {
      const meta = getWithdrawalStatusMeta(value, t);
      return (
        <Text style={{ color: `var(--semi-color-${meta.color}-5)` }}>
          {meta.text}
        </Text>
      );
    },
  },
  {
    title: t('申请时间'),
    dataIndex: 'created_at',
    key: 'created_at',
    render: (value) => timestamp2string(value),
  },
  {
    title: t('操作'),
    key: 'action',
    render: (_, record) => {
      if (record.status === 'pending') {
        return (
          <Space>
            <Button
              size='small'
              type='primary'
              theme='outline'
              onClick={() => openApproveModal(record)}
            >
              {t('审核通过')}
            </Button>
            <Button
              size='small'
              theme='outline'
              onClick={() => openRejectModal(record)}
            >
              {t('驳回')}
            </Button>
          </Space>
        );
      }

      if (record.status === 'approved') {
        return (
          <Space>
            <Button
              size='small'
              type='primary'
              theme='outline'
              onClick={() => openPaidModal(record)}
            >
              {t('确认已打款')}
            </Button>
            <Button
              size='small'
              theme='outline'
              onClick={() => openRejectModal(record)}
            >
              {t('驳回')}
            </Button>
          </Space>
        );
      }

      return null;
    },
  },
];
