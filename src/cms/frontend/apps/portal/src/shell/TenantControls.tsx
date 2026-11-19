// Tenant controls of the topbar and forms (F051 T040, T045):
// - TenantSwitcher (F051-AC-20): staff of several tenants switch the working tenant (same token, new X-Tenant-Id,
//   cache cleared, permissions reloaded, back to the workspace overview); unsaved drafts are confirmed first; staff of
//   one tenant only see its name.
// - TenantFilter (F051-AC-22/26/27): System Admin only (`platform.cross_tenant.read`): searchable multi-select over
//   `listTenants`, default "Tất cả tenant", kept for the browser session, search cut at the OpenAPI `maxLength`;
//   pages follow the cursor while scrolling and a disabled tenant stays listed with "đã tắt" (F057 edge cases).
// - TenantTargetField (F051-AC-25): mandatory target tenant of a System Admin write (optionally "Platform"), no
//   "all tenants" choice. tenantColumn (F051-AC-23): standard "Tenant" column (name + type) for lists.
import { App as AntApp, Form, Select, Tag, Typography, theme } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useState, type UIEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import type { WorkspaceDef } from '../generated/menu-catalog';
import { usePermissions } from '../permissions/usePermissions';
import { useSession } from '../session/SessionProvider';
import { hasUnsavedChanges } from '../session/unsavedChangesGuard';
import { clampTenantSearch, loadMoreOnScroll, useTenantOptions, type TenantSummary } from '../shared/api/hooks/useTenantOptions';
import { PLATFORM_TARGET } from '../shared/api/withTenantScope';
import { tenantFilterStore, useTenantFilter } from './tenantFilterStore';

export function TenantSwitcher({ workspace }: { workspace?: WorkspaceDef }) {
  const { t } = useTranslation();
  const session = useSession();
  const navigate = useNavigate();
  const { modal } = AntApp.useApp();
  const { scope, tenants } = session.state;
  if (scope?.kind !== 'tenant') return null;
  const choices = tenants.filter((x) => x.status !== 'disabled');
  if (choices.length <= 1) {
    return (
      <Typography.Text strong aria-label={t('shell.tenantSwitcher.label')}>
        {scope.name}
      </Typography.Text>
    );
  }
  const apply = (tenantId: string) => {
    const tenant = choices.find((x) => x.tenant_id === tenantId);
    if (!tenant || tenant.tenant_id === scope.tenantId) return;
    session.switchTenant(tenant);
    navigate(workspace ? workspace.path : '/', { replace: true });
  };
  const onChange = (tenantId: string) => {
    if (!hasUnsavedChanges()) return apply(tenantId);
    modal.confirm({
      title: t('shell.tenantSwitcher.unsavedTitle'),
      content: t('shell.tenantSwitcher.unsavedContent'),
      okText: t('shell.tenantSwitcher.confirm'),
      cancelText: t('shell.tenantSwitcher.cancel'),
      onOk: () => apply(tenantId),
    });
  };
  return <TenantSwitcherView tenants={choices} value={scope.tenantId} onChange={onChange} />;
}

type TenantLike = Pick<TenantSummary, 'tenant_id' | 'name' | 'code'> & Partial<Pick<TenantSummary, 'status'>>;
type Translate = (key: string) => string;
/** "Name (code)", plus "đã tắt" for a disabled tenant (F057 edge case: it keeps its place, clearly marked). */
export const tenantLabel = (x: TenantLike, t: Translate): string =>
  x.status === 'disabled' ? `${x.name} (${x.code}) · ${t('shell.tenantFilter.disabled')}` : `${x.name} (${x.code})`;
const tenantOption = (x: TenantLike, t: Translate) => ({ value: x.tenant_id, label: tenantLabel(x, t) });

