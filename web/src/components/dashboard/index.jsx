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

import React, {
  Suspense,
  lazy,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import { BarChart3 } from 'lucide-react';
import { getRelativeTime } from '../../helpers/time';
import { UserContext } from '../../context/User';
import { StatusContext } from '../../context/Status';

import DashboardHeader from './DashboardHeader';

import { useDashboardData } from '../../hooks/dashboard/useDashboardData';
import { useDashboardStats } from '../../hooks/dashboard/useDashboardStats';
import { useDashboardCharts } from '../../hooks/dashboard/useDashboardCharts';

import {
  CHART_CONFIG,
  CARD_PROPS,
  FLEX_CENTER_GAP2,
  ILLUSTRATION_SIZE,
  ANNOUNCEMENT_LEGEND_DATA,
  UPTIME_STATUS_MAP,
} from '../../constants/dashboard.constants';
import {
  handleCopyUrl,
  handleSpeedTest,
  getUptimeStatusColor,
  getUptimeStatusText,
  renderMonitorList,
} from '../../helpers/dashboard';
import { lazyWithRetry } from '../../helpers/lazyWithRetry';
import StatsCards from './StatsCards';
import ApiInfoPanel from './ApiInfoPanel';
import AnnouncementsPanel from './AnnouncementsPanel';
import FaqPanel from './FaqPanel';
import UptimePanel from './UptimePanel';

const SearchModal = lazy(() => import('./modals/SearchModal'));
const ChartsPanel = lazyWithRetry(
  () => import('./ChartsPanel'),
  'dashboard-charts-panel',
);

const Dashboard = () => {
  // ========== Context ==========
  const [userState, userDispatch] = useContext(UserContext);
  const [statusState] = useContext(StatusContext);
  const [chartsPanelEnabled, setChartsPanelEnabled] = useState(false);
  const adminUserChartCacheKeyRef = useRef('');

  // ========== 主要数据管理 ==========
  const dashboardData = useDashboardData(userState, userDispatch, statusState);

  // ========== 图表管理 ==========
  const dashboardCharts = useDashboardCharts(
    dashboardData.dataExportDefaultTime,
    dashboardData.setTrendData,
    dashboardData.setConsumeQuota,
    dashboardData.setTimes,
    dashboardData.setConsumeTokens,
    dashboardData.setPieData,
    dashboardData.setLineData,
    dashboardData.setModelColors,
    dashboardData.t,
  );

  // ========== 统计数据 ==========
  const { groupedStatsData } = useDashboardStats(
    userState,
    dashboardData.consumeQuota,
    dashboardData.consumeTokens,
    dashboardData.times,
    dashboardData.trendData,
    dashboardData.performanceMetrics,
    dashboardData.navigate,
    dashboardData.t,
  );

  const userChartCacheKey = useMemo(() => {
    return [
      dashboardData.inputs.start_timestamp,
      dashboardData.inputs.end_timestamp,
      dashboardData.dataExportDefaultTime,
    ].join('|');
  }, [
    dashboardData.inputs.start_timestamp,
    dashboardData.inputs.end_timestamp,
    dashboardData.dataExportDefaultTime,
  ]);

  const shouldLoadAdminUserCharts =
    chartsPanelEnabled &&
    dashboardData.isAdminUser &&
    ['5', '6'].includes(dashboardData.activeChartTab);

  // ========== 数据处理 ==========
  const loadUserData = useCallback(
    async ({ force = false } = {}) => {
      if (!dashboardData.isAdminUser) {
        return;
      }

      if (!force && adminUserChartCacheKeyRef.current === userChartCacheKey) {
        return;
      }

      const userData = await dashboardData.loadUserQuotaData();
      dashboardCharts.updateUserChartData(userData || []);
      adminUserChartCacheKeyRef.current = userChartCacheKey;
    },
    [
      dashboardCharts,
      dashboardData.isAdminUser,
      dashboardData.loadUserQuotaData,
      userChartCacheKey,
    ],
  );

  const initChart = async () => {
    await dashboardData.loadQuotaData().then((data) => {
      if (data && data.length > 0) {
        dashboardCharts.updateChartData(data);
      }
    });

    if (dashboardData.uptimeEnabled) {
      await dashboardData.loadUptimeData();
    }
  };

  const handleRefresh = async () => {
    const data = await dashboardData.refresh();
    if (data && data.length > 0) {
      dashboardCharts.updateChartData(data);
    }
    if (shouldLoadAdminUserCharts) {
      await loadUserData({ force: true });
    }
  };

  const handleSearchConfirm = async () => {
    await dashboardData.handleSearchConfirm(dashboardCharts.updateChartData);
    if (shouldLoadAdminUserCharts) {
      await loadUserData({ force: true });
    }
  };

  // ========== 数据准备 ==========
  const apiInfoData = statusState?.status?.api_info || [];
  const hasVisibleApiInfoPanel =
    dashboardData.hasApiInfoPanel && apiInfoData.length > 0;
  const announcementData = (statusState?.status?.announcements || []).map(
    (item) => {
      const pubDate = item?.publishDate ? new Date(item.publishDate) : null;
      const absoluteTime =
        pubDate && !isNaN(pubDate.getTime())
          ? `${pubDate.getFullYear()}-${String(pubDate.getMonth() + 1).padStart(2, '0')}-${String(pubDate.getDate()).padStart(2, '0')} ${String(pubDate.getHours()).padStart(2, '0')}:${String(pubDate.getMinutes()).padStart(2, '0')}`
          : item?.publishDate || '';
      const relativeTime = getRelativeTime(item.publishDate);
      return {
        ...item,
        time: absoluteTime,
        relative: relativeTime,
      };
    },
  );
  const faqData = statusState?.status?.faq || [];

  const uptimeLegendData = Object.entries(UPTIME_STATUS_MAP).map(
    ([status, info]) => ({
      status: Number(status),
      color: info.color,
      label: dashboardData.t(info.label),
    }),
  );

  // ========== Effects ==========
  useEffect(() => {
    void initChart();
  }, []);

  useEffect(() => {
    if (!shouldLoadAdminUserCharts) {
      return;
    }

    void loadUserData();
  }, [loadUserData, shouldLoadAdminUserCharts]);

  return (
    <div className='h-full'>
      <DashboardHeader
        getGreeting={dashboardData.getGreeting}
        greetingVisible={dashboardData.greetingVisible}
        showSearchModal={dashboardData.showSearchModal}
        refresh={handleRefresh}
        loading={dashboardData.loading}
        t={dashboardData.t}
      />

      {dashboardData.searchModalVisible ? (
        <Suspense fallback={null}>
          <SearchModal
            searchModalVisible={dashboardData.searchModalVisible}
            handleSearchConfirm={handleSearchConfirm}
            handleCloseModal={dashboardData.handleCloseModal}
            isMobile={dashboardData.isMobile}
            isAdminUser={dashboardData.isAdminUser}
            inputs={dashboardData.inputs}
            dataExportDefaultTime={dashboardData.dataExportDefaultTime}
            timeOptions={dashboardData.timeOptions}
            handleInputChange={dashboardData.handleInputChange}
            t={dashboardData.t}
          />
        </Suspense>
      ) : null}

      <StatsCards
        groupedStatsData={groupedStatsData}
        loading={dashboardData.loading}
        CARD_PROPS={CARD_PROPS}
      />

      {/* API信息和图表面板 */}
      <div className='mb-4'>
        <div
          className={`grid grid-cols-1 gap-4 ${hasVisibleApiInfoPanel ? 'lg:grid-cols-4' : ''}`}
        >
          {chartsPanelEnabled ? (
            <Suspense
              fallback={
                <div
                  className={`rounded-2xl border border-slate-200 bg-white p-6 shadow-sm ${hasVisibleApiInfoPanel ? 'lg:col-span-3' : ''}`}
                >
                  <div className='h-96 animate-pulse rounded-2xl bg-slate-100' />
                </div>
              }
            >
              <ChartsPanel
                activeChartTab={dashboardData.activeChartTab}
                setActiveChartTab={dashboardData.setActiveChartTab}
                spec_line={dashboardCharts.spec_line}
                spec_model_line={dashboardCharts.spec_model_line}
                spec_pie={dashboardCharts.spec_pie}
                spec_rank_bar={dashboardCharts.spec_rank_bar}
                spec_user_rank={dashboardCharts.spec_user_rank}
                spec_user_trend={dashboardCharts.spec_user_trend}
                isAdminUser={dashboardData.isAdminUser}
                CARD_PROPS={CARD_PROPS}
                CHART_CONFIG={CHART_CONFIG}
                FLEX_CENTER_GAP2={FLEX_CENTER_GAP2}
                hasApiInfoPanel={hasVisibleApiInfoPanel}
                t={dashboardData.t}
              />
            </Suspense>
          ) : (
            <section
              className={`rounded-2xl border border-slate-200 bg-white p-6 shadow-sm ${hasVisibleApiInfoPanel ? 'lg:col-span-3' : ''}`}
            >
              <div className='flex flex-col gap-4 md:flex-row md:items-start md:justify-between'>
                <div className='space-y-2'>
                  <div className='flex items-center gap-2 text-sm font-semibold text-slate-900'>
                    <BarChart3 size={16} />
                    <span>{dashboardData.t('模型数据分析')}</span>
                  </div>
                  <p className='max-w-2xl text-sm text-slate-500'>
                    {dashboardData.t(
                      '图表分析改为按需加载，先保证控制台首页首屏更快，再在你需要时拉取大体积图表运行时。',
                    )}
                  </p>
                </div>
                <button
                  type='button'
                  className='inline-flex items-center justify-center rounded-full bg-slate-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-slate-700 self-start'
                  onClick={() => setChartsPanelEnabled(true)}
                >
                  {dashboardData.t('加载图表分析')}
                </button>
              </div>
            </section>
          )}

          {hasVisibleApiInfoPanel && (
            <ApiInfoPanel
              apiInfoData={apiInfoData}
              handleCopyUrl={(url) => handleCopyUrl(url, dashboardData.t)}
              handleSpeedTest={handleSpeedTest}
              CARD_PROPS={CARD_PROPS}
              FLEX_CENTER_GAP2={FLEX_CENTER_GAP2}
              ILLUSTRATION_SIZE={ILLUSTRATION_SIZE}
              t={dashboardData.t}
            />
          )}
        </div>
      </div>

      {/* 系统公告和常见问答卡片 */}
      {dashboardData.hasInfoPanels && (
        <div className='mb-4'>
          <div className='grid grid-cols-1 lg:grid-cols-4 gap-4'>
            {/* 公告卡片 */}
            {dashboardData.announcementsEnabled && (
              <AnnouncementsPanel
                announcementData={announcementData}
                announcementLegendData={ANNOUNCEMENT_LEGEND_DATA.map((item) => ({
                  ...item,
                  label: dashboardData.t(item.label),
                }))}
                CARD_PROPS={CARD_PROPS}
                ILLUSTRATION_SIZE={ILLUSTRATION_SIZE}
                t={dashboardData.t}
              />
            )}

            {/* 常见问答卡片 */}
            {dashboardData.faqEnabled && (
              <FaqPanel
                faqData={faqData}
                CARD_PROPS={CARD_PROPS}
                FLEX_CENTER_GAP2={FLEX_CENTER_GAP2}
                ILLUSTRATION_SIZE={ILLUSTRATION_SIZE}
                t={dashboardData.t}
              />
            )}

            {/* 服务可用性卡片 */}
            {dashboardData.uptimeEnabled && (
              <UptimePanel
                uptimeData={dashboardData.uptimeData}
                uptimeLoading={dashboardData.uptimeLoading}
                activeUptimeTab={dashboardData.activeUptimeTab}
                setActiveUptimeTab={dashboardData.setActiveUptimeTab}
                loadUptimeData={dashboardData.loadUptimeData}
                uptimeLegendData={uptimeLegendData}
                renderMonitorList={(monitors) =>
                  renderMonitorList(
                    monitors,
                    (status) => getUptimeStatusColor(status, UPTIME_STATUS_MAP),
                    (status) =>
                      getUptimeStatusText(
                        status,
                        UPTIME_STATUS_MAP,
                        dashboardData.t,
                      ),
                    dashboardData.t,
                  )
                }
                CARD_PROPS={CARD_PROPS}
                ILLUSTRATION_SIZE={ILLUSTRATION_SIZE}
                t={dashboardData.t}
              />
            )}
          </div>
        </div>
      )}
    </div>
  );
};

export default Dashboard;
