import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const editTokenModalPath = new URL(
  '../classic/src/components/table/tokens/modals/EditTokenModal.jsx',
  import.meta.url,
);

const load = async (path) => readFile(path, 'utf8');

test('edit token modal shows plain none text when no token groups are available', async () => {
  const source = await load(editTokenModalPath);

  assert.match(source, /API\.get\(`\/api\/token\/groups`\)/);
  assert.match(
    source,
    /\{groups\.length > 0 \? \(\s*<Form\.Select[\s\S]*?\) : \(\s*<Form\.Slot label=\{t\('令牌分组'\)\}>[\s\S]*?<Text type='tertiary'>\{t\('没有'\)\}<\/Text>[\s\S]*?<\/Form\.Slot>/,
  );
  assert.doesNotMatch(
    source,
    /placeholder=\{t\('管理员未设置用户可选分组'\)\}/,
  );
  assert.match(
    source,
    /const localGroupOptions =\s*Object\.keys\(data \|\| \{\}\)\.length === 0\s*\?\s*\[\]\s*:\s*processGroupsData\(data\)/,
  );
});

test('edit token modal no longer claims blank token group always falls back to user group', async () => {
  const source = await load(editTokenModalPath);

  assert.doesNotMatch(
    source,
    /placeholder=\{t\('令牌分组，默认为用户的分组'\)\}/,
  );
  assert.match(
    source,
    /请选择用户可选分组；留空仅在默认分组属于用户可选时生效/,
  );
});
