import assert from 'node:assert/strict';
import test from 'node:test';

const publicStartupModulePath = new URL(
  '../src/bootstrap/publicStartup.js',
  import.meta.url,
);

test('resolveRoutePublicBootstrap seeds from cache only on the home route', async () => {
  const { resolveRoutePublicBootstrap } = await import(publicStartupModulePath);

  const homeStorage = createTrackingStorage({
    'hermes-public-bootstrap-v1': JSON.stringify({
      savedAt: Date.now(),
      payload: {
        status: {
          system_name: 'HermesToken Home',
        },
      },
    }),
  });

  const publicHomePayload = resolveRoutePublicBootstrap({
    pathname: '/',
    injectedBootstrap: null,
    storage: homeStorage,
  });

  assert.deepEqual(publicHomePayload, {
    status: {
      system_name: 'HermesToken Home',
      __publicBootstrapScope: 'home',
    },
  });
  assert.equal(homeStorage.getItemCalls, 1);

  const nonHomeStorage = createTrackingStorage({
    'hermes-public-bootstrap-v1': JSON.stringify({
      savedAt: Date.now(),
      payload: {
        status: {
          system_name: 'HermesToken Cached',
        },
      },
    }),
  });

  const nonHomePayload = resolveRoutePublicBootstrap({
    pathname: '/login',
    injectedBootstrap: null,
    storage: nonHomeStorage,
  });

  assert.equal(nonHomePayload, null);
  assert.equal(nonHomeStorage.getItemCalls, 0);
  assert.equal(nonHomeStorage.setItemCalls, 0);
});

test('resolveRoutePublicBootstrap refreshes cached bootstrap for the home route only', async () => {
  const { PUBLIC_HOME_SHELL_ID, removePublicHomeShell, resolveRoutePublicBootstrap } = await import(
    publicStartupModulePath
  );

  const storage = createTrackingStorage();
  const injectedBootstrap = {
    status: {
      system_name: 'HermesToken Fresh',
    },
  };

  assert.deepEqual(
    resolveRoutePublicBootstrap({
      pathname: '/',
      injectedBootstrap,
      storage,
    }),
    {
      status: {
        system_name: 'HermesToken Fresh',
        __publicBootstrapScope: 'home',
      },
    },
  );
  assert.equal(storage.setItemCalls, 1);

  const nonHomeStorage = createTrackingStorage();
  assert.equal(
    resolveRoutePublicBootstrap({
      pathname: '/pricing',
      injectedBootstrap,
      storage: nonHomeStorage,
    }),
    null,
  );
  assert.equal(nonHomeStorage.setItemCalls, 0);

  let removed = false;
  removePublicHomeShell({
    getElementById(id) {
      assert.equal(id, PUBLIC_HOME_SHELL_ID);
      return {
        remove() {
          removed = true;
        },
      };
    },
  });
  assert.equal(removed, true);
});

function createTrackingStorage(initialState = {}) {
  const state = new Map(Object.entries(initialState));

  return {
    getItemCalls: 0,
    setItemCalls: 0,
    getItem(key) {
      this.getItemCalls += 1;
      return state.has(key) ? state.get(key) : null;
    },
    setItem(key, value) {
      this.setItemCalls += 1;
      state.set(key, value);
    },
    removeItem(key) {
      state.delete(key);
    },
  };
}
