import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const readClassicSource = (relativePath) =>
  readFile(new URL(`../classic/src/${relativePath}`, import.meta.url), "utf8");

const readDefaultSource = (relativePath) =>
  readFile(new URL(`../default/src/${relativePath}`, import.meta.url), "utf8");

const readRootSource = (relativePath) =>
  readFile(new URL(`../../${relativePath}`, import.meta.url), "utf8");

test("classic marketplace filters use order-derived order ranges", async () => {
  const [pageSource, enLocale] = await Promise.all([
    readClassicSource("pages/Marketplace/index.jsx"),
    readClassicSource("i18n/locales/en.json"),
  ]);

  assert.match(
    pageSource,
    /label:\s*t\('全部'\),\s*value:\s*MARKETPLACE_FILTER_ALL_VALUE/,
  );
  assert.match(pageSource, /\/api\/marketplace\/order-filter-ranges/);
  assert.match(pageSource, /\/api\/marketplace\/orders/);
  assert.match(pageSource, /marketplaceFilterRangesFromOrders/);
  assert.match(pageSource, /function buildMarketplaceFilterOptions/);
  assert.match(pageSource, /formatMarketplaceQuotaRange/);
  assert.match(pageSource, /formatMarketplaceTimeRange/);
  assert.match(pageSource, /formatMarketplaceMultiplierRange/);
  assert.match(pageSource, /formatMarketplaceConcurrencyRange/);
  assert.match(pageSource, /marketplaceHasLimitedQuotaRange/);
  assert.match(pageSource, /marketplaceHasLimitedTimeRange/);
  assert.match(pageSource, /marketplaceHasMultiplierRange/);
  assert.match(pageSource, /marketplaceHasConcurrencyRange/);
  assert.doesNotMatch(
    pageSource,
    /if\s*\(\s*Number\(ranges\?\.limited_quota_count\)\s*>\s*0\s*\)\s*\{\s*options\.push/,
  );
  assert.doesNotMatch(
    pageSource,
    /if\s*\(\s*Number\(ranges\?\.limited_time_count\)\s*>\s*0\s*\)\s*\{\s*options\.push/,
  );
  assert.match(
    pageSource,
    /const defaultFilters = \{[\s\S]*quota_mode:\s*''[\s\S]*time_mode:\s*''/,
  );
  assert.match(pageSource, /min_quota_limit:\s*''/);
  assert.match(pageSource, /max_quota_limit:\s*''/);
  assert.match(pageSource, /min_time_limit_seconds:\s*''/);
  assert.match(pageSource, /max_time_limit_seconds:\s*''/);
  assert.match(pageSource, /min_multiplier:\s*''/);
  assert.match(pageSource, /max_multiplier:\s*''/);
  assert.match(pageSource, /min_concurrency_limit:\s*''/);
  assert.match(pageSource, /max_concurrency_limit:\s*''/);
  assert.match(pageSource, /clearMarketplaceQuotaRangeFilters/);
  assert.match(pageSource, /clearMarketplaceTimeRangeFilters/);
  assert.match(pageSource, /clearMarketplaceMultiplierRangeFilters/);
  assert.match(pageSource, /clearMarketplaceConcurrencyRangeFilters/);
  assert.match(pageSource, /renderMarketplaceQuotaRangeInputs/);
  assert.match(pageSource, /renderMarketplaceTimeRangeInputs/);
  assert.match(pageSource, /renderMarketplaceMultiplierRangeInputs/);
  assert.match(pageSource, /renderMarketplaceConcurrencyRangeInputs/);
  assert.match(pageSource, /\bSlider\b/);
  assert.match(pageSource, /<Slider[\s\S]*range/);
  assert.match(pageSource, /disabled=\{maxValue <= minValue\}/);
  assert.match(pageSource, /normalizeMarketplaceSliderRange/);
  assert.match(pageSource, /marketplace-filter-range-slider/);
  assert.match(pageSource, /displayAmountToQuota\(minValue\)/);
  assert.match(pageSource, /Math\.round\(minValue\s*\*\s*60\)/);
  assert.match(pageSource, /Math\.round\(minValue\s*\*\s*100\)\s*\/\s*100/);
  assert.match(pageSource, /Math\.round\(minValue\)/);
  assert.doesNotMatch(pageSource, /placeholder=\{t\('最低倍率'\)\}/);
  assert.doesNotMatch(pageSource, /placeholder=\{t\('最高倍率'\)\}/);
  assert.doesNotMatch(pageSource, /placeholder=\{t\('最低并发'\)\}/);
  const classicRangeHookSource =
    pageSource.match(
      /function useMarketplaceFilterRanges[\s\S]*?function useMarketplaceModelOptions/,
    )?.[0] ?? "";
  assert.equal(
    (classicRangeHookSource.match(/skipErrorHandler:\s*true/g) ?? []).length,
    2,
  );
  assert.match(pageSource, /setRanges\(\{\}\)/);
  assert.doesNotMatch(classicRangeHookSource, /showError/);
  assert.match(pageSource, /MARKETPLACE_FILTER_ALL_VALUE/);
  assert.match(pageSource, /marketplace-filter-grid/);
  assert.match(pageSource, /marketplace-filter-main-row/);
  assert.match(pageSource, /marketplace-filter-range-row/);
  assert.match(pageSource, /marketplace-filter-quota-range/);
  assert.match(pageSource, /marketplace-filter-time-range/);
  assert.match(pageSource, /marketplace-filter-multiplier-range/);
  assert.match(pageSource, /marketplace-filter-concurrency-range/);
  assert.match(pageSource, /marketplace-filter-range-value/);
  assert.match(pageSource, /useMarketplaceModelOptions/);
  assert.match(pageSource, /marketplaceModelOptionParams/);
  assert.match(pageSource, /buildMarketplaceVendorModelTree/);
  assert.match(pageSource, /parseMarketplaceVendorModelValue/);
  assert.match(pageSource, /renderMarketplaceVendorModelDisplay/);
  assert.match(pageSource, /\/api\/marketplace\/pool\/models/);
  assert.match(pageSource, /<Cascader/);
  assert.match(pageSource, /changeOnSelect/);
  assert.match(pageSource, /filterTreeNode/);
  assert.match(pageSource, /showNext='click'/);
  assert.match(pageSource, /showClear/);
  assert.match(
    pageSource,
    /onClear=\{\(\) =>\s*patch\(\{ vendor_type:\s*undefined,\s*model:\s*'' \}\)\}/,
  );
  assert.match(pageSource, /aria-label=\{t\('厂商模型级联筛选'\)\}/);
  assert.match(pageSource, /insetLabel=\{t\('厂商 \/ 模型'\)\}/);
  assert.match(pageSource, /dropdownStyle=\{\{ minWidth:\s*360/);
  assert.doesNotMatch(pageSource, /\ssearchable\b/);
  assert.match(pageSource, /separator=' -> '/);
  assert.match(pageSource, /return selected\.join\(' -> '\)/);
  assert.match(
    pageSource,
    /return \[\s*vendorValue,\s*marketplaceModelFilterValue\(/,
  );
  assert.match(pageSource, /label:\s*t\('全部模型'\)/);
  assert.match(
    pageSource,
    /marketplaceModelFilterValue\(\s*vendorType,\s*MARKETPLACE_FILTER_ALL_VALUE/,
  );
  assert.match(enLocale, /"额度范围":\s*"Quota range"/);
  assert.match(enLocale, /"时间范围":\s*"Time range"/);
  assert.match(enLocale, /"倍率范围":\s*"Multiplier range"/);
  assert.match(enLocale, /"并发范围":\s*"Concurrency range"/);
});

test("default marketplace filters use order range metadata instead of all-mode options", async () => {
  const [apiSource, sharedSource, ordersSource, poolSource, typesSource] =
    await Promise.all([
      readDefaultSource("features/marketplace/api.ts"),
      readDefaultSource("features/marketplace/components/shared.tsx"),
      readDefaultSource("features/marketplace/components/orders-tab.tsx"),
      readDefaultSource("features/marketplace/components/pool-tab.tsx"),
      readDefaultSource("features/marketplace/types.ts"),
    ]);

  assert.match(apiSource, /listMarketplaceOrderFilterRanges/);
  assert.match(apiSource, /\/api\/marketplace\/order-filter-ranges/);
  assert.match(apiSource, /\/api\/marketplace\/orders/);
  assert.match(apiSource, /marketplaceFilterRangesFromOrders/);
  assert.match(typesSource, /MarketplaceOrderFilterRanges/);
  assert.match(apiSource, /catch\s*\{/);
  assert.match(apiSource, /success:\s*true/);
  const defaultRangeApiSource =
    apiSource.match(
      /export async function listMarketplaceOrderFilterRanges[\s\S]*?function marketplaceFilterRangesFromOrders/,
    )?.[0] ?? "";
  assert.equal(
    (defaultRangeApiSource.match(/skipErrorHandler:\s*true/g) ?? []).length,
    2,
  );
  assert.equal(
    (defaultRangeApiSource.match(/skipBusinessError:\s*true/g) ?? []).length,
    2,
  );
  assert.match(sharedSource, /buildMarketplaceFilterOptions/);
  assert.match(sharedSource, /formatMarketplaceQuotaRange/);
  assert.match(sharedSource, /formatMarketplaceTimeRange/);
  assert.match(sharedSource, /formatMarketplaceMultiplierRange/);
  assert.match(sharedSource, /formatMarketplaceConcurrencyRange/);
  assert.match(sharedSource, /marketplaceHasLimitedQuotaRange/);
  assert.match(sharedSource, /marketplaceHasLimitedTimeRange/);
  assert.match(sharedSource, /marketplaceHasMultiplierRange/);
  assert.match(sharedSource, /marketplaceHasConcurrencyRange/);
  assert.match(sharedSource, /clearMarketplaceQuotaRangeFilters/);
  assert.match(sharedSource, /clearMarketplaceTimeRangeFilters/);
  assert.match(sharedSource, /clearMarketplaceMultiplierRangeFilters/);
  assert.match(sharedSource, /clearMarketplaceConcurrencyRangeFilters/);
  assert.match(sharedSource, /renderQuotaRangeInputs/);
  assert.match(sharedSource, /renderTimeRangeInputs/);
  assert.match(sharedSource, /renderMultiplierRangeInputs/);
  assert.match(sharedSource, /renderConcurrencyRangeInputs/);
  assert.match(sharedSource, /RangeSlider/);
  assert.match(sharedSource, /type='range'/);
  assert.match(sharedSource, /const isSingleValueRange = max === min/);
  assert.match(sharedSource, /disabled=\{isSingleValueRange\}/);
  assert.match(sharedSource, /normalizeRangeSliderValue/);
  assert.match(sharedSource, /marketplaceDisplayAmountToQuota/);
  assert.match(sharedSource, /Math\.round\(minValue\s*\*\s*60\)/);
  assert.match(sharedSource, /Math\.round\(minValue\s*\*\s*100\)\s*\/\s*100/);
  assert.match(sharedSource, /Math\.round\(minValue\)/);
  assert.doesNotMatch(sharedSource, /<Label>\{t\('Min multiplier'\)\}<\/Label>/);
  assert.doesNotMatch(sharedSource, /<Label>\{t\('Max multiplier'\)\}<\/Label>/);
  assert.doesNotMatch(sharedSource, /<Label>\{t\('Min concurrency'\)\}<\/Label>/);
  assert.doesNotMatch(
    sharedSource,
    /if\s*\(\s*Number\(filterRanges\?\.limited_quota_count\)\s*>\s*0\s*\)\s*\{/,
  );
  assert.doesNotMatch(
    sharedSource,
    /if\s*\(\s*Number\(filterRanges\?\.limited_time_count\)\s*>\s*0\s*\)\s*\{/,
  );
  assert.match(
    await readDefaultSource("features/marketplace/components/shared-data.ts"),
    /quota_mode:\s*''[\s\S]*time_mode:\s*''[\s\S]*min_quota_limit:\s*''[\s\S]*max_quota_limit:\s*''[\s\S]*min_time_limit_seconds:\s*''[\s\S]*max_time_limit_seconds:\s*''[\s\S]*min_multiplier:\s*''[\s\S]*max_multiplier:\s*''[\s\S]*min_concurrency_limit:\s*''[\s\S]*max_concurrency_limit:\s*''/,
  );
  assert.match(typesSource, /min_quota_limit\?:\s*number \| ''/);
  assert.match(typesSource, /max_quota_limit\?:\s*number \| ''/);
  assert.match(typesSource, /min_time_limit_seconds\?:\s*number \| ''/);
  assert.match(typesSource, /max_time_limit_seconds\?:\s*number \| ''/);
  assert.match(typesSource, /min_multiplier\?:\s*number \| ''/);
  assert.match(typesSource, /max_multiplier\?:\s*number \| ''/);
  assert.match(typesSource, /min_concurrency_limit\?:\s*number \| ''/);
  assert.match(typesSource, /max_concurrency_limit\?:\s*number \| ''/);
  assert.match(typesSource, /min_multiplier:\s*number/);
  assert.match(typesSource, /max_multiplier:\s*number/);
  assert.match(typesSource, /min_concurrency_limit:\s*number/);
  assert.match(typesSource, /max_concurrency_limit:\s*number/);
  assert.match(sharedSource, /label:\s*t\('All'\),\s*value:\s*ALL_VALUE/);
  assert.match(ordersSource, /listMarketplaceOrderFilterRanges/);
  assert.match(poolSource, /listMarketplaceOrderFilterRanges/);
});

test("marketplace exposes buyer order filter range endpoint", async () => {
  const [controllerSource, serviceSource, routerSource] = await Promise.all([
    readRootSource("controller/marketplace_buyer.go"),
    readRootSource("service/marketplace_buyer.go"),
    readRootSource("router/api-router.go"),
  ]);

  assert.match(controllerSource, /BuyerGetMarketplaceOrderFilterRanges/);
  assert.match(controllerSource, /min_quota_limit/);
  assert.match(controllerSource, /max_quota_limit/);
  assert.match(controllerSource, /min_time_limit_seconds/);
  assert.match(controllerSource, /max_time_limit_seconds/);
  assert.match(controllerSource, /min_multiplier/);
  assert.match(controllerSource, /max_multiplier/);
  assert.match(controllerSource, /min_concurrency_limit/);
  assert.match(controllerSource, /max_concurrency_limit/);
  assert.match(serviceSource, /GetMarketplaceOrderFilterRanges/);
  assert.match(serviceSource, /MinQuotaLimit/);
  assert.match(serviceSource, /MaxQuotaLimit/);
  assert.match(serviceSource, /MinTimeLimitSeconds/);
  assert.match(serviceSource, /MaxTimeLimitSeconds/);
  assert.match(serviceSource, /MinMultiplier/);
  assert.match(serviceSource, /MaxMultiplier/);
  assert.match(serviceSource, /MinConcurrencyLimit/);
  assert.match(serviceSource, /MaxConcurrencyLimit/);
  assert.match(serviceSource, /marketplaceOrderMultiplierRange/);
  assert.match(serviceSource, /marketplaceOrderConcurrencyLimitRange/);
  assert.match(routerSource, /GET\("\/order-filter-ranges"/);
});
