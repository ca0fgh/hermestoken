import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const readSource = (relativePath) =>
  readFileSync(new URL(relativePath, import.meta.url), 'utf8');

test('useModelPricingData consumes display_groups and exposes displayGroups', () => {
  const source = readSource(
    '../src/hooks/model-pricing/useModelPricingData.jsx',
  );

  assert.match(source, /display_groups/);
  assert.match(source, /const \[displayGroups, setDisplayGroups\] = useState\(\{\}\);/);
  assert.match(source, /displayGroups,/);
  assert.doesNotMatch(source, /\busableGroup\b/);
  assert.doesNotMatch(source, /\bautoGroups\b/);
});

test('PricingGroups uses displayGroups and 模型分组', () => {
  const source = readSource(
    '../src/components/table/model-pricing/filter/PricingGroups.jsx',
  );

  assert.match(source, /displayGroups/);
  assert.match(source, /t\('模型分组'\)/);
  assert.doesNotMatch(source, /\busableGroup\b/);
});

test('PricingSidebar and FilterModalContent pass displayGroups', () => {
  const sidebarSource = readSource(
    '../src/components/table/model-pricing/layout/PricingSidebar.jsx',
  );
  const modalSource = readSource(
    '../src/components/table/model-pricing/modal/components/FilterModalContent.jsx',
  );

  assert.match(sidebarSource, /displayGroups=\{categoryProps\.displayGroups\}/);
  assert.match(modalSource, /displayGroups=\{categoryProps\.displayGroups\}/);
  assert.doesNotMatch(sidebarSource, /\busableGroup\b/);
  assert.doesNotMatch(modalSource, /\busableGroup\b/);
});

test('ModelPricingTable and detail sheet use displayGroups without auto chain UI', () => {
  const sheetSource = readSource(
    '../src/components/table/model-pricing/modal/ModelDetailSideSheet.jsx',
  );
  const tableSource = readSource(
    '../src/components/table/model-pricing/modal/components/ModelPricingTable.jsx',
  );
  const pageSource = readSource(
    '../src/components/table/model-pricing/layout/PricingPage.jsx',
  );

  assert.match(sheetSource, /displayGroups/);
  assert.match(tableSource, /displayGroups/);
  assert.match(pageSource, /displayGroups/);
  assert.doesNotMatch(sheetSource, /\busableGroup\b/);
  assert.doesNotMatch(tableSource, /\busableGroup\b/);
  assert.doesNotMatch(tableSource, /\bautoGroups\b/);
  assert.doesNotMatch(tableSource, /auto分组调用链路/);
});
