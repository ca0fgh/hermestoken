import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const readClassicSource = (relativePath) =>
  readFile(new URL(`../classic/src/${relativePath}`, import.meta.url), 'utf8');

test('classic operation general settings expose the marketplace buyer fee rate option', async () => {
  const [operationSettingSource, generalSettingsSource] = await Promise.all([
    readClassicSource('components/settings/OperationSetting.jsx'),
    readClassicSource('pages/Setting/Operation/SettingsGeneral.jsx'),
  ]);

  assert.match(operationSettingSource, /MarketplaceFeeRate:\s*(?:0|['"]0['"])/);
  assert.match(generalSettingsSource, /MarketplaceFeeRate:\s*['"]['"]/);
  assert.match(generalSettingsSource, /field=\{['"]MarketplaceFeeRate['"]\}/);
  assert.match(generalSettingsSource, /label=\{t\(['"]市场买家交易手续费率['"]\)\}/);
  assert.match(generalSettingsSource, /min=\{0\}/);
  assert.match(generalSettingsSource, /0\.05 表示 5%/);
  assert.match(generalSettingsSource, /订单池会从买家余额扣调用费用 \+ 手续费/);
  assert.match(generalSettingsSource, /买断额度需要包含手续费/);
  assert.match(generalSettingsSource, /API\.put\(['"]\/api\/option\/['"]/);
});
