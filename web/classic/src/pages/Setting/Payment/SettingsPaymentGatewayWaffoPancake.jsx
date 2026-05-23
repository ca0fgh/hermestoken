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

import React, { useEffect, useRef, useState } from 'react';
import { Banner, Button, Col, Form, Row, Spin } from '@douyinfe/semi-ui';
import {
  API,
  removeTrailingSlash,
  showError,
  showSuccess,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';
import { BookOpen } from 'lucide-react';

const defaultInputs = {
  WaffoPancakeMerchantID: '',
  WaffoPancakePrivateKey: '',
  WaffoPancakeReturnURL: '',
  WaffoPancakeStoreID: '',
  WaffoPancakeProductID: '',
};

export default function SettingsPaymentGatewayWaffoPancake(props) {
  const { t } = useTranslation();
  const sectionTitle = props.hideSectionTitle
    ? undefined
    : t('Waffo Pancake 设置');
  const [loading, setLoading] = useState(false);
  const [inputs, setInputs] = useState(defaultInputs);
  const formApiRef = useRef(null);

  useEffect(() => {
    if (!props.options || !formApiRef.current) return;

    const currentInputs = {
      WaffoPancakeMerchantID: props.options.WaffoPancakeMerchantID || '',
      WaffoPancakePrivateKey: props.options.WaffoPancakePrivateKey || '',
      WaffoPancakeReturnURL: props.options.WaffoPancakeReturnURL || '',
      WaffoPancakeStoreID: props.options.WaffoPancakeStoreID || '',
      WaffoPancakeProductID: props.options.WaffoPancakeProductID || '',
    };

    setInputs(currentInputs);
    formApiRef.current.setValues(currentInputs);
  }, [props.options]);

  const handleFormChange = (values) => {
    setInputs(values);
  };

  const submitWaffoPancakeSetting = async () => {
    const values = {
      ...inputs,
      ...(formApiRef.current?.getValues?.() || {}),
    };

    setLoading(true);
    try {
      const res = await API.post('/api/option/waffo-pancake/save', {
        merchant_id: values.WaffoPancakeMerchantID || '',
        private_key: values.WaffoPancakePrivateKey || '',
        return_url: removeTrailingSlash(values.WaffoPancakeReturnURL || ''),
        store_id: values.WaffoPancakeStoreID || '',
        product_id: values.WaffoPancakeProductID || '',
      });

      if (res?.data?.message !== 'success') {
        showError(res?.data?.data || t('更新失败'));
        return;
      }

      showSuccess(t('更新成功'));
      props.refresh?.();
    } catch (error) {
      showError(t('更新失败'));
    } finally {
      setLoading(false);
    }
  };

  const createWaffoPancakePair = async () => {
    const values = {
      ...inputs,
      ...(formApiRef.current?.getValues?.() || {}),
    };
    if (
      !(values.WaffoPancakeMerchantID || '').trim() ||
      !(values.WaffoPancakePrivateKey || '').trim()
    ) {
      showError(t('请先填写商户 ID 和 API 私钥'));
      return;
    }

    setLoading(true);
    try {
      const res = await API.post('/api/option/waffo-pancake/pair', {
        merchant_id: values.WaffoPancakeMerchantID,
        private_key: values.WaffoPancakePrivateKey,
        return_url: removeTrailingSlash(values.WaffoPancakeReturnURL || ''),
      });
      if (res?.data?.message !== 'success' || !res.data.data) {
        const data = res?.data?.data;
        showError(
          typeof data === 'string'
            ? data
            : data?.error || t('创建 Store + Product 失败'),
        );
        if (data?.orphan_store && data.store_id) {
          formApiRef.current?.setValue('WaffoPancakeStoreID', data.store_id);
        }
        return;
      }

      const created = res.data.data;
      const nextValues = {
        ...values,
        WaffoPancakeStoreID: created.store_id || '',
        WaffoPancakeProductID: created.product_id || '',
      };
      setInputs(nextValues);
      formApiRef.current?.setValues(nextValues);
      showSuccess(t('Store + Product 创建成功'));
    } catch (error) {
      showError(t('创建 Store + Product 失败'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Spin spinning={loading}>
      <Form
        initValues={inputs}
        onValueChange={handleFormChange}
        getFormApi={(api) => (formApiRef.current = api)}
      >
        <Form.Section text={sectionTitle}>
          <Banner
            type='info'
            icon={<BookOpen size={16} />}
            description={
              <>
                Waffo Pancake 商户 ID 与私钥请在
                <a
                  href='https://pancake.waffo.ai/merchant/dashboard'
                  target='_blank'
                  rel='noreferrer'
                >
                  Waffo Pancake 控制台
                </a>
                获取。你可以手动填写 Store / Product
                ID，或点击下方按钮自动创建； 环境（test / 生产）由你粘贴的 API
                私钥本身决定。 请在 Pancake 控制台把下面两个回调地址分别注册到
                Test Mode 和 Production Mode 两个 webhook
                位置，分开走避免测试流量污染生产数据：
                <br />
                {t('Test 回调地址')}：
                {props.options.ServerAddress
                  ? removeTrailingSlash(props.options.ServerAddress)
                  : t('网站地址')}
                /api/waffo-pancake/webhook/test
                <br />
                {t('Production 回调地址')}：
                {props.options.ServerAddress
                  ? removeTrailingSlash(props.options.ServerAddress)
                  : t('网站地址')}
                /api/waffo-pancake/webhook/prod
              </>
            }
            style={{ marginBottom: 12 }}
          />
          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.Input
                field='WaffoPancakeMerchantID'
                label={t('商户 ID')}
                placeholder={t('例如：MER_xxx')}
              />
            </Col>
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.Input
                field='WaffoPancakeReturnURL'
                label={t('支付返回地址')}
                placeholder={t('例如：https://example.com/console/topup')}
              />
            </Col>
          </Row>

          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.Input
                field='WaffoPancakeStoreID'
                label={t('Store ID')}
                placeholder={t('例如：STO_xxx')}
              />
            </Col>
            <Col xs={24} sm={24} md={12} lg={12} xl={12}>
              <Form.Input
                field='WaffoPancakeProductID'
                label={t('钱包充值 Product ID')}
                placeholder={t('例如：PROD_xxx')}
              />
            </Col>
          </Row>

          <Row
            gutter={{ xs: 8, sm: 16, md: 24, lg: 24, xl: 24, xxl: 24 }}
            style={{ marginTop: 16 }}
          >
            <Col xs={24}>
              <Form.TextArea
                field='WaffoPancakePrivateKey'
                label={t('API 私钥')}
                placeholder={t('填写后覆盖当前私钥，留空表示保持当前不变')}
                extraText={t(
                  '⚠ 测试 / 生产环境由你粘进来的 API 私钥本身决定——集成阶段用 Test Key，正式上线时再换成 Production Key',
                )}
                type='password'
                autosize={{ minRows: 4, maxRows: 8 }}
              />
            </Col>
          </Row>

          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <Button onClick={createWaffoPancakePair} theme='light'>
              {t('创建 Store + Product')}
            </Button>
            <Button onClick={submitWaffoPancakeSetting}>
              {t('更新 Waffo Pancake 设置')}
            </Button>
          </div>
        </Form.Section>
      </Form>
    </Spin>
  );
}
