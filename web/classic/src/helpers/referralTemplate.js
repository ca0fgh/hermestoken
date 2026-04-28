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
const buildFallbackBundleKey = (item) => {
  const bundleKey = String(item?.bundle_key || '').trim();
  if (bundleKey) {
    return bundleKey;
  }

  const templateIds = Array.isArray(item?.template_ids)
    ? item.template_ids
    : item?.id
      ? [item.id]
      : [];
  const firstTemplateId = Number(templateIds[0] || 0);
  if (firstTemplateId > 0) {
    return `template:${firstTemplateId}`;
  }
  return '';
};

const normalizeGroups = (item) => {
  const sourceGroups = Array.isArray(item?.groups) ? item.groups : [item?.group];
  return [...new Set(sourceGroups.map((group) => String(group || '').trim()).filter(Boolean))].sort();
};

const normalizeTemplateIds = (item) => {
  const sourceTemplateIds = Array.isArray(item?.template_ids)
    ? item.template_ids
    : Number(item?.id || 0) > 0
      ? [item.id]
      : [];

  return [...new Set(sourceTemplateIds.map((templateId) => Number(templateId || 0)).filter((templateId) => templateId > 0))].sort(
    (left, right) => left - right,
  );
};

export function normalizeReferralTemplateItems(items = []) {
  if (!Array.isArray(items)) {
    return [];
  }

  return items.map((item) => ({
    id: Number(item?.id || item?.template_ids?.[0] || 0),
    bundleKey: buildFallbackBundleKey(item),
    templateIds: normalizeTemplateIds(item),
    referralType: String(item?.referral_type || '').trim(),
    group: String(item?.group || '').trim(),
    groups: normalizeGroups(item),
    name: String(item?.name || '').trim(),
    levelType: String(item?.level_type || '').trim(),
    enabled: Boolean(item?.enabled),
    directCapBps: Number(item?.direct_cap_bps || 0),
    teamCapBps: Number(item?.team_cap_bps || 0),
    inviteeShareDefaultBps: Number(item?.invitee_share_default_bps || 0),
  }));
}
