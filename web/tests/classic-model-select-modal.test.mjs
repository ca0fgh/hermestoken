import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const readClassicSource = (relativePath) =>
  readFile(new URL(`../classic/src/${relativePath}`, import.meta.url), 'utf8');

test('classic model selection modals expand visible model categories', async () => {
  const [multiSelectSource, singleSelectSource] = await Promise.all([
    readClassicSource('components/table/channels/modals/ModelSelectModal.jsx'),
    readClassicSource(
      'components/table/channels/modals/SingleModelSelectModal.jsx',
    ),
  ]);

  assert.match(multiSelectSource, /defaultExpandedCategoryKeys/);
  assert.match(multiSelectSource, /shouldExpandCategories/);
  assert.match(
    multiSelectSource,
    /shouldExpandCategories\s*\?\s*defaultExpandedCategoryKeys\s*:\s*\[\]/,
  );
  assert.match(multiSelectSource, /keyword\.trim\(\) !== ''/);
  assert.match(multiSelectSource, /categoryEntries\.length === 1/);
  assert.doesNotMatch(multiSelectSource, /defaultActiveKey=\{\[\]\}/);

  assert.match(singleSelectSource, /defaultExpandedCategoryKeys/);
  assert.match(singleSelectSource, /shouldExpandCategories/);
  assert.match(
    singleSelectSource,
    /shouldExpandCategories\s*\?\s*defaultExpandedCategoryKeys\s*:\s*\[\]/,
  );
  assert.match(singleSelectSource, /keyword\.trim\(\) !== ''/);
  assert.match(singleSelectSource, /categoryEntries\.length === 1/);
  assert.doesNotMatch(singleSelectSource, /defaultActiveKey=\{\[\]\}/);
});
