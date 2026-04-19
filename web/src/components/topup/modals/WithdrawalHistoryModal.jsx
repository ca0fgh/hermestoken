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

import React, { useEffect, useMemo, useState } from 'react';
import { Modal, Table, Typography, Toast } from '@douyinfe/semi-ui';
import {
  API,
  createUnifiedPaginationProps,
  timestamp2string,
} from '../../../helpers';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import {
  formatWithdrawalAmount,
  getWithdrawalStatusMeta,
  maskAlipayAccount,
} from '../../../helpers/withdrawal';

const { Text } = Typography;

const WithdrawalHistoryModal = ({ visible, onCancel, t, config }) => {
  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const isMobile = useIsMobile();

  const loadHistory = async (currentPage, currentPageSize) => {
    setLoading(true);
    try {
      const res = await API.get(
        `/api/user/withdrawals?p=${currentPage}&page_size=${currentPageSize}`,
      );
      const { success, message, data } = res.data;
      if (success) {
        setItems(data.items || []);
        setTotal(data.total || 0);
      } else {
        Toast.error({ content: message || t('加载提现记录失败') });
      }
    } catch (error) {
      Toast.error({ content: t('加载提现记录失败') });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (visible) {
      loadHistory(page, pageSize);
    }
  }, [visible, page, pageSize]);

  const columns = useMemo(
    () => [
      {
        title: t('提现单号'),
        dataIndex: 'trade_no',
        key: 'trade_no',
        render: (value) => <Text copyable>{value}</Text>,
      },
      {
        title: t('支付宝账号'),
        dataIndex: 'alipay_account',
        key: 'alipay_account',
        render: (value) => maskAlipayAccount(value),
      },
      {
        title: t('申请金额'),
        dataIndex: 'apply_amount',
        key: 'apply_amount',
        render: (value) =>
          formatWithdrawalAmount(value, config?.currencySymbol || '¥'),
      },
      {
        title: t('手续费'),
        dataIndex: 'fee_amount',
        key: 'fee_amount',
        render: (value) =>
          formatWithdrawalAmount(value, config?.currencySymbol || '¥'),
      },
      {
        title: t('实际到账'),
        dataIndex: 'net_amount',
        key: 'net_amount',
        render: (value) =>
          formatWithdrawalAmount(value, config?.currencySymbol || '¥'),
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
    ],
    [config?.currencySymbol, t],
  );

  return (
    <Modal
      title={t('提现记录')}
      visible={visible}
      onCancel={onCancel}
      footer={null}
      size={isMobile ? 'full-width' : 'large'}
    >
      <Table
        columns={columns}
        dataSource={items}
        rowKey='id'
        loading={loading}
        pagination={createUnifiedPaginationProps({
          currentPage: page,
          pageSize,
          total,
          setCurrentPage: setPage,
          setPageSize,
        })}
      />
    </Modal>
  );
};

export default WithdrawalHistoryModal;
