import React from 'react';
import {
  afterAll,
  afterEach,
  beforeAll,
  beforeEach,
  describe,
  expect,
  mock,
  test,
} from 'bun:test';
import { act, create } from 'react-test-renderer';
import { MemoryRouter } from 'react-router-dom';
import { StatusContext } from '../src/context/Status/index.jsx';

let importCounter = 0;
const h = React.createElement;
const SHIMMED_GLOBAL_KEYS = [
  'IS_REACT_ACT_ENVIRONMENT',
  'document',
  'localStorage',
  'navigator',
  'sessionStorage',
  'window',
];
let restoreBrowserGlobals = () => {};

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
  const originalGlobals = new Map(
    SHIMMED_GLOBAL_KEYS.map((key) => [key, Object.getOwnPropertyDescriptor(globalThis, key)]),
  );
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
    querySelector() {
      return null;
    },
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

  return () => {
    for (const [key, descriptor] of originalGlobals.entries()) {
      if (descriptor) {
        Object.defineProperty(globalThis, key, descriptor);
      } else {
        delete globalThis[key];
      }
    }
  };
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
    for (let index = 0; index < 5; index += 1) {
      await Promise.resolve();
    }
  });

  return renderer;
}

beforeAll(() => {
  restoreBrowserGlobals = installBrowserShims();
});

beforeEach(() => {
  globalThis.localStorage.clear();
  globalThis.sessionStorage.clear();
});

afterEach(() => {
  mock.restore();
});

afterAll(() => {
  restoreBrowserGlobals();
});

describe('browser shim lifecycle', () => {
  test('restores the original global objects after shimming', () => {
    const originalWindow = globalThis.window;
    const originalDocument = globalThis.document;
    const restoreGlobals = installBrowserShims();

    expect(globalThis.window).not.toBe(originalWindow);
    expect(globalThis.document).not.toBe(originalDocument);

    restoreGlobals();

    expect(globalThis.window).toBe(originalWindow);
    expect(globalThis.document).toBe(originalDocument);
  });
});

describe('PublicRoutes pricing behavior', () => {
  async function renderPricingRoute(props) {
    mock.module('../src/components/common/ui/Loading.jsx', () => ({
      default: () => h('div', null, 'LOADING'),
    }));
    mock.module('../src/pages/NotFound/index.jsx', () => ({
      default: () => h('div', null, 'NOT_FOUND'),
    }));
    mock.module('../src/pages/Pricing/index.jsx', () => ({
      default: () => h('div', null, 'PRICING_PAGE'),
    }));
    mock.module('../src/components/auth/LoginForm.jsx', () => ({
      default: () => h('div', null, 'LOGIN_PAGE'),
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

describe('marketing header pricing visibility', () => {
  async function renderMarketingHeader(status) {
    const actualUseIsMobile = await import('../src/hooks/common/useIsMobile.js');
    const actualLanguage = await import('../src/i18n/language.js');

    mock.module('react-i18next', () => ({
      useTranslation: () => ({
        t: (value) => value,
        i18n: {
          language: 'zh-CN',
          changeLanguage() {},
          off() {},
          on() {},
        },
      }),
    }));
    mock.module('../src/context/Theme/index.jsx', () => ({
      useActualTheme: () => 'light',
      useSetTheme: () => () => {},
      useTheme: () => 'light',
    }));
    mock.module('../src/helpers/branding.js', () => ({
      getLogo: () => '',
    }));
    mock.module('../src/helpers/notifications.js', () => ({
      showSuccess() {},
    }));
    mock.module('../src/hooks/common/useIsMobile.js', () => ({
      ...actualUseIsMobile,
      useIsMobile: () => false,
    }));
    mock.module('../src/hooks/common/useMinimumLoadingTime.js', () => ({
      useMinimumLoadingTime: (value) => value,
    }));
    mock.module('../src/hooks/common/useNotifications.js', () => ({
      useNotifications: () => ({
        getUnreadKeys: () => [],
        handleNoticeClose() {},
        handleNoticeOpen() {},
        noticeVisible: false,
        unreadCount: 0,
      }),
    }));
    mock.module('../src/hooks/common/useSidebarCollapsed.js', () => ({
      useSidebarCollapsed: () => [false, () => {}],
    }));
    mock.module('../src/i18n/i18n.js', () => ({
      ensureLanguageResources: async () => {},
    }));
    mock.module('../src/i18n/language.js', () => ({
      ...actualLanguage,
      normalizeLanguage: (value) => value,
    }));

    const { default: MarketingHeaderBar } = await importFresh(
      '../src/components/layout/MarketingHeaderBar.jsx',
      'marketing-header',
    );
    const { UserContext } = await import('../src/context/User/index.jsx');

    return renderElement(
      h(
        MemoryRouter,
        { initialEntries: ['/'] },
        h(
          UserContext.Provider,
          { value: [{ user: null }, () => {}] },
          h(
            StatusContext.Provider,
            { value: [{ status }, () => {}] },
            h(MarketingHeaderBar),
          ),
        ),
      ),
    );
  }

  test('hides the pricing link while status is still loading', async () => {
    const renderer = await renderMarketingHeader(undefined);
    const renderedText = getText(renderer.toJSON());

    expect(renderedText).toContain('首页');
    expect(renderedText).not.toContain('模型广场');
  });

  test('shows the pricing link once status enables the module', async () => {
    const renderer = await renderMarketingHeader({
      HeaderNavModules: {
        pricing: {
          enabled: true,
          requireAuth: false,
        },
      },
      self_use_mode_enabled: false,
    });
    const renderedText = getText(renderer.toJSON());

    expect(renderedText).toContain('模型广场');
  });
});

describe('App pricing config handoff', () => {
  async function renderApp(status) {
    const mockPublicRoutes = ({ pricingEnabled, pricingRequireAuth }) =>
      h(
        'div',
        null,
        `pricingEnabled:${String(pricingEnabled)};pricingRequireAuth:${String(pricingRequireAuth)}`,
      );

    mock.module('../src/components/layout/SetupCheck.jsx', () => ({
      default: ({ children }) => children,
    }));
    mock.module('../src/components/common/ui/Loading.jsx', () => ({
      default: () => h('div', null, 'LOADING'),
    }));
    mock.module('../src/routes/PublicRoutes.jsx', () => ({
      default: mockPublicRoutes,
    }));

    const { default: App } = await importFresh('../src/App.jsx', 'app');
    return renderElement(
      h(
        MemoryRouter,
        { initialEntries: ['/pricing'] },
        h(
          StatusContext.Provider,
          { value: [{ status }, () => {}] },
          h(App),
        ),
      ),
    );
  }

  test('keeps marketplace routes closed until status is loaded', async () => {
    const renderer = await renderApp(undefined);

    expect(getText(renderer.toJSON())).toContain('pricingEnabled:false');
    expect(getText(renderer.toJSON())).toContain('pricingRequireAuth:false');
  });

  test('passes through loaded marketplace config', async () => {
    const renderer = await renderApp({
      HeaderNavModules: {
        pricing: {
          enabled: true,
          requireAuth: true,
        },
      },
    });

    expect(getText(renderer.toJSON())).toContain('pricingEnabled:true');
    expect(getText(renderer.toJSON())).toContain('pricingRequireAuth:true');
  });
});
