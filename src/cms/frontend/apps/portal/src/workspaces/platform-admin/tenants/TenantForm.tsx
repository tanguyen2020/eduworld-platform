// Tenant form (F057 T039, F057-AC-17, F057-AC-21): create (`TenantCreate`: code 2–64, name ≤ 255, type, Keycloak
// alias pattern, default locale, optional reason) or edit the name (`TenantUpdate`, reason 3–500). Limits come from
// the OpenAPI schema with a character counter; `409 CONFLICT` is shown on the code field (identifier_uniqueness).
// F134 T026 (F134-AC-11): on edit, the "Tenant QA" switch (System Admin only — `platform.tenant.manage`; disabled for
// the `common` tenant, the server answers 422 anyway) asks for a confirmation that states the consequences (excluded
// from Game Analytics and the platform learning summary, no backfill when unflagged) and is sent as `is_qa` of the same
// updateTenant merge patch (If-Match, Idempotency-Key, reason) only when it changed. Nothing is computed here.
import { Alert, App as AntApp, Form, Modal, Switch, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { usePermissions } from '../../../permissions/usePermissions';
import { ApiError } from '../../../shared/api/middleware/problem';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { TENANT_TYPES, useCreateTenant, useUpdateTenant, type TenantCreate, type TenantSummary, type TenantUpdate } from '../api/tenants';
import { LIMITS } from '../limits';
import { applyFieldErrors, LimitedField, ReasonField } from '../ui';
import { SearchSelect } from '@eduworld/ops-ui';

const LOCALES = ['vi-VN', 'en-US'] as const;

/** Permission of the tenant writes; only the System Admin holds it (ADMIN_PORTAL §2.1, AGENTS R20). */
export const TENANT_MANAGE_PERMISSION = 'platform.tenant.manage';

/** Switch of the QA flag: every toggle is confirmed first (F134-AC-11); `onChange` only runs on "Xác nhận". */
function QaSwitch({ checked, onChange, disabled, tenantName }: { checked?: boolean; onChange?: (v: boolean) => void; disabled?: boolean; tenantName: string }) {
  const { t } = useTranslation();
  const { modal } = AntApp.useApp();
  const toggle = (next: boolean) => {
    modal.confirm({
      title: t(next ? 'platform.tenants.qa.confirmOnTitle' : 'platform.tenants.qa.confirmOffTitle', { name: tenantName }),
      content: <div data-testid="tenant-qa-confirm">{t(next ? 'platform.tenants.qa.confirmOnContent' : 'platform.tenants.qa.confirmOffContent')}</div>,
      okText: t('platform.tenants.qa.confirm'),
      cancelText: t('platform.tenants.qa.cancel'),
      onOk: () => onChange?.(next),
    });
  };
  return <Switch aria-label={t('platform.tenants.qa.label')} checked={!!checked} disabled={disabled} onChange={toggle} />;
}

export function TenantForm({ tenant, onClose }: { tenant: TenantSummary | null; onClose: () => void }) {
  const { t, i18n } = useTranslation();
  const { message } = AntApp.useApp();
  const { view } = usePermissions();
  const [form] = Form.useForm();
  const create = useCreateTenant();
  const update = useUpdateTenant();
  const mutation = tenant ? update : create;
  const conflict = mutation.error instanceof ApiError && mutation.error.code === 'CONFLICT';
  // display only: the backend checks platform.tenant.manage and rejects `common` (F134-AC-07)
  const showQa = !!tenant && !!view?.permissions.includes(TENANT_MANAGE_PERMISSION);
  const qaLocked = tenant?.tenant_type === 'common';

  const submit = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    const onError = (e: unknown) => applyFieldErrors(form, e, ['code', 'name', 'keycloak_org_alias', 'reason', 'is_qa'], t, (k) => i18n.exists(k));
    if (tenant) {
      const patch: TenantUpdate = { name: v.name.trim(), reason: v.reason.trim() };
      if (showQa && !qaLocked && typeof v.is_qa === 'boolean' && v.is_qa !== (tenant.is_qa ?? false)) patch.is_qa = v.is_qa;
      update.mutate(
        { tenantId: tenant.tenant_id, version: tenant.version, patch },
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
      <Form form={form} name="tenant" layout="vertical" initialValues={tenant ? { name: tenant.name, is_qa: tenant.is_qa ?? false } : { tenant_type: 'school', default_locale: 'vi-VN' }}>
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
        {showQa && tenant ? (
          <Form.Item
            name="is_qa"
            label={t('platform.tenants.qa.label')}
            valuePropName="checked"
            extra={
              <Typography.Text type="secondary" data-testid="tenant-qa-help">
                {qaLocked ? t('platform.tenants.qa.commonDisabled') : `${t('platform.tenants.qa.help')} ${t('platform.tenants.qa.saveHint')}`}
              </Typography.Text>
            }
          >
            <QaSwitch disabled={qaLocked} tenantName={tenant.name} />
          </Form.Item>
        ) : null}
        <ReasonField limit={tenant ? LIMITS.tenantUpdateReason : LIMITS.tenantCreateReason} required={!!tenant} />
      </Form>
      {mutation.error ? <ProblemMessage error={mutation.error} /> : null}
      {conflict ? <Alert type="warning" showIcon message={t('platform.tenants.duplicate')} /> : null}
    </Modal>
  );
}
