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
import { Typography, Switch } from '@douyinfe/semi-ui';

const { Text } = Typography;

const WithdrawalsDescription = ({ t, compactMode, setCompactMode }) => {
  return (
    <div className='flex flex-col md:flex-row md:items-center md:justify-between gap-3'>
      <div>
        <div className='text-lg font-semibold'>{t('提现管理')}</div>
        <Text type='tertiary'>
          {t('查看用户提现申请，完成审核、驳回和确认打款。')}
        </Text>
      </div>
      <div className='flex items-center gap-2'>
        <Text type='tertiary'>{t('紧凑模式')}</Text>
        <Switch checked={compactMode} onChange={setCompactMode} />
      </div>
    </div>
  );
};

export default WithdrawalsDescription;
