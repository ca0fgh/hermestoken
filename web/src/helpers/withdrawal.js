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

const DEFAULT_EDITOR_RULE = {
  id: '',
  minAmount: 0,
  maxAmount: '',
  feeType: 'fixed',
  feeValue: 0,
  minFee: 0,
  maxFee: '',
  enabled: true,
  sortOrder: 1,
};

const parseWithdrawalRulesInput = (feeRules) => {
  if (Array.isArray(feeRules)) {
    return feeRules;
  }

  if (typeof feeRules !== 'string' || feeRules.trim() === '') {
    return [];
  }

  try {
    const parsed = JSON.parse(feeRules);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
};

const toFiniteNumber = (value, fallback = 0) => {
  if (value === '' || value === null || value === undefined) {
    return fallback;
  }

  const numericValue = Number(value);
  return Number.isFinite(numericValue) ? numericValue : fallback;
};

const toOptionalNumber = (value) => {
  if (value === '' || value === null || value === undefined) {
    return '';
  }

  const numericValue = Number(value);
  return Number.isFinite(numericValue) ? numericValue : '';
};

const createEditorRuleId = (rule, index) =>
  String(rule?.id || rule?.key || `withdrawal-fee-rule-${index + 1}`);

const sortEditorRules = (rules) =>
  [...rules].sort((left, right) => {
    if (left.sortOrder !== right.sortOrder) {
      return left.sortOrder - right.sortOrder;
    }

    return left.minAmount - right.minAmount;
  });

const normalizeEditorRule = (rule, index) => {
  const hasEditorMaxAmount = Object.prototype.hasOwnProperty.call(rule || {}, 'maxAmount');
  const hasEditorMaxFee = Object.prototype.hasOwnProperty.call(rule || {}, 'maxFee');
  const minAmount = toFiniteNumber(rule?.minAmount ?? rule?.min_amount, 0);
  const rawMaxAmount = hasEditorMaxAmount ? rule?.maxAmount : rule?.max_amount;
  const rawMaxFee = hasEditorMaxFee ? rule?.maxFee : rule?.max_fee;
  const feeType =
    rule?.feeType === 'ratio' || rule?.fee_type === 'ratio' ? 'ratio' : 'fixed';

  return {
    ...DEFAULT_EDITOR_RULE,
    id: createEditorRuleId(rule, index),
    minAmount,
    maxAmount:
      hasEditorMaxAmount || Number(rawMaxAmount) !== 0
        ? toOptionalNumber(rawMaxAmount)
        : '',
    feeType,
    feeValue: toFiniteNumber(rule?.feeValue ?? rule?.fee_value, 0),
    minFee: toFiniteNumber(rule?.minFee ?? rule?.min_fee, 0),
    maxFee:
      feeType === 'ratio'
        ? hasEditorMaxFee || Number(rawMaxFee) !== 0
          ? toOptionalNumber(rawMaxFee)
          : ''
        : '',
    enabled: rule?.enabled !== false,
    sortOrder: toFiniteNumber(rule?.sortOrder ?? rule?.sort_order, index + 1),
  };
};

const normalizeStoredWithdrawalFeeRules = (feeRules = []) =>
  sortEditorRules(normalizeWithdrawalFeeEditorRules(feeRules)).map((rule, index) => ({
    min_amount: rule.minAmount,
    max_amount: rule.maxAmount === '' ? 0 : rule.maxAmount,
    fee_type: rule.feeType,
    fee_value: rule.feeValue,
    min_fee: rule.feeType === 'ratio' ? rule.minFee : 0,
    max_fee: rule.feeType === 'ratio' && rule.maxFee !== '' ? rule.maxFee : 0,
    enabled: rule.enabled,
    sort_order: index + 1,
  }));

const getRuleUpperBound = (rule) =>
  rule.maxAmount === '' ? Infinity : Number(rule.maxAmount);

const formatRuleNumber = (value) => {
  const numericValue = Number(value || 0);
  if (Number.isInteger(numericValue)) {
    return String(numericValue);
  }

  return String(Number(numericValue.toFixed(2)));
};

const formatRuleFeeSummary = (rule) => {
  if (rule.feeType === 'fixed') {
    return `固定手续费 ${formatRuleNumber(rule.feeValue)}`;
  }

  const segments = [`按 ${formatRuleNumber(rule.feeValue)}% 收费`];
  if (rule.minFee > 0) {
    segments.push(`最低 ${formatRuleNumber(rule.minFee)}`);
  }
  if (rule.maxFee !== '' && Number(rule.maxFee) > 0) {
    segments.push(`最高 ${formatRuleNumber(rule.maxFee)}`);
  }
  return segments.join('，');
};

const interpolateTemplate = (template, params = {}) =>
  String(template).replace(/\{\{(\w+)\}\}/g, (_, key) =>
    Object.prototype.hasOwnProperty.call(params, key) ? params[key] : '',
  );

const resolveWithdrawalCopy = (t, key, params = {}) => {
  const translated = typeof t === 'function' ? t(key, params) : key;
  return interpolateTemplate(translated, params);
};

const formatUserFacingRuleAmount = (value, options = {}) => {
  const amount = formatRuleNumber(value);
  const currencySymbol = String(options?.currencySymbol || '');
  if (!currencySymbol) {
    return amount;
  }

  return `${currencySymbol}${amount}`;
};

const getRuleLabel = (rule) => {
  if (rule.maxAmount === '') {
    return `金额 > ${formatRuleNumber(rule.minAmount)}`;
  }

  if (Number(rule.minAmount) <= 0) {
    return `0 < 金额 <= ${formatRuleNumber(rule.maxAmount)}`;
  }

  return `${formatRuleNumber(rule.minAmount)} < 金额 <= ${formatRuleNumber(rule.maxAmount)}`;
};

const getUserFacingRuleRangeLabel = (rule, t, options = {}) => {
  if (rule.maxAmount === '') {
    return resolveWithdrawalCopy(t, '高于 {{amountWithSymbol}}', {
      amountWithSymbol: formatUserFacingRuleAmount(rule.minAmount, options),
    });
  }

  if (Number(rule.minAmount) <= 0) {
    return resolveWithdrawalCopy(t, '大于 0 且不超过 {{amountWithSymbol}}', {
      amountWithSymbol: formatUserFacingRuleAmount(rule.maxAmount, options),
    });
  }

  return resolveWithdrawalCopy(t, '高于 {{minWithSymbol}} 至 {{maxWithSymbol}}', {
    minWithSymbol: formatUserFacingRuleAmount(rule.minAmount, options),
    maxWithSymbol: formatUserFacingRuleAmount(rule.maxAmount, options),
  });
};

const getUserFacingRuleFeeSummary = (rule, t, options = {}) => {
  if (rule.feeType === 'fixed') {
    return resolveWithdrawalCopy(t, '固定手续费 {{amountWithSymbol}}', {
      amountWithSymbol: formatUserFacingRuleAmount(rule.feeValue, options),
    });
  }

  const segments = [
    resolveWithdrawalCopy(t, '按 {{rate}}% 收费', {
      rate: formatRuleNumber(rule.feeValue),
    }),
  ];

  if (rule.minFee > 0) {
    segments.push(
      resolveWithdrawalCopy(t, '最低手续费 {{amountWithSymbol}}', {
        amountWithSymbol: formatUserFacingRuleAmount(rule.minFee, options),
      }),
    );
  }

  if (rule.maxFee !== '' && Number(rule.maxFee) > 0) {
    segments.push(
      resolveWithdrawalCopy(t, '最高手续费 {{amountWithSymbol}}', {
        amountWithSymbol: formatUserFacingRuleAmount(rule.maxFee, options),
      }),
    );
  }

  return segments.join('，');
};

const matchesWithdrawalFeeRuleAmount = (amount, rule) => {
  const numericAmount = Number(amount || 0);
  if (!Number.isFinite(numericAmount) || numericAmount <= 0 || rule?.enabled === false) {
    return false;
  }

  const minAmount = toFiniteNumber(rule?.min_amount, 0);
  const maxAmount = toFiniteNumber(rule?.max_amount, 0);
  if (numericAmount <= minAmount) {
    return false;
  }
  if (maxAmount > 0 && numericAmount > maxAmount) {
    return false;
  }
  return true;
};

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

export const normalizeWithdrawalFeeEditorRules = (feeRules = []) =>
  sortEditorRules(parseWithdrawalRulesInput(feeRules).map(normalizeEditorRule));

export const validateWithdrawalFeeEditorRules = (feeRules = []) => {
  const rules = normalizeWithdrawalFeeEditorRules(feeRules);
  const errors = [];
  const warnings = [];

  rules.forEach((rule, index) => {
    const rowNumber = index + 1;
    const upperBound = getRuleUpperBound(rule);
    const maxFee = rule.maxFee === '' ? 0 : Number(rule.maxFee);

    if (rule.minAmount < 0) {
      errors.push(`第 ${rowNumber} 条规则的起始金额必须大于等于 0`);
    }
    if (Number.isFinite(upperBound) && upperBound <= rule.minAmount) {
      errors.push(`第 ${rowNumber} 条规则的结束金额必须大于起始金额`);
    }
    if (rule.feeValue < 0) {
      errors.push(`第 ${rowNumber} 条规则的手续费必须大于等于 0`);
    }
    if (rule.feeType === 'ratio' && (rule.feeValue < 0 || rule.feeValue > 100)) {
      errors.push(`第 ${rowNumber} 条规则的费率必须在 0 到 100 之间`);
    }
    if (rule.feeType === 'ratio' && rule.minFee < 0) {
      errors.push(`第 ${rowNumber} 条规则的最低手续费必须大于等于 0`);
    }
    if (rule.feeType === 'ratio' && maxFee < 0) {
      errors.push(`第 ${rowNumber} 条规则的最高手续费必须大于等于 0`);
    }
    if (rule.feeType === 'ratio' && rule.maxFee !== '' && maxFee < rule.minFee) {
      errors.push(`第 ${rowNumber} 条规则的最高手续费不能小于最低手续费`);
    }
    if (!rule.enabled) {
      warnings.push(`第 ${rowNumber} 条规则已停用，用户侧不会命中它`);
    }
  });

  const enabledRules = rules
    .map((rule, index) => ({
      rule,
      rowNumber: index + 1,
    }))
    .filter(({ rule }) => rule.enabled);

  enabledRules.forEach(({ rule, rowNumber }, index) => {
    const nextEnabledRule = enabledRules[index + 1];
    const nextRule = nextEnabledRule?.rule;
    if (!nextRule) {
      return;
    }

    const upperBound = getRuleUpperBound(rule);

    if (!Number.isFinite(upperBound)) {
      errors.push(`第 ${rowNumber} 条规则已设置为无上限，后面不能再添加规则`);
      return;
    }

    if (nextRule.minAmount < upperBound) {
      errors.push(
        `第 ${rowNumber} 条规则和第 ${nextEnabledRule.rowNumber} 条规则的金额区间发生重叠`,
      );
      return;
    }

    if (nextRule.minAmount > upperBound) {
      warnings.push(
        `第 ${rowNumber} 条规则和第 ${nextEnabledRule.rowNumber} 条规则之间存在金额区间断层`,
      );
    }
  });

  return {
    errors,
    warnings,
  };
};

export const serializeWithdrawalFeeEditorRules = (feeRules = []) =>
  JSON.stringify(normalizeStoredWithdrawalFeeRules(feeRules));

export const describeWithdrawalFeeRule = (rule) => {
  const normalizedRule = normalizeEditorRule(rule, 0);
  return `${getRuleLabel(normalizedRule)}：${formatRuleFeeSummary(normalizedRule)}`;
};

export const describeWithdrawalFeeRuleForUser = (rule, t, options = {}) => {
  const normalizedRule = normalizeEditorRule(rule, 0);
  return `${getUserFacingRuleRangeLabel(normalizedRule, t, options)}：${getUserFacingRuleFeeSummary(
    normalizedRule,
    t,
    options,
  )}`;
};

export const buildWithdrawalFeeRuleDescriptions = (feeRules = [], t, options = {}) =>
  normalizeWithdrawalFeeEditorRules(feeRules)
    .filter((rule) => rule.enabled)
    .map((rule) => describeWithdrawalFeeRuleForUser(rule, t, options));

export const calculateWithdrawalPreview = (amount, feeRules = []) => {
  const numericAmount = Number(amount || 0);
  if (!Number.isFinite(numericAmount) || numericAmount <= 0) {
    return {
      feeAmount: 0,
      netAmount: 0,
      matchedRule: null,
      isValid: false,
      blockReason: '提现金额必须大于 0',
    };
  }

  const matchedRule =
    normalizeStoredWithdrawalFeeRules(feeRules).find((rule) =>
      matchesWithdrawalFeeRuleAmount(numericAmount, rule),
    ) || null;

  if (!matchedRule) {
    return {
      feeAmount: null,
      netAmount: null,
      matchedRule: null,
      isValid: false,
      blockReason: '当前提现金额未命中任何手续费规则，请调整金额或联系管理员',
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
    isValid: true,
    blockReason: '',
  };
};

export const buildWithdrawalFeeSamples = (feeRules = []) => {
  const rules = normalizeWithdrawalFeeEditorRules(feeRules).filter(
    (rule) => rule.enabled,
  );
  const sampleAmounts = [];

  rules.forEach((rule) => {
    const upperBound = getRuleUpperBound(rule);
    if (Number.isFinite(upperBound)) {
      const midpoint = Number(((rule.minAmount + upperBound) / 2).toFixed(2));
      if (midpoint > 0) {
        sampleAmounts.push(midpoint);
      }
      if (upperBound > 0) {
        sampleAmounts.push(upperBound);
      }
      return;
    }

    const openEndedSample = Math.max(rule.minAmount * 2, rule.minAmount + 1);
    if (openEndedSample > 0) {
      sampleAmounts.push(Number(openEndedSample.toFixed(2)));
    }
  });

  return [...new Set(sampleAmounts)]
    .sort((left, right) => left - right)
    .map((amount) => {
      const preview = calculateWithdrawalPreview(amount, rules);
      return {
        amount,
        feeAmount: preview.feeAmount,
        netAmount: preview.netAmount,
        matchedRule: preview.matchedRule,
        ruleText: preview.matchedRule
          ? describeWithdrawalFeeRule(preview.matchedRule)
          : '未命中手续费规则',
      };
    });
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
