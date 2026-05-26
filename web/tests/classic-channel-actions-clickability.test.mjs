import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const channelColumnsPath = new URL(
  "../classic/src/components/table/channels/ChannelsColumnDefs.jsx",
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

test("channel operation column reserves a stable clickable fixed-column area", async () => {
  const source = await loadSource(channelColumnsPath);

  assert.match(source, /width:\s*320/);
  assert.match(source, /className:\s*'channel-operate-cell'/);
  assert.match(source, /className='channel-operate-actions'/);
});

test("channel operation fixed cells keep actions above table hit-test layers", async () => {
  const source = await loadSource(classicCssPath);

  assert.match(
    source,
    /\.channel-operate-cell\s*{[\s\S]*pointer-events:\s*auto;/,
  );
  assert.match(
    source,
    /\.semi-table-tbody\s*>\s*\.semi-table-row\s*>\s*\.channel-operate-cell\.semi-table-cell-fixed-right\s*{[\s\S]*z-index:\s*110;[\s\S]*pointer-events:\s*auto;/,
  );
  assert.match(
    source,
    /\.channel-operate-actions\s*{[\s\S]*position:\s*relative;[\s\S]*z-index:\s*1;[\s\S]*pointer-events:\s*auto;/,
  );
  assert.match(
    source,
    /\.channel-operate-actions\s*\.semi-button,\s*[\s\S]*\.channel-operate-actions\s*\.semi-button-group,\s*[\s\S]*\.channel-operate-actions\s*\.semi-dropdown-trigger\s*{[\s\S]*pointer-events:\s*auto;/,
  );
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
