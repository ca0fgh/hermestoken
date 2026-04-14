import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const appPath = new URL('../src/App.jsx', import.meta.url);
const viteConfigPath = new URL('../vite.config.js', import.meta.url);
const packageJsonPath = new URL('../package.json', import.meta.url);

test('heavy console routes are lazy loaded to keep the app entry chunk small', async () => {
  const source = await readFile(appPath, 'utf8');

  assert.match(
    source,
    /const Playground = lazy\(\(\) => import\('\.\/pages\/Playground'\)\);/,
  );
  assert.match(
    source,
    /const Setting = lazy\(\(\) => import\('\.\/pages\/Setting'\)\);/,
  );
  assert.match(
    source,
    /const ModelPage = lazy\(\(\) => import\('\.\/pages\/Model'\)\);/,
  );
  assert.match(
    source,
    /const Token = lazy\(\(\) => import\('\.\/pages\/Token'\)\);/,
  );
  assert.match(
    source,
    /const Chat = lazy\(\(\) => import\('\.\/pages\/Chat'\)\);/,
  );
});

test('vite build keeps only safe heavy dependencies in dedicated chunks', async () => {
  const source = await readFile(viteConfigPath, 'utf8');
  const packageJson = JSON.parse(await readFile(packageJsonPath, 'utf8'));

  assert.match(source, /manualChunks:\s*(buildManualChunkName|\()/);
  assert.match(source, /return 'visactor';/);
  assert.match(source, /onwarn:\s*(handleBuildWarning|\()/);
  assert.match(source, /lottie-web\/build\/player\/lottie\.js/);
  assert.match(source, /chunkSizeWarningLimit:\s*3500/);
  assert.match(packageJson.scripts.build, /BROWSERSLIST_IGNORE_OLD_DATA=true/);
});

test('vite build keeps startup dependencies in react-core to avoid blank-screen chunk cycles', async () => {
  const source = await readFile(viteConfigPath, 'utf8');

  assert.match(
    source,
    /id\.includes\('i18next'\)[\s\S]*return 'react-core';/,
  );
  assert.doesNotMatch(
    source,
    /id\.includes\('i18next'\)[\s\S]*return 'i18n';/,
  );
  assert.match(
    source,
    /id\.includes\('@lobehub\/fluent-emoji'\)[\s\S]*return 'react-core';/,
  );
  assert.match(
    source,
    /id\.includes\('@lobehub\/icons'\)[\s\S]*return 'react-core';/,
  );
  assert.match(
    source,
    /id\.includes\('lucide-react'\)[\s\S]*return 'react-core';/,
  );
  assert.match(
    source,
    /id\.includes\('\/react-icons\/'\)[\s\S]*return 'react-core';/,
  );
  assert.match(
    source,
    /id\.includes\('react-markdown'\)[\s\S]*return 'react-core';/,
  );
  assert.match(
    source,
    /id\.includes\('@douyinfe\/semi-ui'\)[\s\S]*return 'react-core';/,
  );
  assert.match(
    source,
    /id\.includes\('@radix-ui'\)[\s\S]*return 'react-core';/,
  );
  assert.match(
    source,
    /id\.includes\('\/mermaid\/'\)[\s\S]*return 'react-core';/,
  );
  assert.doesNotMatch(source, /return 'markdown';/);
  assert.doesNotMatch(source, /return 'semi-ui';/);
  assert.doesNotMatch(source, /return 'ui-kit';/);
  assert.doesNotMatch(source, /return 'mermaid';/);
});
