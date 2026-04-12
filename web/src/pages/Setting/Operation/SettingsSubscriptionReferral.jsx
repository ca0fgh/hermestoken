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

import React, { useEffect, useRef, useState } from 'react';
import { Button, Col, Form, Row, Spin } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess, showWarning } from '../../../helpers';
import {
  buildAdminReferralFormValues,
  normalizeAdminReferralPayload,
  parseAdminReferralSettings,
  percentNumberToRateBps,
} from '../../../helpers/subscriptionReferral';

export default function SettingsSubscriptionReferral() {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [enabled, setEnabled] = useState(false);
  const [totalRatePercent, setTotalRatePercent] = useState(0);
  const [snapshot, setSnapshot] = useState({ enabled: false, totalRateBps: 0 });
  const refForm = useRef(null);

  const formValues = buildAdminReferralFormValues({
    enabled,
    totalRatePercent,
  });

  const applySettings = (payload) => {
    const nextSettings = parseAdminReferralSettings(payload);
    setEnabled(nextSettings.enabled);
    setTotalRatePercent(nextSettings.totalRatePercent);
    setSnapshot({
      enabled: nextSettings.enabled,
      totalRateBps: nextSettings.totalRateBps,
    });
  };

  useEffect(() => {
    // Semi Form field components read from form store first, so we must keep
    // the form API in sync with the externally loaded settings.
    refForm.current?.setValues(formValues);
  }, [enabled, totalRatePercent]);

  const loadSettings = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/subscription/admin/referral/settings');
      if (res.data?.success) {
        applySettings(res.data?.data || {});
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
    loadSettings().then();
  }, []);

  const onSubmit = async () => {
    const nextRateBps = percentNumberToRateBps(totalRatePercent);
    if (enabled === snapshot.enabled && nextRateBps === snapshot.totalRateBps) {
      return showWarning(t('你似乎并没有修改什么'));
    }

    setLoading(true);
    try {
      const payload = normalizeAdminReferralPayload({
        enabled,
        totalRateBps: nextRateBps,
      });
      const res = await API.put('/api/subscription/admin/referral/settings', {
        enabled: payload.enabled,
        total_rate_bps: payload.totalRateBps,
      });
      if (res.data?.success) {
        applySettings(res.data?.data || {});
        showSuccess(t('保存成功'));
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
    <Spin spinning={loading}>
      <Form
        values={formValues}
        getFormApi={(formApi) => {
          refForm.current = formApi;
        }}
        style={{ marginBottom: 15 }}
      >
        <Form.Section text={t('订阅返佣设置')}>
          <Row gutter={16}>
            <Col xs={24} sm={12} md={10}>
              <Form.Switch
                field='SubscriptionReferralEnabled'
                label={t('启用订阅返佣')}
                checked={enabled}
                onChange={(value) => setEnabled(value)}
              />
            </Col>
            <Col xs={24} sm={12} md={10}>
              <Form.InputNumber
                field='SubscriptionReferralGlobalRateBps'
                label={t('全局总返佣率')}
                value={totalRatePercent}
                min={0}
                max={100}
                step={0.01}
                precision={2}
                suffix='%'
                onChange={(value) => setTotalRatePercent(Number(value || 0))}
              />
            </Col>
          </Row>
          <Row>
            <Button size='default' onClick={onSubmit}>
              {t('保存订阅返佣设置')}
            </Button>
          </Row>
        </Form.Section>
      </Form>
    </Spin>
  );
}
