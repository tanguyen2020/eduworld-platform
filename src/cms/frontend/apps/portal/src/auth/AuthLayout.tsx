// AuthLayout (F051 T058, ADMIN_PORTAL §1.3b — direction A "Sân trường", owner 2026-11-21): frame of the pages drawn by
// the portal itself (/login, /forgot-password, /reset-password, /account-setup, tenant picker). Two columns on desktop:
// a banner (primary colour, "Cổng quản trị EduWorld" for the whole platform, capability list with small icons, an
// illustration of simple shapes in token colours, all decorative SVG hidden from assistive tech) and the main column (language switch, one card, optional content below it). One
// column at phone width (< screenMD) with a compact header (logo + headline). Sizes of the sign-in controls come from the generated
// `authTheme` (nested ConfigProvider); every other value is an Ant Design theme token (AGENTS R32, ADR-0047).
import { authTheme, domainColors, useOpsTheme } from '@eduworld/ops-ui';
import { Card, ConfigProvider, Flex, Select, Typography, theme } from 'antd';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { usePreferences } from '../app/PreferencesProvider';
import { LOCALES, type Locale } from '../i18n';
import { useMediaQuery } from '../shell/useMediaQuery';

/** What the portal manages, listed on the banner (i18n `shell.brand.capability.<key>`; grade wording — ADR-0095). */
export const BRAND_CAPABILITIES = ['questionBank', 'lessons', 'gameWorld', 'gradeProgress', 'safety'] as const;

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

/** Soft circles in the panel's text colour at low opacity (background of the banner, aria-hidden). */
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
      <circle cx="470" cy="80" r="130" fillOpacity={0.08} />
      <circle cx="30" cy="770" r="170" fillOpacity={0.07} />
    </svg>
  );
}

/**
 * Banner illustration (decorative, aria-hidden): a question card with answer options, a lesson book, a game-world map
 * pin with a star and a progress chart — simple shapes coloured with tokens (container, brand secondary, subject
 * colours of the active mode). No game art asset, no emoji (AGENTS R32).
 */
function BannerIllustration() {
  const { token } = theme.useToken();
  const { mode } = useOpsTheme();
  const c = (name: string) => domainColors[name]?.[mode] ?? token.colorBgContainer;
  const card = token.colorBgContainer;
  return (
    <svg aria-hidden="true" focusable="false" data-testid="banner-illustration" viewBox="0 0 400 200" style={{ position: 'relative', width: '100%', maxWidth: token.screenXS, height: 'auto' }}>
      {/* question card */}
      <rect x="10" y="30" width="150" height="150" rx="14" fill={card} />
      <rect x="28" y="50" width="90" height="10" rx="5" fill={c('color.subject.math')} />
      <rect x="28" y="70" width="114" height="8" rx="4" fill={c('color.content_status.draft')} fillOpacity={0.35} />
      <circle cx="36" cy="102" r="7" fill="none" stroke={c('color.subject.math')} strokeWidth={3} />
      <rect x="50" y="98" width="80" height="8" rx="4" fill={c('color.content_status.draft')} fillOpacity={0.35} />
      <circle cx="36" cy="126" r="7" fill={c('color.subject.science')} />
      <rect x="50" y="122" width="64" height="8" rx="4" fill={c('color.content_status.draft')} fillOpacity={0.35} />
      <circle cx="36" cy="150" r="7" fill="none" stroke={c('color.subject.math')} strokeWidth={3} />
      <rect x="50" y="146" width="72" height="8" rx="4" fill={c('color.content_status.draft')} fillOpacity={0.35} />
      {/* lesson book */}
      <path d="M150 20 q30 -14 60 0 v50 q-30 -14 -60 0 z" fill={c('color.subject.vietnamese')} />
      <path d="M210 20 q30 -14 60 0 v50 q-30 -14 -60 0 z" fill={c('color.subject.english')} />
      {/* game world: map pin with a star, a path to it */}
      <path d="M190 170 q40 -40 90 -20 t80 -40" fill="none" stroke={card} strokeOpacity={0.6} strokeWidth={4} strokeDasharray="8 8" strokeLinecap="round" />
      <path d="M345 40 c-22 0 -36 16 -36 36 c0 26 36 60 36 60 s36 -34 36 -60 c0 -20 -14 -36 -36 -36 z" fill={c('color.brand.secondary')} />
      <path d="M345 58 l6 12 l13 2 l-9 9 l2 13 l-12 -6 l-12 6 l2 -13 l-9 -9 l13 -2 z" fill={card} />
      {/* progress chart */}
      <rect x="200" y="96" width="96" height="84" rx="12" fill={card} />
      <rect x="214" y="146" width="16" height="22" rx="4" fill={c('color.subject.vietnamese')} />
      <rect x="240" y="130" width="16" height="38" rx="4" fill={c('color.subject.english')} />
      <rect x="266" y="112" width="16" height="56" rx="4" fill={c('color.subject.science')} />
    </svg>
  );
}

