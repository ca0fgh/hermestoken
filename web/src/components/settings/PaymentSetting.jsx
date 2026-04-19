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

import React, { Suspense, useEffect, useState } from 'react';
import { Card, Spin } from '@douyinfe/semi-ui';
import { lazyWithRetry } from '../../helpers/lazyWithRetry';
import { API, showError, toBoolean } from '../../helpers';
import { useTranslation } from 'react-i18next';

const SettingsGeneralPayment = lazyWithRetry(
  () => import('../../pages/Setting/Payment/SettingsGeneralPayment'),
  'settings-general-payment-card',
);
const SettingsPaymentGateway = lazyWithRetry(
  () => import('../../pages/Setting/Payment/SettingsPaymentGateway'),
  'settings-payment-gateway-card',
);
const SettingsPaymentGatewayStripe = lazyWithRetry(
  () => import('../../pages/Setting/Payment/SettingsPaymentGatewayStripe'),
  'settings-payment-stripe-card',
);
const SettingsPaymentGatewayCreem = lazyWithRetry(
  () => import('../../pages/Setting/Payment/SettingsPaymentGatewayCreem'),
  'settings-payment-creem-card',
);
const SettingsPaymentGatewayWaffo = lazyWithRetry(
  () => import('../../pages/Setting/Payment/SettingsPaymentGatewayWaffo'),
  'settings-payment-waffo-card',
);
const SettingsWithdrawal = lazyWithRetry(
  () => import('../../pages/Setting/Payment/SettingsWithdrawal'),
  'settings-withdrawal-card',
);

function renderSection(content) {
  return <Suspense fallback={null}>{content}</Suspense>;
}

const PaymentSetting = () => {
  const { t } = useTranslation();
  let [inputs, setInputs] = useState({
    ServerAddress: '',
    PayAddress: '',
    EpayId: '',
    EpayKey: '',
    EpayEnabled: true,
    Price: 7.3,
    MinTopUp: 1,
    TopupGroupRatio: '',
    CustomCallbackAddress: '',
    PayMethods: '',
    AmountOptions: '',
    AmountDiscount: '',

    StripeApiSecret: '',
    StripeWebhookSecret: '',
    StripePriceId: '',
    StripeEnabled: true,
    StripeUnitPrice: 8.0,
    StripeMinTopUp: 1,
    StripePromotionCodesEnabled: false,
    CreemEnabled: true,
  });

  let [loading, setLoading] = useState(false);

  const getOptions = async () => {
    const res = await API.get('/api/option/');
    const { success, message, data } = res.data;
    if (success) {
      let newInputs = {};
      data.forEach((item) => {
        switch (item.key) {
          case 'TopupGroupRatio':
            try {
              newInputs[item.key] = JSON.stringify(
                JSON.parse(item.value),
                null,
                2,
              );
            } catch (error) {
              newInputs[item.key] = item.value;
            }
            break;
          case 'payment_setting.amount_options':
            try {
              newInputs['AmountOptions'] = JSON.stringify(
                JSON.parse(item.value),
                null,
                2,
              );
            } catch (error) {
              newInputs['AmountOptions'] = item.value;
            }
            break;
          case 'payment_setting.amount_discount':
            try {
              newInputs['AmountDiscount'] = JSON.stringify(
                JSON.parse(item.value),
                null,
                2,
              );
            } catch (error) {
              newInputs['AmountDiscount'] = item.value;
            }
            break;
          case 'Price':
          case 'MinTopUp':
          case 'StripeUnitPrice':
          case 'StripeMinTopUp':
            newInputs[item.key] = parseFloat(item.value);
            break;
          default:
            if (item.key.endsWith('Enabled')) {
              newInputs[item.key] = toBoolean(item.value);
            } else {
              newInputs[item.key] = item.value;
            }
            break;
        }
      });
      setInputs((prev) => ({ ...prev, ...newInputs }));
    } else {
      showError(t(message));
    }
  };

  async function onRefresh() {
    try {
      setLoading(true);
      await getOptions();
    } catch (error) {
      showError(t('刷新失败'));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    onRefresh();
  }, []);

  return (
    <>
      <Spin spinning={loading} size='large'>
        <Card style={{ marginTop: '10px' }}>
          {renderSection(
            <SettingsGeneralPayment options={inputs} refresh={onRefresh} />,
          )}
        </Card>
        <Card style={{ marginTop: '10px' }}>
          {renderSection(
            <SettingsPaymentGateway options={inputs} refresh={onRefresh} />,
          )}
        </Card>
        <Card style={{ marginTop: '10px' }}>
          {renderSection(
            <SettingsPaymentGatewayStripe options={inputs} refresh={onRefresh} />,
          )}
        </Card>
        <Card style={{ marginTop: '10px' }}>
          {renderSection(
            <SettingsPaymentGatewayCreem options={inputs} refresh={onRefresh} />,
          )}
        </Card>
        <Card style={{ marginTop: '10px' }}>
          {renderSection(
            <SettingsPaymentGatewayWaffo options={inputs} refresh={onRefresh} />,
          )}
        </Card>
        <Card style={{ marginTop: '10px' }}>
          {renderSection(
            <SettingsWithdrawal options={inputs} refresh={onRefresh} />,
          )}
        </Card>
      </Spin>
    </>
  );
};

export default PaymentSetting;
