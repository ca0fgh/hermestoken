import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const hookSource = readFileSync(
  new URL(
    '../classic/src/pages/Setting/Ratio/hooks/useModelPricingEditorState.js',
    import.meta.url,
  ),
  'utf8',
);
const editorSource = readFileSync(
  new URL(
    '../classic/src/pages/Setting/Ratio/components/ModelPricingEditor.jsx',
    import.meta.url,
  ),
  'utf8',
);

test('model pricing editor persists task per-request and per-second pricing separately', () => {
  assert.match(hookSource, /TaskModelPricing/);
  assert.match(hookSource, /per_request/);
  assert.match(hookSource, /per_second/);
  assert.match(hookSource, /include_other_ratios/);
  assert.match(hookSource, /perSecondPrice/);
  assert.match(hookSource, /billingMode === 'per-second'/);
});

test('model pricing editor exposes separate per-request and per-second modes', () => {
  assert.match(editorSource, /Radio value='per-second'/);
  assert.match(editorSource, /t\('每秒价格'\)/);
  assert.match(editorSource, /t\('固定价格'\)/);
  assert.doesNotMatch(editorSource, /叠加分辨率\/尺寸等其它倍率/);
  assert.doesNotMatch(editorSource, /按次\+按秒/);
});

test('model pricing preview wraps long backend JSON values', () => {
  assert.match(editorSource, /overflowWrap: 'anywhere'/);
  assert.match(editorSource, /wordBreak: 'break-word'/);
  assert.match(editorSource, /minWidth: 0/);
});
