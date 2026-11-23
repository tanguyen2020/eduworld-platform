// Header quick toggles (ADMIN_PORTAL §1.3, owner 2026-11-22): language (globe + current code) and light/dark — used by
// the portal header and by the pre-sign-in pages other than /login (owner 2026-11-23: the login page is always
// English and shows no language control). Same PreferencesProvider + storage as before (F051-AC-16/17).
import { GlobalOutlined, MoonOutlined, SunOutlined } from '@ant-design/icons';
import { Button, Tooltip } from 'antd';
import type { CSSProperties } from 'react';
import { useTranslation } from 'react-i18next';
import { usePreferences } from '../app/PreferencesProvider';
import { LOCALES } from '../i18n';

/**
 * Header quick toggles (ADMIN_PORTAL §1.3, owner 2026-11-22): language (globe + current code, switches to the other
 * locale at once) and light/dark (sun/moon). Same PreferencesProvider + storage as before (AC-16/17); tooltip and
 * accessible name translated; the visible code is part of the accessible name (WCAG 2.5.3).
 */
export function LocaleToggle({ style }: { style?: CSSProperties }) {
  const { t } = useTranslation();
  const { preferences, update } = usePreferences();
  const current = preferences.locale;
  const next = LOCALES[(LOCALES.indexOf(current) + 1) % LOCALES.length]!;
  const code = t(`shell.locale.short.${current}`);
  const label = t('shell.header.language', { current: code, next: t(`shell.locale.${next}`) });
  return (
    <Tooltip title={label}>
      <Button type="text" icon={<GlobalOutlined />} aria-label={label} onClick={() => update({ locale: next })} style={style} data-testid="locale-toggle">
        {code}
      </Button>
    </Tooltip>
  );
}

export function ThemeToggle({ style }: { style?: CSSProperties }) {
  const { t } = useTranslation();
  const { preferences, update } = usePreferences();
  const dark = preferences.theme === 'dark';
  const label = dark ? t('shell.header.themeToLight') : t('shell.header.themeToDark');
  return (
    <Tooltip title={label}>
      <Button
        type="text"
        icon={dark ? <SunOutlined /> : <MoonOutlined />}
        aria-label={label}
        onClick={() => update({ theme: dark ? 'light' : 'dark' })}
        style={style}
        data-testid="theme-toggle"
      />
    </Tooltip>
  );
}
