import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const editChannelModalPath = new URL(
  '../classic/src/components/table/channels/modals/EditChannelModal.jsx',
  import.meta.url,
);

test('classic channel editor model picker reads configured priced models only', async () => {
  const source = await readFile(editChannelModalPath, 'utf8');
  const fetchModelsSource = source.slice(
    source.indexOf('const fetchModels = async () => {'),
    source.indexOf('const fetchGroups = async () => {'),
  );

  assert.match(fetchModelsSource, /API\.get\(`\/api\/channel\/models_priced`\)/);
  assert.doesNotMatch(fetchModelsSource, /\/api\/channel\/models`/);
  assert.match(fetchModelsSource, /const pricedModelIds = \(res\.data\.data \|\| \[\]\)/);
  assert.match(fetchModelsSource, /setOriginModelOptions\(localModelOptions\)/);
  assert.match(fetchModelsSource, /setFullModels\(pricedModelIds\)/);
});
