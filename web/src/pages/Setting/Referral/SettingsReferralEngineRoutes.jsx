import React, { useEffect, useState } from 'react';
import { API, showError } from '../../../../helpers';
import { useTranslation } from 'react-i18next';
import { Empty, Spin, Table, Typography } from '@douyinfe/semi-ui';
import { normalizeReferralEngineRouteItems } from '../../../helpers/referralTemplate';

const SettingsReferralEngineRoutes = () => {
  const { t } = useTranslation();
  const [items, setItems] = useState([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    let mounted = true;
    const load = async () => {
      setLoading(true);
      try {
        const res = await API.get('/api/referral/engine-routes');
        if (!mounted) {
          return;
        }
        if (res.data?.success) {
          setItems(normalizeReferralEngineRouteItems(res.data?.data?.items));
        } else {
          showError(res.data?.message || t('加载失败'));
        }
      } catch (error) {
        if (mounted) {
          showError(error?.message || t('加载失败'));
        }
      } finally {
        if (mounted) {
          setLoading(false);
        }
      }
    };
    load();
    return () => {
      mounted = false;
    };
  }, [t]);

  return (
    <div className='space-y-3'>
      <div>
        <Typography.Title heading={5} style={{ marginBottom: 0 }}>
          {t('返佣引擎路由')}
        </Typography.Title>
        <Typography.Text type='secondary'>
          {t('查看每个返佣类型与分组当前使用的引擎模式。')}
        </Typography.Text>
      </div>
      <Spin spinning={loading}>
        {items.length === 0 ? (
          <Empty description={t('暂无返佣引擎路由')} />
        ) : (
          <Table
            dataSource={items}
            pagination={false}
            columns={[
              { title: t('类型'), dataIndex: 'referralType' },
              { title: t('分组'), dataIndex: 'group' },
              { title: t('引擎'), dataIndex: 'engineMode' },
            ]}
          />
        )}
      </Spin>
    </div>
  );
};

export default SettingsReferralEngineRoutes;
