import test from 'node:test';
import assert from 'node:assert/strict';

import {
  getSyncedDraftValue,
  shouldCommitDraftValue,
} from '../classic/src/pages/Setting/Ratio/components/groupTableDraft.js';

test('getSyncedDraftValue keeps the local draft while IME composition is active', () => {
  const nextDraft = getSyncedDraftValue({
    committedValue: 'group_1',
    draftValue: '中文',
    isComposing: true,
  });

  assert.equal(nextDraft, '中文');
});

test('getSyncedDraftValue syncs the local draft after composition ends', () => {
  const nextDraft = getSyncedDraftValue({
    committedValue: '中文分组',
    draftValue: 'zhong',
    isComposing: false,
  });

  assert.equal(nextDraft, '中文分组');
});

test('shouldCommitDraftValue only commits when the draft changed', () => {
  assert.equal(
    shouldCommitDraftValue({
      committedValue: 'default',
      draftValue: 'default',
    }),
    false,
  );

  assert.equal(
    shouldCommitDraftValue({
      committedValue: 'default',
      draftValue: '默认分组',
    }),
    true,
  );
});
