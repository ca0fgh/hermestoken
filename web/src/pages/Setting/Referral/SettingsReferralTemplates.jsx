import React, { useEffect, useState } from 'react';
import { API, showError } from '../../../../helpers';
import { useTranslation } from 'react-i18next';
import { Empty, Spin, Table, Typography } from '@douyinfe/semi-ui';
import { normalizeReferralTemplateItems } from '../../../helpers/referralTemplate';

const SettingsReferralTemplates = () => {
  const { t } = useTranslation();
  const [items, setItems] = useState([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    let mounted = true;
    const load = async () => {
      setLoading(true);
      try {
        const res = await API.get('/api/referral/templates');
        if (!mounted) {
          return;
        }
        if (res.data?.success) {
          setItems(normalizeReferralTemplateItems(res.data?.data?.items));
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
          {t('返佣模板')}
        </Typography.Title>
        <Typography.Text type='secondary'>
          {t('查看当前返佣类型与分组下的模板配置。')}
        </Typography.Text>
      </div>
      <Spin spinning={loading}>
        {items.length === 0 ? (
          <Empty description={t('暂无返佣模板')} />
        ) : (
          <Table
            dataSource={items}
            pagination={false}
            columns={[
              { title: t('类型'), dataIndex: 'referralType' },
              { title: t('分组'), dataIndex: 'group' },
              { title: t('模板名'), dataIndex: 'name' },
              { title: t('身份'), dataIndex: 'levelType' },
              { title: t('默认分账比例'), dataIndex: 'inviteeShareDefaultBps' },
            ]}
          />
        )}
      </Spin>
    </div>
  );
};

export default SettingsReferralTemplates;
