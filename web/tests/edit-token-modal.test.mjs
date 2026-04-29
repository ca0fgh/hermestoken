import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const editTokenModalPath = new URL(
  "../classic/src/components/table/tokens/modals/EditTokenModal.jsx",
  import.meta.url,
);

const load = async (path) => readFile(path, "utf8");

test("edit token modal shows plain none text when no token groups are available", async () => {
  const source = await load(editTokenModalPath);

  assert.match(source, /API\.get\(`\/api\/token\/groups`\)/);
  assert.match(
    source,
    /\{groups\.length > 0 \? \(\s*<Form\.Select[\s\S]*?\) : \(\s*<Form\.Slot label=\{t\('令牌分组'\)\}>[\s\S]*?<Text type='tertiary'>\{t\('没有'\)\}<\/Text>[\s\S]*?<\/Form\.Slot>/,
  );
  assert.doesNotMatch(
    source,
    /placeholder=\{t\('管理员未设置用户可选分组'\)\}/,
  );
  assert.match(
    source,
    /const localGroupOptions =\s*Object\.keys\(data \|\| \{\}\)\.length === 0\s*\?\s*\[\]\s*:\s*processGroupsData\(data\)/,
  );
});

test("edit token modal no longer claims blank token group always falls back to user group", async () => {
  const source = await load(editTokenModalPath);

  assert.doesNotMatch(
    source,
    /placeholder=\{t\('令牌分组，默认为用户的分组'\)\}/,
  );
  assert.doesNotMatch(
    source,
    /请选择用户可选分组；留空仅在默认分组属于用户可选时生效/,
  );
  assert.match(source, /DEFAULT_TOKEN_GROUP\s*=\s*''/);
  assert.match(source, /group:\s*DEFAULT_TOKEN_GROUP/);
  assert.match(
    source,
    /const group\s*=\s*inputs\.group\s*\|\|\s*DEFAULT_TOKEN_GROUP/,
  );
  assert.match(source, /showClear/);
});

test("edit token modal submits configurable marketplace route order", async () => {
  const source = await load(editTokenModalPath);

  assert.match(source, /MARKETPLACE_ROUTE_ORDER_VALUES/);
  assert.match(
    source,
    /DEFAULT_MARKETPLACE_ROUTE_ORDER\s*=\s*\[\s*'fixed_order',\s*'group',\s*'pool'\s*\]/,
  );
  assert.match(source, /normalizeMarketplaceRouteOrder/);
  assert.match(source, /normalizeMarketplaceRouteEnabled/);
  assert.match(source, /normalizeMarketplaceRouteOrderInputs/);
  assert.match(source, /marketplace_route_enabled/);
  assert.match(source, /marketplace_route_order_0/);
  assert.match(source, /marketplace_route_order_1/);
  assert.match(source, /marketplace_route_order_2/);
  assert.match(source, /handleMarketplaceRouteOrderMove/);
  assert.match(source, /handleMarketplaceRouteEnabledChange/);
  assert.match(source, /IconChevronUp/);
  assert.match(source, /IconChevronDown/);
  assert.match(source, /<Switch/);
  assert.match(source, /令牌路由优先级/);
  assert.match(
    source,
    /已启用路由会按列表顺序尝试。默认顺序：市场买断订单、普通分组订单、订单池/,
  );
});
