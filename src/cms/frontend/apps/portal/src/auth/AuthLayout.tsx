// AuthLayout (F051 T058, ADMIN_PORTAL §1.3b — direction A "Sân trường", owner 2026-11-21): frame of the pages drawn by
// the portal itself (/login, /forgot-password, /reset-password, /account-setup, tenant picker). Two columns on desktop:
// a brand panel (primary colour, product name, headline, subject chips in the shared business colours, decorative
// SVG hidden from assistive tech) and the main column (language switch, one card, optional content below it). One
// column at phone width (< screenMD) with a compact brand band. Sizes of the sign-in controls come from the generated
// `authTheme` (nested ConfigProvider); every other value is an Ant Design theme token (AGENTS R32, ADR-0047).
import { authTheme, domainColors, OPS_UI_NS, useOpsTheme } from '@eduworld/ops-ui';
import { Card, ConfigProvider, Flex, Select, Typography, theme } from 'antd';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { usePreferences } from '../app/PreferencesProvider';
import { LOCALES, type Locale } from '../i18n';
import { useMediaQuery } from '../shell/useMediaQuery';

/** Subjects shown as chips on the brand panel (codes of shared-tokens `color.subject.*`, labels from ops-ui). */
export const BRAND_SUBJECTS = ['math', 'vietnamese', 'english', 'science'] as const;

export function LanguageSelect() {
  const { t } = useTranslation();
  const { preferences, update } = usePreferences();
  return (
    <Select<Locale>
      aria-label={t('shell.locale.label')}
      value={preferences.locale}
      onChange={(locale) => update({ locale })}
      options={LOCALES.map((l) => ({ value: l, label: t(`shell.locale.${l}`) }))}
      popupMatchSelectWidth={false}
    />
  );
}

/** Graduation cap on a light tile (decorative; the product name next to it is the text). */
function LogoMark() {
  const { token } = theme.useToken();
  return (
    <span
      aria-hidden="true"
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        justifyContent: 'center',
        flex: 'none',
        width: token.controlHeight,
        height: token.controlHeight,
        borderRadius: token.borderRadiusLG,
        background: token.colorBgContainer,
      }}
    >
      <svg width={token.sizeLG} height={token.sizeLG} viewBox="0 0 24 24" fill="none" stroke={token.colorPrimary} strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
        <path d="M2 8l10-5 10 5-10 5z" />
        <path d="M6 10.5v5c0 1.5 3 3 6 3s6-1.5 6-3v-5" />
      </svg>
    </span>
  );
}

/** School house, open book and soft circles drawn in the panel's text colour at low opacity (aria-hidden). */
function Decoration() {
  return (
    <svg
      aria-hidden="true"
      focusable="false"
      viewBox="0 0 520 820"
      preserveAspectRatio="xMidYMid slice"
      fill="currentColor"
      style={{ position: 'absolute', inset: 0, width: '100%', height: '100%', pointerEvents: 'none' }}
    >
      <circle cx="460" cy="90" r="120" fillOpacity={0.08} />
      <circle cx="40" cy="760" r="160" fillOpacity={0.07} />
      <rect x="300" y="520" width="150" height="110" rx="14" fillOpacity={0.12} />
      <path d="M300 520 L375 470 L450 520 Z" fillOpacity={0.18} />
      <rect x="360" y="570" width="30" height="60" rx="4" fillOpacity={0.22} />
      <path d="M90 170 q60 -30 120 0 v90 q-60 -30 -120 0 z" fillOpacity={0.16} />
      <path d="M210 170 q60 -30 120 0 v90 q-60 -30 -120 0 z" fillOpacity={0.1} />
    </svg>
  );
}

