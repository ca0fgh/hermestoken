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

import React from 'react';
import CardPro from '../../common/ui/CardPro';
import WithdrawalsTable from './WithdrawalsTable';
import WithdrawalsActions from './WithdrawalsActions';
import WithdrawalsFilters from './WithdrawalsFilters';
import WithdrawalsDescription from './WithdrawalsDescription';
import WithdrawalReviewModal from './modals/WithdrawalReviewModal';
import WithdrawalPaidModal from './modals/WithdrawalPaidModal';
import { useWithdrawalsData } from '../../../hooks/withdrawals/useWithdrawalsData';
import { useIsMobile } from '../../../hooks/common/useIsMobile';
import { createCardProPagination } from '../../../helpers/utils';

const WithdrawalsPage = () => {
  const withdrawalsData = useWithdrawalsData();
  const isMobile = useIsMobile();

  return (
    <>
      <WithdrawalReviewModal
        visible={withdrawalsData.showReviewModal}
        onCancel={() => withdrawalsData.setShowReviewModal(false)}
        onSubmit={withdrawalsData.submitReview}
        record={withdrawalsData.currentWithdrawal}
        mode={withdrawalsData.reviewMode}
        t={withdrawalsData.t}
      />
      <WithdrawalPaidModal
        visible={withdrawalsData.showPaidModal}
        onCancel={() => withdrawalsData.setShowPaidModal(false)}
        onSubmit={withdrawalsData.submitMarkPaid}
        record={withdrawalsData.currentWithdrawal}
        t={withdrawalsData.t}
      />

      <CardPro
        type='type1'
        descriptionArea={
          <WithdrawalsDescription
            t={withdrawalsData.t}
            compactMode={withdrawalsData.compactMode}
            setCompactMode={withdrawalsData.setCompactMode}
          />
        }
        actionsArea={
          <div className='flex flex-col md:flex-row justify-between items-center gap-2 w-full'>
            <WithdrawalsActions
              t={withdrawalsData.t}
              refresh={withdrawalsData.refresh}
              loading={withdrawalsData.loading}
            />
            <WithdrawalsFilters
              t={withdrawalsData.t}
              filters={withdrawalsData.filters}
              setFilters={withdrawalsData.setFilters}
              onSearch={withdrawalsData.searchWithdrawals}
              loading={withdrawalsData.searching}
            />
          </div>
        }
        paginationArea={createCardProPagination({
          currentPage: withdrawalsData.activePage,
          pageSize: withdrawalsData.pageSize,
          total: withdrawalsData.withdrawalCount,
          onPageChange: withdrawalsData.handlePageChange,
          onPageSizeChange: withdrawalsData.handlePageSizeChange,
          isMobile,
        })}
        t={withdrawalsData.t}
      >
        <WithdrawalsTable {...withdrawalsData} />
      </CardPro>
    </>
  );
};

export default WithdrawalsPage;
