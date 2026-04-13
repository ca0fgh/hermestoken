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

export function clampInviteeRateBps(inviteeRateBps, totalRateBps) {
  const invitee = Math.max(0, Number(inviteeRateBps || 0));
  const total = Math.max(0, Number(totalRateBps || 0));
  return Math.min(invitee, total);
}

export function buildReferralRateSummary(
  totalRateBps,
  inviteeRateBps,
  group = '',
) {
  const total = Math.max(0, Number(totalRateBps || 0));
  const invitee = clampInviteeRateBps(inviteeRateBps, total);
  return {
    group: String(group || '').trim(),
    totalRateBps: total,
    inviteeRateBps: invitee,
    inviterRateBps: Math.max(0, total - invitee),
  };
}

export function formatRateBpsPercent(rateBps) {
  const rate = Number(rateBps || 0) / 100;
  return Number.isInteger(rate)
    ? `${rate}%`
    : `${rate.toFixed(2).replace(/\.?0+$/, '')}%`;
}

export function rateBpsToPercentNumber(rateBps) {
  return Number(rateBps || 0) / 100;
}

export function percentNumberToRateBps(percentValue) {
  const normalized = Number(percentValue || 0);
  if (!Number.isFinite(normalized) || normalized <= 0) {
    return 0;
  }
  return Math.round(normalized * 100);
}

export function normalizeRateBps(rateBps) {
  return clampInviteeRateBps(rateBps, 10000);
}

export function normalizeGroupRateMap(payload = {}) {
  if (!payload || typeof payload !== 'object') {
    return {};
  }

  return Object.entries(payload).reduce((groupRates, [groupName, rateBps]) => {
    const normalizedGroup = String(groupName || '').trim();
    if (!normalizedGroup) {
      return groupRates;
    }

    return {
      ...groupRates,
      [normalizedGroup]: normalizeRateBps(rateBps),
    };
  }, {});
}

export function normalizeGroupNames(groups = []) {
  if (!Array.isArray(groups)) {
    return [];
  }

  return Array.from(
    new Set(
      groups
        .map((group) => String(group || '').trim())
        .filter(Boolean),
    ),
  ).sort((left, right) => left.localeCompare(right));
}

export function mergeAdminReferralGroupNames(...groupLists) {
  return normalizeGroupNames(groupLists.flat());
}

export function normalizeAdminReferralPayload({ enabled, totalRateBps }) {
  return {
    enabled: Boolean(enabled),
    totalRateBps: normalizeRateBps(totalRateBps),
  };
}

export function parseAdminReferralSettings(payload = {}) {
  const rawEnabled = payload.enabled;
  let enabled = false;
  if (typeof rawEnabled === 'boolean') {
    enabled = rawEnabled;
  } else if (typeof rawEnabled === 'number') {
    enabled = rawEnabled === 1;
  } else if (typeof rawEnabled === 'string') {
    const normalizedEnabled = rawEnabled.trim().toLowerCase();
    enabled = normalizedEnabled === 'true' || normalizedEnabled === '1';
  }

  const groupRates = normalizeGroupRateMap(payload.group_rates);
  const groups = normalizeGroupNames(payload.groups);

  return {
    enabled,
    groups,
    groupRates,
  };
}

export function buildAdminReferralFormValues({
  enabled = false,
  totalRatePercent = 0,
} = {}) {
  return {
    SubscriptionReferralEnabled: Boolean(enabled),
    SubscriptionReferralGlobalRateBps: Number(totalRatePercent || 0),
  };
}

export function buildAdminReferralRows(groupNames = [], groupRates = {}) {
  const normalizedGroupRates = normalizeGroupRateMap(groupRates);
  const normalizedGroupNames = normalizeGroupNames(groupNames);

  return normalizedGroupNames.map((group) => {
    const totalRateBps = normalizeRateBps(normalizedGroupRates[group] || 0);
    return {
      group,
      enabled: totalRateBps > 0,
      totalRateBps,
      totalRatePercent: rateBpsToPercentNumber(totalRateBps),
    };
  });
}

