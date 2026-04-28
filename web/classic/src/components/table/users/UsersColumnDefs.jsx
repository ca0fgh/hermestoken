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
  Space,
  Tag,
  Tooltip,
  Progress,
  Popover,
  Typography,
  Dropdown,
} from '@douyinfe/semi-ui';
import { IconMore } from '@douyinfe/semi-icons';
import {
  renderGroup,
  renderNumber,
  renderQuota,
  timestamp2string,
} from '../../../helpers';

const renderTimestamp = (text) => (text ? timestamp2string(text) : '-');

const getProgressColor = (pct) => {
  if (pct === 100) return 'var(--semi-color-success)';
  if (pct <= 10) return 'var(--semi-color-danger)';
  if (pct <= 30) return 'var(--semi-color-warning)';
  return undefined;
};

/**
 * Render user role
 */
const renderRole = (role, t) => {
  switch (role) {
    case 1:
      return (
        <Tag color='blue' shape='circle'>
          {t('普通用户')}
        </Tag>
      );
    case 10:
      return (
        <Tag color='yellow' shape='circle'>
          {t('管理员')}
        </Tag>
      );
    case 100:
      return (
        <Tag color='orange' shape='circle'>
          {t('超级管理员')}
        </Tag>
      );
    default:
      return (
        <Tag color='red' shape='circle'>
          {t('未知身份')}
        </Tag>
      );
  }
};

/**
 * Render username with remark
 */
const renderUsername = (text, record) => {
  const remark = record.remark;
  if (!remark) {
    return <span>{text}</span>;
  }
  const maxLen = 10;
  const displayRemark =
    remark.length > maxLen ? remark.slice(0, maxLen) + '…' : remark;
  return (
    <Space spacing={2}>
      <span>{text}</span>
      <Tooltip content={remark} position='top' showArrow>
        <Tag color='white' shape='circle' className='!text-xs'>
          <div className='flex items-center gap-1'>
            <div
              className='w-2 h-2 flex-shrink-0 rounded-full'
              style={{ backgroundColor: '#10b981' }}
            />
            {displayRemark}
          </div>
        </Tag>
      </Tooltip>
    </Space>
  );
};

/**
 * Render user statistics
 */
const renderStatistics = (text, record, showEnableDisableModal, t) => {
  const isDeleted = record.DeletedAt !== null;

  // Determine tag text & color like original status column
  let tagColor = 'grey';
  let tagText = t('未知状态');
  if (isDeleted) {
    tagColor = 'red';
    tagText = t('已注销');
  } else if (record.status === 1) {
    tagColor = 'green';
    tagText = t('已启用');
  } else if (record.status === 2) {
    tagColor = 'red';
    tagText = t('已禁用');
  }

  const content = (
    <Tag color={tagColor} shape='circle' size='small'>
      {tagText}
    </Tag>
  );

  const tooltipContent = (
    <div className='text-xs'>
      <div>
        {t('调用次数')}: {renderNumber(record.request_count)}
      </div>
    </div>
  );

  return (
    <Tooltip content={tooltipContent} position='top'>
      {content}
    </Tooltip>
  );
};

const QuotaUsageRow = ({
  label,
  remainText,
  totalText,
  percent,
  ariaLabel,
}) => {
  const amountText = `${remainText} / ${totalText}`;

  return (
    <div
      className='w-full rounded-md border px-2 py-1.5'
      style={{
        backgroundColor: 'var(--semi-color-bg-0)',
        borderColor: 'var(--semi-color-border)',
      }}
    >
      <div className='mb-1 flex items-center justify-between gap-2 text-xs leading-4'>
        <span className='shrink-0 text-gray-500'>{label}</span>
        <span
          className='min-w-0 truncate whitespace-nowrap font-medium text-gray-900'
          title={amountText}
        >
          {amountText}
        </span>
      </div>
      <Progress
        percent={percent}
        stroke={getProgressColor(percent)}
        showInfo={false}
        aria-label={ariaLabel}
        style={{ width: '100%', margin: 0 }}
      />
    </div>
  );
};

