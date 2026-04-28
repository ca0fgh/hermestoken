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
import { Button, Form, Row, Col, Spin, Typography } from '@douyinfe/semi-ui';
import {
  API,
  buildCryptoPaymentOptionUpdates,
  showError,
  showSuccess,
} from '../../../helpers';
import { useTranslation } from 'react-i18next';

const { Text } = Typography;

export default function SettingsPaymentGatewayCrypto(props) {
  const { t } = useTranslation();
  const { options, refresh } = props;
  const sectionTitle = props.hideSectionTitle ? undefined : t('USDT 设置');
  const [loading, setLoading] = useState(false);
  const formApiRef = useRef(null);
  const [inputs, setInputs] = useState({
    CryptoPaymentEnabled: false,
    CryptoScannerEnabled: true,
    CryptoOrderExpireMinutes: 10,
    CryptoUniqueSuffixMax: 9999,
    CryptoTronEnabled: false,
    CryptoTronReceiveAddress: '',
    CryptoTronUSDTContract: 'TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj',
    CryptoTronRPCURL: '',
    CryptoTronAPIKey: '',
    CryptoTronConfirmations: 20,
    CryptoBSCEnabled: false,
    CryptoBSCReceiveAddress: '',
    CryptoBSCUSDTContract: '0x55d398326f99059fF775485246999027B3197955',
    CryptoBSCRPCURL: '',
    CryptoBSCConfirmations: 15,
    CryptoPolygonEnabled: false,
    CryptoPolygonReceiveAddress: '',
    CryptoPolygonUSDTContract: '0xc2132D05D31c914a87C6611C10748AEb04B58e8F',
    CryptoPolygonRPCURL: '',
    CryptoPolygonConfirmations: 128,
    CryptoSolanaEnabled: false,
    CryptoSolanaReceiveAddress: '',
    CryptoSolanaUSDTMint: 'Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB',
    CryptoSolanaRPCURL: '',
    CryptoSolanaConfirmations: 32,
  });

  useEffect(() => {
    if (!options || !formApiRef.current) return;
    const nextInputs = {
      CryptoPaymentEnabled:
        options.CryptoPaymentEnabled === true ||
        options.CryptoPaymentEnabled === 'true',
      CryptoScannerEnabled:
        options.CryptoScannerEnabled === undefined ||
        options.CryptoScannerEnabled === true ||
        options.CryptoScannerEnabled === 'true',
      CryptoOrderExpireMinutes: Number(options.CryptoOrderExpireMinutes || 10),
      CryptoUniqueSuffixMax: Number(options.CryptoUniqueSuffixMax || 9999),
      CryptoTronEnabled:
        options.CryptoTronEnabled === true ||
        options.CryptoTronEnabled === 'true',
      CryptoTronReceiveAddress: options.CryptoTronReceiveAddress || '',
      CryptoTronUSDTContract:
        options.CryptoTronUSDTContract || 'TXLAQ63Xg1NAzckPwKHvzw7CSEmLMEqcdj',
      CryptoTronRPCURL: options.CryptoTronRPCURL || '',
      CryptoTronAPIKey: '',
      CryptoTronConfirmations: Number(options.CryptoTronConfirmations || 20),
      CryptoBSCEnabled:
        options.CryptoBSCEnabled === true ||
        options.CryptoBSCEnabled === 'true',
      CryptoBSCReceiveAddress: options.CryptoBSCReceiveAddress || '',
      CryptoBSCUSDTContract:
        options.CryptoBSCUSDTContract ||
        '0x55d398326f99059fF775485246999027B3197955',
      CryptoBSCRPCURL: options.CryptoBSCRPCURL || '',
      CryptoBSCConfirmations: Number(options.CryptoBSCConfirmations || 15),
      CryptoPolygonEnabled:
        options.CryptoPolygonEnabled === true ||
        options.CryptoPolygonEnabled === 'true',
      CryptoPolygonReceiveAddress: options.CryptoPolygonReceiveAddress || '',
      CryptoPolygonUSDTContract:
        options.CryptoPolygonUSDTContract ||
        '0xc2132D05D31c914a87C6611C10748AEb04B58e8F',
      CryptoPolygonRPCURL: options.CryptoPolygonRPCURL || '',
      CryptoPolygonConfirmations: Number(
        options.CryptoPolygonConfirmations || 128,
      ),
      CryptoSolanaEnabled:
        options.CryptoSolanaEnabled === true ||
        options.CryptoSolanaEnabled === 'true',
      CryptoSolanaReceiveAddress: options.CryptoSolanaReceiveAddress || '',
      CryptoSolanaUSDTMint:
        options.CryptoSolanaUSDTMint ||
        'Es9vMFrzaCERmJfrF4H2FYD4KCoNkY11McCe8BenwNYB',
      CryptoSolanaRPCURL: options.CryptoSolanaRPCURL || '',
      CryptoSolanaConfirmations: Number(
        options.CryptoSolanaConfirmations || 32,
      ),
    };
    setInputs(nextInputs);
    formApiRef.current.setValues(nextInputs);
  }, [options]);

  const submit = async () => {
    setLoading(true);
    try {
      const formValues = formApiRef.current?.getValues?.() || {};
      const entries = buildCryptoPaymentOptionUpdates(inputs, formValues);
      const requests = entries.map(({ key, value }) =>
        API.put('/api/option/', {
          key,
          value,
        }),
      );
      const results = await Promise.all(requests);
      const failed = results.find((res) => !res.data?.success);
      if (failed) {
        showError(failed.data?.message || t('更新失败'));
        return;
      }
      showSuccess(t('更新成功'));
      refresh && refresh();
    } catch (error) {
      showError(t('更新失败'));
    } finally {
      setLoading(false);
    }
  };

  return (
    <Spin spinning={loading}>
      <Form
        initValues={inputs}
        onValueChange={setInputs}
        getFormApi={(api) => (formApiRef.current = api)}
      >
        <Form.Section text={sectionTitle}>
          <Text type='secondary'>
            {t('仅支持 USDT，用户必须严格支付系统生成的唯一金额。')}
          </Text>
          <Row gutter={16}>
            <Col span={8}>
              <Form.Switch
                field='CryptoPaymentEnabled'
                label={t('启用 USDT 充值')}
              />
            </Col>
            <Col span={8}>
              <Form.Switch
                field='CryptoScannerEnabled'
                label={t('启用扫链器')}
              />
            </Col>
            <Col span={4}>
              <Form.InputNumber
                field='CryptoOrderExpireMinutes'
                label={t('订单有效期分钟')}
                min={5}
                max={60}
                precision={0}
              />
            </Col>
            <Col span={4}>
              <Form.InputNumber
                field='CryptoUniqueSuffixMax'
                label={t('尾数上限')}
                min={99}
                max={999999}
                precision={0}
              />
            </Col>
          </Row>
        </Form.Section>
        <Form.Section text='TRON TRC-20'>
          <Row gutter={16}>
            <Col span={6}>
              <Form.Switch field='CryptoTronEnabled' label={t('启用 TRON')} />
            </Col>
            <Col span={18}>
              <Form.Input
                field='CryptoTronReceiveAddress'
                label={t('TRON 收款地址')}
              />
            </Col>
            <Col span={12}>
              <Form.Input
                field='CryptoTronUSDTContract'
                label={t('TRON USDT 合约地址')}
              />
            </Col>
            <Col span={12}>
              <Form.Input field='CryptoTronRPCURL' label='TRON RPC / API URL' />
            </Col>
            <Col span={12}>
              <Form.Input
                field='CryptoTronAPIKey'
                label='TRON API Key'
                type='password'
                extraText={t('敏感信息不会发送到前端显示')}
              />
            </Col>
            <Col span={12}>
              <Form.InputNumber
                field='CryptoTronConfirmations'
                label={t('TRON 确认数')}
                min={10}
                precision={0}
              />
            </Col>
          </Row>
        </Form.Section>
        <Form.Section text='BSC'>
          <Row gutter={16}>
            <Col span={6}>
              <Form.Switch field='CryptoBSCEnabled' label={t('启用 BSC')} />
            </Col>
            <Col span={18}>
              <Form.Input
                field='CryptoBSCReceiveAddress'
                label={t('BSC 收款地址')}
              />
            </Col>
            <Col span={12}>
              <Form.Input
                field='CryptoBSCUSDTContract'
                label={t('BSC USDT 合约地址')}
              />
            </Col>
            <Col span={12}>
              <Form.Input field='CryptoBSCRPCURL' label='BSC RPC URL' />
            </Col>
            <Col span={12}>
              <Form.InputNumber
                field='CryptoBSCConfirmations'
                label={t('BSC 确认数')}
                min={8}
                precision={0}
              />
            </Col>
          </Row>
        </Form.Section>
        <Form.Section text='Polygon PoS'>
          <Row gutter={16}>
            <Col span={6}>
              <Form.Switch
                field='CryptoPolygonEnabled'
                label={t('启用 Polygon PoS')}
              />
            </Col>
            <Col span={18}>
              <Form.Input
                field='CryptoPolygonReceiveAddress'
                label={t('Polygon 收款地址')}
              />
            </Col>
            <Col span={12}>
              <Form.Input
                field='CryptoPolygonUSDTContract'
                label={t('Polygon USDT 合约地址')}
              />
            </Col>
            <Col span={12}>
              <Form.Input field='CryptoPolygonRPCURL' label='Polygon RPC URL' />
            </Col>
            <Col span={12}>
              <Form.InputNumber
                field='CryptoPolygonConfirmations'
                label={t('Polygon 确认数')}
                min={32}
                precision={0}
              />
            </Col>
          </Row>
        </Form.Section>
        <Form.Section text='Solana'>
          <Row gutter={16}>
            <Col span={6}>
              <Form.Switch
                field='CryptoSolanaEnabled'
                label={t('启用 Solana')}
              />
            </Col>
            <Col span={18}>
              <Form.Input
                field='CryptoSolanaReceiveAddress'
                label={t('Solana 收款地址')}
                extraText={t('建议填写收款钱包的 USDT Token Account 地址')}
              />
            </Col>
            <Col span={12}>
              <Form.Input
                field='CryptoSolanaUSDTMint'
                label={t('Solana USDT Mint 地址')}
              />
            </Col>
            <Col span={12}>
              <Form.Input field='CryptoSolanaRPCURL' label='Solana RPC URL' />
            </Col>
            <Col span={12}>
              <Form.InputNumber
                field='CryptoSolanaConfirmations'
                label={t('Solana 确认数')}
                min={1}
                precision={0}
              />
            </Col>
          </Row>
        </Form.Section>
        <Button type='primary' onClick={submit}>
          {t('更新支付设置')}
        </Button>
      </Form>
    </Spin>
  );
}
