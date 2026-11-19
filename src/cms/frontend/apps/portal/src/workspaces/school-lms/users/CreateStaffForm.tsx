// "Thêm nhân sự" (F052 T033, F052-AC-13, F052-AC-26, ADR-0069): Họ tên, Username (pattern/maxLength of OpenAPI), Email,
// one initial role + scope (Đơn vị/Trường/Môn, never Lớp), createTenantUser with an Idempotency-Key per intent.
// 409 USERNAME_TAKEN / EMAIL_TAKEN → error at the field + hint to grant a role to the existing account instead.
import { Alert, App, Form, Input, Modal, Radio, Select } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ApiError } from '../../../shared/api/middleware/problem';
import { applyFormError, writeErrorKey } from '../common/errors';
import { FIELDS } from '../common/fieldSchemas';
import { useFieldRules } from '../common/ui';
import { ScopeTargetField } from './ScopeTargetField';
import { scopesFor, type GrantMode, type GrantScope, type Role } from './canGrant';
import { useCreateStaff } from './useUsers';

interface StaffForm {
  display_name: string;
  username: string;
  email: string;
  role_id: string;
  scope_type: GrantScope;
  scope_id?: string;
}

const KNOWN = ['display_name', 'username', 'email', 'initial_user_roles', 'role_id', 'scope_type', 'scope_id'];

export function CreateStaffForm({ open, roles, mode, onClose }: { open: boolean; roles: Role[]; mode: GrantMode; onClose: () => void }) {
  const { t, i18n } = useTranslation();
  const { message } = App.useApp();
  const [form] = Form.useForm<StaffForm>();
  const create = useCreateStaff();
  const field = useFieldRules();
  const [taken, setTaken] = useState(false);
  const scopeType = Form.useWatch('scope_type', form);
  const ok = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    setTaken(false);
    try {
      await create.mutateAsync({
        display_name: v.display_name.trim(),
        username: v.username.trim(),
        email: v.email.trim(),
        user_type: 'staff',
        initial_user_roles: [{ role_id: v.role_id, scope_type: v.scope_type, ...(v.scope_type !== 'tenant' && v.scope_id ? { scope_id: v.scope_id } : {}) }],
      });
      message.success(t('portal.school_lms.users.created'));
      onClose();
    } catch (e) {
      const dup = e instanceof ApiError && (e.code === 'USERNAME_TAKEN' || e.code === 'EMAIL_TAKEN');
      setTaken(dup);
      const shown = applyFormError(form, e, KNOWN, t, i18n.exists.bind(i18n), { USERNAME_TAKEN: 'username', EMAIL_TAKEN: 'email', ROLE_NOT_FOUND: 'role_id' });
      if (!shown) message.error(t(writeErrorKey(e)));
    }
  };
  return (
    <Modal open={open} title={t('portal.school_lms.users.addStaff')} okText={t('portal.school_lms.common.save')} cancelText={t('portal.school_lms.common.cancel')} confirmLoading={create.isPending} onOk={ok} onCancel={onClose} destroyOnHidden>
      <Form<StaffForm> form={form} layout="vertical" preserve={false} initialValues={{ scope_type: scopesFor(mode)[0] }} data-testid="create-staff-form">
        <Form.Item name="display_name" label={t('portal.school_lms.users.displayName')} rules={field.rules(FIELDS.displayName, true)}>
          <Input {...field.input(FIELDS.displayName)} />
        </Form.Item>
        <Form.Item name="username" label={t('portal.school_lms.users.username')} extra={t('portal.school_lms.users.usernameHint')} rules={field.rules(FIELDS.username, true)}>
          <Input autoComplete="off" {...field.input(FIELDS.username)} />
        </Form.Item>
        <Form.Item name="email" label={t('portal.school_lms.users.email')} rules={[...field.rules(FIELDS.email, true), { type: 'email', message: t('portal.school_lms.users.emailInvalid') }]}>
          <Input type="email" autoComplete="off" {...field.input(FIELDS.email)} />
        </Form.Item>
        <Form.Item name="role_id" label={t('portal.school_lms.users.initialRole')} rules={[{ required: true, message: t('portal.school_lms.users.roleRequired') }]}>
          <Select options={roles.map((r) => ({ value: r.role_id, label: r.name.vi }))} />
        </Form.Item>
        <Form.Item name="scope_type" label={t('portal.school_lms.userRoles.scope')} rules={[{ required: true }]}>
          <Radio.Group options={scopesFor(mode).map((s) => ({ value: s, label: t(`portal.school_lms.userRoles.scopes.${s}`) }))} />
        </Form.Item>
        <ScopeTargetField scopeType={scopeType} mode={mode} />
      </Form>
      {taken ? <Alert type="info" showIcon message={t('portal.school_lms.users.takenHint')} /> : null}
    </Modal>
  );
}
