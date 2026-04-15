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

import React, { useMemo, useState } from 'react';
import {
  Button,
  Card,
  Empty,
  InputNumber,
  Space,
  Tag,
  Typography,
} from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../helpers';
import {
  clampInviteeRateBps,
  formatRateBpsPercent,
  percentNumberToRateBps,
  rateBpsToPercentNumber,
} from '../../helpers/subscriptionReferral';

const InviteeOverridePanel = ({
  t,
  invitee = null,
  rows = [],
  loading = false,
  onOverridesChanged,
}) => {
  const [draftPercentByGroup, setDraftPercentByGroup] = useState({});
  const [savingGroup, setSavingGroup] = useState('');
  const [deletingGroup, setDeletingGroup] = useState('');

  const normalizedRows = useMemo(() => rows || [], [rows]);

  const getDraftPercent = (row) => {
    const group = String(row?.group || '').trim();
    if (Object.prototype.hasOwnProperty.call(draftPercentByGroup, group)) {
      return draftPercentByGroup[group];
    }
    return Number(row?.inputPercent || 0);
  };

  const updateDraftPercent = (group, value) => {
    setDraftPercentByGroup((currentDrafts) => ({
      ...currentDrafts,
      [group]: Number(value || 0),
    }));
  };

  const saveOverride = async (row) => {
    const group = String(row?.group || '').trim();
    const inviteeRateBps = clampInviteeRateBps(
      percentNumberToRateBps(getDraftPercent(row)),
      row?.effectiveTotalRateBps,
    );

    setSavingGroup(group);
    try {
      const res = await API.put(
        `/api/user/referral/subscription/invitees/${invitee.id}`,
        {
          group,
          invitee_rate_bps: inviteeRateBps,
        },
      );

      if (res.data?.success) {
        showSuccess(t('保存成功'));
        await onOverridesChanged?.();
      } else {
        showError(res.data?.message || t('保存失败'));
      }
    } catch (error) {
      showError(error?.message || t('保存失败'));
    } finally {
      setSavingGroup('');
    }
  };

  const deleteOverride = async (group) => {
    setDeletingGroup(group);
    try {
      const res = await API.delete(
        `/api/user/referral/subscription/invitees/${invitee.id}`,
        {
          params: { group },
        },
      );

      if (res.data?.success) {
        showSuccess(t('删除成功'));
        setDraftPercentByGroup((currentDrafts) => {
          const nextDrafts = { ...currentDrafts };
          delete nextDrafts[group];
          return nextDrafts;
        });
        await onOverridesChanged?.();
      } else {
        showError(res.data?.message || t('删除失败'));
      }
    } catch (error) {
      showError(error?.message || t('删除失败'));
    } finally {
      setDeletingGroup('');
    }
  };

  if (!invitee) {
    return (
      <Card
        className='!rounded-2xl border-0 shadow-sm h-full'
        title={t('邀请用户独立返佣')}
      >
        <Empty description={t('未选择邀请用户')} />
      </Card>
    );
  }

  return (
    <Card
      className='!rounded-2xl border-0 shadow-sm h-full'
      title={t('邀请用户独立返佣')}
      loading={loading}
      bodyStyle={{ display: 'flex', flexDirection: 'column', gap: 16 }}
    >
      <div className='rounded-2xl bg-gray-50 p-4'>
        <div className='flex items-center justify-between gap-3'>
          <Typography.Title heading={5} style={{ margin: 0 }}>
            {invitee.username || `#${invitee.id}`}
          </Typography.Title>
          <Tag color='white'>{invitee.group || '-'}</Tag>
        </div>
        <Typography.Text type='tertiary' className='mt-2 block text-sm'>
          {t('未设置独立返佣时，使用默认规则')}
        </Typography.Text>
      </div>

      {normalizedRows.length === 0 ? (
        <Empty description={t('暂无覆盖项，未设置时使用默认返佣规则')} />
      ) : (
        normalizedRows.map((row) => {
          const group = String(row?.group || '').trim();
          const inputPercent = getDraftPercent(row);
          const currentPercent = Number(row?.inputPercent || 0);
          const canSave =
            !savingGroup &&
            percentNumberToRateBps(inputPercent) !==
              percentNumberToRateBps(currentPercent);

          return (
            <div
              key={row.id}
              className='rounded-2xl border border-gray-100 bg-white p-4'
            >
              <div className='grid grid-cols-1 gap-4 md:grid-cols-3'>
                <div>
                  <Typography.Text type='tertiary' className='block text-xs'>
                    {t('分组')}
                  </Typography.Text>
                  <Typography.Text strong>{group}</Typography.Text>
                </div>
                <div>
                  <Typography.Text type='tertiary' className='block text-xs'>
                    {t('当前默认返佣率')}
                  </Typography.Text>
                  <Typography.Text strong>
                    {formatRateBpsPercent(row.defaultInviteeRateBps)}
                  </Typography.Text>
                </div>
                <div>
                  <Typography.Text type='tertiary' className='block text-xs'>
                    {t('当前默认总返佣率')}
                  </Typography.Text>
                  <Typography.Text strong>
                    {formatRateBpsPercent(row.effectiveTotalRateBps)}
                  </Typography.Text>
                </div>
              </div>

              <div className='mt-4'>
                <Typography.Text type='tertiary' className='block text-xs'>
                  {t('被邀请人返佣比例')}
                </Typography.Text>
                <InputNumber
                  value={inputPercent}
                  min={0}
                  max={rateBpsToPercentNumber(row.effectiveTotalRateBps)}
                  step={0.01}
                  precision={2}
                  suffix='%'
                  style={{ width: '100%' }}
                  onChange={(value) => updateDraftPercent(group, value)}
                />
              </div>

              <Space className='mt-4'>
                <Button
                  type='primary'
                  loading={savingGroup === group}
                  disabled={!canSave}
                  onClick={() => saveOverride(row)}
                >
                  {t('保存')}
                </Button>
                <Button
                  theme='borderless'
                  type='danger'
                  disabled={!row.hasOverride}
                  loading={deletingGroup === group}
                  onClick={() => deleteOverride(group)}
                >
                  {t('删除')}
                </Button>
              </Space>
            </div>
          );
        })
      )}
    </Card>
  );
};

export default InviteeOverridePanel;
