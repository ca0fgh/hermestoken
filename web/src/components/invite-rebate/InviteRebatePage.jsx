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
import { Spin, Typography } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { API, showError } from '../../helpers';
import {
  createInviteeDetailRequestGuard,
  resolveInviteeSelectionAfterPageRefresh,
} from '../../helpers/inviteeDetailRequestGuard';
import {
  buildInviteDefaultRuleRows,
  buildInviteeOverrideRows,
  normalizeInviteeContributionPage,
} from '../../helpers/inviteRebate';
import InviteDefaultRuleSection from './InviteDefaultRuleSection';
import InviteRebateSummary from './InviteRebateSummary';
import InviteeListPanel from './InviteeListPanel';
import InviteeOverridePanel from './InviteeOverridePanel';

const initialPageState = {
  page: 1,
  page_size: 10,
  total: 0,
  invitee_count: 0,
  total_contribution_quota: 0,
  items: [],
};

const InviteRebatePage = () => {
  const { t } = useTranslation();
  const [defaultRuleRows, setDefaultRuleRows] = useState([]);
  const [inviteePage, setInviteePage] = useState(initialPageState);
  const [keyword, setKeyword] = useState('');
  const [queryKeyword, setQueryKeyword] = useState('');
  const [selectedInvitee, setSelectedInvitee] = useState(null);
  const [inviteeOverrideRows, setInviteeOverrideRows] = useState([]);
  const [loadingDefaults, setLoadingDefaults] = useState(false);
  const [loadingInvitees, setLoadingInvitees] = useState(false);
  const [loadingDetail, setLoadingDetail] = useState(false);
  const inviteeDetailRequestGuardRef = useRef(
    createInviteeDetailRequestGuard(),
  );

  const clearInviteeSelection = () => {
    inviteeDetailRequestGuardRef.current.clear();
    setSelectedInvitee(null);
    setInviteeOverrideRows([]);
  };

  const loadDefaultRules = async () => {
    setLoadingDefaults(true);
    try {
      const res = await API.get('/api/user/referral/subscription');
      if (res.data?.success) {
        setDefaultRuleRows(buildInviteDefaultRuleRows(res.data?.data?.groups));
      } else {
        setDefaultRuleRows([]);
        showError(res.data?.message || t('加载失败'));
      }
    } catch (error) {
      setDefaultRuleRows([]);
      showError(error?.message || t('加载失败'));
    } finally {
      setLoadingDefaults(false);
    }
  };

  const loadInvitees = async ({
    page = inviteePage.page,
    page_size = inviteePage.page_size,
    keyword: nextKeyword = queryKeyword,
  } = {}) => {
    setLoadingInvitees(true);
    try {
      const res = await API.get('/api/user/referral/subscription/invitees', {
        params: {
          page,
          page_size,
          keyword: nextKeyword,
        },
      });

      if (res.data?.success) {
        const nextPage = normalizeInviteeContributionPage(res.data?.data);
        setInviteePage(nextPage);
        setSelectedInvitee((currentInvitee) => {
          return resolveInviteeSelectionAfterPageRefresh({
            currentInvitee,
            nextItems: nextPage.items,
            requestGuard: inviteeDetailRequestGuardRef.current,
            onSelectionCleared: () => setInviteeOverrideRows([]),
          });
        });
      } else {
        setInviteePage(initialPageState);
        clearInviteeSelection();
        showError(res.data?.message || t('加载失败'));
      }
    } catch (error) {
      setInviteePage(initialPageState);
      clearInviteeSelection();
      showError(error?.message || t('加载失败'));
    } finally {
      setLoadingInvitees(false);
    }
  };

  const loadInviteeDetail = async (inviteeId) => {
    if (!inviteeId) {
      inviteeDetailRequestGuardRef.current.clear();
      setInviteeOverrideRows([]);
      return;
    }

    const detailRequest = inviteeDetailRequestGuardRef.current.begin(inviteeId);
    setLoadingDetail(true);
    try {
      const res = await API.get(
        `/api/user/referral/subscription/invitees/${inviteeId}`,
      );
      if (!inviteeDetailRequestGuardRef.current.isCurrent(detailRequest)) {
        return;
      }
      if (res.data?.success) {
        const data = res.data?.data || {};
        setSelectedInvitee((currentInvitee) => data.invitee || currentInvitee);
        setInviteeOverrideRows(buildInviteeOverrideRows(data));
      } else {
        setInviteeOverrideRows([]);
        showError(res.data?.message || t('加载失败'));
      }
    } catch (error) {
      if (!inviteeDetailRequestGuardRef.current.isCurrent(detailRequest)) {
        return;
      }
      setInviteeOverrideRows([]);
      showError(error?.message || t('加载失败'));
    } finally {
      if (inviteeDetailRequestGuardRef.current.isCurrent(detailRequest)) {
        setLoadingDetail(false);
      }
    }
  };

  useEffect(() => {
    loadDefaultRules();
  }, []);

  useEffect(() => {
    loadInvitees();
  }, []);

  useEffect(() => {
    loadInviteeDetail(selectedInvitee?.id);
  }, [selectedInvitee?.id]);

  const handleSearch = () => {
    setQueryKeyword(keyword);
    setInviteePage((currentPage) => ({
      ...currentPage,
      page: 1,
    }));
    loadInvitees({
      page: 1,
      page_size: inviteePage.page_size,
      keyword,
    });
  };

  const handlePageChange = ({ page, page_size }) => {
    loadInvitees({
      page,
      page_size,
      keyword: queryKeyword,
    });
  };

  const refreshOverrides = async () => {
    await Promise.all([
      loadDefaultRules(),
      loadInvitees({
        page: inviteePage.page,
        page_size: inviteePage.page_size,
        keyword: queryKeyword,
      }),
      loadInviteeDetail(selectedInvitee?.id),
    ]);
  };

  return (
    <div className='mt-[60px] px-2 pb-8'>
      <div className='mx-auto flex max-w-7xl flex-col gap-5'>
        <div className='space-y-2'>
          <Typography.Title heading={3} style={{ marginBottom: 0 }}>
            {t('邀请返佣')}
          </Typography.Title>
          <Typography.Text type='secondary' className='max-w-3xl block'>
            {t('按已授权订阅分组维护邀请分账，并为指定邀请用户设置独立返佣。')}
          </Typography.Text>
        </div>

        <InviteRebateSummary
          t={t}
          inviteeCount={inviteePage.invitee_count}
          totalContributionQuota={inviteePage.total_contribution_quota}
        />

        <InviteDefaultRuleSection
          t={t}
          rows={defaultRuleRows}
          loading={loadingDefaults}
        />

        <Spin spinning={loadingInvitees && inviteePage.items.length === 0}>
          <div className='grid grid-cols-1 gap-4 xl:grid-cols-[380px_minmax(0,1fr)] xl:items-start'>
            <div className='xl:sticky xl:top-4'>
              <InviteeListPanel
                t={t}
                loading={loadingInvitees}
                keyword={keyword}
                pageData={inviteePage}
                selectedInviteeId={selectedInvitee?.id || null}
                onKeywordChange={setKeyword}
                onSearch={handleSearch}
                onSelectInvitee={setSelectedInvitee}
                onPageChange={handlePageChange}
                emptyHint={
                  queryKeyword ? t('暂无邀请用户') : t('请输入用户名后点击搜索')
                }
              />
            </div>
            <div className='xl:min-h-[680px]'>
              <InviteeOverridePanel
                t={t}
                invitee={selectedInvitee}
                rows={inviteeOverrideRows}
                loading={loadingDetail}
                onOverridesChanged={refreshOverrides}
              />
            </div>
          </div>
        </Spin>
      </div>
    </div>
  );
};

export default InviteRebatePage;
