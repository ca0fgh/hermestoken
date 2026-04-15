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

import React, { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { reducer, initialState } from './reducer';
import { ensureLanguageResources } from '../../i18n/i18n';
import { normalizeLanguage } from '../../i18n/language';

export const UserContext = React.createContext({
  state: initialState,
  dispatch: () => null,
});

export const UserProvider = ({ children }) => {
  const [state, dispatch] = React.useReducer(reducer, initialState);
  const { i18n } = useTranslation();

  // Sync language preference when user data is loaded
  useEffect(() => {
    let cancelled = false;

    const syncLanguagePreference = async () => {
      if (!state.user?.setting) {
        return;
      }

      try {
        const settings = JSON.parse(state.user.setting);
        const normalizedLanguage = normalizeLanguage(settings.language);
        if (normalizedLanguage) {
          await ensureLanguageResources(normalizedLanguage);
        }
        if (
          !cancelled &&
          normalizedLanguage &&
          normalizedLanguage !== i18n.language
        ) {
          i18n.changeLanguage(normalizedLanguage);
        }
        if (normalizedLanguage) {
          localStorage.setItem('i18nextLng', normalizedLanguage);
        }
      } catch (e) {
        // Ignore parse errors
      }
    };

    void syncLanguagePreference();

    return () => {
      cancelled = true;
    };
  }, [state.user?.setting, i18n]);

  return (
    <UserContext.Provider value={[state, dispatch]}>
      {children}
    </UserContext.Provider>
  );
};
