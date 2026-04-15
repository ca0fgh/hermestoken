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

import React, { useState } from 'react';
import {
  Button,
  Card,
  Empty,
  InputNumber,
  Space,
  Typography,
} from '@douyinfe/semi-ui';
import { API, showError, showSuccess } from '../../helpers';
import {
  clampInviteeRateBps,
  formatRateBpsPercent,
  percentNumberToRateBps,
  rateBpsToPercentNumber,
} from '../../helpers/subscriptionReferral';

const InviteDefaultRuleSection = ({
  t,
  rows = [],
  loading = false,
  onRulesChanged,
}) => {
  const getTypeLabel = (type) => {
    if (type === 'subscription') return t('订阅返佣');
    return type || t('未知类型');
  };

  const [savingGroup, setSavingGroup] = useState('');
  const [deletingGroup, setDeletingGroup] = useState('');
  const [draftPercentByGroup, setDraftPercentByGroup] = useState({});

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

  const saveRow = async (row) => {
    const group = String(row?.group || '').trim();
    const inviteeRateBps = clampInviteeRateBps(
      percentNumberToRateBps(getDraftPercent(row)),
      row?.effectiveTotalRateBps,
    );

    setSavingGroup(group);
    try {
      const res = await API.put('/api/user/referral/subscription', {
        group,
        invitee_rate_bps: inviteeRateBps,
      });

      if (res.data?.success) {
        showSuccess(t('保存成功'));
        await onRulesChanged?.();
      } else {
        showError(res.data?.message || t('保存失败'));
      }
    } catch (error) {
      showError(error?.message || t('保存失败'));
    } finally {
      setSavingGroup('');
    }
  };

  const deleteRow = async (group) => {
    setDeletingGroup(group);
    try {
      const res = await API.delete('/api/user/referral/subscription', {
        params: { group },
      });

      if (res.data?.success) {
        showSuccess(t('删除成功'));
        setDraftPercentByGroup((currentDrafts) => {
          const nextDrafts = { ...currentDrafts };
          delete nextDrafts[group];
          return nextDrafts;
        });
        await onRulesChanged?.();
      } else {
        showError(res.data?.message || t('删除失败'));
      }
    } catch (error) {
      showError(error?.message || t('删除失败'));
    } finally {
      setDeletingGroup('');
    }
  };

  return (
    <Card
      className='!rounded-2xl border-0 shadow-sm'
      title={t('默认返佣规则')}
      loading={loading}
    >
      <div className='flex flex-col gap-4'>
        {rows.length === 0 ? (
          <Empty description={t('暂无覆盖时使用默认返佣规则')} />
        ) : (
          rows.map((row) => {
            const group = String(row?.group || '').trim();
            const inputPercent = getDraftPercent(row);
            const canSave =
              !savingGroup &&
              percentNumberToRateBps(inputPercent) !==
                percentNumberToRateBps(row?.inputPercent || 0);

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
                      {t('当前默认总返佣率')}
                    </Typography.Text>
                    <Typography.Text strong>
                      {formatRateBpsPercent(row.effectiveTotalRateBps)}
                    </Typography.Text>
                  </div>
                  <div>
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
                </div>

                <Space className='mt-4'>
                  <Button
                    type='primary'
                    loading={savingGroup === group}
                    disabled={!canSave}
                    onClick={() => saveRow(row)}
                  >
                    {t('保存')}
                  </Button>
                  <Button
                    type='danger'
                    theme='borderless'
                    loading={deletingGroup === group}
                    onClick={() => deleteRow(group)}
                  >
                    {t('删除')}
                  </Button>
                </Space>
              </div>
            );
          })
        )}
      </div>
    </Card>
  );
};

export default InviteDefaultRuleSection;
