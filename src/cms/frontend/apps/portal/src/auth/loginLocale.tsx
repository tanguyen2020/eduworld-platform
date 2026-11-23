// Login locale (owner 2026-11-23, ADMIN_PORTAL §1.3b, F051 Clarifications): the /login page is ALWAYS English (en-US),
// whatever the browser language or the saved preference (`ew.portal.prefs`) is. Strings still come from the i18n catalog
// (AGENTS R2, i18next/no-literal-string): the page gets its own i18next clone pinned to en-US (same resources, its own
// language) plus the Ant Design en_US locale, so nothing changes for the rest of the app — after sign-in the saved
// preference applies as before. The clone never calls `changeLanguage` on the app instance and never writes preferences.
import { ConfigProvider } from 'antd';
import enUS from 'antd/locale/en_US';
import { useEffect, useMemo, type ReactNode } from 'react';
import { I18nextProvider, useTranslation } from 'react-i18next';
import type { Locale } from '../i18n';

/** Language of the login page and of its `portalLogin` request (`locale`, `Accept-Language`). */
export const LOGIN_LOCALE: Locale = 'en-US';

export function LoginLocale({ children }: { children: ReactNode }) {
  const { i18n } = useTranslation();
  const pinned = useMemo(() => i18n.cloneInstance({ lng: LOGIN_LOCALE }), [i18n]);
  useEffect(() => {
    document.documentElement.lang = LOGIN_LOCALE;
    // leaving /login (signed in, forgot password…): the document language follows the app language again
    return () => {
      document.documentElement.lang = i18n.language;
    };
  }, [i18n]);
  return (
    <I18nextProvider i18n={pinned}>
      <ConfigProvider locale={enUS}>{children}</ConfigProvider>
    </I18nextProvider>
  );
}
