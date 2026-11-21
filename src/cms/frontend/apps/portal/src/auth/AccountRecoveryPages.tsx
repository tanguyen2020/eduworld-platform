// Account setup and password recovery (F051 T058, F051-AC-31, F051-AC-32, ADR-0091 mục 3):
// - /account-setup: `403 ACCOUNT_SETUP_REQUIRED` of portalLogin → new password (setupPortalPassword with the
//   in-memory setup_ticket, policy errors per field) → back to /login (no authenticator step, ADR-0094);
//   `401 SETUP_TICKET_INVALID` → /login with the "expired" notice.
// - /forgot-password: forgotPortalPassword, always the same confirmation (no account enumeration).
// - /reset-password?token=…: the token is read then removed from the URL at once (history.replaceState), kept in
//   memory only, never sent to telemetry; the page runs with `Referrer-Policy: no-referrer` (index.html meta);
//   resetPortalPassword then /login; `401 PASSWORD_RESET_TOKEN_INVALID` → "link no longer valid" + request again.
import { Alert, Button, Flex, Form, Input, Result, Typography } from 'antd';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, Navigate, useNavigate } from 'react-router-dom';
import { usePortalRuntime } from '../app/runtime';
import type { Locale } from '../i18n';
import { useSession } from '../session/SessionProvider';
import type { ApiError } from '../shared/api/middleware/problem';
import { AuthLayout } from './AuthLayout';
import type { AuthResult } from './authApi';
import { deviceId } from './deviceId';
import { LOGIN_LIMITS } from './LoginPage';
import { resetTokenStore, setupTicketStore } from './memoryStores';

/** `maxLength` of `new_password` (PortalSetupPasswordRequest / PortalResetPasswordRequest). */
export const NEW_PASSWORD_MAX_LENGTH = 256;
const RESET_TOKEN = /^[A-Za-z0-9_-]{32,128}$/;

interface PasswordValues {
  new_password: string;
  confirm: string;
}

function errorText(t: (k: string, o?: Record<string, unknown>) => string, e: ApiError): string {
  const trace = e.traceId ? ` ${t('shell.problem.traceId', { traceId: e.traceId })}` : '';
  if (e.code === 'RATE_LIMITED' && e.retryAfterMs) return t('shell.login.rateLimited', { seconds: Math.ceil(e.retryAfterMs / 1000) });
  return `${t(e.i18nKey)}${trace}`;
}

/** New password + confirmation; policy violations of the realm come back per field (422 VALIDATION_FAILED). */
function NewPasswordForm({ submitLabel, onSubmit }: { submitLabel: string; onSubmit: (password: string) => Promise<AuthResult<unknown>> }) {
  const { t } = useTranslation();
  const [form] = Form.useForm<PasswordValues>();
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const onFinish = async (v: PasswordValues) => {
    setSubmitting(true);
    setError(null);
    const r = await onSubmit(v.new_password);
    setSubmitting(false);
    if (r.ok) return;
    form.setFieldsValue({ new_password: '', confirm: '' });
    const field = r.error.fieldErrors.new_password ?? Object.values(r.error.fieldErrors)[0];
    if (r.error.code === 'VALIDATION_FAILED' && field) {
      form.setFields([{ name: 'new_password', errors: [t('shell.setup.policyViolation')] }]);
      return;
    }
    setError(errorText(t, r.error));
  };
  return (
    <Flex vertical gap="middle">
      {error ? <Alert type="error" showIcon message={error} role="alert" /> : null}
      <Form<PasswordValues> form={form} layout="vertical" requiredMark={false} onFinish={onFinish} disabled={submitting}>
        <Form.Item name="new_password" label={t('shell.setup.newPassword')} extra={t('shell.setup.policyHint')} rules={[{ required: true, message: t('shell.login.passwordRequired') }]}>
          <Input.Password autoComplete="new-password" maxLength={NEW_PASSWORD_MAX_LENGTH} />
        </Form.Item>
        <Form.Item
          name="confirm"
          label={t('shell.setup.confirmPassword')}
          dependencies={['new_password']}
          rules={[
            { required: true, message: t('shell.login.passwordRequired') },
            ({ getFieldValue }) => ({
              validator: (_, value) => (!value || value === getFieldValue('new_password') ? Promise.resolve() : Promise.reject(new Error(t('shell.setup.mismatch')))),
            }),
          ]}
        >
          <Input.Password autoComplete="new-password" maxLength={NEW_PASSWORD_MAX_LENGTH} />
        </Form.Item>
        <Button type="primary" htmlType="submit" block loading={submitting}>
          {submitLabel}
        </Button>
      </Form>
    </Flex>
  );
}

