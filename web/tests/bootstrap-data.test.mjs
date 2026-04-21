import assert from 'node:assert/strict';
import test from 'node:test';

const bootstrapDataModulePath = new URL(
  '../src/helpers/bootstrapData.js',
  import.meta.url,
);

test('parsePublicBootstrapJson returns null for empty or malformed payloads', async () => {
  const { parsePublicBootstrapJson } = await import(bootstrapDataModulePath);

  assert.equal(parsePublicBootstrapJson(''), null);
  assert.equal(parsePublicBootstrapJson('{not-json'), null);
});

test('readClientStartupSettings normalizes saved theme and language', async () => {
  const { readClientStartupSettings } = await import(bootstrapDataModulePath);

  const fakeStorage = {
    getItem(key) {
      if (key === 'theme-mode') {
        return 'dark';
      }

      if (key === 'i18nextLng') {
        return 'en_US';
      }

      return null;
    },
  };

  assert.deepEqual(readClientStartupSettings(fakeStorage), {
    themeMode: 'dark',
    language: 'en',
  });
});
