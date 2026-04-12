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

export function buildReferralRateSummary(totalRateBps, inviteeRateBps) {
  const total = Math.max(0, Number(totalRateBps || 0));
  const invitee = clampInviteeRateBps(inviteeRateBps, total);
  return {
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
  const legacyTotalRateBps = normalizeRateBps(payload.total_rate_bps);
  const hasLegacyTotalRate =
    payload.total_rate_bps !== undefined && payload.total_rate_bps !== null;

  return {
    enabled,
    groupRates:
      Object.keys(groupRates).length > 0
        ? groupRates
        : hasLegacyTotalRate
          ? { default: legacyTotalRateBps }
          : {},
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
  const normalizedGroupNames = groupNames
    .map((group) => String(group || '').trim())
    .filter(Boolean);
  const orderedGroups = [
    ...normalizedGroupNames,
    ...Object.keys(normalizedGroupRates).filter(
      (group) => !normalizedGroupNames.includes(group),
    ),
  ].sort((left, right) => left.localeCompare(right));

  return orderedGroups.map((group) => {
    const totalRateBps = normalizeRateBps(normalizedGroupRates[group] || 0);
    return {
      group,
      enabled: totalRateBps > 0,
      totalRateBps,
      totalRatePercent: rateBpsToPercentNumber(totalRateBps),
    };
  });
}

export function buildGroupedReferralSummaries(groups = []) {
  return groups.map((groupItem) => ({
    group: groupItem.group || '',
    ...buildReferralRateSummary(
      groupItem.total_rate_bps ?? groupItem.totalRateBps,
      groupItem.invitee_rate_bps ?? groupItem.inviteeRateBps,
    ),
  }));
}
