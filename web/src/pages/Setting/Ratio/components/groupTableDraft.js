function normalizeTextValue(value) {
  return typeof value === 'string' ? value : '';
}

export function getSyncedDraftValue({
  committedValue,
  draftValue,
  isComposing,
}) {
  if (isComposing) {
    return normalizeTextValue(draftValue);
  }

  return normalizeTextValue(committedValue);
}

export function shouldCommitDraftValue({ committedValue, draftValue }) {
  return normalizeTextValue(committedValue) !== normalizeTextValue(draftValue);
}
