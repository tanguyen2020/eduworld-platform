// Create a staff account (F059 T034, F059-AC-12, FR-AUTH-031, ADR-0069, ADR-0081):
// - "Nhân sự platform" = createPlatformUser (OQ F059-1): no home tenant, no X-Tenant-Id, ≥ 1 initial `platform` role
//   (`safety_moderator` is granted later with its training date, ADR-0072), mandatory reason, Idempotency-Key;
// - "Nhân sự trường" = createTenantUser in the target tenant (X-Tenant-Id, Idempotency-Key, one initial `tenant` role at
//   scope tenant/school/subject — no `class`, ADR-0072).
// Email is mandatory for both. `409 USERNAME_TAKEN` / `EMAIL_TAKEN` shows the hint "gán role cho tài khoản sẵn có"
// (identifier_uniqueness). `kinds` = the kinds the caller's actions allow (display only; the backend authorizes).
import { Alert, App as AntApp, Form, Input, Modal, Radio } from 'antd';
import { useTranslation } from 'react-i18next';
import { ApiError } from '../../../shared/api/middleware/problem';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { TenantTargetField } from '../../../shell/TenantControls';
import { useRoleOptions } from '../api/roles';
import { useTenantSchools } from '../api/userRoles';
import { useCreatePlatformUser, useCreateTenantUser, type PlatformUserCreateRequest, type TenantUserCreateRequest } from '../api/users';
import { isUuid, LIMITS } from '../limits';
import { applyFieldErrors, LimitedField, ReasonField } from '../ui';
import { grantableRoles, TRAINING_REQUIRED_ROLE } from '../user-roles/grantableRoles';
import { SearchSelect } from '@eduworld/ops-ui';

const LOCALES = ['vi-VN', 'en-US'] as const;
const TAKEN = ['USERNAME_TAKEN', 'EMAIL_TAKEN'];
/** Initial scopes offered: tenant/school/subject — never `class` (ADR-0072). */
type InitialScope = 'tenant' | 'school' | 'subject';
const INITIAL_SCOPES: readonly InitialScope[] = ['tenant', 'school', 'subject'];
const FIELDS = ['username', 'email', 'display_name', 'locale', 'initial_role_ids', 'reason'];

export type CreateKind = 'platform' | 'tenant';

interface FormValues {
  kind: CreateKind;
  tenant_target: string;
  username: string;
  email: string;
  display_name: string;
  locale: (typeof LOCALES)[number];
  role_id: string;
  initial_role_ids: string[];
  scope_type: InitialScope;
  scope_id?: string;
  reason: string;
}

