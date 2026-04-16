export function normalizeReferralTemplateItems(items = []) {
  if (!Array.isArray(items)) {
    return [];
  }

  return items.map((item) => ({
    id: Number(item?.id || 0),
    referralType: String(item?.referral_type || '').trim(),
    group: String(item?.group || '').trim(),
    name: String(item?.name || '').trim(),
    levelType: String(item?.level_type || '').trim(),
    enabled: Boolean(item?.enabled),
    directCapBps: Number(item?.direct_cap_bps || 0),
    teamCapBps: Number(item?.team_cap_bps || 0),
    teamDecayRatio: Number(item?.team_decay_ratio || 0),
    teamMaxDepth: Number(item?.team_max_depth || 0),
    inviteeShareDefaultBps: Number(item?.invitee_share_default_bps || 0),
  }));
}

export function normalizeReferralEngineRouteItems(items = []) {
  if (!Array.isArray(items)) {
    return [];
  }

  return items.map((item) => ({
    id: Number(item?.id || 0),
    referralType: String(item?.referral_type || '').trim(),
    group: String(item?.group || '').trim(),
    engineMode: String(item?.engine_mode || '').trim(),
  }));
}
