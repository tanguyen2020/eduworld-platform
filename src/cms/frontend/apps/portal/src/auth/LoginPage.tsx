// LoginPage (F051 T058, F051-AC-01/04/30/31, ADR-0091 mục 1): the portal's own /login page — username or email,
// password (show/hide), optional 6-digit code, "Quên mật khẩu?", language — sent as ONE `portalLogin` request with
// `device_id` and `locale`. Every authentication failure shows one generic message and clears password and code; the
// only thing remembered is the non-secret flag "a code was used last time" (`ew.portal.otpHint`). No redirect to
// Keycloak, no tenant choice before sign-in, no tenant in the URL.
import { Alert, App as AntApp, Button, Flex, Form, Input, Spin, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, Navigate, useLocation, useNavigate } from 'react-router-dom';
import { usePortalRuntime } from '../app/runtime';
import type { Locale } from '../i18n';
import { useSession } from '../session/SessionProvider';
import { readItem, STORAGE_KEYS, writeItem } from '../shared/storage';
import { AuthLayout } from './AuthLayout';
import { deviceId } from './deviceId';
import { setupTicketStore } from './memoryStores';
import { returnTarget } from './returnUrl';

/** `maxLength` of PortalLoginRequest (specs/openapi/eduworld-api.v1.yaml, ADR-0041). */
export const LOGIN_LIMITS = { username: 254, password: 256, otp: 6 } as const;
export const OTP_PATTERN = /^[0-9]{6}$/;

interface LoginValues {
  username: string;
  password: string;
  otp?: string;
}

type LoginFailure = { kind: 'message'; text: string } | { kind: 'student' };

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
  const otpHint = readItem('local', STORAGE_KEYS.otpHint) === 'true';
  const status = session.state.status;

  if (status === 'restoring') {
    return (
      <AuthLayout title={t('shell.login.title')}>
        <Spin aria-label={t('shell.session.restoring')} />
      </AuthLayout>
    );
  }
  if (status !== 'unauthenticated' && status !== 'expired') return <Navigate to={returnTarget(location.search)} replace />;

  const onFinish = async (values: LoginValues) => {
    setSubmitting(true);
    setFailure(null);
    session.setNotice(null);
    const otp = values.otp?.trim() || undefined;
    const r = await rt.authApi.login({
      username: values.username.trim(),
      password: values.password,
      ...(otp ? { otp } : {}),
      device_id: deviceId(),
      locale: i18n.language as Locale,
    });
    setSubmitting(false);
    if (r.ok) {
      writeItem('local', STORAGE_KEYS.otpHint, String(Boolean(otp)));
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
    // secrets never stay in the form after a failure
    form.setFieldsValue({ password: '', otp: '' });
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
    <AuthLayout title={t('shell.login.title')} footer={t('shell.login.audience')}>
      <Flex vertical gap="middle">
        {notice ? <Alert type="info" showIcon message={t(`shell.login.notice.${notice}`)} /> : null}
        {failure?.kind === 'message' ? <Alert type="error" showIcon message={failure.text} role="alert" /> : null}
        {failure?.kind === 'student' ? (
          <Alert type="warning" showIcon role="alert" message={t('shell.noAccess.studentTitle')} description={t('shell.noAccess.studentDescription')} />
        ) : null}
        <Form<LoginValues> form={form} layout="vertical" requiredMark={false} onFinish={onFinish} disabled={submitting} autoComplete="on">
          <Form.Item name="username" label={t('shell.login.username')} rules={[{ required: true, whitespace: true, message: t('shell.login.usernameRequired') }]}>
            <Input autoComplete="username" maxLength={LOGIN_LIMITS.username} autoFocus />
          </Form.Item>
          <Form.Item name="password" label={t('shell.login.password')} rules={[{ required: true, message: t('shell.login.passwordRequired') }]}>
            <Input.Password autoComplete="current-password" maxLength={LOGIN_LIMITS.password} />
          </Form.Item>
          <Form.Item
            name="otp"
            label={t('shell.login.otp')}
            extra={otpHint ? <Typography.Text strong>{t('shell.login.otpRemembered')}</Typography.Text> : t('shell.login.otpHelp')}
            rules={[{ pattern: OTP_PATTERN, message: t('shell.login.otpFormat') }]}
          >
            <Input inputMode="numeric" autoComplete="one-time-code" maxLength={LOGIN_LIMITS.otp} status={otpHint ? 'warning' : undefined} />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" block loading={submitting}>
              {t('shell.login.submit')}
            </Button>
          </Form.Item>
        </Form>
        <Link to="/forgot-password">{t('shell.login.forgot')}</Link>
      </Flex>
    </AuthLayout>
  );
}