export function AccountSetupPage() {
  const { t } = useTranslation();
  const rt = usePortalRuntime();
  const session = useSession();
  const navigate = useNavigate();
  const [setup] = useState(() => setupTicketStore.get());
  // the ticket leaves memory when the page is left
  useEffect(() => () => setupTicketStore.clear(), []);

  if (!setup) return <Navigate to="/login" replace />;
  return (
    <AuthLayout title={t('shell.setup.title')}>
      <Typography.Paragraph>{t('shell.setup.description')}</Typography.Paragraph>
      <NewPasswordForm
        submitLabel={t('shell.setup.submit')}
        onSubmit={async (password) => {
          const r = await rt.authApi.setupPassword({ setup_ticket: setup.setupTicket, new_password: password });
          if (r.ok) {
            setupTicketStore.clear();
            session.setNotice('passwordChanged');
            navigate('/login', { replace: true });
          } else if (r.error.code === 'SETUP_TICKET_INVALID') {
            setupTicketStore.clear();
            session.setNotice('setupTicketExpired');
            navigate('/login', { replace: true });
          }
          return r;
        }}
      />
    </AuthLayout>
  );
}

export function ForgotPasswordPage() {
  const { t, i18n } = useTranslation();
  const rt = usePortalRuntime();
  const [sent, setSent] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const onFinish = async (v: { identifier: string }) => {
    setSubmitting(true);
    setError(null);
    const r = await rt.authApi.forgotPassword({ identifier: v.identifier.trim(), device_id: deviceId(), locale: i18n.language as Locale });
    setSubmitting(false);
    // 202 for every identifier: the same confirmation whether the account exists or not
    if (r.ok) setSent(true);
    else setError(errorText(t, r.error));
  };
  return (
    <AuthLayout title={t('shell.forgot.title')}>
      <Flex vertical gap="middle">
        {sent ? (
          <Alert type="success" showIcon message={t('shell.forgot.sent')} role="status" />
        ) : (
          <>
            <Typography.Paragraph>{t('shell.forgot.description')}</Typography.Paragraph>
            {error ? <Alert type="error" showIcon message={error} role="alert" /> : null}
            <Form layout="vertical" requiredMark={false} onFinish={onFinish} disabled={submitting}>
              <Form.Item name="identifier" label={t('shell.login.username')} rules={[{ required: true, whitespace: true, message: t('shell.login.usernameRequired') }]}>
                <Input autoComplete="username" maxLength={LOGIN_LIMITS.username} autoFocus />
              </Form.Item>
              <Button type="primary" htmlType="submit" block loading={submitting}>
                {t('shell.forgot.submit')}
              </Button>
            </Form>
          </>
        )}
        <Link to="/login">{t('shell.forgot.backToLogin')}</Link>
      </Flex>
    </AuthLayout>
  );
}

/** Reads `?token=` once, keeps it in memory and removes it from the address bar and history entry at once. */
export function takeResetTokenFromUrl(win: Pick<Window, 'location' | 'history'> = window): string | null {
  const url = new URL(win.location.href);
  const token = url.searchParams.get('token');
  if (token !== null) {
    url.searchParams.delete('token');
    win.history.replaceState(null, '', `${url.pathname}${url.search}${url.hash}`);
    if (RESET_TOKEN.test(token)) resetTokenStore.set(token);
  }
  return resetTokenStore.get();
}

export function ResetPasswordPage() {
  const { t } = useTranslation();
  const rt = usePortalRuntime();
  const session = useSession();
  const navigate = useNavigate();
  const [token] = useState(() => takeResetTokenFromUrl());
  const [invalid, setInvalid] = useState(!token);
  const signedIn = session.state.status !== 'unauthenticated' && session.state.status !== 'restoring' && session.state.status !== 'expired';

  // edge case: a session is open in this tab → sign it out first (the backend revokes it anyway)
  useEffect(() => {
    if (signedIn) void session.logout();
  }, [signedIn, session]);
  useEffect(() => () => resetTokenStore.clear(), []);

  if (invalid || !token) {
    return (
      <AuthLayout title={t('shell.reset.title')}>
        <Result
          status="warning"
          title={t('shell.reset.invalidTitle')}
          subTitle={t('shell.reset.invalidDescription')}
          extra={
            <Button type="primary" onClick={() => navigate('/forgot-password', { replace: true })}>
              {t('shell.reset.requestAgain')}
            </Button>
          }
        />
      </AuthLayout>
    );
  }
  return (
    <AuthLayout title={t('shell.reset.title')}>
      <NewPasswordForm
        submitLabel={t('shell.reset.submit')}
        onSubmit={async (password) => {
          const r = await rt.authApi.resetPassword({ token, new_password: password, device_id: deviceId() });
          if (r.ok) {
            resetTokenStore.clear();
            session.setNotice('passwordReset');
            navigate('/login', { replace: true });
          } else if (r.error.code === 'PASSWORD_RESET_TOKEN_INVALID') {
            resetTokenStore.clear();
            setInvalid(true);
          }
          return r;
        }}
      />
    </AuthLayout>
  );
}
