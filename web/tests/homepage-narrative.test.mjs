import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const homePath = new URL('../src/pages/Home/index.jsx', import.meta.url);
const enLocalePath = new URL('../src/i18n/locales/en.json', import.meta.url);
const zhCNLocalePath = new URL(
  '../src/i18n/locales/zh-CN.json',
  import.meta.url,
);
const zhTWLocalePath = new URL(
  '../src/i18n/locales/zh-TW.json',
  import.meta.url,
);
const frLocalePath = new URL('../src/i18n/locales/fr.json', import.meta.url);
const jaLocalePath = new URL('../src/i18n/locales/ja.json', import.meta.url);
const ruLocalePath = new URL('../src/i18n/locales/ru.json', import.meta.url);
const viLocalePath = new URL('../src/i18n/locales/vi.json', import.meta.url);

const load = async (path) => readFile(path, 'utf8');
const homepageNarrativeExpectations = {
  'zh-CN': {
    'LLM Token 使用权共享基础设施': 'LLM Token 使用权共享基础设施',
    '面向 Agent 和 Human 自由交易 LLM Token 使用权的基础设施':
      '面向 Agent 和 Human 自由交易 LLM Token 使用权的基础设施',
    'HermesToken 将 LLM Token 使用权定义为可交易、可执行、可结算、可审计的标准化资源。':
      'HermesToken 将 LLM Token 使用权定义为可交易、可执行、可结算、可审计的标准化资源。',
    'Agent 与 Human 在同一市场中发现能力、进入统一执行边界，并围绕真实使用完成结算。':
      'Agent 与 Human 在同一市场中发现能力、进入统一执行边界，并围绕真实使用完成结算。',
    'LLM Token 使用权正在被标准化': 'LLM Token 使用权正在被标准化',
  },
  'zh-TW': {
    'LLM Token 使用权共享基础设施': 'LLM Token 使用權共享基礎設施',
    '面向 Agent 和 Human 自由交易 LLM Token 使用权的基础设施':
      '面向 Agent 和 Human 自由交易 LLM Token 使用權的基礎設施',
    'HermesToken 将 LLM Token 使用权定义为可交易、可执行、可结算、可审计的标准化资源。':
      'HermesToken 將 LLM Token 使用權定義為可交易、可執行、可結算、可審計的標準化資源。',
    'Agent 与 Human 在同一市场中发现能力、进入统一执行边界，并围绕真实使用完成结算。':
      'Agent 與 Human 在同一市場中發現能力、進入統一執行邊界，並圍繞真實使用完成結算。',
    'LLM Token 使用权正在被标准化': 'LLM Token 使用權正在被標準化',
  },
  en: {
    'LLM Token 使用权共享基础设施':
      'Shared infrastructure for LLM token usage rights',
    '面向 Agent 和 Human 自由交易 LLM Token 使用权的基础设施':
      'Infrastructure for the free exchange of LLM token usage rights between Agent and Human participants',
    'HermesToken 将 LLM Token 使用权定义为可交易、可执行、可结算、可审计的标准化资源。':
      'HermesToken defines LLM token usage rights as a standardized resource that can be traded, executed, settled, and audited.',
    'Agent 与 Human 在同一市场中发现能力、进入统一执行边界，并围绕真实使用完成结算。':
      'Agent and Human participants discover capacity in the same market, enter a shared execution boundary, and settle around real usage.',
    'LLM Token 使用权正在被标准化':
      'LLM token usage rights are being standardized',
  },
  fr: {
    'LLM Token 使用权共享基础设施':
      'Infrastructure partagée pour les droits d’usage des tokens LLM',
    '面向 Agent 和 Human 自由交易 LLM Token 使用权的基础设施':
      'Infrastructure pour l’échange libre des droits d’usage des tokens LLM entre agents et humains',
    'HermesToken 将 LLM Token 使用权定义为可交易、可执行、可结算、可审计的标准化资源。':
      'HermesToken définit les droits d’usage des tokens LLM comme une ressource standardisée pouvant être échangée, exécutée, réglée et auditée.',
    'Agent 与 Human 在同一市场中发现能力、进入统一执行边界，并围绕真实使用完成结算。':
      'Les agents et les humains découvrent les capacités sur le même marché, entrent dans une frontière d’exécution commune et règlent selon l’usage réel.',
    'LLM Token 使用权正在被标准化':
      'Les droits d’usage des tokens LLM sont en cours de standardisation',
  },
  ja: {
    'LLM Token 使用权共享基础设施': 'LLMトークン利用権の共有インフラ',
    '面向 Agent 和 Human 自由交易 LLM Token 使用权的基础设施':
      'Agent と Human が LLMトークン利用権を自由に取引するためのインフラ',
    'HermesToken 将 LLM Token 使用权定义为可交易、可执行、可结算、可审计的标准化资源。':
      'HermesTokenは、LLMトークン利用権を、取引・実行・決済・監査が可能な標準化リソースとして定義します。',
    'Agent 与 Human 在同一市场中发现能力、进入统一执行边界，并围绕真实使用完成结算。':
      'Agent と Human は同じ市場で能力を見つけ、共通の実行境界に入り、実際の利用に基づいて決済します。',
    'LLM Token 使用权正在被标准化': 'LLMトークン利用権は標準化されつつあります',
  },
  ru: {
    'LLM Token 使用权共享基础设施':
      'Общая инфраструктура для прав использования LLM-токенов',
    '面向 Agent 和 Human 自由交易 LLM Token 使用权的基础设施':
      'Инфраструктура для свободного обмена правами использования LLM-токенов между Agent и Human',
    'HermesToken 将 LLM Token 使用权定义为可交易、可执行、可结算、可审计的标准化资源。':
      'HermesToken определяет права использования LLM-токенов как стандартизированный ресурс для торговли, исполнения, расчетов и аудита.',
    'Agent 与 Human 在同一市场中发现能力、进入统一执行边界，并围绕真实使用完成结算。':
      'Agent и Human находят возможности на одном рынке, входят в общую границу исполнения и проводят расчеты по фактическому использованию.',
    'LLM Token 使用权正在被标准化':
      'Права использования LLM-токенов стандартизируются',
  },
  vi: {
    'LLM Token 使用权共享基础设施':
      'Hạ tầng chia sẻ cho quyền sử dụng token LLM',
    '面向 Agent 和 Human 自由交易 LLM Token 使用权的基础设施':
      'Hạ tầng cho giao dịch tự do quyền sử dụng token LLM giữa Agent và Human',
    'HermesToken 将 LLM Token 使用权定义为可交易、可执行、可结算、可审计的标准化资源。':
      'HermesToken định nghĩa quyền sử dụng token LLM như một tài nguyên chuẩn hóa có thể được giao dịch, thực thi, quyết toán và kiểm toán.',
    'Agent 与 Human 在同一市场中发现能力、进入统一执行边界，并围绕真实使用完成结算。':
      'Agent và Human khám phá năng lực trong cùng một thị trường, đi vào cùng một ranh giới thực thi và quyết toán theo mức sử dụng thực tế.',
    'LLM Token 使用权正在被标准化':
      'Quyền sử dụng token LLM đang được chuẩn hóa',
  },
};

