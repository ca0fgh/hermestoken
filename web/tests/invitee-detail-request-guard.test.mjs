import test from 'node:test';
import assert from 'node:assert/strict';
import {
  createInviteeDetailRequestGuard,
  resolveInviteeSelectionAfterPageRefresh,
} from '../classic/src/helpers/inviteeDetailRequestGuard.js';

test('resolveInviteeSelectionAfterPageRefresh auto-selects the first invitee when nothing is selected', () => {
  const requestGuard = createInviteeDetailRequestGuard();
  const firstInvitee = { id: 11, username: 'alice' };
  const secondInvitee = { id: 12, username: 'bob' };

  const nextInvitee = resolveInviteeSelectionAfterPageRefresh({
    currentInvitee: null,
    nextItems: [firstInvitee, secondInvitee],
    requestGuard,
  });

  assert.deepEqual(nextInvitee, firstInvitee);
});

test('resolveInviteeSelectionAfterPageRefresh falls back to the first remaining invitee when the current one disappears', () => {
  const requestGuard = createInviteeDetailRequestGuard();
  const firstInvitee = { id: 21, username: 'carol' };
  const secondInvitee = { id: 22, username: 'dave' };
  let clearCount = 0;

  const nextInvitee = resolveInviteeSelectionAfterPageRefresh({
    currentInvitee: { id: 99, username: 'missing' },
    nextItems: [firstInvitee, secondInvitee],
    requestGuard,
    onSelectionCleared: () => {
      clearCount += 1;
    },
  });

  assert.deepEqual(nextInvitee, firstInvitee);
  assert.equal(clearCount, 0);
});

test('resolveInviteeSelectionAfterPageRefresh clears the selection only when no invitees remain', () => {
  const requestGuard = createInviteeDetailRequestGuard();
  let clearCount = 0;

  const nextInvitee = resolveInviteeSelectionAfterPageRefresh({
    currentInvitee: { id: 31, username: 'eve' },
    nextItems: [],
    requestGuard,
    onSelectionCleared: () => {
      clearCount += 1;
    },
  });

  assert.equal(nextInvitee, null);
  assert.equal(clearCount, 1);
});
