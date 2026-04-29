import assert from 'node:assert/strict';
import test from 'node:test';

test('media query helpers support modern listeners, legacy listeners, and missing matchMedia', async () => {
  const {
    getMediaQueryList,
    matchesMediaQuery,
    subscribeToMediaQueryList,
  } = await import('../classic/src/helpers/mediaQuery.js');

  const modernCalls = [];
  const modernMediaQueryList = {
    matches: true,
    addEventListener: (event, listener) => modernCalls.push(['add', event, listener]),
    removeEventListener: (event, listener) =>
      modernCalls.push(['remove', event, listener]),
  };
  const modernListener = () => {};

  const releaseModern = subscribeToMediaQueryList(
    modernMediaQueryList,
    modernListener,
  );
  releaseModern();

  assert.deepEqual(modernCalls, [
    ['add', 'change', modernListener],
    ['remove', 'change', modernListener],
  ]);

  const legacyCalls = [];
  const legacyMediaQueryList = {
    matches: false,
    addListener: (listener) => legacyCalls.push(['add', listener]),
    removeListener: (listener) => legacyCalls.push(['remove', listener]),
  };
  const legacyListener = () => {};

  const releaseLegacy = subscribeToMediaQueryList(
    legacyMediaQueryList,
    legacyListener,
  );
  releaseLegacy();

  assert.deepEqual(legacyCalls, [
    ['add', legacyListener],
    ['remove', legacyListener],
  ]);

  globalThis.window = {
    matchMedia: (query) => ({
      matches: query.includes('max-width'),
    }),
  };

  assert.equal(matchesMediaQuery('(max-width: 767px)'), true);
  assert.equal(getMediaQueryList('(prefers-color-scheme: dark)').matches, false);

  delete globalThis.window;

  assert.equal(matchesMediaQuery('(max-width: 767px)', true), true);
  assert.equal(getMediaQueryList('(max-width: 767px)'), null);
});
