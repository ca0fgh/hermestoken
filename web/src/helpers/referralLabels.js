export function formatReferralTypeLabel(referralType, t) {
  switch (String(referralType || '').trim()) {
    case 'subscription_referral':
      return t('订阅返佣');
    default:
      return String(referralType || '').trim();
  }
}

export function buildReferralTypeOptions(t) {
  return [{ label: formatReferralTypeLabel('subscription_referral', t), value: 'subscription_referral' }];
}

export function formatReferralLevelTypeLabel(levelType, t) {
  switch (String(levelType || '').trim()) {
    case 'direct':
      return t('直推模板（direct）');
    case 'team':
      return t('团队模板（team）');
    default:
      return String(levelType || '').trim();
  }
}

export function buildReferralLevelTypeOptions(t) {
  return [
    { label: formatReferralLevelTypeLabel('direct', t), value: 'direct' },
    { label: formatReferralLevelTypeLabel('team', t), value: 'team' },
  ];
}

export function formatReferralGroupLabel(group, t) {
  const trimmedGroup = String(group || '').trim();
  if (trimmedGroup === '') {
    return t('所有分组');
  }
  return trimmedGroup;
}

const normalizeReferralTemplateGroups = (template) => {
  const sourceGroups = Array.isArray(template?.groups) ? template.groups : [template?.group];
  return [...new Set(sourceGroups.map((group) => String(group || '').trim()).filter(Boolean))].sort();
};

export function formatReferralTemplateOptionLabel(template, t, options = {}) {
  const name = String(template?.name || '').trim();
  const includeGroupSuffixWhenNamed = options?.includeGroupSuffixWhenNamed === true;
  const groups = normalizeReferralTemplateGroups(template);
  const scopeLabel =
    groups.length > 0
      ? groups.map((group) => formatReferralGroupLabel(group, t)).join(', ')
      : formatReferralGroupLabel(template?.group, t);
  if (name !== '') {
    if (!includeGroupSuffixWhenNamed) {
      return name;
    }

    return [name, scopeLabel].filter(Boolean).join(' · ');
  }

  return [
    scopeLabel,
    formatReferralLevelTypeLabel(template?.level_type, t),
  ]
    .filter(Boolean)
    .join(' · ');
}
