import test from 'node:test';
import assert from 'node:assert/strict';

import { createInviteeDetailRequestGuard } from '../src/helpers/inviteeDetailRequestGuard.js';

test('createInviteeDetailRequestGuard ignores stale invitee detail responses', async () => {
  const guard = createInviteeDetailRequestGuard();
  const appliedInvitees = [];
  let resolveInviteeA;
  let resolveInviteeB;

  const inviteeARequest = guard.begin(101);
  const inviteeAPromise = new Promise((resolve) => {
    resolveInviteeA = () => {
      if (guard.isCurrent(inviteeARequest)) {
        appliedInvitees.push('invitee-a');
      }
      resolve();
    };
  });

  const inviteeBRequest = guard.begin(202);
  const inviteeBPromise = new Promise((resolve) => {
    resolveInviteeB = () => {
      if (guard.isCurrent(inviteeBRequest)) {
        appliedInvitees.push('invitee-b');
      }
      resolve();
    };
  });

  resolveInviteeB();
  resolveInviteeA();

  await Promise.all([inviteeAPromise, inviteeBPromise]);

  assert.deepEqual(appliedInvitees, ['invitee-b']);
});

test('createInviteeDetailRequestGuard invalidates in-flight detail requests when selection clears', () => {
  const guard = createInviteeDetailRequestGuard();
  const request = guard.begin(303);

  guard.clear();

  assert.equal(guard.isCurrent(request), false);
});
