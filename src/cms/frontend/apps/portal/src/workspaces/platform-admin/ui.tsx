// Presentational helpers shared by the screens of workspace D (F057): page heading, reason field with the OpenAPI
// limits and a character counter (F057-AC-21), Tenant cell (name + type, "Platform" for platform data), large-screen
// switch (ADMIN_PORTAL §1.5), timestamps, mapping of `422 VALIDATION_FAILED` to form fields. No business rule here.
import { openapiInputProps, openapiRules, OPS_UI_NS } from '@eduworld/ops-ui';
import { Form, Input, Select, Tag, Typography, theme, type FormInstance } from 'antd';
import dayjs from 'dayjs';
import { useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { clampTenantSearch, loadMoreOnScroll, useTenantOptions } from '../../shared/api/hooks/useTenantOptions';
import { mapValidationErrors } from '../../shared/api/mapValidationErrors';
import { ApiError } from '../../shared/api/middleware/problem';
import { tenantLabel } from '../../shell/TenantControls';
import { useMediaQuery } from '../../shell/useMediaQuery';
import type { FieldLimit } from './limits';
import type { TenantSummary } from './api/tenants';

export function PageTitle({ title, extra }: { title: string; extra?: ReactNode }) {
  const { token } = theme.useToken();
  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: token.marginSM, flexWrap: 'wrap' }}>
      <Typography.Title level={1}>{title}</Typography.Title>
      {extra}
    </div>
  );
}

/** Text field limited by an OpenAPI string schema: hard cap, counter, translated rules (ops-ui F050-AC-09). */
export function LimitedField({ name, label, limit, required, textarea, help, extraRules = [] }: {
  name: string | (string | number)[];
  label: string;
  limit: FieldLimit;
  required?: boolean;
  textarea?: boolean;
  help?: string;
  extraRules?: Parameters<typeof Form.Item>[0]['rules'];
}) {
  const { t } = useTranslation(OPS_UI_NS);
  const schema = { type: 'string', maxLength: limit.maxLength, minLength: limit.minLength, pattern: 'pattern' in limit ? limit.pattern : undefined };
  return (
    <Form.Item name={name} label={label} extra={help} rules={[...openapiRules(schema, { required, t }), ...(extraRules ?? [])]}>
      {textarea ? <Input.TextArea autoSize={{ minRows: 2, maxRows: 6 }} {...openapiInputProps(schema)} /> : <Input {...openapiInputProps(schema)} />}
    </Form.Item>
  );
}

/** Hard cap + counter of a free-text filter input (search, action…) from an OpenAPI string limit (F059-AC-27). */
export function limitInputProps(limit: FieldLimit) {
  const schema = { type: 'string', maxLength: limit.maxLength };
  return openapiInputProps(schema);
}

/** Mandatory reason of a platform write (min/max of the operation's schema). */
export function ReasonField({ limit, required = true, name = 'reason' }: { limit: FieldLimit; required?: boolean; name?: string }) {
  const { t } = useTranslation();
  return <LimitedField name={name} label={t('platform.common.reason')} limit={limit} required={required} textarea />;
}

export function TenantCell({ tenantId, directory }: { tenantId?: string | null; directory: ReadonlyMap<string, TenantSummary> }) {
  const { t } = useTranslation();
  if (!tenantId) return <Tag>{t('platform.common.platformScope')}</Tag>;
  const x = directory.get(tenantId);
  if (!x) return <Typography.Text code>{tenantId}</Typography.Text>;
  return (
    <span>
      {x.name} <Tag>{t(`shell.tenantType.${x.tenant_type}`)}</Tag>
    </span>
  );
}

/**
 * Tenant picker of workspace D dialogs (assign: several tenants; clone: one source, one target) over listTenants with
 * search (`q` maxLength) and cursor paging. `exclude` hides tenants (the clone source from the target list). No
 * "all tenants" entry: a write always names its tenants (ADMIN_PORTAL §2.5).
 */
export function TenantSelect({ value, onChange, multiple, exclude = [], ariaLabel, maxCount, allowClear }: {
  value?: string | string[];
  onChange?: (v: string | string[]) => void;
  multiple?: boolean;
  exclude?: readonly string[];
  ariaLabel: string;
  maxCount?: number;
  allowClear?: boolean;
}) {
  const { t } = useTranslation();
  const [search, setSearch] = useState('');
  const options = useTenantOptions(search, true);
  const items = (options.data ?? []).filter((x) => !exclude.includes(x.tenant_id)).map((x) => ({ value: x.tenant_id, label: tenantLabel(x, t) }));
  return (
    <Select
      aria-label={ariaLabel}
      mode={multiple ? 'multiple' : undefined}
      maxCount={multiple ? maxCount : undefined}
      allowClear={allowClear}
      value={value}
      onChange={onChange}
      showSearch
      filterOption={false}
      searchValue={search}
      onSearch={(v) => setSearch(clampTenantSearch(v))}
      loading={options.isFetching}
      onPopupScroll={loadMoreOnScroll(options)}
      options={items}
    />
  );
}

/** ≥ md breakpoint: write tasks (assign, clone, tenant forms) need it; smaller screens are read only. */
export function useLargeScreen(): boolean {
  const { token } = theme.useToken();
  return useMediaQuery(`(min-width: ${token.screenMD}px)`);
}

export const formatTime = (ts?: string | null): string => (ts ? dayjs(ts).format('DD/MM/YYYY HH:mm') : '');

/** `422 VALIDATION_FAILED` errors[] → form fields; returns true when every error landed on a field. */
export function applyFieldErrors(form: FormInstance, error: unknown, fields: readonly string[], t: (k: string) => string, exists: (k: string) => boolean): boolean {
  if (!(error instanceof ApiError)) return false;
  const { fields: updates, unknown } = mapValidationErrors(error, fields, t, exists);
  if (updates.length) form.setFields(updates);
  return updates.length > 0 && unknown.length === 0;
}
