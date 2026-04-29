import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

import {
  getFooterSectionVisibility,
  normalizeHeaderNavModules,
} from '../classic/src/helpers/headerNavModules.js';

const footerPath = new URL(
  '../classic/src/components/layout/Footer.jsx',
  import.meta.url,
);

test('normalizeHeaderNavModules falls back to defaults for empty config', () => {
  assert.deepEqual(normalizeHeaderNavModules(''), {
    home: true,
    console: true,
    pricing: {
      enabled: true,
      requireAuth: false,
    },
    marketplace: true,
    docs: true,
    about: true,
  });
});

test('normalizeHeaderNavModules upgrades legacy pricing boolean config', () => {
  assert.deepEqual(
    normalizeHeaderNavModules(
      JSON.stringify({
        home: true,
        console: true,
        pricing: false,
        docs: false,
        about: true,
      }),
    ),
    {
      home: true,
      console: true,
      pricing: {
        enabled: false,
        requireAuth: false,
      },
      marketplace: true,
      docs: false,
      about: true,
    },
  );
});

test('normalizeHeaderNavModules enables marketplace for legacy saved config', () => {
  assert.deepEqual(
    normalizeHeaderNavModules(
      JSON.stringify({
        home: true,
        console: true,
        pricing: {
          enabled: true,
          requireAuth: false,
        },
        docs: true,
        about: true,
      }),
    ),
    {
      home: true,
      console: true,
      pricing: {
        enabled: true,
        requireAuth: false,
      },
      marketplace: true,
      docs: true,
      about: true,
    },
  );
});

test('getFooterSectionVisibility hides docs and about sections with the same switches as the header', () => {
  const modules = normalizeHeaderNavModules(
    JSON.stringify({
      home: true,
      console: true,
      pricing: {
        enabled: true,
        requireAuth: false,
      },
      docs: false,
      about: false,
    }),
  );

  assert.deepEqual(
    getFooterSectionVisibility(modules, 'https://docs.example.com'),
    {
      showDocsSection: false,
      showAboutSection: false,
    },
  );
});

test('footer uses shared header-nav visibility instead of hard-coded docs/about sections', async () => {
  const source = await readFile(footerPath, 'utf8');

  assert.match(source, /getFooterSectionVisibility/);
  assert.match(source, /showDocsSection/);
  assert.match(source, /showAboutSection/);
});

test('settings header nav source separates marketplace entry visibility from guest access copy', async () => {
  const source = await readFile(
    new URL(
      '../classic/src/pages/Setting/Operation/SettingsHeaderNavModules.jsx',
      import.meta.url,
    ),
    'utf8',
  );

  assert.match(source, /t\('控制是否显示模型广场入口'\)/);
  assert.match(
    source,
    /t\(\s*'关闭后游客按 default 分组浏览模型广场，登录用户可查看全部展示模型'/,
  );
  assert.match(source, /key:\s*'marketplace'/);
  assert.match(source, /t\('控制是否显示市场入口'\)/);
});

test('settings header nav cards use flex grid layout to avoid float masonry gaps', async () => {
  const source = await readFile(
    new URL(
      '../classic/src/pages/Setting/Operation/SettingsHeaderNavModules.jsx',
      import.meta.url,
    ),
    'utf8',
  );

  assert.match(source, /<Row[^>]*\btype='flex'/);
  assert.match(source, /<Row[^>]*\balign='top'/);
});
