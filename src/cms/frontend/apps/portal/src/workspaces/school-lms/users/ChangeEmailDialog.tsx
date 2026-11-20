// "Đổi email" of a staff account (F052 T063, OQ F012-1, F052-AC-13): updateTenantUser with `email` only and If-Match =
// `W/"<version>"` of the listed row. 409 EMAIL_TAKEN → error at the field; 412 VERSION_MISMATCH → the list reloads.
// The new email is marked unverified on the SSO (Keycloak emailVerified = false) and the username never changes
// (realm editUsernameAllowed = false): both are said in the dialog. Limits of OpenAPI (TenantUserUpdateRequest.email).
import { Alert, App, Form, Input, Modal } from 'antd';
import { useTranslation } from 'react-i18next';
import { ApiError } from '../../../shared/api/middleware/problem';
import { applyFormError, writeErrorKey } from '../common/errors';
import { FIELDS } from '../common/fieldSchemas';
import { useFieldRules } from '../common/ui';
import { useUpdateStaffEmail, type User } from './useUsers';

export function ChangeEmailDialog({ user, onClose }: { user: User; onClose: () => void }) {
  const { t, i18n } = useTranslation();
  const { message } = App.useApp();
  const [form] = Form.useForm<{ email: string }>();
  const update = useUpdateStaffEmail();
  const field = useFieldRules();
  const ok = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    try {
      await update.mutateAsync({ user, email: v.email.trim() });
      message.success(t('portal.school_lms.users.emailChanged'));
      onClose();
    } catch (e) {
      if (e instanceof ApiError && e.code === 'VERSION_MISMATCH') {
        message.warning(t('portal.school_lms.users.reloaded'));
        onClose();
        return;
      }
      if (!applyFormError(form, e, ['email'], t, i18n.exists.bind(i18n), { EMAIL_TAKEN: 'email' })) message.error(t(writeErrorKey(e)));
    }
  };
  return (
    <Modal
      open
      title={t('portal.school_lms.users.changeEmailTitle', { name: user.display_name ?? user.username })}
      okText={t('portal.school_lms.common.save')}
      cancelText={t('portal.school_lms.common.cancel')}
      confirmLoading={update.isPending}
      onOk={ok}
      onCancel={onClose}
      destroyOnHidden
    >
      <Form form={form} layout="vertical" preserve={false} initialValues={{ email: user.email ?? '' }} data-testid="change-email-form">
        <Form.Item name="email" label={t('portal.school_lms.users.email')} rules={[...field.rules(FIELDS.emailUpdate, true), { type: 'email', message: t('portal.school_lms.users.emailInvalid') }]}>
          <Input type="email" autoComplete="off" {...field.input(FIELDS.emailUpdate)} />
        </Form.Item>
      </Form>
      <Alert type="info" showIcon message={t('portal.school_lms.users.emailUnverifiedNote')} />
    </Modal>
  );
}
