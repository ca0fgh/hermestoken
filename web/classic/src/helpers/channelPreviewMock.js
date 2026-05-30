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

const CHANNEL_PREVIEW_STORAGE_KEY = 'channel-preview-mock';
const CHANNEL_PREVIEW_QUERY_KEY = 'channelPreview';
const PREVIEW_GROUPS = [
  'default',
  'vip',
  'qa',
  'free',
  'enterprise',
  'sandbox',
];
const PREVIEW_MODELS = [
  'gpt-4o-mini',
  'gpt-4.1',
  'claude-3-5-sonnet-latest',
  'deepseek-chat',
  'gemini-2.0-flash',
  'o3-mini',
  'qwen-max',
  'mistral-large-latest',
  'grok-3-mini',
  'command-r-plus',
];

const previewUser = {
  id: 1,
  username: 'preview-admin',
  display_name: 'Preview Admin',
  role: 10,
  status: 1,
  quota: 500000,
  used_quota: 12000,
  request_count: 42,
  group: 'default',
  setting: JSON.stringify({ language: 'zh-CN' }),
  permissions: {
    sidebar_settings: true,
    sidebar_modules: {
      chat: { enabled: true, playground: true, chat: true },
      console: {
        enabled: true,
        detail: true,
        token: true,
        log: true,
        midjourney: true,
        task: true,
      },
      personal: { enabled: true, topup: true, personal: true },
      invite: { enabled: true, rebate: true },
      admin: {
        enabled: true,
        channel: true,
        models: true,
        withdrawal: true,
        deployment: true,
        redemption: true,
        user: true,
        subscription: true,
        setting: true,
      },
    },
  },
};

const previewStatus = {
  setup: true,
  version: 'preview',
  system_name: 'HermesToken Preview',
  logo: '',
  footer_html: '',
  docs_link: '',
  server_address: '',
  price: 1,
  self_use_mode_enabled: false,
  demo_site_enabled: false,
  HeaderNavModules: JSON.stringify({
    chat: true,
    marketplace: true,
    pricing: { enabled: true, requireAuth: false },
    verification: true,
  }),
  SidebarModulesAdmin: JSON.stringify({
    admin: {
      enabled: true,
      channel: true,
      models: true,
      setting: true,
      user: true,
      token: true,
    },
  }),
};

function getPreviewTimestamp(offsetSeconds) {
  return Math.floor(Date.now() / 1000) - offsetSeconds;
}

function buildSettings(overrides = {}) {
  return JSON.stringify({
    upstream_update: {
      enabled: false,
      pending_add_models: [],
      pending_remove_models: [],
      ...(overrides.upstream_update || {}),
    },
    ...Object.fromEntries(
      Object.entries(overrides).filter(([key]) => key !== 'upstream_update'),
    ),
  });
}

function makePreviewChannel(overrides) {
  const channel = {
    id: 0,
    name: 'Preview Channel',
    type: 1,
    status: 1,
    group: 'default',
    priority: 1,
    weight: 50,
    tag: '默认',
    models: 'gpt-4o-mini',
    model_mapping: '{}',
    used_quota: 10000,
    balance: 100000,
    response_time: 0,
    test_time: getPreviewTimestamp(3600),
    created_time: getPreviewTimestamp(86400),
    other_info: '',
    channel_info: {},
    setting: JSON.stringify({ pass_through_body_enabled: false }),
    settings: buildSettings(),
    remark: '本地预览合成数据：用于验证渠道列表交互。',
    ...overrides,
  };

  channel.key = String(channel.id);
  return channel;
}

