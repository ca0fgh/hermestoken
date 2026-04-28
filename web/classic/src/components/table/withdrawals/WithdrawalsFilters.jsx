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
import { Button, Input, Select } from '@douyinfe/semi-ui';

const WithdrawalsFilters = ({ t, filters, setFilters, onSearch, loading }) => {
  const updateField = (key, value) => {
    setFilters({
      ...filters,
      [key]: value,
    });
  };

  return (
    <div className='flex flex-col md:flex-row gap-2 w-full'>
      <Input
        value={filters.keyword}
        onChange={(value) => updateField('keyword', value)}
        placeholder={`${t('搜索提现单号 / 用户名 / 支付宝账号 / USDT地址')} / ${t('用户 ID')}`}
      />
      <Select
        value={filters.status}
        onChange={(value) => updateField('status', value)}
        placeholder={t('状态')}
        style={{ minWidth: 160 }}
        optionList={[
          { label: t('全部状态'), value: '' },
          { label: t('待审核'), value: 'pending' },
          { label: t('待打款'), value: 'approved' },
          { label: t('已打款'), value: 'paid' },
          { label: t('已驳回'), value: 'rejected' },
        ]}
      />
      <Button loading={loading} onClick={() => onSearch(filters)}>
        {t('搜索')}
      </Button>
    </div>
  );
};

export default WithdrawalsFilters;
