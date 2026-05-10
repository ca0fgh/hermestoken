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

import React, {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import {
  Button,
  Card,
  Cascader,
  Col,
  Dropdown,
  Input,
  InputNumber,
  Modal,
  Popover,
  Row,
  Select,
  Slider,
  Space,
  Table,
  Tabs,
  Tag,
  TextArea,
  Tooltip,
  Typography,
} from '@douyinfe/semi-ui';
import {
  IconArrowRight,
  IconCart,
  IconChevronDown,
  IconCopy,
  IconEyeClosed,
  IconEyeOpened,
  IconKey,
  IconMore,
  IconRoute,
  IconRefresh,
} from '@douyinfe/semi-icons';
import { useTranslation } from 'react-i18next';
import { useSearchParams } from 'react-router-dom';
import {
  CHANNEL_OPTIONS,
  MODEL_FETCHABLE_CHANNEL_TYPES,
} from '../../constants/channel.constants';
import {
  API,
  copy,
  createUnifiedPaginationProps,
  getUserIdFromLocalStorage,
  showError,
  showInfo,
  showSuccess,
} from '../../helpers';
import {
  encodeChannelConnectionString,
  fetchTokenKey,
  getServerAddress,
} from '../../helpers/token';
import {
  displayAmountToQuota,
  getCurrencyConfig,
  quotaToDisplayAmount,
  renderQuota,
} from '../../helpers/quota';
import { useIsMobile } from '../../hooks/common/useIsMobile';
import MarketplaceCredentialModelTestModal from '../../components/table/channels/modals/ModelTestModal';
import './index.css';

const { Text, Title } = Typography;
const PAGE_SIZE = 20;
const MARKETPLACE_STATUS_REFRESH_INTERVAL_MS = 10_000;
const FILTER_RANGE_FALLBACK_PAGE_SIZE = 1000;
const MARKETPLACE_FILTER_ALL_VALUE = '__all__';
const MARKETPLACE_VENDOR_VALUE_PREFIX = 'vendor:';
const MARKETPLACE_MODEL_VALUE_SEPARATOR = ':model:';

const defaultFilters = {
  p: 1,
  page_size: PAGE_SIZE,
  vendor_type: undefined,
  model: '',
  quota_mode: '',
  time_mode: '',
  min_quota_limit: '',
  max_quota_limit: '',
  min_time_limit_seconds: '',
  max_time_limit_seconds: '',
  min_multiplier: '',
  max_multiplier: '',
  min_concurrency_limit: '',
  max_concurrency_limit: '',
};

function useVisibleMarketplaceRefresh(refresh, delay) {
  const refreshRef = useRef(refresh);
  const refreshingRef = useRef(false);

  useEffect(() => {
    refreshRef.current = refresh;
  }, [refresh]);

  useEffect(() => {
    if (
      !delay ||
      typeof window === 'undefined' ||
      typeof document === 'undefined'
    ) {
      return undefined;
    }

    const refreshIfVisible = () => {
      if (document.visibilityState !== 'visible' || refreshingRef.current) {
        return;
      }

      refreshingRef.current = true;
      Promise.resolve()
        .then(() => refreshRef.current?.())
        .catch(() => {})
        .finally(() => {
          refreshingRef.current = false;
        });
    };

    const timer = window.setInterval(refreshIfVisible, delay);
    document.addEventListener('visibilitychange', refreshIfVisible);

    return () => {
      window.clearInterval(timer);
      document.removeEventListener('visibilitychange', refreshIfVisible);
    };
  }, [delay]);
}

function normalizeMarketplacePoolFilters(filters = {}) {
  return {
    ...defaultFilters,
    vendor_type: filters?.vendor_type || undefined,
    model: filters?.model || '',
    quota_mode: filters?.quota_mode || '',
    time_mode: filters?.time_mode || '',
    min_quota_limit: filters?.min_quota_limit || '',
    max_quota_limit: filters?.max_quota_limit || '',
    min_time_limit_seconds: filters?.min_time_limit_seconds || '',
    max_time_limit_seconds: filters?.max_time_limit_seconds || '',
    min_multiplier: filters?.min_multiplier || '',
    max_multiplier: filters?.max_multiplier || '',
    p: 1,
  };
}

function marketplacePoolFilterPayload(filters = {}) {
  const payload = compactParams(normalizeMarketplacePoolFilters(filters));
  delete payload.p;
  delete payload.page_size;
  return payload;
}

const MARKETPLACE_ROUTE_ORDER_VALUES = ['fixed_order', 'group', 'pool'];
const MARKETPLACE_TAB_VALUES = ['pool', 'orders', 'fixed', 'seller'];
const MARKETPLACE_TAB_ALIASES = {
  pool: 'pool',
  marketplace_pool: 'pool',
  order_pool: 'pool',
  orders: 'orders',
  order_list: 'orders',
  marketplace_orders: 'orders',
  fixed_order: 'orders',
  fixed_orders: 'orders',
  marketplace_fixed_order: 'orders',
  fixed: 'fixed',
  my_fixed_orders: 'fixed',
  seller: 'seller',
};
const MARKETPLACE_ROUTE_ALIASES = {
  fixed_order: 'fixed_order',
  marketplace_fixed_order: 'fixed_order',
  fixed: 'fixed_order',
  order: 'fixed_order',
  group: 'group',
  normal_group: 'group',
  ordinary_group: 'group',
  channel: 'group',
  pool: 'pool',
  marketplace_pool: 'pool',
  order_pool: 'pool',
};

function normalizeMarketplaceTab(value) {
  const tab = MARKETPLACE_TAB_ALIASES[String(value || '').trim()];
  return MARKETPLACE_TAB_VALUES.includes(tab) ? tab : 'pool';
}

const defaultCredentialForm = {
  vendor_type: 1,
  api_key: '',
  base_url: '',
  other: '',
  model_mapping: '',
  status_code_mapping: '',
  setting: '',
  proxy: '',
  param_override: '',
  settings: '',
  models: '',
  quota_mode: 'unlimited',
  quota_limit: '',
  time_mode: 'unlimited',
  time_limit_minutes: '',
  multiplier: '1',
  concurrency_limit: '1',
};

const DEFAULT_MAX_CREDENTIAL_CONCURRENCY = 5;

function readOptionValue(options, key, fallback) {
  if (Array.isArray(options)) {
    const option = options.find((item) => item?.key === key);
    return option?.value ?? fallback;
  }
  return options?.[key] ?? fallback;
}

function parseJSONObject(raw) {
  if (!raw || typeof raw !== 'string') return {};
  try {
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed)
      ? parsed
      : {};
  } catch {
    return {};
  }
}

function buildMarketplaceCredentialSetting(setting, proxy) {
  return JSON.stringify({
    ...parseJSONObject(setting),
    proxy: String(proxy || '').trim(),
  });
}

function marketplaceCredentialProxy(setting) {
  const parsed = parseJSONObject(setting);
  return typeof parsed.proxy === 'string' ? parsed.proxy : '';
}

function normalizeMaxCredentialConcurrency(value) {
  const parsed = Math.floor(Number(value));
  return Number.isFinite(parsed) && parsed >= 0
    ? parsed
    : DEFAULT_MAX_CREDENTIAL_CONCURRENCY;
}

function clampCredentialConcurrency(value, maxCredentialConcurrency) {
  const parsed = Math.floor(Number(value));
  if (!Number.isFinite(parsed) || parsed < 0) return 1;
  const normalizedMax = normalizeMaxCredentialConcurrency(
    maxCredentialConcurrency,
  );
  if (normalizedMax <= 0) return parsed;
  return Math.min(parsed, normalizedMax);
}

function formatConcurrencyLimit(value, t) {
  const numeric = Math.floor(Number(value) || 0);
  return numeric > 0 ? String(numeric) : t('无限制');
}

function formatCurrentConcurrency(record, t) {
  const current = Math.max(
    0,
    Math.floor(Number(record?.current_concurrency) || 0),
  );
  return `${current}/${formatConcurrencyLimit(record?.concurrency_limit, t)}`;
}

function compactParams(params) {
  return Object.fromEntries(
    Object.entries(params).filter(([, value]) => {
      if (value === undefined || value === null || value === '') return false;
      if (typeof value === 'number' && value <= 0) return false;
      return true;
    }),
  );
}

const MARKETPLACE_RELAY_FILTER_KEYS = [
  'vendor_type',
  'quota_mode',
  'time_mode',
  'min_quota_limit',
  'max_quota_limit',
  'min_time_limit_seconds',
  'max_time_limit_seconds',
  'min_multiplier',
  'max_multiplier',
  'min_concurrency_limit',
  'max_concurrency_limit',
];

function marketplaceRelayFilterParams(filters = {}) {
  return compactParams(
    Object.fromEntries(
      MARKETPLACE_RELAY_FILTER_KEYS.map((key) => [key, filters[key]]),
    ),
  );
}

function pageItems(response) {
  return response?.data?.data?.items ?? [];
}

function pageTotal(response) {
  return response?.data?.data?.total ?? 0;
}

function ensureSuccess(response) {
  if (!response?.data?.success) {
    throw new Error(response?.data?.message || '请求失败');
  }
  return response.data;
}

function marketplaceFilterRangeParams(filters) {
  return {
    vendor_type: filters.vendor_type,
    model: filters.model,
  };
}

function marketplaceModelOptionParams(filters) {
  return {
    quota_mode: filters.quota_mode,
    time_mode: filters.time_mode,
    min_quota_limit: filters.min_quota_limit,
    max_quota_limit: filters.max_quota_limit,
    min_time_limit_seconds: filters.min_time_limit_seconds,
    max_time_limit_seconds: filters.max_time_limit_seconds,
    min_multiplier: filters.min_multiplier,
    max_multiplier: filters.max_multiplier,
    min_concurrency_limit: filters.min_concurrency_limit,
    max_concurrency_limit: filters.max_concurrency_limit,
  };
}

function marketplaceFilterRangesFromOrders(orders) {
  return orders.reduce((ranges, order) => {
    const quotaModePatch =
      order?.quota_mode === 'unlimited'
        ? { unlimited_quota_count: ranges.unlimited_quota_count + 1 }
        : order?.quota_mode === 'limited'
          ? limitedQuotaRangePatch(ranges, Number(order?.quota_limit) || 0)
          : {};
    const timeModePatch =
      order?.time_mode === 'unlimited'
        ? { unlimited_time_count: ranges.unlimited_time_count + 1 }
        : order?.time_mode === 'limited'
          ? limitedTimeRangePatch(
              ranges,
              Number(order?.time_limit_seconds) || 0,
            )
          : {};
    const multiplierPatch = multiplierRangePatch(
      ranges,
      Number(order?.multiplier) || 0,
    );
    const concurrencyPatch = concurrencyRangePatch(
      ranges,
      Number(order?.concurrency_limit) || 0,
    );

    return {
      ...ranges,
      ...quotaModePatch,
      ...timeModePatch,
      ...multiplierPatch,
      ...concurrencyPatch,
    };
  }, emptyMarketplaceFilterRanges());
}

function emptyMarketplaceFilterRanges() {
  return {
    unlimited_quota_count: 0,
    limited_quota_count: 0,
    min_quota_limit: 0,
    max_quota_limit: 0,
    unlimited_time_count: 0,
    limited_time_count: 0,
    min_time_limit_seconds: 0,
    max_time_limit_seconds: 0,
    min_multiplier: 0,
    max_multiplier: 0,
    min_concurrency_limit: 0,
    max_concurrency_limit: 0,
  };
}

function limitedQuotaRangePatch(ranges, quotaLimit) {
  if (quotaLimit <= 0) return {};
  return {
    limited_quota_count: ranges.limited_quota_count + 1,
    min_quota_limit:
      ranges.min_quota_limit > 0
        ? Math.min(ranges.min_quota_limit, quotaLimit)
        : quotaLimit,
    max_quota_limit: Math.max(ranges.max_quota_limit, quotaLimit),
  };
}

function limitedTimeRangePatch(ranges, timeLimitSeconds) {
  if (timeLimitSeconds <= 0) return {};
  return {
    limited_time_count: ranges.limited_time_count + 1,
    min_time_limit_seconds:
      ranges.min_time_limit_seconds > 0
        ? Math.min(ranges.min_time_limit_seconds, timeLimitSeconds)
        : timeLimitSeconds,
    max_time_limit_seconds: Math.max(
      ranges.max_time_limit_seconds,
      timeLimitSeconds,
    ),
  };
}

function multiplierRangePatch(ranges, multiplier) {
  if (multiplier <= 0) return {};
  return {
    min_multiplier:
      ranges.min_multiplier > 0
        ? Math.min(ranges.min_multiplier, multiplier)
        : multiplier,
    max_multiplier: Math.max(ranges.max_multiplier, multiplier),
  };
}

function concurrencyRangePatch(ranges, concurrencyLimit) {
  if (concurrencyLimit <= 0) return {};
  return {
    min_concurrency_limit:
      ranges.min_concurrency_limit > 0
        ? Math.min(ranges.min_concurrency_limit, concurrencyLimit)
        : concurrencyLimit,
    max_concurrency_limit: Math.max(
      ranges.max_concurrency_limit,
      concurrencyLimit,
    ),
  };
}

function useMarketplaceFilterRanges(filters) {
  const [ranges, setRanges] = useState({});

  useEffect(() => {
    let ignore = false;
    const rangeParams = marketplaceFilterRangeParams(filters);

    async function loadRanges() {
      try {
        const response = await API.get('/api/marketplace/order-filter-ranges', {
          params: compactParams(rangeParams),
          skipErrorHandler: true,
        });
        ensureSuccess(response);
        if (!ignore) {
          setRanges(response.data.data || {});
        }
      } catch {
        await loadRangesFromOrders();
      }
    }

    async function loadRangesFromOrders() {
      try {
        const response = await API.get('/api/marketplace/orders', {
          params: compactParams({
            ...rangeParams,
            p: 1,
            page_size: FILTER_RANGE_FALLBACK_PAGE_SIZE,
          }),
          skipErrorHandler: true,
        });
        ensureSuccess(response);
        if (!ignore) {
          setRanges(marketplaceFilterRangesFromOrders(pageItems(response)));
        }
      } catch {
        if (!ignore) {
          setRanges({});
        }
      }
    }

    loadRanges();

    return () => {
      ignore = true;
    };
  }, [filters.vendor_type, filters.model]);

  return ranges;
}

function useMarketplaceModelOptions(filters) {
  const [items, setItems] = useState([]);

  useEffect(() => {
    let ignore = false;
    const params = marketplaceModelOptionParams(filters);

    async function loadModelOptions() {
      try {
        const response = await API.get('/api/marketplace/pool/models', {
          params: compactParams(params),
          skipErrorHandler: true,
        });
        ensureSuccess(response);
        if (!ignore) {
          setItems(Array.isArray(response.data.data) ? response.data.data : []);
        }
      } catch {
        if (!ignore) {
          setItems([]);
        }
      }
    }

    loadModelOptions();

    return () => {
      ignore = true;
    };
  }, [
    filters.vendor_type,
    filters.quota_mode,
    filters.time_mode,
    filters.min_quota_limit,
    filters.max_quota_limit,
    filters.min_time_limit_seconds,
    filters.max_time_limit_seconds,
    filters.min_multiplier,
    filters.max_multiplier,
    filters.min_concurrency_limit,
    filters.max_concurrency_limit,
  ]);

  return items;
}

function getVendorName(vendorType, fallback) {
  return (
    fallback ||
    CHANNEL_OPTIONS.find((option) => option.value === Number(vendorType))
      ?.label ||
    `#${vendorType}`
  );
}

function splitModels(models) {
  if (Array.isArray(models)) return models;
  return String(models || '')
    .split(',')
    .map((model) => model.trim())
    .filter(Boolean);
}

function marketplaceVendorFilterValue(vendorType) {
  return `${MARKETPLACE_VENDOR_VALUE_PREFIX}${vendorType}`;
}

function marketplaceModelFilterValue(vendorType, model) {
  return `${marketplaceVendorFilterValue(vendorType)}${MARKETPLACE_MODEL_VALUE_SEPARATOR}${model}`;
}

function parseMarketplaceVendorModelValue(value) {
  const selectedValue = Array.isArray(value) ? value[value.length - 1] : value;
  const rawValue = String(selectedValue || '');
  if (!rawValue || rawValue === MARKETPLACE_FILTER_ALL_VALUE) {
    return { vendor_type: undefined, model: '' };
  }
  if (!rawValue.startsWith(MARKETPLACE_VENDOR_VALUE_PREFIX)) {
    return { vendor_type: undefined, model: '' };
  }

  const routeValue = rawValue.slice(MARKETPLACE_VENDOR_VALUE_PREFIX.length);
  const modelSeparatorIndex = routeValue.indexOf(
    MARKETPLACE_MODEL_VALUE_SEPARATOR,
  );
  const vendorType =
    modelSeparatorIndex >= 0
      ? Number(routeValue.slice(0, modelSeparatorIndex))
      : Number(routeValue);
  if (!Number.isFinite(vendorType) || vendorType <= 0) {
    return { vendor_type: undefined, model: '' };
  }
  if (modelSeparatorIndex < 0) {
    return { vendor_type: vendorType, model: '' };
  }

  const model = routeValue.slice(
    modelSeparatorIndex + MARKETPLACE_MODEL_VALUE_SEPARATOR.length,
  );
  return {
    vendor_type: vendorType,
    model: model === MARKETPLACE_FILTER_ALL_VALUE ? '' : model,
  };
}

function marketplaceVendorModelFilterValue(filters) {
  if (!filters.vendor_type) return MARKETPLACE_FILTER_ALL_VALUE;
  const vendorValue = marketplaceVendorFilterValue(filters.vendor_type);
  if (!filters.model) return vendorValue;
  return [
    vendorValue,
    marketplaceModelFilterValue(filters.vendor_type, filters.model),
  ];
}