/** Presentational switcher (stories, tests): one working tenant among several. */
export function TenantSwitcherView({ tenants, value, onChange }: { tenants: readonly TenantLike[]; value: string; onChange: (tenantId: string) => void }) {
  const { t } = useTranslation();
  return (
    <Select
      aria-label={t('shell.tenantSwitcher.label')}
      value={value}
      onChange={onChange}
      showSearch
      optionFilterProp="label"
      popupMatchSelectWidth={false}
      options={tenants.map((x) => tenantOption(x, t))}
    />
  );
}

export interface TenantFilterViewProps {
  options: readonly TenantLike[];
  value: readonly string[];
  onChange: (ids: string[]) => void;
  search: string;
  onSearch: (q: string) => void;
  loading?: boolean;
  /** Scroll of the dropdown list: loads the next cursor page at its end (F057). */
  onPopupScroll?: (e: UIEvent<HTMLDivElement>) => void;
}

/** Presentational System Admin filter: multi-select, empty = "Tất cả tenant", search capped at `q` maxLength. */
export function TenantFilterView({ options, value, onChange, search, onSearch, loading, onPopupScroll }: TenantFilterViewProps) {
  const { t } = useTranslation();
  const { token } = theme.useToken();
  return (
    <Select
      mode="multiple"
      aria-label={t('shell.tenantFilter.label')}
      placeholder={t('shell.tenantFilter.all')}
      value={[...value]}
      onChange={onChange}
      showSearch
      filterOption={false}
      searchValue={search}
      onSearch={(v) => onSearch(clampTenantSearch(v))}
      loading={loading}
      maxTagCount="responsive"
      allowClear
      onPopupScroll={onPopupScroll}
      options={options.map((x) => tenantOption(x, t))}
      style={{ minWidth: token.controlHeightLG * 5 }}
    />
  );
}

export function TenantFilter() {
  const { crossTenant } = usePermissions();
  const selected = useTenantFilter();
  const [search, setSearch] = useState('');
  const options = useTenantOptions(search, crossTenant);
  if (!crossTenant) return null;
  return (
    <TenantFilterView
      options={options.data ?? []}
      value={selected}
      onChange={(ids) => tenantFilterStore.set(ids)}
      search={search}
      onSearch={setSearch}
      loading={options.isFetching}
      onPopupScroll={loadMoreOnScroll(options)}
    />
  );
}

export function TenantTargetField({ name = 'tenant_target', allowPlatform = false }: { name?: string; allowPlatform?: boolean }) {
  const { t } = useTranslation();
  const [search, setSearch] = useState('');
  const options = useTenantOptions(search, true);
  const items = (options.data ?? []).map((x) => tenantOption(x, t));
  return (
    <Form.Item name={name} label={t('shell.tenantTarget.label')} rules={[{ required: true, message: t('shell.tenantTarget.required') }]}>
      <Select
        showSearch
        filterOption={false}
        searchValue={search}
        onSearch={(v) => setSearch(clampTenantSearch(v))}
        loading={options.isFetching}
        onPopupScroll={loadMoreOnScroll(options)}
        options={allowPlatform ? [{ value: PLATFORM_TARGET, label: t('shell.tenantTarget.platform') }, ...items] : items}
      />
    </Form.Item>
  );
}

/** Standard "Tenant" column for System Admin lists; null for tenant users (no Tenant column, F051-AC-26). */
export function useTenantColumn<T extends { tenant_id?: string | null }>(tenants: ReadonlyMap<string, Pick<TenantSummary, 'name' | 'tenant_type'>>): ColumnType<T> | null {
  const { t } = useTranslation();
  const { crossTenant } = usePermissions();
  if (!crossTenant) return null;
  return {
    key: 'tenant',
    title: t('shell.tenantTarget.label'),
    render: (_: unknown, row: T) => {
      const x = row.tenant_id ? tenants.get(row.tenant_id) : undefined;
      return x ? (
        <span>
          {x.name} <Tag>{t(`shell.tenantType.${x.tenant_type}`)}</Tag>
        </span>
      ) : null;
    },
  };
}
