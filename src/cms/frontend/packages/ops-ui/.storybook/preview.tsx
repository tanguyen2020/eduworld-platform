// Storybook preview (F050 T036): every story renders inside OpsThemeProvider + i18n (`ops-ui` namespace).
// A story fixes its mode with `parameters.opsTheme` (Light/Dark/Compact stories); the toolbar sets the rest.
import type { Decorator, Preview } from '@storybook/react';
import { theme } from 'antd';
import type { ReactNode } from 'react';
import i18next from 'i18next';
import { I18nextProvider, initReactI18next } from 'react-i18next';
import { OPS_UI_NS, opsUiResources } from '../src/i18n';
import { OpsThemeProvider, type Density, type OpsLocale, type ThemeMode } from '../src/theme/OpsThemeProvider';

const i18n = i18next.createInstance();
void i18n.use(initReactI18next).init({
  lng: 'vi-VN',
  fallbackLng: 'vi-VN',
  ns: [OPS_UI_NS],
  defaultNS: OPS_UI_NS,
  resources: { 'vi-VN': { [OPS_UI_NS]: opsUiResources.vi }, 'en-US': { [OPS_UI_NS]: opsUiResources.en } },
  interpolation: { escapeValue: false },
  initAsync: false,
});

interface OpsThemeParams {
  mode?: ThemeMode;
  density?: Density;
}

/** Story frame: padding from the theme token, full height so the mode background covers the canvas. */
function StoryFrame({ children }: { children: ReactNode }) {
  const { token } = theme.useToken();
  return <main style={{ padding: token.paddingLG, minHeight: '100vh' }}>{children}</main>;
}

const withOpsTheme: Decorator = (Story, ctx) => {
  const fixed = (ctx.parameters.opsTheme ?? {}) as OpsThemeParams;
  const mode = fixed.mode ?? (ctx.globals.mode as ThemeMode) ?? 'light';
  const density = fixed.density ?? (ctx.globals.density as Density) ?? 'default';
  const locale = ((ctx.globals.locale as OpsLocale) ?? 'vi-VN');
  if (i18n.language !== locale) void i18n.changeLanguage(locale);
  return (
    <I18nextProvider i18n={i18n}>
      <OpsThemeProvider mode={mode} density={density} locale={locale}>
        <StoryFrame>
          <Story />
        </StoryFrame>
      </OpsThemeProvider>
    </I18nextProvider>
  );
};

const preview: Preview = {
  decorators: [withOpsTheme],
  parameters: { layout: 'fullscreen', controls: { expanded: true } },
  globalTypes: {
    mode: { description: 'Theme mode', toolbar: { icon: 'mirror', items: ['light', 'dark'], dynamicTitle: true } },
    density: { description: 'Density', toolbar: { icon: 'component', items: ['default', 'compact'], dynamicTitle: true } },
    locale: { description: 'Locale', toolbar: { icon: 'globe', items: ['vi-VN', 'en-US'], dynamicTitle: true } },
  },
  initialGlobals: { mode: 'light', density: 'default', locale: 'vi-VN' },
};

export default preview;
