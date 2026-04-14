import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const appPath = new URL('../src/App.jsx', import.meta.url);
const indexPath = new URL('../src/index.jsx', import.meta.url);
const i18nPath = new URL('../src/i18n/i18n.js', import.meta.url);
const siderBarPath = new URL(
  '../src/components/layout/SiderBar.jsx',
  import.meta.url,
);
const pageLayoutPath = new URL(
  '../src/components/layout/PageLayout.jsx',
  import.meta.url,
);
const marketingHeaderPath = new URL(
  '../src/components/layout/MarketingHeaderBar.jsx',
  import.meta.url,
);
const semiRuntimePath = new URL(
  '../src/components/common/SemiRuntime.jsx',
  import.meta.url,
);
const noticeModalPath = new URL(
  '../src/components/layout/NoticeModal.jsx',
  import.meta.url,
);
const errorBoundaryPath = new URL(
  '../src/components/common/ErrorBoundary.jsx',
  import.meta.url,
);
const loadingPath = new URL(
  '../src/components/common/ui/Loading.jsx',
  import.meta.url,
);
const footerPath = new URL(
  '../src/components/layout/Footer.jsx',
  import.meta.url,
);
const userAreaPath = new URL(
  '../src/components/layout/headerbar/UserArea.jsx',
  import.meta.url,
);
const homePath = new URL('../src/pages/Home/index.jsx', import.meta.url);
const renderHelperPath = new URL('../src/helpers/render.jsx', import.meta.url);
const viteConfigPath = new URL('../vite.config.js', import.meta.url);
const packageJsonPath = new URL('../package.json', import.meta.url);
const useHeaderBarPath = new URL(
  '../src/hooks/common/useHeaderBar.js',
  import.meta.url,
);
const headerBarPath = new URL(
  '../src/components/layout/headerbar/index.jsx',
  import.meta.url,
);
const apiHelperPath = new URL('../src/helpers/api.js', import.meta.url);

