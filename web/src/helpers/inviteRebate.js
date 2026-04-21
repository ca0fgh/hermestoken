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
    const effectiveInviteeRateBps = normalizeRateBps(
      groupItem?.default_invitee_rate_bps ??
        groupItem?.effective_invitee_rate_bps ??
        groupItem?.invitee_rate_bps ??
        groupItem?.inviteeRateBps,
    );
    const effectiveTotalRateBps = normalizeRateBps(
      groupItem?.total_rate_bps ?? groupItem?.totalRateBps,
    );

    return [
      ...rows,
      {
        id: `subscription:${group}`,
        group,
        templateName: String(
          groupItem?.template_name ?? groupItem?.templateName ?? '',
        ).trim(),
        levelType: String(
          groupItem?.level_type ?? groupItem?.levelType ?? '',
        ).trim(),
        effectiveTotalRateBps,
        effectiveInviteeRateBps,
        effectiveInviterRateBps: Math.max(
          0,
          effectiveTotalRateBps - effectiveInviteeRateBps,
        ),
      },
    ];
  }, []);
}

export function buildReceivedInviteeRuleRows(payload = {}) {
  const inviterId = normalizeCount(payload?.received_inviter?.id);
  const inviterUsername = String(
    payload?.received_inviter?.username ?? '',
  ).trim();
  const groups = Array.isArray(payload?.received_groups)
    ? payload.received_groups
    : [];

  return groups.reduce((rows, groupItem) => {
    const group = String(groupItem?.group || '').trim();
    if (!group) {
      return rows;
    }

    const effectiveInviteeRateBps = normalizeRateBps(
      groupItem?.effective_invitee_rate_bps ??
        groupItem?.effectiveInviteeRateBps ??
        groupItem?.invitee_rate_bps ??
        groupItem?.inviteeRateBps,
    );
    if (effectiveInviteeRateBps <= 0) {
      return rows;
    }

    const effectiveTotalRateBps = normalizeRateBps(
      groupItem?.total_rate_bps ?? groupItem?.totalRateBps,
    );

    return [
      ...rows,
      {
        id: `received:${group}`,
        inviterId,
        inviterUsername,
        group,
        templateName: String(
          groupItem?.template_name ?? groupItem?.templateName ?? '',
        ).trim(),
        levelType: String(
          groupItem?.level_type ?? groupItem?.levelType ?? '',
        ).trim(),
        effectiveTotalRateBps,
        effectiveInviteeRateBps,
        effectiveInviterRateBps: Math.max(
          0,
          normalizeRateBps(
            groupItem?.effective_inviter_rate_bps ??
              groupItem?.effectiveInviterRateBps ??
              (effectiveTotalRateBps - effectiveInviteeRateBps),
          ),
        ),
        hasOverride: Boolean(
          groupItem?.has_override ?? groupItem?.hasOverride,
        ),
      },
    ];
  }, []);
}

export function buildInviteeOverrideRows(payload = {}) {
  const scopes = Array.isArray(payload?.scopes) ? payload.scopes : [];

  return scopes.reduce((rows, scopeItem) => {
    const group = String(scopeItem?.group || '').trim();
    if (!group) {
      return rows;
    }

    const effectiveTotalRateBps = normalizeRateBps(
      scopeItem?.total_rate_bps ?? scopeItem?.totalRateBps,
    );
    const defaultInviteeRateBps = normalizeRateBps(
      scopeItem?.default_invitee_rate_bps ?? scopeItem?.defaultInviteeRateBps,
    );
    const overrideInviteeRateBps = normalizeRateBps(
      scopeItem?.override_invitee_rate_bps ?? scopeItem?.overrideInviteeRateBps,
    );
    const hasOverride = Boolean(
      scopeItem?.has_override ?? scopeItem?.hasOverride,
    );
    const effectiveInviteeRateBps = normalizeRateBps(
      scopeItem?.effective_invitee_rate_bps ??
        scopeItem?.effectiveInviteeRateBps ??
        (hasOverride ? overrideInviteeRateBps : defaultInviteeRateBps),
    );

    const nextRow = {
      id: `subscription:${group}`,
      group,
      templateName: String(
        scopeItem?.template_name ?? scopeItem?.templateName ?? '',
      ).trim(),
      levelType: String(
        scopeItem?.level_type ?? scopeItem?.levelType ?? '',
      ).trim(),
      inputPercent: rateBpsToPercentNumber(effectiveInviteeRateBps),
      effectiveTotalRateBps,
      defaultInviteeRateBps,
      effectiveInviteeRateBps,
      hasOverride,
    };

    return [...rows, nextRow];
  }, []);
}

