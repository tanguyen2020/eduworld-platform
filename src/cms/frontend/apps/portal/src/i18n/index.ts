// i18n of the portal (F050-AC-04, AGENTS §7): `vi-VN` default, `en-US`; namespaces `portal` and `ops-ui`.
import i18next, { type i18n as I18n } from 'i18next';
import { initReactI18next } from 'react-i18next';
import { OPS_UI_NS, opsUiResources } from '@eduworld/ops-ui';
import vi from './vi.json';
import en from './en.json';

export const LOCALES = ['vi-VN', 'en-US'] as const;
export type Locale = (typeof LOCALES)[number];
export const DEFAULT_LOCALE: Locale = 'vi-VN';
export const PORTAL_NS = 'portal';

export const resources = {
  'vi-VN': { [PORTAL_NS]: vi, [OPS_UI_NS]: opsUiResources.vi },
  'en-US': { [PORTAL_NS]: en, [OPS_UI_NS]: opsUiResources.en },
} as const;

export function isLocale(v: unknown): v is Locale {
  return typeof v === 'string' && (LOCALES as readonly string[]).includes(v);
}

/** A fresh i18next instance (one per app / test). */
export async function createI18n(locale: Locale = DEFAULT_LOCALE): Promise<I18n> {
  const instance = i18next.createInstance();
  await instance.use(initReactI18next).init({
    resources,
    lng: locale,
    fallbackLng: DEFAULT_LOCALE,
    ns: [PORTAL_NS, OPS_UI_NS],
    defaultNS: PORTAL_NS,
    interpolation: { escapeValue: false },
    returnNull: false,
  });
  return instance;
}
