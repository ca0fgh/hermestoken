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
import {
  Card,
  Empty,
  Typography,
} from '@douyinfe/semi-ui';
import {
  formatRateBpsPercent,
} from '../../helpers/subscriptionReferral';

const InviteDefaultRuleSection = ({
  t,
  rows = [],
  loading = false,
}) => {
  const getTypeLabel = (type) => {
    if (type === 'subscription') return t('订阅返佣');
    return type || t('未知类型');
  };

  return (
    <Card
      className='!rounded-2xl border-0 shadow-sm'
      title={t('模板默认返给被邀请人比例')}
      loading={loading}
    >
      <div className='flex flex-col gap-4'>
        {rows.length === 0 ? (
          <Empty description={t('暂无已授权返佣分组')} />
        ) : (
          rows.map((row) => {
            const group = String(row?.group || '').trim();

            return (
              <div
                key={row.id}
                className='rounded-2xl border border-gray-100 bg-gray-50/60 p-4'
              >
                <div className='mb-3 flex flex-wrap items-center gap-2'>
                  <Typography.Text type='tertiary' className='text-xs'>
                    {t('返佣类型')}
                  </Typography.Text>
                  <span className='rounded-full bg-white px-3 py-1 text-sm font-semibold text-gray-900 shadow-sm'>
                    {getTypeLabel(row.type)}
                  </span>
                  <Typography.Text type='tertiary' className='ml-2 text-xs'>
                    {t('分组')}
                  </Typography.Text>
                  <span className='rounded-full bg-emerald-50 px-3 py-1 text-sm font-semibold text-emerald-700'>
                    {group}
                  </span>
                </div>

                <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
                  <div>
                    <Typography.Text type='tertiary' className='block text-xs'>
                      {t('当前授权总返佣率')}
                    </Typography.Text>
                    <Typography.Text strong>
                      {formatRateBpsPercent(row.effectiveTotalRateBps)}
                    </Typography.Text>
                  </div>
                  <div>
                    <Typography.Text type='tertiary' className='block text-xs'>
                      {t('模板默认返给被邀请人比例')}
                    </Typography.Text>
                    <Typography.Text strong>
                      {formatRateBpsPercent(row.effectiveInviteeRateBps)}
                    </Typography.Text>
                  </div>
                </div>
              </div>
            );
          })
        )}
      </div>
    </Card>
  );
};

export default InviteDefaultRuleSection;
