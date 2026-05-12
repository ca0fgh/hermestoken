export function currentUnixSeconds() {
  return Math.floor(Date.now() / 1000);
}

export function isUserSubscriptionTimeExpired(sub, now = currentUnixSeconds()) {
  return (sub?.end_time || 0) > 0 && (sub?.end_time || 0) <= now;
}

export function isUserSubscriptionQuotaExhausted(sub) {
  const total = Number(sub?.amount_total || 0);
  if (total <= 0) return false;
  return Number(sub?.amount_used || 0) >= total;
}

export function getUserSubscriptionDisplayStatus(
  sub,
  now = currentUnixSeconds(),
) {
  if (sub?.status === 'cancelled') {
    return 'cancelled';
  }

  const timeExpired = isUserSubscriptionTimeExpired(sub, now);
  const quotaExhausted = isUserSubscriptionQuotaExhausted(sub);

  if (!timeExpired && quotaExhausted) {
    return 'exhausted';
  }
  if (timeExpired || sub?.status === 'expired') {
    return 'expired';
  }
  return 'active';
}

export function isUserSubscriptionDisplayActive(
  sub,
  now = currentUnixSeconds(),
) {
  return getUserSubscriptionDisplayStatus(sub, now) === 'active';
}
