// Per-viewer preferences (F050 data-model `UserPreference`): theme, density, locale, last workspace in
// localStorage `ew.portal.prefs`. Never tokens or business data; every access is guarded (storage may throw).
import { DEFAULT_LOCALE, isLocale, type Locale } from '../i18n';

export interface UserPreference {
  theme: 'light' | 'dark';
  density: 'default' | 'compact';
  locale: Locale;
  lastWorkspace?: string;
}

export const PREFS_KEY = 'ew.portal.prefs';
export const DEFAULT_PREFS: UserPreference = { theme: 'light', density: 'compact', locale: DEFAULT_LOCALE };

export function readPreferences(): UserPreference {
  try {
    const raw = window.localStorage.getItem(PREFS_KEY);
    if (!raw) return { ...DEFAULT_PREFS };
    const p = JSON.parse(raw) as Partial<UserPreference>;
    return {
      theme: p.theme === 'dark' ? 'dark' : 'light',
      density: p.density === 'default' ? 'default' : 'compact',
      locale: isLocale(p.locale) ? p.locale : DEFAULT_LOCALE,
      ...(typeof p.lastWorkspace === 'string' ? { lastWorkspace: p.lastWorkspace } : {}),
    };
  } catch {
    return { ...DEFAULT_PREFS };
  }
}

export function writePreferences(p: UserPreference): void {
  try {
    window.localStorage.setItem(PREFS_KEY, JSON.stringify(p));
  } catch {
    // storage unavailable (private mode, blocked): preferences are a convenience only
  }
}
