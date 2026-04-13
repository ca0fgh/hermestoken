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

test('SubscriptionReferralOverrideSection source uses grouped list-based override editor contracts', () => {
  assert.match(
    componentSource,
    /API\.get\(['"`]\/api\/subscription\/admin\/referral\/settings['"`]\)/,
  );
  assert.match(componentSource, /t\('新增覆盖'\)/);
  assert.match(componentSource, /t\('返佣类型'\)/);
  assert.match(componentSource, /t\('订阅返佣'\)/);
  assert.match(componentSource, /overrideRows\.length === 0/);
  assert.match(componentSource, /t\('暂无覆盖项，未设置时使用默认返佣规则'\)/);
  assert.match(componentSource, /row\.isDraft \? t\('取消'\) : t\('删除'\)/);
  assert.match(componentSource, /settingsData\.group_rates/);
  assert.match(componentSource, /\.filter\(\(row\) => row\.hasOverride\)/);
  assert.match(
    componentSource,
    /onChange=\{\(value\)\s*=>\s*updateRow\(row\.id,\s*\{\s*group:\s*value,\s*effectiveTotalRateBps:\s*getDefaultRateBpsByGroup\(value\)/,
  );
  assert.match(
    componentSource,
    /value=\{row\.type\}[\s\S]*?disabled=\{!row\.isDraft \|\| loading \|\| isSaving\}/,
  );
  assert.match(
    componentSource,
    /value=\{row\.group \|\| undefined\}[\s\S]*?disabled=\{!row\.isDraft \|\| loading \|\| isSaving\}/,
  );
  assert.match(
    componentSource,
    /disabled=\{\s*\(!row\.isDraft && !row\.hasOverride\)\s*\|\|\s*loading\s*\|\|\s*isSaving\s*\}/,
  );
  assert.doesNotMatch(componentSource, /const fallbackGroups =/);
  assert.doesNotMatch(componentSource, /settingsData\.total_rate_bps/);
  assert.doesNotMatch(componentSource, /t\('清除覆盖'\)/);
});
