// Session pages (F051 T020, T021): tenant picker after sign-in (F051-AC-03/05 — search by name/code, pre-selected
// remembered tenant, "Nhớ lựa chọn", sign out), "session ended" dialog (F051-AC-07) and the guard of every protected
// route that maps the session state to a page (spinner, /login with returnUrl, picker, no access, scope error).
import { Button, Checkbox, Flex, Input, Modal, Radio, Result, Spin, Tag, Typography } from 'antd';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Navigate, Outlet, useLocation, useNavigate } from 'react-router-dom';
import { AuthLayout } from '../auth/AuthLayout';
import { loginPath, returnTarget, safeReturnUrl } from '../auth/returnUrl';
import { NoAccessPage } from '../shell/StatusPages';
import { useSession } from './SessionProvider';
import type { MyTenant } from './sessionMachine';

/** `maxLength` of the tenant search box (same as `listTenants` `q`, ADR-0041). */
export const TENANT_PICKER_SEARCH_MAX_LENGTH = 100;

export function TenantTypeTag({ type }: { type: MyTenant['tenant_type'] }) {
  const { t } = useTranslation();
  return <Tag>{t(`shell.tenantType.${type}`)}</Tag>;
}

export function TenantSelectPage() {
  const { t } = useTranslation();
  const session = useSession();
  const location = useLocation();
  const { state } = session;
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState<string | null>(state.preselectedTenantId);
  const [remember, setRemember] = useState(state.preselectedTenantId !== null);
  const tenants = useMemo(() => {
    const q = query.trim().toLocaleLowerCase();
    return state.tenants.filter((x) => x.status !== 'disabled' && (!q || x.name.toLocaleLowerCase().includes(q) || x.code.toLocaleLowerCase().includes(q)));
  }, [state.tenants, query]);

  if (state.status === 'ready') return <Navigate to={returnTarget(location.search)} replace />;
  if (state.status !== 'selecting_tenant') return <Navigate to={`/${location.search}`} replace />;

  const enter = () => {
    const tenant = state.tenants.find((x) => x.tenant_id === selected);
    if (tenant) session.selectTenant(tenant, remember);
  };

  return (
    <AuthLayout title={t('shell.tenantSelect.title')}>
      <Flex vertical gap="middle">
        <Input.Search
          aria-label={t('shell.tenantSelect.search')}
          placeholder={t('shell.tenantSelect.search')}
          maxLength={TENANT_PICKER_SEARCH_MAX_LENGTH}
          allowClear
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
        {tenants.length === 0 ? (
          <Typography.Text type="secondary">{t('shell.tenantSelect.empty')}</Typography.Text>
        ) : (
          <Radio.Group value={selected} onChange={(e) => setSelected(e.target.value as string)} aria-label={t('shell.tenantSelect.title')}>
            <Flex vertical gap="small">
              {tenants.map((x) => (
                <Radio key={x.tenant_id} value={x.tenant_id}>
                  <Flex gap="small" align="center" wrap>
                    <Typography.Text>{x.name}</Typography.Text>
                    <Typography.Text type="secondary">{x.code}</Typography.Text>
                    <TenantTypeTag type={x.tenant_type} />
                  </Flex>
                </Radio>
              ))}
            </Flex>
          </Radio.Group>
        )}
        <Checkbox checked={remember} onChange={(e) => setRemember(e.target.checked)}>
          {t('shell.tenantSelect.remember')}
        </Checkbox>
        <Flex justify="space-between" gap="small">
          <Button onClick={() => void session.logout()}>{t('shell.session.logout')}</Button>
          <Button type="primary" disabled={!selected} onClick={enter}>
            {t('shell.tenantSelect.enter')}
          </Button>
        </Flex>
      </Flex>
    </AuthLayout>
  );
}

export function SessionExpiredModal({ open, onOk }: { open: boolean; onOk: () => void }) {
  const { t } = useTranslation();
  return (
    <Modal open={open} title={t('shell.session.expiredTitle')} onOk={onOk} okText={t('shell.session.loginAgain')} cancelButtonProps={{ style: { display: 'none' } }} closable={false} maskClosable={false} keyboard={false}>
      <Typography.Paragraph>{t('shell.session.expiredDescription')}</Typography.Paragraph>
    </Modal>
  );
}

/** Guard of the protected routes: the session state decides which page shows (no business logic). */
export function RequireSession() {
  const { t } = useTranslation();
  const session = useSession();
  const location = useLocation();
  const navigate = useNavigate();
  const here = `${location.pathname}${location.search}`;
  const status = session.state.status;

  if (status === 'restoring' || status === 'resolving_scope') {
    return (
      <Flex justify="center" align="center" style={{ minHeight: '100vh' }}>
        <Spin size="large" aria-label={t('shell.session.restoring')} />
      </Flex>
    );
  }
  if (status === 'unauthenticated') return <Navigate to={loginPath(here)} replace />;
  if (status === 'expired') return <SessionExpiredModal open onOk={() => session.acknowledgeExpired()} />;
  if (status === 'selecting_tenant') {
    const r = safeReturnUrl(here);
    return <Navigate to={r && r !== '/' ? `/select-tenant?returnUrl=${encodeURIComponent(r)}` : '/select-tenant'} replace />;
  }
  if (status === 'no_access') {
    return (
      <AuthLayout title={t('app.shortTitle')}>
        <NoAccessPage reason={session.state.noAccessReason ?? 'no_role'} onLogout={() => void session.logout().then(() => navigate('/login', { replace: true }))} />
      </AuthLayout>
    );
  }
  if (status === 'scope_error') {
    return (
      <AuthLayout title={t('app.shortTitle')}>
        <Result
          status="error"
          title={t('shell.session.scopeError')}
          extra={
            <Flex gap="small" justify="center">
              <Button onClick={() => void session.logout()}>{t('shell.session.logout')}</Button>
              <Button type="primary" onClick={() => session.retryScope()}>
                {t('shell.session.retry')}
              </Button>
            </Flex>
          }
        />
      </AuthLayout>
    );
  }
  return <Outlet />;
}
