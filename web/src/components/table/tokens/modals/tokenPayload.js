export function buildTokenPayload(values) {
  const selectionMode = values?.selection_mode || 'inherit_user_default';
  const requestedGroup = `${values?.group_key || values?.group || ''}`.trim();

  if (selectionMode === 'fixed') {
    return {
      ...values,
      selection_mode: 'fixed',
      group_key: requestedGroup,
      group: requestedGroup,
    };
  }

  return {
    ...values,
    selection_mode: selectionMode,
    group_key: '',
    group: '',
  };
}