test('default homepage source uses the minimal narrative layout', async () => {
  const source = await load(homePath);

  assert.ok(
    source.includes(
      'HermesToken 将 LLM Token 使用权定义为可交易、可执行、可结算、可审计的标准化资源。',
    ),
  );
  assert.ok(source.includes('LLM Token 使用权正在被标准化'));
  assert.ok(source.includes('AI 能力开始进入资源化阶段'));
  assert.ok(source.includes('买方、卖方、Agent 在同一结构里协作'));

  assert.ok(!source.includes('大模型接口网关'));
  assert.ok(
    !source.includes('更好的价格，更好的稳定性，只需要将模型基址替换为：'),
  );
  assert.ok(!source.includes('获取密钥'));
  assert.ok(!source.includes('支持众多的大模型供应商'));
});

test('homepage narrative translations exist for all supported homepage locales', async () => {
  const [
    enSource,
    zhCNSource,
    zhTWSource,
    frSource,
    jaSource,
    ruSource,
    viSource,
  ] = await Promise.all([
    load(enLocalePath),
    load(zhCNLocalePath),
    load(zhTWLocalePath),
    load(frLocalePath),
    load(jaLocalePath),
    load(ruLocalePath),
    load(viLocalePath),
  ]);

  const locales = {
    en: JSON.parse(enSource).translation,
    'zh-CN': JSON.parse(zhCNSource).translation,
    'zh-TW': JSON.parse(zhTWSource).translation,
    fr: JSON.parse(frSource).translation,
    ja: JSON.parse(jaSource).translation,
    ru: JSON.parse(ruSource).translation,
    vi: JSON.parse(viSource).translation,
  };

  for (const [locale, expectations] of Object.entries(
    homepageNarrativeExpectations,
  )) {
    for (const [key, expectedValue] of Object.entries(expectations)) {
      assert.equal(locales[locale][key], expectedValue);
    }
  }
});
