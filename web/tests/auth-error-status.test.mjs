import assert from 'node:assert/strict';
import test from 'node:test';

const authErrorModulePath = new URL(
  '../classic/src/helpers/authError.js',
  import.meta.url,
);

// Verbatim message a failing channel test returns (HTTP 200, success:false) when
// the upstream relay rejects the key. Quoting the upstream's own 401 must not be
// read as OUR session expiring.
const UPSTREAM_401_TEST_MESSAGE =
  'bad response status code 401, message: 额度不足, body: {"error":{"message":"额度不足","type":"authentication_error"},"type":"error"}';

test('an upstream status quoted in a backend message is not our session expiring', async () => {
  const { isUnauthorizedError, getHttpStatusFromError } =
    await import(authErrorModulePath);

  assert.equal(isUnauthorizedError(UPSTREAM_401_TEST_MESSAGE), false);
  assert.equal(getHttpStatusFromError(UPSTREAM_401_TEST_MESSAGE), undefined);
});

test('an upstream status quoted in a backend message does not hijack the error toast', async () => {
  const { getHttpStatusFromError } = await import(authErrorModulePath);

  // showError() switches on this status to swap in a canned toast ("请求次数过多",
  // "服务器内部错误"), which would bury the real upstream failure.
  for (const status of [429, 500, 405]) {
    assert.equal(
      getHttpStatusFromError(`bad response status code ${status}, message: x`),
      undefined,
    );
  }
});

test('a real 401 response still redirects to login', async () => {
  const { isUnauthorizedError, getHttpStatusFromError } =
    await import(authErrorModulePath);

  // Shape of an axios error for an expired session.
  const axiosError = new Error('Request failed with status code 401');
  axiosError.response = { status: 401 };
  assert.equal(isUnauthorizedError(axiosError), true);

  assert.equal(getHttpStatusFromError({ status: 401 }), 401);
  // Axios' own message, when a caller passes only error.message through.
  assert.equal(isUnauthorizedError('Request failed with status code 401'), true);
});

test('a non-401 response is reported, not treated as expired', async () => {
  const { isUnauthorizedError, getHttpStatusFromError } =
    await import(authErrorModulePath);

  const forbidden = new Error('Request failed with status code 403');
  forbidden.response = { status: 403 };
  assert.equal(isUnauthorizedError(forbidden), false);
  assert.equal(getHttpStatusFromError(forbidden), 403);
});