function renderMarketplaceVendorModelDisplay(selected, t) {
  if (!Array.isArray(selected) || selected.length === 0) {
    return t('全部');
  }
  return selected.join(' -> ');
}

function buildMarketplaceVendorModelTree(items, filters, t) {
  const modelsByVendor = new Map();
  (Array.isArray(items) ? items : []).forEach((item) => {
    const vendorType = Number(item?.vendor_type);
    const model = String(item?.model || '').trim();
    if (!Number.isFinite(vendorType) || vendorType <= 0 || !model) return;
    if (!modelsByVendor.has(vendorType)) {
      modelsByVendor.set(vendorType, {
        vendorName: getVendorName(vendorType, item?.vendor_name_snapshot),
        models: new Set(),
      });
    }
    modelsByVendor.get(vendorType).models.add(model);
  });
  if (filters.vendor_type && filters.model) {
    const vendorType = Number(filters.vendor_type);
    if (!modelsByVendor.has(vendorType)) {
      modelsByVendor.set(vendorType, {
        vendorName: getVendorName(vendorType),
        models: new Set(),
      });
    }
    modelsByVendor.get(vendorType).models.add(filters.model);
  }

  const vendorNodes = Array.from(modelsByVendor.entries())
    .sort(([, a], [, b]) => a.vendorName.localeCompare(b.vendorName))
    .map(([vendorType, vendor]) => ({
      label: t(vendor.vendorName),
      value: marketplaceVendorFilterValue(vendorType),
      children: [
        {
          label: t('全部模型'),
          value: marketplaceModelFilterValue(
            vendorType,
            MARKETPLACE_FILTER_ALL_VALUE,
          ),
        },
        ...Array.from(vendor.models)
          .sort((a, b) => a.localeCompare(b))
          .map((model) => ({
            label: model,
            value: marketplaceModelFilterValue(vendorType, model),
          })),
      ],
    }));

  return [
    {
      label: t('全部'),
      value: MARKETPLACE_FILTER_ALL_VALUE,
    },
    ...vendorNodes,
  ];
}

function mergeModels(...groups) {
  return Array.from(
    new Set(
      groups
        .flat()
        .map((model) => String(model || '').trim())
        .filter(Boolean),
    ),
  );
}

function marketplaceCredentialTestChannel(record) {
  if (!record) return null;
  return {
    ...record,
    name: `${getVendorName(record.vendor_type, record.vendor_name_snapshot)} #${record.id}`,
    models: splitModels(record.models).join(','),
  };
}

function MarketplaceField({ label, help, children }) {
  return (
    <Space vertical spacing={4} style={{ width: '100%' }}>
      <Text strong>{label}</Text>
      {children}
      {help ? (
        <Text type='tertiary' size='small'>
          {help}
        </Text>
      ) : null}
    </Space>
  );
}

const marketplaceStatusLabels = {
  listed: '已上架',
  unlisted: '未上架',
  enabled: '已启用',
  disabled: '已禁用',
  healthy: '健康',
  degraded: '降级',
  failed: '失败',
  untested: '未测试',
  available: '可用',
  busy: '忙碌',
  exhausted: '已耗尽',
  active: '生效',
  expired: '已过期',
  suspended: '已暂停',
  refunded: '已退款',
  pending: '待结算',
  withdrawn: '已提现',
  blocked: '已阻止',
  reversed: '已冲正',
  normal: '正常',
  watching: '观察中',
  risk_paused: '风险暂停',
  unscored: '未检测',
  pending_probe: '待检测',
  running_probe: '检测中',
  passed_probe: '通过',
  warning_probe: '需复核',
  failed_probe: '失败',
  route_available: '可路由',
  route_unlisted: '未上架',
  route_disabled: '已停用',
  route_failed: '不可路由',
  route_risk_paused: '风险暂停',
  route_exhausted: '额度耗尽',
  route_busy: '并发忙碌',
  route_reason_unlisted: '卖家尚未上架该托管 Key。',
  route_reason_disabled: '卖家已停用该托管 Key。',
  route_reason_health_failed: '最近测试失败，需先修复并重新测试通过。',
  route_reason_health_unavailable: '当前健康状态不可用，暂时不能路由。',
  route_reason_probe_unscored: '尚未完成探针检测，完成检测后才可路由。',
  route_reason_probe_in_progress: '探针检测正在进行中，完成后会重新判断。',
  route_reason_probe_failed: '探针检测失败，需重新检测通过后才可路由。',
  route_reason_probe_score_missing: '探针评分不完整，需重新检测生成有效评分。',
  route_reason_probe_score_zero: '探针评分为 0，不能参与路由。',
  route_reason_risk_paused: '该托管 Key 已被风险控制暂停。',
  route_reason_quota_exhausted: '额度条件已耗尽，不能继续路由。',
  route_reason_concurrency_busy: '当前并发已达到上限，请稍后重试。',
  route_reason_unavailable: '当前状态不满足路由条件。',
};

function statusTag(status, t) {
  const colorMap = {
    listed: 'green',
    unlisted: 'grey',
    enabled: 'green',
    disabled: 'red',
    healthy: 'green',
    degraded: 'orange',
    failed: 'red',
    untested: 'grey',
    available: 'green',
    busy: 'orange',
    exhausted: 'red',
    active: 'green',
    expired: 'grey',
    suspended: 'orange',
    refunded: 'red',
    pending: 'orange',
    withdrawn: 'green',
    blocked: 'red',
    normal: 'green',
    watching: 'orange',
    risk_paused: 'red',
    route_available: 'green',
    route_unlisted: 'grey',
    route_disabled: 'red',
    route_failed: 'red',
    route_risk_paused: 'red',
    route_exhausted: 'red',
    route_busy: 'orange',
  };

  return (
    <Tag color={colorMap[status] || 'grey'}>
      {t(marketplaceStatusLabels[status] || status || '-')}
    </Tag>
  );
}

const marketplaceProbeStatusLabels = {
  unscored: '未检测',
  pending: '待检测',
  running: '检测中',
  passed: '通过',
  warning: '需复核',
  failed: '失败',
};

function renderMarketplaceProbeScore(record, t) {
  const status = String(record?.probe_status || 'unscored');
  const score = Number(record?.probe_score) || 0;
  const scoreMax = Number(record?.probe_score_max) || score;
  const grade = String(record?.probe_grade || '').trim();
  const checkedAt = Number(record?.probe_checked_at) || 0;
  const hasScore = score > 0 || scoreMax > 0;
  const normalizedMax = scoreMax > 0 ? scoreMax : 100;
  const scoreText = hasScore ? `${score}/${normalizedMax}` : '--';
  const statusLabel = t(marketplaceProbeStatusLabels[status] || status);

  return (
    <div className='marketplace-probe-score'>
      <span className='marketplace-probe-score-main'>{scoreText}</span>
      <span className='marketplace-probe-score-meta'>
        {grade || statusLabel}
      </span>
      {checkedAt > 0 ? (
        <Text
          type='tertiary'
          size='small'
          className='marketplace-probe-score-time'
        >
          {formatFixedOrderExpiresAt(checkedAt)}
        </Text>
      ) : null}
    </div>
  );
}

function marketplaceProbeInProgress(record) {
  const status = String(record?.probe_status || '').trim();
  return status === 'pending' || status === 'running';
}

function marketplaceResponseTimeColor(responseTime, healthStatus) {
  if (healthStatus === 'failed') return 'red';
  if (healthStatus === 'degraded') return 'orange';
  if (responseTime <= 1000) return 'green';
  if (responseTime <= 3000) return 'lime';
  if (responseTime <= 5000) return 'yellow';
  return 'red';
}

function renderMarketplaceResponseTime(responseTime, t, healthStatus = '') {
  const numeric = Number(responseTime) || 0;
  if (numeric <= 0) {
    return (
      <Tag color='grey' shape='circle'>
        {t('未测试')}
      </Tag>
    );
  }
  const time = `${(numeric / 1000).toFixed(2)}${t(' 秒')}`;
  return (
    <Tag
      color={marketplaceResponseTimeColor(numeric, healthStatus)}
      shape='circle'
    >
      {time}
    </Tag>
  );
}

function renderMarketplaceSellerModels(models) {
  const modelList = splitModels(models);
  if (modelList.length === 0) {
    return <span className='marketplace-seller-muted'>-</span>;
  }
  const modelText = modelList.join(', ');

  return (
    <Tooltip content={modelText} position='topLeft'>
      <div className='marketplace-seller-model-cell'>{modelText}</div>
    </Tooltip>
  );
}

function renderMarketplaceSellerStatus(record, t) {
  return (
    <div className='marketplace-status-tags'>
      {statusTag(record.listing_status, t)}
      {statusTag(record.service_status, t)}
      {statusTag(record.health_status, t)}
      {renderMarketplaceRouteStatusTag(record, t)}
    </div>
  );
}

function renderMarketplaceRouteStatus(record, t) {
  return (
    <div className='marketplace-status-tags'>
      {statusTag(record?.health_status, t)}
      {renderMarketplaceRouteStatusTag(record, t)}
    </div>
  );
}

function marketplaceRouteReasonText(record, t) {
  const routeReason = String(record?.route_reason || '').trim();
  if (!routeReason) return '';
  return t(marketplaceStatusLabels[routeReason] || routeReason);
}

function renderMarketplaceRouteStatusTag(record, t) {
  const tag = statusTag(record?.route_status, t);
  const reasonText = marketplaceRouteReasonText(record, t);
  if (!reasonText) return tag;
  return (
    <Tooltip content={reasonText} position='top'>
      <span className='marketplace-route-status-with-reason'>{tag}</span>
    </Tooltip>
  );
}

function formatPricePoint(point) {
  if (!point?.configured) return '未配置';
  if (point.quota_type === 'tiered_expr') {
    return point.billing_expr ? '表达式/阶梯计费：已配置' : '表达式/阶梯计费';
  }
  if (point.quota_type === 'per_second') {
    const parts = ['按秒计费'];
    if (Number(point.task_per_request_price) > 0) {
      parts.push(`基础 ${formatUSD(point.task_per_request_price)}/次`);
    }
    if (Number(point.task_per_second_price) > 0) {
      parts.push(`${formatUSD(point.task_per_second_price)}/秒`);
    }
    return parts.join(' · ');
  }
  if (point.quota_type === 'price') {
    return `按次计费：${formatUSD(point.model_price)}/次`;
  }
  const parts = [
    '按量计费',
    `输入 ${formatUSD(point.input_price_per_mtok)}/1M tokens`,
  ];
  if (Number.isFinite(Number(point.output_price_per_mtok))) {
    parts.push(`输出 ${formatUSD(point.output_price_per_mtok)}/1M tokens`);
  }
  if (point.cache_read_price_per_mtok != null) {
    parts.push(
      `缓存读 ${formatUSD(point.cache_read_price_per_mtok)}/1M tokens`,
    );
  }
  if (point.cache_write_price_per_mtok != null) {
    parts.push(
      `缓存写 ${formatUSD(point.cache_write_price_per_mtok)}/1M tokens`,
    );
  }
  return parts.join(' · ');
}

function formatUSD(value) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric)) return '$0';
  const fixed = numeric >= 1 ? numeric.toFixed(4) : numeric.toFixed(6);
  return `$${fixed.replace(/\.?0+$/, '')}`;
}

function getMarketplaceQuotaPerUSD() {
  const quotaPerUnit = Number(localStorage.getItem('quota_per_unit'));
  return Number.isFinite(quotaPerUnit) && quotaPerUnit > 0
    ? quotaPerUnit
    : 500000;
}

function formatMarketplaceQuotaUSD(quota) {
  const numeric = Number(quota);
  if (!Number.isFinite(numeric) || numeric <= 0) return renderQuota(0, 2);
  const displayAmount = quotaToDisplayAmount(numeric);
  const digits = Math.abs(displayAmount) >= 1 ? 4 : 6;
  return renderQuota(numeric, digits);
}

function normalizeMarketplaceFeeRate(value) {
  const feeRate = Number(value);
  return Number.isFinite(feeRate) && feeRate > 0 ? feeRate : 0;
}

function marketplaceBuyerPaymentUSD(baseAmountUSD, feeRate) {
  const amount = Number(baseAmountUSD);
  if (!Number.isFinite(amount) || amount <= 0) return 0;
  return amount * (1 + normalizeMarketplaceFeeRate(feeRate));
}

function formatMarketplaceFeePercent(feeRate) {
  const normalized = normalizeMarketplaceFeeRate(feeRate);
  if (normalized <= 0) return '0%';
  return `${Number((normalized * 100).toFixed(6))}%`;
}

function marketplaceQuotaDisplayLabel() {
  const { symbol, type } = getCurrencyConfig();
  if (type === 'TOKENS') return 'Tokens';
  if (type === 'CNY') return 'CNY (¥)';
  if (type === 'USD') return 'USD ($)';
  return symbol || type || 'USD';
}

function marketplaceQuotaInputStep() {
  return getCurrencyConfig().type === 'TOKENS' ? '1' : '0.01';
}

function marketplaceQuotaSliderStep() {
  return Number(marketplaceQuotaInputStep());
}

function formatMarketplaceRange(minLabel, maxLabel) {
  return minLabel === maxLabel ? minLabel : `${minLabel} - ${maxLabel}`;
}

function formatMarketplaceQuotaRange(ranges) {
  return formatMarketplaceRange(
    formatMarketplaceQuotaUSD(ranges?.min_quota_limit),
    formatMarketplaceQuotaUSD(ranges?.max_quota_limit),
  );
}

function formatMarketplaceTimeValue(seconds, t) {
  const minutes = Math.ceil((Number(seconds) || 0) / 60);
  return `${minutes} ${t('分钟')}`;
}

function formatMarketplaceTimeRange(ranges, t) {
  return formatMarketplaceRange(
    formatMarketplaceTimeValue(ranges?.min_time_limit_seconds, t),
    formatMarketplaceTimeValue(ranges?.max_time_limit_seconds, t),
  );
}

function formatMarketplaceMultiplierValue(value) {
  const numeric = Number(value);
  if (!Number.isFinite(numeric) || numeric <= 0) return '0x';
  return `${Number(numeric.toFixed(2))}x`;
}

function formatMarketplaceMultiplierRange(ranges) {
  return formatMarketplaceRange(
    formatMarketplaceMultiplierValue(ranges?.min_multiplier),
    formatMarketplaceMultiplierValue(ranges?.max_multiplier),
  );
}

function formatMarketplaceConcurrencyValue(value) {
  const numeric = Math.round(Number(value) || 0);
  return String(Math.max(0, numeric));
}

function formatMarketplaceConcurrencyRange(ranges) {
  return formatMarketplaceRange(
    formatMarketplaceConcurrencyValue(ranges?.min_concurrency_limit),
    formatMarketplaceConcurrencyValue(ranges?.max_concurrency_limit),
  );
}

function clearMarketplaceQuotaRangeFilters() {
  return {
    min_quota_limit: '',
    max_quota_limit: '',
  };
}

function clearMarketplaceTimeRangeFilters() {
  return {
    min_time_limit_seconds: '',
    max_time_limit_seconds: '',
  };
}

function clearMarketplaceMultiplierRangeFilters() {
  return {
    min_multiplier: '',
    max_multiplier: '',
  };
}

function clearMarketplaceConcurrencyRangeFilters() {
  return {
    min_concurrency_limit: '',
    max_concurrency_limit: '',
  };
}

function marketplaceHasLimitedQuotaRange(ranges) {
  return (
    Number(ranges?.limited_quota_count) > 0 &&
    Number(ranges?.min_quota_limit) > 0 &&
    Number(ranges?.max_quota_limit) > 0
  );
}

function marketplaceHasLimitedTimeRange(ranges) {
  return (
    Number(ranges?.limited_time_count) > 0 &&
    Number(ranges?.min_time_limit_seconds) > 0 &&
    Number(ranges?.max_time_limit_seconds) > 0
  );
}

function marketplaceHasMultiplierRange(ranges) {
  return (
    Number(ranges?.min_multiplier) > 0 &&
    Number(ranges?.max_multiplier) > 0 &&
    Number(ranges?.max_multiplier) >= Number(ranges?.min_multiplier)
  );
}

function marketplaceHasConcurrencyRange(ranges) {
  return (
    Number(ranges?.min_concurrency_limit) > 0 &&
    Number(ranges?.max_concurrency_limit) > 0 &&
    Number(ranges?.max_concurrency_limit) >=
      Number(ranges?.min_concurrency_limit)
  );
}

function buildMarketplaceFilterOptions(type, ranges, t) {
  if (type === 'quota') {
    return [
      { label: t('全部'), value: MARKETPLACE_FILTER_ALL_VALUE },
      { label: t('不限额'), value: 'unlimited' },
      {
        label: t('限额'),
        value: 'limited',
      },
    ];
  }

  return [
    { label: t('全部'), value: MARKETPLACE_FILTER_ALL_VALUE },
    { label: t('不限时'), value: 'unlimited' },
    {
      label: t('限时'),
      value: 'limited',
    },
  ];
}

function normalizeMarketplaceSliderRange(value, minValue, maxValue) {
  const rawRange = Array.isArray(value) ? value : [minValue, maxValue];
  const first = Number(rawRange[0]);
  const second = Number(rawRange[1]);
  const low = Number.isFinite(first) ? first : minValue;
  const high = Number.isFinite(second) ? second : maxValue;
  return [
    Math.max(minValue, Math.min(low, high)),
    Math.min(maxValue, Math.max(low, high)),
  ];
}

function marketplaceFilterRangeClassName(className = '') {
  return [
    'marketplace-filter-item',
    'marketplace-filter-range-slider',
    className,
  ]
    .filter(Boolean)
    .join(' ');
}

