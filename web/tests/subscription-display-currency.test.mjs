import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const addEditSource = readFileSync(
  new URL(
    '../classic/src/components/table/subscriptions/modals/AddEditSubscriptionModal.jsx',
    import.meta.url,
  ),
  'utf8',
);

test('subscription admin editor currency field follows the site-wide display setting', () => {
  assert.match(addEditSource, /const getSiteDisplayCurrencyLabel = \(\) => \{/);
  assert.match(
    addEditSource,
    /localStorage\.getItem\('quota_display_type'\) \|\| 'USD'/,
  );
  assert.match(addEditSource, /if \(quotaDisplayType === 'CNY'\) return 'CNY \(¥\)'/);
  assert.match(addEditSource, /if \(quotaDisplayType === 'TOKENS'\) return 'Tokens'/);
  assert.match(addEditSource, /if \(quotaDisplayType === 'CUSTOM'\)/);
  assert.match(addEditSource, /custom_currency_symbol/);
  assert.match(addEditSource, /return `\$\{t\('自定义货币'\)\} \(\$\{symbol\}\)`/);
  assert.match(addEditSource, /currency: getSiteDisplayCurrencyLabel\(\)/);
  assert.match(addEditSource, /extraText=\{t\('由全站货币展示设置统一控制'\)\}/);
  assert.match(addEditSource, /Plans are still stored with USD as the internal billing base\./);
});
