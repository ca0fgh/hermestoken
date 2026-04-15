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
import { Card, Typography } from '@douyinfe/semi-ui';

const numberFormatter = new Intl.NumberFormat();

const summaryItems = (t, inviteeCount, totalContributionQuota) => [
  {
    label: t('被邀请人数'),
    value: numberFormatter.format(Number(inviteeCount || 0)),
  },
  {
    label: t('累计返佣收益'),
    value: numberFormatter.format(Number(totalContributionQuota || 0)),
  },
];

const InviteRebateSummary = ({
  t,
  inviteeCount = 0,
  totalContributionQuota = 0,
}) => {
  return (
    <div className='grid grid-cols-1 gap-4 md:grid-cols-2'>
      {summaryItems(t, inviteeCount, totalContributionQuota).map((item) => (
        <Card
          key={item.label}
          className='!rounded-2xl border-0 shadow-sm bg-[linear-gradient(135deg,#0f766e,#164e63)] text-white'
          bodyStyle={{ padding: 20 }}
        >
          <Typography.Text
            style={{ color: 'rgba(255,255,255,0.75)' }}
            className='block text-sm'
          >
            {item.label}
          </Typography.Text>
          <Typography.Title
            heading={3}
            style={{ color: 'white', margin: '10px 0 0' }}
          >
            {item.value}
          </Typography.Title>
        </Card>
      ))}
    </div>
  );
};

export default InviteRebateSummary;
