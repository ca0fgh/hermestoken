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
import {
  Card,
  Empty,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import {
  formatRateBpsPercent,
} from '../../helpers/subscriptionReferral';
import ListPagination from '../common/ui/ListPagination';

const PAGE_SIZE = 5;

const InviteDefaultRuleSection = ({
  t,
  rows = [],
  loading = false,
}) => {
  const [currentPage, setCurrentPage] = useState(1);

  const formatModeLabel = (levelType) => {
    if (levelType === 'direct') return t('直推');
    if (levelType === 'team') return t('团队');
    return '-';
  };

  const total = Array.isArray(rows) ? rows.length : 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  useEffect(() => {
    setCurrentPage((page) => Math.min(page, totalPages));
  }, [totalPages]);

  const pagedRows = useMemo(() => {
    const start = (currentPage - 1) * PAGE_SIZE;
    return rows.slice(start, start + PAGE_SIZE);
  }, [currentPage, rows]);

  return (
    <Card
      className='!rounded-2xl border-0 shadow-sm'
      title={t('我的返佣方案')}
      loading={loading}
    >
      <div className='flex flex-col gap-4'>
        {rows.length === 0 ? (
          <Empty description={t('暂无已启用返佣模板')} />
        ) : (
          pagedRows.map((row) => {
            const group = String(row?.group || '').trim();
            const templateName = String(row?.templateName || '').trim() || '-';

            return (
              <div
                key={row.id}
                className='rounded-2xl border border-gray-100 bg-gray-50/60 p-4'
              >
                <div className='flex flex-wrap items-center justify-between gap-3'>
                  <div>
                    <Typography.Text type='tertiary' className='block text-xs'>
                      {t('当前返佣方案')}
                    </Typography.Text>
                    <Typography.Text strong>{templateName}</Typography.Text>
                  </div>
                  <Tag color='light-blue'>{formatModeLabel(row?.levelType)}</Tag>
                </div>

                <div className='mt-4 grid grid-cols-1 gap-4 md:grid-cols-3'>
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
                    <Typography.Text strong>{formatRateBpsPercent(row.effectiveTotalRateBps)}</Typography.Text>
                  </div>
                </div>

                <div className='mt-4 grid grid-cols-1 gap-4 md:grid-cols-2'>
                  <div>
                    <Typography.Text type='tertiary' className='block text-xs'>
                      {t('默认返给对方比例')}
                    </Typography.Text>
                    <Typography.Text strong>
                      {formatRateBpsPercent(row.effectiveInviteeRateBps)}
                    </Typography.Text>
                  </div>
                  <div>
                    <Typography.Text type='tertiary' className='block text-xs'>
                      {t('你本单保留比例')}
                    </Typography.Text>
                    <Typography.Text strong>
                      {formatRateBpsPercent(row.effectiveInviterRateBps)}
                    </Typography.Text>
                  </div>
                </div>
              </div>
            );
          })
        )}
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

export default InviteDefaultRuleSection;