const initialChannels = [
  makePreviewChannel({
    id: 101,
    name: 'OpenAI 主渠道',
    type: 1,
    status: 1,
    group: 'default,vip',
    priority: 10,
    weight: 100,
    tag: '生产',
    models: 'gpt-4o-mini,gpt-4.1',
    model_mapping: '{}',
    used_quota: 125000,
    balance: 988000,
    response_time: 760,
    test_time: getPreviewTimestamp(3600),
    created_time: getPreviewTimestamp(86400),
    settings: buildSettings({
      upstream_update: {
        enabled: true,
        pending_add_models: ['gpt-4.1'],
        pending_remove_models: [],
      },
    }),
    remark: '本地预览合成数据：用于验证右侧操作按钮点击区域。',
  }),
  makePreviewChannel({
    id: 102,
    name: 'Claude 备用渠道',
    type: 14,
    status: 2,
    group: 'default',
    priority: 5,
    weight: 60,
    tag: '备用',
    models: 'claude-3-5-sonnet-latest',
    model_mapping: '{}',
    used_quota: 38000,
    balance: 352000,
    response_time: 1280,
    test_time: getPreviewTimestamp(7200),
    created_time: getPreviewTimestamp(43200),
    remark: '禁用状态示例，用来验证启用按钮。',
  }),
  makePreviewChannel({
    id: 103,
    name: 'OpenRouter 多密钥',
    type: 20,
    status: 1,
    group: 'qa',
    priority: 8,
    weight: 80,
    tag: '实验',
    models: 'deepseek-chat,gemini-2.0-flash',
    model_mapping: '{}',
    used_quota: 82000,
    balance: 715000,
    response_time: 430,
    test_time: getPreviewTimestamp(1800),
    created_time: getPreviewTimestamp(21600),
    channel_info: {
      is_multi_key: true,
      multi_key_mode: 'random',
      multi_key_size: 3,
      multi_key_status_list: {},
    },
    setting: JSON.stringify({ pass_through_body_enabled: true }),
    settings: buildSettings({ upstream_update: { enabled: true } }),
    remark: '多密钥渠道示例，用来验证编辑下拉按钮。',
  }),
  makePreviewChannel({
    id: 104,
    name: 'Gemini 快速渠道',
    type: 24,
    status: 1,
    group: 'default,free',
    priority: 12,
    weight: 90,
    tag: '生产',
    models: 'gemini-2.0-flash',
    used_quota: 21000,
    balance: 445000,
    response_time: 620,
    remark: '响应较快的 Google Gemini 渠道示例。',
  }),
  makePreviewChannel({
    id: 105,
    name: 'DeepSeek 高并发',
    type: 43,
    status: 1,
    group: 'vip,enterprise',
    priority: 20,
    weight: 120,
    tag: '高并发',
    models: 'deepseek-chat',
    used_quota: 310000,
    balance: 1660000,
    response_time: 980,
    remark: '用于验证长标签、较高权重和较大额度显示。',
  }),
  makePreviewChannel({
    id: 106,
    name: 'Azure EastUS',
    type: 3,
    status: 3,
    group: 'enterprise',
    priority: 3,
    weight: 30,
    tag: '异常',
    models: 'gpt-4o-mini',
    used_quota: 64000,
    balance: 120000,
    response_time: 5400,
    other_info: JSON.stringify({
      status_reason: '连续探活失败',
      status_time: getPreviewTimestamp(900),
    }),
    remark: '异常禁用状态示例，用来验证状态原因 tooltip。',
  }),
  makePreviewChannel({
    id: 107,
    name: 'Ollama 本地节点',
    type: 4,
    status: 1,
    group: 'sandbox',
    priority: 1,
    weight: 40,
    tag: '本地',
    models: 'llama3.1,qwen2.5',
    used_quota: 4200,
    balance: 0,
    response_time: 220,
    remark: 'Ollama 类型会在更多菜单里出现测活入口。',
  }),
  makePreviewChannel({
    id: 108,
    name: '通义千问备用',
    type: 17,
    status: 2,
    group: 'free,qa',
    priority: -1,
    weight: 10,
    tag: '备用',
    models: 'qwen-max',
    used_quota: 18000,
    balance: 76000,
    response_time: 0,
    remark: '未测试响应时间和负优先级示例。',
  }),
  makePreviewChannel({
    id: 109,
    name: 'SiliconCloud 低价池',
    type: 40,
    status: 1,
    group: 'free',
    priority: 6,
    weight: 55,
    tag: '低价',
    models: 'deepseek-chat,qwen-max',
    used_quota: 9900,
    balance: 250000,
    response_time: 1880,
    remark: '低价池渠道示例。',
  }),
  makePreviewChannel({
    id: 110,
    name: 'xAI 实验渠道',
    type: 48,
    status: 1,
    group: 'sandbox,qa',
    priority: 2,
    weight: 25,
    tag: '实验',
    models: 'grok-3-mini',
    used_quota: 5700,
    balance: 88000,
    response_time: 2450,
    remark: '实验标签下的 xAI 渠道示例。',
  }),
  makePreviewChannel({
    id: 111,
    name: 'Codex OAuth 帐号',
    type: 57,
    status: 1,
    group: 'enterprise',
    priority: 9,
    weight: 65,
    tag: '账号',
    models: 'codex-mini-latest',
    used_quota: 46000,
    balance: 0,
    response_time: 710,
    remark: 'Codex 类型会在余额列显示帐号信息入口。',
  }),
  makePreviewChannel({
    id: 112,
    name: 'AWS Claude 多区',
    type: 33,
    status: 1,
    group: 'enterprise,vip',
    priority: 11,
    weight: 85,
    tag: '生产',
    models: 'claude-3-5-sonnet-latest',
    used_quota: 178000,
    balance: 790000,
    response_time: 1620,
    channel_info: {
      is_multi_key: true,
      multi_key_mode: 'polling',
      multi_key_size: 5,
      multi_key_status_list: { 2: true },
    },
    remark: '轮询多密钥，含 1 个禁用子密钥。',
  }),
  makePreviewChannel({
    id: 113,
    name: 'Vertex AI 亚太',
    type: 41,
    status: 2,
    group: 'enterprise',
    priority: 4,
    weight: 20,
    tag: '维护',
    models: 'gemini-2.0-flash',
    used_quota: 27000,
    balance: 610000,
    response_time: 3120,
    remark: '维护中的 Vertex AI 渠道示例。',
  }),
  makePreviewChannel({
    id: 114,
    name: '自定义 OpenAI 兼容',
    type: 8,
    status: 1,
    group: 'qa,sandbox',
    priority: 7,
    weight: 45,
    tag: '兼容',
    models: 'gpt-4o-mini,deepseek-chat',
    model_mapping: JSON.stringify({ 'gpt-4o-mini': 'provider-fast-model' }),
    used_quota: 7300,
    balance: 190000,
    response_time: 890,
    setting: JSON.stringify({ pass_through_body_enabled: true }),
    remark: '自定义兼容渠道，开启请求透传标记。',
  }),
  makePreviewChannel({
    id: 115,
    name: 'Cohere Rerank 渠道',
    type: 34,
    status: 1,
    group: 'qa',
    priority: 0,
    weight: 35,
    tag: '工具',
    models: 'command-r-plus',
    used_quota: 12600,
    balance: 225000,
    response_time: 1340,
    remark: 'Cohere 类型示例。',
  }),
  makePreviewChannel({
    id: 116,
    name: 'Perplexity 搜索渠道',
    type: 27,
    status: 1,
    group: 'default,qa',
    priority: 13,
    weight: 70,
    tag: '搜索',
    models: 'sonar-pro',
    used_quota: 41200,
    balance: 340000,
    response_time: 2190,
    remark: '搜索增强类渠道示例。',
  }),
  makePreviewChannel({
    id: 117,
    name: 'Mistral EU',
    type: 42,
    status: 2,
    group: 'free',
    priority: 2,
    weight: 15,
    tag: '备用',
    models: 'mistral-large-latest',
    used_quota: 5800,
    balance: 130000,
    response_time: 0,
    remark: '禁用备用 Mistral 渠道。',
  }),
  makePreviewChannel({
    id: 118,
    name: 'Xinference 内网模型',
    type: 47,
    status: 1,
    group: 'sandbox',
    priority: 5,
    weight: 50,
    tag: '本地',
    models: 'qwen2.5-coder',
    used_quota: 2600,
    balance: 0,
    response_time: 350,
    remark: '内网推理渠道示例。',
  }),
];

