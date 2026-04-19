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

test('operation settings lazy load their heavyweight cards so switching tabs does not pull every section at once', () => {
  assert.match(
    operationSettingSource,
    /const SettingsGeneral = lazyWithRetry\([\s\S]*import\('\.\.\/\.\.\/pages\/Setting\/Operation\/SettingsGeneral'\),/,
  );
  assert.match(
    operationSettingSource,
    /const SettingsHeaderNavModules = lazyWithRetry\([\s\S]*import\('\.\.\/\.\.\/pages\/Setting\/Operation\/SettingsHeaderNavModules'\),/,
  );
  assert.match(
    operationSettingSource,
    /const SettingsMonitoring = lazyWithRetry\([\s\S]*import\('\.\.\/\.\.\/pages\/Setting\/Operation\/SettingsMonitoring'\),/,
  );
  assert.match(operationSettingSource, /<Suspense fallback=\{null\}>/);
});

test('payment settings lazy load provider panels so the settings route stops inheriting every gateway form', () => {
  assert.match(
    paymentSettingSource,
    /const SettingsGeneralPayment = lazyWithRetry\([\s\S]*import\('\.\.\/\.\.\/pages\/Setting\/Payment\/SettingsGeneralPayment'\),/,
  );
  assert.match(
    paymentSettingSource,
    /const SettingsPaymentGatewayStripe = lazyWithRetry\([\s\S]*import\('\.\.\/\.\.\/pages\/Setting\/Payment\/SettingsPaymentGatewayStripe'\),/,
  );
  assert.match(
    paymentSettingSource,
    /const SettingsPaymentGatewayCreem = lazyWithRetry\([\s\S]*import\('\.\.\/\.\.\/pages\/Setting\/Payment\/SettingsPaymentGatewayCreem'\),/,
  );
  assert.match(
    paymentSettingSource,
    /const SettingsWithdrawal = lazyWithRetry\([\s\S]*import\('\.\.\/\.\.\/pages\/Setting\/Payment\/SettingsWithdrawal'\),/,
  );
  assert.match(paymentSettingSource, /<Suspense fallback=\{null\}>/);
});

test('api helper reuses recent admin option responses and invalidates them after option writes', () => {
  assert.match(apiHelperSource, /const OPTION_RESPONSE_CACHE_TTL_MS = 30_000;/);
  assert.match(apiHelperSource, /url === '\/api\/option\/'/);
  assert.match(apiHelperSource, /config\?\.disableResponseCache/);
  assert.match(apiHelperSource, /startsWith\('\/api\/option\/'\)/);
  assert.match(apiHelperSource, /const cachedGetResponses = new Map\(\);/);
});
