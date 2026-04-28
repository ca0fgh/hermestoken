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
function formatTrimmedAmount(amount, digits = 6) {
  return amount
    .toFixed(digits)
    .replace(/(\.\d*?[1-9])0+$/, '$1')
    .replace(/\.0+$/, '');
}

export function formatTopUpPaymentAmount(
  amount,
  sourceCurrency = 'CNY',
  t = (key) => key,
) {
  const numericAmount = Number(amount || 0);
  const normalizedSourceCurrency = String(
    sourceCurrency || 'CNY',
  ).toUpperCase();
  if (!Number.isFinite(numericAmount)) {
    if (normalizedSourceCurrency === 'USD') {
      return '$0.00';
    }
    if (normalizedSourceCurrency === 'USDT') {
      return '0 USDT';
    }
    return `¥0.00`;
  }

  if (normalizedSourceCurrency === 'CNY') {
    return `¥${numericAmount.toFixed(2)}`;
  }
  if (normalizedSourceCurrency === 'USD') {
    return `$${numericAmount.toFixed(2)}`;
  }
  if (normalizedSourceCurrency === 'USDT') {
    return `${formatTrimmedAmount(numericAmount)} USDT`;
  }
  if (normalizedSourceCurrency === 'TOKENS') {
    return `${numericAmount.toFixed(2)} ${t('元')}`;
  }

  return `${normalizedSourceCurrency} ${numericAmount.toFixed(2)}`;
}

export function formatTopUpPresetSettlementSummary({
  actualPay,
  save = 0,
  currency = 'CNY',
  hasDiscount = false,
  t = (key) => key,
}) {
  const formattedActualPay = formatTopUpPaymentAmount(actualPay, currency, t);
  const formattedSave = formatTopUpPaymentAmount(
    hasDiscount ? save : 0,
    currency,
    t,
  );

  return `${t('实付')} ${formattedActualPay}，${t('节省')} ${formattedSave}`;
}
