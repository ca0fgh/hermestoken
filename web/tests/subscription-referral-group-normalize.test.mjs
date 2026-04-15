import test from 'node:test';
import assert from 'node:assert/strict';

import { normalizeGroupNames } from '../src/helpers/subscriptionReferral.js';

test('normalizeGroupNames accepts canonical group catalog objects', () => {
  const normalized = normalizeGroupNames([
    { group_key: 'premium', display_name: 'Premium' },
    { group: 'default' },
    'vip',
  ]);

  assert.deepEqual(normalized, ['default', 'premium', 'vip']);
});
