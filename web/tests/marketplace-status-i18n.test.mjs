import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const readDefaultSource = (relativePath) =>
  readFile(new URL(`../default/src/${relativePath}`, import.meta.url), "utf8");

const readClassicSource = (relativePath) =>
  readFile(new URL(`../classic/src/${relativePath}`, import.meta.url), "utf8");

test("default marketplace status badges use explicit i18n labels", async () => {
  const [libSource, sellerSource, ordersSource, fixedOrdersSource, zhLocale] =
    await Promise.all([
      readDefaultSource("features/marketplace/lib.ts"),
      readDefaultSource("features/marketplace/components/seller-tab.tsx"),
      readDefaultSource("features/marketplace/components/orders-tab.tsx"),
      readDefaultSource("features/marketplace/components/fixed-orders-tab.tsx"),
      readDefaultSource("i18n/locales/zh.json"),
    ]);

  assert.match(libSource, /marketplaceStatusLabel/);
  assert.match(sellerSource, /marketplaceStatusLabel/);
  assert.match(ordersSource, /marketplaceStatusLabel/);
  assert.match(fixedOrdersSource, /marketplaceStatusLabel/);
  assert.doesNotMatch(sellerSource, /label=\{t\(credential\./);
  assert.doesNotMatch(ordersSource, /label=\{t\(item\./);
  assert.doesNotMatch(fixedOrdersSource, /t\(order\.status\)/);

  assert.match(zhLocale, /"Marketplace status listed":\s*"已上架"/);
  assert.match(zhLocale, /"Marketplace status enabled":\s*"已启用"/);
  assert.match(zhLocale, /"Marketplace status available":\s*"可用"/);
  assert.match(zhLocale, /"Marketplace status route_available":\s*"可路由"/);
  assert.match(zhLocale, /"Marketplace status route_failed":\s*"不可路由"/);
});

test("classic marketplace status tags translate status values", async () => {
  const [marketplaceSource, enLocale] = await Promise.all([
    readClassicSource("pages/Marketplace/index.jsx"),
    readClassicSource("i18n/locales/en.json"),
  ]);

  assert.match(marketplaceSource, /function statusTag\(status,\s*t\)/);
  assert.match(marketplaceSource, /marketplaceStatusLabels/);
  assert.match(marketplaceSource, /t\(marketplaceStatusLabels\[status\]/);
  assert.doesNotMatch(
    marketplaceSource,
    new RegExp(">\\{status \\|\\| '-'\\}</Tag>"),
  );

  assert.match(enLocale, /"已上架":\s*"Listed"/);
  assert.match(enLocale, /"已启用":\s*"Enabled"/);
  assert.match(enLocale, /"可用":\s*"Available"/);
});
