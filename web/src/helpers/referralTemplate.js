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
  return [...new Set(sourceGroups.map((group) => String(group || '').trim()).filter(Boolean))];
};

export function normalizeReferralTemplateItems(items = []) {
  if (!Array.isArray(items)) {
    return [];
  }

  return items.map((item) => ({
    id: Number(item?.id || item?.template_ids?.[0] || 0),
    bundleKey: buildFallbackBundleKey(item),
    templateIds: Array.isArray(item?.template_ids)
      ? item.template_ids.map((templateId) => Number(templateId || 0)).filter((templateId) => templateId > 0)
      : Number(item?.id || 0) > 0
        ? [Number(item.id)]
        : [],
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