export function buildInviteeOverrideDraftPercentMap(rows = []) {
  if (!Array.isArray(rows)) {
    return {};
  }

  return rows.reduce((drafts, row) => {
    const group = String(row?.group || '').trim();
    if (!group) {
      return drafts;
    }

    return {
      ...drafts,
      [group]: Number(row?.inputPercent || 0),
    };
  }, {});
}

export function buildInviteeContributionSummary(cards = []) {
  if (!Array.isArray(cards)) {
    return {
      orderCount: 0,
      ownRewardQuota: 0,
      inviteeRewardQuota: 0,
      directDetailCount: 0,
      teamDetailCount: 0,
      inviteeShareCount: 0,
    };
  }

  return cards.reduce(
    (summary, card) => {
      const nextSummary = {
        ...summary,
        orderCount: summary.orderCount + 1,
        ownRewardQuota: summary.ownRewardQuota + normalizeCount(card?.ownRewardQuota),
        inviteeRewardQuota:
          summary.inviteeRewardQuota + normalizeCount(card?.inviteeRewardQuota),
      };

      const items = Array.isArray(card?.items) ? card.items : [];
      return items.reduce((itemSummary, item) => {
        if (item?.isInviteeShare) {
          return {
            ...itemSummary,
            inviteeShareCount: itemSummary.inviteeShareCount + 1,
          };
        }
        if (item?.roleType === 'team') {
          return {
            ...itemSummary,
            teamDetailCount: itemSummary.teamDetailCount + 1,
          };
        }
        if (item?.roleType === 'direct') {
          return {
            ...itemSummary,
            directDetailCount: itemSummary.directDetailCount + 1,
          };
        }
        return itemSummary;
      }, nextSummary);
    },
    {
      orderCount: 0,
      ownRewardQuota: 0,
      inviteeRewardQuota: 0,
      directDetailCount: 0,
      teamDetailCount: 0,
      inviteeShareCount: 0,
    },
  );
}

export function buildInviteeContributionLedgerRows(cards = []) {
  if (!Array.isArray(cards)) {
    return [];
  }

  return cards.reduce((rows, card) => {
    const items = Array.isArray(card?.items) ? card.items : [];
    const cardRows = items.map((item) => ({
      id: `${String(card?.id || '').trim()}:${String(item?.id || '').trim()}`,
      tradeNo: String(card?.tradeNo || '').trim(),
      group: String(card?.group || '').trim(),
      settledAt: normalizeCount(card?.settledAt),
      status: String(item?.status ?? card?.status ?? '').trim(),
      roleType: String(item?.roleType || '').trim(),
      roleLabel: String(item?.roleLabel || '').trim(),
      componentLabel: String(item?.componentLabel || '').trim(),
      effectiveRewardQuota: normalizeCount(item?.effectiveRewardQuota),
      isInviteeShare:
        Boolean(item?.isInviteeShare) ||
        String(item?.componentLabel || '').trim() === '返给对方',
    }));

    return [...rows, ...cardRows];
  }, []);
}