/** Small stroke icons of the capability list (24×24, currentColor, aria-hidden). */
const CAPABILITY_ICONS: Record<(typeof BRAND_CAPABILITIES)[number], ReactNode> = {
  questionBank: (
    <>
      <rect x="4" y="3" width="16" height="18" rx="2" />
      <path d="M9.5 9a2.5 2.5 0 1 1 3.5 2.3c-.6.3-1 .8-1 1.5V14M12 17.5v.01" />
    </>
  ),
  lessons: (
    <>
      <path d="M2 5c3-1.5 7-1.5 10 1v14c-3-2.5-7-2.5-10-1z" />
      <path d="M22 5c-3-1.5-7-1.5-10 1v14c3-2.5 7-2.5 10-1z" />
    </>
  ),
  gameWorld: (
    <>
      <path d="M12 21s7-6.2 7-11.5a7 7 0 1 0-14 0C5 14.8 12 21 12 21z" />
      <path d="M12 6.5l1.2 2.4 2.6.4-1.9 1.8.5 2.6-2.4-1.2-2.4 1.2.5-2.6-1.9-1.8 2.6-.4z" />
    </>
  ),
  gradeProgress: (
    <>
      <path d="M4 20V4M4 20h16" />
      <path d="M8 16v-4M12 16V9M16 16v-7" />
    </>
  ),
  safety: (
    <>
      <path d="M12 3l8 3v6c0 4.5-3.4 8.2-8 9-4.6-.8-8-4.5-8-9V6z" />
      <path d="M8.5 12l2.5 2.5 4.5-5" />
    </>
  ),
};

function Capabilities() {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  return (
    <ul aria-label={t('shell.brand.capabilities')} style={{ position: 'relative', display: 'flex', flexDirection: 'column', gap: token.marginSM, listStyle: 'none', margin: 0, padding: 0 }}>
      {BRAND_CAPABILITIES.map((k) => (
        <li key={k} data-capability={k} style={{ display: 'flex', alignItems: 'center', gap: token.marginSM, fontSize: token.fontSizeLG }}>
          <span
            aria-hidden="true"
            style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', flex: 'none', width: token.controlHeightSM, height: token.controlHeightSM, borderRadius: token.borderRadius, background: token.colorBgContainer, color: token.colorPrimary }}
          >
            <svg width={token.sizeMD} height={token.sizeMD} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round">
              {CAPABILITY_ICONS[k]}
            </svg>
          </span>
          {t(`shell.brand.capability.${k}`)}
        </li>
      ))}
    </ul>
  );
}

/**
 * Banner of the sign-in pages (owner 2026-11-21): the portal manages the whole EduWorld platform — learning content,
 * the game world and student follow-up — not only school teaching. Phone width: a compact header with the headline.
 */
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
        gap: compact ? token.marginSM : token.marginLG,
        flex: compact ? 'none' : '0 0 42%',
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
          {compact ? null : <div>{t('shell.brand.tagline')}</div>}
        </div>
      </Flex>
      <Flex vertical gap="small" style={{ position: 'relative', maxWidth: token.screenXS }}>
        <p style={{ margin: 0, fontSize: compact ? token.fontSizeHeading4 : token.fontSizeHeading1, lineHeight: token.lineHeightHeading1, fontWeight: token.fontWeightStrong }}>
          {t('shell.brand.headline')}
        </p>
        {compact ? null : <p style={{ margin: 0, fontSize: token.fontSizeLG, lineHeight: token.lineHeightLG }}>{t('shell.brand.lead')}</p>}
      </Flex>
      {compact ? null : (
        <>
          <BannerIllustration />
          <Capabilities />
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
