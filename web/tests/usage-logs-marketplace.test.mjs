import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const readDefaultSource = (relativePath) =>
  readFile(new URL(`../default/src/${relativePath}`, import.meta.url), "utf8");

const readClassicSource = (relativePath) =>
  readFile(new URL(`../classic/src/${relativePath}`, import.meta.url), "utf8");

test("default usage logs display marketplace billing sources and IDs", async () => {
  const [typesSource, columnsSource, detailsDialogSource] = await Promise.all([
    readDefaultSource("features/usage-logs/types.ts"),
    readDefaultSource(
      "features/usage-logs/components/columns/common-logs-columns.tsx",
    ),
    readDefaultSource(
      "features/usage-logs/components/dialogs/details-dialog.tsx",
    ),
  ]);

  assert.match(typesSource, /marketplace_relay\?:\s*boolean/);
  assert.match(typesSource, /marketplace_fixed_order_id\?:\s*number/);
  assert.match(typesSource, /marketplace_pool_credential_id\?:\s*number/);
  assert.match(typesSource, /marketplace_original_path\?:\s*string/);

  assert.match(columnsSource, /getMarketplaceLogInfo/);
  assert.match(columnsSource, /marketplace_fixed_order/);
  assert.match(columnsSource, /Marketplace fixed order/);
  assert.match(columnsSource, /marketplace_pool/);
  assert.match(columnsSource, /Marketplace pool/);
  assert.match(columnsSource, /marketplaceLogInfo\.label/);
  assert.match(columnsSource, /marketplaceLogInfo\.meta/);

  assert.match(detailsDialogSource, /Marketplace source/);
  assert.match(detailsDialogSource, /marketplace_fixed_order_id/);
  assert.match(detailsDialogSource, /marketplace_pool_credential_id/);
  assert.match(detailsDialogSource, /marketplace_original_path/);
});

test("classic usage logs display marketplace billing sources and IDs", async () => {
  const [columnsSource, detailsHookSource] = await Promise.all([
    readClassicSource("components/table/usage-logs/UsageLogsColumnDefs.jsx"),
    readClassicSource("hooks/usage-logs/useUsageLogsData.jsx"),
  ]);

  assert.match(columnsSource, /getMarketplaceLogInfo/);
  assert.match(columnsSource, /marketplace_fixed_order/);
  assert.match(columnsSource, /市场买断/);
  assert.match(columnsSource, /marketplace_pool/);
  assert.match(columnsSource, /市场订单池/);
  assert.match(columnsSource, /marketplaceInfo\.label/);
  assert.match(columnsSource, /marketplaceInfo\.meta/);

  assert.match(detailsHookSource, /市场来源/);
  assert.match(detailsHookSource, /marketplace_fixed_order_id/);
  assert.match(detailsHookSource, /marketplace_pool_credential_id/);
  assert.match(detailsHookSource, /marketplace_original_path/);
});