export function CreateUserDialog({ kinds, onClose }: { kinds: readonly CreateKind[]; onClose: () => void }) {
  const { t, i18n } = useTranslation();
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<FormValues>();
  const initialKind: CreateKind = kinds.includes('tenant') ? 'tenant' : 'platform';
  const kind: CreateKind = Form.useWatch('kind', form) ?? initialKind;
  const target: string | undefined = Form.useWatch('tenant_target', form);
  const scopeType: InitialScope | undefined = Form.useWatch('scope_type', form);
  const tenantRoles = useRoleOptions('tenant', target, kind === 'tenant' && !!target);
  const platformRoles = useRoleOptions('platform', undefined, kind === 'platform');
  const schools = useTenantSchools(kind === 'tenant' && scopeType === 'school' ? target : undefined);
  const createTenant = useCreateTenantUser();
  const createPlatform = useCreatePlatformUser();
  const mutation = kind === 'platform' ? createPlatform : createTenant;
  const taken = mutation.error instanceof ApiError && TAKEN.includes(mutation.error.code);

  const submit = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    const done = {
      onSuccess: () => {
        message.success(t('platform.users.created'));
        onClose();
      },
      onError: (e: unknown) => applyFieldErrors(form, e, FIELDS, t, (k) => i18n.exists(k)),
    };
    const common = { username: v.username.trim(), email: v.email.trim(), display_name: v.display_name.trim(), locale: v.locale };
    if (v.kind === 'platform') {
      const body: PlatformUserCreateRequest = { ...common, initial_role_ids: v.initial_role_ids, reason: v.reason.trim() };
      createPlatform.mutate(body, done);
      return;
    }
    const body: TenantUserCreateRequest = {
      ...common,
      user_type: 'staff',
      initial_user_roles: [{ role_id: v.role_id, scope_type: v.scope_type, ...(v.scope_id ? { scope_id: v.scope_id.trim() } : {}) }],
    };
    createTenant.mutate({ tenantId: v.tenant_target, body }, done);
  };

  return (
    <Modal open title={t('platform.users.createTitle')} width={640} okText={t('platform.common.save')} cancelText={t('platform.common.cancel')} confirmLoading={mutation.isPending} onOk={submit} onCancel={onClose} destroyOnHidden>
      <Form
        form={form}
        name="create_user"
        layout="vertical"
        initialValues={{ kind: initialKind, locale: 'vi-VN', scope_type: 'tenant' }}
        onValuesChange={(changed: Partial<FormValues>) => {
          if ('tenant_target' in changed) form.setFieldsValue({ role_id: undefined, scope_id: undefined });
          if ('kind' in changed) {
            createTenant.reset();
            createPlatform.reset();
          }
        }}
      >
        <Form.Item name="kind" label={t('platform.users.accountKind')}>
          <Radio.Group
            options={[
              { value: 'tenant', label: t('platform.userRoles.kind.tenant'), disabled: !kinds.includes('tenant') },
              { value: 'platform', label: t('platform.userRoles.kind.platform'), disabled: !kinds.includes('platform') },
            ]}
          />
        </Form.Item>
        {kind === 'tenant' ? <TenantTargetField /> : null}
        <LimitedField name="username" label={t('platform.users.username')} limit={kind === 'platform' ? LIMITS.platformUsername : LIMITS.username} required />
        <LimitedField
          name="email"
          label={t('platform.users.email')}
          limit={kind === 'platform' ? LIMITS.platformEmail : LIMITS.email}
          required
          extraRules={[{ type: 'email', message: t('platform.users.emailInvalid') }]}
        />
        <LimitedField name="display_name" label={t('platform.users.displayName')} limit={kind === 'platform' ? LIMITS.platformDisplayName : LIMITS.displayName} required />
        <Form.Item name="locale" label={t('platform.users.locale')}>
          <SearchSelect options={LOCALES.map((x) => ({ value: x, label: t(`shell.locale.${x}`) }))} />
        </Form.Item>
        {kind === 'platform' ? (
          <>
            <Form.Item
              name="initial_role_ids"
              label={t('platform.users.initialPlatformRoles')}
              extra={t('platform.users.safetyLater')}
              rules={[{ required: true, type: 'array', min: 1, message: t('platform.userRoles.roleRequired') }]}
            >
              <SearchSelect
                mode="multiple"
                maxCount={LIMITS.platformInitialRoles.maxItems}
                optionFilterProp="label"
                loading={platformRoles.isFetching}
                options={grantableRoles(platformRoles.data ?? [], 'platform')
                  .filter((r) => r.code !== TRAINING_REQUIRED_ROLE)
                  .map((r) => ({ value: r.role_id, label: `${r.code} — ${r.name.vi}` }))}
              />
            </Form.Item>
            <ReasonField limit={LIMITS.platformUserReason} />
          </>
        ) : (
          <>
            <Form.Item name="role_id" label={t('platform.users.initialRole')} rules={[{ required: true, message: t('platform.userRoles.roleRequired') }]}>
              <SearchSelect
                showSearch
                optionFilterProp="label"
                disabled={!target}
                loading={tenantRoles.isFetching}
                options={grantableRoles(tenantRoles.data ?? [], 'tenant', target).map((r) => ({ value: r.role_id, label: `${r.code} — ${r.name.vi}` }))}
              />
            </Form.Item>
            <Form.Item name="scope_type" label={t('platform.userRoles.scope')} rules={[{ required: true }]}>
              <SearchSelect options={INITIAL_SCOPES.map((x) => ({ value: x, label: t(`platform.scopeType.${x}`) }))} />
            </Form.Item>
            {scopeType === 'school' ? (
              <Form.Item name="scope_id" label={t('platform.scopeType.school')} rules={[{ required: true, message: t('platform.userRoles.scopeIdRequired') }]}>
                <SearchSelect loading={schools.isFetching} options={(schools.data ?? []).map((s) => ({ value: s.school_id, label: `${s.name.vi} (${s.code})` }))} />
              </Form.Item>
            ) : null}
            {scopeType === 'subject' ? (
              <Form.Item
                name="scope_id"
                label={t('platform.userRoles.subjectId')}
                rules={[
                  { required: true, whitespace: true, message: t('platform.userRoles.scopeIdRequired') },
                  { validator: (_, v?: string) => (!v || isUuid(v.trim()) ? Promise.resolve() : Promise.reject(new Error(t('platform.common.uuidInvalid')))) },
                ]}
              >
                <Input maxLength={36} />
              </Form.Item>
            ) : null}
          </>
        )}
      </Form>
      {mutation.error ? <ProblemMessage error={mutation.error} /> : null}
      {taken ? <Alert type="warning" showIcon message={t('platform.users.takenHint')} /> : null}
    </Modal>
  );
}
