import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const testDir = path.dirname(fileURLToPath(import.meta.url));
const webRoot = path.resolve(testDir, '../classic');
const helperPath = path.join(webRoot, 'src/helpers/withdrawal.js');
const helperUrl = pathToFileURL(helperPath).href;

const readSource = (relativePath) =>
  fs.readFileSync(path.join(webRoot, relativePath), 'utf8');

const loadHelpers = () => import(`${helperUrl}?t=${Date.now()}`);

test('wallet topup page loads withdrawal config and renders withdrawal entry', () => {
  const source = readSource('src/components/topup/index.jsx');
  const rechargeCardSource = readSource(
    'src/components/topup/RechargeCard.jsx',
  );
  const helperSource = readSource('src/helpers/withdrawal.js');
  const withdrawalCardSource = readSource(
    'src/components/topup/WithdrawalCard.jsx',
  );
  const withdrawalModalSource = readSource(
    'src/components/topup/modals/WithdrawalApplyModal.jsx',
  );
  assert.match(source, /\/api\/user\/withdrawal\/config/);
  assert.match(source, /WithdrawalCard/);
  assert.match(source, /WithdrawalApplyModal/);
  assert.match(source, /WithdrawalHistoryModal/);
  assert.match(source, /submitWithdrawal/);
  assert.match(source, /支付宝姓名不能为空/);
  assert.match(source, /USDT 收款地址不能为空/);
  assert.match(source, /channel:\s*withdrawalChannel/);
  assert.match(source, /withdrawalPreview\?\.isValid/);
  assert.match(
    source,
    /当前提现金额未命中任何手续费规则，请调整金额或联系管理员/,
  );
  assert.match(source, /withdrawalSection=\{/);
  assert.doesNotMatch(source, /<\/RechargeCard>\s*<WithdrawalCard/);
  assert.match(rechargeCardSource, /withdrawalSection = null/);
  assert.match(rechargeCardSource, /{withdrawalSection}/);
  assert.match(withdrawalModalSource, /preview\?\.isValid/);
  assert.match(withdrawalModalSource, /提现方式/);
  assert.match(withdrawalModalSource, /USDT 网络/);
  assert.match(withdrawalModalSource, /USDT 收款地址/);
  assert.match(helperSource, /Polygon PoS/);
  assert.match(withdrawalModalSource, /规则说明/);
  assert.match(withdrawalModalSource, /命中规则/);
  assert.match(helperSource, /getWithdrawalBalanceAmounts/);
  assert.match(helperSource, /totalAmount\s*=\s*Number\(config\?\.total_amount/);
  assert.match(
    helperSource,
    /rechargeAmount\s*=\s*Number\(config\?\.recharge_amount/,
  );
  assert.match(
    helperSource,
    /redemptionAmount\s*=\s*Number\(config\?\.redemption_amount/,
  );
  assert.match(withdrawalCardSource, /getWithdrawalBalanceAmounts/);
  assert.match(withdrawalModalSource, /getWithdrawalBalanceAmounts/);
  assert.match(withdrawalCardSource, /label={t\('充值余额'\)}/);
  assert.match(withdrawalCardSource, /badge={t\('可提现'\)}/);
  assert.match(withdrawalCardSource, /label={t\('兑换码余额'\)}/);
  assert.match(withdrawalCardSource, /badge={t\('不可提现'\)}/);
  assert.match(withdrawalCardSource, /可用总余额/);
  assert.match(withdrawalModalSource, /充值余额（可提现）/);
  assert.match(withdrawalModalSource, /兑换码余额（不可提现）/);
  assert.match(withdrawalModalSource, /最多可提 {{amount}}/);
  assert.match(
    withdrawalModalSource,
    /当前提现金额未命中任何手续费规则，请调整金额或联系管理员/,
  );
  assert.doesNotMatch(
    withdrawalModalSource,
    /未命中手续费规则，按 0 手续费计算/,
  );
});

test('withdrawal config keeps balance breakdown fallback logic in one helper', async () => {
  const { normalizeWithdrawalConfig, getWithdrawalBalanceAmounts } =
    await loadHelpers();

  const config = normalizeWithdrawalConfig({
    available_amount: 80,
    recharge_amount: 80,
    redemption_amount: 20,
    frozen_amount: 15,
    min_amount: 10,
  });

  assert.deepEqual(getWithdrawalBalanceAmounts(config), {
    totalAmount: 100,
    rechargeAmount: 80,
    redemptionAmount: 20,
    frozenAmount: 15,
    minAmount: 10,
  });

  assert.deepEqual(
    getWithdrawalBalanceAmounts(
      normalizeWithdrawalConfig({
        available_amount: 42,
      }),
    ),
    {
      totalAmount: 42,
      rechargeAmount: 42,
      redemptionAmount: 0,
      frozenAmount: 0,
      minAmount: 0,
    },
  );
});

test('user-facing withdrawal rule descriptions stay currency-aware and preserve the strict first-band lower bound', async () => {
  const {
    buildWithdrawalFeeRuleDescriptions,
    describeWithdrawalFeeRuleForUser,
  } = await loadHelpers();

  const usdDescriptions = buildWithdrawalFeeRuleDescriptions(
    [
      {
        min_amount: 0,
        max_amount: 100,
        fee_type: 'fixed',
        fee_value: 5,
        enabled: true,
        sort_order: 1,
      },
      {
        min_amount: 100,
        max_amount: 500,
        fee_type: 'ratio',
        fee_value: 3,
        enabled: true,
        sort_order: 2,
      },
    ],
    (key) => key,
    {
      currencySymbol: '$',
      currency: 'USD',
    },
  );

  assert.equal(usdDescriptions[0], '大于 0 且不超过 $100：固定手续费 $5');
  assert.equal(usdDescriptions[1], '高于 $100 至 $500：按 3% 收费');
  assert.doesNotMatch(usdDescriptions.join('\n'), /元|CNY|及以下|or less/);

  const customDescription = describeWithdrawalFeeRuleForUser(
    {
      min_amount: 0,
      max_amount: 200,
      fee_type: 'fixed',
      fee_value: 8,
      enabled: true,
    },
    (key) => key,
    {
      currencySymbol: 'PTS ',
      currency: 'CUSTOM',
    },
  );

  assert.equal(customDescription, '大于 0 且不超过 PTS 200：固定手续费 PTS 8');
});
