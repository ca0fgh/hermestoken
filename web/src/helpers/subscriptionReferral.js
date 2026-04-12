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

export function normalizeAdminReferralPayload({ enabled, totalRateBps }) {
  return {
    enabled: Boolean(enabled),
    totalRateBps: clampInviteeRateBps(totalRateBps, 10000),
  };
}
