import { getCurrencyConfig } from '../../helpers/quota.js';

export function getTopUpUSDExchangeRate() {
  const statusStr = localStorage.getItem('status');
  try {
    if (statusStr) {
      const status = JSON.parse(statusStr);
      const rate = Number(status?.usd_exchange_rate);
      if (Number.isFinite(rate) && rate > 0) {
        return rate;
      }
    }
  } catch {}
  return 1;
}

function convertUSDToDisplayAmount(usdAmount) {
  const { rate, type } = getCurrencyConfig();
  if (type === 'TOKENS') {
    return usdAmount;
  }
  return usdAmount * (Number(rate) || 1);
}

export function formatTopUpPaymentAmount(
  amount,
  sourceCurrency = 'CNY',
  t = (key) => key,
) {
  const numericAmount = Number(amount || 0);
  if (!Number.isFinite(numericAmount)) {
    return `0.00 ${t('元')}`;
  }

  const { symbol, type } = getCurrencyConfig();
  const normalizedSourceCurrency = String(
    sourceCurrency || 'CNY',
  ).toUpperCase();
  const usdExchangeRate = getTopUpUSDExchangeRate();
  const usdAmount =
    normalizedSourceCurrency === 'CNY'
      ? numericAmount / usdExchangeRate
      : numericAmount;

  if (type === 'CNY') {
    const cnyAmount =
      normalizedSourceCurrency === 'CNY'
        ? numericAmount
        : numericAmount * usdExchangeRate;
    return `¥${cnyAmount.toFixed(2)}`;
  }

  if (type === 'TOKENS') {
    return `${numericAmount.toFixed(2)} ${t('元')}`;
  }

  return `${symbol}${convertUSDToDisplayAmount(usdAmount).toFixed(2)}`;
}
