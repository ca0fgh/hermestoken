import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const storageJsonPath = new URL('../classic/src/helpers/storageJson.js', import.meta.url);
const authPath = new URL('../classic/src/helpers/auth.jsx', import.meta.url);
const tokenPagePath = new URL('../classic/src/components/table/tokens/index.jsx', import.meta.url);
const tokenHookPath = new URL('../classic/src/hooks/tokens/useTokensData.jsx', import.meta.url);
const tokenColumnsPath = new URL('../classic/src/components/table/tokens/TokensColumnDefs.jsx', import.meta.url);
const ccSwitchModalPath = new URL('../classic/src/components/table/tokens/modals/CCSwitchModal.jsx', import.meta.url);
const notificationsHookPath = new URL('../classic/src/hooks/common/useNotifications.js', import.meta.url);
const homePath = new URL('../classic/src/pages/Home/index.jsx', import.meta.url);
const marketingNoticeModalPath = new URL(
  '../classic/src/components/layout/MarketingNoticeModal.jsx',
  import.meta.url,
);
const noticeModalPath = new URL('../classic/src/components/layout/NoticeModal.jsx', import.meta.url);

function readSource(url) {
  return readFileSync(url, 'utf8');
}

test('shared storage helpers guard malformed localStorage JSON for token and notification routes', () => {
  const storageSource = readSource(storageJsonPath);
  const authSource = readSource(authPath);
  const tokenPageSource = readSource(tokenPagePath);
  const tokenHookSource = readSource(tokenHookPath);
  const tokenColumnsSource = readSource(tokenColumnsPath);
  const ccSwitchModalSource = readSource(ccSwitchModalPath);
  const notificationsSource = readSource(notificationsHookPath);
  const homeSource = readSource(homePath);
  const marketingNoticeModalSource = readSource(marketingNoticeModalPath);
  const noticeModalSource = readSource(noticeModalPath);

  assert.match(storageSource, /export function readStoredValue\(key, fallback = null\)/);
  assert.match(storageSource, /export function readStoredJson\(key, fallback = null\)/);
  assert.match(storageSource, /const rawValue = readStoredValue\(key, null\);/);
  assert.match(storageSource, /return JSON\.parse\(rawValue\);/);
  assert.match(storageSource, /catch \{\s*return fallback;\s*\}/s);
  assert.match(storageSource, /export function writeStoredValue\(key, value\)/);
  assert.match(storageSource, /export function removeStoredValue\(key\)/);
  assert.match(storageSource, /export function getStoredServerAddress\(\)/);
  assert.match(storageSource, /window\.location\.origin/);
  assert.match(storageSource, /export function getStoredUser\(\)/);
  assert.match(storageSource, /export function readStoredArray\(key\)/);

  assert.match(authSource, /from '\.\/storageJson';/);
  assert.match(authSource, /const user = getStoredUser\(\);/);
  assert.doesNotMatch(authSource, /JSON\.parse\(localStorage\.getItem\('user'\)\)/);

  assert.match(tokenPageSource, /from '\.\.\/\.\.\/\.\.\/helpers\/storageJson';/);
  assert.match(tokenPageSource, /const serverAddress = getStoredServerAddress\(\);/);
  assert.doesNotMatch(tokenPageSource, /JSON\.parse\(status\)/);

  assert.match(tokenHookSource, /from '\.\.\/\.\.\/helpers\/storageJson';/);
  assert.match(tokenHookSource, /const serverAddress = getStoredServerAddress\(\);/);
  assert.doesNotMatch(tokenHookSource, /JSON\.parse\(status\)/);

  assert.match(tokenColumnsSource, /from '\.\.\/\.\.\/\.\.\/helpers\/storageJson';/);
  assert.match(tokenColumnsSource, /const parsed = readStoredArray\('chats'\);/);
  assert.doesNotMatch(tokenColumnsSource, /JSON\.parse\(raw\)/);

  assert.match(ccSwitchModalSource, /from '\.\.\/\.\.\/\.\.\/\.\.\/helpers\/storageJson';/);
  assert.match(ccSwitchModalSource, /return getStoredServerAddress\(\);/);
  assert.doesNotMatch(ccSwitchModalSource, /JSON\.parse\(raw\)/);

  assert.match(notificationsSource, /from '\.\.\/\.\.\/helpers\/storageJson';/);
  assert.match(notificationsSource, /readStoredArray\('notice_read_keys'\)/);
  assert.doesNotMatch(notificationsSource, /JSON\.parse\(localStorage\.getItem\('notice_read_keys'\)\)/);

  assert.match(homeSource, /from '\.\.\/\.\.\/helpers\/storageJson';/);
  assert.match(homeSource, /readStoredValue\('notice_close_date', ''\)/);
  assert.doesNotMatch(homeSource, /localStorage\.getItem\('notice_close_date'\)/);

  assert.match(marketingNoticeModalSource, /from '\.\.\/\.\.\/helpers\/storageJson';/);
  assert.match(marketingNoticeModalSource, /writeStoredValue\('notice_close_date', today\)/);
  assert.doesNotMatch(
    marketingNoticeModalSource,
    /localStorage\.setItem\('notice_close_date', today\)/,
  );

  assert.match(noticeModalSource, /from '\.\.\/\.\.\/helpers\/storageJson';/);
  assert.match(noticeModalSource, /writeStoredValue\('notice_close_date', today\)/);
  assert.doesNotMatch(
    noticeModalSource,
    /localStorage\.setItem\('notice_close_date', today\)/,
  );
});