// Render separate quota usage column
const renderQuotaUsage = (text, record, t) => {
  const { Paragraph } = Typography;
  const walletUsed = parseInt(record.wallet_amount_used) || 0;
  const walletRemain = parseInt(record.quota) || 0;
  const walletTotal = walletUsed + walletRemain;
  const walletPercent =
    walletTotal > 0 ? (walletRemain / walletTotal) * 100 : 0;
  const subscriptionUsed = parseInt(record.subscription_amount_used) || 0;
  const subscriptionTotal = parseInt(record.subscription_amount_total) || 0;
  const subscriptionUnlimited = Boolean(record.subscription_quota_unlimited);
  const subscriptionRemain = subscriptionUnlimited
    ? 0
    : Math.max(subscriptionTotal - subscriptionUsed, 0);
  const subscriptionPercent = subscriptionUnlimited
    ? 100
    : subscriptionTotal > 0
      ? (subscriptionRemain / subscriptionTotal) * 100
      : 0;
  const subscriptionTotalText = subscriptionUnlimited
    ? t('不限')
    : renderQuota(subscriptionTotal);
  const subscriptionRemainText = subscriptionUnlimited
    ? t('不限')
    : renderQuota(subscriptionRemain);

  const popoverContent = (
    <div className='text-xs p-2'>
      <Paragraph copyable={{ content: renderQuota(walletUsed) }}>
        {t('钱包已用额度')}: {renderQuota(walletUsed)}
      </Paragraph>
      <Paragraph copyable={{ content: renderQuota(walletRemain) }}>
        {t('钱包剩余额度')}: {renderQuota(walletRemain)} (
        {walletPercent.toFixed(0)}%)
      </Paragraph>
      <Paragraph copyable={{ content: renderQuota(walletTotal) }}>
        {t('钱包总额度')}: {renderQuota(walletTotal)}
      </Paragraph>
      <Paragraph copyable={{ content: renderQuota(subscriptionUsed) }}>
        {t('订阅已用额度')}: {renderQuota(subscriptionUsed)}
      </Paragraph>
      <Paragraph
        copyable={{
          content: subscriptionUnlimited
            ? t('不限')
            : renderQuota(subscriptionRemain),
        }}
      >
        {t('订阅剩余额度')}: {subscriptionRemainText} (
        {subscriptionPercent.toFixed(0)}%)
      </Paragraph>
      <Paragraph
        copyable={{
          content: subscriptionUnlimited
            ? t('不限')
            : renderQuota(subscriptionTotal),
        }}
      >
        {t('订阅总额度')}: {subscriptionTotalText}
      </Paragraph>
    </div>
  );
  return (
    <Popover content={popoverContent} position='top'>
      <div className='flex w-[210px] max-w-full flex-col gap-1.5'>
        <QuotaUsageRow
          label={t('钱包')}
          remainText={renderQuota(walletRemain)}
          totalText={renderQuota(walletTotal)}
          percent={walletPercent}
          ariaLabel='wallet quota usage'
        />
        <QuotaUsageRow
          label={t('订阅')}
          remainText={subscriptionRemainText}
          totalText={subscriptionTotalText}
          percent={subscriptionPercent}
          ariaLabel='subscription quota usage'
        />
      </div>
    </Popover>
  );
};

/**
 * Render invite information
 */
const renderInviteInfo = (text, record, t) => {
  return (
    <div>
      <Space spacing={1}>
        <Tag color='white' shape='circle' className='!text-xs'>
          {t('邀请')}: {renderNumber(record.aff_count)}
        </Tag>
        <Tag color='white' shape='circle' className='!text-xs'>
          {t('收益')}: {renderQuota(record.aff_history_quota)}
        </Tag>
        <Tag color='white' shape='circle' className='!text-xs'>
          {record.inviter_id === 0
            ? t('无邀请人')
            : `${t('邀请人')}: ${record.inviter_id}`}
        </Tag>
      </Space>
    </div>
  );
};

