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
const marketingPageLayoutPath = new URL(
  '../src/components/layout/MarketingPageLayout.jsx',
  import.meta.url,
);
const consolePageLayoutPath = new URL(
  '../src/components/layout/ConsolePageLayout.jsx',
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
const apiInfoPanelPath = new URL(
  '../src/components/dashboard/ApiInfoPanel.jsx',
  import.meta.url,
);
const dashboardChartsHookPath = new URL(
  '../src/hooks/dashboard/useDashboardCharts.jsx',
  import.meta.url,
);
const dashboardStatsHookPath = new URL(
  '../src/hooks/dashboard/useDashboardStats.jsx',
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
const homeRoutesPath = new URL('../src/routes/HomeRoutes.jsx', import.meta.url);
const headerBarHookPath = new URL(
  '../src/hooks/common/useHeaderBar.js',
  import.meta.url,
);

test('app entry keeps only the home route group in the startup bundle while lazy loading console and non-home public route groups', async () => {
  const appSource = await readFile(appPath, 'utf8');
  const consoleSource = await readFile(consoleRoutesPath, 'utf8');
  const publicSource = await readFile(publicRoutesPath, 'utf8');
  const homeSource = await readFile(homeRoutesPath, 'utf8');

  assert.match(
    appSource,
    /const ConsoleRoutes = lazyWithRetry\([\s\S]*import\('\.\/routes\/ConsoleRoutes'\),/,
  );
  assert.match(
    appSource,
    /const PublicRoutes = lazyWithRetry\([\s\S]*import\('\.\/routes\/PublicRoutes'\),/,
  );
  assert.match(
    appSource,
    /import HomeRoutes from '\.\/routes\/HomeRoutes';/,
  );
  assert.doesNotMatch(
    appSource,
    /const HomeRoutes = lazyWithRetry\([\s\S]*import\('\.\/routes\/HomeRoutes'\),/,
  );
  assert.match(
    appSource,
    /const isHomeRoute = location\.pathname === '\/';/,
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
  assert.match(homeSource, /import Home from '\.\.\/pages\/Home';/);
  assert.doesNotMatch(
    homeSource,
    /lazyWithRetry\([\s\S]*LoginForm|lazyWithRetry\([\s\S]*RegisterForm|lazyWithRetry\([\s\S]*Pricing/,
  );
});

test('vite build keeps only safe heavy dependencies in dedicated chunks', async () => {
  const source = await readFile(viteConfigPath, 'utf8');
  const packageJson = JSON.parse(await readFile(packageJsonPath, 'utf8'));

  assert.match(source, /manualChunks:\s*(buildManualChunkName|\()/);
  assert.match(
    source,
    /HOME_DEFERRED_PRELOAD_PATTERN = \/\(\?:semi-core\|semi-icons\|visactor\|data-viz\)-\//,
  );
  assert.match(source, /return 'seasonal-effects';/);
  assert.doesNotMatch(source, /return 'semi-vendor';/);
  assert.match(source, /onwarn:\s*(handleBuildWarning|\()/);
  assert.match(source, /lottie-web\/build\/player\/lottie\.js/);
  assert.match(source, /chunkSizeWarningLimit:\s*3500/);
  assert.match(packageJson.scripts.build, /BROWSERSLIST_IGNORE_OLD_DATA=true/);
});

test('vite build keeps startup routing stable without forcing the entire Semi UI graph into one chunk', async () => {
  const source = await readFile(viteConfigPath, 'utf8');
  const reactCoreIndex = source.indexOf("return 'react-core';");

  assert.match(
    source,
    /id\.includes\('react-router-dom'\)[\s\S]*id\.includes\('\/react\/'\)[\s\S]*id\.includes\('scheduler'\)[\s\S]*id\.includes\('i18next'\)[\s\S]*return 'react-core';/,
  );
  assert.doesNotMatch(source, /id\.includes\('@douyinfe\/semi'\)[\s\S]*return 'semi-vendor';/);
  assert.doesNotMatch(source, /id\.includes\('@douyinfe\/semi-ui'\)[\s\S]*return 'semi-vendor';/);
  assert.doesNotMatch(source, /id\.includes\('@douyinfe\/semi-icons'\)[\s\S]*return 'semi-vendor';/);
  assert.doesNotMatch(source, /id\.includes\('lucide-react'\)[\s\S]*return 'react-core';/);
  assert.doesNotMatch(source, /id\.includes\('i18next'\)[\s\S]*return 'i18n';/);
  assert.doesNotMatch(source, /return 'rich-content';/);
  assert.notEqual(reactCoreIndex, -1);
});

test('vite build groups semi icons into one shared async chunk instead of many tiny icon requests', async () => {
  const source = await readFile(viteConfigPath, 'utf8');

  assert.match(
    source,
    /id\.includes\('@douyinfe\/semi-icons'\)[\s\S]*return 'semi-icons';/,
  );
  assert.match(
    source,
    /HOME_DEFERRED_PRELOAD_PATTERN = \/\(\?:semi-core\|semi-icons\|visactor\|data-viz\)-\//,
  );
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

test('page layout keeps the console chrome out of the startup shell by deferring the console layout module', async () => {
  const pageLayoutSource = await readFile(pageLayoutPath, 'utf8');
  const marketingPageLayoutSource = await readFile(marketingPageLayoutPath, 'utf8');
  const consolePageLayoutSource = await readFile(consolePageLayoutPath, 'utf8');

  assert.match(
    pageLayoutSource,
    /const ConsolePageLayout = lazyWithRetry\([\s\S]*import\('\.\/ConsolePageLayout'\),/,
  );
  assert.match(pageLayoutSource, /import MarketingPageLayout from '\.\/MarketingPageLayout';/);
  assert.doesNotMatch(pageLayoutSource, /sidebar-width-collapsed/);
  assert.doesNotMatch(pageLayoutSource, /ConsoleHeaderBar|ConsoleSiderBar|SemiRuntime/);
  assert.match(consolePageLayoutSource, /sidebar-width-collapsed/);
  assert.match(consolePageLayoutSource, /const ConsoleHeaderBar = lazy\(\(\) => import\('\.\/headerbar'\)\);/);
  assert.match(consolePageLayoutSource, /const ConsoleSiderBar = lazy\(\(\) => import\('\.\/SiderBar'\)\);/);
  assert.match(consolePageLayoutSource, /const SemiRuntime = lazy\(\(\) => import\('\.\.\/common\/SemiRuntime'\)\);/);
  assert.doesNotMatch(marketingPageLayoutSource, /sidebar-width-collapsed/);
  assert.doesNotMatch(marketingPageLayoutSource, /ConsoleHeaderBar|ConsoleSiderBar|SemiRuntime/);
});

test('page layout lazy loads the footer so the home startup bundle does not ship the entire footer link graph', async () => {
  const pageLayoutSource = await readFile(pageLayoutPath, 'utf8');
  const marketingPageLayoutSource = await readFile(marketingPageLayoutPath, 'utf8');
  const consolePageLayoutSource = await readFile(consolePageLayoutPath, 'utf8');

  assert.match(pageLayoutSource, /const FooterBar = lazy\(\(\) => import\('\.\/Footer'\)\);/);
  assert.match(pageLayoutSource, /<Suspense fallback=\{null\}>[\s\S]*<FooterBar \/>[\s\S]*<\/Suspense>/);
  assert.doesNotMatch(marketingPageLayoutSource, /from '\.\/Footer';/);
  assert.doesNotMatch(consolePageLayoutSource, /from '\.\/Footer';/);
});

test('marketing layout lazy loads the interactive header chrome so the home shell can paint before user controls hydrate', async () => {
  const marketingPageLayoutSource = await readFile(marketingPageLayoutPath, 'utf8');

  assert.match(
    marketingPageLayoutSource,
    /const MarketingHeaderBar = lazyWithRetry\([\s\S]*import\('\.\/MarketingHeaderBar'\),/,
  );
  assert.doesNotMatch(marketingPageLayoutSource, /import MarketingHeaderBar from '\.\/MarketingHeaderBar';/);
  assert.match(marketingPageLayoutSource, /<Suspense fallback=\{MARKETING_HEADER_FALLBACK\}>[\s\S]*<MarketingHeaderBar \/>[\s\S]*<\/Suspense>/);
  assert.match(marketingPageLayoutSource, /const MARKETING_HEADER_FALLBACK = \(/);
});

test('page layout only mounts the console shell for /console routes so public auth pages do not fetch console-only user data', async () => {
  const pageLayoutSource = await readFile(pageLayoutPath, 'utf8');

  assert.match(
    pageLayoutSource,
    /const isConsoleRoute = location\.pathname\.startsWith\('\/console'\);/,
  );
  assert.match(
    pageLayoutSource,
    /\{isConsoleRoute \? \([\s\S]*consoleShell[\s\S]*\) : \([\s\S]*marketingShell[\s\S]*\)\}/,
  );
  assert.doesNotMatch(pageLayoutSource, /const isMarketingRoute = location\.pathname === '\/';/);
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

test('i18n lazy loads zh-CN while keeping the locale loader map explicit', async () => {
  const source = await readFile(i18nPath, 'utf8');

  assert.doesNotMatch(source, /from 'i18next-browser-languagedetector';/);
  assert.doesNotMatch(source, /\.use\(LanguageDetector\)/);
  assert.match(source, /const localeLoaders = \{/);
  assert.doesNotMatch(source, /import\.meta\.glob\('\.\/locales\/\*\.json'\)/);
  assert.match(source, /'zh-CN': \(\) => import\('\.\/locales\/zh-CN\.json'\)/);
  assert.match(source, /en: \(\) => import\('\.\/locales\/en\.json'\)/);
  assert.match(source, /'zh-TW': \(\) => import\('\.\/locales\/zh-TW\.json'\)/);
  assert.match(source, /const defaultLanguageMessages = null;/);
  assert.match(source, /resources:\s*defaultLanguageMessages\s*\?\s*\{\s*\[DEFAULT_LANGUAGE\]:\s*\{\s*translation:\s*defaultLanguageMessages,/);
  assert.match(source, /const loadedLanguages = new Set\(defaultLanguageMessages \? \[DEFAULT_LANGUAGE\] : \[\]\);/);
});

test('dashboard keeps the original four summary cards including the performance metrics block', async () => {
  const statsCardsSource = await readFile(statsCardsPath, 'utf8');
  const dashboardStatsHookSource = await readFile(
    dashboardStatsHookPath,
    'utf8',
  );

  assert.match(statsCardsSource, /lg:grid-cols-4/);
  assert.match(dashboardStatsHookSource, /createSectionTitle\(Gauge, t\('性能指标'\)\)/);
  assert.match(dashboardStatsHookSource, /title: t\('平均RPM'\)/);
  assert.match(dashboardStatsHookSource, /title: t\('平均TPM'\)/);
});

test('dashboard keeps the chart runtime behind an explicit first-screen load action', async () => {
  const dashboardSource = await readFile(dashboardPath, 'utf8');

  assert.match(
    dashboardSource,
    /const \[chartsPanelEnabled, setChartsPanelEnabled\] = useState\(false\);/,
  );
  assert.match(
    dashboardSource,
    /const shouldLoadAdminUserCharts =\s*chartsPanelEnabled &&\s*dashboardData\.isAdminUser &&\s*\['5', '6'\]\.includes\(dashboardData\.activeChartTab\);/,
  );
  assert.match(
    dashboardSource,
    /const hasVisibleApiInfoPanel =\s*dashboardData\.hasApiInfoPanel && apiInfoData\.length > 0;/,
  );
  assert.match(
    dashboardSource,
    /className=\{`grid grid-cols-1 gap-4 \$\{hasVisibleApiInfoPanel \? 'lg:grid-cols-4' : ''\}`\}/,
  );
  assert.match(dashboardSource, /图表分析改为按需加载/);
  assert.match(dashboardSource, /加载图表分析/);
  assert.match(
    dashboardSource,
    /chartsPanelEnabled \? \([\s\S]*<Suspense[\s\S]*<ChartsPanel[\s\S]*<\/Suspense>[\s\S]*\) : \([\s\S]*setChartsPanelEnabled\(true\)[\s\S]*\)/,
  );
  assert.match(
    dashboardSource,
    /\{hasVisibleApiInfoPanel && \([\s\S]*<ApiInfoPanel[\s\S]*\)\}/,
  );
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

test('dashboard route keeps its modal lazy while rendering a compact chart placeholder before the heavy panel is requested', async () => {
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
  assert.match(
    source,
    /const hasVisibleApiInfoPanel =\s*dashboardData\.hasApiInfoPanel && apiInfoData\.length > 0;/,
  );
  assert.match(
    source,
    /<section[\s\S]*className=\{`rounded-2xl border border-slate-200 bg-white p-6 shadow-sm \$\{hasVisibleApiInfoPanel \? 'lg:col-span-3' : ''\}`\}/,
  );
  assert.match(
    source,
    /className='flex flex-col gap-4 md:flex-row md:items-start md:justify-between'/,
  );
  assert.match(
    source,
    /className='inline-flex items-center justify-center rounded-full bg-slate-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-slate-700 self-start'/,
  );
  assert.match(source, /chartsPanelEnabled/);
  assert.match(source, /加载图表分析/);
  assert.match(source, /图表分析改为按需加载/);
  assert.match(source, /import \{ getRelativeTime \} from '\.\.\/\.\.\/helpers\/time';/);
});

test('dashboard api info empty state stays compact so an empty side panel does not push the second row downward', async () => {
  const source = await readFile(apiInfoPanelPath, 'utf8');

  assert.doesNotMatch(source, /min-h-\[20rem\]/);
  assert.doesNotMatch(
    source,
    /import \{ Card, Avatar, Tag, Divider, Empty \} from '@douyinfe\/semi-ui';/,
  );
  assert.doesNotMatch(source, /<Empty/);
  assert.doesNotMatch(source, /className='flex items-center justify-center py-8'/);
  assert.match(
    source,
    /className='flex items-start gap-3 rounded-xl border border-dashed border-slate-200 bg-white\/80 px-3 py-3 text-left'/,
  );
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

test('dashboard summary cards restore the original fourth performance tile', async () => {
  const [statsSource, dashboardStatsHookSource] = await Promise.all([
    readFile(statsCardsPath, 'utf8'),
    readFile(dashboardStatsHookPath, 'utf8'),
  ]);

  assert.match(statsSource, /grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4/);
  assert.match(dashboardStatsHookSource, /性能指标/);
  assert.match(dashboardStatsHookSource, /平均RPM/);
  assert.match(dashboardStatsHookSource, /平均TPM/);
  assert.match(dashboardStatsHookSource, /import \{ Wallet, Activity, Zap, Gauge \} from 'lucide-react';/);
  assert.match(
    dashboardStatsHookSource,
    /IconStopwatchStroked|IconTypograph/,
  );
});

test('dashboard only loads admin user chart data after the chart surface has been explicitly enabled', async () => {
  const source = await readFile(dashboardPath, 'utf8');

  assert.match(
    source,
    /const shouldLoadAdminUserCharts =\s*chartsPanelEnabled &&\s*dashboardData\.isAdminUser &&\s*\['5', '6'\]\.includes\(dashboardData\.activeChartTab\);/,
  );
  assert.match(source, /if \(!force && adminUserChartCacheKeyRef\.current === userChartCacheKey\) \{/);
  assert.match(source, /if \(shouldLoadAdminUserCharts\) \{\s*await loadUserData\(\{ force: true \}\);/);
  assert.match(source, /useEffect\(\(\) => \{\s*if \(!shouldLoadAdminUserCharts\) \{\s*return;\s*\}\s*void loadUserData\(\);\s*\}, \[loadUserData, shouldLoadAdminUserCharts\]\);/);
  assert.match(source, /chartsPanelEnabled/);
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
