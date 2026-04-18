import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

function readSource(relativePath) {
  return readFileSync(
    new URL(`../${relativePath}`, import.meta.url),
    'utf8',
  );
}

test('shared pagination entrypoints delegate to ListPagination', () => {
  const utilsSource = readSource('src/helpers/utils.jsx');
  const cardTableSource = readSource('src/components/common/ui/CardTable.jsx');
  const pricingCardViewSource = readSource(
    'src/components/table/model-pricing/view/card/PricingCardView.jsx',
  );

  assert.match(utilsSource, /import ListPagination from '\.\.\/components\/common\/ui\/ListPagination';/);
  assert.match(utilsSource, /<ListPagination/);
  assert.match(utilsSource, /export const createUnifiedPaginationProps = \(/);
  assert.doesNotMatch(utilsSource, /import\s*\{\s*Toast,\s*Pagination\s*\}\s*from '@douyinfe\/semi-ui';/);

  assert.match(cardTableSource, /import ListPagination from '\.\/ListPagination';/);
  assert.match(cardTableSource, /pagination: false/);
  assert.match(cardTableSource, /<ListPagination/);
  assert.doesNotMatch(cardTableSource, /import\s*\{[\s\S]*Pagination[\s\S]*\}\s*from '@douyinfe\/semi-ui';/);

  assert.match(pricingCardViewSource, /import ListPagination from '\.\.\/\.\.\/\.\.\/\.\.\/common\/ui\/ListPagination';/);
  assert.match(pricingCardViewSource, /<ListPagination/);
  assert.doesNotMatch(pricingCardViewSource, /import\s*\{[\s\S]*Pagination[\s\S]*\}\s*from '@douyinfe\/semi-ui';/);
});
