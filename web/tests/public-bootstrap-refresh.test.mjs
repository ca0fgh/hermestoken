import assert from 'node:assert/strict';
import test from 'node:test';

const publicBootstrapRefreshModulePath = new URL(
  '../src/pages/Home/publicBootstrapRefresh.js',
  import.meta.url,
);

test('fetchPublicBootstrap uses a browser no-store request for deferred refreshes', async () => {
  const { fetchPublicBootstrap } = await import(publicBootstrapRefreshModulePath);

  let capturedRequest = null;
  const payload = {
    success: true,
    data: {
      home: {
        mode: 'default',
      },
    },
  };

  const response = await fetchPublicBootstrap(async (url, init) => {
    capturedRequest = { url, init };
    return {
      ok: true,
      async json() {
        return payload;
      },
    };
  });

  assert.deepEqual(response, payload);
  assert.equal(capturedRequest.url, '/api/public/bootstrap');
  assert.equal(capturedRequest.init.cache, 'no-store');
  assert.equal(capturedRequest.init.headers['Cache-Control'], 'no-store');
});
