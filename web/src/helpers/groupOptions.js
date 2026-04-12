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
