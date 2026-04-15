import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const marketingHeaderBarPath = new URL(
  '../src/components/layout/MarketingHeaderBar.jsx',
  import.meta.url,
);
const homePath = new URL('../src/pages/Home/index.jsx', import.meta.url);
const marketingNoticeModalPath = new URL(
  '../src/components/layout/MarketingNoticeModal.jsx',
  import.meta.url,
);

test('marketing shells use the lightweight marketing notice modal instead of the Semi modal', async () => {
  const [marketingHeaderSource, homeSource] = await Promise.all([
    readFile(marketingHeaderBarPath, 'utf8'),
    readFile(homePath, 'utf8'),
  ]);

  assert.match(
    marketingHeaderSource,
    /const MarketingNoticeModal = lazy\(\(\) => import\('\.\/MarketingNoticeModal'\)\);/,
  );
  assert.doesNotMatch(
    marketingHeaderSource,
    /const NoticeModal = lazy\(\(\) => import\('\.\/NoticeModal'\)\);/,
  );
  assert.match(
    homeSource,
    /const MarketingNoticeModal = lazy\([\s\S]*import\('\.\.\/\.\.\/components\/layout\/MarketingNoticeModal'\)[\s\S]*\);/,
  );
  assert.doesNotMatch(
    homeSource,
    /const NoticeModal = lazy\(\(\) => import\('\.\.\/\.\.\/components\/layout\/NoticeModal'\)\);/,
  );
});

test('marketing notice modal avoids Semi UI so opening it cannot inject Semi global styles into the homepage shell', async () => {
  const source = await readFile(marketingNoticeModalPath, 'utf8');

  assert.doesNotMatch(source, /from '@douyinfe\/semi-ui';/);
  assert.match(source, /from 'react-dom';/);
  assert.match(source, /createPortal\(/);
  assert.match(source, /const MarketingNoticeModal = \(/);
  assert.match(source, /className='fixed inset-0 z-\[130\]/);
  assert.doesNotMatch(source, /bg-white\/96/);
  assert.doesNotMatch(source, /bg-slate-50\/80/);
  assert.doesNotMatch(source, /backdrop-blur-sm/);
});

test('marketing header dropdowns use opaque panels so hero text does not bleed through menus', async () => {
  const source = await readFile(marketingHeaderBarPath, 'utf8');

  assert.match(
    source,
    /function DropdownPanel[\s\S]*bg-white p-2 shadow-xl dark:border-slate-700 dark:bg-slate-950/,
  );
  assert.doesNotMatch(
    source,
    /function DropdownPanel[\s\S]*bg-white\/96[\s\S]*backdrop-blur-xl/,
  );
});
