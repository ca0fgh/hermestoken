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
import { Card, Empty, Tag, Typography } from '@douyinfe/semi-ui';
import {
  buildInviteeContributionLedgerRows,
  buildInviteeContributionSummary,
} from '../../helpers/inviteRebate';
import { renderQuotaWithLessThanFloor } from '../../helpers/quota';
import { timestamp2string } from '../../helpers/utils';
import ListPagination from '../common/ui/ListPagination';

const PAGE_SIZE = 5;

const InviteReceivedContributionSection = ({
  t,
  cards = [],
  loading = false,
}) => {
  const [currentPage, setCurrentPage] = useState(1);
  const summary = useMemo(
    () => buildInviteeContributionSummary(cards),
    [cards],
  );
  const ledgerRows = useMemo(
    () => buildInviteeContributionLedgerRows(cards),
    [cards],
  );
  const totalPages = Math.max(1, Math.ceil(ledgerRows.length / PAGE_SIZE));

  useEffect(() => {
    setCurrentPage((page) => Math.min(page, totalPages));
  }, [totalPages]);

  const pagedRows = useMemo(() => {
    const start = (currentPage - 1) * PAGE_SIZE;
    return ledgerRows.slice(start, start + PAGE_SIZE);
  }, [currentPage, ledgerRows]);

  const formatStatusLabel = (status) => {
    if (status === 'reversed') return t('已冲正');
    if (status === 'pending') return t('待结算');
    return t('已到账');
  };

  const renderStatusColor = (status) => {
    if (status === 'reversed') return 'red';
    if (status === 'pending') return 'orange';
    return 'green';
  };

  return (
    <Card
      className='!rounded-2xl border-0 shadow-sm'
      title={t('上级返给我的流水')}
      loading={loading}
    >
      <div className='flex flex-col gap-4'>
        <Typography.Text type='secondary' className='block text-sm'>
          {t('这里展示邀请人返给你的返佣到账记录。')}
        </Typography.Text>

        <div className='grid grid-cols-1 gap-3 md:grid-cols-2'>
          <div className='rounded-2xl border border-gray-100 bg-white px-4 py-3'>
            <Typography.Text type='tertiary' className='block text-xs'>
              {t('到账订单数')}
            </Typography.Text>
            <Typography.Text strong className='text-base'>
              {summary.orderCount}
            </Typography.Text>
          </div>
          <div className='rounded-2xl border border-emerald-100 bg-emerald-50/70 px-4 py-3'>
            <Typography.Text type='tertiary' className='block text-xs'>
              {t('累计收到返佣')}
            </Typography.Text>
            <Typography.Text strong className='text-base'>
              {renderQuotaWithLessThanFloor(summary.inviteeRewardQuota)}
            </Typography.Text>
          </div>
        </div>

        {ledgerRows.length === 0 ? (
          <Empty description={t('暂无返佣明细')} />
        ) : (
          <div className='space-y-3'>
            {pagedRows.map((item) => (
              <div
                key={item.id}
                className='rounded-xl border border-emerald-100 bg-emerald-50/40 px-4 py-3'
              >
                <div className='flex flex-wrap items-center justify-between gap-3'>
                  <div className='flex flex-wrap gap-2'>
                    <Tag color='white'>{item.group || '-'}</Tag>
                    <Tag color='blue'>{t(item.roleLabel)}</Tag>
                    <Tag color='green'>{t('收到返佣')}</Tag>
                    <Tag color={renderStatusColor(item.status)}>
                      {formatStatusLabel(item.status)}
                    </Tag>
                  </div>
                </div>

                <div className='mt-3 grid grid-cols-1 gap-3 text-sm md:grid-cols-5'>
                  <div>
                    <Typography.Text type='tertiary' className='block text-xs'>
                      {t('订单号')}
                    </Typography.Text>
                    <Typography.Text>{item.tradeNo || '-'}</Typography.Text>
                  </div>
                  <div>
                    <Typography.Text type='tertiary' className='block text-xs'>
                      {t('来源身份')}
                    </Typography.Text>
                    <Typography.Text>{t(item.roleLabel)}</Typography.Text>
                  </div>
                  <div>
                    <Typography.Text type='tertiary' className='block text-xs'>
                      {t('来源返佣')}
                    </Typography.Text>
                    <Typography.Text>
                      {item.sourceComponentLabel
                        ? t(item.sourceComponentLabel)
                        : '-'}
                    </Typography.Text>
                  </div>
                  <div>
                    <Typography.Text type='tertiary' className='block text-xs'>
                      {t('到账返佣')}
                    </Typography.Text>
                    <Typography.Text>
                      {renderQuotaWithLessThanFloor(
                        item.effectiveRewardQuota || 0,
                      )}
                    </Typography.Text>
                  </div>
                  <div>
                    <Typography.Text type='tertiary' className='block text-xs'>
                      {t('结算时间')}
                    </Typography.Text>
                    <Typography.Text>
                      {item.settledAt ? timestamp2string(item.settledAt) : '-'}
                    </Typography.Text>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}

        <ListPagination
          total={ledgerRows.length}
          pageSize={PAGE_SIZE}
          currentPage={currentPage}
          pageSizeOpts={[PAGE_SIZE]}
          showSizeChanger={false}
          onPageChange={setCurrentPage}
        />
      </div>
    </Card>
  );
};

export default InviteReceivedContributionSection;
