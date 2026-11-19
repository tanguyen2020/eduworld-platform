// Role editor (F052 T034, F052-AC-14, F052-AC-15): a system role (`is_system`) is read-only with "Nhân bản thành role
// tùy biến" instead of Sửa/Xóa; a custom role is saved with a reason and If-Match (ETag of getTenantRole); deleting a
// role still held by users answers 409 → hint to revoke the UserRole first. Create = createTenantRole (Idempotency-Key).
import { Alert, App, Button, Drawer, Form, Input, Space, Spin, Tag } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ApiError } from '../../../shared/api/middleware/problem';
import { applyFormError, QueryError, writeErrorKey } from '../common/errors';
import { FIELDS } from '../common/fieldSchemas';
import { ReasonModal } from '../common/ReasonModal';
import { useFieldRules } from '../common/ui';
import { PermissionPicker, refusedPermissions } from './PermissionPicker';
import { useCreateRole, useDeleteRole, usePermissionCatalog, useRole, useUpdateRole, type Role, type RoleInput } from './useRoles';

interface RoleForm {
  code: string;
  name_vi: string;
  name_en?: string;
  permission_codes: string[];
}

/** Draft of a custom role copied from a system role (F052-AC-15). */
export function cloneDraft(role: Pick<Role, 'code' | 'name' | 'permission_codes'>): RoleForm {
  return { code: `${role.code}_custom`.slice(0, FIELDS.roleCode.maxLength), name_vi: role.name.vi, name_en: role.name.en, permission_codes: [...role.permission_codes] };
}

function toInput(v: RoleForm, reason: string): RoleInput {
  return { code: v.code.trim(), name: { vi: v.name_vi.trim(), ...(v.name_en?.trim() ? { en: v.name_en.trim() } : {}) }, role_scope_kind: 'tenant', permission_codes: v.permission_codes, reason };
}

export type RoleEditorTarget = { kind: 'existing'; roleId: string } | { kind: 'new'; draft?: RoleForm };

