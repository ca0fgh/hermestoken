import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const readDefaultSource = (relativePath) =>
  readFile(new URL(`../default/src/${relativePath}`, import.meta.url), "utf8");

const readClassicSource = (relativePath) =>
  readFile(new URL(`../classic/src/${relativePath}`, import.meta.url), "utf8");

test("classic seller escrow table labels quota limit and consumed quota", async () => {
  const pageSource = await readClassicSource("pages/Marketplace/index.jsx");

  assert.match(pageSource, /function renderSellerQuotaUsage\(item,\s*t\)/);
  assert.match(pageSource, /t\('已消耗额度'\)/);
  assert.match(
    pageSource,
    /title:\s*t\('额度'\),\s*render:\s*\(_,\s*record\)\s*=>\s*renderSellerQuotaUsage\(record,\s*t\)/,
  );
});

test("marketplace order and seller views omit sold fixed-order exposure", async () => {
  const [classicSource, defaultOrdersSource, defaultSellerSource] =
    await Promise.all([
      readClassicSource("pages/Marketplace/index.jsx"),
      readDefaultSource("features/marketplace/components/orders-tab.tsx"),
      readDefaultSource("features/marketplace/components/seller-tab.tsx"),
    ]);
  const classicOrdersSource = classicSource.slice(
    classicSource.indexOf("function OrdersTab"),
    classicSource.indexOf("function PoolTab"),
  );
  const classicSellerSource = classicSource.slice(
    classicSource.indexOf("function SellerTab"),
    classicSource.indexOf("export default function Marketplace"),
  );

  for (const source of [
    classicOrdersSource,
    classicSellerSource,
    defaultOrdersSource,
    defaultSellerSource,
  ]) {
    assert.doesNotMatch(source, /已售买断|Sold exposure/);
    assert.doesNotMatch(source, /fixed_order_sold_quota/);
    assert.doesNotMatch(source, /formatSoldExposure/);
  }
});

test("seller escrow views omit sold fixed-order exposure", async () => {
  const [classicSource, defaultSource] = await Promise.all([
    readClassicSource("pages/Marketplace/index.jsx"),
    readDefaultSource("features/marketplace/components/seller-tab.tsx"),
  ]);
  const sellerTabSource = classicSource.slice(
    classicSource.indexOf("function SellerTab"),
    classicSource.indexOf("export default function Marketplace"),
  );

  assert.doesNotMatch(sellerTabSource, /title:\s*t\('已售买断'\)/);
  assert.doesNotMatch(sellerTabSource, /formatSoldExposure\(record\)/);
  assert.doesNotMatch(defaultSource, /label=\{t\('Sold exposure'\)\}/);
  assert.doesNotMatch(
    defaultSource,
    /formatMarketplaceQuotaUSD\(credential\.fixed_order_sold_quota\)/,
  );
});

test("fixed-order buyer views omit remaining amount", async () => {
  const [classicSource, defaultSource] = await Promise.all([
    readClassicSource("pages/Marketplace/index.jsx"),
    readDefaultSource("features/marketplace/components/fixed-orders-tab.tsx"),
  ]);

  assert.match(classicSource, /title:\s*t\('已买断金额'\)/);
  assert.match(classicSource, /title:\s*t\('已消耗金额'\)/);
  assert.doesNotMatch(classicSource, /title:\s*t\('剩余金额'\)/);
  assert.doesNotMatch(
    classicSource,
    /formatMarketplaceQuotaUSD\(record\.remaining_quota\)/,
  );

  assert.match(defaultSource, /label=\{t\('Purchased amount'\)\}/);
  assert.match(defaultSource, /label=\{t\('Spent amount'\)\}/);
  assert.doesNotMatch(defaultSource, /label=\{t\('Remaining amount'\)\}/);
  assert.doesNotMatch(
    defaultSource,
    /formatMarketplaceQuotaUSD\(order\.remaining_quota\)/,
  );
});

test("fixed-order buyer views show time limit status", async () => {
  const [classicSource, defaultSource] = await Promise.all([
    readClassicSource("pages/Marketplace/index.jsx"),
    readDefaultSource("features/marketplace/components/fixed-orders-tab.tsx"),
  ]);

  assert.match(
    classicSource,
    /function renderFixedOrderCombinedStatus\(record,\s*t\)/,
  );
  assert.doesNotMatch(classicSource, /title:\s*t\('限时状态'\)/);
  assert.match(
    classicSource,
    /title:\s*t\('状态'\),\s*render:\s*\(_,\s*record\)\s*=>\s*renderFixedOrderCombinedStatus\(record,\s*t\)/,
  );
  assert.match(classicSource, /renderFixedOrderTimeStatus\(record,\s*t\)/);
  assert.match(defaultSource, /function formatFixedOrderTimeLimit\(/);
  assert.match(defaultSource, /formatFixedOrderTimeLimit\(order,\s*t\)/);
  assert.doesNotMatch(defaultSource, /label=\{t\('Time limit'\)\}/);
});
