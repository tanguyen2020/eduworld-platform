// Role editor (F059 T020, F059-AC-03/04/05, ADR-0066): create a platform/shared role (createPlatformRole,
// Idempotency-Key), edit one (getPlatformRole → updatePlatformRole with If-Match + mandatory reason) or clone a system
// role into a custom one (create prefilled from the source). The permission picker offers only the permissions of the
// role's `scope_kind` and is reloaded when the editor opens (a new `catalog_version` refreshes it). Server answers are
// shown by `code` and never clear the form: `422 PERMISSION_NOT_ALLOWED_FOR_SCOPE` lists the rejected codes, `412`,
// `409`, `403` (system role) show their message; a role deleted meanwhile (`404`) closes the editor.
import { Alert, App as AntApp, Form, Modal, Radio, Spin } from 'antd';
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { PermissionPicker } from '../../../shared/authz/PermissionPicker';
import { ApiError } from '../../../shared/api/middleware/problem';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { rejectedPermissions, SCOPE_KINDS, useCreateRole, usePermissionCatalog, useRole, useUpdateRole, type PermissionScopeKind, type Role, type RoleInput } from '../api/roles';
import { LIMITS, PRINCIPAL_CACHE_TTL_SECONDS } from '../limits';
import { applyFieldErrors, LimitedField, ReasonField } from '../ui';

export type EditorTarget = { mode: 'create' } | { mode: 'edit'; roleId: string } | { mode: 'clone'; source: Role };

interface FormValues {
  code: string;
  name_vi: string;
  name_en?: string;
  description?: string;
  role_scope_kind: PermissionScopeKind;
  permission_codes: string[];
  reason?: string;
}

const cloneCode = (code: string) => `${code}_custom`.slice(0, LIMITS.roleCode.maxLength);

export function PlatformRoleEditor({ target, onClose }: { target: EditorTarget; onClose: () => void }) {
  const { t, i18n } = useTranslation();
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<FormValues>();
  const catalog = usePermissionCatalog();
  const existing = useRole(target.mode === 'edit' ? target.roleId : undefined);
  const create = useCreateRole();
  const update = useUpdateRole();
  const mutation = target.mode === 'edit' ? update : create;
  const scopeKind = Form.useWatch('role_scope_kind', form);
  const vars = mutation.variables as RoleInput | { body: RoleInput } | undefined;
  const sent = vars ? ('body' in vars ? vars.body : vars).permission_codes : [];
  const rejected = rejectedPermissions(mutation.error, sent);
  const { refetch } = catalog;

  useEffect(() => {
    void refetch(); // a newer catalog_version reloads the picker (spec edge case)
  }, [refetch]);

  useEffect(() => {
    if (target.mode === 'edit' && existing.error instanceof ApiError && existing.error.status === 404) {
      message.warning(t('platform.roles.gone'));
      onClose();
    }
  }, [existing.error, message, onClose, t, target.mode]);

  const role = target.mode === 'edit' ? existing.data : target.mode === 'clone' ? target.source : undefined;
  const initialValues: Partial<FormValues> = role
    ? {
        code: target.mode === 'clone' ? cloneCode(role.code) : role.code,
        name_vi: target.mode === 'clone' ? t('platform.roles.cloneName', { name: role.name.vi }).slice(0, LIMITS.roleNameVi.maxLength) : role.name.vi,
        name_en: role.name.en,
        description: role.description,
        role_scope_kind: role.role_scope_kind,
        permission_codes: role.permission_codes,
      }
    : { role_scope_kind: 'platform', permission_codes: [] };

  const submit = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    const body: RoleInput = {
      code: v.code.trim(),
      name: { vi: v.name_vi.trim(), ...(v.name_en?.trim() ? { en: v.name_en.trim() } : {}) },
      role_scope_kind: v.role_scope_kind,
      permission_codes: v.permission_codes,
      ...(v.description?.trim() ? { description: v.description.trim() } : {}),
      ...(v.reason?.trim() ? { reason: v.reason.trim() } : {}),
    };
    const onError = (e: unknown) => applyFieldErrors(form, e, ['code', 'name', 'description', 'reason'], t, (k) => i18n.exists(k));
    const done = (saved: Role) => {
      message.success(t('platform.roles.appliesWithin', { seconds: PRINCIPAL_CACHE_TTL_SECONDS, users: saved.active_user_role_count ?? 0 }));
      onClose();
    };
    if (target.mode === 'edit') update.mutate({ roleId: target.roleId, body }, { onSuccess: done, onError });
    else create.mutate(body, { onSuccess: done, onError });
  };

  const title = t(target.mode === 'edit' ? 'platform.roles.editTitle' : target.mode === 'clone' ? 'platform.roles.cloneTitle' : 'platform.roles.createTitle');
  const loading = target.mode === 'edit' && existing.isLoading;
  return (
    <Modal open title={title} width={720} okText={t('platform.common.save')} cancelText={t('platform.common.cancel')} confirmLoading={mutation.isPending} onOk={submit} onCancel={onClose} destroyOnHidden okButtonProps={{ disabled: loading || !!existing.error }}>
      {loading ? (
        <Spin />
      ) : (
        <Form form={form} name="platform_role" layout="vertical" initialValues={initialValues}>
          <LimitedField name="code" label={t('platform.roles.code')} limit={LIMITS.roleCode} required />
          <LimitedField name="name_vi" label={t('platform.roles.nameVi')} limit={LIMITS.roleNameVi} required />
          <LimitedField name="name_en" label={t('platform.roles.nameEn')} limit={LIMITS.roleNameEn} />
          <LimitedField name="description" label={t('platform.roles.description')} limit={LIMITS.roleDescription} textarea />
          <Form.Item name="role_scope_kind" label={t('platform.roles.scopeKind')} rules={[{ required: true }]}>
            <Radio.Group
              disabled={target.mode !== 'create'}
              onChange={() => form.setFieldValue('permission_codes', [])}
              options={SCOPE_KINDS.map((x) => ({ value: x, label: t(`platform.scopeKind.${x}`) }))}
            />
          </Form.Item>
          <Form.Item
            name="permission_codes"
            label={t('platform.roles.permissions')}
            extra={t('platform.roles.permissionsHelp', { max: LIMITS.permissionCodes.maxItems })}
            rules={[
              { required: true, type: 'array', min: 1, message: t('platform.roles.permissionsRequired') },
              { type: 'array', max: LIMITS.permissionCodes.maxItems, message: t('platform.common.tooMany', { max: LIMITS.permissionCodes.maxItems }) },
            ]}
          >
            <PermissionPicker catalog={catalog.data?.items ?? []} scopeKind={scopeKind ?? 'platform'} ariaLabel={t('platform.roles.permissions')} />
          </Form.Item>
          <ReasonField limit={LIMITS.roleReason} required={target.mode === 'edit'} />
        </Form>
      )}
      {existing.error && !(existing.error instanceof ApiError && existing.error.status === 404) ? <ProblemMessage error={existing.error} /> : null}
      {mutation.error ? <ProblemMessage error={mutation.error} /> : null}
      {rejected.length ? <Alert type="warning" showIcon message={t('platform.roles.scopeNotAllowed', { codes: rejected.join(', ') })} /> : null}
    </Modal>
  );
}
