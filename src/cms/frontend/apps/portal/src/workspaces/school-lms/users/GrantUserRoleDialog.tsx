// Grant a UserRole (F052 T035, F052-AC-16): only roles the granter may hand out (grantableRoles), scope Đơn vị/Trường/Môn
// (no Lớp; a School Admin gets Trường/Môn only), optional validity, reason required; Tenant Admin → grantTenantUserRole,
// School Admin → grantSchoolUserRole (Idempotency-Key per intent). 403 PERMISSION_DENIED / 422 ROLE_NOT_FOUND shown.
import { App, DatePicker, Form, Input, Modal, Radio, Select } from 'antd';
import type { Dayjs } from 'dayjs';
import { useTranslation } from 'react-i18next';
import { applyFormError, writeErrorKey } from '../common/errors';
import { FIELDS } from '../common/fieldSchemas';
import { localizedName } from '../common/format';
import { useFieldRules } from '../common/ui';
import { ScopeTargetField } from './ScopeTargetField';
import { scopesFor, type GrantMode, type GrantScope, type Role } from './canGrant';
import { useGrantUserRole } from './useUserRoles';

interface GrantForm {
  role_id: string;
  scope_type: GrantScope;
  scope_id?: string;
  valid?: [Dayjs | null, Dayjs | null];
  reason: string;
}

export function GrantUserRoleDialog({ userId, roles, mode, onClose }: { userId: string; roles: Role[]; mode: GrantMode; onClose: () => void }) {
  const { t, i18n } = useTranslation();
  const { message } = App.useApp();
  const [form] = Form.useForm<GrantForm>();
  const grant = useGrantUserRole(mode);
  const field = useFieldRules();
  const scopeType = Form.useWatch('scope_type', form);
  const ok = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    const [from, to] = v.valid ?? [null, null];
    const schoolId = mode.kind === 'school' ? (v.scope_type === 'school' ? v.scope_id : mode.schoolIds[0]) : undefined;
    try {
      await grant.mutateAsync({
        schoolId,
        body: {
          user_id: userId,
          role_id: v.role_id,
          scope_type: v.scope_type,
          scope_id: v.scope_type === 'tenant' ? null : (v.scope_id ?? null),
          ...(from ? { valid_from: from.toISOString() } : {}),
          ...(to ? { valid_to: to.toISOString() } : {}),
          reason: v.reason.trim(),
        },
      });
      message.success(t('portal.school_lms.userRoles.granted'));
      onClose();
    } catch (e) {
      const shown = applyFormError(form, e, ['role_id', 'scope_type', 'scope_id', 'valid_from', 'valid_to', 'reason'], t, i18n.exists.bind(i18n), { ROLE_NOT_FOUND: 'role_id' });
      if (!shown) message.error(t(writeErrorKey(e)));
    }
  };
  return (
    <Modal open title={t('portal.school_lms.userRoles.grant')} okText={t('portal.school_lms.userRoles.grant')} cancelText={t('portal.school_lms.common.cancel')} confirmLoading={grant.isPending} onOk={ok} onCancel={onClose} destroyOnHidden>
      <Form<GrantForm> form={form} layout="vertical" preserve={false} initialValues={{ scope_type: scopesFor(mode)[0] }} data-testid="grant-form">
        <Form.Item name="role_id" label={t('portal.school_lms.userRoles.role')} rules={[{ required: true, message: t('portal.school_lms.users.roleRequired') }]}>
          <Select aria-label={t('portal.school_lms.userRoles.role')} options={roles.map((r) => ({ value: r.role_id, label: localizedName(r.name, i18n.language) }))} />
        </Form.Item>
        <Form.Item name="scope_type" label={t('portal.school_lms.userRoles.scope')} rules={[{ required: true }]}>
          <Radio.Group options={scopesFor(mode).map((s) => ({ value: s, label: t(`portal.school_lms.userRoles.scopes.${s}`) }))} />
        </Form.Item>
        <ScopeTargetField scopeType={scopeType} mode={mode} />
        <Form.Item name="valid" label={t('portal.school_lms.userRoles.validity')}>
          <DatePicker.RangePicker allowEmpty={[true, true]} />
        </Form.Item>
        <Form.Item name="reason" label={t('portal.school_lms.common.reason')} rules={field.rules(FIELDS.grantReason, true)}>
          <Input.TextArea autoSize={{ minRows: 2, maxRows: 5 }} {...field.input(FIELDS.grantReason)} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
