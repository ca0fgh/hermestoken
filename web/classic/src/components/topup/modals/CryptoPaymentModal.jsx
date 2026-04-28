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
import React, { useEffect, useMemo, useState } from 'react';
import {
  Modal,
  Typography,
  Button,
  Tag,
  Progress,
  Banner,
} from '@douyinfe/semi-ui';
import { CircleCheckBig, Copy, Wallet } from 'lucide-react';
import { QRCodeSVG } from 'qrcode.react';
import { copy } from '../../../helpers';

const { Text, Title } = Typography;

const terminalStatuses = new Set([
  'success',
  'expired',
  'failed',
  'underpaid',
  'overpaid',
  'late_paid',
  'ambiguous',
]);

const CryptoPaymentModal = ({ t, open, order, onCancel }) => {
  const isSuccess = order?.status === 'success';
  const [now, setNow] = useState(() => Math.floor(Date.now() / 1000));

  useEffect(() => {
    if (!open || !order?.expires_at || isSuccess) return;
    setNow(Math.floor(Date.now() / 1000));
    const timer = setInterval(() => setNow(Math.floor(Date.now() / 1000)), 1000);
    return () => clearInterval(timer);
  }, [open, order?.expires_at, isSuccess]);

  const secondsLeft = useMemo(() => {
    if (!order?.expires_at) return 0;
    return Math.max(0, order.expires_at - now);
  }, [order?.expires_at, now]);

  const confirmedConfirmations = order?.required_confirmations
    ? Math.min(order.confirmations || 0, order.required_confirmations)
    : order?.confirmations || 0;
  const progress = order?.required_confirmations
    ? Math.min(
        100,
        Math.round(
          (confirmedConfirmations / order.required_confirmations) * 100,
        ),
      )
    : 0;
  const expired =
    !isSuccess && (order?.status === 'expired' || secondsLeft <= 0);
  const terminal = terminalStatuses.has(order?.status);
  const paymentQrValue = order?.receive_address || '';

  return (
    <Modal
      visible={open}
      title={
        <span className='flex items-center gap-2'>
          <Wallet size={18} />
          {t('USDT 充值')}
        </span>
      }
      onCancel={onCancel}
      footer={null}
      maskClosable={false}
      size='medium'
      centered
    >
      {!order ? null : (
        <div className='space-y-4'>
          <div className='flex items-center gap-2'>
            <Tag color={order.network === 'tron_trc20' ? 'green' : 'yellow'}>
              {order.network === 'tron_trc20' ? 'TRON TRC-20' : 'BSC'}
            </Tag>
            <Tag color='blue'>USDT</Tag>
            <Tag color={isSuccess ? 'green' : terminal ? 'grey' : 'orange'}>
              {isSuccess ? t('订单已完成') : order.status}
            </Tag>
          </div>

          {isSuccess && (
            <div className='flex items-start gap-3 rounded-xl border border-green-200 bg-green-50 px-4 py-3'>
              <CircleCheckBig
                size={22}
                className='mt-0.5 shrink-0'
                color='var(--semi-color-success)'
              />
              <div>
                <Text strong>{t('充值成功')}</Text>
                <div className='text-sm text-gray-600'>
                  {t('额度已自动入账，可关闭窗口后查看余额。')}
                </div>
              </div>
            </div>
          )}

          {!isSuccess && (
            <Banner
              type='warning'
              closeIcon={null}
              description={t(
                '请严格使用当前网络并支付完整显示金额。少付、多付、超时到账都不会自动入账。',
              )}
            />
          )}

          <div className='rounded-xl border border-gray-200 p-4'>
            <div className='flex flex-col gap-4 md:flex-row md:items-start md:justify-between'>
              <div className='min-w-0 flex-1 space-y-3'>
                <div>
                  <Text type='secondary'>{t('应付金额')}</Text>
                  <div className='flex items-center justify-between gap-2'>
                    <Title heading={3} style={{ margin: 0 }}>
                      {order.pay_amount} USDT
                    </Title>
                    <Button
                      icon={<Copy size={14} />}
                      disabled={expired}
                      onClick={() => copy(order.pay_amount)}
                    >
                      {t('复制')}
                    </Button>
                  </div>
                </div>
                <div>
                  <Text type='secondary'>{t('收款地址')}</Text>
                  <div className='flex items-center justify-between gap-2 break-all'>
                    <Text copyable={false}>{order.receive_address}</Text>
                    <Button
                      icon={<Copy size={14} />}
                      disabled={expired}
                      onClick={() => copy(order.receive_address)}
                    >
                      {t('复制')}
                    </Button>
                  </div>
                </div>
              </div>

              {!isSuccess && paymentQrValue && (
                <div className='mx-auto flex w-36 shrink-0 flex-col items-center gap-2 rounded-lg border border-gray-100 bg-white p-2 md:mx-0'>
                  <QRCodeSVG
                    value={paymentQrValue}
                    size={128}
                    level='M'
                    role='img'
                    aria-label={t('收款地址二维码')}
                  />
                  <Text type='secondary' size='small'>
                    {t('收款地址二维码')}
                  </Text>
                </div>
              )}
            </div>
          </div>

          <div className='space-y-2'>
            {!isSuccess && (
              <div className='flex justify-between'>
                <Text type='secondary'>{t('订单倒计时')}</Text>
                <Text strong>
                  {expired
                    ? t('已过期')
                    : `${Math.floor(secondsLeft / 60)}:${String(
                        secondsLeft % 60,
                      ).padStart(2, '0')}`}
                </Text>
              </div>
            )}
            <div className='flex justify-between'>
              <Text type='secondary'>{t('确认进度')}</Text>
              <Text>
                {confirmedConfirmations}/{order.required_confirmations}
              </Text>
            </div>
            <Progress percent={progress} showInfo={false} />
          </div>

          {order.tx_hash && (
            <div className='break-all'>
              <Text type='secondary'>TX Hash: </Text>
              <Text>{order.tx_hash}</Text>
            </div>
          )}
        </div>
      )}
    </Modal>
  );
};

export default CryptoPaymentModal;
