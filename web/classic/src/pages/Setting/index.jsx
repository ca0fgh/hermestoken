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

import React, { Suspense, useEffect, useState } from 'react';
import { Layout, Spin, TabPane, Tabs } from '@douyinfe/semi-ui';
import { useNavigate, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  Settings,
  Calculator,
  Gauge,
  Shapes,
  Cog,
  MoreHorizontal,
  LayoutDashboard,
  MessageSquare,
  Palette,
  CreditCard,
  Server,
  Activity,
} from 'lucide-react';
import { isRoot } from '../../helpers/session';
import { lazyWithRetry } from '../../helpers/lazyWithRetry';

const SystemSetting = lazyWithRetry(
  () => import('../../components/settings/SystemSetting'),
  'setting-system-tab',
);
const OtherSetting = lazyWithRetry(
  () => import('../../components/settings/OtherSetting'),
  'setting-other-tab',
);
const OperationSetting = lazyWithRetry(
  () => import('../../components/settings/OperationSetting'),
  'setting-operation-tab',
);
const RateLimitSetting = lazyWithRetry(
  () => import('../../components/settings/RateLimitSetting'),
  'setting-ratelimit-tab',
);
const ModelSetting = lazyWithRetry(
  () => import('../../components/settings/ModelSetting'),
  'setting-models-tab',
);
const DashboardSetting = lazyWithRetry(
  () => import('../../components/settings/DashboardSetting'),
  'setting-dashboard-tab',
);
const RatioSetting = lazyWithRetry(
  () => import('../../components/settings/RatioSetting'),
  'setting-ratio-tab',
);
const ChatsSetting = lazyWithRetry(
  () => import('../../components/settings/ChatsSetting'),
  'setting-chats-tab',
);
const DrawingSetting = lazyWithRetry(
  () => import('../../components/settings/DrawingSetting'),
  'setting-drawing-tab',
);
const PaymentSetting = lazyWithRetry(
  () => import('../../components/settings/PaymentSetting'),
  'setting-payment-tab',
);
const ModelDeploymentSetting = lazyWithRetry(
  () => import('../../components/settings/ModelDeploymentSetting'),
  'setting-model-deployment-tab',
);
const PerformanceSetting = lazyWithRetry(
  () => import('../../components/settings/PerformanceSetting'),
  'setting-performance-tab',
);
const ReferralSetting = lazyWithRetry(
  () => import('../../components/settings/ReferralSetting'),
  'setting-referral-tab',
);

const Setting = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const location = useLocation();
  const [tabActiveKey, setTabActiveKey] = useState('operation');
  let panes = [];

  if (isRoot()) {
    panes.push({
      tab: (
        <span style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
          <Settings size={18} />
          {t('运营设置')}
        </span>
      ),
      content: <OperationSetting />,
      itemKey: 'operation',
    });
    panes.push({
      tab: (
        <span style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
          <LayoutDashboard size={18} />
          {t('仪表盘设置')}
        </span>
      ),
      content: <DashboardSetting />,
      itemKey: 'dashboard',
    });
    panes.push({
      tab: (
        <span style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
          <MessageSquare size={18} />
          {t('聊天设置')}
        </span>
      ),
      content: <ChatsSetting />,
      itemKey: 'chats',
    });
    panes.push({
      tab: (
        <span style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
          <Palette size={18} />
          {t('绘图设置')}
        </span>
      ),
      content: <DrawingSetting />,
      itemKey: 'drawing',
    });
    panes.push({
      tab: (
        <span style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
          <CreditCard size={18} />
          {t('支付设置')}
        </span>
      ),
      content: <PaymentSetting />,
      itemKey: 'payment',
    });
    panes.push({
      tab: (
        <span style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
          <Settings size={18} />
          {t('返佣模板设置')}
        </span>
      ),
      content: <ReferralSetting />,
      itemKey: 'referral',
    });
    panes.push({
      tab: (
        <span style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
          <Calculator size={18} />
          {t('分组与模型定价设置')}
        </span>
      ),
      content: <RatioSetting />,
      itemKey: 'ratio',
    });
    panes.push({
      tab: (
        <span style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
          <Gauge size={18} />
          {t('速率限制设置')}
        </span>
      ),
      content: <RateLimitSetting />,
      itemKey: 'ratelimit',
    });
    panes.push({
      tab: (
        <span style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
          <Shapes size={18} />
          {t('模型相关设置')}
        </span>
      ),
      content: <ModelSetting />,
      itemKey: 'models',
    });
    panes.push({
      tab: (
        <span style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
          <Server size={18} />
          {t('模型部署设置')}
        </span>
      ),
      content: <ModelDeploymentSetting />,
      itemKey: 'model-deployment',
    });
    panes.push({
      tab: (
        <span style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
          <Activity size={18} />
          {t('性能设置')}
        </span>
      ),
      content: <PerformanceSetting />,
      itemKey: 'performance',
    });
    panes.push({
      tab: (
        <span style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
          <Cog size={18} />
          {t('系统设置')}
        </span>
      ),
      content: <SystemSetting />,
      itemKey: 'system',
    });
    panes.push({
      tab: (
        <span style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
          <MoreHorizontal size={18} />
          {t('其他设置')}
        </span>
      ),
      content: <OtherSetting />,
      itemKey: 'other',
    });
  }

  const onChangeTab = (key) => {
    setTabActiveKey(key);
    navigate(`?tab=${key}`);
  };

  useEffect(() => {
    const searchParams = new URLSearchParams(window.location.search);
    const tab = searchParams.get('tab');
    if (tab) {
      setTabActiveKey(tab);
      return;
    }
    onChangeTab('operation');
  }, [location.search]);

  return (
    <div className='mt-[60px] px-2'>
      <Layout>
        <Layout.Content>
          <Tabs
            type='card'
            collapsible
            activeKey={tabActiveKey}
            onChange={(key) => onChangeTab(key)}
          >
            {panes.map((pane) => (
              <TabPane itemKey={pane.itemKey} tab={pane.tab} key={pane.itemKey}>
                {tabActiveKey === pane.itemKey ? (
                  <Suspense fallback={<Spin spinning />}>
                    {pane.content}
                  </Suspense>
                ) : null}
              </TabPane>
            ))}
          </Tabs>
        </Layout.Content>
      </Layout>
    </div>
  );
};

export default Setting;