let previewChannels = cloneValue(initialChannels);

function cloneValue(value) {
  if (value === undefined) {
    return value;
  }

  if (typeof structuredClone === 'function') {
    return structuredClone(value);
  }

  return JSON.parse(JSON.stringify(value));
}

function isBrowserRuntime() {
  return typeof window !== 'undefined' && typeof localStorage !== 'undefined';
}

function updatePreviewModeFromQuery() {
  if (!isBrowserRuntime()) {
    return;
  }

  const params = new URLSearchParams(window.location.search);
  if (!params.has(CHANNEL_PREVIEW_QUERY_KEY)) {
    return;
  }

  const value = params.get(CHANNEL_PREVIEW_QUERY_KEY);
  if (value === '1' || value === 'true') {
    localStorage.setItem('channel-preview-mock', '1');
    return;
  }

  if (value === '0' || value === 'false') {
    localStorage.removeItem(CHANNEL_PREVIEW_STORAGE_KEY);
  }
}

export function isChannelPreviewMockEnabled() {
  if (!import.meta.env.DEV || !isBrowserRuntime()) {
    return false;
  }

  updatePreviewModeFromQuery();
  return localStorage.getItem(CHANNEL_PREVIEW_STORAGE_KEY) === '1';
}

function ensurePreviewAdminUser() {
  if (!isChannelPreviewMockEnabled()) {
    return;
  }

  const rawUser = localStorage.getItem('user');
  if (rawUser) {
    try {
      const user = JSON.parse(rawUser);
      if (user && typeof user.role === 'number' && user.role >= 10) {
        return;
      }
    } catch {
      // Replace invalid local user state with preview admin state.
    }
  }

  localStorage.setItem('user', JSON.stringify(previewUser));
}

