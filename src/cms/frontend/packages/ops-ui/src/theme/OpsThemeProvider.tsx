// OpsThemeProvider (F050-AC-05, TC-UI-005, DESIGN_SYSTEM §4.1): Ant Design ConfigProvider with the theme generated
// from design/admin-tokens (light, dark, compact), Ant Design + dayjs locale, CSS variables `--ew-*`.
// No override of Ant Design internal CSS: customization goes through tokens only.
import { App as AntApp, ConfigProvider, type ThemeConfig } from 'antd';
import enUS from 'antd/locale/en_US';
import viVN from 'antd/locale/vi_VN';
import dayjs from 'dayjs';
import 'dayjs/locale/en';
import 'dayjs/locale/vi';
import { createContext, useContext, useMemo, type ReactNode } from 'react';
import { compactOverrides, compactTheme, darkTheme, lightTheme } from './antd-theme';
import './tokens.css';
import './fonts.css';
import './base.css';

export type ThemeMode = 'light' | 'dark';
export type Density = 'default' | 'compact';
export type OpsLocale = 'vi-VN' | 'en-US';

export interface OpsThemeState {
  mode: ThemeMode;
  density: Density;
  locale: OpsLocale;
}

const ANTD_LOCALES = { 'vi-VN': viVN, 'en-US': enUS } as const;
export const DAYJS_LOCALES = { 'vi-VN': 'vi', 'en-US': 'en' } as const;

/** ThemeConfig for a mode + density; dark + compact merges compactOverrides into the dark seeds. */
export function resolveThemeConfig(mode: ThemeMode, density: Density): ThemeConfig {
  if (mode === 'dark') {
    return density === 'compact' ? { ...darkTheme, token: { ...darkTheme.token, ...compactOverrides } } : darkTheme;
  }
  return density === 'compact' ? compactTheme : lightTheme;
}

const OpsThemeContext = createContext<OpsThemeState>({ mode: 'light', density: 'default', locale: 'vi-VN' });

export function useOpsTheme(): OpsThemeState {
  return useContext(OpsThemeContext);
}

export interface OpsThemeProviderProps extends Partial<OpsThemeState> {
  children: ReactNode;
}

export function OpsThemeProvider({ mode = 'light', density = 'default', locale = 'vi-VN', children }: OpsThemeProviderProps) {
  // dayjs keeps a global locale: set it before children format dates
  if (dayjs.locale() !== DAYJS_LOCALES[locale]) dayjs.locale(DAYJS_LOCALES[locale]);
  const state = useMemo(() => ({ mode, density, locale }), [mode, density, locale]);
  const themeConfig = useMemo(() => resolveThemeConfig(mode, density), [mode, density]);
  return (
    <OpsThemeContext.Provider value={state}>
      <ConfigProvider theme={themeConfig} locale={ANTD_LOCALES[locale]}>
        <AntApp>
          <div className="ew-ops-root" data-theme={mode} data-density={density} lang={locale}>
            {children}
          </div>
        </AntApp>
      </ConfigProvider>
    </OpsThemeContext.Provider>
  );
}
