import assert from 'node:assert/strict';
import test from 'node:test';

import { buildTokenPayload } from '../src/components/table/tokens/modals/tokenPayload.js';

test('buildTokenPayload keeps auto mode out of fixed group key fields', () => {
  assert.deepEqual(
    buildTokenPayload({
      name: 'demo',
      selection_mode: 'auto',
      group_key: 'premium',
      group: 'premium',
      cross_group_retry: true,
    }),
    {
      name: 'demo',
      selection_mode: 'auto',
      group_key: '',
      group: '',
      cross_group_retry: true,
    },
  );
});

test('buildTokenPayload keeps inherit mode free of explicit group assignment', () => {
  assert.deepEqual(
    buildTokenPayload({
      name: 'demo',
      selection_mode: 'inherit_user_default',
      group_key: 'premium',
      group: 'premium',
      cross_group_retry: false,
    }),
    {
      name: 'demo',
      selection_mode: 'inherit_user_default',
      group_key: '',
      group: '',
      cross_group_retry: false,
    },
  );
});

test('buildTokenPayload preserves fixed mode canonical group key', () => {
  assert.deepEqual(
    buildTokenPayload({
      name: 'demo',
      selection_mode: 'fixed',
      group_key: 'premium',
      group: 'legacy-premium',
      cross_group_retry: false,
    }),
    {
      name: 'demo',
      selection_mode: 'fixed',
      group_key: 'premium',
      group: 'premium',
      cross_group_retry: false,
    },
  );
});
