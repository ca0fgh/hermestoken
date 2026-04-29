import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const readClassicSource = (relativePath) =>
  readFile(new URL(`../classic/src/${relativePath}`, import.meta.url), "utf8");

const readDefaultSource = (relativePath) =>
  readFile(new URL(`../default/src/${relativePath}`, import.meta.url), "utf8");

test("classic marketplace header is structured for a localized product surface", async () => {
  const [pageSource, styleSource, enLocale] = await Promise.all([
    readClassicSource("pages/Marketplace/index.jsx"),
    readClassicSource("pages/Marketplace/index.css"),
    readClassicSource("i18n/locales/en.json"),
  ]);

  assert.match(pageSource, /marketplace-page-eyebrow/);
  assert.match(pageSource, /marketplace-page-title/);
  assert.match(pageSource, /marketplace-page-summary/);
  assert.match(pageSource, /marketplace-page-guide/);
  assert.match(pageSource, /marketplace-page-guide-label/);
  assert.match(pageSource, /marketplace-page-guide-text/);
  assert.match(pageSource, /marketplace-page-guide-icon/);
  assert.match(pageSource, /marketplace-page-guide-copy/);
  assert.match(pageSource, /marketplace-page-guide-arrow/);
  assert.match(pageSource, /marketplace-page-guide-source/);
  assert.match(pageSource, /marketplace-page-guide-branch/);
  assert.match(pageSource, /marketplace-page-guide-branch-label/);
  assert.match(pageSource, /marketplace-page-guide-paths/);
  assert.match(pageSource, /aria-label=\{t\('市场工作流'\)\}/);
  assert.match(pageSource, /IconKey/);
  assert.match(pageSource, /IconCart/);
  assert.match(pageSource, /IconRoute/);
  assert.match(pageSource, /IconArrowRight/);
  assert.match(pageSource, /t\('AI 供给市场'\)/);
  assert.match(
    pageSource,
    /t\('市场把可用 AI API Key 变成可购买、可路由的调用能力。'\)/,
  );
  assert.match(pageSource, /t\('卖家上架'\)/);
  assert.match(pageSource, /t\('托管 Key，设置模型、价格和并发'\)/);
  assert.match(pageSource, /t\('可直接用于'\)/);
  assert.match(pageSource, /t\('买家购买'\)/);
  assert.match(pageSource, /t\('买断额度，绑定令牌固定路由'\)/);
  assert.match(pageSource, /t\('订单池调用'\)/);
  assert.match(pageSource, /t\('按模型、价格和并发自动选择供给'\)/);

  assert.match(styleSource, /\.marketplace-page-header\s*\{/);
  assert.match(styleSource, /border-bottom:\s*1px solid/);
  assert.match(styleSource, /\.marketplace-page-guide\s*\{/);
  assert.match(styleSource, /\.marketplace-page-guide-item\s*\{/);
  assert.match(styleSource, /\.marketplace-page-guide-icon\s*\{/);
  assert.match(styleSource, /\.marketplace-page-guide-copy\s*\{/);
  assert.match(styleSource, /\.marketplace-page-guide-arrow\s*\{/);
  assert.match(styleSource, /\.marketplace-page-guide-source\s*\{/);
  assert.match(styleSource, /\.marketplace-page-guide-branch\s*\{/);
  assert.match(styleSource, /\.marketplace-page-guide-branch-label\s*\{/);
  assert.match(styleSource, /\.marketplace-page-guide-paths\s*\{/);
  assert.match(styleSource, /@media\s*\(max-width:\s*640px\)/);

  assert.match(enLocale, /"AI 供给市场":\s*"AI Capacity Market"/);
  assert.match(enLocale, /"市场工作流":\s*"Marketplace workflow"/);
  assert.match(enLocale, /"可直接用于":\s*"Directly usable for"/);
  assert.match(enLocale, /"卖家上架":\s*"Sellers list"/);
  assert.match(
    enLocale,
    /"托管 Key，设置模型、价格和并发":\s*"Host keys, set models, prices, and concurrency"/,
  );
});

test("default marketplace header explains the buyer and seller flow", async () => {
  const [pageSource, enLocale, zhLocale] = await Promise.all([
    readDefaultSource("features/marketplace/index.tsx"),
    readDefaultSource("i18n/locales/en.json"),
    readDefaultSource("i18n/locales/zh.json"),
  ]);

  assert.match(
    pageSource,
    /Marketplace turns available AI API keys into purchasable, routable AI capacity\./,
  );
  assert.match(pageSource, /Sellers list/);
  assert.match(pageSource, /Host keys, set models, prices, and concurrency/);
  assert.match(pageSource, /Buyers purchase/);
  assert.match(pageSource, /Buy fixed quota and bind tokens for fixed routing/);
  assert.match(pageSource, /Order pool calls/);
  assert.match(
    pageSource,
    /Automatically select supply by model, price, and concurrency/,
  );
  assert.match(pageSource, /guideItems\.map/);
  assert.match(pageSource, /rounded-lg border/);

  assert.match(enLocale, /"Sellers list":\s*"Sellers list"/);
  assert.match(zhLocale, /"Sellers list":\s*"卖家上架"/);
  assert.match(
    zhLocale,
    /"Marketplace turns available AI API keys into purchasable, routable AI capacity\.":\s*"市场把可用 AI API Key 变成可购买、可路由的调用能力。"/,
  );
});
