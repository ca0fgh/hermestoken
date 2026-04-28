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