function normalizeContributionDetailRoleType(rewardComponent, roleType, sourceRewardComponent) {
  const normalizedRewardComponent = String(rewardComponent || '').trim();
  const normalizedRoleType = String(roleType || '').trim();
  const normalizedSourceRewardComponent = String(sourceRewardComponent || '').trim();

  if (normalizedRoleType) {
    return normalizedRoleType;
  }
  if (
    normalizedRewardComponent === 'team_direct_reward' ||
    normalizedRewardComponent === 'team_reward'
  ) {
    return 'team';
  }
  if (normalizedRewardComponent === 'direct_reward') {
    return 'direct';
  }
  if (
    normalizedRewardComponent === 'invitee_reward' &&
    normalizedSourceRewardComponent === 'team_direct_reward'
  ) {
    return 'team';
  }
  if (
    normalizedRewardComponent === 'invitee_reward' &&
    normalizedSourceRewardComponent === 'direct_reward'
  ) {
    return 'direct';
  }
  return '';
}

function resolveContributionComponentLabel(rewardComponent) {
  const normalizedRewardComponent = String(rewardComponent || '').trim();
  if (normalizedRewardComponent === 'team_direct_reward') {
    return '团队直返';
  }
  if (normalizedRewardComponent === 'team_reward') {
    return '团队级差';
  }
  if (normalizedRewardComponent === 'invitee_reward') {
    return '返给对方';
  }
  return '直推返佣';
}

function resolveContributionRoleLabel(roleType) {
  if (roleType === 'team') {
    return '团队';
  }
  if (roleType === 'direct') {
    return '直推';
  }
  return '-';
}

export function buildInviteeContributionDetailCards(payload = {}) {
  const details = Array.isArray(payload?.contribution_details)
    ? payload.contribution_details
    : [];

  return details.reduce((cards, detailItem) => {
    const batchId = normalizeCount(detailItem?.batch_id);
    const tradeNo = String(detailItem?.trade_no || '').trim();
    const group = String(detailItem?.group || '').trim();
    const rewardComponent = String(detailItem?.reward_component || '').trim();
    const sourceRewardComponent = String(
      detailItem?.source_reward_component || '',
    ).trim();
    const roleType = normalizeContributionDetailRoleType(
      rewardComponent,
      detailItem?.role_type ?? detailItem?.roleType,
      sourceRewardComponent,
    );
    const effectiveRewardQuota = normalizeCount(
      detailItem?.effective_reward_quota ?? detailItem?.effectiveRewardQuota,
    );
    const status = String(detailItem?.status || '').trim();
    const settledAt = normalizeCount(
      detailItem?.settled_at ?? detailItem?.settledAt,
    );
    const isInviteeShare = rewardComponent === 'invitee_reward';
    const cardId = `${batchId}:${tradeNo}`;
    const nextItem = {
      id: `${cardId}:${cards
        .find((card) => card.id === cardId)
        ?.items.length || 0}`,
      rewardComponent,
      sourceRewardComponent,
      roleType,
      effectiveRewardQuota,
      status,
      componentLabel: resolveContributionComponentLabel(rewardComponent),
      roleLabel: resolveContributionRoleLabel(roleType),
      isInviteeShare,
    };

    const existingCard = cards.find((card) => card.id === cardId);
    if (!existingCard) {
      return [
        ...cards,
        {
          id: cardId,
          batchId,
          tradeNo,
          group,
          status,
          settledAt,
          ownRewardQuota: isInviteeShare ? 0 : effectiveRewardQuota,
          inviteeRewardQuota: isInviteeShare ? effectiveRewardQuota : 0,
          items: [nextItem],
        },
      ];
    }

    return cards.map((card) => {
      if (card.id !== cardId) {
        return card;
      }
      return {
        ...card,
        ownRewardQuota: card.ownRewardQuota + (isInviteeShare ? 0 : effectiveRewardQuota),
        inviteeRewardQuota:
          card.inviteeRewardQuota + (isInviteeShare ? effectiveRewardQuota : 0),
        items: [...card.items, nextItem],
      };
    });
  }, []);
}
