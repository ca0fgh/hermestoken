import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

import {
  getFooterSectionVisibility,
  normalizeHeaderNavModules,
} from '../src/helpers/headerNavModules.js';

const footerPath = new URL(
  '../src/components/layout/Footer.jsx',
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
      docs: false,
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