function SubjectChips() {
  const { t } = useTranslation();
  const { t: tOps } = useTranslation(OPS_UI_NS);
  const { token } = theme.useToken();
  const { mode } = useOpsTheme();
  return (
    <ul
      aria-label={t('shell.brand.subjects')}
      style={{ position: 'relative', display: 'flex', flexWrap: 'wrap', gap: token.marginXS, listStyle: 'none', margin: 0, padding: 0 }}
    >
      {BRAND_SUBJECTS.map((s) => (
        <li
          key={s}
          data-subject={s}
          style={{
            background: token.colorBgContainer,
            color: domainColors[`color.subject.${s}`]?.[mode],
            borderRadius: token.controlHeight,
            paddingInline: token.paddingSM,
            paddingBlock: token.paddingXXS,
            fontSize: token.fontSizeSM,
            fontWeight: token.fontWeightStrong,
          }}
        >
          {tOps(`domain.subject.${s}`)}
        </li>
      ))}
    </ul>
  );
}

function BrandPanel({ compact }: { compact: boolean }) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  return (
    <aside
      aria-label={t('shell.brand.panel')}
      data-testid="auth-brand-panel"
      style={{
        position: 'relative',
        overflow: 'hidden',
        display: 'flex',
        flexDirection: 'column',
        justifyContent: 'space-between',
        gap: token.marginXL,
        flex: compact ? 'none' : '0 0 40%',
        maxWidth: compact ? undefined : token.screenSM,
        padding: compact ? token.paddingMD : token.paddingXL,
        background: token.colorPrimary,
        color: token.colorTextLightSolid,
      }}
    >
      {compact ? null : <Decoration />}
      <Flex align="center" gap="middle" style={{ position: 'relative' }}>
        <LogoMark />
        <div>
          <div style={{ fontSize: token.fontSizeHeading4, fontWeight: token.fontWeightStrong, lineHeight: token.lineHeightHeading4 }}>{t('shell.brand.name')}</div>
          <div>{t('shell.brand.tagline')}</div>
        </div>
      </Flex>
      {compact ? null : (
        <>
          <Flex vertical gap="middle" style={{ position: 'relative', maxWidth: token.screenXS }}>
            <p style={{ margin: 0, fontSize: token.fontSizeHeading1, lineHeight: token.lineHeightHeading1, fontWeight: token.fontWeightStrong }}>{t('shell.brand.headline')}</p>
            <p style={{ margin: 0, fontSize: token.fontSizeLG, lineHeight: token.lineHeightLG }}>{t('shell.brand.lead')}</p>
          </Flex>
          <SubjectChips />
        </>
      )}
    </aside>
  );
}

export interface AuthLayoutProps {
  /** h1 of the card. */
  title: string;
  /** One line under the title. */
  subtitle?: ReactNode;
  children: ReactNode;
  /** Content under the card (hints, data protection line). */
  below?: ReactNode;
}

export function AuthLayout({ title, subtitle, children, below }: AuthLayoutProps) {
  const { token } = theme.useToken();
  const compact = !useMediaQuery(`(min-width: ${token.screenMD}px)`);
  return (
    <ConfigProvider theme={authTheme}>
      <div
        data-testid="auth-layout"
        data-layout={compact ? 'one-column' : 'two-columns'}
        style={{ minHeight: '100vh', display: 'flex', flexDirection: compact ? 'column' : 'row', background: token.colorBgLayout }}
      >
        <BrandPanel compact={compact} />
        <main style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: compact ? token.paddingMD : token.paddingXL }}>
          <Flex vertical gap="large" style={{ width: '100%', maxWidth: token.screenXS }}>
            <Flex justify="flex-end">
              <LanguageSelect />
            </Flex>
            <Card>
              <Typography.Title level={1} style={{ marginTop: 0, marginBottom: token.marginXXS, fontSize: token.fontSizeHeading2 }}>
                {title}
              </Typography.Title>
              {subtitle ? <Typography.Paragraph type="secondary">{subtitle}</Typography.Paragraph> : null}
              {children}
            </Card>
            {below}
          </Flex>
        </main>
      </div>
    </ConfigProvider>
  );
}
