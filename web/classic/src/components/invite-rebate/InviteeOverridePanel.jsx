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

import React, { useEffect, useMemo, useState } from 'react';
import {
  Button,
  Card,
  Empty,
  InputNumber,
  Space,
  TabPane,
  Tag,
  Tabs,
  Typography,
} from '@douyinfe/semi-ui';
import { API } from '../../helpers/api';
import { showError, showSuccess } from '../../helpers/notifications';
import { renderQuota } from '../../helpers/quota';
import { timestamp2string } from '../../helpers/utils';
import {
  clampInviteeRateBps,
  formatRateBpsPercent,
  percentNumberToRateBps,
  rateBpsToPercentNumber,
} from '../../helpers/subscriptionReferral';
import {
  buildInviteeContributionLedgerRows,
  buildInviteeContributionSummary,
  buildInviteeOverrideDraftPercentMap,
} from '../../helpers/inviteRebate';
import ListPagination from '../common/ui/ListPagination';

const CONTRIBUTION_PAGE_SIZE = 5;
const OVERRIDE_PAGE_SIZE = 5;
const DETAIL_ITEM_PAGE_SIZE = 3;
const LEDGER_PAGE_SIZE = 8;

const InviteeOverridePanel = ({
  t,
  invitee = null,
  rows = [],
  contributionCards = [],
  loading = false,
  onOverridesChanged,
}) => {
  const formatModeLabel = (levelType) => {
    if (levelType === 'direct') return t('直推');
    if (levelType === 'team') return t('团队');
    return '-';
  };

  const [draftPercentByGroup, setDraftPercentByGroup] = useState({});
  const [savingGroup, setSavingGroup] = useState('');
  const [deletingGroup, setDeletingGroup] = useState('');
  const [contributionView, setContributionView] = useState('orders');
  const [contributionPage, setContributionPage] = useState(1);
  const [ledgerPage, setLedgerPage] = useState(1);
  const [overridePage, setOverridePage] = useState(1);
  const [detailItemPageByCard, setDetailItemPageByCard] = useState({});

  const normalizedRows = useMemo(() => rows || [], [rows]);
  const normalizedContributionCards = useMemo(
    () => contributionCards || [],
    [contributionCards],
  );

  const formatContributionStatusLabel = (status) => {
    if (status === 'reversed') return t('已冲正');
    if (status === 'pending') return t('待结算');
    return t('已到账');
  };

  const renderContributionRoleColor = (roleType) => {
    if (roleType === 'team') return 'blue';
    return 'green';
  };

  const renderContributionComponentColor = (item) => {
    if (item?.isInviteeShare) return 'orange';
    if (item?.roleType === 'team') return 'light-blue';
    return 'green';
  };

  const renderContributionSurfaceClass = (item) => {
    if (item?.isInviteeShare) {
      return 'border-amber-100 bg-amber-50/70';
    }
    if (item?.roleType === 'team') {
      return 'border-sky-100 bg-sky-50/70';
    }
    return 'border-emerald-100 bg-emerald-50/70';
  };

  const renderContributionStatusColor = (status) => {
    if (status === 'reversed') return 'red';
    if (status === 'pending') return 'orange';
    return 'green';
  };

  const contributionTotalPages = Math.max(
    1,
    Math.ceil(normalizedContributionCards.length / CONTRIBUTION_PAGE_SIZE),
  );
  const overrideTotalPages = Math.max(
    1,
    Math.ceil(normalizedRows.length / OVERRIDE_PAGE_SIZE),
  );

  useEffect(() => {
    setContributionPage((page) => Math.min(page, contributionTotalPages));
  }, [contributionTotalPages]);

  useEffect(() => {
    setOverridePage((page) => Math.min(page, overrideTotalPages));
  }, [overrideTotalPages]);

  useEffect(() => {
    setContributionView('orders');
    setContributionPage(1);
    setLedgerPage(1);
    setOverridePage(1);
    setDetailItemPageByCard({});
  }, [invitee?.id]);

  useEffect(() => {
    setDraftPercentByGroup(buildInviteeOverrideDraftPercentMap(normalizedRows));
  }, [invitee?.id, normalizedRows]);

  useEffect(() => {
    setDetailItemPageByCard((currentPages) => {
      const nextPages = {};
      normalizedContributionCards.forEach((card) => {
        const totalPages = Math.max(
          1,
          Math.ceil((card?.items?.length || 0) / DETAIL_ITEM_PAGE_SIZE),
        );
        nextPages[card.id] = Math.min(currentPages[card.id] || 1, totalPages);
      });
      return nextPages;
    });
  }, [normalizedContributionCards]);

  const pagedContributionCards = useMemo(() => {
    const start = (contributionPage - 1) * CONTRIBUTION_PAGE_SIZE;
    return normalizedContributionCards.slice(
      start,
      start + CONTRIBUTION_PAGE_SIZE,
    );
  }, [contributionPage, normalizedContributionCards]);

  const pagedOverrideRows = useMemo(() => {
    const start = (overridePage - 1) * OVERRIDE_PAGE_SIZE;
    return normalizedRows.slice(start, start + OVERRIDE_PAGE_SIZE);
  }, [normalizedRows, overridePage]);

  const contributionSummary = useMemo(
    () => buildInviteeContributionSummary(normalizedContributionCards),
    [normalizedContributionCards],
  );

  const contributionLedgerRows = useMemo(
    () => buildInviteeContributionLedgerRows(normalizedContributionCards),
    [normalizedContributionCards],
  );

  const pagedContributionLedgerRows = useMemo(() => {
    const start = (ledgerPage - 1) * LEDGER_PAGE_SIZE;
    return contributionLedgerRows.slice(start, start + LEDGER_PAGE_SIZE);
  }, [contributionLedgerRows, ledgerPage]);

  const getDraftPercent = (row) => {
    const group = String(row?.group || '').trim();
    if (Object.prototype.hasOwnProperty.call(draftPercentByGroup, group)) {
      return draftPercentByGroup[group];
    }
    return Number(row?.inputPercent || 0);
  };

  const updateDraftPercent = (group, value) => {
    setDraftPercentByGroup((currentDrafts) => ({
      ...currentDrafts,
      [group]: Number(value || 0),
    }));
  };

  const saveOverride = async (row) => {
    const group = String(row?.group || '').trim();
    const inviteeRateBps = clampInviteeRateBps(
      percentNumberToRateBps(getDraftPercent(row)),
      row?.effectiveTotalRateBps,
    );

    setSavingGroup(group);
    try {
      const res = await API.put(
        `/api/user/referral/subscription/invitees/${invitee.id}`,
        {
          group,
          invitee_rate_bps: inviteeRateBps,
        },
      );

      if (res.data?.success) {
        showSuccess(t('保存成功'));
        await onOverridesChanged?.();
      } else {
        showError(res.data?.message || t('保存失败'));
      }
    } catch (error) {
      showError(error?.message || t('保存失败'));
    } finally {
      setSavingGroup('');
    }
  };

  const deleteOverride = async (group) => {
    setDeletingGroup(group);
    try {
      const res = await API.delete(
        `/api/user/referral/subscription/invitees/${invitee.id}`,
        {
          params: { group },
        },
      );

      if (res.data?.success) {
        showSuccess(t('删除成功'));
        setDraftPercentByGroup((currentDrafts) => {
          const nextDrafts = { ...currentDrafts };
          delete nextDrafts[group];
          return nextDrafts;
        });
        await onOverridesChanged?.();
      } else {
        showError(res.data?.message || t('删除失败'));
      }
    } catch (error) {
      showError(error?.message || t('删除失败'));
    } finally {
      setDeletingGroup('');
    }
  };

  if (!invitee) {
    return (
      <Card
        className='!rounded-2xl border-0 shadow-sm h-full bg-white/95 backdrop-blur'
        title={t('邀请用户返佣详情')}
      >
        <Empty description={t('未选择邀请用户')} />
        <Typography.Text type='tertiary' className='mt-3 block text-center text-sm'>
          {t('从左侧选择一位邀请用户后，可查看返佣流水并单独设置返佣比例。')}
        </Typography.Text>
      </Card>
    );
  }

  return (
    <Card
      className='!rounded-2xl border-0 shadow-sm h-full bg-white/95 backdrop-blur'
      title={t('邀请用户返佣详情')}
      loading={loading}
      bodyStyle={{
        display: 'flex',
        flexDirection: 'column',
        gap: 16,
        minHeight: 640,
      }}
    >
      <div className='rounded-2xl border border-gray-100 bg-gradient-to-br from-gray-50 to-white p-4'>
        <div className='flex items-center justify-between gap-3'>
          <Typography.Title heading={5} style={{ margin: 0 }}>
            {invitee.username || `#${invitee.id}`}
          </Typography.Title>
          <Tag color='white'>{invitee.group || '-'}</Tag>
        </div>
        <div className='mt-3 flex flex-wrap gap-3 text-sm'>
          <Typography.Text type='secondary'>
            {t('累计返佣收益')}: {renderQuota(invitee.contribution_quota || 0)}
          </Typography.Text>
          <Typography.Text type='secondary'>
            {t('购买订单')}: {Number(invitee.order_count || 0)}
          </Typography.Text>
        </div>
        <Typography.Text type='tertiary' className='mt-2 block text-sm'>
          {t('未单独设置时，自动使用当前返佣方案默认值。')}
        </Typography.Text>
      </div>

      <div className='space-y-3'>
        <div className='flex items-center justify-between gap-3'>
          <Typography.Title heading={6} style={{ margin: 0 }}>
            {t('贡献流水')}
          </Typography.Title>
          <Typography.Text type='tertiary' className='text-sm'>
            {t('返佣明细')} · {t('这里只展示和你本人有关的返佣到账与返给对方明细。')}
          </Typography.Text>
        </div>
        <div className='grid grid-cols-2 gap-3 xl:grid-cols-5'>
          <div className='rounded-2xl border border-gray-100 bg-white px-4 py-3'>
            <Typography.Text type='tertiary' className='block text-xs'>
              {t('贡献订单数')}
            </Typography.Text>
            <Typography.Text strong className='text-base'>
              {contributionSummary.orderCount}
            </Typography.Text>
          </div>
          <div className='rounded-2xl border border-emerald-100 bg-emerald-50/70 px-4 py-3'>
            <Typography.Text type='tertiary' className='block text-xs'>
              {t('你累计到账')}
            </Typography.Text>
            <Typography.Text strong className='text-base'>
              {renderQuota(contributionSummary.ownRewardQuota)}
            </Typography.Text>
          </div>
          <div className='rounded-2xl border border-amber-100 bg-amber-50/70 px-4 py-3'>
            <Typography.Text type='tertiary' className='block text-xs'>
              {t('累计返给对方')}
            </Typography.Text>
            <Typography.Text strong className='text-base'>
              {renderQuota(contributionSummary.inviteeRewardQuota)}
            </Typography.Text>
          </div>
          <div className='rounded-2xl border border-emerald-100 bg-emerald-50/70 px-4 py-3'>
            <Typography.Text type='tertiary' className='block text-xs'>
              {t('直推分录')}
            </Typography.Text>
            <Typography.Text strong className='text-base'>
              {contributionSummary.directDetailCount}
            </Typography.Text>
          </div>
          <div className='rounded-2xl border border-sky-100 bg-sky-50/70 px-4 py-3'>
            <Typography.Text type='tertiary' className='block text-xs'>
              {t('团队分录')}
            </Typography.Text>
            <Typography.Text strong className='text-base'>
              {contributionSummary.teamDetailCount}
            </Typography.Text>
          </div>
        </div>

        <div className='rounded-2xl border border-gray-100 bg-white p-4'>
          <Typography.Text strong className='block text-sm'>
            {t('贡献概览')}
          </Typography.Text>

          <Tabs
            type='button'
            activeKey={contributionView}
            onChange={setContributionView}
            className='mt-3'
          >
            <TabPane tab={t('按订单看')} itemKey='orders'>
              <div className='space-y-4 pt-2'>
                <Typography.Text type='tertiary' className='block text-sm'>
                  {t('每笔订单先看总贡献，再看下面拆分。')}
                </Typography.Text>

                {normalizedContributionCards.length === 0 ? (
                  <Empty description={t('暂无返佣明细')} />
                ) : (
                  pagedContributionCards.map((card) => {
                    const detailItemCurrentPage = detailItemPageByCard[card.id] || 1;
                    const detailItemStart =
                      (detailItemCurrentPage - 1) * DETAIL_ITEM_PAGE_SIZE;
                    const pagedDetailItems = card.items.slice(
                      detailItemStart,
                      detailItemStart + DETAIL_ITEM_PAGE_SIZE,
                    );

                    return (
                      <div
                        key={card.id}
                        className='rounded-2xl border border-gray-100 bg-gradient-to-br from-white to-gray-50 p-4'
                      >
                        <div className='flex flex-wrap items-start justify-between gap-3'>
                          <div className='min-w-0 space-y-1'>
                            <div>
                              <Typography.Text
                                type='tertiary'
                                className='block text-xs'
                              >
                                {t('订单号')}
                              </Typography.Text>
                              <Typography.Text
                                strong
                                className='block max-w-full break-all leading-snug'
                              >
                                {card.tradeNo || '-'}
                              </Typography.Text>
                            </div>
                            <div className='flex flex-wrap gap-2'>
                              <Tag color='white'>{card.group || '-'}</Tag>
                              <Tag color={renderContributionStatusColor(card.status)}>
                                {formatContributionStatusLabel(card.status)}
                              </Tag>
                            </div>
                          </div>
                          <div className='grid grid-cols-1 gap-2 text-right sm:grid-cols-2 sm:gap-4'>
                            <div>
                              <Typography.Text
                                type='tertiary'
                                className='block text-xs'
                              >
                                {t('你的到账返佣')}
                              </Typography.Text>
                              <Typography.Text strong>
                                {renderQuota(card.ownRewardQuota || 0)}
                              </Typography.Text>
                            </div>
                            <div>
                              <Typography.Text
                                type='tertiary'
                                className='block text-xs'
                              >
                                {t('返给对方')}
                              </Typography.Text>
                              <Typography.Text strong>
                                {renderQuota(card.inviteeRewardQuota || 0)}
                              </Typography.Text>
                            </div>
                            <div className='sm:col-span-2'>
                              <Typography.Text
                                type='tertiary'
                                className='block text-xs'
                              >
                                {t('结算时间')}
                              </Typography.Text>
                              <Typography.Text>
                                {card.settledAt ? timestamp2string(card.settledAt) : '-'}
                              </Typography.Text>
                            </div>
                          </div>
                        </div>

                        <div className='mt-4 border-t border-gray-100 pt-4'>
                          <Typography.Text type='tertiary' className='block text-xs'>
                            {t('本单贡献拆分')}
                          </Typography.Text>

                          <div className='mt-3 space-y-3'>
                            {pagedDetailItems.map((item) => {
                              return (
                                <div
                                  key={item.id}
                                  className={`rounded-xl border px-4 py-3 ${renderContributionSurfaceClass(item)}`}
                                >
                                  <div className='flex flex-wrap items-center justify-between gap-3'>
                                    <div className='flex flex-wrap gap-2'>
                                      <Tag color={renderContributionRoleColor(item.roleType)}>
                                        {t(item.roleLabel)}
                                      </Tag>
                                      <Tag color={renderContributionComponentColor(item)}>
                                        {t(item.componentLabel)}
                                      </Tag>
                                    </div>
                                    <Typography.Text strong>
                                      {renderQuota(item.effectiveRewardQuota || 0)}
                                    </Typography.Text>
                                  </div>

                                  <div className='mt-3 grid grid-cols-1 gap-3 text-sm md:grid-cols-3'>
                                    <div>
                                      <Typography.Text
                                        type='tertiary'
                                        className='block text-xs'
                                      >
                                        {t('本笔身份')}
                                      </Typography.Text>
                                      <Typography.Text>{t(item.roleLabel)}</Typography.Text>
                                    </div>
                                    <div>
                                      <Typography.Text
                                        type='tertiary'
                                        className='block text-xs'
                                      >
                                        {t('返佣类型')}
                                      </Typography.Text>
                                      <Typography.Text>
                                        {t(item.componentLabel)}
                                      </Typography.Text>
                                    </div>
                                    <div>
                                      <Typography.Text
                                        type='tertiary'
                                        className='block text-xs'
                                      >
                                        {t('状态')}
                                      </Typography.Text>
                                      <Typography.Text>
                                        {formatContributionStatusLabel(item.status)}
                                      </Typography.Text>
                                    </div>
                                  </div>
                                </div>
                              );
                            })}
                          </div>
                        </div>

                        <ListPagination
                          className='mt-4'
                          showRangeSummary={false}
                          total={card.items.length}
                          pageSize={DETAIL_ITEM_PAGE_SIZE}
                          currentPage={detailItemCurrentPage}
                          pageSizeOpts={[DETAIL_ITEM_PAGE_SIZE]}
                          showSizeChanger={false}
                          onPageChange={(page) =>
                            setDetailItemPageByCard((currentPages) => ({
                              ...currentPages,
                              [card.id]: page,
                            }))
                          }
                        />
                      </div>
                    );
                  })
                )}

                <ListPagination
                  total={normalizedContributionCards.length}
                  pageSize={CONTRIBUTION_PAGE_SIZE}
                  currentPage={contributionPage}
                  pageSizeOpts={[CONTRIBUTION_PAGE_SIZE]}
                  showSizeChanger={false}
                  onPageChange={setContributionPage}
                />
              </div>
            </TabPane>

            <TabPane tab={t('按分录看')} itemKey='ledger'>
              <div className='space-y-4 pt-2'>
                <Typography.Text type='tertiary' className='block text-sm'>
                  {t('按到账分录逐条看清楚每一笔返佣来源。')}
                </Typography.Text>

                {contributionLedgerRows.length === 0 ? (
                  <Empty description={t('暂无返佣明细')} />
                ) : (
                  <div className='space-y-3'>
                    <Typography.Text strong className='block text-sm'>
                      {t('贡献分录明细')}
                    </Typography.Text>

                    {pagedContributionLedgerRows.map((item) => (
                      <div
                        key={item.id}
                        className={`rounded-xl border px-4 py-3 ${renderContributionSurfaceClass(item)}`}
                      >
                        <div className='flex flex-wrap items-center justify-between gap-3'>
                          <div className='flex flex-wrap gap-2'>
                            <Tag color='white'>{item.group || '-'}</Tag>
                            <Tag color={renderContributionRoleColor(item.roleType)}>
                              {t(item.roleLabel)}
                            </Tag>
                            <Tag color={renderContributionComponentColor(item)}>
                              {t(item.componentLabel)}
                            </Tag>
                            <Tag color={renderContributionStatusColor(item.status)}>
                              {formatContributionStatusLabel(item.status)}
                            </Tag>
                          </div>
                          <Typography.Text strong>
                            {renderQuota(item.effectiveRewardQuota || 0)}
                          </Typography.Text>
                        </div>

                        <div className='mt-3 grid grid-cols-1 gap-3 text-sm sm:grid-cols-2 xl:grid-cols-4'>
                          <div className='min-w-0'>
                            <Typography.Text type='tertiary' className='block text-xs'>
                              {t('订单号')}
                            </Typography.Text>
                            <Typography.Text className='block max-w-full break-all leading-snug'>
                              {item.tradeNo || '-'}
                            </Typography.Text>
                          </div>
                          <div className='min-w-0'>
                            <Typography.Text type='tertiary' className='block text-xs'>
                              {t('本笔身份')}
                            </Typography.Text>
                            <Typography.Text className='block max-w-full break-words leading-snug'>
                              {t(item.roleLabel)}
                            </Typography.Text>
                          </div>
                          <div className='min-w-0'>
                            <Typography.Text type='tertiary' className='block text-xs'>
                              {t('返佣类型')}
                            </Typography.Text>
                            <Typography.Text className='block max-w-full break-words leading-snug'>
                              {t(item.componentLabel)}
                            </Typography.Text>
                          </div>
                          <div className='min-w-0'>
                            <Typography.Text type='tertiary' className='block text-xs'>
                              {t('结算时间')}
                            </Typography.Text>
                            <Typography.Text className='block max-w-full break-words leading-snug'>
                              {item.settledAt ? timestamp2string(item.settledAt) : '-'}
                            </Typography.Text>
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                )}

                <ListPagination
                  total={contributionLedgerRows.length}
                  pageSize={LEDGER_PAGE_SIZE}
                  currentPage={ledgerPage}
                  pageSizeOpts={[LEDGER_PAGE_SIZE]}
                  showSizeChanger={false}
                  onPageChange={setLedgerPage}
                />
              </div>
            </TabPane>
          </Tabs>
        </div>
      </div>

      <div className='space-y-3'>
        <div className='flex items-center justify-between gap-3'>
          <Typography.Title heading={6} style={{ margin: 0 }}>
            {t('单独返佣设置')}
          </Typography.Title>
          <Typography.Text type='tertiary' className='text-sm'>
            {t('未单独设置时，自动使用当前返佣方案默认值。')}
          </Typography.Text>
        </div>

        {normalizedRows.length === 0 ? (
          <Empty description={t('暂无可覆盖的模板作用域')} />
        ) : (
          pagedOverrideRows.map((row) => {
            const group = String(row?.group || '').trim();
            const inputPercent = getDraftPercent(row);
            const currentPercent = Number(row?.inputPercent || 0);
            const draftInviteeRateBps = clampInviteeRateBps(
              percentNumberToRateBps(inputPercent),
              row?.effectiveTotalRateBps,
            );
            const canSave =
              !savingGroup &&
              draftInviteeRateBps !== percentNumberToRateBps(currentPercent);

            return (
              <div
                key={row.id}
                className='rounded-2xl border border-gray-100 bg-white p-4'
              >
                <div className='flex flex-wrap items-center justify-between gap-3'>
                  <div>
                    <Typography.Text type='tertiary' className='block text-xs'>
                      {t('当前返佣方案')}
                    </Typography.Text>
                    <Typography.Text strong>
                      {row.templateName || '-'}
                    </Typography.Text>
                  </div>
                  <Tag color={row.hasOverride ? 'green' : 'grey'}>
                    {row.hasOverride ? t('已单独设置返佣') : t('使用默认')}
                  </Tag>
                </div>

                <div className='mt-4 grid grid-cols-1 gap-4 md:grid-cols-4'>
                  <div>
                    <Typography.Text type='tertiary' className='block text-xs'>
                      {t('返佣模式')}
                    </Typography.Text>
                    <Typography.Text strong>
                      {formatModeLabel(row?.levelType)}
                    </Typography.Text>
                  </div>
                  <div>
                    <Typography.Text type='tertiary' className='block text-xs'>
                      {t('所在分组')}
                    </Typography.Text>
                    <Typography.Text strong>{group}</Typography.Text>
                  </div>
                  <div>
                    <Typography.Text type='tertiary' className='block text-xs'>
                      {t('本组总返佣比例')}
                    </Typography.Text>
                    <Typography.Text strong>
                      {formatRateBpsPercent(row.effectiveTotalRateBps)}
                    </Typography.Text>
                  </div>
                </div>

                <div className='mt-4 grid grid-cols-1 gap-4 md:grid-cols-3'>
                  <div>
                    <Typography.Text type='tertiary' className='block text-xs'>
                      {t('默认返给对方比例')}
                    </Typography.Text>
                    <Typography.Text strong>
                      {formatRateBpsPercent(row.defaultInviteeRateBps)}
                    </Typography.Text>
                  </div>
                  <div>
                    <Typography.Text type='tertiary' className='block text-xs'>
                      {t('实际返给对方比例')}
                    </Typography.Text>
                    <Typography.Text strong>
                      {formatRateBpsPercent(
                        canSave ? draftInviteeRateBps : row.effectiveInviteeRateBps,
                      )}
                    </Typography.Text>
                  </div>
                  <div>
                    <Typography.Text type='tertiary' className='block text-xs'>
                      {t('你本单保留比例')}
                    </Typography.Text>
                    <Typography.Text strong>
                      {formatRateBpsPercent(
                        Math.max(0, row.effectiveTotalRateBps - draftInviteeRateBps),
                      )}
                    </Typography.Text>
                  </div>
                </div>

                <div className='mt-4'>
                  <Typography.Text type='tertiary' className='block text-xs'>
                    {t('被邀请人返佣比例')}
                  </Typography.Text>
                  <InputNumber
                    value={inputPercent}
                    min={0}
                    max={rateBpsToPercentNumber(row.effectiveTotalRateBps)}
                    step={0.01}
                    precision={2}
                    suffix='%'
                    className='w-full'
                    style={{ width: '100%' }}
                    onChange={(value) => updateDraftPercent(group, value)}
                  />
                </div>

                <Space className='mt-4 flex-wrap'>
                  <Button
                    type='primary'
                    loading={savingGroup === group}
                    disabled={!canSave}
                    onClick={() => saveOverride(row)}
                  >
                    {t('保存')}
                  </Button>
                  <Button
                    theme='borderless'
                    type='danger'
                    disabled={!row.hasOverride}
                    loading={deletingGroup === group}
                    onClick={() => deleteOverride(group)}
                  >
                    {t('删除')}
                  </Button>
                </Space>
              </div>
            );
          })
        )}

        <ListPagination
          total={normalizedRows.length}
          pageSize={OVERRIDE_PAGE_SIZE}
          currentPage={overridePage}
          pageSizeOpts={[OVERRIDE_PAGE_SIZE]}
          showSizeChanger={false}
          onPageChange={setOverridePage}
        />
      </div>
    </Card>
  );
};

export default InviteeOverridePanel;
