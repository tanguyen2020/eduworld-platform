// LoginPage (F051 T058, F051-AC-01/04/30/31, ADR-0091 mục 1; layout: ADMIN_PORTAL §1.3b direction A "Sân trường"):
// the portal's own /login page — username or email, password (show/hide), "Quên mật khẩu?", language — sent as ONE
// `portalLogin` request with `device_id` and `locale`. No 6-digit code (owner 2026-11-21: portal MFA removed, new ADR
// replacing ADR-0091 mục 2): the request never carries `otp`. Every authentication failure shows one generic message
// and clears the password. No redirect to Keycloak, no tenant choice before sign-in, no tenant in the URL. The privacy
// policy and game links are not shown: the runtime config has no URL for them (AGENTS R3), only the sentences stay.
import { UserOutlined } from '@ant-design/icons';
import { Alert, App as AntApp, Button, Flex, Form, Input, Spin, Typography, theme } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, Navigate, useLocation, useNavigate } from 'react-router-dom';
import { usePortalRuntime } from '../app/runtime';
import type { Locale } from '../i18n';
import { useSession } from '../session/SessionProvider';
import { removeItem, STORAGE_KEYS } from '../shared/storage';
import { AuthLayout } from './AuthLayout';
import { deviceId } from './deviceId';
import { setupTicketStore } from './memoryStores';
import { returnTarget } from './returnUrl';

/** `maxLength` of PortalLoginRequest (specs/openapi/eduworld-api.v1.yaml, ADR-0041). */
export const LOGIN_LIMITS = { username: 254, password: 256 } as const;

interface LoginValues {
  username: string;
  password: string;
}

type LoginFailure = { kind: 'message'; text: string } | { kind: 'student' };

/** "Em là học sinh?" hint and the data protection line under the card. */
function LoginFooter() {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  return (
    <>
      <Flex
        gap="small"
        align="flex-start"
        data-testid="student-hint"
        style={{
          background: token.colorInfoBg,
          border: `${token.lineWidth}px ${token.lineType} ${token.colorInfoBorder}`,
          borderRadius: token.borderRadiusLG,
          padding: token.padding,
        }}
      >
        <UserOutlined aria-hidden style={{ color: token.colorInfo, marginTop: token.marginXXS }} />
        <Typography.Text>{t('shell.login.studentHint')}</Typography.Text>
      </Flex>
      <Typography.Paragraph type="secondary" style={{ textAlign: 'center', marginBottom: 0 }}>
        {t('shell.login.privacy')}
      </Typography.Paragraph>
    </>
  );
}

export function LoginPage() {
  const { t, i18n } = useTranslation();
  const rt = usePortalRuntime();
  const session = useSession();
  const { notification } = AntApp.useApp();
  const location = useLocation();
  const navigate = useNavigate();
  const [form] = Form.useForm<LoginValues>();
  const [failure, setFailure] = useState<LoginFailure | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const status = session.state.status;

  if (status === 'restoring') {
    return (
      <AuthLayout title={t('shell.login.welcome')}>
        <Spin aria-label={t('shell.session.restoring')} />
      </AuthLayout>
    );
  }
  if (status !== 'unauthenticated' && status !== 'expired') return <Navigate to={returnTarget(location.search)} replace />;

  const onFinish = async (values: LoginValues) => {
    setSubmitting(true);
    setFailure(null);
    session.setNotice(null);
    const r = await rt.authApi.login({
      username: values.username.trim(),
      password: values.password,
      device_id: deviceId(),
      locale: i18n.language as Locale,
    });
    setSubmitting(false);
    if (r.ok) {
      removeItem('local', STORAGE_KEYS.legacyOtpHint);
      if (!rt.readCsrf()) notification.warning({ key: 'cookies-blocked', message: t('shell.login.cookiesBlocked') });
      session.loginSucceeded(r.data);
      return;
    }
    const code = r.error.code;
    if (code === 'ACCOUNT_SETUP_REQUIRED' && r.setup) {
      setupTicketStore.set(r.setup);
      navigate('/account-setup');
      return;
    }
    // the password never stays in the form after a failure
    form.setFieldsValue({ password: '' });
    if (code === 'INVALID_CREDENTIALS') setFailure({ kind: 'message', text: t('shell.login.invalidCredentials') });
    else if (code === 'PORTAL_ACCESS_STAFF_ONLY') setFailure({ kind: 'student' });
    else if (code === 'ACCOUNT_DEACTIVATED') setFailure({ kind: 'message', text: t('shell.login.accountDeactivated') });
    else if (code === 'RATE_LIMITED') {
      const seconds = r.error.retryAfterMs ? Math.ceil(r.error.retryAfterMs / 1000) : null;
      setFailure({ kind: 'message', text: seconds ? t('shell.login.rateLimited', { seconds }) : t('shell.login.rateLimitedNoTime') });
    } else {
      const trace = r.error.traceId ? ` ${t('shell.problem.traceId', { traceId: r.error.traceId })}` : '';
      setFailure({ kind: 'message', text: `${t(r.error.i18nKey)}${trace}` });
    }
  };

  const notice = session.notice;
  return (
    <AuthLayout title={t('shell.login.welcome')} subtitle={t('shell.login.subtitle')} below={<LoginFooter />}>
      <Flex vertical gap="middle">
        {notice ? <Alert type="info" showIcon message={t(`shell.login.notice.${notice}`)} /> : null}
        {failure?.kind === 'message' ? <Alert type="error" showIcon message={failure.text} role="alert" /> : null}
        {failure?.kind === 'student' ? (
          <Alert type="warning" showIcon role="alert" message={t('shell.noAccess.studentTitle')} description={t('shell.noAccess.studentDescription')} />
        ) : null}
        <Form<LoginValues>
          form={form}
          name="portal-login"
          aria-label={t('shell.login.title')}
          layout="vertical"
          requiredMark={false}
          onFinish={onFinish}
          disabled={submitting}
          autoComplete="on"
        >
          <Form.Item name="username" label={t('shell.login.username')} rules={[{ required: true, whitespace: true, message: t('shell.login.usernameRequired') }]}>
            <Input autoComplete="username" maxLength={LOGIN_LIMITS.username} autoFocus />
          </Form.Item>
          <Form.Item name="password" label={t('shell.login.password')} rules={[{ required: true, message: t('shell.login.passwordRequired') }]}>
            <Input.Password autoComplete="current-password" maxLength={LOGIN_LIMITS.password} />
          </Form.Item>
          <Form.Item style={{ marginBottom: 0 }}>
            <Button type="primary" htmlType="submit" block loading={submitting}>
              {t('shell.login.submit')}
            </Button>
          </Form.Item>
        </Form>
        <Flex justify="space-between" align="baseline" wrap gap="small">
          <Link to="/forgot-password">{t('shell.login.forgot')}</Link>
          <Typography.Text type="secondary">{t('shell.login.support')}</Typography.Text>
        </Flex>
      </Flex>
    </AuthLayout>
  );
}
