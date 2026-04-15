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

export function createInviteeDetailRequestGuard() {
  let latestRequestSequence = 0;
  let latestInviteeId = null;

  return {
    begin(inviteeId) {
      latestRequestSequence += 1;
      latestInviteeId = inviteeId ?? null;

      return {
        sequence: latestRequestSequence,
        inviteeId: latestInviteeId,
      };
    },
    clear() {
      latestRequestSequence += 1;
      latestInviteeId = null;
    },
    isCurrent(request) {
      return (
        Boolean(request) &&
        request.sequence === latestRequestSequence &&
        request.inviteeId === latestInviteeId
      );
    },
  };
}

export function resolveInviteeSelectionAfterPageRefresh({
  currentInvitee,
  nextItems = [],
  requestGuard,
  onSelectionCleared = () => {},
} = {}) {
  if (!currentInvitee) {
    return currentInvitee;
  }

  const nextInvitee =
    (Array.isArray(nextItems) ? nextItems : []).find(
      (item) => item?.id === currentInvitee.id,
    ) || null;

  if (!nextInvitee) {
    requestGuard?.clear?.();
    onSelectionCleared();
  }

  return nextInvitee;
}
