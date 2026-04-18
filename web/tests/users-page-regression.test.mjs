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
