import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import test from 'node:test';

const noticeHtmlHelperPath = new URL(
  '../src/helpers/noticeHtml.js',
  import.meta.url,
);
const noticeModalPath = new URL(
  '../src/components/layout/NoticeModal.jsx',
  import.meta.url,
);

test('sanitizeNoticeHtml strips global style injections while preserving notice markup', async () => {
  const { sanitizeNoticeHtml } = await import(noticeHtmlHelperPath);

  const sanitized = sanitizeNoticeHtml(`
    <style>body { font-family: "Times New Roman" !important; }</style>
    <link rel="stylesheet" href="https://example.com/fonts.css">
    <h2>系统公告</h2>
    <p><strong>正常内容</strong> 需要保留。</p>
  `);

  assert.doesNotMatch(sanitized, /<style/i);
  assert.doesNotMatch(sanitized, /<link/i);
  assert.match(sanitized, /<h2>系统公告<\/h2>/);
  assert.match(sanitized, /<strong>正常内容<\/strong>/);
});

test('NoticeModal sanitizes injected notice html before rendering', async () => {
  const source = await readFile(noticeModalPath, 'utf8');

  assert.match(source, /sanitizeNoticeHtml/);
});
