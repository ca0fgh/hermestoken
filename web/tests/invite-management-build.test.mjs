import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import test from 'node:test';

test('invite management slice survives a real frontend production build', () => {
  const result = spawnSync('npm', ['run', 'build'], {
    cwd: new URL('..', import.meta.url),
    encoding: 'utf8',
    env: { ...process.env, BROWSERSLIST_IGNORE_OLD_DATA: 'true' },
  });

  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.match(result.stdout, /vite build/);
  assert.match(result.stdout + result.stderr, /built in/);
});