function renderMarketplaceQuotaRangeInputs(
  filters,
  ranges,
  patch,
  t,
  className = '',
) {
  if (filters.quota_mode !== 'limited') return null;
  if (!marketplaceHasLimitedQuotaRange(ranges)) {
    return (
      <div className={marketplaceFilterRangeClassName(className)}>
        <div className='marketplace-filter-range-empty'>
          <Text type='tertiary' size='small'>
            {t('暂无可选额度')}
          </Text>
        </div>
      </div>
    );
  }
  const minDisplayValue = quotaToDisplayAmount(ranges.min_quota_limit);
  const maxDisplayValue = quotaToDisplayAmount(ranges.max_quota_limit);
  const sliderValue = normalizeMarketplaceSliderRange(
    [
      filters.min_quota_limit
        ? quotaToDisplayAmount(filters.min_quota_limit)
        : minDisplayValue,
      filters.max_quota_limit
        ? quotaToDisplayAmount(filters.max_quota_limit)
        : maxDisplayValue,
    ],
    minDisplayValue,
    maxDisplayValue,
  );
  const selectedMinQuota = displayAmountToQuota(sliderValue[0]);
  const selectedMaxQuota = displayAmountToQuota(sliderValue[1]);
  return (
    <div className={marketplaceFilterRangeClassName(className)}>
      <div className='marketplace-filter-range-heading'>
        <Text type='tertiary' size='small'>
          {t('额度范围')}
        </Text>
        <Text size='small' className='marketplace-filter-range-value'>
          {formatMarketplaceRange(
            formatMarketplaceQuotaUSD(selectedMinQuota),
            formatMarketplaceQuotaUSD(selectedMaxQuota),
          )}
        </Text>
      </div>
      <Slider
        range
        min={minDisplayValue}
        max={maxDisplayValue}
        step={marketplaceQuotaSliderStep()}
        value={sliderValue}
        tipFormatter={(value) =>
          Array.isArray(value)
            ? formatMarketplaceRange(
                formatMarketplaceQuotaUSD(displayAmountToQuota(value[0])),
                formatMarketplaceQuotaUSD(displayAmountToQuota(value[1])),
              )
            : formatMarketplaceQuotaUSD(displayAmountToQuota(value))
        }
        onChange={(value) => {
          const [minValue, maxValue] = normalizeMarketplaceSliderRange(
            value,
            minDisplayValue,
            maxDisplayValue,
          );
          patch({
            min_quota_limit: displayAmountToQuota(minValue),
            max_quota_limit: displayAmountToQuota(maxValue),
          });
        }}
      />
      <div className='marketplace-filter-range-boundary'>
        <span>{formatMarketplaceQuotaUSD(ranges.min_quota_limit)}</span>
        <span>{formatMarketplaceQuotaUSD(ranges.max_quota_limit)}</span>
      </div>
    </div>
  );
}

function renderMarketplaceTimeRangeInputs(
  filters,
  ranges,
  patch,
  t,
  className = '',
) {
  if (filters.time_mode !== 'limited') return null;
  if (!marketplaceHasLimitedTimeRange(ranges)) {
    return (
      <div className={marketplaceFilterRangeClassName(className)}>
        <div className='marketplace-filter-range-empty'>
          <Text type='tertiary' size='small'>
            {t('暂无可选时间')}
          </Text>
        </div>
      </div>
    );
  }
  const minMinute = Math.ceil(ranges.min_time_limit_seconds / 60);
  const maxMinute = Math.ceil(ranges.max_time_limit_seconds / 60);
  const sliderValue = normalizeMarketplaceSliderRange(
    [
      filters.min_time_limit_seconds
        ? Math.ceil(filters.min_time_limit_seconds / 60)
        : minMinute,
      filters.max_time_limit_seconds
        ? Math.ceil(filters.max_time_limit_seconds / 60)
        : maxMinute,
    ],
    minMinute,
    maxMinute,
  );
  const selectedMinSeconds = Math.round(sliderValue[0] * 60);
  const selectedMaxSeconds = Math.round(sliderValue[1] * 60);
  return (
    <div className={marketplaceFilterRangeClassName(className)}>
      <div className='marketplace-filter-range-heading'>
        <Text type='tertiary' size='small'>
          {t('时间范围')}
        </Text>
        <Text size='small' className='marketplace-filter-range-value'>
          {formatMarketplaceRange(
            formatMarketplaceTimeValue(selectedMinSeconds, t),
            formatMarketplaceTimeValue(selectedMaxSeconds, t),
          )}
        </Text>
      </div>
      <Slider
        range
        min={minMinute}
        max={maxMinute}
        step={1}
        value={sliderValue}
        tipFormatter={(value) =>
          Array.isArray(value)
            ? formatMarketplaceRange(
                formatMarketplaceTimeValue(Math.round(value[0] * 60), t),
                formatMarketplaceTimeValue(Math.round(value[1] * 60), t),
              )
            : formatMarketplaceTimeValue(Math.round(value * 60), t)
        }
        onChange={(value) => {
          const [minValue, maxValue] = normalizeMarketplaceSliderRange(
            value,
            minMinute,
            maxMinute,
          );
          patch({
            min_time_limit_seconds: Math.round(minValue * 60),
            max_time_limit_seconds: Math.round(maxValue * 60),
          });
        }}
      />
      <div className='marketplace-filter-range-boundary'>
        <span>
          {formatMarketplaceTimeValue(ranges.min_time_limit_seconds, t)}
        </span>
        <span>
          {formatMarketplaceTimeValue(ranges.max_time_limit_seconds, t)}
        </span>
      </div>
    </div>
  );
}

function renderMarketplaceMultiplierRangeInputs(
  filters,
  ranges,
  patch,
  t,
  className = '',
) {
  if (!marketplaceHasMultiplierRange(ranges)) {
    return (
      <div className={marketplaceFilterRangeClassName(className)}>
        <div className='marketplace-filter-range-empty'>
          <Text type='tertiary' size='small'>
            {t('暂无可选倍率')}
          </Text>
        </div>
      </div>
    );
  }
  const minValue = Number(ranges.min_multiplier);
  const maxValue = Number(ranges.max_multiplier);
  const sliderValue = normalizeMarketplaceSliderRange(
    [
      filters.min_multiplier ? Number(filters.min_multiplier) : minValue,
      filters.max_multiplier ? Number(filters.max_multiplier) : maxValue,
    ],
    minValue,
    maxValue,
  );
  const selectedMinMultiplier = Math.round(sliderValue[0] * 100) / 100;
  const selectedMaxMultiplier = Math.round(sliderValue[1] * 100) / 100;
  return (
    <div className={marketplaceFilterRangeClassName(className)}>
      <div className='marketplace-filter-range-heading'>
        <Text type='tertiary' size='small'>
          {t('倍率范围')}
        </Text>
        <Text size='small' className='marketplace-filter-range-value'>
          {formatMarketplaceRange(
            formatMarketplaceMultiplierValue(selectedMinMultiplier),
            formatMarketplaceMultiplierValue(selectedMaxMultiplier),
          )}
        </Text>
      </div>
      <Slider
        range
        disabled={maxValue <= minValue}
        min={minValue}
        max={maxValue > minValue ? maxValue : minValue + 0.01}
        step={0.01}
        value={maxValue > minValue ? sliderValue : [minValue, minValue]}
        tipFormatter={(value) =>
          Array.isArray(value)
            ? formatMarketplaceRange(
                formatMarketplaceMultiplierValue(value[0]),
                formatMarketplaceMultiplierValue(value[1]),
              )
            : formatMarketplaceMultiplierValue(value)
        }
        onChange={(value) => {
          const [minValue, maxValue] = normalizeMarketplaceSliderRange(
            value,
            Number(ranges.min_multiplier),
            Number(ranges.max_multiplier),
          );
          patch({
            min_multiplier: Math.round(minValue * 100) / 100,
            max_multiplier: Math.round(maxValue * 100) / 100,
          });
        }}
      />
      <div className='marketplace-filter-range-boundary'>
        <span>{formatMarketplaceMultiplierValue(ranges.min_multiplier)}</span>
        <span>{formatMarketplaceMultiplierValue(ranges.max_multiplier)}</span>
      </div>
    </div>
  );
}

function renderMarketplaceConcurrencyRangeInputs(
  filters,
  ranges,
  patch,
  t,
  className = '',
) {
  if (!marketplaceHasConcurrencyRange(ranges)) {
    return (
      <div className={marketplaceFilterRangeClassName(className)}>
        <div className='marketplace-filter-range-empty'>
          <Text type='tertiary' size='small'>
            {t('暂无可选并发')}
          </Text>
        </div>
      </div>
    );
  }
  const minValue = Math.round(Number(ranges.min_concurrency_limit));
  const maxValue = Math.round(Number(ranges.max_concurrency_limit));
  const sliderValue = normalizeMarketplaceSliderRange(
    [
      filters.min_concurrency_limit
        ? Number(filters.min_concurrency_limit)
        : minValue,
      filters.max_concurrency_limit
        ? Number(filters.max_concurrency_limit)
        : maxValue,
    ],
    minValue,
    maxValue,
  );
  const selectedMinConcurrency = Math.round(sliderValue[0]);
  const selectedMaxConcurrency = Math.round(sliderValue[1]);
  return (
    <div className={marketplaceFilterRangeClassName(className)}>
      <div className='marketplace-filter-range-heading'>
        <Text type='tertiary' size='small'>
          {t('并发范围')}
        </Text>
        <Text size='small' className='marketplace-filter-range-value'>
          {formatMarketplaceRange(
            formatMarketplaceConcurrencyValue(selectedMinConcurrency),
            formatMarketplaceConcurrencyValue(selectedMaxConcurrency),
          )}
        </Text>
      </div>
      <Slider
        range
        disabled={maxValue <= minValue}
        min={minValue}
        max={maxValue > minValue ? maxValue : minValue + 1}
        step={1}
        value={maxValue > minValue ? sliderValue : [minValue, minValue]}
        tipFormatter={(value) =>
          Array.isArray(value)
            ? formatMarketplaceRange(
                formatMarketplaceConcurrencyValue(value[0]),
                formatMarketplaceConcurrencyValue(value[1]),
              )
            : formatMarketplaceConcurrencyValue(value)
        }
        onChange={(value) => {
          const [minValue, maxValue] = normalizeMarketplaceSliderRange(
            value,
            Math.round(Number(ranges.min_concurrency_limit)),
            Math.round(Number(ranges.max_concurrency_limit)),
          );
          patch({
            min_concurrency_limit: Math.round(minValue),
            max_concurrency_limit: Math.round(maxValue),
          });
        }}
      />
      <div className='marketplace-filter-range-boundary'>
        <span>
          {formatMarketplaceConcurrencyValue(ranges.min_concurrency_limit)}
        </span>
        <span>
          {formatMarketplaceConcurrencyValue(ranges.max_concurrency_limit)}
        </span>
      </div>
    </div>
  );
}

function marketplaceRatioToUSDPerMTok(ratio) {
  return ((Number(ratio) || 0) * 1000000) / getMarketplaceQuotaPerUSD();
}

function modelNameFromPricingItem(item) {
  return item?.model_name || item?.model || item?.id || '';
}

function pricingPointFromPricingItem(item) {
  if (!item) {
    return { quota_type: 'ratio', model_ratio: 0, configured: false };
  }
  const quotaType =
    item.quota_type === 1 || item.quota_type === 'price'
      ? 'price'
      : item.quota_type === 0
        ? 'ratio'
        : String(item.quota_type || 'ratio');
  const configured =
    item.configured ??
    (quotaType === 'price'
      ? Number(item.model_price) > 0
      : quotaType === 'tiered_expr' || quotaType === 'per_second'
        ? true
        : Number(item.model_ratio) > 0);
  const modelRatio = Number(item.model_ratio) || 0;
  const completionRatio = Number(item.completion_ratio) || 0;
  const inputPricePerMTok = Number.isFinite(Number(item.input_price_per_mtok))
    ? Number(item.input_price_per_mtok)
    : marketplaceRatioToUSDPerMTok(modelRatio);
  const outputPricePerMTok = Number.isFinite(Number(item.output_price_per_mtok))
    ? Number(item.output_price_per_mtok)
    : inputPricePerMTok * completionRatio;
  const cacheReadPricePerMTok =
    item.cache_read_price_per_mtok != null
      ? item.cache_read_price_per_mtok
      : item.cache_ratio != null
        ? inputPricePerMTok * Number(item.cache_ratio)
        : item.cache_read_price_per_mtok;
  const cacheWritePricePerMTok =
    item.cache_write_price_per_mtok != null
      ? item.cache_write_price_per_mtok
      : item.create_cache_ratio != null
        ? inputPricePerMTok * Number(item.create_cache_ratio)
        : item.cache_write_price_per_mtok;
  return {
    quota_type: quotaType,
    billing_mode: item.billing_mode,
    billing_expr: item.billing_expr,
    model_price: Number(item.model_price) || 0,
    model_ratio: modelRatio,
    completion_ratio: completionRatio,
    cache_ratio: item.cache_ratio,
    create_cache_ratio: item.create_cache_ratio,
    input_price_per_mtok: inputPricePerMTok,
    output_price_per_mtok: outputPricePerMTok,
    cache_read_price_per_mtok: cacheReadPricePerMTok,
    cache_write_price_per_mtok: cacheWritePricePerMTok,
    task_per_request_price: Number(item.task_per_request_price) || 0,
    task_per_second_price: Number(item.task_per_second_price) || 0,
    configured,
  };
}

function multiplyPricePoint(point, multiplier) {
  if (!point?.configured) return point;
  return {
    ...point,
    applied_multiplier: multiplier,
    model_price: (Number(point.model_price) || 0) * multiplier,
    model_ratio: (Number(point.model_ratio) || 0) * multiplier,
    input_price_per_mtok:
      (Number(point.input_price_per_mtok) || 0) * multiplier,
    output_price_per_mtok:
      (Number(point.output_price_per_mtok) || 0) * multiplier,
    cache_read_price_per_mtok:
      point.cache_read_price_per_mtok == null
        ? point.cache_read_price_per_mtok
        : Number(point.cache_read_price_per_mtok) * multiplier,
    cache_write_price_per_mtok:
      point.cache_write_price_per_mtok == null
        ? point.cache_write_price_per_mtok
        : Number(point.cache_write_price_per_mtok) * multiplier,
    task_per_request_price:
      (Number(point.task_per_request_price) || 0) * multiplier,
    task_per_second_price:
      (Number(point.task_per_second_price) || 0) * multiplier,
  };
}

function buildSellerPricePreview(models, pricingByModel, multiplier) {
  return splitModels(models).map((model) => {
    const official = pricingByModel[model] || pricingPointFromPricingItem(null);
    return {
      model,
      official,
      buyer: multiplyPricePoint(official, multiplier),
    };
  });
}

function QuotaCompoundControl({ mode, limit, onModeChange, onLimitChange }) {
  const { t } = useTranslation();
  const isUnlimited = mode === 'unlimited';

  return (
    <div
      className={`marketplace-quota-compound ${
        isUnlimited ? 'is-unlimited' : 'is-limited'
      }`}
    >
      <Select
        value={mode}
        onChange={onModeChange}
        optionList={[
          { label: t('不限额'), value: 'unlimited' },
          { label: t('限额'), value: 'limited' },
        ]}
        style={{ width: '100%' }}
      />
      {mode === 'unlimited' ? null : (
        <Input
          type='number'
          min={0}
          step={marketplaceQuotaInputStep()}
          value={limit}
          placeholder={marketplaceQuotaDisplayLabel()}
          onChange={onLimitChange}
          style={{ width: '100%' }}
        />
      )}
    </div>
  );
}

function TimeCompoundControl({ mode, limit, onModeChange, onLimitChange }) {
  const { t } = useTranslation();
  const isUnlimited = mode === 'unlimited';

  return (
    <div
      className={`marketplace-quota-compound ${
        isUnlimited ? 'is-unlimited' : 'is-limited'
      }`}
    >
      <Select
        value={mode}
        onChange={onModeChange}
        optionList={[
          { label: t('不限时'), value: 'unlimited' },
          { label: t('限时'), value: 'limited' },
        ]}
        style={{ width: '100%' }}
      />
      {mode === 'unlimited' ? null : (
        <Input
          type='number'
          min={1}
          value={limit}
          placeholder={t('分钟')}
          onChange={onLimitChange}
          style={{ width: '100%' }}
        />
      )}
    </div>
  );
}

function formatPricePreview(item) {
  const preview = item?.price_preview?.[0];
  if (!preview) return null;

  return {
    buyer: formatPricePoint(preview.buyer),
    official: formatPricePoint(preview.official),
    multiplier: item.multiplier,
  };
}

function renderMarketplacePricePreview(item) {
  const price = formatPricePreview(item);
  if (!price) return <span className='marketplace-price-empty'>-</span>;
  return (
    <div className='marketplace-price-cell'>
      <span className='marketplace-price-line marketplace-price-line-primary'>
        买方 {price.buyer}
      </span>
      <span className='marketplace-price-line marketplace-price-line-secondary'>
        官方 {price.official}
        {price.multiplier ? ` x ${price.multiplier}` : ''}
      </span>
    </div>
  );
}

function formatQuota(item) {
  if (item.quota_mode === 'unlimited') return '不限额';
  return `${formatMarketplaceQuotaUSD(item.quota_used || 0)}/${formatMarketplaceQuotaUSD(
    item.quota_limit || 0,
  )}`;
}

