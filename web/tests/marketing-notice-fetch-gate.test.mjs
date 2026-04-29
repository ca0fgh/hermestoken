import assert from 'node:assert/strict';
import test from 'node:test';

const marketingNoticeFetchGateModulePath = new URL(
  '../classic/src/components/layout/marketingNoticeFetchGate.js',
  import.meta.url,
);

test('bootstrap notice html skips the initial marketing notice fetch', async () => {
  const { shouldFetchMarketingNotice } = await import(
    marketingNoticeFetchGateModulePath
  );

  assert.equal(
    shouldFetchMarketingNotice({
      visible: true,
      initialNoticeHtml: '<p>ready</p>',
    }),
    false,
  );
});

test('visible marketing notice without bootstrap html still fetches notice content', async () => {
  const { shouldFetchMarketingNotice } = await import(
    marketingNoticeFetchGateModulePath
  );

  assert.equal(
    shouldFetchMarketingNotice({
      visible: true,
      initialNoticeHtml: '',
    }),
    true,
  );
});
