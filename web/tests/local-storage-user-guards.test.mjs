import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';

const utilsPath = new URL('../classic/src/helpers/utils.jsx', import.meta.url);
const apiPath = new URL('../classic/src/helpers/api.js', import.meta.url);
const authErrorPath = new URL(
  '../classic/src/helpers/authError.js',
  import.meta.url,
);
const authPath = new URL('../classic/src/helpers/auth.jsx', import.meta.url);
const notificationsPath = new URL(
  '../classic/src/helpers/notifications.jsx',
  import.meta.url,
);
const userInfoHeaderPath = new URL(
  '../classic/src/components/settings/personal/components/UserInfoHeader.jsx',
  import.meta.url,
);

test('user-state helpers guard malformed localStorage.user before rendering', () => {
  const utilsSource = readFileSync(utilsPath, 'utf8');
  const authSource = readFileSync(authPath, 'utf8');
  const userInfoHeaderSource = readFileSync(userInfoHeaderPath, 'utf8');

  assert.match(utilsSource, /function readStoredUser\(\)/);
  assert.match(utilsSource, /try \{\s*return JSON\.parse\(rawUser\);\s*\} catch \{\s*return null;\s*\}/s);
  assert.match(utilsSource, /const user = readStoredUser\(\);/);
  assert.match(utilsSource, /return user\.role >= 10;/);
  assert.match(utilsSource, /return user\.role >= 100;/);
  assert.match(utilsSource, /return user\.id;/);

  assert.match(authSource, /try \{\s*const user = JSON\.parse\(raw\);\s*if \(user && typeof user\.role === 'number' && user\.role >= 10\)/s);

  assert.doesNotMatch(userInfoHeaderSource, /import\s+\{\s*isRoot,\s*isAdmin,\s*renderQuota,\s*stringToColor\s*\}\s+from\s+'\.{4}\/\.{4}\/\.{4}\/\.{4}\/helpers';/);
  assert.match(userInfoHeaderSource, /const userRole = Number\(userState\?\.user\?\.role\) \|\| 0;/);
  assert.match(userInfoHeaderSource, /const isRootUser = userRole >= 100;/);
  assert.match(userInfoHeaderSource, /const isAdminUser = userRole >= 10;/);
});

test('auth-expired errors redirect without showing raw axios 401 text', () => {
  const apiSource = readFileSync(apiPath, 'utf8');
  const authErrorSource = readFileSync(authErrorPath, 'utf8');
  const notificationsSource = readFileSync(notificationsPath, 'utf8');
  const utilsSource = readFileSync(utilsPath, 'utf8');

  assert.match(authErrorSource, /AUTH_EXPIRED_REDIRECT_PATH\s*=\s*'\/login\?expired=true'/);
  assert.match(authErrorSource, /status code\\s\+\(\?<status>\\d\{3\}\)/);
  assert.match(authErrorSource, /localStorage\.removeItem\('user'\)/);

  assert.match(
    apiSource,
    /isUnauthorizedError\(error\)[\s\S]*redirectToLoginWhenExpired\(\)[\s\S]*skipErrorHandler/,
  );
  assert.match(
    notificationsSource,
    /isUnauthorizedError\(error\)[\s\S]*redirectToLoginWhenExpired\(\)/,
  );
  assert.match(
    utilsSource,
    /isUnauthorizedError\(error\)[\s\S]*redirectToLoginWhenExpired\(\)/,
  );
});
