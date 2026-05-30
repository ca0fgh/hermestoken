import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const aboutPath = new URL(
  '../classic/src/pages/About/index.jsx',
  import.meta.url,
);
const documentRendererPath = new URL(
  '../classic/src/components/common/DocumentRenderer/index.jsx',
  import.meta.url,
);

const load = async (path) => readFile(path, 'utf8');

test('about page clears stale cached backend content when backend content is empty', async () => {
  const source = await load(aboutPath);

  assert.match(source, /localStorage\.removeItem\('about'\)/);
  assert.match(source, /setAbout\(''\)/);
  assert.match(source, /setUseDefault\(true\)/);
});

test('document renderer clears stale legal document cache when backend content is empty', async () => {
  const source = await load(documentRendererPath);

  assert.match(source, /localStorage\.removeItem\(cacheKey\)/);
  assert.match(source, /setContent\(defaultContent\)/);
});
