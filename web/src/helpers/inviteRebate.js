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

import {
  normalizeGroupNames,
  normalizeGroupRateMap,
  normalizeRateBps,
  rateBpsToPercentNumber,
} from './subscriptionReferral.js';

function normalizeCount(value, fallback = 0) {
  const normalized = Number(value);
  if (!Number.isFinite(normalized) || normalized < 0) {
    return fallback;
  }
  return Math.trunc(normalized);
}

function normalizeOverrideGroupMap(groups = []) {
  if (!Array.isArray(groups)) {
    return {};
  }

  return groups.reduce((overrideGroups, groupItem) => {
    const group = String(groupItem?.group || '').trim();
    if (!group) {
      return overrideGroups;
    }

    return {
      ...overrideGroups,
      [group]: normalizeRateBps(
        groupItem?.invitee_rate_bps ?? groupItem?.inviteeRateBps,
      ),
    };
  }, {});
}

export function normalizeInviteeContributionPage(payload = {}) {
  return {
    page: normalizeCount(payload?.page, 1) || 1,
    page_size: normalizeCount(payload?.page_size),
    total: normalizeCount(payload?.total),
    invitee_count: normalizeCount(payload?.invitee_count),
    total_contribution_quota: normalizeCount(payload?.total_contribution_quota),
    items: Array.isArray(payload?.items) ? payload.items : [],
  };
}

export function buildInviteDefaultRuleRows(groups = []) {
  if (!Array.isArray(groups)) {
    return [];
  }

  return groups.reduce((rows, groupItem) => {
    const group = String(groupItem?.group || '').trim();
    if (!group) {
      return rows;
    }
    const type = String(groupItem?.type || 'subscription').trim() || 'subscription';

    const inputPercent = rateBpsToPercentNumber(
      normalizeRateBps(
        groupItem?.invitee_rate_bps ?? groupItem?.inviteeRateBps,
      ),
    );

    return [
      ...rows,
      {
        id: `${type}:${group}`,
        type,
        group,
        inputPercent,
        effectiveTotalRateBps: normalizeRateBps(
          groupItem?.total_rate_bps ?? groupItem?.totalRateBps,
        ),
        hasOverride: true,
        isDraft: false,
      },
    ];
  }, []);
}

export function buildInviteeOverrideRows(payload = {}) {
  const availableGroups = normalizeGroupNames(payload?.available_groups);
  const defaultInviteeRateBpsByGroup = normalizeGroupRateMap(
    payload?.default_invitee_rate_bps_by_group,
  );
  const effectiveTotalRateBpsByGroup = normalizeGroupRateMap(
    payload?.effective_total_rate_bps_by_group,
  );
  const overrideInviteeRateBpsByGroup = normalizeOverrideGroupMap(
    payload?.overrides,
  );

  const groups = normalizeGroupNames([
    ...availableGroups,
    ...Object.keys(overrideInviteeRateBpsByGroup),
  ]);

  return groups.map((group) => {
    const type = 'subscription';
    const defaultInviteeRateBps = normalizeRateBps(
      defaultInviteeRateBpsByGroup[group],
    );
    const overrideInviteeRateBps = overrideInviteeRateBpsByGroup[group];
    const hasOverride = overrideInviteeRateBps !== undefined;

    return {
      id: `${type}:${group}`,
      type,
      group,
      inputPercent: rateBpsToPercentNumber(
        hasOverride ? overrideInviteeRateBps : defaultInviteeRateBps,
      ),
      effectiveTotalRateBps: normalizeRateBps(
        effectiveTotalRateBpsByGroup[group],
      ),
      defaultInviteeRateBps,
      hasOverride,
      isDraft: false,
    };
  });
}
