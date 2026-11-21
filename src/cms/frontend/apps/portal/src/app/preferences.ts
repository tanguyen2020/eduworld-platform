// Per-viewer preferences (F050 data-model `UserPreference`): theme, density, locale, last workspace in
// localStorage `ew.portal.prefs`. Never tokens or business data; every access is guarded (storage may throw).
// Density: comfortable 15 px by default (DESIGN_SYSTEM §4.1, owner 2026-11-21); compact only when the viewer turned it on
// in the user menu (`densityChosen`) — a compact value written by the former default is not a choice and is ignored.
import { DEFAULT_LOCALE, isLocale, type Locale } from '../i18n';

export interface UserPreference {
  theme: 'light' | 'dark';
  density: 'default' | 'compact';
  /** The viewer picked the density in the user menu (otherwise the default applies). */
  densityChosen?: boolean;
  locale: Locale;
  lastWorkspace?: string;
}

export const PREFS_KEY = 'ew.portal.prefs';
export const DEFAULT_PREFS: UserPreference = { theme: 'light', density: 'default', locale: DEFAULT_LOCALE };

export function readPreferences(): UserPreference {
  try {
    const raw = window.localStorage.getItem(PREFS_KEY);
    if (!raw) return { ...DEFAULT_PREFS };
    const p = JSON.parse(raw) as Partial<UserPreference>;
    return {
      theme: p.theme === 'dark' ? 'dark' : 'light',
      density: p.densityChosen === true && p.density === 'compact' ? 'compact' : 'default',
      ...(p.densityChosen === true ? { densityChosen: true } : {}),
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
