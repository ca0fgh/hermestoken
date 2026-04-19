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

export const formatWithdrawalAmount = (amount, symbol = '¥') => {
  const numericAmount = Number(amount || 0);
  return `${symbol}${numericAmount.toFixed(2)}`;
};

export const getWithdrawalCurrencySymbol = (currency, fallback = '¥') => {
  switch (String(currency || '').toUpperCase()) {
    case 'USD':
      return '$';
    case 'CNY':
      return '¥';
    case 'CUSTOM':
      return fallback;
    default:
      return fallback;
  }
};

export const maskAlipayAccount = (account) => {
  const value = String(account || '').trim();
  if (value.length <= 6) return value;
  return `${value.slice(0, 3)}***${value.slice(-3)}`;
};

export const getWithdrawalStatusMeta = (status, t) => {
  switch (status) {
    case 'pending':
      return { color: 'orange', text: t('待审核') };
    case 'approved':
      return { color: 'blue', text: t('待打款') };
    case 'paid':
      return { color: 'green', text: t('已打款') };
    case 'rejected':
      return { color: 'red', text: t('已驳回') };
    default:
      return { color: 'grey', text: status || '--' };
  }
};

export const calculateWithdrawalPreview = (amount, feeRules = []) => {
  const numericAmount = Number(amount || 0);
  if (!Number.isFinite(numericAmount) || numericAmount <= 0) {
    return {
      feeAmount: 0,
      netAmount: 0,
      matchedRule: null,
    };
  }

  const matchedRule =
    feeRules.find((rule) => {
      const min = Number(rule?.min_amount || 0);
      const max = Number(rule?.max_amount || 0);
      if (numericAmount < min) return false;
      if (max > 0 && numericAmount >= max) return false;
      return true;
    }) || null;

  if (!matchedRule) {
    return {
      feeAmount: 0,
      netAmount: numericAmount,
      matchedRule: null,
    };
  }

  let feeAmount = 0;
  if (matchedRule.fee_type === 'fixed') {
    feeAmount = Number(matchedRule.fee_value || 0);
  } else {
    feeAmount = numericAmount * (Number(matchedRule.fee_value || 0) / 100);
    const minFee = Number(matchedRule.min_fee || 0);
    const maxFee = Number(matchedRule.max_fee || 0);
    if (minFee > 0) {
      feeAmount = Math.max(feeAmount, minFee);
    }
    if (maxFee > 0) {
      feeAmount = Math.min(feeAmount, maxFee);
    }
  }

  feeAmount = Number(feeAmount.toFixed(2));
  return {
    feeAmount,
    netAmount: Number(Math.max(numericAmount - feeAmount, 0).toFixed(2)),
    matchedRule,
  };
};

export const normalizeWithdrawalConfig = (config) => ({
  enabled: config?.enabled === true,
  minAmount: Number(config?.min_amount || 0),
  instruction: config?.instruction || '',
  feeRules: Array.isArray(config?.fee_rules) ? config.fee_rules : [],
  hasOpenWithdrawal: config?.has_open_withdrawal === true,
  currency: config?.currency || 'CNY',
  currencySymbol: config?.currency_symbol || '¥',
  quotaDisplayType: config?.quota_display_type || 'CNY',
  exchangeRate: Number(config?.exchange_rate || 1),
  availableQuota: Number(config?.available_quota || 0),
  frozenQuota: Number(config?.frozen_quota || 0),
  availableAmount: Number(config?.available_amount || 0),
  frozenAmount: Number(config?.frozen_amount || 0),
});
