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
  Button,
  Card,
  Empty,
  Input,
  List,
  Pagination,
  Space,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';

const InviteeListPanel = ({
  t,
  loading = false,
  keyword = '',
  pageData = {},
  selectedInviteeId = null,
  onKeywordChange,
  onSearch,
  onSelectInvitee,
  onPageChange,
  emptyHint,
}) => {
  const items = Array.isArray(pageData?.items) ? pageData.items : [];

  return (
    <Card
      className='!rounded-2xl border-0 shadow-sm h-full bg-white/95 backdrop-blur'
      title={t('邀请用户')}
      bodyStyle={{
        display: 'flex',
        flexDirection: 'column',
        gap: 16,
        minHeight: 640,
      }}
      loading={loading}
    >
      <Space style={{ width: '100%' }} align='start'>
        <Input
          value={keyword}
          showClear
          style={{ flex: 1 }}
          placeholder={t('搜索用户名')}
          onChange={onKeywordChange}
          onEnterPress={onSearch}
        />
        <Button type='primary' className='shrink-0' onClick={onSearch}>
          {t('搜索')}
        </Button>
      </Space>

      {items.length === 0 ? (
        <Empty description={emptyHint || t('暂无邀请用户')} />
      ) : (
        <List
          dataSource={items}
          renderItem={(invitee) => {
            const isActive = invitee.id === selectedInviteeId;
            return (
              <List.Item
                className={`cursor-pointer rounded-2xl border px-4 py-3 transition ${
                  isActive
                    ? 'border-emerald-400 bg-emerald-50 shadow-sm'
                    : 'border-transparent bg-gray-50 hover:border-gray-200 hover:bg-white'
                }`}
                onClick={() => onSelectInvitee?.(invitee)}
                main={
                  <div className='flex flex-col gap-2'>
                    <div className='flex items-center justify-between gap-3'>
                      <Typography.Text strong>
                        {invitee.username || `#${invitee.id}`}
                      </Typography.Text>
                      <Tag color={isActive ? 'green' : 'light-blue'}>
                        {invitee.group || '-'}
                      </Tag>
                    </div>
                    <div className='flex items-center justify-between gap-3 text-xs'>
                      <Typography.Text type='tertiary'>
                        ID: {invitee.id}
                      </Typography.Text>
                      <Typography.Text type='tertiary'>
                        {t('累计返佣收益')}:{' '}
                        {Number(invitee.contribution_quota || 0)}
                      </Typography.Text>
                    </div>
                  </div>
                }
              />
            );
          }}
        />
      )}

      <div className='pt-1'>
        <Pagination
          total={Number(pageData?.total || 0)}
          pageSize={Number(pageData?.page_size || 10)}
          currentPage={Number(pageData?.page || 1)}
          pageSizeOpts={[10, 20, 50]}
          onPageChange={(page) =>
            onPageChange?.({
              page,
              page_size: Number(pageData?.page_size || 10),
            })
          }
          onPageSizeChange={(page_size) =>
            onPageChange?.({
              page: 1,
              page_size,
            })
          }
        />
      </div>
    </Card>
  );
};

export default InviteeListPanel;
