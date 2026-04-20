import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const settingPageSource = readFileSync('web/src/pages/Setting/index.jsx', 'utf8');
const operationSettingSource = readFileSync(
  'web/src/components/settings/OperationSetting.jsx',
  'utf8',
);
const paymentSettingSource = readFileSync(
  'web/src/components/settings/PaymentSetting.jsx',
  'utf8',
);
const apiHelperSource = readFileSync('web/src/helpers/api.js', 'utf8');

test('settings page lazy loads heavyweight tab surfaces instead of importing every tab into one chunk', () => {
  assert.match(settingPageSource, /import \{ lazyWithRetry \} from '\.\.\/\.\.\/helpers\/lazyWithRetry';/);
  assert.match(
    settingPageSource,
    /const OperationSetting = lazyWithRetry\([\s\S]*import\('\.\.\/\.\.\/components\/settings\/OperationSetting'\),/,
  );
  assert.match(
    settingPageSource,
    /const PaymentSetting = lazyWithRetry\([\s\S]*import\('\.\.\/\.\.\/components\/settings\/PaymentSetting'\),/,
  );
  assert.match(
    settingPageSource,
    /const RatioSetting = lazyWithRetry\([\s\S]*import\('\.\.\/\.\.\/components\/settings\/RatioSetting'\),/,
  );
  assert.match(settingPageSource, /<Suspense fallback=\{<Spin spinning \/>?\}>/);
});

test('operation settings keep the tab lazy but ship one section bundle once the tab is opened', () => {
  assert.match(
    operationSettingSource,
    /import SettingsGeneral from '\.\.\/\.\.\/pages\/Setting\/Operation\/SettingsGeneral';/,
  );
  assert.match(
    operationSettingSource,
    /import SettingsHeaderNavModules from '\.\.\/\.\.\/pages\/Setting\/Operation\/SettingsHeaderNavModules';/,
  );
  assert.match(
    operationSettingSource,
    /import SettingsMonitoring from '\.\.\/\.\.\/pages\/Setting\/Operation\/SettingsMonitoring';/,
  );
  assert.doesNotMatch(operationSettingSource, /lazyWithRetry/);
  assert.doesNotMatch(operationSettingSource, /<Suspense fallback=\{null\}>/);
});

test('payment settings keep the tab lazy but bundle gateway sections together after the tab opens', () => {
  assert.match(
    paymentSettingSource,
    /import SettingsGeneralPayment from '\.\.\/\.\.\/pages\/Setting\/Payment\/SettingsGeneralPayment';/,
  );
  assert.match(
    paymentSettingSource,
    /import SettingsPaymentGatewayStripe from '\.\.\/\.\.\/pages\/Setting\/Payment\/SettingsPaymentGatewayStripe';/,
  );
  assert.match(
    paymentSettingSource,
    /import SettingsPaymentGatewayCreem from '\.\.\/\.\.\/pages\/Setting\/Payment\/SettingsPaymentGatewayCreem';/,
  );
  assert.match(
    paymentSettingSource,
    /import SettingsWithdrawal from '\.\.\/\.\.\/pages\/Setting\/Payment\/SettingsWithdrawal';/,
  );
  assert.doesNotMatch(paymentSettingSource, /lazyWithRetry/);
  assert.doesNotMatch(paymentSettingSource, /<Suspense fallback=\{null\}>/);
});

test('api helper reuses recent admin option responses and invalidates them after option writes', () => {
  assert.match(apiHelperSource, /const OPTION_RESPONSE_CACHE_TTL_MS = 30_000;/);
  assert.match(apiHelperSource, /url === '\/api\/option\/'/);
  assert.match(apiHelperSource, /config\?\.disableResponseCache/);
  assert.match(apiHelperSource, /startsWith\('\/api\/option\/'\)/);
  assert.match(apiHelperSource, /const cachedGetResponses = new Map\(\);/);
});
