import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const appPath = new URL('../src/App.jsx', import.meta.url);
const siderBarPath = new URL(
  '../src/components/layout/SiderBar.jsx',
  import.meta.url,
);
const pageLayoutPath = new URL(
  '../src/components/layout/PageLayout.jsx',
  import.meta.url,
);
const noticeModalPath = new URL(
  '../src/components/layout/NoticeModal.jsx',
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
const i18nPath = new URL('../src/i18n/i18n.js', import.meta.url);
const headersFilePath = new URL('../public/_headers', import.meta.url);
const dashboardPath = new URL(
  '../src/components/dashboard/index.jsx',
  import.meta.url,
);
const statsCardsPath = new URL(
  '../src/components/dashboard/StatsCards.jsx',
  import.meta.url,
);
const miniTrendSparklinePath = new URL(
  '../src/components/dashboard/MiniTrendSparkline.jsx',
  import.meta.url,
);
const chartsPanelPath = new URL(
  '../src/components/dashboard/ChartsPanel.jsx',
  import.meta.url,
);
const dashboardChartsHookPath = new URL(
  '../src/hooks/dashboard/useDashboardCharts.jsx',
  import.meta.url,
);
const lazyVChartPath = new URL(
  '../src/components/dashboard/LazyVChart.jsx',
  import.meta.url,
);
const dashboardRuntimePath = new URL(
  '../src/components/dashboard/vchartDashboardRuntime.js',
  import.meta.url,
);
const markdownRendererPath = new URL(
  '../src/components/common/markdown/MarkdownRenderer.jsx',
  import.meta.url,
);
const consoleRoutesPath = new URL('../src/routes/ConsoleRoutes.jsx', import.meta.url);
const publicRoutesPath = new URL('../src/routes/PublicRoutes.jsx', import.meta.url);
const headerBarHookPath = new URL(
  '../src/hooks/common/useHeaderBar.js',
  import.meta.url,
);

test('app entry keeps public routes in the startup bundle while lazy loading the console route group', async () => {
  const appSource = await readFile(appPath, 'utf8');
  const consoleSource = await readFile(consoleRoutesPath, 'utf8');
  const publicSource = await readFile(publicRoutesPath, 'utf8');

  assert.match(
    appSource,
    /const ConsoleRoutes = lazyWithRetry\([\s\S]*import\('\.\/routes\/ConsoleRoutes'\),/,
  );
  assert.match(
    appSource,
    /import PublicRoutes from '\.\/routes\/PublicRoutes';/,
  );
  assert.doesNotMatch(
    appSource,
    /const PublicRoutes = lazyWithRetry\([\s\S]*import\('\.\/routes\/PublicRoutes'\),/,
  );
  assert.doesNotMatch(appSource, /import\('\.\/pages\/Setting'\)/);
  assert.doesNotMatch(appSource, /import\('\.\/pages\/Playground'\)/);
  assert.match(
    consoleSource,
    /const Playground = lazyWithRetry\([\s\S]*import\('\.\.\/pages\/Playground'\),/,
  );
  assert.match(
    consoleSource,
    /const Setting = lazyWithRetry\([\s\S]*import\('\.\.\/pages\/Setting'\),/,
  );
  assert.match(
    consoleSource,
    /const ModelPage = lazyWithRetry\([\s\S]*import\('\.\.\/pages\/Model'\),/,
  );
  assert.match(
    consoleSource,
    /const Token = lazyWithRetry\([\s\S]*import\('\.\.\/pages\/Token'\),/,
  );
  assert.match(
    consoleSource,
    /const Chat = lazyWithRetry\([\s\S]*import\('\.\.\/pages\/Chat'\),/,
  );
  assert.match(
    publicSource,
    /const LoginForm = lazyWithRetry\([\s\S]*import\('\.\.\/components\/auth\/LoginForm'\),/,
  );
  assert.match(
    publicSource,
    /const RegisterForm = lazyWithRetry\([\s\S]*import\('\.\.\/components\/auth\/RegisterForm'\),/,
  );
});

test('vite build keeps only safe heavy dependencies in dedicated chunks', async () => {
  const source = await readFile(viteConfigPath, 'utf8');
  const packageJson = JSON.parse(await readFile(packageJsonPath, 'utf8'));

  assert.match(source, /manualChunks:\s*(buildManualChunkName|\()/);
  assert.match(source, /HOME_DEFERRED_PRELOAD_PATTERN = \/\(\?:semi-core\|visactor\|data-viz\)-\//);
  assert.match(source, /return 'seasonal-effects';/);
  assert.match(source, /return 'semi-vendor';/);
  assert.match(source, /onwarn:\s*(handleBuildWarning|\()/);
  assert.match(source, /lottie-web\/build\/player\/lottie\.js/);
  assert.match(source, /chunkSizeWarningLimit:\s*3500/);
  assert.match(packageJson.scripts.build, /BROWSERSLIST_IGNORE_OLD_DATA=true/);
});

test('vite build groups the noisy semi and visactor vendor graphs while keeping startup routing stable', async () => {
  const source = await readFile(viteConfigPath, 'utf8');
  const reactCoreIndex = source.indexOf("return 'react-core';");

  assert.match(
    source,
    /id\.includes\('react-router-dom'\)[\s\S]*id\.includes\('\/react\/'\)[\s\S]*id\.includes\('scheduler'\)[\s\S]*id\.includes\('i18next'\)[\s\S]*return 'react-core';/,
  );
  assert.match(source, /id\.includes\('@douyinfe\/semi'\)[\s\S]*return 'semi-vendor';/);
  assert.doesNotMatch(source, /id\.includes\('lucide-react'\)[\s\S]*return 'react-core';/);
  assert.doesNotMatch(source, /id\.includes\('i18next'\)[\s\S]*return 'i18n';/);
  assert.doesNotMatch(source, /return 'rich-content';/);
  assert.notEqual(reactCoreIndex, -1);
});

test('vite build inlines the startup stylesheet into index.html to avoid an extra round trip on first paint', async () => {
  const source = await readFile(viteConfigPath, 'utf8');

  assert.match(source, /const DEFAULT_ASSET_BASE_URL = '\/';/);
  assert.match(source, /function normalizeAssetBaseUrl\(value\)/);
  assert.match(source, /const assetBaseUrl = normalizeAssetBaseUrl\(process\.env\.VITE_ASSET_BASE_URL\);/);
  assert.match(source, /base:\s*assetBaseUrl,/);
  assert.match(source, /name:\s*'inline-startup-styles'/);
  assert.match(source, /transformIndexHtml:\s*\{/);
  assert.match(source, /const STARTUP_STYLE_FILE_PREFIX = 'assets\/index-';/);
  assert.match(source, /const startupStyleHref = `\$\{assetBaseUrl\}\$\{startupStyle\.fileName\}`;/);
  assert.match(source, /asset\.fileName\.startsWith\(STARTUP_STYLE_FILE_PREFIX\)/);
  assert.match(source, /asset\.fileName\.endsWith\('\.css'\)/);
  assert.match(source, /<style data-inline-startup-css>/);
  assert.doesNotMatch(source, /<link rel="stylesheet" crossorigin href="\\\/\$\{startupStyle\.fileName\}">/);
});

test('static asset deployments define Cloudflare custom headers for cross-origin module loading', async () => {
  const source = await readFile(headersFilePath, 'utf8');

  assert.match(source, /\/assets\/\*/);
  assert.match(source, /Access-Control-Allow-Origin:\s*\*/);
  assert.match(source, /Cache-Control:\s*public,\s*max-age=31536000,\s*immutable/);
});

test('sidebar startup shell imports lightweight icons helper instead of the heavy render module', async () => {
  const source = await readFile(siderBarPath, 'utf8');

  assert.match(source, /from '\.\.\/\.\.\/helpers\/sidebarIcons';/);
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

test('startup shell avoids the heavyweight axios helper and lazy loads markdown parsing for the home page', async () => {
  const pageLayoutSource = await readFile(pageLayoutPath, 'utf8');
  const homeSource = await readFile(homePath, 'utf8');
  const headerBarHookSource = await readFile(headerBarHookPath, 'utf8');

  assert.doesNotMatch(pageLayoutSource, /from '\.\.\/\.\.\/helpers\/api';/);
  assert.doesNotMatch(homeSource, /from '\.\.\/\.\.\/helpers\/api';/);
  assert.doesNotMatch(headerBarHookSource, /from '\.\.\/\.\.\/helpers\/api';/);
  assert.doesNotMatch(homeSource, /from 'marked';/);
  assert.match(homeSource, /await import\('marked'\)/);
  assert.match(headerBarHookSource, /await import\('\.\.\/\.\.\/helpers\/api'\)/);
});

test('shared render helpers do not statically import rich-content traversal utilities', async () => {
  const source = await readFile(renderHelperPath, 'utf8');

  assert.doesNotMatch(source, /from 'unist-util-visit';/);
});

test('vite build does not group @lobehub icons into one oversized manual chunk', async () => {
  const source = await readFile(viteConfigPath, 'utf8');

  assert.doesNotMatch(source, /id\.includes\('@lobehub\/icons'\)/);
  assert.doesNotMatch(source, /brand-icons/);
});

test('vite build does not carve markdown dependencies into a dedicated startup chunk', async () => {
  const source = await readFile(viteConfigPath, 'utf8');

  assert.doesNotMatch(source, /return 'rich-content';/);
});

test('vite build keeps axios isolated without forcing history into its own startup chunk', async () => {
  const source = await readFile(viteConfigPath, 'utf8');

  assert.match(source, /id\.includes\('axios'\)[\s\S]*return 'api-client';/);
  assert.doesNotMatch(source, /id\.includes\('\/history\/'\)[\s\S]*return 'history';/);
  assert.match(source, /id\.includes\('\/marked\/'\)[\s\S]*return 'markdown-runtime';/);
});

test('i18n preloads zh-CN while keeping the non-default locales lazy', async () => {
  const source = await readFile(i18nPath, 'utf8');

  assert.doesNotMatch(source, /from 'i18next-browser-languagedetector';/);
  assert.doesNotMatch(source, /\.use\(LanguageDetector\)/);
  assert.match(source, /import zhCNTranslation from '\.\/locales\/zh-CN\.json';/);
  assert.match(source, /const localeLoaders = \{/);
  assert.doesNotMatch(source, /import\.meta\.glob\('\.\/locales\/\*\.json'\)/);
  assert.doesNotMatch(source, /'zh-CN': \(\) => import\('\.\/locales\/zh-CN\.json'\)/);
  assert.match(source, /en: \(\) => import\('\.\/locales\/en\.json'\)/);
  assert.match(source, /'zh-TW': \(\) => import\('\.\/locales\/zh-TW\.json'\)/);
  assert.match(source, /const defaultLanguageMessages = getTranslationMessages\(zhCNTranslation\);/);
  assert.match(source, /resources:\s*defaultLanguageMessages\s*\?\s*\{\s*\[DEFAULT_LANGUAGE\]:\s*\{\s*translation:\s*defaultLanguageMessages,/);
  assert.match(source, /const loadedLanguages = new Set\(defaultLanguageMessages \? \[DEFAULT_LANGUAGE\] : \[\]\);/);
});

test('render helper lazy loads lobe icons instead of importing the full icon registry upfront', async () => {
  const source = await readFile(renderHelperPath, 'utf8');

  assert.doesNotMatch(source, /from '@lobehub\/icons';/);
  assert.match(source, /from '@lobehub\/icons\/es\/OpenAI';/);
  assert.match(source, /const staticLobeIconRegistry = \{/);
  assert.match(
    source,
    /import\.meta\.glob\([\s\S]*\.\.\/\.\.\/node_modules\/@lobehub\/icons\/es\/\*\/index\.js[\s\S]*\)/,
  );
  assert.match(
    source,
    /!\.\.\/\.\.\/node_modules\/@lobehub\/icons\/es\/\{Ai360,Claude/,
  );
  assert.match(source, /function DynamicLobeHubIcon\(/);
});

test('dashboard route lazy loads the search modal instead of bundling it into the main dashboard chunk', async () => {
  const source = await readFile(dashboardPath, 'utf8');

  assert.match(
    source,
    /const SearchModal = lazy\(\(\) => import\('\.\/modals\/SearchModal'\)\);/,
  );
  assert.match(source, /<Suspense fallback=\{null\}>[\s\S]*<SearchModal/);
});

test('dashboard chart runtime deep imports the lightweight VChartSimple entry and only boots visactor after the chart placeholder becomes visible', async () => {
  const lazyVChartSource = await readFile(lazyVChartPath, 'utf8');
  const dashboardChartsHookSource = await readFile(
    dashboardChartsHookPath,
    'utf8',
  );

  assert.match(
    lazyVChartSource,
    /import\('@visactor\/react-vchart\/esm\/VChartSimple\.js'\)/,
  );
  assert.match(lazyVChartSource, /import\('@visactor\/vchart-semi-theme'\)/);
  assert.match(lazyVChartSource, /IntersectionObserver/);
  assert.match(lazyVChartSource, /rootMargin: '200px'/);
  assert.doesNotMatch(lazyVChartSource, /import\('@visactor\/react-vchart'\)/);
  assert.doesNotMatch(dashboardChartsHookSource, /ensureVChartSemiTheme/);
  assert.doesNotMatch(
    dashboardChartsHookSource,
    /import\('@visactor\/vchart-semi-theme'\)/,
  );
});

test('dashboard route keeps its modal lazy and moves the heavyweight chart analysis surface behind an explicit lazy gate', async () => {
  const source = await readFile(dashboardPath, 'utf8');

  assert.match(
    source,
    /import StatsCards from '\.\/StatsCards';/,
  );
  assert.match(
    source,
    /const ChartsPanel = lazyWithRetry\([\s\S]*import\('\.\/ChartsPanel'\),[\s\S]*'dashboard-charts-panel',/,
  );
  assert.match(
    source,
    /import ApiInfoPanel from '\.\/ApiInfoPanel';/,
  );
  assert.match(
    source,
    /import AnnouncementsPanel from '\.\/AnnouncementsPanel';/,
  );
  assert.match(
    source,
    /import FaqPanel from '\.\/FaqPanel';/,
  );
  assert.match(
    source,
    /import UptimePanel from '\.\/UptimePanel';/,
  );
  assert.doesNotMatch(source, /const StatsCards = lazy/);
  assert.doesNotMatch(source, /import ChartsPanel from '\.\/ChartsPanel';/);
  assert.match(source, /const \[chartsPanelEnabled, setChartsPanelEnabled\] = useState\(false\);/);
  assert.match(source, /onClick=\{\(\) => setChartsPanelEnabled\(true\)\}/);
  assert.match(source, /import \{ getRelativeTime \} from '\.\.\/\.\.\/helpers\/time';/);
});

test('dashboard chart panels use the lazy VChart wrapper while stats cards render with an inline sparkline instead of visactor', async () => {
  const [
    statsSource,
    miniTrendSource,
    chartsSource,
    lazyVChartSource,
    dashboardRuntimeSource,
  ] =
    await Promise.all([
      readFile(statsCardsPath, 'utf8'),
      readFile(miniTrendSparklinePath, 'utf8'),
      readFile(chartsPanelPath, 'utf8'),
      readFile(lazyVChartPath, 'utf8'),
      readFile(dashboardRuntimePath, 'utf8'),
    ]);

  assert.doesNotMatch(statsSource, /from '@visactor\/react-vchart';/);
  assert.doesNotMatch(chartsSource, /from '@visactor\/react-vchart';/);
  assert.doesNotMatch(statsSource, /from '\.\/LazyVChart';/);
  assert.match(statsSource, /from '\.\/MiniTrendSparkline';/);
  assert.match(chartsSource, /from '\.\/LazyVChart';/);
  assert.match(miniTrendSource, /function buildSparklinePoints\(/);
  assert.match(miniTrendSource, /<polyline/);
  assert.match(
    lazyVChartSource,
    /import\('@visactor\/react-vchart\/esm\/VChartSimple\.js'\)/,
  );
  assert.match(lazyVChartSource, /import\('\.\/vchartDashboardRuntime'\)/);
  assert.match(lazyVChartSource, /import\('@visactor\/vchart-semi-theme'\)/);
  assert.match(lazyVChartSource, /<module\.VChartSimple/);
  assert.match(lazyVChartSource, /vchartConstrouctor=\{/);
  assert.match(lazyVChartSource, /IntersectionObserver/);
  assert.match(dashboardRuntimeSource, /registerLineChart/);
  assert.match(dashboardRuntimeSource, /registerPieChart/);
  assert.doesNotMatch(dashboardRuntimeSource, /registerLabel/);
});

test('dashboard only loads admin user chart data after the heavy chart surface is enabled on user-specific tabs', async () => {
  const source = await readFile(dashboardPath, 'utf8');

  assert.match(source, /const shouldLoadAdminUserCharts =[\s\S]*\['5', '6'\]\.includes\(dashboardData\.activeChartTab\);/);
  assert.match(source, /if \(!force && adminUserChartCacheKeyRef\.current === userChartCacheKey\) \{/);
  assert.match(source, /if \(shouldLoadAdminUserCharts\) \{\s*await loadUserData\(\{ force: true \}\);/);
  assert.match(source, /useEffect\(\(\) => \{\s*if \(!shouldLoadAdminUserCharts\) \{\s*return;\s*\}\s*void loadUserData\(\);\s*\}, \[loadUserData, shouldLoadAdminUserCharts\]\);/);
});

test('dashboard chart hook no longer preloads the visactor theme before the chart runtime is requested', async () => {
  const source = await readFile(dashboardChartsHookPath, 'utf8');
  const pieSpecSection = source.slice(
    source.indexOf('const [spec_pie'),
    source.indexOf('const [spec_line'),
  );

  assert.doesNotMatch(source, /from '@visactor\/vchart-semi-theme';/);
  assert.doesNotMatch(source, /import\('@visactor\/vchart-semi-theme'\)/);
  assert.doesNotMatch(source, /function ensureVChartSemiTheme\(/);
  assert.doesNotMatch(source, /position:\s*'outside'/);
  assert.doesNotMatch(pieSpecSection, /label:\s*\{\s*visible:\s*true/);
});

test('markdown renderer lazy loads mermaid, code highlighting, and math plugins instead of bundling them upfront', async () => {
  const source = await readFile(markdownRendererPath, 'utf8');

  assert.doesNotMatch(source, /import mermaid from 'mermaid';/);
  assert.doesNotMatch(source, /import RemarkMath from 'remark-math';/);
  assert.doesNotMatch(source, /import RehypeKatex from 'rehype-katex';/);
  assert.doesNotMatch(
    source,
    /import RehypeHighlight from 'rehype-highlight';/,
  );
  assert.match(source, /import\('mermaid'\)/);
  assert.match(source, /import\('remark-math'\)/);
  assert.match(source, /import\('rehype-katex'\)/);
  assert.match(source, /import\('rehype-highlight'\)/);
  assert.match(source, /function detectMarkdownFeatures\(/);
});
