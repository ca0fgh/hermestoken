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
const dashboardPath = new URL(
  '../src/components/dashboard/index.jsx',
  import.meta.url,
);
const statsCardsPath = new URL(
  '../src/components/dashboard/StatsCards.jsx',
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

test('heavy console routes are lazy loaded to keep the app entry chunk small', async () => {
  const source = await readFile(appPath, 'utf8');

  assert.match(
    source,
    /const Playground = lazyWithRetry\([\s\S]*import\('\.\/pages\/Playground'\),/,
  );
  assert.match(
    source,
    /const Setting = lazyWithRetry\([\s\S]*import\('\.\/pages\/Setting'\),/,
  );
  assert.match(
    source,
    /const ModelPage = lazyWithRetry\([\s\S]*import\('\.\/pages\/Model'\),/,
  );
  assert.match(
    source,
    /const Token = lazyWithRetry\([\s\S]*import\('\.\/pages\/Token'\),/,
  );
  assert.match(
    source,
    /const Chat = lazyWithRetry\([\s\S]*import\('\.\/pages\/Chat'\),/,
  );
});

test('vite build keeps only safe heavy dependencies in dedicated chunks', async () => {
  const source = await readFile(viteConfigPath, 'utf8');
  const packageJson = JSON.parse(await readFile(packageJsonPath, 'utf8'));

  assert.match(source, /manualChunks:\s*(buildManualChunkName|\()/);
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
  assert.doesNotMatch(source, /id\.includes\('@douyinfe\/semi-ui'\)/);
  assert.doesNotMatch(source, /id\.includes\('@visactor\/'\)/);
  assert.doesNotMatch(source, /return 'brand-icons';/);
  assert.doesNotMatch(source, /return 'rich-content';/);
  assert.notEqual(reactCoreIndex, -1);
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

test('i18n keeps zh-CN bundled by default and lazy loads only non-default locales', async () => {
  const source = await readFile(i18nPath, 'utf8');

  assert.match(
    source,
    /import zhCNTranslation from '\.\/locales\/zh-CN\.json';/,
  );
  assert.match(source, /const localeLoaders = \{/);
  assert.doesNotMatch(source, /import\.meta\.glob\('\.\/locales\/\*\.json'\)/);
  assert.doesNotMatch(
    source,
    /'zh-CN': \(\) => import\('\.\/locales\/zh-CN\.json'\)/,
  );
  assert.match(source, /en: \(\) => import\('\.\/locales\/en\.json'\)/);
  assert.match(source, /'zh-TW': \(\) => import\('\.\/locales\/zh-TW\.json'\)/);
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

test('dashboard route lazy loads its heavy visual panels instead of statically importing them', async () => {
  const source = await readFile(dashboardPath, 'utf8');

  assert.match(
    source,
    /const StatsCards = lazy\(\(\) => import\('\.\/StatsCards'\)\);/,
  );
  assert.match(
    source,
    /const ChartsPanel = lazy\(\(\) => import\('\.\/ChartsPanel'\)\);/,
  );
  assert.match(
    source,
    /const ApiInfoPanel = lazy\(\(\) => import\('\.\/ApiInfoPanel'\)\);/,
  );
  assert.match(
    source,
    /const AnnouncementsPanel = lazy\(\(\) => import\('\.\/AnnouncementsPanel'\)\);/,
  );
  assert.match(
    source,
    /const FaqPanel = lazy\(\(\) => import\('\.\/FaqPanel'\)\);/,
  );
  assert.match(
    source,
    /const UptimePanel = lazy\(\(\) => import\('\.\/UptimePanel'\)\);/,
  );
  assert.doesNotMatch(source, /import StatsCards from '\.\/StatsCards';/);
  assert.doesNotMatch(source, /import ChartsPanel from '\.\/ChartsPanel';/);
});

test('dashboard chart panels use the lazy VChart wrapper instead of importing visactor directly', async () => {
  const [statsSource, chartsSource, lazyVChartSource, dashboardRuntimeSource] =
    await Promise.all([
      readFile(statsCardsPath, 'utf8'),
      readFile(chartsPanelPath, 'utf8'),
      readFile(lazyVChartPath, 'utf8'),
      readFile(dashboardRuntimePath, 'utf8'),
    ]);

  assert.doesNotMatch(statsSource, /from '@visactor\/react-vchart';/);
  assert.doesNotMatch(chartsSource, /from '@visactor\/react-vchart';/);
  assert.match(statsSource, /from '\.\/LazyVChart';/);
  assert.match(chartsSource, /from '\.\/LazyVChart';/);
  assert.match(lazyVChartSource, /import\('@visactor\/react-vchart'\)/);
  assert.match(lazyVChartSource, /import\('\.\/vchartDashboardRuntime'\)/);
  assert.match(lazyVChartSource, /<module\.VChartSimple/);
  assert.match(lazyVChartSource, /vchartConstrouctor=\{/);
  assert.match(dashboardRuntimeSource, /registerLineChart/);
  assert.match(dashboardRuntimeSource, /registerPieChart/);
  assert.doesNotMatch(dashboardRuntimeSource, /registerLabel/);
});

test('dashboard chart hook lazy loads the visactor semi theme instead of statically importing it', async () => {
  const source = await readFile(dashboardChartsHookPath, 'utf8');
  const pieSpecSection = source.slice(
    source.indexOf('const [spec_pie'),
    source.indexOf('const [spec_line'),
  );

  assert.doesNotMatch(source, /from '@visactor\/vchart-semi-theme';/);
  assert.match(source, /import\('@visactor\/vchart-semi-theme'\)/);
  assert.match(source, /function ensureVChartSemiTheme\(/);
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
