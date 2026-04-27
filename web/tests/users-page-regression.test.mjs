import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

function readSource(relativePath) {
  return readFileSync(new URL(`../src/${relativePath}`, import.meta.url), 'utf8');
}

test('UsersTable keeps activePage in scope when wiring DeleteUserModal', () => {
  const source = readSource('components/table/users/UsersTable.jsx');

  assert.match(
    source,
    /const\s*\{\s*[\s\S]*?activePage,\s*[\s\S]*?\}\s*=\s*usersData;/,
  );
  assert.match(source, /<DeleteUserModal[\s\S]*activePage=\{activePage\}/);
});

test('user quota column uses wallet-specific usage instead of global used quota', () => {
  const source = readSource('components/table/users/UsersColumnDefs.jsx');

  assert.match(source, /record\.wallet_amount_used/);
  assert.doesNotMatch(
    source,
    /const\s+walletUsed\s*=\s*parseInt\(record\.used_quota\)/,
  );
});