export function buildAdminOverrideRows(groups = []) {
  return groups.map((groupItem) => {
    const group = String(groupItem?.group || '').trim();
    const effectiveTotalRateBps = normalizeRateBps(
      groupItem?.effective_total_rate_bps ?? groupItem?.effectiveTotalRateBps,
    );
    const hasOverride = Boolean(
      groupItem?.has_override ?? groupItem?.hasOverride,
    );
    const overrideRateBpsRaw =
      groupItem?.override_rate_bps ?? groupItem?.overrideRateBps;
    const normalizedOverrideRateBps =
      overrideRateBpsRaw === null || overrideRateBpsRaw === undefined
        ? null
        : normalizeRateBps(overrideRateBpsRaw);
    const inputRateBps = normalizedOverrideRateBps ?? effectiveTotalRateBps;

    const overrideRatePercent = rateBpsToPercentNumber(inputRateBps);

    return {
      id: `subscription:${group}`,
      type: 'subscription',
      group,
      effectiveTotalRateBps,
      hasOverride,
      overrideRateBps: normalizedOverrideRateBps,
      overrideRatePercent,
      inputPercent: overrideRatePercent,
      isDraft: false,
    };
  });
}

let adminOverrideDraftCounter = 0;

export function createAdminOverrideDraftRow() {
  adminOverrideDraftCounter += 1;
  return {
    id: `draft:${adminOverrideDraftCounter}`,
    type: 'subscription',
    group: '',
    effectiveTotalRateBps: 0,
    hasOverride: false,
    overrideRateBps: null,
    overrideRatePercent: 0,
    isDraft: true,
  };
}

export function buildAdminOverrideGroupOptions(
  groupNames = [],
  overrideRows = [],
  currentRow = {},
) {
  const currentId = String(currentRow.id || '').trim();
  const currentType = String(currentRow.type || '').trim();
  const currentGroup = String(currentRow.group || '').trim();
  const takenGroups = new Set(
    overrideRows
      .filter((row) => {
        const rowId = String(row.id || '').trim();
        const rowType = String(row.type || '').trim();
        return rowType === currentType && rowId !== '' && rowId !== currentId;
      })
      .map((row) => String(row.group || '').trim())
      .filter(Boolean),
  );
  let catalogGroups = normalizeGroupNames(groupNames);
  if (currentGroup && !catalogGroups.includes(currentGroup)) {
    catalogGroups = normalizeGroupNames([...catalogGroups, currentGroup]);
  }

  return catalogGroups.map((group) => ({
    label: group,
    value: group,
    disabled: takenGroups.has(group),
  }));
}

export function buildGroupedReferralSummaries(groups = []) {
  return groups.reduce((summaries, groupItem) => {
    const group = String(groupItem?.group || '').trim();
    if (!group) {
      return summaries;
    }

    return [
      ...summaries,
      {
        ...buildReferralRateSummary(
          groupItem.total_rate_bps ?? groupItem.totalRateBps,
          groupItem.invitee_rate_bps ?? groupItem.inviteeRateBps,
          group,
        ),
      },
    ];
  }, []);
}

export function buildInvitationDraftPercentInputs(
  currentDrafts = {},
  referralGroups = [],
) {
  return (referralGroups || []).reduce((drafts, groupSummary) => {
    const group = String(groupSummary?.group || '').trim();
    if (!group) {
      return drafts;
    }

    const persistedPercent = rateBpsToPercentNumber(
      groupSummary?.inviteeRateBps || 0,
    );
    const hasExistingDraft = Object.prototype.hasOwnProperty.call(
      currentDrafts,
      group,
    );

    return {
      ...drafts,
      [group]: !hasExistingDraft ? persistedPercent : currentDrafts[group],
    };
  }, {});
}
