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
import { Card, Tag, Typography } from '@douyinfe/semi-ui';
import { formatRateBpsPercent } from '../../helpers/subscriptionReferral';
import ListPagination from '../common/ui/ListPagination';

const PAGE_SIZE = 5;

const InviteReceivedRuleSection = ({ t, rows = [], loading = false }) => {
  const [currentPage, setCurrentPage] = useState(1);
  const total = Array.isArray(rows) ? rows.length : 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  useEffect(() => {
    setCurrentPage((page) => Math.min(page, totalPages));
  }, [totalPages]);

  const pagedRows = useMemo(() => {
    const start = (currentPage - 1) * PAGE_SIZE;
    return rows.slice(start, start + PAGE_SIZE);
  }, [currentPage, rows]);

  const formatModeLabel = (levelType) => {
    if (levelType === 'direct') return t('直推');
    if (levelType === 'team') return t('团队');
    return '-';
  };

  return (
    <Card
      className='!rounded-2xl border-0 shadow-sm'
      title={t('上级给我的返佣')}
      loading={loading}
    >
      <div className='flex flex-col gap-4'>
        <Typography.Text type='secondary' className='block text-sm'>
          {t('如果邀请人给你单独设置了返佣，这里会展示当前生效规则。')}
        </Typography.Text>

        {pagedRows.map((row) => {
          const inviterUsername =
            String(row?.inviterUsername || '').trim() ||
            `#${String(row?.inviterId || '').trim() || '-'}`;
          const group = String(row?.group || '').trim() || '-';
          const templateName = String(row?.templateName || '').trim() || '-';

          return (
            <div
              key={row.id}
              className='rounded-2xl border border-emerald-100 bg-emerald-50/40 p-4'
            >
              <div className='flex flex-wrap items-center justify-between gap-3'>
                <div>
                  <Typography.Text type='tertiary' className='block text-xs'>
                    {t('邀请人')}
                  </Typography.Text>
                  <Typography.Text strong>{inviterUsername}</Typography.Text>
                </div>
                {row?.hasOverride ? (
                  <Tag color='green'>{t('已单独设置返佣')}</Tag>
                ) : null}
              </div>

              <div className='mt-4 grid grid-cols-1 gap-4 md:grid-cols-4'>
                <div>
                  <Typography.Text type='tertiary' className='block text-xs'>
                    {t('当前返佣方案')}
                  </Typography.Text>
                  <Typography.Text strong>{templateName}</Typography.Text>
                </div>
                <div>
                  <Typography.Text type='tertiary' className='block text-xs'>
                    {t('返佣模式')}
                  </Typography.Text>
                  <Typography.Text strong>{formatModeLabel(row?.levelType)}</Typography.Text>
                </div>
                <div>
                  <Typography.Text type='tertiary' className='block text-xs'>
                    {t('所在分组')}
                  </Typography.Text>
                  <Typography.Text strong>{group}</Typography.Text>
                </div>
                <div>
                  <Typography.Text type='tertiary' className='block text-xs'>
                    {t('本组总返佣比例')}
                  </Typography.Text>
                  <Typography.Text strong>
                    {formatRateBpsPercent(row?.effectiveTotalRateBps)}
                  </Typography.Text>
                </div>
              </div>

              <div className='mt-4 grid grid-cols-1 gap-4 md:grid-cols-2'>
                <div>
                  <Typography.Text type='tertiary' className='block text-xs'>
                    {t('给我的返佣比例')}
                  </Typography.Text>
                  <Typography.Text strong>
                    {formatRateBpsPercent(row?.effectiveInviteeRateBps)}
                  </Typography.Text>
                </div>
                <div>
                  <Typography.Text type='tertiary' className='block text-xs'>
                    {t('你本单保留比例')}
                  </Typography.Text>
                  <Typography.Text strong>
                    {formatRateBpsPercent(row?.effectiveInviterRateBps)}
                  </Typography.Text>
                </div>
              </div>
            </div>
          );
        })}

        <ListPagination
          total={rows.length}
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

export default InviteReceivedRuleSection;