function createAxiosResponse(data, config = {}) {
  return {
    data: cloneValue(data),
    status: 200,
    statusText: 'OK',
    headers: {},
    config,
  };
}

function createFetchResponse(data) {
  return new Response(JSON.stringify(data), {
    status: 200,
    headers: {
      'Content-Type': 'application/json',
      'Cache-Control': 'no-store',
    },
  });
}

function normalizePath(url) {
  const parsed = new URL(url, window.location.origin);
  return {
    pathname: parsed.pathname,
    searchParams: parsed.searchParams,
  };
}

function getChannelTypeCounts(channels) {
  return channels.reduce((counts, channel) => {
    const typeKey = String(channel.type);
    return {
      ...counts,
      [typeKey]: (counts[typeKey] || 0) + 1,
    };
  }, {});
}

function readPositiveInteger(value, fallback) {
  const parsed = Number.parseInt(value, 10);
  if (!Number.isFinite(parsed) || parsed < 1) {
    return fallback;
  }
  return parsed;
}

function filterChannels(searchParams) {
  const type = searchParams.get('type');
  const status = searchParams.get('status');
  const keyword = searchParams.get('keyword') || '';
  const group = searchParams.get('group') || '';
  const model = searchParams.get('model') || '';

  return previewChannels.filter((channel) => {
    if (type && String(channel.type) !== type) {
      return false;
    }
    if (status && String(channel.status) !== status) {
      return false;
    }
    if (
      keyword &&
      !channel.name.toLowerCase().includes(keyword.toLowerCase())
    ) {
      return false;
    }
    if (group && !channel.group.split(',').includes(group)) {
      return false;
    }
    if (model && !channel.models.split(',').includes(model)) {
      return false;
    }
    return true;
  });
}

function getChannelListPayload(searchParams) {
  const filtered = filterChannels(searchParams);
  const page = readPositiveInteger(searchParams.get('p'), 1);
  const pageSize = readPositiveInteger(searchParams.get('page_size'), 10);
  const offset = (page - 1) * pageSize;
  const items = filtered.slice(offset, offset + pageSize);

  return {
    success: true,
    data: {
      items: cloneValue(items),
      total: filtered.length,
      type_counts: getChannelTypeCounts(filtered),
    },
  };
}

function getPreviewOptionsPayload() {
  return {
    success: true,
    data: [
      {
        key: 'global.pass_through_request_enabled',
        value: 'false',
      },
    ],
  };
}

function getPreviewUserPayload() {
  return {
    success: true,
    data: {
      ...previewUser,
      sidebar_modules: {
        admin: {
          enabled: true,
          channel: true,
          models: true,
          setting: true,
          user: true,
        },
      },
    },
  };
}

function getPreviewStatusPayload() {
  return {
    success: true,
    data: previewStatus,
  };
}

function getChannelById(channelId) {
  return previewChannels.find((channel) => String(channel.id) === channelId);
}

function updateChannel(payload = {}) {
  const channelId = String(payload.id);
  const existing = getChannelById(channelId);
  if (!existing) {
    return {
      success: false,
      message: 'channel not found',
    };
  }

  previewChannels = previewChannels.map((channel) => {
    if (String(channel.id) !== channelId) {
      return channel;
    }

    return {
      ...channel,
      ...payload,
      channel_info: payload.channel_info || channel.channel_info,
    };
  });

  return {
    success: true,
    data: getChannelById(channelId),
  };
}

