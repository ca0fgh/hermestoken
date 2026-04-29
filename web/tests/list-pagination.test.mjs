import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

test('ListPagination centralizes single-page hiding and mobile sizing', () => {
  const source = readFileSync(
    new URL('../classic/src/components/common/ui/ListPagination.jsx', import.meta.url),
    'utf8',
  );

  assert.match(source, /useIsMobile/);
  assert.match(source, /useTranslation/);
  assert.match(source, /hideOnSinglePage = true/);
  assert.match(source, /showRangeSummary = true/);
  assert.match(source, /total <= pageSize/);
  assert.match(source, /t\('显示第'\)/);
  assert.match(source, /t\('条 - 第'\)/);
  assert.match(source, /t\('条，共'\)/);
  assert.match(source, /showSizeChanger/);
  assert.match(source, /showQuickJumper/);
  assert.match(source, /typeof showQuickJumper === 'boolean'/);
  assert.match(source, /size \|\| \(isMobile \? 'small' : 'default'\)/);
});