function renderSellerQuotaUsage(item, t) {
  const quotaLimit =
    item.quota_mode === 'unlimited'
      ? t('不限额')
      : formatMarketplaceQuotaUSD(item.quota_limit || 0);
  const quotaUsed = formatMarketplaceQuotaUSD(item.quota_used || 0);

  return (
    <div className='marketplace-quota-usage'>
      <div className='marketplace-quota-usage-row'>
        <Text type='tertiary' size='small' className='marketplace-quota-label'>
          {t('额度')}
        </Text>
        <span className='marketplace-quota-value'>{quotaLimit}</span>
      </div>
      <div className='marketplace-quota-usage-row'>
        <Text type='tertiary' size='small' className='marketplace-quota-label'>
          {t('已消耗额度')}
        </Text>
        <span className='marketplace-quota-value'>{quotaUsed}</span>
      </div>
    </div>
  );
}

function formatTimeCondition(item) {
  if (item.time_mode !== 'limited') return '不限时';
  return `${Math.ceil((item.time_limit_seconds || 0) / 60)} 分钟`;
}

function formatFixedOrderExpiresAt(expiresAt) {
  const timestamp = Number(expiresAt) || 0;
  if (timestamp <= 0) return '-';
  const date = new Date(timestamp * 1000);
  const pad = (value) => String(value).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(
    date.getDate(),
  )} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(
    date.getSeconds(),
  )}`;
}

function formatFixedOrderRemainingTime(seconds, t) {
  const value = Math.max(0, Number(seconds) || 0);
  if (value < 60) return `${Math.ceil(value)} ${t('秒')}`;
  if (value < 3600) return `${Math.ceil(value / 60)} ${t('分钟')}`;
  if (value < 86400) return `${Math.ceil(value / 3600)} ${t('小时')}`;
  return `${Math.ceil(value / 86400)} ${t('天')}`;
}

function renderFixedOrderTimeStatus(record, t) {
  const expiresAt = Number(record?.expires_at) || 0;
  if (expiresAt <= 0) {
    return <Tag color='green'>{t('不限时')}</Tag>;
  }

  const expireTime = formatFixedOrderExpiresAt(expiresAt);
  const remainingSeconds = expiresAt - Math.floor(Date.now() / 1000);
  if (record?.status === 'expired' || remainingSeconds <= 0) {
    return (
      <Space vertical spacing={2}>
        <Tag color='grey'>{t('已过期')}</Tag>
        <Text type='tertiary' size='small'>
          {expireTime}
        </Text>
      </Space>
    );
  }

  return (
    <Space vertical spacing={2}>
      <Tag color='orange'>
        {t('剩余')} {formatFixedOrderRemainingTime(remainingSeconds, t)}
      </Tag>
      <Text type='tertiary' size='small'>
        {expireTime}
      </Text>
    </Space>
  );
}

function isFixedOrderTimeExpired(record) {
  const expiresAt = Number(record?.expires_at) || 0;
  return (
    expiresAt > 0 &&
    (record?.status === 'expired' || expiresAt <= Math.floor(Date.now() / 1000))
  );
}

function renderFixedOrderCombinedStatus(record, t) {
  const timeStatus = renderFixedOrderTimeStatus(record, t);
  if (isFixedOrderTimeExpired(record)) {
    return timeStatus;
  }
  return (
    <Space vertical spacing={4}>
      {statusTag(record?.status, t)}
      {timeStatus}
    </Space>
  );
}

function renderFixedOrderIdentity(record, t) {
  return (
    <div className='marketplace-fixed-order-identity'>
      <span className='marketplace-fixed-order-id'>#{record.id}</span>
      <span className='marketplace-fixed-order-meta'>
        {t('托管Key')} #{record.credential_id}
      </span>
    </div>
  );
}

function renderFixedOrderAmounts(record, t) {
  const amountRows = [
    {
      label: t('买断'),
      value: formatMarketplaceQuotaUSD(record.purchased_quota),
      tone: 'primary',
    },
    {
      label: t('消耗'),
      value: formatMarketplaceQuotaUSD(record.spent_quota),
    },
    {
      label: t('剩余'),
      value: formatMarketplaceQuotaUSD(record.remaining_quota),
      tone: Number(record.remaining_quota) > 0 ? 'success' : '',
    },
  ];

  return (
    <div className='marketplace-fixed-order-amounts'>
      {amountRows.map((row) => (
        <div
          className={`marketplace-fixed-order-amount-row${
            row.tone ? ` is-${row.tone}` : ''
          }`}
          key={row.label}
        >
          <span>{row.label}</span>
          <strong>{row.value}</strong>
        </div>
      ))}
    </div>
  );
}

function buildCurl(
  orderId,
  filters = {},
  token = '$TOKEN',
  model = 'gpt-4o-mini',
  options = {},
) {
  const query = new URLSearchParams(
    marketplaceRelayFilterParams(filters),
  ).toString();
  const endpoint = `/v1/chat/completions${!orderId && query ? `?${query}` : ''}`;
  const headers =
    orderId && !options.fixedOrderBound
      ? ` \\\n  -H "X-Marketplace-Fixed-Order-Id: ${orderId}"`
      : '';
  return `curl "${endpoint}" \\\n  -H "Authorization: Bearer ${token}" \\\n  -H "Content-Type: application/json"${headers} \\\n  -d '{"model":"${model || 'gpt-4o-mini'}","messages":[{"role":"user","content":"hello"}]}'`;
}

function fullTokenKey(key) {
  if (!key) return '';
  return String(key).startsWith('sk-') ? String(key) : `sk-${key}`;
}

function tokenKeyLabel(token, key) {
  const name = String(token?.name || '').trim();
  return name ? `${name} · ${key}` : key;
}

function BoundTokenKeyChip({
  token,
  revealed,
  resolvedKey,
  loading,
  onToggleVisibility,
  onCopyKey,
  onCopyConnectionInfo,
  t,
}) {
  const displayKey = fullTokenKey(
    revealed && resolvedKey ? resolvedKey : token.key,
  );
  const displayLabel = tokenKeyLabel(token, displayKey);

  return (
    <span
      className={`marketplace-bound-token-chip${
        revealed ? ' is-revealed' : ''
      }`}
    >
      <span className='marketplace-bound-token-key'>{displayLabel}</span>
      <Tooltip content={revealed ? t('隐藏密钥') : t('查看密钥')}>
        <Button
          theme='borderless'
          type='tertiary'
          size='small'
          className='marketplace-bound-token-action'
          icon={revealed ? <IconEyeClosed /> : <IconEyeOpened />}
          loading={loading}
          aria-label={
            revealed ? 'hide bound token key' : 'show bound token key'
          }
          onClick={(event) => {
            event.stopPropagation();
            onToggleVisibility(token);
          }}
        />
      </Tooltip>
      <Dropdown
        trigger='click'
        position='bottomRight'
        clickToHide
        menu={[
          {
            node: 'item',
            name: t('复制密钥'),
            onClick: () => onCopyKey(token),
          },
          {
            node: 'item',
            name: t('复制连接信息'),
            onClick: () => onCopyConnectionInfo(token),
          },
        ]}
      >
        <Button
          theme='borderless'
          type='tertiary'
          size='small'
          className='marketplace-bound-token-action'
          icon={<IconCopy />}
          loading={loading}
          aria-label='copy bound token'
          onClick={(event) => {
            event.stopPropagation();
          }}
        />
      </Dropdown>
    </span>
  );
}

function BoundTokenDropdown({
  tokens,
  visibleTokenKeys,
  resolvedTokenKeys,
  loadingTokenKeys,
  onToggleVisibility,
  onCopyKey,
  onCopyConnectionInfo,
  t,
}) {
  const content = (
    <div className='marketplace-bound-token-dropdown'>
      <div className='marketplace-bound-token-dropdown-list'>
        {tokens.map((token) => (
          <BoundTokenKeyChip
            key={token.id}
            token={token}
            revealed={!!visibleTokenKeys[token.id]}
            resolvedKey={resolvedTokenKeys[token.id]}
            loading={!!loadingTokenKeys[token.id]}
            onToggleVisibility={onToggleVisibility}
            onCopyKey={onCopyKey}
            onCopyConnectionInfo={onCopyConnectionInfo}
            t={t}
          />
        ))}
      </div>
    </div>
  );

  return (
    <Popover content={content} trigger='click' position='bottomRight' showArrow>
      <Button
        size='small'
        theme='light'
        type='tertiary'
        className='marketplace-bound-token-trigger'
      >
        {tokens.length} {t('令牌')} <IconChevronDown size={12} />
      </Button>
    </Popover>
  );
}

function marketplaceTokenFixedOrderIds(token) {
  const rawIds = Array.isArray(token?.marketplace_fixed_order_ids)
    ? token.marketplace_fixed_order_ids
    : typeof token?.marketplace_fixed_order_ids === 'string'
      ? token.marketplace_fixed_order_ids.split(',')
      : [];
  const legacyId = Number(token?.marketplace_fixed_order_id) || 0;
  return Array.from(
    new Set(
      [legacyId, ...rawIds]
        .map((id) => Number(id))
        .filter((id) => Number.isFinite(id) && id > 0),
    ),
  );
}

function normalizeMarketplaceRouteEnabledForToken(value) {
  if (value == null) return [...MARKETPLACE_ROUTE_ORDER_VALUES];
  const rawRoutes = Array.isArray(value)
    ? value
    : typeof value === 'string'
      ? value.split(',')
      : [];
  const seen = new Set();

  return rawRoutes.reduce((routes, rawRoute) => {
    const route = MARKETPLACE_ROUTE_ALIASES[String(rawRoute).trim()];
    if (!route || seen.has(route)) return routes;
    seen.add(route);
    return [...routes, route];
  }, []);
}

function normalizeMarketplaceRouteOrderForToken(value) {
  const rawRoutes = Array.isArray(value)
    ? value
    : typeof value === 'string'
      ? value.split(',')
      : [];
  const seen = new Set();
  const normalized = rawRoutes.reduce((routes, rawRoute) => {
    const route = MARKETPLACE_ROUTE_ALIASES[String(rawRoute).trim()];
    if (!route || seen.has(route)) return routes;
    seen.add(route);
    return [...routes, route];
  }, []);

  return MARKETPLACE_ROUTE_ORDER_VALUES.reduce(
    (routes, route) => (seen.has(route) ? routes : [...routes, route]),
    normalized,
  );
}

function marketplacePoolRouteEnabled(token) {
  return normalizeMarketplaceRouteEnabledForToken(
    token?.marketplace_route_enabled,
  ).includes('pool');
}

function marketplacePoolSavedFilters(token) {
  if (!token?.marketplace_pool_filters_enabled) return null;
  return normalizeMarketplacePoolFilters(token.marketplace_pool_filters);
}

function tokenWithMarketplacePoolRoute(token) {
  const enabledRoutes = new Set(
    normalizeMarketplaceRouteEnabledForToken(token.marketplace_route_enabled),
  );
  enabledRoutes.add('pool');
  return {
    ...token,
    marketplace_route_order: normalizeMarketplaceRouteOrderForToken(
      token.marketplace_route_order,
    ),
    marketplace_route_enabled: MARKETPLACE_ROUTE_ORDER_VALUES.filter((route) =>
      enabledRoutes.has(route),
    ),
  };
}

function tokenWithoutMarketplacePoolRoute(token) {
  const enabledRoutes = new Set(
    normalizeMarketplaceRouteEnabledForToken(token.marketplace_route_enabled),
  );
  enabledRoutes.delete('pool');
  return {
    ...token,
    marketplace_route_order: normalizeMarketplaceRouteOrderForToken(
      token.marketplace_route_order,
    ),
    marketplace_route_enabled: MARKETPLACE_ROUTE_ORDER_VALUES.filter((route) =>
      enabledRoutes.has(route),
    ),
  };
}

function marketplacePoolTokenUpdatePayload(token) {
  return {
    id: token.id,
    name: token.name,
    remain_quota: token.unlimited_quota ? 0 : token.remain_quota,
    expired_time: token.expired_time,
    unlimited_quota: !!token.unlimited_quota,
    model_limits_enabled: !!token.model_limits_enabled,
    model_limits: token.model_limits || '',
    allow_ips: token.allow_ips || '',
    group: token.group || '',
    cross_group_retry:
      token.group === 'auto' ? !!token.cross_group_retry : false,
    marketplace_fixed_order_id: token.marketplace_fixed_order_id || 0,
    marketplace_fixed_order_ids: marketplaceTokenFixedOrderIds(token),
    marketplace_route_order: normalizeMarketplaceRouteOrderForToken(
      token.marketplace_route_order,
    ),
    marketplace_route_enabled: normalizeMarketplaceRouteEnabledForToken(
      token.marketplace_route_enabled,
    ),
  };
}

async function copyMarketplaceCurl({ orderId, filters, token, model }) {
  if (!token?.id) {
    showError('请先选择控制台令牌');
    return;
  }
  try {
    const key = await fetchTokenKey(token.id);
    const fixedOrderBound =
      !!orderId && marketplaceTokenFixedOrderIds(token).includes(orderId);
    const ok = await copy(
      buildCurl(orderId, filters, fullTokenKey(key), model, {
        fixedOrderBound,
      }),
    );
    if (ok) {
      showSuccess('调用配置已复制');
    } else {
      showError('复制失败');
    }
  } catch (error) {
    showError(error.message || '获取令牌密钥失败');
  }
}

async function bindMarketplaceFixedOrderTokens({ fixedOrderId, tokenIds }) {
  const response = await API.post(
    `/api/marketplace/fixed-orders/${fixedOrderId}/bind-tokens`,
    {
      token_ids: tokenIds,
    },
  );
  ensureSuccess(response);
  return response?.data?.data;
}

function applyMarketplaceFixedOrderTokenBindings(
  tokens,
  fixedOrderId,
  tokenIds,
) {
  const selectedTokenIds = new Set(tokenIds);
  return tokens.map((token) => {
    const remainingOrderIds = marketplaceTokenFixedOrderIds(token).filter(
      (orderId) => orderId !== fixedOrderId,
    );
    const nextOrderIds = selectedTokenIds.has(token.id)
      ? [fixedOrderId, ...remainingOrderIds]
      : remainingOrderIds;

    return {
      ...token,
      marketplace_fixed_order_id: nextOrderIds[0] || 0,
      marketplace_fixed_order_ids: nextOrderIds,
    };
  });
}

function CallSnippet({
  orderId,
  filters,
  token,
  model,
  boundOrderIds = [],
  onEditBindings,
}) {
  const { t } = useTranslation();
  const previewToken = token ? fullTokenKey(token.key) : '$TOKEN';
  const fixedOrderBound = !!orderId && boundOrderIds.includes(orderId);

  return (
    <div className='marketplace-call-snippet'>
      <div className='marketplace-call-snippet-main'>
        <Text type='secondary' className='marketplace-call-snippet-copy'>
          {orderId
            ? token
              ? fixedOrderBound
                ? t('使用已绑定令牌：{{name}}', { name: token.name })
                : t('使用控制台令牌：{{name}}', { name: token.name })
              : t('使用绑定到该买断订单的控制台令牌')
            : token
              ? t('使用控制台令牌：{{name}}', { name: token.name })
              : t('请先选择控制台令牌')}
        </Text>
        {orderId ? (
          <Button
            size='small'
            theme='light'
            type='primary'
            className='marketplace-call-snippet-action'
            onClick={onEditBindings}
          >
            {t('编辑绑定令牌')}
          </Button>
        ) : null}
        {!orderId ? (
          <Button
            size='small'
            theme='light'
            type='primary'
            className='marketplace-call-snippet-action'
            onClick={() =>
              copyMarketplaceCurl({ orderId, filters, token, model })
            }
            disabled={!token}
          >
            {t('复制调用配置')}
          </Button>
        ) : null}
      </div>
      {!orderId ? (
        <TextArea
          autosize
          readonly
          value={buildCurl(orderId, filters, previewToken, model, {
            fixedOrderBound,
          })}
        />
      ) : null}
    </div>
  );
}

function ManageTokenButton() {
  const { t } = useTranslation();

  return (
    <Button
      size='small'
      theme='light'
      type='primary'
      className='marketplace-manage-token-button'
      onClick={() => window.location.assign('/console/token')}
    >
      {t('管理令牌')}
    </Button>
  );
}

function BuyerTokenPanel({ tokens, selectedTokenId, onChange }) {
  const { t } = useTranslation();
  const [showSelectedTokenKey, setShowSelectedTokenKey] = useState(false);
  const [resolvedTokenKeys, setResolvedTokenKeys] = useState({});
  const [loadingTokenKey, setLoadingTokenKey] = useState(false);
  const enabledTokens = tokens.filter((token) => token.status === 1);
  const selectedToken = enabledTokens.find(
    (token) => token.id === selectedTokenId,
  );
  const selectedTokenDisplayKey = fullTokenKey(
    showSelectedTokenKey && resolvedTokenKeys[selectedToken?.id]
      ? resolvedTokenKeys[selectedToken?.id]
      : selectedToken?.key,
  );

  useEffect(() => {
    setShowSelectedTokenKey(false);
  }, [selectedTokenId]);

  const resolveSelectedTokenKey = useCallback(async () => {
    if (!selectedToken?.id) {
      showError(t('请先选择控制台令牌'));
      return '';
    }
    if (resolvedTokenKeys[selectedToken.id]) {
      return resolvedTokenKeys[selectedToken.id];
    }

    setLoadingTokenKey(true);
    try {
      const key = await fetchTokenKey(selectedToken.id);
      setResolvedTokenKeys((current) => ({
        ...current,
        [selectedToken.id]: key,
      }));
      return key;
    } catch (error) {
      showError(error.message || t('获取令牌密钥失败'));
      return '';
    } finally {
      setLoadingTokenKey(false);
    }
  }, [resolvedTokenKeys, selectedToken?.id, t]);

  const toggleSelectedTokenKey = useCallback(async () => {
    if (showSelectedTokenKey) {
      setShowSelectedTokenKey(false);
      return;
    }

    const key = await resolveSelectedTokenKey();
    if (key) {
      setShowSelectedTokenKey(true);
    }
  }, [resolveSelectedTokenKey, showSelectedTokenKey]);

  const copySelectedTokenKey = useCallback(async () => {
    const key = await resolveSelectedTokenKey();
    if (!key) return;

    const ok = await copy(fullTokenKey(key));
    if (ok) {
      showSuccess(t('已复制'));
    } else {
      showError(t('复制失败'));
    }
  }, [resolveSelectedTokenKey, t]);

  const copySelectedTokenConnectionInfo = useCallback(async () => {
    const key = await resolveSelectedTokenKey();
    if (!key) return;

    const ok = await copy(
      encodeChannelConnectionString(fullTokenKey(key), getServerAddress()),
    );
    if (ok) {
      showSuccess(t('已复制'));
    } else {
      showError(t('复制失败'));
    }
  }, [resolveSelectedTokenKey, t]);

  return (
    <Card bodyStyle={{ padding: 16 }}>
      <Row gutter={[12, 12]} align='middle'>
        <Col xs={24} md={8}>
          <Space vertical spacing={2}>
            <Text strong>{t('买家令牌')}</Text>
            <Text type='secondary' size='small'>
              {t(
                '市场调用复用控制台令牌，用于买家身份、额度、模型限制和 IP 限制。',
              )}
            </Text>
          </Space>
        </Col>
        <Col xs={24} md={16}>
          <div className='marketplace-buyer-token-picker'>
            <Select
              value={selectedTokenId}
              placeholder={
                enabledTokens.length === 0
                  ? t('暂无启用令牌')
                  : t('选择控制台令牌')
              }
              disabled={enabledTokens.length === 0}
              onChange={onChange}
              optionList={enabledTokens.map((token) => ({
                label: tokenKeyLabel(
                  token,
                  token.id === selectedToken?.id && selectedTokenDisplayKey
                    ? selectedTokenDisplayKey
                    : fullTokenKey(token.key),
                ),
                value: token.id,
              }))}
              style={{ width: '100%' }}
            />
            <Tooltip
              content={showSelectedTokenKey ? t('隐藏密钥') : t('查看密钥')}
            >
              <Button
                theme='borderless'
                type='tertiary'
                size='small'
                className='marketplace-buyer-token-action'
                icon={
                  showSelectedTokenKey ? <IconEyeClosed /> : <IconEyeOpened />
                }
                loading={loadingTokenKey}
                disabled={!selectedToken}
                aria-label={
                  showSelectedTokenKey
                    ? 'hide selected token key'
                    : 'show selected token key'
                }
                onClick={toggleSelectedTokenKey}
              />
            </Tooltip>
            <Dropdown
              trigger='click'
              position='bottomRight'
              clickToHide
              menu={[
                {
                  node: 'item',
                  name: t('复制密钥'),
                  onClick: copySelectedTokenKey,
                },
                {
                  node: 'item',
                  name: t('复制连接信息'),
                  onClick: copySelectedTokenConnectionInfo,
                },
              ]}
            >
              <Button
                theme='borderless'
                type='tertiary'
                size='small'
                className='marketplace-buyer-token-action'
                icon={<IconCopy />}
                loading={loadingTokenKey}
                disabled={!selectedToken}
                aria-label='copy selected token'
              />
            </Dropdown>
          </div>
        </Col>
      </Row>
    </Card>
  );
}

function FilterBar({
  filters,
  filterRanges,
  onChange,
  onReset,
  showResetButton = true,
  showQuotaTimeFilters = true,
  showConcurrencyFilter = true,
}) {
  const { t } = useTranslation();
  const patch = (next) => onChange({ ...filters, ...next, p: 1 });
  const quotaOptions = buildMarketplaceFilterOptions('quota', filterRanges, t);
  const timeOptions = buildMarketplaceFilterOptions('time', filterRanges, t);
  const marketplaceModels = useMarketplaceModelOptions(filters);
  const vendorModelTree = useMemo(
    () => buildMarketplaceVendorModelTree(marketplaceModels, filters, t),
    [marketplaceModels, filters, t],
  );
  const compactFilterClassName = showQuotaTimeFilters
    ? ''
    : ' marketplace-filter-card-compact';

  return (
    <Card
      className={`marketplace-filter-card${compactFilterClassName}`}
      bodyStyle={{ padding: 12 }}
    >
      <div className='marketplace-filter-grid'>
        <div className='marketplace-filter-main-row'>
          <div className='marketplace-filter-main-controls'>
            <div className='marketplace-filter-item marketplace-filter-cascader'>
              <Cascader
                value={marketplaceVendorModelFilterValue(filters)}
                onChange={(value) =>
                  patch(parseMarketplaceVendorModelValue(value))
                }
                treeData={vendorModelTree}
                insetLabel={t('厂商 / 模型')}
                placeholder={t('全部厂商 / 全部模型')}
                searchPlaceholder={t('搜索厂商或模型')}
                filterTreeNode
                changeOnSelect
                showNext='click'
                showClear
                onClear={() => patch({ vendor_type: undefined, model: '' })}
                aria-label={t('厂商模型级联筛选')}
                separator=' -> '
                displayRender={(selected) =>
                  renderMarketplaceVendorModelDisplay(selected, t)
                }
                emptyContent={t('暂无模型')}
                dropdownStyle={{
                  minWidth: 360,
                  maxWidth: 'calc(100vw - 48px)',
                }}
                style={{ width: '100%' }}
              />
            </div>
            {showQuotaTimeFilters ? (
              <>
                <div className='marketplace-filter-item marketplace-filter-mode'>
                  <Select
                    value={filters.quota_mode || MARKETPLACE_FILTER_ALL_VALUE}
                    insetLabel={t('额度')}
                    onChange={(value) => {
                      const quotaMode =
                        value === MARKETPLACE_FILTER_ALL_VALUE ? '' : value;
                      patch({
                        quota_mode: quotaMode,
                        ...(quotaMode === 'limited'
                          ? {}
                          : clearMarketplaceQuotaRangeFilters()),
                      });
                    }}
                    optionList={quotaOptions}
                    style={{ width: '100%' }}
                  />
                </div>
                <div className='marketplace-filter-item marketplace-filter-mode'>
                  <Select
                    value={filters.time_mode || MARKETPLACE_FILTER_ALL_VALUE}
                    insetLabel={t('时间')}
                    onChange={(value) => {
                      const timeMode =
                        value === MARKETPLACE_FILTER_ALL_VALUE ? '' : value;
                      patch({
                        time_mode: timeMode,
                        ...(timeMode === 'limited'
                          ? {}
                          : clearMarketplaceTimeRangeFilters()),
                      });
                    }}
                    optionList={timeOptions}
                    style={{ width: '100%' }}
                  />
                </div>
              </>
            ) : null}
          </div>
          {showResetButton && onReset ? (
            <div className='marketplace-filter-actions'>
              <Space wrap>
                <Button
                  theme='light'
                  type='tertiary'
                  icon={<IconRefresh />}
                  className='marketplace-filter-action-button marketplace-filter-reset-button'
                  aria-label={t('重置')}
                  onClick={onReset}
                >
                  {t('重置')}
                </Button>
              </Space>
            </div>
          ) : null}
        </div>
        <div className='marketplace-filter-range-row'>
          {showQuotaTimeFilters
            ? renderMarketplaceQuotaRangeInputs(
                filters,
                filterRanges,
                patch,
                t,
                'marketplace-filter-quota-range',
              )
            : null}
          {showQuotaTimeFilters
            ? renderMarketplaceTimeRangeInputs(
                filters,
                filterRanges,
                patch,
                t,
                'marketplace-filter-time-range',
              )
            : null}
          {renderMarketplaceMultiplierRangeInputs(
            filters,
            filterRanges,
            patch,
            t,
            'marketplace-filter-multiplier-range',
          )}
          {showConcurrencyFilter
            ? renderMarketplaceConcurrencyRangeInputs(
                filters,
                filterRanges,
                patch,
                t,
                'marketplace-filter-concurrency-range',
              )
            : null}
        </div>
      </div>
    </Card>
  );
}

function OrdersTab() {
  const { t } = useTranslation();
  const currentUserId = getUserIdFromLocalStorage();
  const [filters, setFilters] = useState(defaultFilters);
  const filterRanges = useMarketplaceFilterRanges(filters);
  const [items, setItems] = useState([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [buying, setBuying] = useState(null);
  const [buyAmountUSD, setBuyAmountUSD] = useState('');
  const [marketplaceFeeRate, setMarketplaceFeeRate] = useState(0);
  const buyFeeRate = normalizeMarketplaceFeeRate(marketplaceFeeRate);
  const estimatedBuyerPaymentUSD = marketplaceBuyerPaymentUSD(
    buyAmountUSD,
    buyFeeRate,
  );

  const load = useCallback(
    async ({ silent = false } = {}) => {
      if (!silent) {
        setLoading(true);
      }
      try {
        const response = await API.get('/api/marketplace/orders', {
          params: compactParams(filters),
        });
        ensureSuccess(response);
        setItems(pageItems(response));
        setTotal(pageTotal(response));
      } catch (error) {
        if (!silent) {
          showError(error.message);
        }
      } finally {
        if (!silent) {
          setLoading(false);
        }
      }
    },
    [filters],
  );

  useEffect(() => {
    load();
  }, [load]);

  useVisibleMarketplaceRefresh(
    () => load({ silent: true }),
    MARKETPLACE_STATUS_REFRESH_INTERVAL_MS,
  );

  useEffect(() => {
    let mounted = true;
    const loadMarketplaceFeeRate = async () => {
      try {
        const response = await API.get('/api/option/');
        const feeRate = normalizeMarketplaceFeeRate(
          readOptionValue(response?.data?.data, 'MarketplaceFeeRate', 0),
        );
        if (mounted) {
          setMarketplaceFeeRate(feeRate);
        }
      } catch {
        if (mounted) {
          setMarketplaceFeeRate(0);
        }
      }
    };
    loadMarketplaceFeeRate();
    return () => {
      mounted = false;
    };
  }, []);

  const createFixedOrder = async () => {
    if (!buying) return;
    try {
      const response = await API.post('/api/marketplace/fixed-orders', {
        credential_id: buying.id,
        purchased_amount_usd: Number(buyAmountUSD),
      });
      ensureSuccess(response);
      showSuccess(t('买断订单已创建'));
      setBuying(null);
      setBuyAmountUSD('');
      load();
    } catch (error) {
      showError(error.message);
    }
  };

  const columns = [
    {
      title: t('厂商'),
      dataIndex: 'vendor_name_snapshot',
      render: (text, record) => getVendorName(record.vendor_type, text),
    },
    {
      title: t('模型'),
      dataIndex: 'models',
      render: (models) => (
        <Space wrap>
          {splitModels(models)
            .slice(0, 4)
            .map((model) => (
              <Tag key={model}>{model}</Tag>
            ))}
        </Space>
      ),
    },
    { title: t('额度'), render: (_, record) => formatQuota(record) },
    { title: t('时间'), render: (_, record) => formatTimeCondition(record) },
    {
      title: t('价格'),
      render: (_, record) => renderMarketplacePricePreview(record),
    },
    {
      title: t('状态'),
      render: (_, record) => renderMarketplaceRouteStatus(record, t),
    },
    {
      title: t('探针评分'),
      render: (_, record) => renderMarketplaceProbeScore(record, t),
    },
    {
      title: t('成功率'),
      render: (_, record) => {
        const totalCount =
          (record.success_count || 0) + (record.upstream_error_count || 0);
        return totalCount
          ? `${Math.round((record.success_count / totalCount) * 100)}%`
          : '-';
      },
    },
    {
      title: t('操作'),
      render: (_, record) => {
        const isOwnOrder = record.seller_user_id === currentUserId;
        const canCreateFixedOrder =
          record?.route_status === 'route_available' && !isOwnOrder;
        const button = (
          <Button
            size='small'
            type='primary'
            disabled={!canCreateFixedOrder}
            onClick={() => {
              if (!canCreateFixedOrder) return;
              setBuying(record);
              setBuyAmountUSD('1');
            }}
          >
            {isOwnOrder ? t('自己的托管') : t('买断金额')}
          </Button>
        );
        if (canCreateFixedOrder) {
          return button;
        }
        const reasonText = isOwnOrder
          ? t('不能购买自己的托管')
          : marketplaceRouteReasonText(record, t);
        return (
          <Tooltip content={reasonText || t('当前订单不可路由')} position='top'>
            <span className='marketplace-disabled-action-tooltip'>
              {button}
            </span>
          </Tooltip>
        );
      },
    },
  ];

  return (
    <Space vertical style={{ width: '100%' }}>
      <FilterBar
        filters={filters}
        filterRanges={filterRanges}
        onChange={setFilters}
        onReset={() => setFilters(defaultFilters)}
      />
      <Table
        rowKey='id'
        columns={columns}
        dataSource={items}
        loading={loading}
        pagination={createUnifiedPaginationProps({
          currentPage: filters.p,
          pageSize: filters.page_size,
          total,
          onPageChange: (page, size) =>
            setFilters({ ...filters, p: page, page_size: size }),
        })}
      />
      <Modal
        title={t('买断金额 (USD)')}
        visible={Boolean(buying)}
        onCancel={() => setBuying(null)}
        onOk={createFixedOrder}
      >
        <Space vertical style={{ width: '100%' }}>
          <Text>
            {buying
              ? getVendorName(buying.vendor_type, buying.vendor_name_snapshot)
              : ''}
          </Text>
          {buying ? (
            <Table
              size='small'
              pagination={false}
              rowKey='model'
              columns={[
                { title: t('模型'), dataIndex: 'model' },
                {
                  title: t('官方计费'),
                  render: (_, record) => formatPricePoint(record.official),
                },
                {
                  title: t('倍率后计费'),
                  render: (_, record) => formatPricePoint(record.buyer),
                },
              ]}
              dataSource={(buying.price_preview || []).slice(0, 4)}
            />
          ) : null}
          <Input
            value={buyAmountUSD}
            placeholder={t('请输入买断美元金额，例如 30')}
            onChange={setBuyAmountUSD}
          />
          <Text type='secondary'>
            {t(
              '填写的是基础调用额度，创建时会额外收取买家交易手续费。当前费率 {{rate}}，预计实际扣除 {{amount}}。',
              {
                rate: formatMarketplaceFeePercent(buyFeeRate),
                amount: formatUSD(estimatedBuyerPaymentUSD),
              },
            )}
          </Text>
        </Space>
      </Modal>
    </Space>
  );
}

function PoolTab({
  buyerTokens,
  selectedBuyerTokenId,
  onBuyerTokenChange,
  onBuyerTokensChange,
}) {
  const { t } = useTranslation();
  const [filters, setFilters] = useState(defaultFilters);
  const [activatingPool, setActivatingPool] = useState(false);
  const [savingPoolFilters, setSavingPoolFilters] = useState(false);
  const [resettingPoolFilters, setResettingPoolFilters] = useState(false);
  const filterRanges = useMarketplaceFilterRanges(filters);
  const [candidates, setCandidates] = useState([]);
  const [loading, setLoading] = useState(false);
  const selectedBuyerToken = useMemo(
    () =>
      buyerTokens.find(
        (token) => token.status === 1 && token.id === selectedBuyerTokenId,
      ),
    [buyerTokens, selectedBuyerTokenId],
  );
  const savedPoolFilters = useMemo(
    () => marketplacePoolSavedFilters(selectedBuyerToken),
    [selectedBuyerToken],
  );
  const poolActivated = marketplacePoolRouteEnabled(selectedBuyerToken);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const params = compactParams(filters);
      const candidateResponse = await API.get(
        '/api/marketplace/pool/candidates',
        { params },
      );
      ensureSuccess(candidateResponse);
      setCandidates(pageItems(candidateResponse));
    } catch (error) {
      showError(error.message);
    } finally {
      setLoading(false);
    }
  }, [filters]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (!selectedBuyerToken) setActivatingPool(false);
  }, [selectedBuyerToken]);

  useEffect(() => {
    setFilters(savedPoolFilters ?? defaultFilters);
  }, [savedPoolFilters]);

  const saveMarketplacePoolFiltersForToken = async () => {
    if (!selectedBuyerToken) return;
    const nextFilters = normalizeMarketplacePoolFilters(filters);
    setSavingPoolFilters(true);
    try {
      const response = await API.post('/api/marketplace/pool/token-filters', {
        token_id: selectedBuyerToken.id,
        filters: marketplacePoolFilterPayload(nextFilters),
      });
      const { success, message, data } = response.data;
      if (!success) {
        showError(t(message || '保存订单池条件失败'));
        return;
      }
      const updatedToken = {
        ...selectedBuyerToken,
        ...(data || {}),
        marketplace_pool_filters_enabled: true,
        marketplace_pool_filters: marketplacePoolFilterPayload(nextFilters),
      };
      onBuyerTokensChange((currentTokens) =>
        currentTokens.map((token) =>
          token.id === updatedToken.id ? updatedToken : token,
        ),
      );
      showSuccess(t('订单池条件已保存'));
    } catch (error) {
      showError(error.message || t('保存订单池条件失败'));
    } finally {
      setSavingPoolFilters(false);
    }
  };

  const resetMarketplacePoolFiltersForToken = async () => {
    if (!selectedBuyerToken) return;
    setResettingPoolFilters(true);
    try {
      const response = await API.delete('/api/marketplace/pool/token-filters', {
        data: { token_id: selectedBuyerToken.id },
      });
      const { success, message, data } = response.data;
      if (!success) {
        showError(t(message || '重置订单池条件失败'));
        return;
      }
      const updatedToken = {
        ...selectedBuyerToken,
        ...(data || {}),
        marketplace_pool_filters_enabled: false,
        marketplace_pool_filters: null,
      };
      onBuyerTokensChange((currentTokens) =>
        currentTokens.map((token) =>
          token.id === updatedToken.id ? updatedToken : token,
        ),
      );
      setFilters(defaultFilters);
      showSuccess(t('订单池条件已重置'));
    } catch (error) {
      showError(error.message || t('重置订单池条件失败'));
    } finally {
      setResettingPoolFilters(false);
    }
  };

  const activateMarketplacePoolForToken = async () => {
    if (!selectedBuyerToken || poolActivated) return;
    const nextToken = tokenWithMarketplacePoolRoute(selectedBuyerToken);
    setActivatingPool(true);
    try {
      const response = await API.put(
        '/api/token/',
        marketplacePoolTokenUpdatePayload(nextToken),
      );
      const { success, message, data } = response.data;
      if (!success) {
        showError(t(message || '激活订单池失败'));
        return;
      }
      const updatedToken = {
        ...nextToken,
        ...(data || {}),
        marketplace_route_order: nextToken.marketplace_route_order,
        marketplace_route_enabled: nextToken.marketplace_route_enabled,
      };
      onBuyerTokensChange((currentTokens) =>
        currentTokens.map((token) =>
          token.id === updatedToken.id ? updatedToken : token,
        ),
      );
      showSuccess(t('该令牌已激活订单池'));
    } catch (error) {
      showError(error.message || t('激活订单池失败'));
    } finally {
      setActivatingPool(false);
    }
  };

  const deactivateMarketplacePoolForToken = async () => {
    if (!selectedBuyerToken || !poolActivated) return;
    const nextToken = tokenWithoutMarketplacePoolRoute(selectedBuyerToken);
    setActivatingPool(true);
    try {
      const response = await API.put(
        '/api/token/',
        marketplacePoolTokenUpdatePayload(nextToken),
      );
      const { success, message, data } = response.data;
      if (!success) {
        showError(t(message || '取消激活订单池失败'));
        return;
      }
      const updatedToken = {
        ...nextToken,
        ...(data || {}),
        marketplace_route_order: nextToken.marketplace_route_order,
        marketplace_route_enabled: nextToken.marketplace_route_enabled,
      };
      onBuyerTokensChange((currentTokens) =>
        currentTokens.map((token) =>
          token.id === updatedToken.id ? updatedToken : token,
        ),
      );
      showSuccess(t('该令牌已取消激活订单池'));
    } catch (error) {
      showError(error.message || t('取消激活订单池失败'));
    } finally {
      setActivatingPool(false);
    }
  };

  const toggleMarketplacePoolForToken = poolActivated
    ? deactivateMarketplacePoolForToken
    : activateMarketplacePoolForToken;

  return (
    <Space vertical style={{ width: '100%' }}>
      <BuyerTokenPanel
        tokens={buyerTokens}
        selectedTokenId={selectedBuyerTokenId}
        onChange={onBuyerTokenChange}
      />
      <FilterBar
        filters={filters}
        filterRanges={filterRanges}
        onChange={setFilters}
        showResetButton={false}
        showQuotaTimeFilters={false}
        showConcurrencyFilter={false}
      />
      <div className='marketplace-pool-activation'>
        <div className='marketplace-pool-activation-header'>
          <Space>
            <IconRoute />
            <Text strong>{t('订单池激活')}</Text>
          </Space>
          <Space wrap>
            <Button
              theme='light'
              type='tertiary'
              icon={<IconRefresh />}
              loading={resettingPoolFilters}
              disabled={
                !selectedBuyerToken || resettingPoolFilters || savingPoolFilters
              }
              onClick={resetMarketplacePoolFiltersForToken}
            >
              {t('重置条件')}
            </Button>
            <Button
              theme='light'
              loading={savingPoolFilters}
              disabled={
                !selectedBuyerToken || savingPoolFilters || resettingPoolFilters
              }
              onClick={saveMarketplacePoolFiltersForToken}
            >
              {t('保存条件')}
            </Button>
            <Button
              type='primary'
              theme={poolActivated ? 'light' : 'solid'}
              loading={activatingPool}
              disabled={!selectedBuyerToken || activatingPool}
              onClick={toggleMarketplacePoolForToken}
            >
              {activatingPool
                ? t(poolActivated ? '取消激活中' : '激活中')
                : t(poolActivated ? '取消激活' : '激活使用')}
            </Button>
          </Space>
        </div>
      </div>
      <Table
        rowKey={(record) => record.credential?.id}
        title={() => t('路由候选')}
        columns={[
          {
            title: t('厂商'),
            render: (_, record) =>
              getVendorName(
                record.credential?.vendor_type,
                record.credential?.vendor_name_snapshot,
              ),
          },
          { title: t('评分'), dataIndex: 'route_score' },
          {
            title: t('价格'),
            render: (_, record) =>
              renderMarketplacePricePreview(record.credential),
          },
          {
            title: t('成功率'),
            render: (_, record) =>
              `${Math.round((record.success_rate || 0) * 100)}%`,
          },
          {
            title: t('状态'),
            render: (_, record) =>
              renderMarketplaceRouteStatus(record.credential, t),
          },
          {
            title: t('探针评分'),
            render: (_, record) =>
              renderMarketplaceProbeScore(record.credential, t),
          },
          {
            title: t('负载'),
            render: (_, record) =>
              `${Math.round((record.load_ratio || 0) * 100)}%`,
          },
        ]}
        dataSource={candidates}
        loading={loading}
        pagination={false}
      />
    </Space>
  );
}

function FixedOrdersTab({ buyerTokens, onBuyerTokensChange }) {
  const { t } = useTranslation();
  const [page, setPage] = useState(1);
  const [items, setItems] = useState([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [bindingOpen, setBindingOpen] = useState(false);
  const [bindingOrderId, setBindingOrderId] = useState();
  const [bindingTokenSelection, setBindingTokenSelection] = useState([]);
  const [bindingSaving, setBindingSaving] = useState(false);
  const [visibleTokenKeys, setVisibleTokenKeys] = useState({});
  const [resolvedTokenKeys, setResolvedTokenKeys] = useState({});
  const [loadingTokenKeys, setLoadingTokenKeys] = useState({});
  const [probingOrderId, setProbingOrderId] = useState(null);
  const enabledTokens = useMemo(
    () => buyerTokens.filter((token) => token.status === 1),
    [buyerTokens],
  );

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const response = await API.get('/api/marketplace/fixed-orders', {
        params: { p: page, page_size: PAGE_SIZE },
      });
      ensureSuccess(response);
      setItems(pageItems(response));
      setTotal(pageTotal(response));
    } catch (error) {
      showError(error.message);
    } finally {
      setLoading(false);
    }
  }, [page]);

  useEffect(() => {
    load();
  }, [load]);

  const tokenIdsBoundToOrder = useCallback(
    (orderId) =>
      enabledTokens
        .filter((token) =>
          marketplaceTokenFixedOrderIds(token).includes(orderId),
        )
        .map((token) => token.id),
    [enabledTokens],
  );

  const boundTokensForOrder = useCallback(
    (orderId) =>
      enabledTokens.filter((token) =>
        marketplaceTokenFixedOrderIds(token).includes(orderId),
      ),
    [enabledTokens],
  );

  const resolveBoundTokenKey = useCallback(
    async (token) => {
      if (!token?.id) return '';
      if (resolvedTokenKeys[token.id]) return resolvedTokenKeys[token.id];

      setLoadingTokenKeys((current) => ({ ...current, [token.id]: true }));
      try {
        const key = await fetchTokenKey(token.id);
        setResolvedTokenKeys((current) => ({ ...current, [token.id]: key }));
        return key;
      } catch (error) {
        showError(error.message || t('获取令牌密钥失败'));
        return '';
      } finally {
        setLoadingTokenKeys((current) => ({ ...current, [token.id]: false }));
      }
    },
    [resolvedTokenKeys, t],
  );

  const toggleBoundTokenVisibility = useCallback(
    async (token) => {
      if (visibleTokenKeys[token.id]) {
        setVisibleTokenKeys((current) => ({ ...current, [token.id]: false }));
        return;
      }

      const key = await resolveBoundTokenKey(token);
      if (key) {
        setVisibleTokenKeys((current) => ({ ...current, [token.id]: true }));
      }
    },
    [resolveBoundTokenKey, visibleTokenKeys],
  );

  const copyBoundTokenKey = useCallback(
    async (token) => {
      const key = await resolveBoundTokenKey(token);
      if (!key) return;

      const ok = await copy(fullTokenKey(key));
      if (ok) {
        showSuccess(t('已复制'));
      } else {
        showError(t('复制失败'));
      }
    },
    [resolveBoundTokenKey, t],
  );

  const copyBoundTokenConnectionInfo = useCallback(
    async (token) => {
      const key = await resolveBoundTokenKey(token);
      if (!key) return;

      const ok = await copy(
        encodeChannelConnectionString(fullTokenKey(key), getServerAddress()),
      );
      if (ok) {
        showSuccess(t('已复制'));
      } else {
        showError(t('复制失败'));
      }
    },
    [resolveBoundTokenKey, t],
  );

  const renderBoundTokens = (orderId) => {
    const boundTokens = boundTokensForOrder(orderId);
    if (boundTokens.length === 0) {
      return (
        <Tag color='white' shape='circle'>
          {t('未绑定')}
        </Tag>
      );
    }

    return (
      <BoundTokenDropdown
        tokens={boundTokens}
        visibleTokenKeys={visibleTokenKeys}
        resolvedTokenKeys={resolvedTokenKeys}
        loadingTokenKeys={loadingTokenKeys}
        onToggleVisibility={toggleBoundTokenVisibility}
        onCopyKey={copyBoundTokenKey}
        onCopyConnectionInfo={copyBoundTokenConnectionInfo}
        t={t}
      />
    );
  };

  const openBindingEditor = (orderId) => {
    setBindingOrderId(orderId);
    setBindingTokenSelection(tokenIdsBoundToOrder(orderId));
    setBindingOpen(true);
  };

  const toggleBindingTokenId = (tokenId) => {
    setBindingTokenSelection((ids) => {
      if (ids.includes(tokenId)) {
        return ids.filter((id) => id !== tokenId);
      }
      return [...ids, tokenId];
    });
  };

  const saveBindingSelection = async () => {
    if (!bindingOrderId) {
      showError(t('请先选择买断订单'));
      return;
    }
    setBindingSaving(true);
    try {
      await bindMarketplaceFixedOrderTokens({
        fixedOrderId: bindingOrderId,
        tokenIds: bindingTokenSelection,
      });
      onBuyerTokensChange?.(
        applyMarketplaceFixedOrderTokenBindings(
          buyerTokens,
          bindingOrderId,
          bindingTokenSelection,
        ),
      );
      showSuccess(t('令牌绑定已更新'));
      setBindingOpen(false);
    } catch (error) {
      showError(error.message || t('绑定失败'));
    } finally {
      setBindingSaving(false);
    }
  };

  const probeFixedOrder = async (record) => {
    if (!record?.id) return;
    setProbingOrderId(record.id);
    try {
      const response = await API.post(
        `/api/marketplace/fixed-orders/${record.id}/probe`,
        null,
        { skipErrorHandler: true },
      );
      ensureSuccess(response);
      const updatedOrder = response?.data?.data;
      setItems((current) =>
        current.map((item) =>
          item.id === updatedOrder?.id ? { ...item, ...updatedOrder } : item,
        ),
      );
      const scoreDrop =
        Number(updatedOrder?.purchase_probe_score) -
        Number(updatedOrder?.refund_probe_score);
      showSuccess(
        scoreDrop >= 5
          ? t('检测完成，该订单可以解除')
          : t('检测完成，该订单不满足解除条件'),
      );
      await load();
    } catch (error) {
      showError(error.message || t('检测失败'));
    } finally {
      setProbingOrderId(null);
    }
  };

  const releaseFixedOrder = async (record) => {
    if (!record?.id) return;
    Modal.confirm({
      title: t('解除买断订单'),
      content: t(
        '该订单最近一次检测已不合格。解除后将退还剩余金额并移除令牌绑定。',
      ),
      okText: t('解除订单'),
      cancelText: t('取消'),
      onOk: async () => {
        setProbingOrderId(record.id);
        try {
          const response = await API.post(
            `/api/marketplace/fixed-orders/${record.id}/release`,
            null,
            { skipErrorHandler: true },
          );
          ensureSuccess(response);
          const updatedOrder = response?.data?.data;
          setItems((current) =>
            current.map((item) =>
              item.id === updatedOrder?.id
                ? { ...item, ...updatedOrder }
                : item,
            ),
          );
          const refundedQuota = Number(updatedOrder?.refunded_quota) || 0;
          showSuccess(
            refundedQuota > 0
              ? `${t('订单已解除，剩余金额已退还')}：${formatMarketplaceQuotaUSD(
                  refundedQuota,
                )}`
              : t('订单已解除，剩余金额已退还'),
          );
          onBuyerTokensChange?.(
            buyerTokens.map((token) => {
              const nextOrderIds = marketplaceTokenFixedOrderIds(token).filter(
                (orderId) => orderId !== record.id,
              );
              return {
                ...token,
                marketplace_fixed_order_id: nextOrderIds[0] || 0,
                marketplace_fixed_order_ids: nextOrderIds,
              };
            }),
          );
          await load();
        } catch (error) {
          showError(error.message || t('检测或解除失败'));
        } finally {
          setProbingOrderId(null);
        }
      },
    });
  };

  const canProbeAndRefundFixedOrder = (record) =>
    record?.status === 'active' &&
    Number(record?.remaining_quota) > 0 &&
    Number(record?.purchase_probe_score) > 0;

  const canReleaseFixedOrder = (record) =>
    canProbeAndRefundFixedOrder(record) &&
    Number(record?.refund_probe_checked_at) > 0 &&
    Number(record?.purchase_probe_score) - Number(record?.refund_probe_score) >=
      5;

  return (
    <>
      <Table
        rowKey='id'
        className='marketplace-fixed-orders-table'
        size='middle'
        tableLayout='fixed'
        columns={[
          {
            title: t('订单'),
            width: 116,
            render: (_, record) => renderFixedOrderIdentity(record, t),
          },
          {
            title: t('状态'),
            width: 120,
            render: (_, record) => renderFixedOrderCombinedStatus(record, t),
          },
          {
            title: t('探针评分'),
            width: 128,
            render: (_, record) => renderMarketplaceProbeScore(record, t),
          },
          {
            title: t('金额'),
            width: 210,
            render: (_, record) => renderFixedOrderAmounts(record, t),
          },
          {
            title: t('调用令牌'),
            render: (_, record) => (
              <div className='marketplace-fixed-order-call'>
                <CallSnippet
                  orderId={record.id}
                  onEditBindings={() => openBindingEditor(record.id)}
                />
                <div className='marketplace-fixed-order-bound'>
                  {renderBoundTokens(record.id)}
                </div>
              </div>
            ),
          },
          {
            title: t('操作'),
            width: 72,
            render: (_, record) => {
              const moreMenu = [
                {
                  node: 'item',
                  name: t('检测'),
                  disabled: !canProbeAndRefundFixedOrder(record),
                  onClick: () => probeFixedOrder(record),
                },
                {
                  node: 'item',
                  name: t('解除订单'),
                  type: 'danger',
                  disabled: !canReleaseFixedOrder(record),
                  onClick: () => releaseFixedOrder(record),
                },
              ];

              return (
                <div className='marketplace-fixed-order-actions'>
                  <Dropdown
                    trigger='click'
                    position='bottomRight'
                    menu={moreMenu}
                  >
                    <Button
                      size='small'
                      type='tertiary'
                      icon={<IconMore />}
                      loading={probingOrderId === record.id}
                    />
                  </Dropdown>
                </div>
              );
            },
          },
        ]}
        dataSource={items}
        loading={loading}
        pagination={createUnifiedPaginationProps({
          currentPage: page,
          pageSize: PAGE_SIZE,
          total,
          onPageChange: setPage,
        })}
      />
      <Modal
        title={t('编辑绑定令牌')}
        visible={bindingOpen}
        onCancel={() => setBindingOpen(false)}
        onOk={saveBindingSelection}
        confirmLoading={bindingSaving}
      >
        <Space vertical style={{ width: '100%' }}>
          <Text type='secondary'>
            {t('选择一个或多个控制台令牌，保存后该市场订单会绑定这些令牌。')}
          </Text>
          {enabledTokens.length === 0 ? (
            <Text type='secondary'>{t('暂无启用令牌')}</Text>
          ) : null}
          {enabledTokens.map((token) => {
            const checked = bindingTokenSelection.includes(token.id);
            return (
              <Card key={token.id} bodyStyle={{ padding: 12 }}>
                <Space style={{ width: '100%' }} align='center'>
                  <Button
                    size='small'
                    theme={checked ? 'solid' : 'light'}
                    type={checked ? 'primary' : 'tertiary'}
                    onClick={() => toggleBindingTokenId(token.id)}
                  >
                    {checked ? t('已选择') : t('选择')}
                  </Button>
                  <Text strong>{token.name}</Text>
                  <Tag>sk-{token.key}</Tag>
                  <Tag color={checked ? 'green' : 'white'}>
                    {checked ? t('已绑定') : t('未绑定')}
                  </Tag>
                </Space>
              </Card>
            );
          })}
        </Space>
      </Modal>
    </>
  );
}

function SellerTab() {
  const { t } = useTranslation();
  const isMobile = useIsMobile();
  const [form, setForm] = useState(defaultCredentialForm);
  const [editing, setEditing] = useState(null);
  const [items, setItems] = useState([]);
  const [maxCredentialConcurrency, setMaxCredentialConcurrency] = useState(
    DEFAULT_MAX_CREDENTIAL_CONCURRENCY,
  );
  const [pricingByModel, setPricingByModel] = useState({});
  const [customModel, setCustomModel] = useState('');
  const [loading, setLoading] = useState(false);
  const [showModelTestModal, setShowModelTestModal] = useState(false);
  const [modelActionMode, setModelActionMode] = useState('test');
  const [currentTestChannel, setCurrentTestChannel] = useState(null);
  const [modelTestResults, setModelTestResults] = useState({});
  const [testingModels, setTestingModels] = useState(new Set());
  const [isBatchTesting, setIsBatchTesting] = useState(false);
  const [modelSearchKeyword, setModelSearchKeyword] = useState('');
  const [selectedModelKeys, setSelectedModelKeys] = useState([]);
  const [modelTablePage, setModelTablePage] = useState(1);
  const [selectedEndpointType, setSelectedEndpointType] = useState('');
  const [isStreamTest, setIsStreamTest] = useState(false);
  const allSelectingRef = useRef(false);
  const shouldStopBatchTestingRef = useRef(false);
  const pricedModelsRef = useRef([]);
  const pricedModelsLoadedRef = useRef(false);
  const pricedModelsPromiseRef = useRef(null);

  const patch = (next) => setForm({ ...form, ...next });
  const modelActionResultKey = (mode, credentialID, modelName) =>
    `${mode}:${credentialID}-${String(modelName || '').trim()}`;

  const load = useCallback(async ({ silent = false } = {}) => {
    if (!silent) {
      setLoading(true);
    }
    try {
      const response = await API.get('/api/marketplace/seller/credentials', {
        params: { p: 1, page_size: PAGE_SIZE },
      });
      ensureSuccess(response);
      setItems(pageItems(response));
    } catch (error) {
      if (!silent) {
        showError(error.message);
      }
    } finally {
      if (!silent) {
        setLoading(false);
      }
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  useVisibleMarketplaceRefresh(
    () => load({ silent: true }),
    MARKETPLACE_STATUS_REFRESH_INTERVAL_MS,
  );

  useEffect(() => {
    let mounted = true;
    const loadMarketplaceSettings = async () => {
      try {
        const response = await API.get('/api/option/');
        const value = readOptionValue(
          response?.data?.data,
          'MarketplaceMaxCredentialConcurrency',
          DEFAULT_MAX_CREDENTIAL_CONCURRENCY,
        );
        if (mounted) {
          setMaxCredentialConcurrency(normalizeMaxCredentialConcurrency(value));
        }
      } catch {
        // The backend will still enforce the default marketplace concurrency limit.
      }
    };
    loadMarketplaceSettings();
    return () => {
      mounted = false;
    };
  }, []);

  const loadPricedModels = useCallback(
    async ({ silent = false } = {}) => {
      if (pricedModelsLoadedRef.current) {
        return pricedModelsRef.current;
      }

      let request = pricedModelsPromiseRef.current;
      if (!request) {
        request = (async () => {
          const response = await API.get(
            '/api/marketplace/seller/priced-models',
            {
              skipErrorHandler: true,
            },
          );
          if (!response?.data?.success) {
            throw new Error(response?.data?.message || t('获取模型列表失败'));
          }
          const pricedModelsByName = {};
          for (const item of response?.data?.data || []) {
            const modelName = modelNameFromPricingItem(item);
            if (!modelName || pricedModelsByName[modelName]) continue;
            pricedModelsByName[modelName] = pricingPointFromPricingItem(item);
          }
          const modelNames = Object.keys(pricedModelsByName);
          pricedModelsRef.current = modelNames;
          setPricingByModel(pricedModelsByName);
          pricedModelsLoadedRef.current = true;
          return modelNames;
        })();

        pricedModelsPromiseRef.current = request;
      }
      try {
        return await request;
      } catch (error) {
        if (!silent) {
          showError(error.message || t('获取模型列表失败'));
        }
        return [];
      } finally {
        if (pricedModelsPromiseRef.current === request) {
          pricedModelsPromiseRef.current = null;
        }
      }
    },
    [t],
  );
  const selectedModels = useMemo(() => splitModels(form.models), [form.models]);
  const marketplaceModelsForSave = useMemo(
    () => mergeModels(selectedModels, splitModels(customModel)),
    [customModel, selectedModels],
  );

  useEffect(() => {
    loadPricedModels({ silent: true });
  }, [loadPricedModels]);
  const sellerPricePreview = useMemo(
    () =>
      buildSellerPricePreview(
        marketplaceModelsForSave,
        pricingByModel,
        Number(form.multiplier) || 1,
      ),
    [form.multiplier, marketplaceModelsForSave, pricingByModel],
  );
  const sellerPricePreviewColumns = useMemo(
    () => [
      { title: t('模型'), dataIndex: 'model' },
      {
        title: t('官方计费'),
        render: (_, record) => formatPricePoint(record.official),
      },
      {
        title: t('倍率后计费'),
        render: (_, record) => formatPricePoint(record.buyer),
      },
    ],
    [t],
  );

  const setSelectedModels = (models) => {
    const nextModels = mergeModels(models);
    patch({ models: nextModels.join(',') });
  };

  const addCustomModels = () => {
    const nextModels = splitModels(customModel);
    if (nextModels.length === 0) return;
    const merged = mergeModels(selectedModels, nextModels);
    setCustomModel('');
    setSelectedModels(merged);
    showSuccess(
      t('已新增 {{count}} 个模型：{{list}}', {
        count: nextModels.length,
        list: nextModels.join(', '),
      }),
    );
  };

  const fetchUpstreamModels = async () => {
    if (!MODEL_FETCHABLE_CHANNEL_TYPES.has(Number(form.vendor_type))) {
      showInfo(t('该渠道类型暂不支持获取模型列表'));
      return;
    }
    if (!form.api_key.trim()) {
      showError(t('请填写密钥'));
      return;
    }
    setLoading(true);
    try {
      const response = await API.post(
        '/api/marketplace/seller/credentials/fetch-models',
        {
          base_url: form.base_url.trim(),
          vendor_type: Number(form.vendor_type),
          api_key: form.api_key.trim(),
          other: form.other.trim(),
          model_mapping: form.model_mapping.trim(),
          status_code_mapping: form.status_code_mapping.trim(),
          setting: buildMarketplaceCredentialSetting(form.setting, form.proxy),
          param_override: form.param_override.trim(),
          settings: form.settings.trim(),
        },
        { skipErrorHandler: true },
      );
      if (!response?.data?.success) {
        throw new Error(response?.data?.message || t('获取模型列表失败'));
      }
      const fetchedModels = mergeModels(response.data.data || []);
      if (fetchedModels.length === 0) {
        showInfo(t('暂无模型'));
        return;
      }
      setSelectedModels(fetchedModels);
      showSuccess(t('模型列表已更新'));
    } catch (error) {
      showError(error.message || t('获取模型列表失败'));
    } finally {
      setLoading(false);
    }
  };

  const payload = useMemo(
    () => ({
      vendor_type: Number(form.vendor_type),
      api_key: form.api_key.trim(),
      base_url: form.base_url.trim(),
      other: form.other.trim(),
      model_mapping: form.model_mapping.trim(),
      status_code_mapping: form.status_code_mapping.trim(),
      setting: buildMarketplaceCredentialSetting(form.setting, form.proxy),
      param_override: form.param_override.trim(),
      settings: form.settings.trim(),
      models: marketplaceModelsForSave,
      quota_mode: form.quota_mode,
      quota_limit:
        form.quota_mode === 'limited'
          ? displayAmountToQuota(form.quota_limit)
          : 0,
      time_mode: form.time_mode,
      time_limit_seconds:
        form.time_mode === 'limited' ? Number(form.time_limit_minutes) * 60 : 0,
      multiplier: Number(form.multiplier) || 1,
      concurrency_limit: clampCredentialConcurrency(
        form.concurrency_limit,
        maxCredentialConcurrency,
      ),
    }),
    [form, marketplaceModelsForSave, maxCredentialConcurrency],
  );

  const save = async () => {
    if (marketplaceModelsForSave.length === 0) {
      showError(t('请至少选择一个模型'));
      return;
    }

    try {
      const response = editing
        ? await API.put(
            `/api/marketplace/seller/credentials/${editing.id}`,
            payload,
          )
        : await API.post('/api/marketplace/seller/credentials', payload);
      ensureSuccess(response);
      showSuccess(editing ? t('保存成功') : t('创建成功'));
      setEditing(null);
      setForm(defaultCredentialForm);
      setCustomModel('');
      load();
    } catch (error) {
      showError(error.message);
    }
  };

  const callAction = async (record, action) => {
    try {
      const response = await API.post(
        `/api/marketplace/seller/credentials/${record.id}/${action}`,
      );
      ensureSuccess(response);
      showSuccess(action === 'probe' ? t('检测已排队') : t('操作成功'));
      load();
    } catch (error) {
      showError(error.message);
    }
  };

  const deleteMarketplaceCredential = (record) => {
    Modal.confirm({
      title: t('确认删除托管 Key'),
      content: t('删除后无法恢复；如果仍有有效买断订单，系统会阻止删除。'),
      okText: t('删除'),
      cancelText: t('取消'),
      type: 'warning',
      okType: 'danger',
      onOk: () => {
        (async () => {
          try {
            const response = await API.delete(
              `/api/marketplace/seller/credentials/${record.id}`,
            );
            ensureSuccess(response);
            showSuccess(t('删除成功'));
            setItems((current) =>
              current.filter((item) => item.id !== record.id),
            );
            if (editing?.id === record.id) {
              setEditing(null);
              setForm(defaultCredentialForm);
              setCustomModel('');
            }
            if (currentTestChannel?.id === record.id) {
              setShowModelTestModal(false);
              setCurrentTestChannel(null);
            }
            await load();
          } catch (error) {
            showError(error.message || t('删除失败'));
          }
        })();
      },
    });
  };

  const updateTestedCredential = (credential) => {
    if (!credential?.id) return;
    setItems((current) =>
      current.map((item) =>
        item.id === credential.id ? { ...item, ...credential } : item,
      ),
    );
    setCurrentTestChannel((current) =>
      current?.id === credential.id
        ? marketplaceCredentialTestChannel({ ...current, ...credential })
        : current,
    );
  };

  const testMarketplaceCredential = async (
    record,
    model = '',
    endpointType = '',
    stream = false,
  ) => {
    const testModel = String(model || '').trim();
    const testKey = modelActionResultKey('test', record.id, testModel);

    if (shouldStopBatchTestingRef.current) {
      return undefined;
    }

    setTestingModels((current) => new Set([...current, testModel]));

    try {
      let url = `/api/marketplace/seller/credentials/${record.id}/test?model=${encodeURIComponent(testModel)}`;
      if (endpointType) {
        url += `&endpoint_type=${encodeURIComponent(endpointType)}`;
      }
      if (stream) {
        url += '&stream=true';
      }
      const response = await API.post(url, null, { skipErrorHandler: true });
      const responseData = response?.data || {};
      const updatedCredential = responseData.data;
      const responseTimeMS = Number(updatedCredential?.response_time) || 0;
      const elapsedSeconds = responseTimeMS > 0 ? responseTimeMS / 1000 : 0;

      if (updatedCredential?.id) {
        updateTestedCredential(updatedCredential);
      }

      setModelTestResults((current) => ({
        ...current,
        [testKey]: {
          success: Boolean(responseData.success),
          message: responseData.message || '',
          time: elapsedSeconds,
          timestamp: Date.now(),
          errorCode: responseData.error_code || null,
        },
      }));

      if (!responseData.success) {
        showError(responseData.message || t('测试失败'));
        return response;
      }

      const channelName =
        record.name || marketplaceCredentialTestChannel(record)?.name || '';
      if (!testModel) {
        showInfo(
          t('通道 ${name} 测试成功，耗时 ${time.toFixed(2)} 秒。')
            .replace('${name}', channelName)
            .replace('${time.toFixed(2)}', elapsedSeconds.toFixed(2)),
        );
      } else {
        showInfo(
          t('通道 ${name} 测试成功，模型 ${model} 耗时 ${time.toFixed(2)} 秒。')
            .replace('${name}', channelName)
            .replace('${model}', testModel)
            .replace('${time.toFixed(2)}', elapsedSeconds.toFixed(2)),
        );
      }
      return response;
    } catch (error) {
      setModelTestResults((current) => ({
        ...current,
        [testKey]: {
          success: false,
          message: error.message || t('网络错误'),
          time: 0,
          timestamp: Date.now(),
          errorCode: null,
        },
      }));
      showError(error.message || t('测试失败'));
      return undefined;
    } finally {
      setTestingModels((current) => {
        const next = new Set(current);
        next.delete(testModel);
        return next;
      });
    }
  };

  const probeMarketplaceCredential = async (record, model = '') => {
    const probeModel = String(model || '').trim();
    const probeKey = modelActionResultKey('probe', record.id, probeModel);

    if (shouldStopBatchTestingRef.current) {
      return undefined;
    }

    setTestingModels((current) => new Set([...current, probeModel]));

    try {
      let url = `/api/marketplace/seller/credentials/${record.id}/probe`;
      if (probeModel) {
        url += `?model=${encodeURIComponent(probeModel)}`;
      }
      const response = await API.post(url, null, {
        skipErrorHandler: true,
      });
      const responseData = response?.data || {};
      const updatedCredential = responseData.data;

      if (responseData.success && updatedCredential?.id) {
        updateTestedCredential(updatedCredential);
      }

      setModelTestResults((current) => ({
        ...current,
        [probeKey]: {
          success: Boolean(responseData.success),
          message: responseData.message || '',
          time: 0,
          timestamp: Date.now(),
          errorCode: responseData.error_code || null,
        },
      }));

      if (!responseData.success) {
        showError(responseData.message || t('检测失败'));
        return response;
      }

      showSuccess(
        probeModel
          ? t('模型 {{model}} 检测已排队', { model: probeModel })
          : t('检测已排队'),
      );
      return response;
    } catch (error) {
      setModelTestResults((current) => ({
        ...current,
        [probeKey]: {
          success: false,
          message: error.message || t('网络错误'),
          time: 0,
          timestamp: Date.now(),
          errorCode: null,
        },
      }));
      showError(error.message || t('检测失败'));
      return undefined;
    } finally {
      setTestingModels((current) => {
        const next = new Set(current);
        next.delete(probeModel);
        return next;
      });
    }
  };

  const runMarketplaceCredentialModelAction = (
    record,
    model = '',
    endpointType = '',
    stream = false,
  ) => {
    if (modelActionMode === 'probe') {
      return probeMarketplaceCredential(record, model);
    }
    return testMarketplaceCredential(record, model, endpointType, stream);
  };

  const batchRunMarketplaceCredentialModels = async () => {
    if (!currentTestChannel?.models) {
      showError(t('渠道模型信息不完整'));
      return;
    }

    const models = splitModels(currentTestChannel.models).filter((model) =>
      model.toLowerCase().includes(modelSearchKeyword.toLowerCase()),
    );

    if (models.length === 0) {
      showError(t('没有找到匹配的模型'));
      return;
    }

    setIsBatchTesting(true);
    shouldStopBatchTestingRef.current = false;
    setModelTestResults((current) => {
      const next = { ...current };
      models.forEach((model) => {
        delete next[
          modelActionResultKey(modelActionMode, currentTestChannel.id, model)
        ];
      });
      return next;
    });

    const isProbeMode = modelActionMode === 'probe';
    const actionText = isProbeMode ? t('检测') : t('测试');
    try {
      showInfo(
        t('开始批量{{action}} {{count}} 个模型，已清空上次结果...', {
          action: actionText,
          count: models.length,
        }),
      );
      const concurrencyLimit = 5;
      for (let i = 0; i < models.length; i += concurrencyLimit) {
        if (shouldStopBatchTestingRef.current) {
          showInfo(t('批量{{action}}已停止', { action: actionText }));
          break;
        }

        const batch = models.slice(i, i + concurrencyLimit);
        showInfo(
          t('正在{{action}}第 {{current}} - {{end}} 个模型 (共 {{total}} 个)', {
            action: actionText,
            current: i + 1,
            end: Math.min(i + concurrencyLimit, models.length),
            total: models.length,
          }),
        );

        await Promise.allSettled(
          batch.map((model) =>
            runMarketplaceCredentialModelAction(
              currentTestChannel,
              model,
              selectedEndpointType,
              isStreamTest,
            ),
          ),
        );

        if (shouldStopBatchTestingRef.current) {
          showInfo(t('批量{{action}}已停止', { action: actionText }));
          break;
        }

        if (i + concurrencyLimit < models.length) {
          await new Promise((resolve) => setTimeout(resolve, 100));
        }
      }

      if (!shouldStopBatchTestingRef.current) {
        setModelTestResults((currentResults) => {
          let successCount = 0;
          let failCount = 0;
          models.forEach((model) => {
            const result =
              currentResults[
                modelActionResultKey(
                  modelActionMode,
                  currentTestChannel.id,
                  model,
                )
              ];
            if (result?.success) {
              successCount++;
            } else {
              failCount++;
            }
          });
          setTimeout(() => {
            showSuccess(
              t(
                '批量{{action}}完成！成功: {{success}}, 失败: {{fail}}, 总计: {{total}}',
                {
                  action: actionText,
                  success: successCount,
                  fail: failCount,
                  total: models.length,
                },
              ),
            );
          }, 100);
          return currentResults;
        });
      }
    } catch (error) {
      showError(
        t('批量{{action}}过程中发生错误: ', { action: actionText }) +
          error.message,
      );
    } finally {
      setIsBatchTesting(false);
    }
  };

  const openMarketplaceModelActionModal = (record, mode = 'test') => {
    shouldStopBatchTestingRef.current = false;
    setModelActionMode(mode);
    setCurrentTestChannel(marketplaceCredentialTestChannel(record));
    setModelSearchKeyword('');
    setSelectedModelKeys([]);
    setModelTablePage(1);
    setShowModelTestModal(true);
  };

  const handleCloseModal = () => {
    if (isBatchTesting) {
      shouldStopBatchTestingRef.current = true;
      showInfo(
        modelActionMode === 'probe'
          ? t('关闭弹窗，已停止批量检测')
          : t('关闭弹窗，已停止批量测试'),
      );
    }
    setShowModelTestModal(false);
    setModelSearchKeyword('');
    setSelectedModelKeys([]);
    setModelTablePage(1);
    setIsBatchTesting(false);
    setTestingModels(new Set());
    setSelectedEndpointType('');
    setIsStreamTest(false);
  };

  const edit = (record) => {
    setEditing(record);
    setCustomModel('');
    setForm({
      ...defaultCredentialForm,
      vendor_type: record.vendor_type,
      api_key: '',
      base_url: record.base_url || '',
      other: record.other || '',
      model_mapping: record.model_mapping || '',
      status_code_mapping: record.status_code_mapping || '',
      setting: record.setting || '',
      proxy: marketplaceCredentialProxy(record.setting),
      param_override: record.param_override || '',
      settings: record.settings || '',
      models: splitModels(record.models).join(','),
      quota_mode: record.quota_mode || 'unlimited',
      quota_limit: record.quota_limit
        ? String(quotaToDisplayAmount(record.quota_limit))
        : '',
      time_mode: record.time_mode || 'unlimited',
      time_limit_minutes: record.time_limit_seconds
        ? String(Math.ceil(record.time_limit_seconds / 60))
        : '',
      multiplier: String(record.multiplier || 1),
      concurrency_limit: String(
        clampCredentialConcurrency(
          record.concurrency_limit,
          maxCredentialConcurrency,
        ),
      ),
    });
  };

  return (
    <Space vertical style={{ width: '100%' }}>
      <Card title={editing ? t('编辑托管Key') : t('托管 AI API Key')}>
        <Row gutter={[12, 12]}>
          <Col xs={24} md={6}>
            <MarketplaceField label={t('类型 *')}>
              <Select
                value={form.vendor_type}
                onChange={(value) => patch({ vendor_type: value })}
                optionList={CHANNEL_OPTIONS.map((option) => ({
                  label: t(option.label),
                  value: option.value,
                }))}
                style={{ width: '100%' }}
              />
            </MarketplaceField>
          </Col>
          <Col xs={24} md={10}>
            <MarketplaceField label='API Key'>
              <Input
                value={form.api_key}
                placeholder={editing ? t('留空则不更新 Key') : 'sk-...'}
                onChange={(value) => patch({ api_key: value })}
              />
            </MarketplaceField>
          </Col>
          <Col xs={24} md={8}>
            <MarketplaceField label={t('接口地址 / Base URL')}>
              <Input
                value={form.base_url}
                placeholder={t('https://api.openai.com，可选')}
                onChange={(value) => patch({ base_url: value })}
              />
            </MarketplaceField>
          </Col>
          <Col xs={24} md={8}>
            <MarketplaceField
              label={t('代理地址')}
              help={t('用于配置网络代理，支持 socks5 协议')}
            >
              <Input
                value={form.proxy}
                placeholder={t('例如: socks5://user:pass@host:port')}
                onChange={(value) => patch({ proxy: value })}
                showClear
              />
            </MarketplaceField>
          </Col>
          <Col xs={24}>
            <MarketplaceField
              label={t('模型 *')}
              help={t(
                '读取模型定价设置中的模型，卖家的模型会成为买家的筛选条件',
              )}
            >
              <Select
                className='marketplace-seller-model-select'
                dropdownClassName='marketplace-seller-model-dropdown-hidden'
                value={selectedModels}
                placeholder={t('暂无模型')}
                multiple
                optionList={[]}
                emptyContent={null}
                showArrow={false}
                autoAdjustOverflow={false}
                style={{ width: '100%' }}
                onChange={(value) => setSelectedModels(value)}
                onDropdownVisibleChange={() => {}}
              />
              <Space wrap className='marketplace-seller-model-actions'>
                {MODEL_FETCHABLE_CHANNEL_TYPES.has(
                  Number(form.vendor_type),
                ) && (
                  <Button
                    size='small'
                    type='tertiary'
                    onClick={fetchUpstreamModels}
                  >
                    {t('获取模型列表')}
                  </Button>
                )}
              </Space>
            </MarketplaceField>
          </Col>
          <Col xs={24}>
            <MarketplaceField label={t('自定义模型名称')}>
              <Input
                value={customModel}
                placeholder={t('输入自定义模型名称')}
                onChange={(value) => setCustomModel(value)}
                suffix={
                  <Button size='small' type='primary' onClick={addCustomModels}>
                    {t('填入')}
                  </Button>
                }
              />
            </MarketplaceField>
          </Col>
          <Col xs={24} md={8}>
            <MarketplaceField label={t('额度条件')}>
              <QuotaCompoundControl
                mode={form.quota_mode}
                limit={form.quota_limit}
                onModeChange={(value) =>
                  patch({
                    quota_mode: value,
                    quota_limit: value === 'unlimited' ? '' : form.quota_limit,
                  })
                }
                onLimitChange={(value) => patch({ quota_limit: value })}
              />
            </MarketplaceField>
          </Col>
          <Col xs={24} md={8}>
            <MarketplaceField label={t('时间条件')}>
              <TimeCompoundControl
                mode={form.time_mode}
                limit={form.time_limit_minutes}
                onModeChange={(value) =>
                  patch({
                    time_mode: value,
                    time_limit_minutes:
                      value === 'unlimited' ? '' : form.time_limit_minutes,
                  })
                }
                onLimitChange={(value) => patch({ time_limit_minutes: value })}
              />
            </MarketplaceField>
          </Col>
          <Col xs={24} md={8}>
            <MarketplaceField
              label={t('计费倍率')}
              help={t('买家价格 = 官方计费 x 倍率')}
            >
              <InputNumber
                value={Number(form.multiplier) || 1}
                min={0.01}
                step={0.01}
                onChange={(value) =>
                  patch({
                    multiplier: value === undefined ? '1' : String(value),
                  })
                }
                style={{ width: '100%' }}
              />
            </MarketplaceField>
          </Col>
          <Col xs={24} md={8}>
            <MarketplaceField
              label={t('并发上限')}
              help={`${t('此托管 Key 同时可承载的请求数')}，${t('0 表示不限')}`}
            >
              <InputNumber
                value={Number(form.concurrency_limit)}
                min={0}
                max={
                  maxCredentialConcurrency > 0
                    ? maxCredentialConcurrency
                    : undefined
                }
                step={1}
                onChange={(value) =>
                  patch({
                    concurrency_limit: String(
                      clampCredentialConcurrency(
                        value === undefined ? 1 : value,
                        maxCredentialConcurrency,
                      ),
                    ),
                  })
                }
                style={{ width: '100%' }}
              />
            </MarketplaceField>
          </Col>
          <Col xs={24}>
            <MarketplaceField
              label={t('计费预览')}
              help={t('选择模型和倍率后展示官方计费以及买家实际计费')}
            >
              <Table
                rowKey='model'
                size='small'
                columns={sellerPricePreviewColumns}
                dataSource={sellerPricePreview}
                pagination={false}
              />
            </MarketplaceField>
          </Col>
          <Col xs={24}>
            <Space>
              <Button type='primary' onClick={save}>
                {editing ? t('保存编辑') : t('创建托管')}
              </Button>
              {editing && (
                <Button
                  onClick={() => {
                    setEditing(null);
                    setForm(defaultCredentialForm);
                    setCustomModel('');
                  }}
                >
                  {t('取消编辑')}
                </Button>
              )}
            </Space>
          </Col>
        </Row>
      </Card>
      <Table
        rowKey='id'
        className='marketplace-seller-credentials-table'
        size='middle'
        tableLayout='fixed'
        columns={[
          { title: 'ID', dataIndex: 'id', width: 56 },
          {
            title: t('厂商'),
            width: 96,
            render: (_, record) =>
              getVendorName(record.vendor_type, record.vendor_name_snapshot),
          },
          {
            title: t('模型'),
            width: 128,
            render: (_, record) => renderMarketplaceSellerModels(record.models),
          },
          {
            title: t('额度'),
            width: 180,
            render: (_, record) => renderSellerQuotaUsage(record, t),
          },
          {
            title: t('时间'),
            width: 104,
            render: (_, record) => formatTimeCondition(record),
          },
          { title: t('倍率'), dataIndex: 'multiplier', width: 72 },
          {
            title: t('并发'),
            width: 92,
            render: (_, record) => formatCurrentConcurrency(record, t),
          },
          {
            title: t('状态'),
            width: 136,
            render: (_, record) => renderMarketplaceSellerStatus(record, t),
          },
          {
            title: t('探针评分'),
            width: 128,
            render: (_, record) => renderMarketplaceProbeScore(record, t),
          },
          {
            title: t('响应时间'),
            dataIndex: 'response_time',
            width: 110,
            render: (text, record) => (
              <div>
                {renderMarketplaceResponseTime(text, t, record.health_status)}
              </div>
            ),
          },
          {
            title: t('操作'),
            width: 64,
            render: (_, record) => {
              const marketplaceCredentialMoreMenu = [
                {
                  node: 'item',
                  name: t('编辑'),
                  onClick: () => edit(record),
                },
                {
                  node: 'item',
                  name:
                    record.listing_status === 'listed' ? t('下架') : t('上架'),
                  onClick: () =>
                    callAction(
                      record,
                      record.listing_status === 'listed' ? 'unlist' : 'list',
                    ),
                },
                {
                  node: 'item',
                  name:
                    record.service_status === 'enabled' ? t('禁用') : t('启用'),
                  onClick: () =>
                    callAction(
                      record,
                      record.service_status === 'enabled'
                        ? 'disable'
                        : 'enable',
                    ),
                },
                {
                  node: 'item',
                  name: marketplaceProbeInProgress(record)
                    ? t('检测中')
                    : t('检测指定模型'),
                  disabled: marketplaceProbeInProgress(record),
                  onClick: () =>
                    openMarketplaceModelActionModal(record, 'probe'),
                },
                {
                  node: 'item',
                  name: t('测试指定模型'),
                  onClick: () =>
                    openMarketplaceModelActionModal(record, 'test'),
                },
                {
                  node: 'item',
                  name: t('删除'),
                  type: 'danger',
                  onClick: () => deleteMarketplaceCredential(record),
                },
              ];

              return (
                <div className='marketplace-seller-actions'>
                  <Dropdown
                    trigger='click'
                    position='topRight'
                    contentClassName='marketplace-seller-actions-dropdown'
                    menu={marketplaceCredentialMoreMenu}
                  >
                    <Button size='small' type='tertiary' icon={<IconMore />} />
                  </Dropdown>
                </div>
              );
            },
          },
        ]}
        dataSource={items}
        loading={loading}
        pagination={false}
      />
      <MarketplaceCredentialModelTestModal
        showModelTestModal={showModelTestModal}
        currentTestChannel={currentTestChannel}
        handleCloseModal={handleCloseModal}
        isBatchTesting={isBatchTesting}
        batchTestModels={batchRunMarketplaceCredentialModels}
        modelSearchKeyword={modelSearchKeyword}
        setModelSearchKeyword={setModelSearchKeyword}
        selectedModelKeys={selectedModelKeys}
        setSelectedModelKeys={setSelectedModelKeys}
        modelTestResults={modelTestResults}
        testingModels={testingModels}
        testChannel={runMarketplaceCredentialModelAction}
        modelTablePage={modelTablePage}
        setModelTablePage={setModelTablePage}
        selectedEndpointType={selectedEndpointType}
        setSelectedEndpointType={setSelectedEndpointType}
        isStreamTest={isStreamTest}
        setIsStreamTest={setIsStreamTest}
        allSelectingRef={allSelectingRef}
        isMobile={isMobile}
        mode={modelActionMode}
        t={t}
      />
    </Space>
  );
}

export default function Marketplace() {
  const { t } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();
  const requestedMarketplaceTab = normalizeMarketplaceTab(
    searchParams.get('tab'),
  );
  const requestedBuyerTokenId = Number(searchParams.get('token_id')) || 0;
  const [activeMarketplaceTab, setActiveMarketplaceTab] = useState(
    requestedMarketplaceTab,
  );
  const [buyerTokens, setBuyerTokens] = useState([]);
  const [selectedBuyerTokenId, setSelectedBuyerTokenId] = useState();

  useEffect(() => {
    setActiveMarketplaceTab(requestedMarketplaceTab);
  }, [requestedMarketplaceTab]);

  useEffect(() => {
    let mounted = true;
    const loadBuyerTokens = async () => {
      try {
        const response = await API.get('/api/token/', {
          params: { p: 1, size: 100 },
        });
        ensureSuccess(response);
        const tokens = response?.data?.data?.items || [];
        if (!mounted) return;
        setBuyerTokens(tokens);
      } catch (error) {
        showError(error.message);
      }
    };
    loadBuyerTokens();
    return () => {
      mounted = false;
    };
  }, []);

  useEffect(() => {
    if (buyerTokens.length === 0) return;
    setSelectedBuyerTokenId((current) => {
      const requestedToken = buyerTokens.find(
        (token) => token.status === 1 && token.id === requestedBuyerTokenId,
      );
      if (requestedToken) return requestedToken.id;

      const currentToken = buyerTokens.find(
        (token) => token.status === 1 && token.id === current,
      );
      if (currentToken) return current;

      return buyerTokens.find((token) => token.status === 1)?.id;
    });
  }, [buyerTokens, requestedBuyerTokenId]);

  const handleMarketplaceTabChange = (tab) => {
    const nextTab = normalizeMarketplaceTab(tab);
    setActiveMarketplaceTab(nextTab);
    const params = new URLSearchParams(searchParams);
    params.set('tab', nextTab);
    if (nextTab === 'pool' && selectedBuyerTokenId) {
      params.set('token_id', String(selectedBuyerTokenId));
    } else {
      params.delete('token_id');
    }
    setSearchParams(params, { replace: true });
  };

  const handleBuyerTokenChange = (tokenId) => {
    setSelectedBuyerTokenId(tokenId);
    const params = new URLSearchParams(searchParams);
    params.set('tab', 'pool');
    if (tokenId) {
      params.set('token_id', String(tokenId));
    } else {
      params.delete('token_id');
    }
    setSearchParams(params, { replace: true });
  };

  return (
    <div className='marketplace-page mt-[60px] px-3 pb-6'>
      <Space vertical style={{ width: '100%' }}>
        <div className='marketplace-page-header'>
          <div className='marketplace-page-heading'>
            <Text className='marketplace-page-eyebrow' type='tertiary'>
              {t('AI 供给市场')}
            </Text>
            <Title heading={3} className='marketplace-page-title'>
              {t('市场')}
            </Title>
            <Text className='marketplace-page-summary' type='secondary'>
              {t('市场把可用 AI API Key 变成可购买、可路由的调用能力。')}
            </Text>
            <div
              className='marketplace-page-guide'
              aria-label={t('市场工作流')}
            >
              <div className='marketplace-page-guide-item marketplace-page-guide-source'>
                <span
                  className='marketplace-page-guide-icon'
                  aria-hidden='true'
                >
                  <IconKey />
                </span>
                <span className='marketplace-page-guide-copy'>
                  <span className='marketplace-page-guide-label'>
                    {t('卖家上架')}
                  </span>
                  <span className='marketplace-page-guide-text'>
                    {t('托管 Key，设置模型、价格和并发')}
                  </span>
                </span>
              </div>
              <div className='marketplace-page-guide-branch'>
                <span
                  className='marketplace-page-guide-arrow'
                  aria-hidden='true'
                >
                  <IconArrowRight />
                </span>
                <span className='marketplace-page-guide-branch-label'>
                  {t('可直接用于')}
                </span>
              </div>
              <div className='marketplace-page-guide-paths'>
                <div className='marketplace-page-guide-item'>
                  <span
                    className='marketplace-page-guide-icon'
                    aria-hidden='true'
                  >
                    <IconRoute />
                  </span>
                  <span className='marketplace-page-guide-copy'>
                    <span className='marketplace-page-guide-label'>
                      {t('订单池调用')}
                    </span>
                    <span className='marketplace-page-guide-text'>
                      {t('按模型、价格和并发自动选择供给')}
                    </span>
                  </span>
                </div>
                <div className='marketplace-page-guide-item'>
                  <span
                    className='marketplace-page-guide-icon'
                    aria-hidden='true'
                  >
                    <IconCart />
                  </span>
                  <span className='marketplace-page-guide-copy'>
                    <span className='marketplace-page-guide-label'>
                      {t('买家购买')}
                    </span>
                    <span className='marketplace-page-guide-text'>
                      {t('买断额度，绑定令牌固定路由')}
                    </span>
                  </span>
                </div>
              </div>
            </div>
          </div>
          <div className='marketplace-page-actions'>
            <ManageTokenButton />
          </div>
        </div>
        <Tabs
          type='line'
          keepDOM={false}
          activeKey={activeMarketplaceTab}
          onChange={handleMarketplaceTabChange}
        >
          <Tabs.TabPane tab={t('订单池')} itemKey='pool'>
            <PoolTab
              buyerTokens={buyerTokens}
              selectedBuyerTokenId={selectedBuyerTokenId}
              onBuyerTokenChange={handleBuyerTokenChange}
              onBuyerTokensChange={setBuyerTokens}
            />
          </Tabs.TabPane>
          <Tabs.TabPane tab={t('订单列表')} itemKey='orders'>
            <OrdersTab />
          </Tabs.TabPane>
          <Tabs.TabPane tab={t('我的买断订单')} itemKey='fixed'>
            <FixedOrdersTab
              buyerTokens={buyerTokens}
              onBuyerTokensChange={setBuyerTokens}
            />
          </Tabs.TabPane>
          <Tabs.TabPane tab={t('卖家托管')} itemKey='seller'>
            <SellerTab />
          </Tabs.TabPane>
        </Tabs>
      </Space>
    </div>
  );
}
