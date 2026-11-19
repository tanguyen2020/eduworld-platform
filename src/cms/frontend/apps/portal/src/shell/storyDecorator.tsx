// Storybook decorator of the portal shell stories (F051 T047): portal + ops-ui translations (synchronous instance,
// locale from the Storybook toolbar) and a memory router. The theme comes from the ops-ui preview (Light/Dark/
// Compact). Used by *.stories.tsx only — never by the application.
import type { Decorator } from '@storybook/react';
import i18next from 'i18next';
import { I18nextProvider, initReactI18next } from 'react-i18next';
import { MemoryRouter } from 'react-router-dom';
import { DEFAULT_LOCALE, PORTAL_NS, resources } from '../i18n';
import { OPS_UI_NS } from '@eduworld/ops-ui';

const i18n = i18next.createInstance();
void i18n.use(initReactI18next).init({
  resources,
  lng: DEFAULT_LOCALE,
  fallbackLng: DEFAULT_LOCALE,
  ns: [PORTAL_NS, OPS_UI_NS],
  defaultNS: PORTAL_NS,
  interpolation: { escapeValue: false },
  initAsync: false,
});

export const withPortal: Decorator = (Story, ctx) => {
  const locale = (ctx.globals.locale as string) ?? DEFAULT_LOCALE;
  if (i18n.language !== locale) void i18n.changeLanguage(locale);
  return (
    <I18nextProvider i18n={i18n}>
      <MemoryRouter>
        <Story />
      </MemoryRouter>
    </I18nextProvider>
  );
};
