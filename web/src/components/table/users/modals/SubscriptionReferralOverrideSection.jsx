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
  formatRateBpsPercent,
  percentNumberToRateBps,
  rateBpsToPercentNumber,
} from '../../../../helpers/subscriptionReferral';

const { Text } = Typography;

const SubscriptionReferralOverrideSection = ({ userId }) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState({
    hasOverride: false,
    overrideRateBps: null,
    effectiveTotalRateBps: 0,
  });
  const [inputPercent, setInputPercent] = useState(0);

  const loadOverride = async () => {
    if (!userId) return;
    setLoading(true);
    try {
      const res = await API.get(`/api/subscription/admin/referral/users/${userId}`);
      if (res.data?.success) {
        const next = res.data.data || {};
        setData({
          hasOverride: Boolean(next.has_override),
          overrideRateBps: next.override_rate_bps,
          effectiveTotalRateBps: next.effective_total_rate_bps || 0,
        });
        setInputPercent(rateBpsToPercentNumber(next.override_rate_bps || next.effective_total_rate_bps || 0));
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

  const saveOverride = async () => {
    setLoading(true);
    try {
      const totalRateBps = percentNumberToRateBps(inputPercent);
      const res = await API.put(`/api/subscription/admin/referral/users/${userId}`, {
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
      setLoading(false);
    }
  };

  const clearOverride = async () => {
    setLoading(true);
    try {
      const res = await API.delete(`/api/subscription/admin/referral/users/${userId}`);
      if (res.data?.success) {
        showSuccess(t('已恢复为全局总返佣率'));
        await loadOverride();
      } else {
        showError(res.data?.message || t('保存失败，请重试'));
      }
    } catch (error) {
      showError(t('保存失败，请重试'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Card className='!rounded-2xl shadow-sm border-0'>
      <div className='flex flex-col gap-3'>
        <div>
          <Text className='text-lg font-medium'>{t('邀请人返佣覆盖')}</Text>
          <div className='text-xs text-gray-600'>
            {t('未设置覆盖时使用全局总返佣率')}
          </div>
        </div>

        <div className='rounded-xl bg-gray-50 p-3'>
          <Text type='tertiary' className='text-xs block mb-1'>
            {t('当前生效总返佣率')}
          </Text>
          <Text strong>{formatRateBpsPercent(data.effectiveTotalRateBps)}</Text>
        </div>

        <div>
          <Text type='tertiary' className='text-xs block mb-2'>
            {t('覆盖总返佣率')}
          </Text>
          <InputNumber
            value={inputPercent}
            min={0}
            max={100}
            step={0.01}
            precision={2}
            suffix='%'
            style={{ width: '100%' }}
            onChange={(value) => setInputPercent(Number(value || 0))}
          />
        </div>

        <Space>
          <Button type='primary' theme='solid' loading={loading} onClick={saveOverride}>
            {t('保存订阅返佣设置')}
          </Button>
          <Button theme='light' disabled={!data.hasOverride || loading} onClick={clearOverride}>
            {t('清除覆盖')}
          </Button>
        </Space>
      </div>
    </Card>
  );
};

export default SubscriptionReferralOverrideSection;
