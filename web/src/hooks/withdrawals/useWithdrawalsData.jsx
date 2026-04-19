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

import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../../helpers';
import { ITEMS_PER_PAGE } from '../../constants';
import { useTableCompactMode } from '../common/useTableCompactMode';

export const useWithdrawalsData = () => {
  const { t } = useTranslation();
  const [compactMode, setCompactMode] = useTableCompactMode('withdrawals');
  const [withdrawals, setWithdrawals] = useState([]);
  const [loading, setLoading] = useState(true);
  const [searching, setSearching] = useState(false);
  const [activePage, setActivePage] = useState(1);
  const [pageSize, setPageSize] = useState(ITEMS_PER_PAGE);
  const [withdrawalCount, setWithdrawalCount] = useState(0);
  const [filters, setFilters] = useState({
    keyword: '',
    status: '',
  });
  const [showReviewModal, setShowReviewModal] = useState(false);
  const [reviewMode, setReviewMode] = useState('approve');
  const [currentWithdrawal, setCurrentWithdrawal] = useState(null);
  const [showPaidModal, setShowPaidModal] = useState(false);

  const loadWithdrawals = async (
    page = activePage,
    size = pageSize,
    nextFilters = filters,
  ) => {
    setLoading(true);
    try {
      const params = new URLSearchParams({
        p: String(page),
        page_size: String(size),
      });
      if (nextFilters.status) params.set('status', nextFilters.status);
      if (nextFilters.keyword) params.set('keyword', nextFilters.keyword);

      const res = await API.get(`/api/admin/withdrawals?${params.toString()}`);
      const { success, message, data } = res.data;
      if (success) {
        setWithdrawals(
          (data.items || []).map((item) => ({ ...item, key: item.id })),
        );
        setWithdrawalCount(data.total || 0);
        setActivePage(data.page || page);
      } else {
        showError(message || t('加载提现列表失败'));
      }
    } catch (error) {
      showError(t('加载提现列表失败'));
    } finally {
      setLoading(false);
    }
  };

  const searchWithdrawals = async (nextFilters = filters) => {
    setSearching(true);
    setActivePage(1);
    setFilters(nextFilters);
    await loadWithdrawals(1, pageSize, nextFilters);
    setSearching(false);
  };

  const handlePageChange = (page) => {
    setActivePage(page);
    loadWithdrawals(page, pageSize, filters).then();
  };

  const handlePageSizeChange = (size) => {
    localStorage.setItem('page-size', size + '');
    setPageSize(size);
    setActivePage(1);
    loadWithdrawals(1, size, filters).then();
  };

  const openApproveModal = (record) => {
    setCurrentWithdrawal(record);
    setReviewMode('approve');
    setShowReviewModal(true);
  };

  const openRejectModal = (record) => {
    setCurrentWithdrawal(record);
    setReviewMode('reject');
    setShowReviewModal(true);
  };

  const openPaidModal = (record) => {
    setCurrentWithdrawal(record);
    setShowPaidModal(true);
  };

  const submitReview = async (note) => {
    if (!currentWithdrawal?.id) return;
    const endpoint =
      reviewMode === 'approve'
        ? `/api/admin/withdrawals/${currentWithdrawal.id}/approve`
        : `/api/admin/withdrawals/${currentWithdrawal.id}/reject`;
    const payload =
      reviewMode === 'approve'
        ? { review_note: note }
        : { rejection_note: note };

    const res = await API.post(endpoint, payload);
    const { success, message } = res.data;
    if (success) {
      showSuccess(reviewMode === 'approve' ? t('审核通过成功') : t('驳回成功'));
      setShowReviewModal(false);
      setCurrentWithdrawal(null);
      loadWithdrawals(activePage, pageSize, filters).then();
    } else {
      showError(message || t('操作失败'));
    }
  };

  const submitMarkPaid = async ({ payReceiptNo, payReceiptUrl, paidNote }) => {
    if (!currentWithdrawal?.id) return;
    const res = await API.post(
      `/api/admin/withdrawals/${currentWithdrawal.id}/mark-paid`,
      {
        pay_receipt_no: payReceiptNo,
        pay_receipt_url: payReceiptUrl,
        paid_note: paidNote,
      },
    );
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('确认打款成功'));
      setShowPaidModal(false);
      setCurrentWithdrawal(null);
      loadWithdrawals(activePage, pageSize, filters).then();
    } else {
      showError(message || t('操作失败'));
    }
  };

  useEffect(() => {
    loadWithdrawals(1, pageSize, filters).then();
  }, []);

  return {
    withdrawals,
    loading,
    searching,
    activePage,
    pageSize,
    withdrawalCount,
    filters,
    setFilters,
    compactMode,
    setCompactMode,
    handlePageChange,
    handlePageSizeChange,
    loadWithdrawals,
    searchWithdrawals,
    refresh: () => loadWithdrawals(activePage, pageSize, filters),
    showReviewModal,
    setShowReviewModal,
    reviewMode,
    currentWithdrawal,
    showPaidModal,
    setShowPaidModal,
    openApproveModal,
    openRejectModal,
    openPaidModal,
    submitReview,
    submitMarkPaid,
    t,
  };
};
