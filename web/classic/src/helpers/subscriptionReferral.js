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
    new Set(groups.map((group) => String(group || '').trim()).filter(Boolean)),
  ).sort((left, right) => left.localeCompare(right));
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