function deleteChannel(channelId) {
  const beforeCount = previewChannels.length;
  previewChannels = previewChannels.filter(
    (channel) => String(channel.id) !== channelId,
  );

  return {
    success: true,
    data: beforeCount - previewChannels.length,
  };
}

function copyChannel(channelId) {
  const existing = getChannelById(channelId);
  if (!existing) {
    return {
      success: false,
      message: 'channel not found',
    };
  }

  const nextId = Math.max(...previewChannels.map((channel) => channel.id)) + 1;
  const copied = {
    ...cloneValue(existing),
    id: nextId,
    name: `${existing.name} Copy`,
    key: String(nextId),
  };
  previewChannels = [copied, ...previewChannels];

  return {
    success: true,
    data: copied,
  };
}

function setTagStatus(tag, status) {
  previewChannels = previewChannels.map((channel) => {
    if (channel.tag !== tag) {
      return channel;
    }

    return {
      ...channel,
      status,
    };
  });

  return {
    success: true,
    data: previewChannels.filter((channel) => channel.tag === tag).length,
  };
}

function getPreviewPayload(method, url, body) {
  if (!isChannelPreviewMockEnabled() || !url || typeof url !== 'string') {
    return undefined;
  }

  const { pathname, searchParams } = normalizePath(url);
  const normalizedMethod = method.toLowerCase();

  if (normalizedMethod === 'get' && pathname === '/api/status') {
    return getPreviewStatusPayload();
  }

  if (normalizedMethod === 'get' && pathname === '/api/option/') {
    return getPreviewOptionsPayload();
  }

  if (normalizedMethod === 'get' && pathname === '/api/user/self') {
    return getPreviewUserPayload();
  }

  if (normalizedMethod === 'get' && pathname === '/api/data/') {
    return {
      success: true,
      data: {
        quota: previewUser.quota,
        used_quota: previewUser.used_quota,
        request_count: previewUser.request_count,
        statistical_quota: 0,
        statistical_token: 0,
        rpm: 0,
        tpm: 0,
      },
    };
  }

  if (normalizedMethod === 'get' && pathname === '/api/user/self/groups') {
    return {
      success: true,
      data: PREVIEW_GROUPS,
    };
  }

  if (normalizedMethod === 'get' && pathname === '/api/user/2fa/status') {
    return {
      success: true,
      data: {
        enabled: false,
        locked: false,
        backup_codes_remaining: 0,
      },
    };
  }

  if (normalizedMethod === 'get' && pathname === '/api/user/passkey') {
    return {
      success: true,
      data: [],
    };
  }

  if (normalizedMethod === 'get' && pathname === '/api/user/logout') {
    return {
      success: true,
      message: 'preview logout skipped',
    };
  }

  if (normalizedMethod === 'get' && pathname === '/api/notice') {
    return {
      success: true,
      data: '',
    };
  }

  if (normalizedMethod === 'get' && pathname === '/api/group/') {
    return {
      success: true,
      data: PREVIEW_GROUPS,
    };
  }

  if (normalizedMethod === 'get' && pathname === '/api/models') {
    return {
      success: true,
      data: {
        1: PREVIEW_MODELS,
        14: PREVIEW_MODELS,
        20: PREVIEW_MODELS,
      },
    };
  }

  if (
    normalizedMethod === 'get' &&
    (pathname === '/api/channel/' || pathname === '/api/channel/search')
  ) {
    return getChannelListPayload(searchParams);
  }

  if (normalizedMethod === 'get' && pathname === '/api/channel/models_priced') {
    return {
      success: true,
      data: PREVIEW_MODELS.map((id) => ({ id })),
    };
  }

  if (normalizedMethod === 'get' && pathname === '/api/prefill_group') {
    return {
      success: true,
      data: PREVIEW_MODELS,
    };
  }

  if (normalizedMethod === 'get' && pathname === '/api/channel/test') {
    return {
      success: true,
      message: 'preview test queued',
    };
  }

  if (normalizedMethod === 'get' && pathname.startsWith('/api/channel/test/')) {
    return {
      success: true,
      message: 'preview channel test passed',
      time: 0.42,
    };
  }

  if (
    normalizedMethod === 'get' &&
    pathname === '/api/channel/update_balance'
  ) {
    return {
      success: true,
      message: 'preview balance updated',
    };
  }

  if (
    normalizedMethod === 'get' &&
    pathname.startsWith('/api/channel/update_balance/')
  ) {
    return {
      success: true,
      balance: 999000,
    };
  }

  if (
    normalizedMethod === 'get' &&
    pathname.startsWith('/api/channel/ollama/version/')
  ) {
    return {
      success: true,
      data: {
        version: '0.5.7-preview',
      },
    };
  }

  if (
    normalizedMethod === 'get' &&
    pathname.startsWith('/api/channel/fetch_models/')
  ) {
    return {
      success: true,
      data: PREVIEW_MODELS,
    };
  }

  if (normalizedMethod === 'get' && pathname === '/api/channel/tag/models') {
    return {
      success: true,
      data: PREVIEW_MODELS,
    };
  }

  if (normalizedMethod === 'get' && pathname.startsWith('/api/channel/')) {
    const channelId = pathname.replace('/api/channel/', '').replace('/', '');
    const channel = getChannelById(channelId);
    return channel
      ? {
          success: true,
          data: cloneValue(channel),
        }
      : {
          success: false,
          message: 'channel not found',
        };
  }

  if (normalizedMethod === 'put' && pathname === '/api/channel/') {
    return updateChannel(body);
  }

  if (
    normalizedMethod === 'delete' &&
    pathname.startsWith('/api/channel/') &&
    pathname.endsWith('/')
  ) {
    const channelId = pathname.replace('/api/channel/', '').replace('/', '');
    return deleteChannel(channelId);
  }

  if (normalizedMethod === 'delete' && pathname === '/api/channel/disabled') {
    const disabledCount = previewChannels.filter(
      (channel) => channel.status !== 1,
    ).length;
    previewChannels = previewChannels.filter((channel) => channel.status === 1);
    return {
      success: true,
      data: disabledCount,
    };
  }

  if (
    normalizedMethod === 'post' &&
    pathname.startsWith('/api/channel/copy/')
  ) {
    return copyChannel(pathname.replace('/api/channel/copy/', ''));
  }

  if (normalizedMethod === 'post' && pathname === '/api/channel/fix') {
    return {
      success: true,
      data: {
        success: previewChannels.length,
        fails: 0,
      },
    };
  }

  if (normalizedMethod === 'post' && pathname === '/api/channel/tag/enabled') {
    return setTagStatus(body?.tag, 1);
  }

  if (normalizedMethod === 'post' && pathname === '/api/channel/tag/disabled') {
    return setTagStatus(body?.tag, 2);
  }

  return undefined;
}

