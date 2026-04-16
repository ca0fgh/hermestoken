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
import { Card, Empty, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError } from '../../../../helpers';
import {
  buildAdminOverrideRows,
  buildReferralRateSummary,
  formatRateBpsPercent,
} from '../../../../helpers/subscriptionReferral';

const { Text } = Typography;

const buildGroupDefaultRates = (groups = []) =>
  groups.reduce((rateMap, groupItem) => {
    const group = String(groupItem?.group || '').trim();
    if (!group) {
      return rateMap;
    }

    return {
      ...rateMap,
      [group]: Number(
        groupItem?.effective_total_rate_bps ??
          groupItem?.effectiveTotalRateBps ??
          0,
      ),
    };
  }, {});

const SubscriptionReferralOverrideSection = ({ userId }) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [groupDefaultRates, setGroupDefaultRates] = useState({});
  const [overrideRows, setOverrideRows] = useState([]);

  const getDefaultRateBpsByGroup = (group, fallbackRateBps = 0) => {
    const normalizedGroup = String(group || '').trim();
    if (!normalizedGroup) {
      return Number(fallbackRateBps || 0);
    }
    if (
      Object.prototype.hasOwnProperty.call(groupDefaultRates, normalizedGroup)
    ) {
      return Number(groupDefaultRates[normalizedGroup] || 0);
    }
    return Number(fallbackRateBps || 0);
  };

  const loadOverrides = async () => {
    if (!userId) return;
    setLoading(true);
    try {
      const [groupRes, userRes] = await Promise.all([
        API.get('/api/group/'),
        API.get(`/api/subscription/admin/referral/users/${userId}`),
      ]);

      if (!groupRes.data?.success) {
        showError(groupRes.data?.message || t('加载失败'));
        return;
      }
      if (!userRes.data?.success) {
        showError(userRes.data?.message || t('加载失败'));
        return;
      }

      const next = userRes.data?.data || {};
      const responseGroups = Array.isArray(next.groups) ? next.groups : [];
      const persistedOverrideRows = buildAdminOverrideRows(
        Array.isArray(next.groups) ? next.groups : [],
      ).filter((row) => row.hasOverride);
      setGroupDefaultRates(buildGroupDefaultRates(responseGroups));
      setOverrideRows(persistedOverrideRows);
    } catch (error) {
      showError(t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadOverrides().then();
  }, [userId]);

  return (
    <Card className='!rounded-2xl shadow-sm border-0'>
      <div className='flex flex-col gap-3'>
        <div className='flex items-center justify-between gap-3'>
          <div>
            <Text className='text-lg font-medium'>{t('邀请人返佣覆盖')}</Text>
            <div className='text-xs text-gray-600'>
              {t('暂无覆盖时使用授权返佣规则')}
            </div>
          </div>
        </div>

        {overrideRows.length === 0 ? (
          <div className='rounded-xl border border-dashed border-gray-200 py-8'>
            <Empty
              title={t('暂无覆盖项，未设置时使用授权返佣规则')}
              description=''
            />
          </div>
        ) : (
          <div className='flex flex-col gap-3'>
            {overrideRows.map((row) => {
              const summary = buildReferralRateSummary(
                row.effectiveTotalRateBps,
                row.overrideRateBps ?? row.inputPercent,
                row.group,
              );
              return (
                <div
                  key={row.id}
                  className='rounded-xl border border-gray-200 p-4'
                >
                  <div className='grid grid-cols-1 gap-3 md:grid-cols-3'>
                    <div className='min-w-0'>
                      <Text type='tertiary' className='text-xs block mb-2'>
                        {t('总返佣')}
                      </Text>
                      <Text strong>
                        {formatRateBpsPercent(summary.totalRateBps)}
                      </Text>
                    </div>
                    <div className='min-w-0'>
                      <Text type='tertiary' className='text-xs block mb-2'>
                        {t('邀请人返佣')}
                      </Text>
                      <Text strong>
                        {formatRateBpsPercent(summary.inviterRateBps)}
                      </Text>
                    </div>
                    <div className='min-w-0 lg:col-span-2'>
                      <Text type='tertiary' className='text-xs block mb-2'>
                        {t('被邀请人返佣')}
                      </Text>
                      <Text strong>
                        {formatRateBpsPercent(summary.inviteeRateBps)}
                      </Text>
                    </div>
                  </div>

                  <div className='mt-2 text-xs text-gray-500'>
                    {`${t('当前授权总返佣')} ${formatRateBpsPercent(
                      getDefaultRateBpsByGroup(
                        row.group,
                        row.effectiveTotalRateBps,
                      ),
                    )}`}
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </Card>
  );
};

export default SubscriptionReferralOverrideSection;
