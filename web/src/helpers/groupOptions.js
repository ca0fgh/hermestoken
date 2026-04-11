function normalizeGroupDescription(desc) {
  return typeof desc === 'string' ? desc.trim() : '';
}

function truncateDescription(desc, maxLength) {
  if (!maxLength || desc.length <= maxLength) {
    return desc;
  }
  return `${desc.substring(0, maxLength)}...`;
}

export function buildGroupOption(
  group,
  info = {},
  { truncateDescAt } = {},
) {
  const description = normalizeGroupDescription(info?.desc);

  return {
    label: group,
    value: group,
    ratio: info?.ratio ?? 1,
    desc: description,
    fullLabel: description,
    optionDescription: truncateDescription(description, truncateDescAt),
  };
}

export function buildGroupOptions(
  data,
  { userGroup = '', truncateDescAt } = {},
) {
  let groupOptions = Object.entries(data || {}).map(([group, info]) =>
    buildGroupOption(group, info, { truncateDescAt }),
  );

  if (userGroup) {
    const userGroupIndex = groupOptions.findIndex((g) => g.value === userGroup);
    if (userGroupIndex > -1) {
      const userGroupOption = groupOptions.splice(userGroupIndex, 1)[0];
      groupOptions.unshift(userGroupOption);
    }
  }

  return groupOptions;
}