function installChannelPreviewFetchMock() {
  if (!isChannelPreviewMockEnabled() || window.__channelPreviewFetchMock) {
    return;
  }

  window.__channelPreviewFetchMock = true;
  window.__channelPreviewOriginalFetch = window.fetch.bind(window);
  window.fetch = async (input, init = {}) => {
    const url = typeof input === 'string' ? input : input?.url;
    const method = init?.method || 'get';
    const payload = getPreviewPayload(method, url);

    if (payload !== undefined) {
      return createFetchResponse(payload);
    }

    return window.__channelPreviewOriginalFetch(input, init);
  };
}

export function getChannelPreviewApiResponse(method, url, body, config = {}) {
  const payload = getPreviewPayload(method, url, body);
  if (payload === undefined) {
    return undefined;
  }

  return createAxiosResponse(payload, {
    ...config,
    url,
    method,
  });
}

export function installChannelPreviewApiMock(instance) {
  if (!import.meta.env.DEV || !instance || instance.__channelPreviewApiMock) {
    return instance;
  }

  instance.__channelPreviewApiMock = true;
  ['get', 'delete', 'post', 'put', 'patch'].forEach((method) => {
    if (typeof instance[method] !== 'function') {
      return;
    }

    const originalMethod = instance[method].bind(instance);
    instance[method] = (url, ...args) => {
      const body =
        method === 'get' || method === 'delete' ? undefined : args[0];
      const config =
        method === 'get' || method === 'delete' ? args[0] || {} : args[1] || {};
      const previewResponse = getChannelPreviewApiResponse(
        method,
        url,
        body,
        config,
      );

      if (previewResponse !== undefined) {
        return Promise.resolve(previewResponse);
      }

      return originalMethod(url, ...args);
    };
  });

  return instance;
}

export function installChannelPreviewMock() {
  if (!isChannelPreviewMockEnabled()) {
    return false;
  }

  ensurePreviewAdminUser();
  installChannelPreviewFetchMock();
  return true;
}
