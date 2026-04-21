import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const homePath = new URL('../src/pages/Home/index.jsx', import.meta.url);

test('home page prefers public bootstrap content before network fetches', async () => {
  const source = await readFile(homePath, 'utf8');

  assert.match(source, /resolveHomeStartupBootstrap\(\)/);
  assert.match(source, /scheduleNonCriticalWork\(/);
  assert.match(source, /initialNoticeHtml=\{noticeHtml\}/);
});

test('home page no longer parses markdown in the startup path', async () => {
  const source = await readFile(homePath, 'utf8');

  assert.doesNotMatch(source, /import\('marked'\)/);
  assert.match(source, /fetchPublicBootstrap\(/);
});
