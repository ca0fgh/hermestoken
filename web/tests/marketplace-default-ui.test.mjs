import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const readDefaultSource = (relativePath) =>
  readFile(new URL(`../default/src/${relativePath}`, import.meta.url), "utf8");

const readClassicSource = (relativePath) =>
  readFile(new URL(`../classic/src/${relativePath}`, import.meta.url), "utf8");

const readRootSource = (relativePath) =>
  readFile(new URL(`../../${relativePath}`, import.meta.url), "utf8");

test("local docker compose passes the marketplace credential secret", async () => {
  const composeSource = await readRootSource("docker-compose.yml");

  assert.match(
    composeSource,
    /MARKETPLACE_CREDENTIAL_SECRET=\$\{MARKETPLACE_CREDENTIAL_SECRET/,
  );
});

test("default frontend wires marketplace route, top navigation, and API endpoints", async () => {
  const [
    routeSource,
    sidebarSource,
    apiSource,
    pageSource,
    poolTabSource,
    libSource,
    typesSource,
    topNavSource,
    sidebarConfigSource,
    sellerSource,
    sharedDataSource,
    sharedSource,
    maintenanceConfigSource,
    headerNavigationSource,
    generalRegistrySource,
    marketplaceSettingsSource,
    buyerTokenPanelSource,
    callSnippetSource,
    ordersTabSource,
    fixedOrdersTabSource,
    keyTypesSource,
    keyApiSource,
    keyDrawerSource,
    keyConstantsSource,
    keyFormSource,
    keyGroupComboboxSource,
  ] = await Promise.all([
    readDefaultSource("routes/_authenticated/marketplace/index.tsx"),
    readDefaultSource("hooks/use-sidebar-data.ts"),
    readDefaultSource("features/marketplace/api.ts"),
    readDefaultSource("features/marketplace/index.tsx"),
    readDefaultSource("features/marketplace/components/pool-tab.tsx"),
    readDefaultSource("features/marketplace/lib.ts"),
    readDefaultSource("features/marketplace/types.ts"),
    readDefaultSource("hooks/use-top-nav-links.ts"),
    readDefaultSource("hooks/use-sidebar-config.ts"),
    readDefaultSource("features/marketplace/components/seller-tab.tsx"),
    readDefaultSource("features/marketplace/components/shared-data.ts"),
    readDefaultSource("features/marketplace/components/shared.tsx"),
    readDefaultSource("features/system-settings/maintenance/config.ts"),
    readDefaultSource(
      "features/system-settings/maintenance/header-navigation-section.tsx",
    ),
    readDefaultSource("features/system-settings/general/section-registry.tsx"),
    readDefaultSource(
      "features/system-settings/general/marketplace-settings-section.tsx",
    ),
    readDefaultSource("features/marketplace/components/buyer-token-panel.tsx"),
    readDefaultSource("features/marketplace/components/call-snippet.tsx"),
    readDefaultSource("features/marketplace/components/orders-tab.tsx"),
    readDefaultSource("features/marketplace/components/fixed-orders-tab.tsx"),
    readDefaultSource("features/keys/types.ts"),
    readDefaultSource("features/keys/api.ts"),
    readDefaultSource("features/keys/components/api-keys-mutate-drawer.tsx"),
    readDefaultSource("features/keys/constants.ts"),
    readDefaultSource("features/keys/lib/api-key-form.ts"),
    readDefaultSource("features/keys/components/api-key-group-combobox.tsx"),
  ]);

  assert.match(
    routeSource,
    /createFileRoute\('\/_authenticated\/marketplace\/'\)/,
  );
  assert.doesNotMatch(sidebarSource, /title:\s*t\('Marketplace'\)/);
  assert.doesNotMatch(sidebarSource, /url:\s*'\/marketplace'/);
  assert.match(topNavSource, /title:\s*t\('Marketplace'\)/);
  assert.match(topNavSource, /href:\s*'\/marketplace'/);
  assert.match(
    topNavSource,
    /title:\s*t\('Home'\)[\s\S]*?title:\s*t\('Marketplace'\)[\s\S]*?title:\s*t\('Console'\)/,
  );
  assert.match(topNavSource, /marketplace:\s*true/);
  assert.doesNotMatch(sidebarConfigSource, /marketplace:\s*true/);
  assert.doesNotMatch(sidebarConfigSource, /'\/marketplace'/);
  assert.match(maintenanceConfigSource, /marketplace:\s*true/);
  assert.match(headerNavigationSource, /key:\s*'marketplace'/);

  assert.match(apiSource, /\/api\/marketplace\/orders/);
  assert.match(apiSource, /\/api\/marketplace\/fixed-orders/);
  assert.match(apiSource, /\/bind-token/);
  assert.match(apiSource, /\/bind-tokens/);
  assert.match(apiSource, /fixed_order_ids/);
  assert.match(apiSource, /token_ids/);
  assert.match(apiSource, /\/api\/marketplace\/pool\/models/);
  assert.match(apiSource, /\/api\/marketplace\/pricing/);
  assert.match(apiSource, /\/api\/marketplace\/seller\/priced-models/);
  assert.match(apiSource, /\/api\/marketplace\/seller\/credentials/);
  assert.match(apiSource, /\/api\/marketplace\/seller\/settlements\/release/);
  assert.doesNotMatch(
    apiSource,
    /seller\/credentials\/\$\{credentialId\}\/events/,
  );
  assert.doesNotMatch(apiSource, /listSellerMarketplaceCredentialEvents/);
  assert.match(apiSource, /base_url/);
  assert.match(apiSource, /model_mapping/);
  assert.match(apiSource, /buildMarketplaceCredentialSetting/);
  assert.match(
    apiSource,
    /setting:\s*buildMarketplaceCredentialSetting\(values\)/,
  );
  assert.match(typesSource, /MarketplacePricingItem/);
  assert.match(typesSource, /proxy:\s*string/);
  assert.match(sharedDataSource, /proxy:\s*''/);
  assert.match(
    sharedDataSource,
    /export const defaultFilters[\s\S]*quota_mode:\s*''[\s\S]*time_mode:\s*''/,
  );
  assert.match(sharedSource, /label:\s*t\('All'\),\s*value:\s*ALL_VALUE/);
  assert.match(sharedSource, /value\s*===\s*ALL_VALUE[\s\S]*\?\s*''/);
  assert.match(libSource, /parseMarketplaceCredentialSetting/);
  assert.match(libSource, /marketplaceCredentialProxy/);
  assert.match(libSource, /marketplaceCredentialSettingWithoutProxy/);
  assert.match(libSource, /proxy:\s*values\.proxy\.trim\(\)/);
  assert.match(sellerSource, /function ModelPicker/);
  assert.match(sellerSource, /buildSellerPricePreview/);
  assert.doesNotMatch(sellerSource, /DEFAULT_MARKETPLACE_MODELS/);
  assert.doesNotMatch(sellerSource, /gpt-5\.5/);
  assert.doesNotMatch(apiSource, /listPricedMarketplaceModels/);
  assert.match(sellerSource, /listSellerMarketplacePricedModels/);
  assert.match(sellerSource, /enabled:\s*dialogOpen/);
  assert.doesNotMatch(sellerSource, /channel_priced_models/);
  assert.doesNotMatch(sellerSource, /\/api\/channel\/models_priced/);
  assert.doesNotMatch(sellerSource, /listMarketplacePricing/);
  assert.doesNotMatch(sellerSource, /\/api\/pricing/);
  assert.match(sellerSource, /Quota condition/);
  assert.match(sellerSource, /QuotaCompoundControl/);
  assert.match(sellerSource, /Time condition/);
  assert.match(sellerSource, /TimeCompoundControl/);
  assert.match(sellerSource, /time_limit_seconds/);
  assert.match(sellerSource, /value === 'unlimited'/);
  assert.match(sellerSource, /API URL \/ Base URL/);
  assert.match(sellerSource, /Proxy Address/);
  assert.match(sellerSource, /socks5:\/\/user:pass@host:port/);
  assert.match(
    sellerSource,
    /Network proxy for this channel \(supports socks5 protocol\)/,
  );
  assert.match(sellerSource, /setField\('proxy'/);
  assert.match(
    sellerSource,
    /marketplaceCredentialProxy\(credential\.setting\)/,
  );
  assert.match(
    sellerSource,
    /marketplaceCredentialSettingWithoutProxy\(credential\.setting\)/,
  );
  assert.match(sellerSource, /Pricing preview/);
  assert.match(sellerSource, /Official billing/);
  assert.match(sellerSource, /Buyer billing/);
  assert.match(typesSource, /input_price_per_mtok/);
  assert.match(typesSource, /output_price_per_mtok/);
  assert.match(typesSource, /cache_read_price_per_mtok/);
  assert.match(libSource, /按量计费/);
  assert.match(libSource, /1M tokens/);
  assert.match(libSource, /按次计费/);
  assert.match(libSource, /按秒计费/);
  assert.match(sellerSource, /getSystemOptions/);
  assert.match(sellerSource, /getOptionValue/);
  assert.match(sellerSource, /MarketplaceMaxCredentialConcurrency/);
  assert.match(sellerSource, /maxCredentialConcurrency/);
  assert.match(sellerSource, /maxConcurrency/);
  assert.match(sellerSource, /clampMarketplaceConcurrency/);
  assert.doesNotMatch(sellerSource, /HistoryDialog/);
  assert.doesNotMatch(sellerSource, /onHistory/);
  assert.doesNotMatch(sellerSource, /Credential history/);
  assert.doesNotMatch(
    sellerSource,
    /Header override JSON|Test model|Organization/,
  );

  assert.match(`${pageSource}\n${libSource}`, /X-Marketplace-Fixed-Order-Id/);
  assert.match(
    libSource,
    /MARKETPLACE_UNIFIED_RELAY_ENDPOINT\s*=\s*'\/v1\/chat\/completions'/,
  );
  assert.doesNotMatch(
    libSource,
    /\/marketplace\/(?:pool\/)?v1\/chat\/completions/,
  );
  assert.doesNotMatch(pageSource, /BuyerTokenPanel/);
  assert.match(pageSource, /SectionPageLayout\.Actions/);
  assert.match(pageSource, /to='\/keys'/);
  assert.match(pageSource, /Manage console tokens/);
  assert.match(poolTabSource, /BuyerTokenPanel/);
  assert.doesNotMatch(poolTabSource, /CallSnippet/);
  assert.match(poolTabSource, /Activate order pool/);
  assert.match(poolTabSource, /Deactivate order pool/);
  assert.match(poolTabSource, /poolActivated/);
  assert.match(poolTabSource, /updateApiKey/);
  assert.match(poolTabSource, /marketplace_route_enabled/);
  assert.match(poolTabSource, /Order pool activated for this token/);
  assert.match(poolTabSource, /Order pool deactivated for this token/);
  assert.match(poolTabSource, /tokenWithoutMarketplacePoolRoute/);
  assert.match(poolTabSource, /PowerOff/);
  assert.match(poolTabSource, /setFilters\(savedPoolFilters \?\? defaultFilters\)/);
  assert.doesNotMatch(poolTabSource, /MARKETPLACE_POOL_RELAY_ENDPOINT/);
  assert.match(buyerTokenPanelSource, /getApiKeys/);
  assert.match(buyerTokenPanelSource, /to='\/keys'/);
  assert.match(buyerTokenPanelSource, /fullTokenKey\(token\.key\)/);
  assert.match(buyerTokenPanelSource, /fetchTokenKey/);
  assert.match(buyerTokenPanelSource, /DropdownMenu/);
  assert.match(buyerTokenPanelSource, /Copy Key/);
  assert.match(buyerTokenPanelSource, /Copy Connection Info/);
  assert.match(buyerTokenPanelSource, /encodeConnectionString/);
  assert.match(buyerTokenPanelSource, /getConfiguredConnectionURL/);
  assert.match(buyerTokenPanelSource, /useStatus/);
  assert.match(buyerTokenPanelSource, /copySelectedTokenConnectionInfo/);
  assert.match(
    buyerTokenPanelSource,
    /encodeConnectionString\(key,\s*getConfiguredConnectionURL\(status\)\)/,
  );
  assert.match(buyerTokenPanelSource, /function tokenKeyLabel/);
  assert.match(
    buyerTokenPanelSource,
    /tokenKeyLabel\(\s*token,\s*token\.id === selectedToken\?\.id && selectedTokenDisplayKey/,
  );
  assert.doesNotMatch(buyerTokenPanelSource, /TokenSummary/);
  assert.doesNotMatch(
    buyerTokenPanelSource,
    /API_KEY_STATUSES|StatusBadge|Market order|Select a token before copying/,
  );
  assert.match(callSnippetSource, /fetchTokenKey/);
  assert.match(callSnippetSource, /fixedOrderBound/);
  assert.match(callSnippetSource, /!orderId/);
  assert.match(callSnippetSource, /\{!orderId\s*\?\s*\(\s*<pre/);
  assert.match(fixedOrdersTabSource, /bindMarketplaceFixedOrderTokens/);
  assert.match(fixedOrdersTabSource, /getApiKeys/);
  assert.match(
    `${fixedOrdersTabSource}\n${callSnippetSource}`,
    /Edit token bindings/,
  );
  assert.match(fixedOrdersTabSource, /bindingTokenSelection/);
  assert.match(fixedOrdersTabSource, /Bound tokens/);
  assert.match(fixedOrdersTabSource, /BoundTokenKeyChip/);
  assert.match(fixedOrdersTabSource, /BoundTokenDropdown/);
  assert.match(fixedOrdersTabSource, /PopoverContent/);
  assert.match(fixedOrdersTabSource, /DropdownMenu/);
  assert.match(fixedOrdersTabSource, /Copy Key/);
  assert.match(fixedOrdersTabSource, /Copy Connection Info/);
  assert.match(fixedOrdersTabSource, /encodeConnectionString/);
  assert.match(fixedOrdersTabSource, /getConfiguredConnectionURL/);
  assert.match(fixedOrdersTabSource, /useStatus/);
  assert.match(
    fixedOrdersTabSource,
    /encodeConnectionString\(key,\s*getConfiguredConnectionURL\(status\)\)/,
  );
  assert.match(fixedOrdersTabSource, /onCopyConnectionInfo/);
  assert.match(fixedOrdersTabSource, /function tokenKeyLabel/);
  assert.match(
    fixedOrdersTabSource,
    /const displayLabel = tokenKeyLabel\(token, displayKey\)/,
  );
  assert.match(
    fixedOrdersTabSource,
    /grid-cols-\[minmax\(0,1fr\)_1\.5rem_1\.5rem\]/,
  );
  assert.match(fixedOrdersTabSource, /break-all whitespace-normal/);
  assert.match(fixedOrdersTabSource, /w-\[min\(42rem,calc\(100vw-2rem\)\)\]/);
  assert.match(fixedOrdersTabSource, /fetchTokenKey/);
  assert.match(fixedOrdersTabSource, /Eye/);
  assert.match(fixedOrdersTabSource, /Copy/);
  assert.doesNotMatch(fixedOrdersTabSource, /tokenNamesBoundToOrder/);
  assert.match(fixedOrdersTabSource, /marketplace_fixed_order_ids/);
  assert.doesNotMatch(fixedOrdersTabSource, /onTokenBound/);
  assert.match(libSource, /YOUR_PLATFORM_TOKEN/);
  assert.match(libSource, /fixedOrderBound/);
  assert.match(libSource, /token\?: string/);
  assert.match(keyTypesSource, /marketplace_fixed_order_id/);
  assert.match(keyTypesSource, /marketplace_fixed_order_ids/);
  assert.match(keyTypesSource, /marketplace_route_order/);
  assert.match(keyTypesSource, /marketplace_route_enabled/);
  assert.match(keyTypesSource, /DEFAULT_MARKETPLACE_ROUTE_ORDER/);
  assert.match(keyTypesSource, /DEFAULT_MARKETPLACE_ROUTE_ENABLED/);
  assert.match(keyTypesSource, /normalizeMarketplaceRouteOrder/);
  assert.match(keyTypesSource, /normalizeMarketplaceRouteEnabled/);
  assert.match(keyApiSource, /\/api\/marketplace\/fixed-orders/);
  assert.match(keyDrawerSource, /Marketplace fixed order binding/);
  assert.match(keyDrawerSource, /marketplace_fixed_order_ids/);
  assert.match(keyDrawerSource, /Token route priority/);
  assert.match(keyDrawerSource, /marketplace_route_order/);
  assert.match(keyDrawerSource, /marketplace_route_enabled/);
  assert.match(keyDrawerSource, /Order pool is active for this token/);
  assert.match(keyDrawerSource, /moveMarketplaceRouteOrderItem/);
  assert.match(keyDrawerSource, /toggleMarketplaceRouteEnabled/);
  assert.match(keyDrawerSource, /ArrowUp/);
  assert.match(keyDrawerSource, /ArrowDown/);
  assert.match(keyDrawerSource, /MultiSelect/);
  assert.match(keyConstantsSource, /DEFAULT_GROUP\s*=\s*''/);
  assert.match(keyFormSource, /group:\s*data\.group\s*\|\|\s*DEFAULT_GROUP/);
  assert.match(keyFormSource, /DEFAULT_MARKETPLACE_ROUTE_ORDER/);
  assert.match(keyFormSource, /DEFAULT_MARKETPLACE_ROUTE_ENABLED/);
  assert.match(keyFormSource, /normalizeMarketplaceRouteOrder/);
  assert.match(keyFormSource, /normalizeMarketplaceRouteEnabled/);
  assert.doesNotMatch(keyFormSource, /group:\s*data\.group\s*\|\|\s*''/);
  assert.doesNotMatch(keyDrawerSource, /value:\s*'auto'/);
  assert.match(keyGroupComboboxSource, /__empty__/);
  assert.match(keyGroupComboboxSource, /Do not set group/);
  assert.match(ordersTabSource, /Buy fixed amount/);
  assert.match(ordersTabSource, /purchased_amount_usd/);
  assert.match(ordersTabSource, /Purchase amount \(USD\)/);
  assert.match(ordersTabSource, /getSystemOptions/);
  assert.match(ordersTabSource, /MarketplaceFeeRate/);
  assert.match(ordersTabSource, /marketplaceBuyerPaymentUSD/);
  assert.match(ordersTabSource, /Buyer transaction fee \{\{rate\}\}/);
  assert.match(
    ordersTabSource,
    /final fixed order balance and deduction include the global buyer transaction fee/,
  );
  assert.doesNotMatch(ordersTabSource, /Equivalent fixed quota/);
  assert.match(ordersTabSource, /Official price/);
  assert.match(ordersTabSource, /Multiplied price/);
  assert.doesNotMatch(ordersTabSource, /Own escrow/);
  assert.doesNotMatch(ordersTabSource, /disabled=\{isOwnOrder\}/);
  assert.doesNotMatch(poolTabSource, /CallSnippet/);
  assert.match(poolTabSource, /Activate order pool/);
  assert.match(poolTabSource, /updateApiKey/);
  assert.doesNotMatch(poolTabSource, /MARKETPLACE_POOL_RELAY_ENDPOINT/);
  assert.match(`${pageSource}\n${libSource}`, /\/v1\/chat\/completions/);
  assert.match(libSource, /min_concurrency_limit/);
  assert.match(generalRegistrySource, /id:\s*'marketplace'/);
  assert.match(generalRegistrySource, /MarketplaceFeeRate:\s*settings\.MarketplaceFeeRate/);
  assert.match(marketplaceSettingsSource, /MarketplaceFeeRate/);
  assert.match(marketplaceSettingsSource, /Buyer transaction fee rate/);
  assert.match(marketplaceSettingsSource, /0\.05 means 5%/);
  assert.match(marketplaceSettingsSource, /MarketplaceMaxSellerMultiplier/);
  assert.match(marketplaceSettingsSource, /MarketplaceEnabledVendorTypes/);
  assert.doesNotMatch(
    `${pageSource}\n${typesSource}\n${sellerSource}`,
    /encrypted_api_key|key_fingerprint/i,
  );
});

test("classic frontend exposes marketplace top navigation, route, and page", async () => {
  const [
    navigationSource,
    publicRoutesSource,
    appSource,
    sidebarSource,
    sidebarHookSource,
    adminSidebarSettingsSource,
    pageSource,
    pageStyleSource,
    tokenModalSource,
  ] = await Promise.all([
    readClassicSource("hooks/common/useNavigation.js"),
    readClassicSource("routes/PublicRoutes.jsx"),
    readClassicSource("App.jsx"),
    readClassicSource("components/layout/SiderBar.jsx"),
    readClassicSource("hooks/common/useSidebar.js"),
    readClassicSource(
      "pages/Setting/Operation/SettingsSidebarModulesAdmin.jsx",
    ),
    readClassicSource("pages/Marketplace/index.jsx"),
    readClassicSource("pages/Marketplace/index.css"),
    readClassicSource("components/table/tokens/modals/EditTokenModal.jsx"),
  ]);
  const sellerTabSource = pageSource.slice(
    pageSource.indexOf("function SellerTab()"),
    pageSource.indexOf("export default function Marketplace()"),
  );

  assert.match(navigationSource, /text:\s*t\('市场'\)/);
  assert.match(navigationSource, /to:\s*'\/marketplace'/);
  assert.match(
    navigationSource,
    /text:\s*t\('首页'\)[\s\S]*?text:\s*t\('市场'\)[\s\S]*?text:\s*t\('控制台'\)/,
  );
  assert.match(publicRoutesSource, /path='\/marketplace'/);
  assert.match(appSource, /marketplaceEnabled=/);
  assert.doesNotMatch(
    sidebarSource,
    /text:\s*t\('市场'\),\s*itemKey:\s*'marketplace'/,
  );
  assert.doesNotMatch(sidebarHookSource, /marketplace:\s*true/);
  assert.doesNotMatch(adminSidebarSettingsSource, /key:\s*'marketplace'/);
  assert.match(pageSource, /\/api\/marketplace\/orders/);
  assert.match(pageSource, /\/api\/marketplace\/pool\/models/);
  assert.match(
    pageSource,
    /const defaultFilters = \{[\s\S]*quota_mode:\s*''[\s\S]*time_mode:\s*''/,
  );
  assert.match(pageSource, /MARKETPLACE_FILTER_ALL_VALUE\s*=\s*'__all__'/);
  assert.match(
    pageSource,
    /label:\s*t\('全部'\),\s*value:\s*MARKETPLACE_FILTER_ALL_VALUE/,
  );
  assert.match(pageSource, /marketplace-filter-grid/);
  assert.match(
    pageStyleSource,
    /\.marketplace-filter-grid\s*\{[\s\S]*display:\s*flex;[\s\S]*flex-direction:\s*column;/,
  );
  assert.match(pageSource, /marketplace-filter-main-row/);
  assert.match(pageSource, /marketplace-filter-range-row/);
  assert.match(
    pageStyleSource,
    /\.marketplace-filter-range-row\s*\{[\s\S]*grid-template-columns:\s*repeat\(auto-fit,\s*minmax\(min\(100%,\s*240px\),\s*1fr\)\)/,
  );
  assert.match(pageStyleSource, /marketplace-filter-quota-range/);
  assert.match(pageStyleSource, /marketplace-filter-time-range/);
  assert.match(pageStyleSource, /marketplace-filter-multiplier-range/);
  assert.match(pageStyleSource, /marketplace-filter-concurrency-range/);
  assert.match(pageStyleSource, /marketplace-filter-range-value/);
  assert.match(pageSource, /useMarketplaceModelOptions/);
  assert.match(pageSource, /\/api\/marketplace\/pool\/models/);
  assert.match(pageSource, /<Cascader/);
  assert.match(pageSource, /buildMarketplaceVendorModelTree/);
  assert.match(pageSource, /parseMarketplaceVendorModelValue/);
  assert.match(pageSource, /renderMarketplaceVendorModelDisplay/);
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
  assert.match(pageSource, /dropdownStyle=\{\{ maxHeight:\s*360/);
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
  assert.match(
    pageStyleSource,
    /\.marketplace-filter-cascader\s*\{[\s\S]*grid-column:\s*span 2;/,
  );
  assert.doesNotMatch(pageSource, /placeholder='gpt-4o-mini'/);
  assert.match(pageSource, /\/bind-token/);
  assert.match(pageSource, /\/bind-tokens/);
  assert.match(pageSource, /fixed_order_ids/);
  assert.match(pageSource, /token_ids/);
  assert.match(pageSource, /编辑绑定令牌/);
  assert.match(pageSource, /bindMarketplaceFixedOrderTokens/);
  assert.match(pageSource, /applyMarketplaceFixedOrderTokenBindings/);
  assert.match(pageSource, /buyerTokens=\{buyerTokens\}/);
  assert.match(pageSource, /onBuyerTokensChange=\{setBuyerTokens\}/);
  assert.match(pageSource, /已绑定令牌/);
  assert.match(pageSource, /BoundTokenKeyChip/);
  assert.match(pageSource, /BoundTokenDropdown/);
  assert.match(pageSource, /Popover/);
  assert.match(pageSource, /Dropdown/);
  assert.match(pageSource, /IconEyeOpened/);
  assert.match(pageSource, /IconCopy/);
  assert.match(pageSource, /复制密钥/);
  assert.match(pageSource, /复制连接信息/);
  assert.match(pageSource, /encodeChannelConnectionString/);
  assert.match(pageSource, /getServerAddress/);
  assert.match(pageSource, /\/v1\/chat\/completions/);
  assert.doesNotMatch(
    pageSource,
    /\/marketplace\/(?:pool\/)?v1\/chat\/completions/,
  );
  assert.match(pageSource, /copyBoundTokenConnectionInfo/);
  {
    const buyerTokenPanelSource = pageSource.slice(
      pageSource.indexOf("function BuyerTokenPanel"),
      pageSource.indexOf("function PoolTab"),
    );
    assert.match(buyerTokenPanelSource, /Dropdown/);
    assert.match(buyerTokenPanelSource, /复制密钥/);
    assert.match(buyerTokenPanelSource, /复制连接信息/);
    assert.match(buyerTokenPanelSource, /copySelectedTokenConnectionInfo/);
    assert.match(buyerTokenPanelSource, /encodeChannelConnectionString/);
    assert.match(buyerTokenPanelSource, /getServerAddress/);
    assert.match(
      buyerTokenPanelSource,
      /tokenKeyLabel\(\s*token,\s*token\.id === selectedToken\?\.id && selectedTokenDisplayKey/,
    );
    assert.doesNotMatch(
      buyerTokenPanelSource,
      /已选择|未选择|市场订单|未绑定市场订单/,
    );
  }
  assert.doesNotMatch(pageSource, /tokenNamesBoundToOrder/);
  assert.match(
    pageStyleSource,
    /\.marketplace-page-header\s*\{[\s\S]*align-items:\s*center;/,
  );
  assert.match(pageStyleSource, /\.marketplace-bound-token-chip\s*\{/);
  assert.match(pageStyleSource, /\.marketplace-bound-token-dropdown\s*\{/);
  assert.match(pageSource, /className='marketplace-page/);
  assert.match(
    pageStyleSource,
    /\.marketplace-page \.semi-space-vertical\s*\{[\s\S]*align-items:\s*stretch;/,
  );
  assert.match(
    pageStyleSource,
    /\.marketplace-page \.semi-space-vertical > \.semi-space-item\s*\{[\s\S]*width:\s*100%;/,
  );
  assert.match(
    pageStyleSource,
    /\.marketplace-bound-token-dropdown\s*\{[\s\S]*box-sizing:\s*border-box;[\s\S]*width:\s*min\(680px, calc\(100vw - 48px\)\);/,
  );
  assert.match(
    pageStyleSource,
    /\.marketplace-bound-token-chip\s*\{[\s\S]*box-sizing:\s*border-box;[\s\S]*grid-template-columns:\s*minmax\(0, 1fr\) 24px 24px;[\s\S]*max-width:\s*100%;/,
  );
  assert.match(
    pageStyleSource,
    /\.marketplace-bound-token-chip\.is-revealed \.marketplace-bound-token-key\s*\{[\s\S]*white-space:\s*normal;[\s\S]*overflow-wrap:\s*anywhere;[\s\S]*word-break:\s*break-all;/,
  );
  {
    const marketplaceRenderSource = pageSource.slice(
      pageSource.indexOf("export default function Marketplace()"),
    );
    const poolTabSource = pageSource.slice(
      pageSource.indexOf("function PoolTab"),
      pageSource.indexOf("function FixedOrdersTab"),
    );
    assert.doesNotMatch(marketplaceRenderSource, /<BuyerTokenPanel/);
    assert.match(marketplaceRenderSource, /<ManageTokenButton \/>/);
    assert.match(poolTabSource, /<BuyerTokenPanel/);
    assert.doesNotMatch(poolTabSource, /<ManageTokenButton \/>/);
    assert.doesNotMatch(poolTabSource, /管理令牌/);
    assert.match(poolTabSource, /激活使用/);
    assert.match(poolTabSource, /取消激活/);
    assert.match(poolTabSource, /取消激活中/);
    assert.match(poolTabSource, /激活中/);
    assert.match(poolTabSource, /保存条件/);
    assert.match(poolTabSource, /poolActivated/);
    assert.match(poolTabSource, /saveMarketplacePoolFiltersForToken/);
    assert.match(poolTabSource, /activateMarketplacePoolForToken/);
    assert.match(poolTabSource, /deactivateMarketplacePoolForToken/);
    assert.match(poolTabSource, /\/api\/marketplace\/pool\/token-filters/);
    assert.match(poolTabSource, /tokenWithMarketplacePoolRoute/);
    assert.match(poolTabSource, /tokenWithoutMarketplacePoolRoute/);
    assert.match(poolTabSource, /savedPoolFilters \?\? defaultFilters/);
    assert.match(
      poolTabSource,
      /disabled=\{!selectedBuyerToken \|\| activatingPool\}/,
    );
    assert.doesNotMatch(poolTabSource, /停止激活/);
    assert.doesNotMatch(poolTabSource, /该令牌已停止使用订单池/);
    assert.ok(
      poolTabSource.indexOf("<FilterBar") <
        poolTabSource.indexOf("className='marketplace-pool-activation'"),
      "pool filters should render before the activation control",
    );
    assert.doesNotMatch(
      poolTabSource,
      /<CallSnippet/,
    );
    assert.doesNotMatch(pageStyleSource, /marketplace-pool-call-snippet/);
  }
  assert.match(tokenModalSource, /订单池已激活/);
  assert.match(pageSource, /fullTokenKey\(token\.key\)/);
  assert.match(pageSource, /function tokenKeyLabel/);
  assert.match(
    pageSource,
    /const displayLabel = tokenKeyLabel\(token, displayKey\)/,
  );
  assert.match(pageSource, /!orderId/);
  assert.match(pageSource, /\{!orderId\s*\?\s*\(\s*<TextArea/);
  assert.match(tokenModalSource, /marketplace_fixed_order_ids/);
  assert.match(tokenModalSource, /marketplace_route_order/);
  assert.match(tokenModalSource, /marketplace_route_enabled/);
  assert.match(tokenModalSource, /MARKETPLACE_ROUTE_ORDER_VALUES/);
  assert.match(tokenModalSource, /令牌路由优先级/);
  assert.match(tokenModalSource, /handleMarketplaceRouteOrderMove/);
  assert.match(tokenModalSource, /handleMarketplaceRouteEnabledChange/);
  assert.match(tokenModalSource, /IconChevronUp/);
  assert.match(tokenModalSource, /IconChevronDown/);
  assert.match(
    tokenModalSource,
    /已启用路由会按列表顺序尝试。默认顺序：市场买断订单、普通分组订单、订单池/,
  );
  assert.match(tokenModalSource, /multiple/);
  assert.match(tokenModalSource, /不绑定，可选择多个市场买断订单/);
  assert.match(tokenModalSource, /DEFAULT_TOKEN_GROUP\s*=\s*''/);
  assert.match(tokenModalSource, /group:\s*DEFAULT_TOKEN_GROUP/);
  assert.match(
    tokenModalSource,
    /const group\s*=\s*inputs\.group\s*\|\|\s*DEFAULT_TOKEN_GROUP/,
  );
  assert.match(tokenModalSource, /showClear/);
  assert.match(pageSource, /\/api\/marketplace\/seller\/credentials/);
  assert.match(
    pageSource,
    /\/api\/marketplace\/seller\/credentials\/fetch-models/,
  );
  assert.doesNotMatch(
    pageSource,
    /\/api\/marketplace\/seller\/credentials\/\$\{record\.id\}\/events/,
  );
  assert.doesNotMatch(pageSource, /showEvents/);
  assert.doesNotMatch(pageSource, /历史状态变更/);
  assert.doesNotMatch(pageSource, /ModelSelectModal/);
  assert.doesNotMatch(pageSource, /openSellerModelModal/);
  assert.doesNotMatch(pageSource, /sellerModelModalVisible/);
  assert.match(
    sellerTabSource,
    /label=\{t\('模型 \*'\)\}[\s\S]*?<Select[\s\S]*?multiple[\s\S]*?optionList=\{modelOptionsWithSelected\}/,
  );
  assert.match(sellerTabSource, /filter=\{selectFilter\}/);
  assert.match(sellerTabSource, /renderSelectedItem=\{\(optionNode\) =>/);
  assert.match(sellerTabSource, /value=\{selectedModels\}/);
  assert.match(sellerTabSource, /onDropdownVisibleChange=\{\(visible\) =>/);
  assert.match(
    sellerTabSource,
    /if \(visible\) \{\s*loadPricedModels\(\{ silent: true \}\);\s*\}/,
  );
  assert.doesNotMatch(
    sellerTabSource,
    /<TextArea[\s\S]*?value=\{selectedModels\.join\(','\)\}/,
  );
  assert.match(pageSource, /modelOptionsWithSelected/);
  assert.match(pageSource, /function renderModelOptionLabel/);
  assert.match(sellerTabSource, /renderOptionItem=\{renderSellerModelOption\}/);
  assert.match(sellerTabSource, /pricedModelsReady/);
  assert.match(
    sellerTabSource,
    /key=\{\s*pricedModelsReady[\s\S]*?'priced-models-ready'[\s\S]*?'priced-models-loading'[\s\S]*?\}/,
  );
  assert.match(sellerTabSource, /position='bottomLeft'/);
  assert.match(sellerTabSource, /autoAdjustOverflow=\{false\}/);
  assert.match(sellerTabSource, /pricedModelsLoadedRef/);
  assert.match(
    pageSource,
    /label:\s*model,[\s\S]*?value:\s*model/,
  );
  assert.doesNotMatch(
    pageSource,
    /label:\s*\(\s*<span className='flex items-center gap-1'>/,
  );
  assert.doesNotMatch(sellerTabSource, /models=\{fullModels\}/);
  assert.doesNotMatch(sellerTabSource, /selected=\{selectedModels\}/);
  assert.match(pageSource, /getModelCategories/);
  assert.match(sellerTabSource, /loadPricedModels\(\{ silent: true \}\)/);
  assert.match(
    sellerTabSource,
    /useEffect\(\(\) => \{\s*loadPricedModels\(\{ silent: true \}\);\s*\}, \[loadPricedModels\]\);/,
  );
  assert.match(sellerTabSource, /emptyContent=\{/);
  assert.match(sellerTabSource, /正在加载模型/);
  assert.match(sellerTabSource, /暂无可用模型，请先在模型定价中配置模型/);
  assert.doesNotMatch(sellerTabSource, /setFullModels\(models\)/);
  assert.doesNotMatch(sellerTabSource, /setFullModels\(selectableModels\)/);
  assert.doesNotMatch(sellerTabSource, /setSellerModelModalVisible\(true\)/);
  assert.match(pageSource, /已获取 \{\{count\}\} 个模型，可在下拉框中选择/);
  assert.doesNotMatch(pageSource, /模型列表加载失败，点击重试/);
  assert.doesNotMatch(sellerTabSource, />\s*\{t\('选择模型'\)\}/);
  assert.match(sellerTabSource, /fetchUpstreamModels/);
  assert.match(sellerTabSource, /\/api\/marketplace\/seller\/credentials\/fetch-models/);
  assert.match(sellerTabSource, />\s*\{t\('获取模型列表'\)\}/);
  assert.match(pageSource, /marketplaceModelsForSave/);
  assert.match(
    pageSource,
    /mergeModels\(selectedModels, splitModels\(customModel\)\)/,
  );
  assert.match(pageSource, /models:\s*marketplaceModelsForSave/);
  assert.match(
    pageSource,
    /marketplaceModelsForSave\.length === 0[\s\S]*请至少选择一个模型/,
  );
  assert.doesNotMatch(pageSource, /models:\s*splitModels\(form\.models\)/);
  assert.doesNotMatch(sellerTabSource, /DEFAULT_MARKETPLACE_MODELS/);
  assert.doesNotMatch(sellerTabSource, /gpt-5\.4/);
  assert.match(sellerTabSource, /\/api\/marketplace\/seller\/priced-models/);
  assert.doesNotMatch(sellerTabSource, /\/api\/marketplace\/pricing/);
  assert.doesNotMatch(sellerTabSource, /\/api\/pricing/);
  assert.match(pageSource, /modelNameFromPricingItem/);
  assert.match(sellerTabSource, /loadPricedModels/);
  assert.match(sellerTabSource, /pricedModelsLoadedRef/);
  assert.match(sellerTabSource, /pricedModelsRef/);
  assert.match(sellerTabSource, /pricedModelsPromiseRef/);
  assert.doesNotMatch(sellerTabSource, /\/api\/channel\/models_priced/);
  assert.doesNotMatch(pageSource, /MODEL_PRICING_OPTION_KEYS/);
  assert.doesNotMatch(pageSource, /CompletionRatioMeta/);
  assert.doesNotMatch(pageSource, /modelNamesFromOptions/);
  assert.match(pageSource, /读取模型定价设置中的模型/);
  assert.doesNotMatch(pageSource, /填入相关模型/);
  assert.match(sellerTabSource, />\s*\{t\('获取模型列表'\)\}/);
  assert.match(pageSource, /自定义模型名称/);
  assert.match(pageSource, /接口地址 \/ Base URL/);
  assert.match(pageSource, /代理地址/);
  assert.match(pageSource, /socks5:\/\/user:pass@host:port/);
  assert.match(pageSource, /用于配置网络代理，支持 socks5 协议/);
  assert.match(pageSource, /buildMarketplaceCredentialSetting/);
  assert.match(pageSource, /proxy:\s*String\(proxy \|\| ''\)\.trim\(\)/);
  assert.match(
    pageSource,
    /buildMarketplaceCredentialSetting\(form\.setting, form\.proxy\)/,
  );
  assert.match(pageSource, /计费预览/);
  assert.match(pageSource, /官方计费/);
  assert.match(pageSource, /倍率后计费/);
  assert.match(pageSource, /input_price_per_mtok/);
  assert.match(pageSource, /output_price_per_mtok/);
  assert.match(pageSource, /cache_read_price_per_mtok/);
  assert.match(pageSource, /按量计费/);
  assert.match(pageSource, /1M tokens/);
  assert.match(pageSource, /按次计费/);
  assert.match(pageSource, /按秒计费/);
  assert.match(pageSource, /buildSellerPricePreview/);
  assert.match(pageSource, /计费倍率/);
  assert.match(pageSource, /并发上限/);
  assert.match(pageSource, /SplitButtonGroup/);
  assert.match(pageSource, /IconTreeTriangleDown/);
  assert.match(pageSource, /MarketplaceCredentialModelTestModal/);
  assert.match(pageSource, /testMarketplaceCredential/);
  assert.match(pageSource, /renderMarketplaceResponseTime/);
  assert.match(
    pageSource,
    /renderMarketplaceResponseTime\(text,\s*t,\s*record\.health_status\)/,
  );
  assert.match(pageSource, /healthStatus === 'failed'/);
  assert.match(pageSource, /statusTag\(record\.health_status,\s*t\)/);
  assert.match(pageSource, /statusTag\(record\.route_status,\s*t\)/);
  assert.doesNotMatch(pageSource, /statusTag\(record\.capacity_status,\s*t\)/);
  assert.match(pageSource, /response_time/);
  assert.match(pageSource, /响应时间/);
  assert.match(
    pageSource,
    /\/api\/marketplace\/seller\/credentials\/\$\{record\.id\}\/test\?/,
  );
  assert.match(
    pageSource,
    /\/api\/marketplace\/seller\/credentials\/\$\{record\.id\}\/\$\{action\}/,
  );
  assert.match(pageSource, /\/api\/option\//);
  assert.match(pageSource, /MarketplaceMaxCredentialConcurrency/);
  assert.match(pageSource, /maxCredentialConcurrency/);
  assert.match(pageSource, /clampCredentialConcurrency/);
  assert.match(pageSource, /额度条件/);
  assert.match(pageSource, /QuotaCompoundControl/);
  assert.match(pageSource, /时间条件/);
  assert.match(pageSource, /TimeCompoundControl/);
  assert.match(pageSource, /time_limit_seconds/);
  assert.match(pageSource, /mode === 'unlimited'/);
  assert.match(pageSource, /买断金额/);
  assert.match(pageSource, /purchased_amount_usd/);
  assert.match(pageSource, /买断金额 \(USD\)/);
  assert.match(pageSource, /MarketplaceFeeRate/);
  assert.match(pageSource, /marketplaceBuyerPaymentUSD/);
  assert.match(pageSource, /formatMarketplaceFeePercent/);
  assert.match(pageSource, /预计实际扣除 \{\{amount\}\}/);
  assert.match(pageSource, /填写的是基础调用额度/);
  assert.doesNotMatch(pageSource, /折算固定额度/);
  assert.doesNotMatch(pageSource, /dataIndex:\s*'purchased_quota'/);
  assert.doesNotMatch(pageSource, /dataIndex:\s*'remaining_quota'/);
  assert.match(pageSource, /官方计费/);
  assert.match(pageSource, /倍率后计费/);
  assert.doesNotMatch(pageSource, /自己的托管/);
  assert.doesNotMatch(
    pageSource,
    /测试模型|OpenAI Organization|Header override JSON/,
  );
});
