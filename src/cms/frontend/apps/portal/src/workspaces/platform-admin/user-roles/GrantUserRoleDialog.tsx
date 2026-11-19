// Grant a UserRole (F059 T028, F059-AC-07/08/10, ADR-0081, ADR-0072, AGENTS R20): platform staff (no home tenant) get
// only `platform` roles at scope `platform`, sent without X-Tenant-Id; staff with a home tenant get only `tenant`
// roles of the target tenant (`TenantTargetField`, or only the account's member tenants when the caller knows them from
// `User.tenant_ids` — OQ F059-5), scope `tenant`/`school`/`subject` with its id, sent with
// X-Tenant-Id = target. Idempotency-Key per intent (kept on retry), mandatory reason, optional validity
// (`valid_to` > `valid_from`), training date mandatory for `safety_moderator`. Server answers (422 incl.
// ROLE_NOT_FOUND, 409) are shown by `code` without clearing the form.
import { Alert, App as AntApp, DatePicker, Form, Input, Modal, Radio, Select } from 'antd';
import type { Dayjs } from 'dayjs';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { clampTenantSearch } from '../../../shared/api/hooks/useTenantOptions';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { tenantLabel, TenantTargetField } from '../../../shell/TenantControls';
import { useRoleOptions } from '../api/roles';
import { useTenantDirectory } from '../api/tenants';
import { useGrantUserRole, useTenantSchools, useTenantStaff, type UserRoleGrantRequest, type UserRoleScopeType } from '../api/userRoles';
import { isUuid, LIMITS, PRINCIPAL_CACHE_TTL_SECONDS } from '../limits';
import { applyFieldErrors, ReasonField } from '../ui';
import { grantableRoles, grantableScopes, scopeNeedsId, TRAINING_REQUIRED_ROLE, validRange, type AccountKind } from './grantableRoles';

interface FormValues {
  kind: AccountKind;
  user_id: string;
  tenant_target?: string;
  role_id: string;
  scope_type: UserRoleScopeType;
  scope_id?: string;
  valid_from?: Dayjs;
  valid_to?: Dayjs;
  training_completed_at?: Dayjs;
  reason: string;
}

export interface GrantPreset {
  /** Known kind of the account (fixed in the dialog); unknown → the System Admin chooses. */
  kind?: AccountKind;
  userId?: string;
  tenantId?: string;
  /** Member tenants of the account (`User.tenant_ids` or its home tenant): the only target tenants offered. */
  tenantIds?: readonly string[];
}

/** Target tenant among the account's member tenants (names from listTenants; unknown ids shown raw). */
function MemberTenantField({ tenantIds }: { tenantIds: readonly string[] }) {
  const { t } = useTranslation();
  const directory = useTenantDirectory();
  return (
    <Form.Item name="tenant_target" label={t('shell.tenantTarget.label')} extra={t('platform.userRoles.memberTenantsHelp')} rules={[{ required: true, message: t('shell.tenantTarget.required') }]}>
      <Select
        options={tenantIds.map((id) => {
          const x = directory.get(id);
          return { value: id, label: x ? tenantLabel(x, t) : id };
        })}
      />
    </Form.Item>
  );
}

