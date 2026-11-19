// Create a staff account (F059 T034, F059-AC-12, FR-AUTH-031, ADR-0069, ADR-0081): "Nhân sự trường" = createTenantUser
// in the target tenant (X-Tenant-Id, Idempotency-Key, email mandatory, one initial `tenant` role at scope
// tenant/school/subject — no `class`, ADR-0072). "Nhân sự platform" needs `createPlatformUser` (not in the contract yet,
// Câu hỏi mở F059-Q1): the choice is shown disabled. `409 USERNAME_TAKEN` / `EMAIL_TAKEN` shows the hint "gán role cho
// tài khoản sẵn có" (identifier_uniqueness).
import { Alert, App as AntApp, Form, Input, Modal, Radio, Select } from 'antd';
import { useTranslation } from 'react-i18next';
import { ApiError } from '../../../shared/api/middleware/problem';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { TenantTargetField } from '../../../shell/TenantControls';
import { useRoleOptions } from '../api/roles';
import { useTenantSchools } from '../api/userRoles';
import { PLATFORM_USERS_AVAILABLE, useCreateTenantUser, type TenantUserCreateRequest } from '../api/users';
import { isUuid, LIMITS } from '../limits';
import { applyFieldErrors, LimitedField } from '../ui';
import { grantableRoles } from '../user-roles/grantableRoles';

const LOCALES = ['vi-VN', 'en-US'] as const;
const TAKEN = ['USERNAME_TAKEN', 'EMAIL_TAKEN'];
/** Initial scopes offered: tenant/school/subject — never `class` (ADR-0072) even though the schema still lists it (Câu hỏi mở F059-Q4). */
type InitialScope = 'tenant' | 'school' | 'subject';
const INITIAL_SCOPES: readonly InitialScope[] = ['tenant', 'school', 'subject'];

interface FormValues {
  kind: 'platform' | 'tenant';
  tenant_target: string;
  username: string;
  email: string;
  display_name: string;
  locale: (typeof LOCALES)[number];
  role_id: string;
  scope_type: InitialScope;
  scope_id?: string;
}

export function CreateUserDialog({ onClose }: { onClose: () => void }) {
  const { t, i18n } = useTranslation();
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<FormValues>();
  const target: string | undefined = Form.useWatch('tenant_target', form);
  const scopeType: InitialScope | undefined = Form.useWatch('scope_type', form);
  const roles = useRoleOptions('tenant', target, !!target);
  const schools = useTenantSchools(scopeType === 'school' ? target : undefined);
  const create = useCreateTenantUser();
  const taken = create.error instanceof ApiError && TAKEN.includes(create.error.code);

  const submit = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    const body: TenantUserCreateRequest = {
      username: v.username.trim(),
      email: v.email.trim(),
      display_name: v.display_name.trim(),
      user_type: 'staff',
      locale: v.locale,
      initial_user_roles: [{ role_id: v.role_id, scope_type: v.scope_type, ...(v.scope_id ? { scope_id: v.scope_id.trim() } : {}) }],
    };
    create.mutate(
      { tenantId: v.tenant_target, body },
      {
        onSuccess: () => {
          message.success(t('platform.users.created'));
          onClose();
        },
        onError: (e) => applyFieldErrors(form, e, ['username', 'email', 'display_name', 'locale'], t, (k) => i18n.exists(k)),
      },
    );
  };

  return (
    <Modal open title={t('platform.users.createTitle')} width={640} okText={t('platform.common.save')} cancelText={t('platform.common.cancel')} confirmLoading={create.isPending} onOk={submit} onCancel={onClose} destroyOnHidden>
      <Form
        form={form}
        name="create_user"
        layout="vertical"
        initialValues={{ kind: 'tenant', locale: 'vi-VN', scope_type: 'tenant' }}
        onValuesChange={(changed: Partial<FormValues>) => {
          if ('tenant_target' in changed) form.setFieldsValue({ role_id: undefined, scope_id: undefined });
        }}
      >
        <Form.Item name="kind" label={t('platform.users.accountKind')} extra={PLATFORM_USERS_AVAILABLE ? undefined : t('platform.users.platformCreateUnavailable')}>
          <Radio.Group
            options={[
              { value: 'tenant', label: t('platform.userRoles.kind.tenant') },
              { value: 'platform', label: t('platform.userRoles.kind.platform'), disabled: !PLATFORM_USERS_AVAILABLE },
            ]}
          />
        </Form.Item>
        <TenantTargetField />
        <LimitedField name="username" label={t('platform.users.username')} limit={LIMITS.username} required />
        <LimitedField name="email" label={t('platform.users.email')} limit={LIMITS.email} required extraRules={[{ type: 'email', message: t('platform.users.emailInvalid') }]} />
        <LimitedField name="display_name" label={t('platform.users.displayName')} limit={LIMITS.displayName} required />
        <Form.Item name="locale" label={t('platform.users.locale')}>
          <Select options={LOCALES.map((x) => ({ value: x, label: t(`shell.locale.${x}`) }))} />
        </Form.Item>
        <Form.Item name="role_id" label={t('platform.users.initialRole')} rules={[{ required: true, message: t('platform.userRoles.roleRequired') }]}>
          <Select
            showSearch
            optionFilterProp="label"
            disabled={!target}
            loading={roles.isFetching}
            options={grantableRoles(roles.data ?? [], 'tenant', target).map((r) => ({ value: r.role_id, label: `${r.code} — ${r.name.vi}` }))}
          />
        </Form.Item>
        <Form.Item name="scope_type" label={t('platform.userRoles.scope')} rules={[{ required: true }]}>
          <Select options={INITIAL_SCOPES.map((x) => ({ value: x, label: t(`platform.scopeType.${x}`) }))} />
        </Form.Item>
        {scopeType === 'school' ? (
          <Form.Item name="scope_id" label={t('platform.scopeType.school')} rules={[{ required: true, message: t('platform.userRoles.scopeIdRequired') }]}>
            <Select loading={schools.isFetching} options={(schools.data ?? []).map((s) => ({ value: s.school_id, label: `${s.name.vi} (${s.code})` }))} />
          </Form.Item>
        ) : null}
        {scopeType === 'subject' ? (
          <Form.Item
            name="scope_id"
            label={t('platform.userRoles.subjectId')}
            rules={[{ required: true, whitespace: true, message: t('platform.userRoles.scopeIdRequired') }, { validator: (_, v?: string) => (!v || isUuid(v.trim()) ? Promise.resolve() : Promise.reject(new Error(t('platform.common.uuidInvalid')))) }]}
          >
            <Input maxLength={36} />
          </Form.Item>
        ) : null}
      </Form>
      {create.error ? <ProblemMessage error={create.error} /> : null}
      {taken ? <Alert type="warning" showIcon message={t('platform.users.takenHint')} /> : null}
    </Modal>
  );
}
