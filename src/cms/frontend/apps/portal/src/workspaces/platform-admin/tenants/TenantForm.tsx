// Tenant form (F057 T039, F057-AC-17, F057-AC-21): create (`TenantCreate`: code 2–64, name ≤ 255, type, Keycloak
// alias pattern, default locale, optional reason) or edit the name (`TenantUpdate`, reason 3–500). Limits come from
// the OpenAPI schema with a character counter; `409 CONFLICT` is shown on the code field (identifier_uniqueness).
import { Alert, App as AntApp, Form, Modal } from 'antd';
import { useTranslation } from 'react-i18next';
import { ApiError } from '../../../shared/api/middleware/problem';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { TENANT_TYPES, useCreateTenant, useUpdateTenant, type TenantCreate, type TenantSummary } from '../api/tenants';
import { LIMITS } from '../limits';
import { applyFieldErrors, LimitedField, ReasonField } from '../ui';
import { SearchSelect } from '@eduworld/ops-ui';

const LOCALES = ['vi-VN', 'en-US'] as const;

export function TenantForm({ tenant, onClose }: { tenant: TenantSummary | null; onClose: () => void }) {
  const { t, i18n } = useTranslation();
  const { message } = AntApp.useApp();
  const [form] = Form.useForm();
  const create = useCreateTenant();
  const update = useUpdateTenant();
  const mutation = tenant ? update : create;
  const conflict = mutation.error instanceof ApiError && mutation.error.code === 'CONFLICT';

  const submit = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    const onError = (e: unknown) => applyFieldErrors(form, e, ['code', 'name', 'keycloak_org_alias', 'reason'], t, (k) => i18n.exists(k));
    if (tenant) {
      update.mutate(
        { tenantId: tenant.tenant_id, version: tenant.version, patch: { name: v.name.trim(), reason: v.reason.trim() } },
        { onSuccess: () => (message.success(t('platform.tenants.updated')), onClose()), onError },
      );
    } else {
      const body: TenantCreate = { code: v.code.trim(), name: v.name.trim(), tenant_type: v.tenant_type, keycloak_org_alias: v.keycloak_org_alias.trim(), default_locale: v.default_locale };
      if (v.reason?.trim()) body.reason = v.reason.trim();
      create.mutate(body, { onSuccess: () => (message.success(t('platform.tenants.created')), onClose()), onError });
    }
  };

  return (
    <Modal
      open
      title={t(tenant ? 'platform.tenants.editTitle' : 'platform.tenants.createTitle')}
      okText={t('platform.common.save')}
      cancelText={t('platform.common.cancel')}
      confirmLoading={mutation.isPending}
      onOk={submit}
      onCancel={onClose}
      destroyOnHidden
    >
      <Form form={form} name="tenant" layout="vertical" initialValues={tenant ? { name: tenant.name } : { tenant_type: 'school', default_locale: 'vi-VN' }}>
        {tenant ? null : <LimitedField name="code" label={t('platform.tenants.code')} limit={LIMITS.tenantCode} required />}
        <LimitedField name="name" label={t('platform.tenants.name')} limit={LIMITS.tenantName} required />
        {tenant ? null : (
          <>
            <Form.Item name="tenant_type" label={t('platform.tenants.type')} rules={[{ required: true }]}>
              <SearchSelect options={TENANT_TYPES.map((x) => ({ value: x, label: t(`shell.tenantType.${x}`) }))} />
            </Form.Item>
            <LimitedField name="keycloak_org_alias" label={t('platform.tenants.alias')} help={t('platform.tenants.aliasHelp')} limit={LIMITS.tenantAlias} required />
            <Form.Item name="default_locale" label={t('platform.tenants.locale')}>
              <SearchSelect options={LOCALES.map((x) => ({ value: x, label: t(`shell.locale.${x}`) }))} />
            </Form.Item>
          </>
        )}
        <ReasonField limit={tenant ? LIMITS.tenantUpdateReason : LIMITS.tenantCreateReason} required={!!tenant} />
      </Form>
      {mutation.error ? <ProblemMessage error={mutation.error} /> : null}
      {conflict ? <Alert type="warning" showIcon message={t('platform.tenants.duplicate')} /> : null}
    </Modal>
  );
}
