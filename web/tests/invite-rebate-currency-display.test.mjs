import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';

function readSource(relativePath) {
  return readFileSync(new URL(`../${relativePath}`, import.meta.url), 'utf8');
}

test('invite rebate earnings displays reuse quota currency rendering helpers', () => {
  const summarySource = readSource(
    'src/components/invite-rebate/InviteRebateSummary.jsx',
  );
  const listSource = readSource('src/components/invite-rebate/InviteeListPanel.jsx');

  assert.match(
    summarySource,
    /import\s+\{\s*renderQuota\s*\}\s+from\s+['"]\.\.\/\.\.\/helpers\/quota['"]/,
  );
  assert.match(summarySource, /renderQuota\(totalContributionQuota\)/);

  assert.match(
    listSource,
    /import\s+\{\s*renderQuota\s*\}\s+from\s+['"]\.\.\/\.\.\/helpers\/quota['"]/,
  );
  assert.match(listSource, /renderQuota\(invitee\.contribution_quota \|\| 0\)/);
});
