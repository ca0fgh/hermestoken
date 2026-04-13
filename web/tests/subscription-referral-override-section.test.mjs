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

test('SubscriptionReferralOverrideSection source uses list-based override editor contracts', () => {
  assert.match(
    componentSource,
    /API\.get\(`\/api\/subscription\/admin\/referral\/users\/\$\{userId\}`\)/,
  );
  assert.match(
    componentSource,
    /buildAdminOverrideRows\(\s*Array\.isArray\(next\.groups\) \? next\.groups : \[\],\s*\)/,
  );
  assert.match(componentSource, /group,\s*total_rate_bps:\s*totalRateBps/);
  assert.match(componentSource, /params:\s*\{\s*group\s*\}/);
  assert.match(componentSource, /t\('当前生效总返佣率'\)/);
  assert.match(componentSource, /t\('覆盖总返佣率'\)/);
  assert.match(componentSource, /t\('清除覆盖'\)/);
  assert.match(
    componentSource,
    /onChange=\{\(value\) => updateInputPercent\(row\.group,\s*value\)\}/,
  );
  assert.doesNotMatch(componentSource, /API\.get\(['"`]\/api\/group\/?['"`]\)/);
  assert.doesNotMatch(componentSource, /<Select/);
  assert.doesNotMatch(componentSource, /t\('新增覆盖'\)/);
});

test('SubscriptionReferralOverrideSection source keeps override row controls within the modal width', () => {
  assert.match(
    componentSource,
    /className='flex flex-col gap-3'/,
  );
  assert.match(
    componentSource,
    /className='rounded-xl border border-gray-100 p-4'/,
  );
  assert.match(
    componentSource,
    /<InputNumber[\s\S]*?style=\{\{ width: '100%' \}\}/,
  );
});
