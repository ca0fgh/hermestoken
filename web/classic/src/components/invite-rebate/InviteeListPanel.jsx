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
  Space,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { renderQuota } from '../../helpers/quota';
import ListPagination from '../common/ui/ListPagination';

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
  const totalInvitees = Number(pageData?.total || items.length || 0);

  return (
    <Card
      className='!rounded-2xl border-0 shadow-sm h-full bg-white/95 backdrop-blur'
      title={t('我的邀请用户')}
      bodyStyle={{
        display: 'flex',
        flexDirection: 'column',
        gap: 16,
        minHeight: 640,
      }}
      loading={loading}
    >
      <div className='rounded-2xl border border-blue-100 bg-blue-50/70 px-4 py-3'>
        <Typography.Text type='secondary' className='block text-sm'>
          {t('按返佣收益排序，支持按用户名、用户ID或分组查找。')}
        </Typography.Text>
        <Typography.Text type='tertiary' className='mt-1 block text-xs'>
          {t('共 {{count}} 位邀请用户', { count: totalInvitees })}
        </Typography.Text>
      </div>

      <Space style={{ width: '100%' }} align='start'>
        <Input
          value={keyword}
          showClear
          style={{ flex: 1 }}
          placeholder={t('搜索用户名 / 用户ID / 分组')}
          onChange={onKeywordChange}
          onEnterPress={onSearch}
        />
        <Button type='primary' className='shrink-0' onClick={onSearch}>
          {t('搜索')}
        </Button>
      </Space>

      {items.length === 0 ? (
        <Empty
          description={
            emptyHint || (keyword ? t('未找到匹配邀请用户') : t('暂无邀请用户数据'))
          }
        />
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
                        {renderQuota(invitee.contribution_quota || 0)}
                      </Typography.Text>
                    </div>
                    <div className='flex items-center justify-between gap-3 text-xs'>
                      <Typography.Text type='tertiary'>
                        {t('已单独设置')}: {Number(invitee.override_group_count || 0)}
                      </Typography.Text>
                      <Typography.Text type='tertiary'>
                        {t('购买订单')}: {Number(invitee.order_count || 0)}
                      </Typography.Text>
                    </div>
                    <div className='flex items-center justify-between gap-3 text-xs'>
                      <Tag
                        size='small'
                        color={
                          Number(invitee.override_group_count || 0) > 0
                            ? 'green'
                            : 'grey'
                        }
                      >
                        {Number(invitee.override_group_count || 0) > 0
                          ? t('已单独设置返佣')
                          : t('使用默认')}
                      </Tag>
                    </div>
                  </div>
                }
              />
            );
          }}
        />
      )}

      <ListPagination
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
    </Card>
  );
};

export default InviteeListPanel;