const UserOperations = ({
  record,
  setEditingUser,
  setShowEditUser,
  showPromoteModal,
  showDemoteModal,
  showEnableDisableModal,
  showDeleteModal,
  showResetPasskeyModal,
  showResetTwoFAModal,
  showUserSubscriptionsModal,
  t,
}) => {
  if (record.DeletedAt !== null) {
    return <></>;
  }

  const moreMenu = [
    {
      node: 'item',
      name: t('订阅管理'),
      onClick: () => showUserSubscriptionsModal(record),
    },
    {
      node: 'divider',
    },
    {
      node: 'item',
      name: t('重置 Passkey'),
      onClick: () => showResetPasskeyModal(record),
    },
    {
      node: 'item',
      name: t('重置 2FA'),
      onClick: () => showResetTwoFAModal(record),
    },
    {
      node: 'divider',
    },
    {
      node: 'item',
      name: t('注销'),
      type: 'danger',
      onClick: () => showDeleteModal(record),
    },
  ];

  return (
    <Space>
      {record.status === 1 ? (
        <Button
          type='danger'
          size='small'
          onClick={() => showEnableDisableModal(record, 'disable')}
        >
          {t('禁用')}
        </Button>
      ) : (
        <Button
          size='small'
          onClick={() => showEnableDisableModal(record, 'enable')}
        >
          {t('启用')}
        </Button>
      )}
      <Button
        type='tertiary'
        size='small'
        onClick={() => {
          setEditingUser(record);
          setShowEditUser(true);
        }}
      >
        {t('编辑')}
      </Button>
      <Button
        type='warning'
        size='small'
        onClick={() => showPromoteModal(record)}
      >
        {t('提升')}
      </Button>
      <Button
        type='secondary'
        size='small'
        onClick={() => showDemoteModal(record)}
      >
        {t('降级')}
      </Button>
      <Dropdown
        menu={moreMenu}
        trigger='click'
        position='bottomRight'
        clickToHide
      >
        <Button type='tertiary' size='small' icon={<IconMore />} />
      </Dropdown>
    </Space>
  );
};

/**
 * Render operations column
 */
const renderOperations = (
  text,
  record,
  {
    setEditingUser,
    setShowEditUser,
    showPromoteModal,
    showDemoteModal,
    showEnableDisableModal,
    showDeleteModal,
    showResetPasskeyModal,
    showResetTwoFAModal,
    showUserSubscriptionsModal,
    t,
  },
) => (
  <UserOperations
    record={record}
    setEditingUser={setEditingUser}
    setShowEditUser={setShowEditUser}
    showPromoteModal={showPromoteModal}
    showDemoteModal={showDemoteModal}
    showEnableDisableModal={showEnableDisableModal}
    showDeleteModal={showDeleteModal}
    showResetPasskeyModal={showResetPasskeyModal}
    showResetTwoFAModal={showResetTwoFAModal}
    showUserSubscriptionsModal={showUserSubscriptionsModal}
    t={t}
  />
);

/**
 * Get users table column definitions
 */
export const getUsersColumns = ({
  t,
  setEditingUser,
  setShowEditUser,
  showPromoteModal,
  showDemoteModal,
  showEnableDisableModal,
  showDeleteModal,
  showResetPasskeyModal,
  showResetTwoFAModal,
  showUserSubscriptionsModal,
}) => {
  return [
    {
      title: 'ID',
      dataIndex: 'id',
    },
    {
      title: t('用户名'),
      dataIndex: 'username',
      render: (text, record) => renderUsername(text, record),
    },
    {
      title: t('状态'),
      dataIndex: 'info',
      render: (text, record, index) =>
        renderStatistics(text, record, showEnableDisableModal, t),
    },
    {
      title: t('钱包/订阅额度'),
      key: 'quota_usage',
      width: 240,
      render: (text, record) => renderQuotaUsage(text, record, t),
    },
    {
      title: t('分组'),
      dataIndex: 'group',
      render: (text, record, index) => {
        return <div>{renderGroup(text)}</div>;
      },
    },
    {
      title: t('角色'),
      dataIndex: 'role',
      render: (text, record, index) => {
        return <div>{renderRole(text, t)}</div>;
      },
    },
    {
      title: t('邀请信息'),
      dataIndex: 'invite',
      render: (text, record, index) => renderInviteInfo(text, record, t),
    },
    {
      title: t('创建时间'),
      dataIndex: 'created_at',
      render: renderTimestamp,
    },
    {
      title: t('最后登录'),
      dataIndex: 'last_login_at',
      render: renderTimestamp,
    },
    {
      title: '',
      dataIndex: 'operate',
      fixed: 'right',
      width: 200,
      render: (text, record, index) =>
        renderOperations(text, record, {
          setEditingUser,
          setShowEditUser,
          showPromoteModal,
          showDemoteModal,
          showEnableDisableModal,
          showDeleteModal,
          showResetPasskeyModal,
          showResetTwoFAModal,
          showUserSubscriptionsModal,
          t,
        }),
    },
  ];
};
