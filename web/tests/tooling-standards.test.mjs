import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const prettierIgnorePath = new URL('../.prettierignore', import.meta.url);
const eslintIgnorePath = new URL('../.eslintignore', import.meta.url);
const headerNavModulesPath = new URL(
  '../classic/src/helpers/headerNavModules.js',
  import.meta.url,
);
const groupTableDraftPath = new URL(
  '../classic/src/pages/Setting/Ratio/components/groupTableDraft.js',
  import.meta.url,
);

const requiredHeader = `/*
Copyright (C) 2025 QuantumNous`;

test('tooling ignores generated dist artifacts', async () => {
  const [prettierIgnoreSource, eslintIgnoreSource] = await Promise.all([
    readFile(prettierIgnorePath, 'utf8'),
    readFile(eslintIgnorePath, 'utf8'),
  ]);

  assert.match(prettierIgnoreSource, /(^|\n)dist\/?(\n|$)/);
  assert.match(eslintIgnoreSource, /(^|\n)dist\/?(\n|$)/);
});

test('shared helper modules keep the required license header', async () => {
  const [headerNavModulesSource, groupTableDraftSource] = await Promise.all([
    readFile(headerNavModulesPath, 'utf8'),
    readFile(groupTableDraftPath, 'utf8'),
  ]);

  assert.ok(headerNavModulesSource.startsWith(requiredHeader));
  assert.ok(groupTableDraftSource.startsWith(requiredHeader));
});
