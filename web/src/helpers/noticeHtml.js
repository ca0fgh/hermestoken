/*
Copyright (C) 2025 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

const STYLE_TAG_PATTERN = /<style\b[^>]*>[\s\S]*?<\/style>/gi;
const STYLESHEET_LINK_PATTERN =
  /<link\b(?=[^>]*\brel\s*=\s*["']?stylesheet["']?)[^>]*>/gi;
const DOCUMENT_TAG_PATTERN = /<\/?(?:html|head|body)\b[^>]*>/gi;

export function sanitizeNoticeHtml(html) {
  if (!html) {
    return '';
  }

  return String(html)
    .replace(STYLE_TAG_PATTERN, '')
    .replace(STYLESHEET_LINK_PATTERN, '')
    .replace(DOCUMENT_TAG_PATTERN, '')
    .trim();
}
