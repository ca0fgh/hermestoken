function trimString(value) {
  return `${value ?? ''}`.trim();
}

export function buildGroupUpsertPayload(existingGroup, draftGroup) {
  const immutableKey = trimString(existingGroup?.group_key);
  const nextKey = immutableKey || trimString(draftGroup?.group_key);
  return {
    group_key: nextKey,
    display_name:
      trimString(draftGroup?.display_name) ||
      trimString(existingGroup?.display_name) ||
      nextKey,
    billing_ratio: Number(draftGroup?.billing_ratio ?? existingGroup?.billing_ratio ?? 1),
    user_selectable: Boolean(
      draftGroup?.user_selectable ?? existingGroup?.user_selectable ?? false,
    ),
    description: trimString(draftGroup?.description ?? existingGroup?.description),
    sort_order: Number(draftGroup?.sort_order ?? existingGroup?.sort_order ?? 0),
    status: Number(draftGroup?.status ?? existingGroup?.status ?? 1),
  };
}

export function buildMergePayload(values) {
  return {
    source_group_key: trimString(values?.source_group_key),
    target_group_key: trimString(values?.target_group_key),
  };
}

export function canMergeGroups(values) {
  const payload = buildMergePayload(values);
  return Boolean(
    payload.source_group_key &&
      payload.target_group_key &&
      payload.source_group_key !== payload.target_group_key,
  );
}

export function buildArchivePayload(groupKey) {
  return {
    group_key: trimString(groupKey),
  };
}

export function buildLegacyGroupOptionPayload(groups = []) {
  const groupRatio = {};
  const userUsableGroups = {};

  (groups || []).forEach((group) => {
    const groupKey = trimString(group?.group_key);
    const displayName = trimString(group?.display_name) || groupKey;
    const billingRatio = Number(group?.billing_ratio ?? 1);
    const status = Number(group?.status ?? 1);

    if (!groupKey || status === 3) {
      return;
    }

    groupRatio[groupKey] = billingRatio;
    if (group?.user_selectable) {
      userUsableGroups[groupKey] = displayName;
    }
  });

  return {
    GroupRatio: JSON.stringify(groupRatio, null, 2),
    UserUsableGroups: JSON.stringify(userUsableGroups, null, 2),
  };
}

async function getAPI() {
  const mod = await import('./api.js');
  return mod.API;
}

export async function listPricingGroupsAdmin() {
  const API = await getAPI();
  return API.get('/api/group/admin');
}

export async function createPricingGroup(payload) {
  const API = await getAPI();
  return API.post('/api/group/admin', payload);
}

export async function updatePricingGroup(groupKey, payload) {
  const API = await getAPI();
  return API.put(`/api/group/admin/${encodeURIComponent(groupKey)}`, payload);
}

export async function archivePricingGroup(groupKey) {
  const API = await getAPI();
  return API.post('/api/group/admin/archive', buildArchivePayload(groupKey));
}

export async function mergePricingGroups(values) {
  const API = await getAPI();
  return API.post('/api/group/admin/merge', buildMergePayload(values));
}
