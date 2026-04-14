/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const componentSource = readFileSync(
  new URL(
    '../src/components/table/users/modals/SubscriptionReferralOverrideSection.jsx',
    import.meta.url,
  ),
  'utf8',
);

test('SubscriptionReferralOverrideSection loads group catalog and renders extensible override controls', () => {
  assert.match(
    componentSource,
    /API\.get\(`\/api\/subscription\/admin\/referral\/users\/\$\{userId\}`\)/,
  );
  assert.match(componentSource, /API\.get\(['"`]\/api\/group\/['"`]\)/);
  assert.match(
    componentSource,
    /buildAdminOverrideRows\(\s*Array\.isArray\(next\.groups\) \? next\.groups : \[\],\s*\)/,
  );
  assert.match(componentSource, /createAdminOverrideDraftRow\(\)/);
  assert.match(componentSource, /buildAdminOverrideGroupOptions\(/);
  assert.match(componentSource, /<Select/);
  assert.match(componentSource, /t\('新增覆盖'\)/);
  assert.match(componentSource, /t\('返佣类型'\)/);
  assert.doesNotMatch(
    componentSource,
    /API\.get\(['"`]\/api\/subscription\/admin\/referral\/settings['"`]\)/,
  );
});

test('SubscriptionReferralOverrideSection copy reflects override list UX', () => {
  assert.match(componentSource, /t\('暂无覆盖时使用默认返佣规则'\)/);
  assert.match(componentSource, /t\('暂无覆盖项，未设置时使用默认返佣规则'\)/);
  assert.match(componentSource, /t\('当前默认总返佣率'\)/);
  assert.match(componentSource, /t\('取消'\)/);
  assert.match(componentSource, /t\('删除'\)/);
  assert.doesNotMatch(
    componentSource,
    /t\('未设置覆盖时，该分组不启用订阅返佣'\)/,
  );
  assert.doesNotMatch(componentSource, /t\('当前未启用该分组订阅返佣'\)/);
});
