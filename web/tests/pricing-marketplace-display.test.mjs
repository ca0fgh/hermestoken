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

async function importResetPricingUtilsSubset() {
  const source = readSource('../src/helpers/utils.jsx');
  const sentinelLine = "export const PRICING_GROUP_ALL_SENTINEL = '__all__';";
  const defaultsStart = source.indexOf('const DEFAULT_PRICING_FILTERS = {');
  const resetEndMarker =
    "  setCurrentPage?.(DEFAULT_PRICING_FILTERS.currentPage);\n};";
  const resetEnd = source.indexOf(resetEndMarker, defaultsStart);
  const subset = [
    sentinelLine,
    source.slice(defaultsStart, resetEnd + resetEndMarker.length),
  ].join('\n\n');
  const tempPath = path.join(
    tmpdir(),
    `pricing-reset-subset-${Date.now()}-${importCounter}.mjs`,
  );
  writeFileSync(tempPath, subset, 'utf8');
  try {
    return await import(`file://${tempPath}`);
  } finally {
    unlinkSync(tempPath);
  }
}

async function importModelsColumnDefsSubset() {
  const source = readSource('../src/components/table/models/ModelsColumnDefs.jsx');
  const rewrittenSource = source
    .replace(
      "import React from 'react';",
      "import React from 'react';\nconst Button = ({ children, onClick }) => React.createElement('button', { onClick }, children);\nconst Space = ({ children }) => React.createElement('div', null, children);\nconst Tag = ({ children }) => React.createElement('span', null, children);\nconst Typography = { Text: ({ children }) => React.createElement('span', null, children) };\nconst Modal = { confirm: () => {} };\nconst Tooltip = ({ children }) => React.createElement('span', null, children);",
    )
    .replace(
      /import\s*\{\s*Button,\s*Space,\s*Tag,\s*Typography,\s*Modal,\s*Tooltip,\s*\}\s*from '@douyinfe\/semi-ui';\n/,
      '',
    )
    .replace(
      /import\s*\{\s*timestamp2string,\s*getLobeHubIcon,\s*stringToColor,\s*\}\s*from '\.\.\/\.\.\/\.\.\/helpers';\n/,
      "const timestamp2string = (value) => `ts:${value}`;\nconst getLobeHubIcon = (iconKey) => React.createElement('i', null, iconKey);\nconst stringToColor = () => 'blue';\n",
    )
    .replace(
      /import\s*\{\s*renderLimitedItems,\s*renderDescription,\s*\}\s*from '\.\.\/\.\.\/common\/ui\/RenderUtils';\n/,
      "const renderLimitedItems = ({ items, renderItem }) => React.createElement('div', null, items.map((item, index) => renderItem(item, index)));\nconst renderDescription = (text) => text;\n",
    );

  const tempPath = path.join(
    process.cwd(),
    'tests',
    `models-column-defs-subset-${Date.now()}-${importCounter}.jsx`,
  );
  writeFileSync(tempPath, rewrittenSource, 'utf8');
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
    expect(text).toContain('all|all|1x');
    expect(text).toContain('default|default|3x');
    expect(text).not.toContain('__all__|全部分组|1x');
    expect(text).not.toContain('auto|auto|1x');
    expect(text).not.toContain('usableGroup');
  });

  test('PricingSidebar, FilterModalContent, and PricingFilterModal thread reset/display props', () => {
    const sidebarSource = readSource(
      '../src/components/table/model-pricing/layout/PricingSidebar.jsx',
    );
    const modalSource = readSource(
      '../src/components/table/model-pricing/modal/components/FilterModalContent.jsx',
    );
    const filterModalSource = readSource(
      '../src/components/table/model-pricing/modal/PricingFilterModal.jsx',
    );

    expect(sidebarSource).toMatch(
      /displayGroups=\{categoryProps\.displayGroups\}/,
    );
    expect(modalSource).toMatch(
      /displayGroups=\{categoryProps\.displayGroups\}/,
    );
    expect(modalSource).toMatch(/setFilterGroup=\{handleGroupClick\}/);
    expect(sidebarSource).toMatch(/setSelectedGroup:\s*categoryProps\.setSelectedGroup/);
    expect(filterModalSource).toMatch(
      /setSelectedGroup:\s*sidebarProps\.setSelectedGroup/,
    );
    expect(sidebarSource).not.toMatch(/\busableGroup\b/);
    expect(modalSource).not.toMatch(/\busableGroup\b/);
  });

  test('FilterModalContent uses the desktop group-click handler semantics on mobile', async () => {
    const handleGroupClickCalls = [];
    const setFilterGroupCalls = [];

    mock.module(
      '../src/hooks/model-pricing/usePricingFilterCounts.js',
      () => ({
        usePricingFilterCounts: () => ({
          quotaTypeModels: [],
          endpointTypeModels: [],
          vendorModels: [],
          tagModels: [],
          groupCountModels: [],
        }),
      }),
    );
    mock.module(
      '../src/components/table/model-pricing/filter/PricingDisplaySettings.jsx',
      () => ({
        default: () => h('div', null, 'display-settings'),
      }),
    );
    mock.module(
      '../src/components/table/model-pricing/filter/PricingGroups.jsx',
      () => ({
        default: ({ setFilterGroup }) =>
          h(
            'button',
            {
              onClick: () => setFilterGroup('default'),
              type: 'button',
            },
            'choose-group',
          ),
      }),
    );
    mock.module(
      '../src/components/table/model-pricing/filter/PricingQuotaTypes.jsx',
      () => ({
        default: () => h('div', null, 'quota-types'),
      }),
    );
    mock.module(
      '../src/components/table/model-pricing/filter/PricingEndpointTypes.jsx',
      () => ({
        default: () => h('div', null, 'endpoint-types'),
      }),
    );
    mock.module(
      '../src/components/table/model-pricing/filter/PricingVendors.jsx',
      () => ({
        default: () => h('div', null, 'vendors'),
      }),
    );
    mock.module(
      '../src/components/table/model-pricing/filter/PricingTags.jsx',
      () => ({
        default: () => h('div', null, 'tags'),
      }),
    );

    const { default: FilterModalContent } = await importFresh(
      '../src/components/table/model-pricing/modal/components/FilterModalContent.jsx',
      'filter-modal-content',
    );

    const renderer = await renderElement(
      h(FilterModalContent, {
        sidebarProps: {
          showWithRecharge: false,
          setShowWithRecharge: () => {},
          currency: 'USD',
          setCurrency: () => {},
          siteDisplayType: 'USD',
          handleChange: () => {},
          setActiveKey: () => {},
          showRatio: false,
          setShowRatio: () => {},
          viewMode: 'card',
          setViewMode: () => {},
          filterGroup: '__all__',
          setFilterGroup: (value) => setFilterGroupCalls.push(value),
          handleGroupClick: (value) => handleGroupClickCalls.push(value),
          filterQuotaType: 'all',
          setFilterQuotaType: () => {},
          filterEndpointType: 'all',
          setFilterEndpointType: () => {},
          filterVendor: 'all',
          setFilterVendor: () => {},
          filterTag: 'all',
          setFilterTag: () => {},
          tokenUnit: 'M',
          setTokenUnit: () => {},
          loading: false,
          models: [],
          displayGroups: {
            default: {},
          },
          groupRatio: {
            default: 1,
          },
          searchValue: '',
        },
        t: (value) => value,
      }),
    );

    await act(async () => {
      renderer.root.findByType('button').props.onClick();
    });

    expect(handleGroupClickCalls).toEqual(['default']);
    expect(setFilterGroupCalls).toEqual([]);
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

  test('resetPricingFilters resets selectedGroup back to the synthetic all sentinel', async () => {
    const { resetPricingFilters, PRICING_GROUP_ALL_SENTINEL } =
      await importResetPricingUtilsSubset();

    const calls = [];
    resetPricingFilters({
      handleChange: (value) => calls.push(['search', value]),
      setShowWithRecharge: (value) => calls.push(['showWithRecharge', value]),
      setCurrency: (value) => calls.push(['currency', value]),
      setShowRatio: (value) => calls.push(['showRatio', value]),
      setViewMode: (value) => calls.push(['viewMode', value]),
      setSelectedGroup: (value) => calls.push(['selectedGroup', value]),
      setFilterGroup: (value) => calls.push(['filterGroup', value]),
      setFilterQuotaType: (value) => calls.push(['filterQuotaType', value]),
      setFilterEndpointType: (value) =>
        calls.push(['filterEndpointType', value]),
      setFilterVendor: (value) => calls.push(['filterVendor', value]),
      setFilterTag: (value) => calls.push(['filterTag', value]),
      setCurrentPage: (value) => calls.push(['currentPage', value]),
      setTokenUnit: (value) => calls.push(['tokenUnit', value]),
    });

    expect(calls).toContainEqual(['selectedGroup', PRICING_GROUP_ALL_SENTINEL]);
    expect(calls).toContainEqual(['filterGroup', PRICING_GROUP_ALL_SENTINEL]);
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

  test('model admin source uses marketplace display terminology for modal and table copy', () => {
    const editModalSource = readSource(
      '../src/components/table/models/modals/EditModelModal.jsx',
    );
    const columnDefsSource = readSource(
      '../src/components/table/models/ModelsColumnDefs.jsx',
    );

    expect(editModalSource).toMatch(/label=\{t\('模型广场展示'\)\}/);
    expect(editModalSource).toMatch(
      /不会影响模型的实际调用与路由/,
    );
    expect(columnDefsSource).toMatch(/title: t\('广场展示'\)/);
  });

  test('getModelsColumns exposes marketplace display labels for the display column and row actions', async () => {
    const { getModelsColumns } = await importModelsColumnDefsSubset();

    const manageModelCalls = [];
    const columns = getModelsColumns({
      t: (value) => value,
      manageModel: (...args) => manageModelCalls.push(args),
      setEditingModel: () => {},
      setShowEdit: () => {},
      refresh: async () => {},
      vendorMap: {},
    });

    const displayColumn = columns.find((column) => column.dataIndex === 'status');
    const operationsColumn = columns.find((column) => column.dataIndex === 'operate');

    expect(displayColumn.title).toBe('广场展示');

    const displayShown = await renderElement(
      displayColumn.render(1, { status: 1 }),
    );
    expect(getText(displayShown.toJSON())).toContain('展示');

    const displayHidden = await renderElement(
      displayColumn.render(0, { status: 0 }),
    );
    expect(getText(displayHidden.toJSON())).toContain('隐藏');

    const shownActions = await renderElement(
      operationsColumn.render(null, { id: 11, status: 1 }),
    );
    const shownButtons = shownActions.root.findAllByType('button');
    expect(getText(shownActions.toJSON())).toContain('隐藏');
    expect(getText(shownActions.toJSON())).not.toContain('禁用');

    await act(async () => {
      shownButtons[0].props.onClick();
    });
    expect(manageModelCalls).toContainEqual([11, 'disable', { id: 11, status: 1 }]);

    const hiddenActions = await renderElement(
      operationsColumn.render(null, { id: 12, status: 0 }),
    );
    expect(getText(hiddenActions.toJSON())).toContain('展示');
    expect(getText(hiddenActions.toJSON())).not.toContain('启用');
  });
});
