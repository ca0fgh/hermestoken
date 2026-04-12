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

import React, { useEffect, useState } from 'react';
import { Button, Card, InputNumber, Space, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../../../helpers';
import {
  buildAdminOverrideRows,
  formatRateBpsPercent,
  percentNumberToRateBps,
} from '../../../../helpers/subscriptionReferral';

const { Text } = Typography;

const SubscriptionReferralOverrideSection = ({ userId }) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [savingGroup, setSavingGroup] = useState('');
  const [overrideRows, setOverrideRows] = useState([]);

  const loadOverride = async () => {
    if (!userId) return;
    setLoading(true);
    try {
      const res = await API.get(`/api/subscription/admin/referral/users/${userId}`);
      if (res.data?.success) {
        const next = res.data.data || {};
        const fallbackGroups = next.group
          ? [
              {
                group: next.group,
                effective_total_rate_bps: next.effective_total_rate_bps,
                has_override: next.has_override,
                override_rate_bps: next.override_rate_bps,
              },
            ]
          : [];
        setOverrideRows(
          buildAdminOverrideRows(
            Array.isArray(next.groups) && next.groups.length > 0
              ? next.groups
              : fallbackGroups,
          ),
        );
      } else {
        showError(res.data?.message || t('加载失败'));
      }
    } catch (error) {
      showError(t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadOverride().then();
  }, [userId]);

  const updateInputPercent = (group, value) => {
    setOverrideRows((currentRows) =>
      currentRows.map((row) =>
        row.group === group
          ? {
              ...row,
              inputPercent: Number(value || 0),
            }
          : row,
      ),
    );
  };

  const saveOverride = async (group) => {
    const targetRow = overrideRows.find((row) => row.group === group);
    if (!targetRow) {
      return;
    }

    setSavingGroup(group);
    try {
      const totalRateBps = percentNumberToRateBps(targetRow.inputPercent);
      const res = await API.put(`/api/subscription/admin/referral/users/${userId}`, {
        group,
        total_rate_bps: totalRateBps,
      });
      if (res.data?.success) {
        showSuccess(t('保存成功'));
        await loadOverride();
      } else {
        showError(res.data?.message || t('保存失败，请重试'));
      }
    } catch (error) {
      showError(t('保存失败，请重试'));
    } finally {
      setSavingGroup('');
    }
  };

  const clearOverride = async (group) => {
    setSavingGroup(group);
    try {
      const res = await API.delete(
        `/api/subscription/admin/referral/users/${userId}`,
        {
          params: { group },
        },
      );
      if (res.data?.success) {
        showSuccess(t('已清除该分组覆盖'));
        await loadOverride();
      } else {
        showError(res.data?.message || t('保存失败，请重试'));
      }
    } catch (error) {
      showError(t('保存失败，请重试'));
    } finally {
      setSavingGroup('');
    }
  };

  return (
    <Card className='!rounded-2xl shadow-sm border-0'>
      <div className='flex flex-col gap-3'>
        <div>
          <Text className='text-lg font-medium'>{t('邀请人返佣覆盖')}</Text>
          <div className='text-xs text-gray-600'>
            {t('未设置覆盖时使用对应分组默认总返佣率')}
          </div>
        </div>

        <div className='flex flex-col gap-3'>
          {overrideRows.map((row) => {
            const isSaving = savingGroup === row.group;
            return (
              <div key={row.group} className='rounded-xl border border-gray-100 p-4'>
                <div className='flex flex-col gap-3'>
                  <div className='flex items-center justify-between gap-3'>
                    <div>
                      <Text strong>{row.group}</Text>
                      <div className='text-xs text-gray-600'>
                        {row.hasOverride
                          ? t('已设置单独覆盖')
                          : t('当前使用该分组默认总返佣率')}
                      </div>
                    </div>
                    <div className='text-right'>
                      <Text type='tertiary' className='text-xs block mb-1'>
                        {t('当前生效总返佣率')}
                      </Text>
                      <Text strong>
                        {formatRateBpsPercent(row.effectiveTotalRateBps)}
                      </Text>
                    </div>
                  </div>

                  <div>
                    <Text type='tertiary' className='text-xs block mb-2'>
                      {t('覆盖总返佣率')}
                    </Text>
                    <InputNumber
                      value={row.inputPercent}
                      min={0}
                      max={100}
                      step={0.01}
                      precision={2}
                      suffix='%'
                      style={{ width: '100%' }}
                      disabled={loading || isSaving}
                      onChange={(value) => updateInputPercent(row.group, value)}
                    />
                  </div>

                  <Space>
                    <Button
                      type='primary'
                      theme='solid'
                      loading={isSaving}
                      disabled={loading}
                      onClick={() => saveOverride(row.group)}
                    >
                      {t('保存订阅返佣设置')}
                    </Button>
                    <Button
                      theme='light'
                      disabled={!row.hasOverride || loading || isSaving}
                      onClick={() => clearOverride(row.group)}
                    >
                      {t('清除覆盖')}
                    </Button>
                  </Space>
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </Card>
  );
};

export default SubscriptionReferralOverrideSection;
