// Minimal portal frame (F050). The real shell (ProLayout, login, tenant picker, menu by permission) is F051.
// Spacing and colors come from the Ant Design theme tokens (generated from design/admin-tokens).
import { Layout, Spin, Typography, theme } from 'antd';
import { Suspense } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, Outlet, useNavigation } from 'react-router-dom';

export function Shell() {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  const navigation = useNavigation();
  const loading = <Spin aria-label={t('app.loading')} />;
  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Layout.Header style={{ background: token.colorBgContainer, borderBottom: `${token.lineWidth}px ${token.lineType} ${token.colorBorderSecondary}`, paddingInline: token.paddingLG }}>
        <Link to="/">
          <Typography.Text strong>{t('app.title')}</Typography.Text>
        </Link>
      </Layout.Header>
      <Layout.Content style={{ padding: token.paddingLG }}>
        <main>
          <Suspense fallback={loading}>{navigation.state === 'loading' ? loading : <Outlet />}</Suspense>
        </main>
      </Layout.Content>
    </Layout>
  );
}