export function RoleEditor({ target, onClose, onClone }: { target: RoleEditorTarget; onClose: () => void; onClone: (draft: RoleForm) => void }) {
  const { t, i18n } = useTranslation();
  const { message } = App.useApp();
  const role = useRole(target.kind === 'existing' ? target.roleId : undefined);
  const perms = usePermissionCatalog(i18n.language);
  const create = useCreateRole();
  const update = useUpdateRole(target.kind === 'existing' ? target.roleId : '');
  const remove = useDeleteRole(target.kind === 'existing' ? target.roleId : '');
  const field = useFieldRules();
  const [form] = Form.useForm<RoleForm>();
  const [pending, setPending] = useState<'save' | 'delete' | null>(null);
  const [invalid, setInvalid] = useState<Set<string>>(new Set());
  const [conflict, setConflict] = useState(false);
  const r = role.data;
  const readOnly = !!r?.is_system;
  const initial: RoleForm | undefined = target.kind === 'new' ? (target.draft ?? { code: '', name_vi: '', permission_codes: [] }) : r ? { code: r.code, name_vi: r.name.vi, name_en: r.name.en, permission_codes: r.permission_codes } : undefined;
  const codes = Form.useWatch('permission_codes', form) ?? initial?.permission_codes ?? [];

  const save = async (reason: string) => {
    const v = form.getFieldsValue(true) as RoleForm;
    setInvalid(new Set());
    try {
      if (target.kind === 'new') await create.mutateAsync(toInput(v, reason));
      else await update.mutateAsync(toInput(v, reason));
      message.success(t('portal.school_lms.common.saved'));
      onClose();
    } catch (e) {
      if (e instanceof ApiError) setInvalid(refusedPermissions(e.fieldErrors, v.permission_codes));
      if (!applyFormError(form, e, ['code', 'name', 'name_vi', 'name_en'], t, i18n.exists.bind(i18n))) message.error(t(writeErrorKey(e)));
    } finally {
      setPending(null);
    }
  };
  const del = async (reason: string) => {
    setConflict(false);
    try {
      await remove.mutateAsync(reason);
      message.success(t('portal.school_lms.roles.deleted'));
      onClose();
    } catch (e) {
      if (e instanceof ApiError && e.code === 'CONFLICT') setConflict(true);
      else message.error(t(writeErrorKey(e)));
    } finally {
      setPending(null);
    }
  };

  const title = target.kind === 'new' ? t('portal.school_lms.roles.create') : r ? `${r.name.vi} (${r.code})` : '';
  return (
    <Drawer open title={title} onClose={onClose} width="min(720px, 100vw)" data-testid="role-editor">
      {role.error ? <QueryError error={role.error} resource /> : null}
      {!initial ? (
        <Spin aria-label={t('app.loading')} />
      ) : (
        <Space direction="vertical" size="middle" style={{ width: '100%' }}>
          {readOnly ? (
            <Space wrap>
              <Tag>{t('portal.school_lms.roles.system')}</Tag>
              <ActionGate actionKey="action.school_lms.access_control.edit_roles">
                <Button onClick={() => r && onClone(cloneDraft(r))}>{t('portal.school_lms.roles.clone')}</Button>
              </ActionGate>
            </Space>
          ) : null}
          {r?.active_user_role_count ? <Alert type="info" showIcon message={t('portal.school_lms.roles.impact', { count: r.active_user_role_count })} /> : null}
          {conflict ? <Alert type="warning" showIcon role="alert" message={t('portal.school_lms.roles.deleteConflict')} /> : null}
          <Form<RoleForm> form={form} layout="vertical" initialValues={initial} disabled={readOnly}>
            <Form.Item name="code" label={t('portal.school_lms.roles.code')} rules={field.rules(FIELDS.roleCode, true)}>
              <Input disabled={readOnly || target.kind === 'existing'} {...field.input(FIELDS.roleCode)} />
            </Form.Item>
            <Form.Item name="name_vi" label={t('portal.school_lms.roles.nameVi')} rules={field.rules(FIELDS.schoolNameVi, true)}>
              <Input {...field.input(FIELDS.schoolNameVi)} />
            </Form.Item>
            <Form.Item name="name_en" label={t('portal.school_lms.roles.nameEn')} rules={field.rules(FIELDS.schoolNameEn)}>
              <Input {...field.input(FIELDS.schoolNameEn)} />
            </Form.Item>
            <Form.Item name="permission_codes" label={t('portal.school_lms.roles.permissions')} rules={[{ required: true, type: 'array', min: 1, message: t('portal.school_lms.roles.permissionsRequired') }]} valuePropName="value">
              <PermissionPicker permissions={perms.data ?? []} value={codes} invalid={invalid} disabled={readOnly} onChange={(c) => form.setFieldValue('permission_codes', c)} />
            </Form.Item>
          </Form>
          {readOnly ? null : (
            <ActionGate actionKey="action.school_lms.access_control.edit_roles">
              <Space>
                {target.kind === 'existing' ? (
                  <Button danger onClick={() => setPending('delete')}>
                    {t('portal.school_lms.common.delete')}
                  </Button>
                ) : null}
                <Button type="primary" onClick={async () => (await form.validateFields().catch(() => null)) && setPending('save')}>
                  {t('portal.school_lms.roles.saveWithReason')}
                </Button>
              </Space>
            </ActionGate>
          )}
        </Space>
      )}
      <ReasonModal
        open={pending === 'save'}
        title={t('portal.school_lms.roles.saveWithReason')}
        schema={{ ...FIELDS.roleReason, minLength: 1 }}
        confirmLoading={create.isPending || update.isPending}
        onCancel={() => setPending(null)}
        onSubmit={save}
      />
      <ReasonModal
        open={pending === 'delete'}
        title={t('portal.school_lms.roles.deleteTitle')}
        schema={FIELDS.revokeReason}
        danger
        okText={t('portal.school_lms.common.delete')}
        confirmLoading={remove.isPending}
        onCancel={() => setPending(null)}
        onSubmit={del}
      />
    </Drawer>
  );
}
