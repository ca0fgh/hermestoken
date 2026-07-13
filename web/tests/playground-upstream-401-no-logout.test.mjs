import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

const read = (path) => readFileSync(new URL(path, import.meta.url), 'utf8');

const api = read('../default/src/lib/api.ts');
const main = read('../default/src/main.tsx');
const playground = read('../../controller/playground.go');

// /pg/chat/completions authenticates with the operator's session cookie and then
// relays the upstream's HTTP status verbatim. A channel whose provider key was
// revoked or ran out of credit answers 401 — and the frontend used to read that as
// its own session expiring and wipe the auth store. Worse, the playground passes
// skipErrorHandler, which suppressed the toast but not the reset, so the operator
// was logged out with no explanation at all.
test('an upstream 401 relayed by the playground is not treated as our session expiring', () => {
  assert.match(
    api,
    /export function isRelayedUpstreamError\(error: unknown\): boolean/,
    'the frontend needs one place that decides whether a status is ours',
  );
  assert.match(
    api,
    /body\?\.error_source === 'upstream'/,
    'the discriminator is the marker the backend sets, not a guess about the message',
  );
  assert.match(
    api,
    /if \(status === 401 && !isRelayedUpstreamError\(error\)\) \{/,
    'the axios interceptor must not reset auth on a relayed upstream 401',
  );
  assert.match(
    main,
    /error\.response\?\.status === 401 && !isRelayedUpstreamError\(error\)/,
    'the react-query cache handler redirects to /sign-in and needs the same guard',
  );
});

test('the backend marks playground errors as coming from upstream', () => {
  assert.match(
    playground,
    /"error_source": "upstream"/,
    'the frontend guard is worthless if the backend stops sending the marker',
  );
});
