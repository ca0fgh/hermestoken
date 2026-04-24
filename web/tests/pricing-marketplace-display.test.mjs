import React from 'react';
import { afterEach, describe, expect, mock, test } from 'bun:test';
import { act, create } from 'react-test-renderer';
import { readFileSync, unlinkSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';

let importCounter = 0;
const h = React.createElement;

function readSource(relativePath) {
  return readFileSync(new URL(relativePath, import.meta.url), 'utf8');
}

function getText(node) {
  if (node == null) {
    return '';
  }

  if (typeof node === 'string') {
    return node;
  }

  if (Array.isArray(node)) {
    return node.map(getText).join('');
  }

  return getText(node.children);
}

async function importFresh(modulePath, suffix) {
  importCounter += 1;
  return import(`${modulePath}?${suffix}-${importCounter}`);
}

async function importPricingUtilsSubset() {
  const source = readSource('../src/helpers/utils.jsx');
  const start = source.indexOf('export const PRICING_GROUP_ALL_SENTINEL');
  const end = source.indexOf('export const getModelPriceItems');
  const subset = source.slice(start, end);
  const tempPath = path.join(
    tmpdir(),
    `pricing-utils-subset-${Date.now()}-${importCounter}.mjs`,
  );
  writeFileSync(tempPath, subset, 'utf8');
  try {
    return await import(`file://${tempPath}`);
  } finally {
    unlinkSync(tempPath);
  }
}

async function renderElement(element) {
  let renderer;

  await act(async () => {
    renderer = create(element);
    await Promise.resolve();
  });

  return renderer;
}

afterEach(() => {
  mock.restore();
});

describe('marketplace display group wiring', () => {
  test('useModelPricingData consumes display_groups and exposes displayGroups', () => {
    const source = readSource(
      '../src/hooks/model-pricing/useModelPricingData.jsx',
    );
    const helperSource = readSource('../src/helpers/utils.jsx');

    expect(source).toMatch(/display_groups/);
    expect(source).toMatch(/PRICING_GROUP_ALL_SENTINEL/);
    expect(source).toMatch(
      /const \[displayGroups, setDisplayGroups\] = useState\(\{\}\);/,
    );
    expect(source).toMatch(/displayGroups,/);
    expect(source).not.toMatch(/\busableGroup\b/);
    expect(source).not.toMatch(/\bautoGroups\b/);
    expect(helperSource).toMatch(
      /export const PRICING_GROUP_ALL_SENTINEL = '__all__';/,
    );
  });

  test('PricingGroups uses a distinct UI sentinel while preserving backend all/default display groups', async () => {
    mock.module('../src/helpers/utils.jsx', () => ({
      PRICING_GROUP_ALL_SENTINEL: '__all__',
    }));
    mock.module('../src/components/common/ui/SelectableButtonGroup.jsx', () => ({
      default: ({ title, items, activeValue }) =>
        h(
          'section',
          null,
          h('h1', null, title),
          h('div', { 'data-active': activeValue }, `active:${activeValue}`),
          h(
            'ul',
            null,
            items.map((item) =>
              h(
                'li',
                { key: item.value },
                `${item.value}|${item.label}|${item.tagCount ?? ''}`,
              ),
            ),
          ),
        ),
    }));

    const { default: PricingGroups } = await importFresh(
      '../src/components/table/model-pricing/filter/PricingGroups.jsx',
      'pricing-groups',
    );

    const renderer = await renderElement(
      h(PricingGroups, {
        filterGroup: '__all__',
        setFilterGroup: () => {},
        displayGroups: {
          auto: { label: 'AUTO' },
          all: { label: 'ALL' },
          default: { label: 'DEFAULT' },
        },
        groupRatio: {
          auto: 1,
          default: 3,
        },
        models: [
          { model_name: 'alpha', enable_groups: ['all', 'auto'] },
          { model_name: 'beta', enable_groups: ['all', 'default'] },
        ],
        loading: false,
        t: (value) => value,
      }),
    );

    const text = getText(renderer.toJSON());
    expect(text).toContain('模型分组');
    expect(text).toContain('active:__all__');
    expect(text).toContain('__all__|全部分组|');
    expect(text).toContain('all|all|');
    expect(text).toContain('default|default|3x');
    expect(text).not.toContain('all|all|1x');
    expect(text).not.toContain('auto|auto|1x');
    expect(text).not.toContain('usableGroup');
  });

  test('PricingSidebar and FilterModalContent pass displayGroups', () => {
    const sidebarSource = readSource(
      '../src/components/table/model-pricing/layout/PricingSidebar.jsx',
    );
    const modalSource = readSource(
      '../src/components/table/model-pricing/modal/components/FilterModalContent.jsx',
    );

    expect(sidebarSource).toMatch(
      /displayGroups=\{categoryProps\.displayGroups\}/,
    );
    expect(modalSource).toMatch(
      /displayGroups=\{categoryProps\.displayGroups\}/,
    );
    expect(sidebarSource).not.toMatch(/\busableGroup\b/);
    expect(modalSource).not.toMatch(/\busableGroup\b/);
  });

  test('ModelPricingTable renders backend all/default groups and still excludes auto', async () => {
    mock.module('@douyinfe/semi-ui', () => ({
      Card: ({ children }) => h('section', null, children),
      Avatar: ({ children }) => h('span', null, children),
      Typography: {
        Text: ({ children }) => h('span', null, children),
      },
      Table: ({ dataSource, columns }) =>
        h(
          'table',
          null,
          dataSource.map((row) =>
            h(
              'tr',
              { key: row.key },
              columns.map((column) => {
                const value = row[column.dataIndex];
                const rendered = column.render ? column.render(value, row) : value;
                return h('td', { key: `${row.key}-${column.dataIndex}` }, rendered);
              }),
            ),
          ),
        ),
      Tag: ({ children }) => h('span', null, children),
    }));
    mock.module('@douyinfe/semi-icons', () => ({
      IconCoinMoneyStroked: () => h('span', null, 'coin'),
    }));
    mock.module('../src/helpers/index.js', () => ({
      calculateModelPrice: ({ selectedGroup }) => ({
        inputPrice: `${selectedGroup}-input`,
        outputPrice: `${selectedGroup}-output`,
        price: `${selectedGroup}-price`,
      }),
      getModelPriceItems: (priceData) => [
        {
          key: 'summary',
          label: 'price',
          value: priceData.price,
          suffix: 'suffix',
        },
      ],
    }));

    const { default: ModelPricingTable } = await importFresh(
      '../src/components/table/model-pricing/modal/components/ModelPricingTable.jsx',
      'model-pricing-table',
    );

    const renderer = await renderElement(
      h(ModelPricingTable, {
        modelData: {
          model_name: 'demo',
          quota_type: 0,
          enable_groups: ['all', 'default', 'team', 'auto'],
        },
        groupRatio: {
          auto: 1,
          all: 2,
          default: 3,
          team: 4,
        },
        currency: 'USD',
        siteDisplayType: 'USD',
        tokenUnit: 'M',
        displayPrice: (value) => `$${value}`,
        showRatio: true,
        displayGroups: {
          auto: { label: 'AUTO' },
          all: { label: 'ALL' },
          default: { label: 'DEFAULT' },
        },
        t: (value) => value,
      }),
    );

    const text = getText(renderer.toJSON());
    expect(text).toContain('分组价格');
    expect(text).toContain('all分组');
    expect(text).toContain('default分组');
    expect(text).toContain('price all-price');
    expect(text).toContain('price default-price');
    expect(text).not.toContain('team分组');
    expect(text).not.toContain('auto分组');
    expect(text).not.toContain('price auto-price');
    expect(text).not.toContain('auto分组调用链路');
  });

  test('calculateModelPrice keeps concrete all separate from synthetic cheapest-group mode', async () => {
    const {
      calculateModelPrice,
      PRICING_GROUP_ALL_SENTINEL,
    } = await importPricingUtilsSubset();

    const baseRecord = {
      quota_type: 1,
      model_price: 10,
      enable_groups: ['all', 'default'],
    };

    const concreteAll = calculateModelPrice({
      record: baseRecord,
      selectedGroup: 'all',
      groupRatio: { default: 0.5 },
      tokenUnit: 'M',
      displayPrice: (value) => `$${value}`,
      currency: 'USD',
    });

    const bestPrice = calculateModelPrice({
      record: baseRecord,
      selectedGroup: PRICING_GROUP_ALL_SENTINEL,
      groupRatio: { default: 0.5 },
      tokenUnit: 'M',
      displayPrice: (value) => `$${value}`,
      currency: 'USD',
    });

    expect(concreteAll.usedGroup).toBe('all');
    expect(concreteAll.usedGroupRatio).toBe(1);
    expect(bestPrice.usedGroup).toBe('default');
    expect(bestPrice.usedGroupRatio).toBe(0.5);
  });

  test('PricingPage and detail sheet use displayGroups without legacy marketplace props', () => {
    const sheetSource = readSource(
      '../src/components/table/model-pricing/modal/ModelDetailSideSheet.jsx',
    );
    const pageSource = readSource(
      '../src/components/table/model-pricing/layout/PricingPage.jsx',
    );

    expect(sheetSource).toMatch(/displayGroups/);
    expect(pageSource).toMatch(/displayGroups/);
    expect(sheetSource).not.toMatch(/\busableGroup\b/);
    expect(sheetSource).not.toMatch(/\bautoGroups\b/);
  });
});
