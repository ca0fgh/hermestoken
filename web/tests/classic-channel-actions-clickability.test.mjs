import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const channelColumnsPath = new URL(
  "../classic/src/components/table/channels/ChannelsColumnDefs.jsx",
  import.meta.url,
);
const channelOperateActionsPath = new URL(
  "../classic/src/components/table/channels/ChannelOperateActions.jsx",
  import.meta.url,
);
const channelsDataHookPath = new URL(
  "../classic/src/hooks/channels/useChannelsData.jsx",
  import.meta.url,
);
const channelPreviewMockPath = new URL(
  "../classic/src/helpers/channelPreviewMock.js",
  import.meta.url,
);
const classicIndexPath = new URL("../classic/src/index.jsx", import.meta.url);
const classicApiPath = new URL(
  "../classic/src/helpers/api.js",
  import.meta.url,
);
const classicCssPath = new URL("../classic/src/index.css", import.meta.url);

const loadSource = (path) => readFile(path, "utf8");

test("channel operation column delegates click actions to a compact fixed action component", async () => {
  const source = await loadSource(channelColumnsPath);

  assert.match(source, /import ChannelOperateActions/);
  assert.match(source, /<ChannelOperateActions/);
  assert.match(source, /fixed:\s*'right'/);
  assert.match(source, /width:\s*240/);
  assert.match(source, /className:\s*'channel-operate-cell'/);
  assert.doesNotMatch(source, /SplitButtonGroup/);
});

test("channel operation actions expose edit as the first direct button", async () => {
  const source = await loadSource(channelOperateActionsPath);

  assert.match(source, /const handleEdit\s*=/);
  assert.match(source, /data-testid='channel-edit-action'/);
  assert.match(
    source,
    /<Button[\s\S]*data-testid='channel-edit-action'[\s\S]*onClick=\{handleEdit\}[\s\S]*>\s*\{t\('编辑'\)\}/,
  );
  assert.match(
    source,
    /const moreMenuItems\s*=\s*\[[\s\S]*name:\s*t\('多密钥管理'\)[\s\S]*name:\s*t\('复制'\)[\s\S]*name:\s*t\('删除'\)/,
  );
});

test("channel operation styles are compact without hard z-index hit-test overrides", async () => {
  const source = await loadSource(classicCssPath);

  assert.match(source, /\.channel-operate-cell\s*{[\s\S]*min-width:\s*240px;/);
  assert.match(
    source,
    /\.channel-operate-actions\s*{[\s\S]*display:\s*flex[\s\S]*justify-content:\s*flex-end;/,
  );
  assert.doesNotMatch(source, /z-index:\s*110/);
});

test("channel list loading state is always cleared after request failures or stale responses", async () => {
  const source = await loadSource(channelsDataHookPath);

  assert.match(source, /try\s*{/);
  assert.match(source, /finally\s*{[\s\S]*setLoading\(false\);/);
  assert.doesNotMatch(
    source,
    /if \(res === undefined \|\| reqId !== requestCounter\.current\) {\s*return;\s*}/,
  );
});

test("classic channel preview mock is explicit and development-only", async () => {
  const source = await loadSource(channelPreviewMockPath);

  assert.match(source, /import\.meta\.env\.DEV/);
  assert.match(source, /channelPreview/);
  assert.match(source, /localStorage\.setItem\('channel-preview-mock', '1'\)/);
  assert.match(source, /role:\s*10/);
  assert.match(source, /id:\s*118/);
  assert.match(source, /filtered\.slice\(offset,\s*offset\s*\+\s*pageSize\)/);
  assert.match(source, /pathname\s*===\s*'\/api\/channel\/'/);
  assert.match(source, /\/api\/status/);
});

test("classic app installs channel preview mock before console routing and API requests", async () => {
  const indexSource = await loadSource(classicIndexPath);
  const apiSource = await loadSource(classicApiPath);

  assert.match(indexSource, /installChannelPreviewMock/);
  assert.match(
    indexSource,
    /installChannelPreviewMock\(\);[\s\S]*renderConsoleApp/,
  );
  assert.match(apiSource, /installChannelPreviewApiMock\(API\)/);
});
