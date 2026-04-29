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
export function renderNumber(num) {
  if (num >= 1000000000) {
    return (num / 1000000000).toFixed(1) + 'B';
  } else if (num >= 1000000) {
    return (num / 1000000).toFixed(1) + 'M';
  } else if (num >= 10000) {
    return (num / 1000).toFixed(1) + 'k';
  } else {
    return num;
  }
}

export function renderQuotaNumberWithDigit(num, digits = 2) {
  if (typeof num !== 'number' || isNaN(num)) {
    return 0;
  }

  const quotaDisplayType = localStorage.getItem('quota_display_type') || 'USD';
  const fixedNumber = num.toFixed(digits);

  if (quotaDisplayType === 'CNY') {
    return '¥' + fixedNumber;
  }
  if (quotaDisplayType === 'USD') {
    return '$' + fixedNumber;
  }
  if (quotaDisplayType === 'CUSTOM') {
    const statusStr = localStorage.getItem('status');
    let symbol = '¤';
    try {
      if (statusStr) {
        const status = JSON.parse(statusStr);
        symbol = status?.custom_currency_symbol || symbol;
      }
    } catch {}
    return symbol + fixedNumber;
  }

  return fixedNumber;
}

export function getQuotaPerUnit() {
  const quotaPerUnit = parseFloat(localStorage.getItem('quota_per_unit'));
  return quotaPerUnit;
}

export function renderUnitWithQuota(quota) {
  const quotaPerUnit = parseFloat(localStorage.getItem('quota_per_unit'));
  return quotaPerUnit * parseFloat(quota);
}

export function getQuotaWithUnit(quota, digits = 6) {
  const quotaPerUnit = parseFloat(localStorage.getItem('quota_per_unit'));
  return (quota / quotaPerUnit).toFixed(digits);
}

export function renderQuotaWithAmount(amount) {
  const quotaDisplayType = localStorage.getItem('quota_display_type') || 'USD';
  if (quotaDisplayType === 'TOKENS') {
    return renderNumber(renderUnitWithQuota(amount));
  }

  const numericAmount = Number(amount);
  const formattedAmount = Number.isFinite(numericAmount)
    ? numericAmount.toFixed(2)
    : amount;

  if (quotaDisplayType === 'CNY') {
    return '¥' + formattedAmount;
  }

  if (quotaDisplayType === 'CUSTOM') {
    const statusStr = localStorage.getItem('status');
    let symbol = '¤';
    try {
      if (statusStr) {
        const status = JSON.parse(statusStr);
        symbol = status?.custom_currency_symbol || symbol;
      }
    } catch {}
    return symbol + formattedAmount;
  }

  return '$' + formattedAmount;
}

export function getCurrencyConfig() {
  const quotaDisplayType = localStorage.getItem('quota_display_type') || 'USD';
  const statusStr = localStorage.getItem('status');

  let symbol = '$';
  let rate = 1;

  if (quotaDisplayType === 'CNY') {
    symbol = '¥';
    try {
      if (statusStr) {
        const status = JSON.parse(statusStr);
        rate = status?.usd_exchange_rate || 7;
      }
    } catch {}
  } else if (quotaDisplayType === 'CUSTOM') {
    try {
      if (statusStr) {
        const status = JSON.parse(statusStr);
        symbol = status?.custom_currency_symbol || '¤';
        rate = status?.custom_currency_exchange_rate || 1;
      }
    } catch {}
  }

  return { symbol, rate, type: quotaDisplayType };
}

export function convertUSDToCurrency(usdAmount, digits = 2) {
  const { symbol, rate } = getCurrencyConfig();
  return symbol + (usdAmount * rate).toFixed(digits);
}

export function renderQuota(quota, digits = 2) {
  const quotaPerUnit = parseFloat(localStorage.getItem('quota_per_unit'));
  const quotaDisplayType = localStorage.getItem('quota_display_type') || 'USD';

  if (quotaDisplayType === 'TOKENS') {
    return renderNumber(quota);
  }

  const resultUSD = quota / quotaPerUnit;
  let symbol = '$';
  let value = resultUSD;

  if (quotaDisplayType === 'CNY') {
    const statusStr = localStorage.getItem('status');
    let usdRate = 1;
    try {
      if (statusStr) {
        const status = JSON.parse(statusStr);
        usdRate = status?.usd_exchange_rate || 1;
      }
    } catch {}
    value = resultUSD * usdRate;
    symbol = '¥';
  } else if (quotaDisplayType === 'CUSTOM') {
    const statusStr = localStorage.getItem('status');
    let symbolCustom = '¤';
    let rate = 1;
    try {
      if (statusStr) {
        const status = JSON.parse(statusStr);
        symbolCustom = status?.custom_currency_symbol || symbolCustom;
        rate = status?.custom_currency_exchange_rate || rate;
      }
    } catch {}
    value = resultUSD * rate;
    symbol = symbolCustom;
  }

  const fixedResult = value.toFixed(digits);
  if (parseFloat(fixedResult) === 0 && quota > 0 && value > 0) {
    const minValue = Math.pow(10, -digits);
    return symbol + minValue.toFixed(digits);
  }
  return symbol + fixedResult;
}

export function renderQuotaWithLessThanFloor(quota, digits = 2) {
  const numericQuota = Number(quota || 0);
  if (!Number.isFinite(numericQuota) || numericQuota <= 0) {
    return renderQuota(numericQuota, digits);
  }

  const { symbol, type } = getCurrencyConfig();
  if (type === 'TOKENS') {
    return renderQuota(numericQuota, digits);
  }

  const displayAmount = quotaToDisplayAmount(numericQuota);
  const minValue = Math.pow(10, -digits);
  if (displayAmount > 0 && displayAmount < minValue) {
    return `<${symbol}${minValue.toFixed(digits)}`;
  }

  return renderQuota(numericQuota, digits);
}

export function quotaToDisplayAmount(quota) {
  const numericQuota = Number(quota || 0);
  if (!Number.isFinite(numericQuota) || numericQuota === 0) {
    return 0;
  }

  const sign = Math.sign(numericQuota);
  const absQuota = Math.abs(numericQuota);
  const { type, rate } = getCurrencyConfig();
  if (type === 'TOKENS') {
    return numericQuota;
  }

  const displayAmount = absQuota / getQuotaPerUnit();
  if (type === 'USD') {
    return sign * displayAmount;
  }

  return sign * displayAmount * (rate || 1);
}

export function displayAmountToQuota(amount) {
  const numericAmount = Number(amount || 0);
  if (!Number.isFinite(numericAmount) || numericAmount === 0) {
    return 0;
  }

  const sign = Math.sign(numericAmount);
  const absAmount = Math.abs(numericAmount);
  const { type, rate } = getCurrencyConfig();
  if (type === 'TOKENS') {
    return Math.round(numericAmount);
  }

  const usdAmount = type === 'USD' ? absAmount : absAmount / (rate || 1);
  return sign * Math.round(usdAmount * getQuotaPerUnit());
}
