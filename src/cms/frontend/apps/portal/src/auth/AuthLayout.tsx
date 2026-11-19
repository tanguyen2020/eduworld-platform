// AuthLayout (F051 T058, ADMIN_PORTAL §1.3b wireframe): frame of the pages drawn by the portal itself (/login,
// /forgot-password, /reset-password, /account-setup, tenant picker): product name, language switch, one centered
// card. Ant Design + theme tokens only (AGENTS R32, ADR-0047); no Keycloak page (ADR-0091).
import { Card, Flex, Layout, Select, Typography, theme } from 'antd';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { usePreferences } from '../app/PreferencesProvider';
import { LOCALES, type Locale } from '../i18n';

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

export function AuthLayout({ title, children, footer }: { title: string; children: ReactNode; footer?: ReactNode }) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  return (
    <Layout style={{ minHeight: '100vh', background: token.colorBgLayout }}>
      <Layout.Header style={{ background: token.colorBgContainer, paddingInline: token.paddingLG, borderBottom: `${token.lineWidth}px ${token.lineType} ${token.colorBorderSecondary}` }}>
        <Flex justify="space-between" align="center" style={{ height: '100%' }}>
          <Typography.Text strong>{t('app.title')}</Typography.Text>
          <LanguageSelect />
        </Flex>
      </Layout.Header>
      <Layout.Content style={{ padding: token.paddingLG }}>
          <Flex vertical align="center" gap={token.marginLG}>
            <Card style={{ width: '100%', maxWidth: token.screenSM }}>
              <Typography.Title level={1} style={{ marginTop: 0, fontSize: token.fontSizeHeading3 }}>
                {title}
              </Typography.Title>
              {children}
            </Card>
            {footer ? (
              <Typography.Paragraph type="secondary" style={{ maxWidth: token.screenSM, textAlign: 'center' }}>
                {footer}
              </Typography.Paragraph>
            ) : null}
          </Flex>
      </Layout.Content>
    </Layout>
  );
}