test('heavy console routes are lazy loaded to keep the app entry chunk small', async () => {
  const appSource = await readFile(appPath, 'utf8');

  assert.match(appSource, /from '\.\/helpers\/auth';/);
  assert.doesNotMatch(appSource, /const AppRoutes = lazy\(/);
  assert.doesNotMatch(appSource, /from '\.\/helpers';/);
  assert.match(
    appSource,
    /const InviteRebate = lazy\(\(\) => import\('\.\/pages\/InviteRebate'\)\);/,
  );
  assert.match(
    appSource,
    /const Playground = lazy\(\(\) => import\('\.\/pages\/Playground'\)\);/,
  );
  assert.match(
    appSource,
    /const Setting = lazy\(\(\) => import\('\.\/pages\/Setting'\)\);/,
  );
  assert.match(
    appSource,
    /const ModelPage = lazy\(\(\) => import\('\.\/pages\/Model'\)\);/,
  );
  assert.match(
    appSource,
    /const Token = lazy\(\(\) => import\('\.\/pages\/Token'\)\);/,
  );
  assert.match(
    appSource,
    /const Chat = lazy\(\(\) => import\('\.\/pages\/Chat'\)\);/,
  );
  assert.match(
    appSource,
    /path='\/console\/invite\/rebate'[\s\S]*<PrivateRoute>\{renderWithSuspense\(<InviteRebate \/>\)\}<\/PrivateRoute>/,
  );
});

test('root entry avoids importing semi runtime directly so the homepage can boot without semi-ui', async () => {
  const source = await readFile(indexPath, 'utf8');

  assert.doesNotMatch(source, /@douyinfe\/semi-ui/);
  assert.doesNotMatch(source, /LocaleProvider/);
});

test('i18n boot keeps only the default locale bundled and lazy loads other locale packs', async () => {
  const [indexSource, i18nSource] = await Promise.all([
    readFile(indexPath, 'utf8'),
    readFile(i18nPath, 'utf8'),
  ]);

  assert.match(indexSource, /initializeI18n/);
  assert.match(i18nSource, /import\.meta\.glob\('\.\/locales\/\*\.json'\)/);
  assert.match(
    i18nSource,
    /import zhCNTranslation from '\.\/locales\/zh-CN\.json';/,
  );
  assert.doesNotMatch(
    i18nSource,
    /import enTranslation from '\.\/locales\/en\.json';/,
  );
  assert.doesNotMatch(
    i18nSource,
    /import frTranslation from '\.\/locales\/fr\.json';/,
  );
  assert.doesNotMatch(
    i18nSource,
    /import zhTWTranslation from '\.\/locales\/zh-TW\.json';/,
  );
  assert.doesNotMatch(
    i18nSource,
    /import ruTranslation from '\.\/locales\/ru\.json';/,
  );
  assert.doesNotMatch(
    i18nSource,
    /import jaTranslation from '\.\/locales\/ja\.json';/,
  );
  assert.doesNotMatch(
    i18nSource,
    /import viTranslation from '\.\/locales\/vi\.json';/,
  );
});

test('header bar startup hook imports only focused helper modules', async () => {
  const source = await readFile(useHeaderBarPath, 'utf8');

  assert.match(source, /from '\.\.\/\.\.\/helpers\/api';/);
  assert.match(source, /from '\.\.\/\.\.\/helpers\/branding';/);
  assert.match(source, /from '\.\.\/\.\.\/helpers\/notifications';/);
  assert.doesNotMatch(source, /from '\.\.\/\.\.\/helpers';/);
});

test('vite build keeps only safe heavy dependencies in dedicated chunks', async () => {
  const source = await readFile(viteConfigPath, 'utf8');
  const packageJson = JSON.parse(await readFile(packageJsonPath, 'utf8'));

  assert.match(source, /manualChunks:\s*(buildManualChunkName|\()/);
  assert.match(source, /modulePreload:\s*\{/);
  assert.match(source, /resolveDependencies\s*\(/);
  assert.match(source, /semi-core-/);
  assert.match(source, /return 'visactor';/);
  assert.match(source, /return 'seasonal-effects';/);
  assert.match(source, /onwarn:\s*(handleBuildWarning|\()/);
  assert.match(source, /lottie-web\/build\/player\/lottie\.js/);
  assert.match(source, /chunkSizeWarningLimit:\s*3500/);
  assert.match(packageJson.scripts.build, /BROWSERSLIST_IGNORE_OLD_DATA=true/);
});

test('vite build keeps only the safe dedicated chunks and avoids startup cycle chunks', async () => {
  const source = await readFile(viteConfigPath, 'utf8');
  const reactCoreIndex = source.indexOf("return 'react-core';");

  assert.match(source, /id\.includes\('i18next'\)[\s\S]*return 'react-core';/);
  assert.doesNotMatch(source, /id\.includes\('i18next'\)[\s\S]*return 'i18n';/);
  assert.doesNotMatch(
    source,
    /id\.includes\('@douyinfe\/semi-ui'\)[\s\S]*return 'react-core';/,
  );
  assert.doesNotMatch(
    source,
    /id\.includes\('@douyinfe\/semi-icons'\)[\s\S]*return 'react-core';/,
  );
  assert.match(
    source,
    /id\.includes\('@douyinfe\/semi-ui'\)[\s\S]*return 'semi-core';/,
  );
  assert.match(
    source,
    /id\.includes\('@douyinfe\/semi-icons'\)[\s\S]*return 'semi-core';/,
  );
  assert.match(
    source,
    /id\.includes\('lucide-react'\)[\s\S]*return 'react-core';/,
  );
  assert.doesNotMatch(source, /return 'brand-icons';/);
  assert.doesNotMatch(source, /return 'rich-content';/);
  assert.notEqual(reactCoreIndex, -1);
});

test('sidebar startup shell imports lightweight icons helper instead of the heavy render module', async () => {
  const source = await readFile(siderBarPath, 'utf8');

  assert.match(source, /from '\.\.\/\.\.\/helpers\/sidebarIcons';/);
  assert.match(source, /from '\.\.\/\.\.\/helpers\/utils';/);
  assert.doesNotMatch(
    source,
    /from '\.\.\/\.\.\/helpers\/notifications';/,
  );
  assert.doesNotMatch(source, /from '\.\.\/\.\.\/helpers\/session';/);
  assert.doesNotMatch(source, /from '\.\.\/\.\.\/helpers\/render';/);
});

test('startup shell files avoid the helpers barrel so render utilities stay out of the home route', async () => {
  const startupFiles = [
    pageLayoutPath,
    noticeModalPath,
    footerPath,
    userAreaPath,
    homePath,
    siderBarPath,
  ];

  await Promise.all(
    startupFiles.map(async (filePath) => {
      const source = await readFile(filePath, 'utf8');

      assert.doesNotMatch(source, /from '\.\.\/\.\.\/helpers';/);
      assert.doesNotMatch(source, /from '\.\.\/\.\.\/\.\.\/helpers';/);
    }),
  );
});

test('page layout uses a dedicated marketing shell and lazy loads the semi runtime only for non-home routes', async () => {
  const source = await readFile(pageLayoutPath, 'utf8');

  assert.match(source, /const SemiRuntime = lazy\(/);
  assert.match(source, /const ConsoleHeaderBar = lazy\(/);
  assert.match(source, /const ConsoleSiderBar = lazy\(/);
  assert.match(source, /from '\.\/MarketingHeaderBar';/);
  assert.match(
    source,
    /const sidebarWidth = collapsed[\s\S]*\? 'var\(--sidebar-width-collapsed\)'[\s\S]*: 'var\(--sidebar-width\)';/,
  );
  assert.doesNotMatch(source, /var\(--sidebar-current-width\)/);
  assert.doesNotMatch(source, /from '@douyinfe\/semi-ui';/);
  assert.doesNotMatch(source, /import HeaderBar from '\.\/headerbar';/);
  assert.doesNotMatch(source, /import SiderBar from '\.\/SiderBar';/);
});

test('sidebar component does not mutate body classes to drive collapsed layout width', async () => {
  const source = await readFile(siderBarPath, 'utf8');

  assert.doesNotMatch(source, /document\.body\.classList\.(add|remove)/);
  assert.match(
    source,
    /const sidebarWidth = collapsed[\s\S]*\? 'var\(--sidebar-width-collapsed\)'[\s\S]*: 'var\(--sidebar-width\)';/,
  );
});

test('marketing header avoids semi-ui so the homepage shell can stay lightweight', async () => {
  const source = await readFile(marketingHeaderPath, 'utf8');

  assert.doesNotMatch(source, /@douyinfe\/semi-ui/);
});

test('root loading and error fallback components avoid semi-ui so startup can stay on the lightweight shell', async () => {
  const [loadingSource, errorBoundarySource] = await Promise.all([
    readFile(loadingPath, 'utf8'),
    readFile(errorBoundaryPath, 'utf8'),
  ]);

  assert.doesNotMatch(loadingSource, /@douyinfe\/semi-ui/);
  assert.doesNotMatch(errorBoundarySource, /@douyinfe\/semi-ui/);
  assert.doesNotMatch(errorBoundarySource, /semi-illustrations/);
});

test('semi runtime owns semi-ui css and locale wiring for heavyweight routes', async () => {
  const source = await readFile(semiRuntimePath, 'utf8');

  assert.match(source, /@douyinfe\/semi-ui\/dist\/css\/semi\.css/);
  assert.match(source, /LocaleProvider/);
});

test('startup shell files avoid the legacy utils helper so homepage code only pulls focused helper modules', async () => {
  const startupFiles = [
    pageLayoutPath,
    noticeModalPath,
    footerPath,
    homePath,
    useHeaderBarPath,
  ];

  await Promise.all(
    startupFiles.map(async (filePath) => {
      const source = await readFile(filePath, 'utf8');

      assert.doesNotMatch(source, /helpers\/utils/);
    }),
  );
});

test('sidebar keeps using the tracked legacy utils helper until focused helper modules are committed', async () => {
  const source = await readFile(siderBarPath, 'utf8');

  assert.match(source, /from '\.\.\/\.\.\/helpers\/utils';/);
});

test('api helper imports focused session and notification helpers instead of the legacy utils bundle', async () => {
  const source = await readFile(apiHelperPath, 'utf8');

  assert.match(source, /from '\.\/session';/);
  assert.match(source, /from '\.\/notifications';/);
  assert.doesNotMatch(source, /from '\.\/utils';/);
});

test('notice modal is lazy loaded so startup shell avoids pulling announcement UI into the first paint', async () => {
  const [headerSource, homeSource] = await Promise.all([
    readFile(headerBarPath, 'utf8'),
    readFile(homePath, 'utf8'),
  ]);

  assert.match(headerSource, /const NoticeModal = lazy\(/);
  assert.doesNotMatch(
    headerSource,
    /import NoticeModal from '\.\.\/NoticeModal';/,
  );
  assert.match(homeSource, /const NoticeModal = lazy\(/);
  assert.doesNotMatch(
    homeSource,
    /import NoticeModal from '\.\.\/\.\.\/components\/layout\/NoticeModal';/,
  );
});

test('shared render helpers do not statically import rich-content traversal utilities', async () => {
  const source = await readFile(renderHelperPath, 'utf8');

  assert.doesNotMatch(source, /from 'unist-util-visit';/);
});

test('vite build does not carve @lobehub icons into a dedicated startup chunk', async () => {
  const source = await readFile(viteConfigPath, 'utf8');

  assert.doesNotMatch(source, /return 'brand-icons';/);
});

test('vite build does not carve markdown dependencies into a dedicated startup chunk', async () => {
  const source = await readFile(viteConfigPath, 'utf8');

  assert.doesNotMatch(source, /return 'rich-content';/);
});
