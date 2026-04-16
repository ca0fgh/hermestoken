import React, { useEffect, useState } from 'react';
import { API, showError, showSuccess } from '../../../helpers';
import { useTranslation } from 'react-i18next';
import {
  Banner,
  Button,
  Empty,
  Input,
  Select,
  Space,
  Spin,
  Typography,
} from '@douyinfe/semi-ui';
import { normalizeReferralEngineRouteItems } from '../../../helpers/referralTemplate';
import ReferralFieldBlock from './ReferralFieldBlock';

const createDraftRoute = () => ({
  id: `draft-${Date.now()}-${Math.random()}`,
  referralType: 'subscription_referral',
  group: '',
  engineMode: 'legacy',
  isDraft: true,
});

const SettingsReferralEngineRoutes = () => {
  const { t } = useTranslation();
  const [items, setItems] = useState([]);
  const [loading, setLoading] = useState(false);
  const [savingKey, setSavingKey] = useState('');

  const load = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/referral/engine-routes');
      if (res.data?.success) {
        setItems(normalizeReferralEngineRouteItems(res.data?.data?.items));
      } else {
        showError(res.data?.message || t('加载失败'));
      }
    } catch (error) {
      showError(error?.message || t('加载失败'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, [t]);

  const updateRow = (id, patch) => {
    setItems((currentItems) =>
      currentItems.map((item) => (item.id === id ? { ...item, ...patch } : item)),
    );
  };

  const addDraft = () => {
    setItems((currentItems) => [...currentItems, createDraftRoute()]);
  };

  const saveRow = async (row) => {
    setSavingKey(String(row.id));
    try {
      const res = await API.put('/api/referral/engine-routes', {
        referral_type: row.referralType,
        group: row.group,
        engine_mode: row.engineMode,
      });
      if (res.data?.success) {
        showSuccess(t('保存成功'));
        await load();
      } else {
        showError(res.data?.message || t('保存失败'));
      }
    } catch (error) {
      showError(error?.message || t('保存失败'));
    } finally {
      setSavingKey('');
    }
  };

  return (
    <div className='space-y-3'>
      <div className='flex items-center justify-between gap-3'>
        <div>
          <Typography.Title heading={5} style={{ marginBottom: 0 }}>
            {t('返佣引擎路由')}
          </Typography.Title>
          <Typography.Text type='secondary'>
            {t('管理每个返佣类型与分组当前使用的引擎模式。')}
          </Typography.Text>
        </div>
        <Button type='primary' onClick={addDraft}>
          {t('新增路由')}
        </Button>
      </div>
      <Banner
        type='info'
        bordered
        title={t('切换说明')}
        description={t(
          '引擎路由按 referral_type + group 决定结算入口。未配置路由时默认走 legacy；只有切到 template 的分组，才会使用上面的模板、模板绑定和 direct/team 两级返佣逻辑。',
        )}
      />
      <Spin spinning={loading}>
        {items.length === 0 ? (
          <Empty description={t('暂无返佣引擎路由')} />
        ) : (
          items.map((row) => (
            <div key={row.id} className='rounded-xl border border-gray-200 p-4 space-y-3'>
              <div className='grid grid-cols-1 gap-3 lg:grid-cols-3'>
                <ReferralFieldBlock
                  label={t('返佣类型')}
                  description={t('当前路由属于哪个返佣体系。当前页面主要用于 subscription_referral。')}
                >
                  <Input
                    value={row.referralType}
                    placeholder={t('返佣类型')}
                    onChange={(value) => updateRow(row.id, { referralType: value })}
                  />
                </ReferralFieldBlock>
                <ReferralFieldBlock
                  label={t('分组')}
                  description={t('这条路由控制哪个分组的返佣结算入口。它要和模板分组、订阅计划分组保持一致。')}
                >
                  <Input
                    value={row.group}
                    placeholder={t('分组')}
                    onChange={(value) => updateRow(row.id, { group: value })}
                  />
                </ReferralFieldBlock>
                <ReferralFieldBlock
                  label={t('引擎模式')}
                  description={t('legacy 表示继续使用旧订阅返佣逻辑；template 表示切到模板 + 绑定 + direct/team 新引擎。')}
                >
                  <Select
                    value={row.engineMode}
                    optionList={[
                      { label: 'legacy', value: 'legacy' },
                      { label: 'template', value: 'template' },
                    ]}
                    onChange={(value) => updateRow(row.id, { engineMode: value })}
                  />
                </ReferralFieldBlock>
              </div>
              <Space>
                <Button
                  type='primary'
                  loading={savingKey === String(row.id)}
                  onClick={() => saveRow(row)}
                >
                  {t('保存')}
                </Button>
              </Space>
            </div>
          ))
        )}
      </Spin>
    </div>
  );
};

export default SettingsReferralEngineRoutes;
