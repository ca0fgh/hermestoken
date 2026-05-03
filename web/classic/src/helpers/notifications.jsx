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
import { toast } from 'react-toastify';
import { toastConstants } from '../constants';
import { MOBILE_BREAKPOINT } from '../hooks/common/useIsMobile';
import {
  getHttpStatusFromError,
  isUnauthorizedError,
  redirectToLoginWhenExpired,
} from './authError';
import { matchesMediaQuery } from './mediaQuery';

const HTMLToastContent = ({ htmlContent }) => {
  return <div dangerouslySetInnerHTML={{ __html: htmlContent }} />;
};

let showErrorOptions = { autoClose: toastConstants.ERROR_TIMEOUT };
let showSuccessOptions = { autoClose: toastConstants.SUCCESS_TIMEOUT };
let showInfoOptions = { autoClose: toastConstants.INFO_TIMEOUT };
let showWarningOptions = { autoClose: toastConstants.WARNING_TIMEOUT };
let showNoticeOptions = { autoClose: false };

const isMobileScreen = matchesMediaQuery(
  `(max-width: ${MOBILE_BREAKPOINT - 1}px)`,
);
if (isMobileScreen) {
  showErrorOptions.position = 'top-center';
  showSuccessOptions.position = 'top-center';
  showInfoOptions.position = 'top-center';
  showWarningOptions.position = 'top-center';
  showNoticeOptions.position = 'top-center';
}

export function showError(error) {
  console.error(error);
  if (isUnauthorizedError(error)) {
    redirectToLoginWhenExpired();
    return;
  }

  switch (getHttpStatusFromError(error)) {
    case 429:
      toast.error('错误：请求次数过多，请稍后再试！', showErrorOptions);
      return;
    case 500:
      toast.error('错误：服务器内部错误，请联系管理员！', showErrorOptions);
      return;
    case 405:
      toast.info('本站仅作演示之用，无服务端！', showInfoOptions);
      return;
    default:
      break;
  }

  const message = typeof error === 'string' ? error : error?.message;
  toast.error(`错误：${message || error}`, showErrorOptions);
}

export function showWarning(message) {
  toast.warning(message, showWarningOptions);
}

export function showSuccess(message) {
  toast.success(message, showSuccessOptions);
}

export function showInfo(message) {
  toast.info(message, showInfoOptions);
}

export function showNotice(message, isHTML = false) {
  if (isHTML) {
    toast(<HTMLToastContent htmlContent={message} />, showNoticeOptions);
    return;
  }

  toast.info(message, showNoticeOptions);
}
