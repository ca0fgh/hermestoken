import React, { useEffect, useState } from 'react';
import { Button, Col, Form, Row, Spin } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess, showWarning } from '../../../helpers';
import {
  normalizeAdminReferralPayload,
  rateBpsToPercentNumber,
  percentNumberToRateBps,
} from '../../../helpers/subscriptionReferral';

export default function SettingsSubscriptionReferral(props) {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [enabled, setEnabled] = useState(false);
  const [totalRatePercent, setTotalRatePercent] = useState(0);
  const [snapshot, setSnapshot] = useState({ enabled: false, totalRateBps: 0 });

  useEffect(() => {
    const nextEnabled = Boolean(props.options?.SubscriptionReferralEnabled);
    const nextRateBps = Number(props.options?.SubscriptionReferralGlobalRateBps || 0);
    setEnabled(nextEnabled);
    setTotalRatePercent(rateBpsToPercentNumber(nextRateBps));
    setSnapshot({ enabled: nextEnabled, totalRateBps: nextRateBps });
  }, [props.options]);

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
        showSuccess(t('保存成功'));
        props.refresh?.();
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
      <Form style={{ marginBottom: 15 }}>
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
