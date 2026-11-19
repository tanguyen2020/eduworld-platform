// ReLoginDialog (F051 T034, F051-AC-13, ADR-0091 mục 4): on `403 MFA_REQUIRED` (auth_time older than 8 h or no MFA on a
// platform route) the apiClient asks the session to re-authenticate. This dialog opens on the current page (URL kept):
// username, password and code → `portalLogin`; success replaces the access token and the pending request is retried
// once; cancel → back to /login. A sign-in as another user is refused (the session would mix two identities).
import { Alert, Form, Input, Modal, Typography } from 'antd';
import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate } from 'react-router-dom';
import { usePortalRuntime } from '../app/runtime';
import type { Locale } from '../i18n';
import { useSession } from '../session/SessionProvider';
import { deviceId } from './deviceId';
import { LOGIN_LIMITS, OTP_PATTERN } from './LoginPage';
import { loginPath } from './returnUrl';
import { tokenSubject } from './tokenStore';

interface Values {
  username: string;
  password: string;
  otp: string;
}

export function ReLoginHost() {
  const { t, i18n } = useTranslation();
  const rt = usePortalRuntime();
  const session = useSession();
  const navigate = useNavigate();
  const location = useLocation();
  const [form] = Form.useForm<Values>();
  const [open, setOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const resolver = useRef<((ok: boolean) => void) | null>(null);

  const request = useCallback(
    () =>
      new Promise<boolean>((resolve) => {
        resolver.current = resolve;
        setError(null);
        setOpen(true);
      }),
    [],
  );

  useEffect(() => {
    rt.reauth.bind(request);
    return () => rt.reauth.bind(null);
  }, [rt, request]);

  const finish = (ok: boolean) => {
    form.resetFields();
    setOpen(false);
    resolver.current?.(ok);
    resolver.current = null;
  };

  const onCancel = () => {
    finish(false);
    const back = loginPath(`${location.pathname}${location.search}`);
    void session.logout().then(() => navigate(back, { replace: true }));
  };

  const onOk = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    setSubmitting(true);
    setError(null);
    const before = rt.tokens.get();
    const r = await rt.authApi.login({ username: v.username.trim(), password: v.password, otp: v.otp, device_id: deviceId(), locale: i18n.language as Locale });
    setSubmitting(false);
    if (!r.ok) {
      form.setFieldsValue({ password: '', otp: '' });
      setError(r.error.code === 'INVALID_CREDENTIALS' ? t('shell.login.invalidCredentials') : t(r.error.i18nKey));
      return;
    }
    const prevSub = before ? tokenSubject(before.token) : null;
    const nextSub = tokenSubject(r.data.access_token);
    if (prevSub && nextSub && prevSub !== nextSub) {
      form.setFieldsValue({ password: '', otp: '' });
      setError(t('shell.login.invalidCredentials'));
      return;
    }
    rt.tokens.setFromResponse(r.data);
    rt.scheduler.schedule();
    finish(true);
  };

  return (
    <Modal
      open={open}
      title={t('shell.relogin.title')}
      okText={t('shell.relogin.submit')}
      cancelText={t('shell.relogin.cancel')}
      onOk={onOk}
      onCancel={onCancel}
      confirmLoading={submitting}
      maskClosable={false}
      destroyOnHidden
    >
      <Typography.Paragraph>{t('shell.relogin.description')}</Typography.Paragraph>
      {error ? <Alert type="error" showIcon message={error} role="alert" /> : null}
      <Form<Values> form={form} layout="vertical" requiredMark={false} preserve={false}>
        <Form.Item name="username" label={t('shell.login.username')} rules={[{ required: true, whitespace: true, message: t('shell.login.usernameRequired') }]}>
          <Input autoComplete="username" maxLength={LOGIN_LIMITS.username} />
        </Form.Item>
        <Form.Item name="password" label={t('shell.login.password')} rules={[{ required: true, message: t('shell.login.passwordRequired') }]}>
          <Input.Password autoComplete="current-password" maxLength={LOGIN_LIMITS.password} />
        </Form.Item>
        <Form.Item name="otp" label={t('shell.login.otp')} rules={[{ required: true, pattern: OTP_PATTERN, message: t('shell.login.otpFormat') }]}>
          <Input inputMode="numeric" autoComplete="one-time-code" maxLength={LOGIN_LIMITS.otp} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