export function GrantUserRoleDialog({ preset, onClose }: { preset?: GrantPreset; onClose: () => void }) {
  const { t, i18n } = useTranslation();
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<FormValues>();
  const kind: AccountKind = Form.useWatch('kind', form) ?? preset?.kind ?? 'platform';
  const target: string | undefined = Form.useWatch('tenant_target', form) ?? preset?.tenantId;
  const roleId: string | undefined = Form.useWatch('role_id', form);
  const scopeType: UserRoleScopeType | undefined = Form.useWatch('scope_type', form);
  const [staffSearch, setStaffSearch] = useState('');
  const roles = useRoleOptions(kind, kind === 'tenant' ? target : undefined, kind === 'platform' || !!target);
  const offered = grantableRoles(roles.data ?? [], kind, target);
  const staff = useTenantStaff(kind === 'tenant' ? target : undefined, staffSearch);
  const schools = useTenantSchools(kind === 'tenant' && scopeType === 'school' ? target : undefined);
  const grant = useGrantUserRole();
  const needsTraining = offered.find((r) => r.role_id === roleId)?.code === TRAINING_REQUIRED_ROLE;
  const scopes = grantableScopes(kind);

  const submit = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    const body: UserRoleGrantRequest = {
      user_id: v.user_id.trim(),
      role_id: v.role_id,
      scope_type: v.scope_type,
      ...(scopeNeedsId(v.scope_type) && v.scope_id ? { scope_id: v.scope_id.trim() } : {}),
      ...(v.valid_from ? { valid_from: v.valid_from.toISOString() } : {}),
      ...(v.valid_to ? { valid_to: v.valid_to.toISOString() } : {}),
      ...(needsTraining && v.training_completed_at ? { training_completed_at: v.training_completed_at.toISOString() } : {}),
      reason: v.reason.trim(),
    };
    grant.mutate(
      { body, tenantId: v.kind === 'tenant' ? v.tenant_target : undefined },
      {
        onSuccess: () => {
          message.success(t('platform.userRoles.granted', { seconds: PRINCIPAL_CACHE_TTL_SECONDS }));
          onClose();
        },
        onError: (e) => applyFieldErrors(form, e, ['user_id', 'role_id', 'scope_type', 'scope_id', 'valid_from', 'valid_to', 'training_completed_at', 'reason'], t, (k) => i18n.exists(k)),
      },
    );
  };

  const uuidRule = { validator: (_: unknown, v?: string) => (!v || isUuid(v.trim()) ? Promise.resolve() : Promise.reject(new Error(t('platform.common.uuidInvalid')))) };
  return (
    <Modal open title={t('platform.userRoles.grantTitle')} width={640} okText={t('platform.userRoles.grant')} cancelText={t('platform.common.cancel')} confirmLoading={grant.isPending} onOk={submit} onCancel={onClose} destroyOnHidden>
      <Form
        form={form}
        name="grant_user_role"
        layout="vertical"
        initialValues={{ kind: preset?.kind ?? 'platform', user_id: preset?.userId, tenant_target: preset?.tenantId, scope_type: (preset?.kind ?? 'platform') === 'platform' ? 'platform' : 'tenant' }}
        onValuesChange={(changed: Partial<FormValues>) => {
          if ('kind' in changed) form.setFieldsValue({ role_id: undefined, user_id: preset?.userId, scope_id: undefined, scope_type: changed.kind === 'platform' ? 'platform' : 'tenant' });
          if ('tenant_target' in changed) form.setFieldsValue({ role_id: undefined, user_id: preset?.userId, scope_id: undefined });
        }}
      >
        <Form.Item name="kind" label={t('platform.userRoles.accountKind')}>
          <Radio.Group disabled={!!preset?.kind} options={(['platform', 'tenant'] as const).map((x) => ({ value: x, label: t(`platform.userRoles.kind.${x}`) }))} />
        </Form.Item>
        {kind === 'tenant' ? preset?.tenantIds?.length ? <MemberTenantField tenantIds={preset.tenantIds} /> : <TenantTargetField /> : null}
        {kind === 'platform' ? (
          <Form.Item name="user_id" label={t('platform.userRoles.userId')} extra={t('platform.userRoles.platformUserHelp')} rules={[{ required: true, whitespace: true, message: t('platform.userRoles.userRequired') }, uuidRule]}>
            <Input maxLength={36} disabled={!!preset?.userId} />
          </Form.Item>
        ) : (
          <Form.Item name="user_id" label={t('platform.userRoles.staff')} rules={[{ required: true, message: t('platform.userRoles.userRequired') }]}>
            <Select
              showSearch
              disabled={!target || !!preset?.userId}
              filterOption={false}
              searchValue={staffSearch}
              onSearch={(v) => setStaffSearch(clampTenantSearch(v).slice(0, LIMITS.tenantUserSearch.maxLength))}
              loading={staff.isFetching}
              options={(staff.data ?? []).map((u) => ({ value: u.user_id, label: `${u.display_name ?? u.username} (${u.username})` }))}
            />
          </Form.Item>
        )}
        <Form.Item name="role_id" label={t('platform.userRoles.role')} rules={[{ required: true, message: t('platform.userRoles.roleRequired') }]}>
          <Select
            showSearch
            optionFilterProp="label"
            disabled={kind === 'tenant' && !target}
            loading={roles.isFetching}
            options={offered.map((r) => ({ value: r.role_id, label: `${r.code} — ${r.name.vi}` }))}
          />
        </Form.Item>
        <Form.Item name="scope_type" label={t('platform.userRoles.scope')} rules={[{ required: true }]}>
          <Select disabled={scopes.length === 1} options={scopes.map((x) => ({ value: x, label: t(`platform.scopeType.${x}`) }))} />
        </Form.Item>
        {scopeType === 'school' ? (
          <Form.Item name="scope_id" label={t('platform.scopeType.school')} rules={[{ required: true, message: t('platform.userRoles.scopeIdRequired') }]}>
            <Select loading={schools.isFetching} options={(schools.data ?? []).map((s) => ({ value: s.school_id, label: `${s.name.vi} (${s.code})` }))} />
          </Form.Item>
        ) : null}
        {scopeType === 'subject' ? (
          <Form.Item name="scope_id" label={t('platform.userRoles.subjectId')} rules={[{ required: true, whitespace: true, message: t('platform.userRoles.scopeIdRequired') }, uuidRule]}>
            <Input maxLength={36} />
          </Form.Item>
        ) : null}
        <Form.Item name="valid_from" label={t('platform.userRoles.validFromLabel')}>
          <DatePicker showTime format="DD/MM/YYYY HH:mm" />
        </Form.Item>
        <Form.Item
          name="valid_to"
          label={t('platform.userRoles.validToLabel')}
          dependencies={['valid_from']}
          rules={[
            ({ getFieldValue }) => ({
              validator: (_, v?: Dayjs) =>
                validRange((getFieldValue('valid_from') as Dayjs | undefined)?.toISOString(), v?.toISOString()) ? Promise.resolve() : Promise.reject(new Error(t('platform.userRoles.validToAfter'))),
            }),
          ]}
        >
          <DatePicker showTime format="DD/MM/YYYY HH:mm" />
        </Form.Item>
        {needsTraining ? (
          <>
            <Alert type="info" showIcon message={t('platform.userRoles.trainingHelp')} />
            <Form.Item name="training_completed_at" label={t('platform.userRoles.trainingDate')} rules={[{ required: true, message: t('platform.userRoles.trainingRequired') }]}>
              <DatePicker format="DD/MM/YYYY" />
            </Form.Item>
          </>
        ) : null}
        <ReasonField limit={LIMITS.grantReason} />
      </Form>
      {grant.error ? <ProblemMessage error={grant.error} /> : null}
    </Modal>
  );
}
