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
import {
  Avatar,
  Typography,
  Card,
  Button,
  Input,
  InputNumber,
  Badge,
  Space,
} from '@douyinfe/semi-ui';
import { Copy, Users, BarChart2, TrendingUp, Gift, Zap } from 'lucide-react';
import {
  formatRateBpsPercent,
  rateBpsToPercentNumber,
} from '../../helpers/subscriptionReferral';

const { Text } = Typography;

const InvitationCard = ({
  t,
  userState,
  renderQuota,
  setOpenTransfer,
  affLink,
  inviteeRateBps,
  inviteeRateDirty,
  inviteeRateSaving,
  maxInviteeRateBps,
  onInviteeRateChange,
  onInviteeRateSave,
  handleAffLinkClick,
}) => {
  const hasInviteeRateCapacity = Number(maxInviteeRateBps || 0) > 0;
  const canSaveInviteeRate = hasInviteeRateCapacity && inviteeRateDirty;

  return (
    <Card className='!rounded-2xl shadow-sm border-0'>
      {/* 卡片头部 */}
      <div className='flex items-center mb-4'>
        <Avatar size='small' color='green' className='mr-3 shadow-md'>
          <Gift size={16} />
        </Avatar>
        <div>
          <Typography.Text className='text-lg font-medium'>
            {t('邀请奖励')}
          </Typography.Text>
          <div className='text-xs'>{t('邀请被邀请人获得额外奖励')}</div>
        </div>
      </div>

      {/* 收益展示区域 */}
      <Space vertical style={{ width: '100%' }}>
        {/* 统计数据统一卡片 */}
        <Card
          className='!rounded-xl w-full'
          cover={
            <div
              className='relative h-30'
              style={{
                '--palette-primary-darkerChannel': '0 75 80',
                backgroundImage: `linear-gradient(0deg, rgba(var(--palette-primary-darkerChannel) / 80%), rgba(var(--palette-primary-darkerChannel) / 80%)), url('/cover-4.webp')`,
                backgroundSize: 'cover',
                backgroundPosition: 'center',
                backgroundRepeat: 'no-repeat',
              }}
            >
              {/* 标题和按钮 */}
              <div className='relative z-10 h-full flex flex-col justify-between p-4'>
                <div className='flex justify-between items-center'>
                  <Text strong style={{ color: 'white', fontSize: '16px' }}>
                    {t('收益统计')}
                  </Text>
                  <Button
                    type='primary'
                    theme='solid'
                    size='small'
                    disabled={
                      !userState?.user?.aff_quota ||
                      userState?.user?.aff_quota <= 0
                    }
                    onClick={() => setOpenTransfer(true)}
                    className='!rounded-lg'
                  >
                    <Zap size={12} className='mr-1' />
                    {t('划转到余额')}
                  </Button>
                </div>

                {/* 统计数据 */}
                <div className='grid grid-cols-3 gap-6 mt-4'>
                  {/* 待使用收益 */}
                  <div className='text-center'>
                    <div
                      className='text-base sm:text-2xl font-bold mb-2'
                      style={{ color: 'white' }}
                    >
                      {renderQuota(userState?.user?.aff_quota || 0)}
                    </div>
                    <div className='flex items-center justify-center text-sm'>
                      <TrendingUp
                        size={14}
                        className='mr-1'
                        style={{ color: 'rgba(255,255,255,0.8)' }}
                      />
                      <Text
                        style={{
                          color: 'rgba(255,255,255,0.8)',
                          fontSize: '12px',
                        }}
                      >
                        {t('待使用收益')}
                      </Text>
                    </div>
                  </div>

                  {/* 总收益 */}
                  <div className='text-center'>
                    <div
                      className='text-base sm:text-2xl font-bold mb-2'
                      style={{ color: 'white' }}
                    >
                      {renderQuota(userState?.user?.aff_history_quota || 0)}
                    </div>
                    <div className='flex items-center justify-center text-sm'>
                      <BarChart2
                        size={14}
                        className='mr-1'
                        style={{ color: 'rgba(255,255,255,0.8)' }}
                      />
                      <Text
                        style={{
                          color: 'rgba(255,255,255,0.8)',
                          fontSize: '12px',
                        }}
                      >
                        {t('总收益')}
                      </Text>
                    </div>
                  </div>

                  {/* 邀请人数 */}
                  <div className='text-center'>
                    <div
                      className='text-base sm:text-2xl font-bold mb-2'
                      style={{ color: 'white' }}
                    >
                      {userState?.user?.aff_count || 0}
                    </div>
                    <div className='flex items-center justify-center text-sm'>
                      <Users
                        size={14}
                        className='mr-1'
                        style={{ color: 'rgba(255,255,255,0.8)' }}
                      />
                      <Text
                        style={{
                          color: 'rgba(255,255,255,0.8)',
                          fontSize: '12px',
                        }}
                      >
                        {t('邀请人数')}
                      </Text>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          }
        >
          <div className='grid grid-cols-1 gap-3 lg:grid-cols-[240px_minmax(0,1fr)]'>
            <div className='rounded-xl border border-[rgba(var(--semi-grey-2),1)] bg-[rgba(var(--semi-grey-0),.55)] px-4 py-3'>
              <div className='mb-2 flex items-center justify-between gap-2'>
                <Text strong className='whitespace-nowrap text-sm'>
                  {t('受邀人比例')}
                </Text>
                <div className='flex items-center gap-2'>
                  <Text type='tertiary' className='whitespace-nowrap text-xs'>
                    {t('最高')} {formatRateBpsPercent(maxInviteeRateBps)}
                  </Text>
                  <Button
                    type={canSaveInviteeRate ? 'primary' : 'tertiary'}
                    theme={canSaveInviteeRate ? 'solid' : 'borderless'}
                    size='small'
                    disabled={!canSaveInviteeRate}
                    loading={inviteeRateSaving}
                    onClick={onInviteeRateSave}
                    className='!rounded-lg'
                  >
                    {t('保存')}
                  </Button>
                </div>
              </div>
              <InputNumber
                value={rateBpsToPercentNumber(inviteeRateBps)}
                min={0}
                max={rateBpsToPercentNumber(maxInviteeRateBps)}
                step={0.01}
                precision={2}
                suffix='%'
                disabled={!hasInviteeRateCapacity}
                className='w-full'
                style={{ width: '100%' }}
                onChange={onInviteeRateChange}
              />
            </div>

            <div className='min-w-0 rounded-xl border border-[rgba(var(--semi-grey-2),1)] bg-[rgba(var(--semi-grey-0),.55)] px-4 py-3'>
              <div className='mb-2 flex items-center justify-between gap-3'>
                <Text strong className='whitespace-nowrap text-sm'>
                  {t('邀请链接')}
                </Text>
                <Button
                  type='primary'
                  theme='solid'
                  size='small'
                  onClick={handleAffLinkClick}
                  icon={<Copy size={14} />}
                  className='!rounded-lg'
                >
                  {t('复制')}
                </Button>
              </div>
              <Input value={affLink} readOnly className='!rounded-lg' />
            </div>
          </div>
        </Card>

        {/* 奖励说明 */}
        <Card
          className='!rounded-xl w-full'
          title={<Text type='tertiary'>{t('奖励说明')}</Text>}
        >
          <div className='space-y-3'>
            <div className='flex items-start gap-2'>
              <Badge dot type='success' />
              <Text type='tertiary' className='text-sm'>
                {t('被邀请人订阅支付成功后，邀请人和被邀请人可按规则获得奖励')}
              </Text>
            </div>

            <div className='flex items-start gap-2'>
              <Badge dot type='success' />
              <Text type='tertiary' className='text-sm'>
                {t('被邀请人获得多少，由邀请人当前设置决定')}
              </Text>
            </div>

            <div className='flex items-start gap-2'>
              <Badge dot type='success' />
              <Text type='tertiary' className='text-sm'>
                {t('通过划转功能将奖励额度转入到您的账户余额中')}
              </Text>
            </div>

            <div className='flex items-start gap-2'>
              <Badge dot type='success' />
              <Text type='tertiary' className='text-sm'>
                {t('邀请的被邀请人越多，获得的奖励越多')}
              </Text>
            </div>
          </div>
        </Card>
      </Space>
    </Card>
  );
};

export default InvitationCard;
