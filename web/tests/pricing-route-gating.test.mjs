import React from 'react';
import { afterEach, beforeAll, beforeEach, describe, expect, mock, test } from 'bun:test';
import { act, create } from 'react-test-renderer';
import { MemoryRouter } from 'react-router-dom';
import { StatusContext } from '../src/context/Status/index.jsx';

let importCounter = 0;
const h = React.createElement;

function createStorage() {
  const store = new Map();

  return {
    clear() {
      store.clear();
    },
    getItem(key) {
      return store.has(key) ? store.get(key) : null;
    },
    key(index) {
      return Array.from(store.keys())[index] ?? null;
    },
    removeItem(key) {
      store.delete(key);
    },
    setItem(key, value) {
      store.set(key, String(value));
    },
    get length() {
      return store.size;
    },
  };
}

function installBrowserShims() {
  const localStorage = createStorage();
  const sessionStorage = createStorage();
  const location = {
    hash: '',
    href: 'http://localhost/pricing',
    origin: 'http://localhost',
    pathname: '/pricing',
    search: '',
  };

  const history = {
    length: 1,
    state: null,
    back() {},
    forward() {},
    go() {},
    pushState(_state, _unused, nextUrl = '/pricing') {
      this.state = _state ?? null;
      location.pathname = String(nextUrl);
      location.href = `http://localhost${location.pathname}`;
    },
    replaceState(_state, _unused, nextUrl = '/pricing') {
      this.state = _state ?? null;
      location.pathname = String(nextUrl);
      location.href = `http://localhost${location.pathname}`;
    },
  };

  const document = {
    addEventListener() {},
    body: {},
    createElement() {
      return {};
    },
    defaultView: null,
    documentElement: {},
    location,
    removeEventListener() {},
  };

  const windowObject = {
    addEventListener() {},
    dispatchEvent() {
      return true;
    },
    document,
    history,
    localStorage,
    location,
    navigator: { userAgent: 'bun-test' },
    removeEventListener() {},
    sessionStorage,
  };

  document.defaultView = windowObject;

  Object.assign(globalThis, {
    IS_REACT_ACT_ENVIRONMENT: true,
    document,
    localStorage,
    navigator: windowObject.navigator,
    sessionStorage,
    window: windowObject,
  });
}

function getText(node) {
  if (node == null) {
    return '';
  }

  if (typeof node === 'string') {
    return node;
  }

  if (Array.isArray(node)) {
    return node.map(getText).join('');
  }

  return getText(node.children);
}

async function importFresh(modulePath, suffix) {
  importCounter += 1;
  return import(`${modulePath}?${suffix}-${importCounter}`);
}

async function renderElement(element) {
  let renderer;

  await act(async () => {
    renderer = create(element);
    await Promise.resolve();
    await Promise.resolve();
  });

  return renderer;
}

beforeAll(() => {
  installBrowserShims();
});

beforeEach(() => {
  globalThis.localStorage.clear();
  globalThis.sessionStorage.clear();
});

afterEach(() => {
  mock.restore();
});

describe('App pricing config handoff', () => {
  test('keeps marketplace routes closed until status is loaded', async () => {
    mock.module('../src/components/layout/SetupCheck.jsx', () => ({
      default: ({ children }) => children,
    }));
    mock.module('../src/components/common/ui/Loading.jsx', () => ({
      default: () => h('div', null, 'LOADING'),
    }));
    mock.module('../src/helpers/lazyWithRetry.js', () => ({
      lazyWithRetry: (_load, key) => {
        if (key === 'public-routes') {
          return ({ pricingEnabled, pricingRequireAuth }) =>
            h(
              'div',
              null,
              `pricingEnabled:${String(pricingEnabled)};pricingRequireAuth:${String(pricingRequireAuth)}`,
            );
        }

        return () => h('div', null, `route:${key}`);
      },
    }));

    const { default: App } = await importFresh('../src/App.jsx', 'app-no-status');
    const renderer = await renderElement(
      h(
        MemoryRouter,
        { initialEntries: ['/pricing'] },
        h(
          StatusContext.Provider,
          { value: [{ status: undefined }, () => {}] },
          h(App),
        ),
      ),
    );

    expect(getText(renderer.toJSON())).toContain('pricingEnabled:false');
    expect(getText(renderer.toJSON())).toContain('pricingRequireAuth:false');
  });

  test('passes through loaded marketplace config', async () => {
    mock.module('../src/components/layout/SetupCheck.jsx', () => ({
      default: ({ children }) => children,
    }));
    mock.module('../src/components/common/ui/Loading.jsx', () => ({
      default: () => h('div', null, 'LOADING'),
    }));
    mock.module('../src/helpers/lazyWithRetry.js', () => ({
      lazyWithRetry: (_load, key) => {
        if (key === 'public-routes') {
          return ({ pricingEnabled, pricingRequireAuth }) =>
            h(
              'div',
              null,
              `pricingEnabled:${String(pricingEnabled)};pricingRequireAuth:${String(pricingRequireAuth)}`,
            );
        }

        return () => h('div', null, `route:${key}`);
      },
    }));

    const { default: App } = await importFresh('../src/App.jsx', 'app-loaded-status');
    const renderer = await renderElement(
      h(
        MemoryRouter,
        { initialEntries: ['/pricing'] },
        h(
          StatusContext.Provider,
          {
            value: [
              {
                status: {
                  HeaderNavModules: {
                    pricing: {
                      enabled: true,
                      requireAuth: true,
                    },
                  },
                },
              },
              () => {},
            ],
          },
          h(App),
        ),
      ),
    );

    expect(getText(renderer.toJSON())).toContain('pricingEnabled:true');
    expect(getText(renderer.toJSON())).toContain('pricingRequireAuth:true');
  });
});

describe('PublicRoutes pricing behavior', () => {
  async function renderPricingRoute(props) {
    mock.module('../src/components/common/ui/Loading.jsx', () => ({
      default: () => h('div', null, 'LOADING'),
    }));
    mock.module('../src/helpers/lazyWithRetry.js', () => ({
      lazyWithRetry: (_load, key) => {
        if (key === 'not-found-route') {
          return () => h('div', null, 'NOT_FOUND');
        }
        if (key === 'pricing-route') {
          return () => h('div', null, 'PRICING_PAGE');
        }
        if (key === 'login-route') {
          return () => h('div', null, 'LOGIN_PAGE');
        }

        return () => h('div', null, `route:${key}`);
      },
    }));

    const { default: PublicRoutes } = await importFresh(
      '../src/routes/PublicRoutes.jsx',
      'public-routes',
    );

    return renderElement(
      h(
        MemoryRouter,
        { initialEntries: ['/pricing'] },
        h(PublicRoutes, props),
      ),
    );
  }

  test('renders NotFound when pricing is disabled', async () => {
    const renderer = await renderPricingRoute({ pricingEnabled: false });

    expect(getText(renderer.toJSON())).toBe('NOT_FOUND');
  });

  test('redirects unauthenticated users to login when pricing requires auth', async () => {
    const renderer = await renderPricingRoute({
      pricingEnabled: true,
      pricingRequireAuth: true,
    });

    expect(getText(renderer.toJSON())).toBe('LOGIN_PAGE');
  });

  test('renders pricing when enabled without auth requirement', async () => {
    const renderer = await renderPricingRoute({
      pricingEnabled: true,
      pricingRequireAuth: false,
    });

    expect(getText(renderer.toJSON())).toBe('PRICING_PAGE');
  });
});
