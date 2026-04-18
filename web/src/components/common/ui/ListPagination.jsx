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

import React from 'react';
import PropTypes from 'prop-types';
import { Pagination } from '@douyinfe/semi-ui';
import { useTranslation } from 'react-i18next';
import { useIsMobile } from '../../../hooks/common/useIsMobile';

const ListPagination = ({
  currentPage,
  pageSize,
  total,
  onPageChange,
  onPageSizeChange,
  pageSizeOpts = [10, 20, 50, 100],
  showSizeChanger = true,
  hideOnSinglePage = true,
  showRangeSummary = true,
  isMobile: isMobileOverride,
  showQuickJumper,
  size,
  className = 'pt-1',
  paginationClassName = '',
  ...paginationProps
}) => {
  const { t } = useTranslation();
  const viewportIsMobile = useIsMobile();
  const isMobile =
    typeof isMobileOverride === 'boolean'
      ? isMobileOverride
      : viewportIsMobile;

  if (!total || total <= 0) {
    return null;
  }

  if (hideOnSinglePage && total <= pageSize) {
    return null;
  }

  const start = (currentPage - 1) * pageSize + 1;
  const end = Math.min(currentPage * pageSize, total);
  const shouldShowRangeSummary = showRangeSummary && !isMobile;
  const summaryText = `${t('显示第')} ${start} ${t('条 - 第')} ${end} ${t('条，共')} ${total} ${t('条')}`;
  const containerClassName = shouldShowRangeSummary
    ? `${className} flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between`
    : `${className} flex justify-center`;
  const resolvedPaginationClassName = shouldShowRangeSummary
    ? `flex justify-end ${paginationClassName}`.trim()
    : paginationClassName;

  return (
    <div className={containerClassName.trim()}>
      {shouldShowRangeSummary ? (
        <span
          className='text-sm select-none'
          style={{ color: 'var(--semi-color-text-2)' }}
        >
          {summaryText}
        </span>
      ) : null}
      <div className={resolvedPaginationClassName}>
        <Pagination
          currentPage={currentPage}
          pageSize={pageSize}
          total={total}
          pageSizeOpts={pageSizeOpts}
          showSizeChanger={showSizeChanger}
          onPageSizeChange={onPageSizeChange}
          onPageChange={onPageChange}
          size={size || (isMobile ? 'small' : 'default')}
          showQuickJumper={
            typeof showQuickJumper === 'boolean' ? showQuickJumper : isMobile
          }
          {...paginationProps}
        />
      </div>
    </div>
  );
};

ListPagination.propTypes = {
  currentPage: PropTypes.number.isRequired,
  pageSize: PropTypes.number.isRequired,
  total: PropTypes.number.isRequired,
  onPageChange: PropTypes.func.isRequired,
  onPageSizeChange: PropTypes.func,
  pageSizeOpts: PropTypes.array,
  showSizeChanger: PropTypes.bool,
  hideOnSinglePage: PropTypes.bool,
  showRangeSummary: PropTypes.bool,
  isMobile: PropTypes.bool,
  showQuickJumper: PropTypes.bool,
  size: PropTypes.string,
  className: PropTypes.string,
  paginationClassName: PropTypes.string,
};

export default ListPagination;
