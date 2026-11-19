// PreferencesProvider (F051 T008, T035, F051-AC-16/17): language, light/dark theme and compact density chosen in the
// user menu or on the login page apply immediately — no reload: i18next language, Ant Design theme + locale and
// dayjs locale (OpsThemeProvider) all follow the state. Stored per viewer (`ew.portal.prefs`, F050 data-model).
import { OpsThemeProvider } from '@eduworld/ops-ui';
import type { i18n as I18n } from 'i18next';
import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from 'react';
import type { Locale } from '../i18n';
import { writePreferences, type UserPreference } from './preferences';

interface PreferencesApi {
  preferences: UserPreference;
  update(patch: Partial<UserPreference>): void;
}

const PreferencesContext = createContext<PreferencesApi | null>(null);

export function usePreferences(): PreferencesApi {
  const p = useContext(PreferencesContext);
  if (!p) throw new Error('PreferencesProvider missing');
  return p;
}

export function PreferencesProvider({ initial, i18n, children }: { initial: UserPreference; i18n: I18n; children: ReactNode }) {
  const [preferences, setPreferences] = useState(initial);
  const update = useCallback(
    (patch: Partial<UserPreference>) => {
      setPreferences((prev) => ({ ...prev, ...patch }));
      if (patch.locale && patch.locale !== i18n.language) {
        void i18n.changeLanguage(patch.locale).then(() => {
          document.documentElement.lang = patch.locale!;
          document.title = i18n.t('app.title');
        });
      }
    },
    [i18n],
  );
  useEffect(() => writePreferences(preferences), [preferences]);
  const api = useMemo(() => ({ preferences, update }), [preferences, update]);
  return (
    <PreferencesContext.Provider value={api}>
      <OpsThemeProvider mode={preferences.theme} density={preferences.density} locale={preferences.locale as Locale}>
        {children}
      </OpsThemeProvider>
    </PreferencesContext.Provider>
  );
}
