// School catalog (F057 T033, F057-AC-13, ADMIN_PORTAL §3.16): listCatalogSchools filtered by tenant, province, level,
// status and name/code search (`q` 2–100). The System Admin reads one or more tenants of the tenant filter with
// `tenant_ids[]` (CrossTenantScope, no X-Tenant-Id) and must pick at least one — the backend refuses a cross-tenant read
// without tenant_ids (OQ F057-4); the Tenant column shows the tenant of each school.
import { Alert, Button, Input, Select, Space, Table, Tag } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { TenantFilter } from '../../../shell/TenantControls';
import { SCHOOL_LEVELS, SCHOOL_STATUSES, useCatalogSchools, useProvinces, type School, type SchoolFilter, type SchoolLevel, type SchoolStatus } from '../api/catalog';
import { useTenantDirectory } from '../api/tenants';
import { LIMITS } from '../limits';
import { TenantCell } from '../ui';

export function SchoolCatalogPage() {
  const { t, i18n } = useTranslation();
  const [filter, setFilter] = useState<SchoolFilter>({});
  const provinces = useProvinces('active');
  const list = useCatalogSchools(filter);
  const scope = useCrossTenantScope();
  const directory = useTenantDirectory(scope.crossTenant);
  const en = i18n.language.startsWith('en');

  const columns: ColumnsType<School> = [
    ...(scope.crossTenant ? [{ key: 'tenant', title: t('platform.common.tenant'), render: (_: unknown, r: School) => <TenantCell tenantId={r.tenant_id} directory={directory} /> }] : []),
    { key: 'code', title: t('platform.catalog.schoolCode'), dataIndex: 'code' },
    { key: 'name', title: t('platform.catalog.schoolName'), render: (_, r) => (en && r.name.en ? r.name.en : r.name.vi) },
    { key: 'province', title: t('platform.catalog.province'), render: (_, r) => (r.province ? (en ? r.province.name.en : r.province.name.vi) : '') },
    { key: 'level', title: t('platform.catalog.schoolLevel'), render: (_, r) => t(`platform.catalog.schoolLevels.${r.school_level}`) },
    { key: 'status', title: t('platform.common.status'), render: (_, r) => <Tag>{t(`platform.catalog.schoolStatus.${r.status}`)}</Tag> },
    { key: 'source', title: t('platform.catalog.source'), render: (_, r) => (r.source ? t(`platform.catalog.sources.${r.source}`) : '') },
  ];

  return (
    <section aria-label={t('platform.catalog.schools')}>
      <Space wrap>
        <TenantFilter />
        <Select
          aria-label={t('platform.catalog.province')}
          placeholder={t('platform.catalog.allProvinces')}
          allowClear
          showSearch
          optionFilterProp="label"
          value={filter.province}
          onChange={(province?: string) => setFilter((f) => ({ ...f, province }))}
          options={(provinces.data?.items ?? []).map((p) => ({ value: p.code, label: p.name.vi }))}
        />
        <Select
          aria-label={t('platform.catalog.schoolLevel')}
          placeholder={t('platform.catalog.allLevels')}
          allowClear
          value={filter.school_level}
          onChange={(school_level?: SchoolLevel) => setFilter((f) => ({ ...f, school_level }))}
          options={SCHOOL_LEVELS.map((x) => ({ value: x, label: t(`platform.catalog.schoolLevels.${x}`) }))}
        />
        <Select
          aria-label={t('platform.common.status')}
          placeholder={t('platform.catalog.allSchoolStatuses')}
          allowClear
          value={filter.status}
          onChange={(status?: SchoolStatus) => setFilter((f) => ({ ...f, status }))}
          options={SCHOOL_STATUSES.map((x) => ({ value: x, label: t(`platform.catalog.schoolStatus.${x}`) }))}
        />
        <Input.Search
          aria-label={t('platform.catalog.schoolSearch')}
          placeholder={t('platform.catalog.schoolSearch')}
          maxLength={LIMITS.schoolSearch.maxLength}
          showCount
          allowClear
          onSearch={(v) => setFilter((f) => ({ ...f, q: v.trim().length >= LIMITS.schoolSearch.minLength ? v.trim() : undefined }))}
        />
      </Space>
      {list.ready ? null : <Alert type="warning" showIcon message={t('platform.catalog.selectTenants')} />}
      {list.error ? <ProblemMessage error={list.error} /> : null}
      {list.ready ? (
        <>
          <Table rowKey="school_id" columns={columns} dataSource={list.rows} loading={list.isFetching} pagination={false} locale={{ emptyText: t('platform.common.empty') }} />
          {list.hasNextPage ? (
            <Button onClick={() => list.fetchNextPage()} loading={list.isFetchingNextPage}>
              {t('platform.common.loadMore')}
            </Button>
          ) : null}
        </>
      ) : null}
    </section>
  );
}
