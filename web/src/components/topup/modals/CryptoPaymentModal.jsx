import React, { useMemo } from 'react';
import {
  Modal,
  Typography,
  Button,
  Tag,
  Progress,
  Banner,
} from '@douyinfe/semi-ui';
import { Copy, Wallet } from 'lucide-react';
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
  const secondsLeft = useMemo(() => {
    if (!order?.expires_at) return 0;
    return Math.max(0, order.expires_at - Math.floor(Date.now() / 1000));
  }, [order?.expires_at]);

  const progress = order?.required_confirmations
    ? Math.min(
        100,
        Math.round(
          ((order.confirmations || 0) / order.required_confirmations) * 100,
        ),
      )
    : 0;
  const expired = order?.status === 'expired' || secondsLeft <= 0;
  const terminal = terminalStatuses.has(order?.status);

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
            <Tag color={terminal ? 'grey' : 'orange'}>{order.status}</Tag>
          </div>

          <Banner
            type='warning'
            closeIcon={null}
            description={t(
              '请严格使用当前网络并支付完整显示金额。少付、多付、超时到账都不会自动入账。',
            )}
          />

          <div className='rounded-xl border border-gray-200 p-4 space-y-3'>
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

          <div className='space-y-2'>
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
            <div className='flex justify-between'>
              <Text type='secondary'>{t('确认进度')}</Text>
              <Text>
                {order.confirmations || 0}/{order.required_confirmations}
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
