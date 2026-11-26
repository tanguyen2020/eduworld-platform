// Filter bar of Game Analytics (F080 T044, F080-AC-21, AC-22, ADMIN_PORTAL §3.14.1): range (DashboardRangePicker —
// presets, custom range ≤ dashboard_max_range_days = 2 years, coarser grain only), tenants (the System Admin tenant filter
// of the shell: every page of listTenants, searchable — ADR-0027 §6), ONE "cắt theo" dimension with its values (school /
// grade band / platform / locale / device tier — the server stores one dimension per rollup row and answers 422 to two),
// client versions (performance panel only; ≤ 20 values of ≤ 64 characters) and "So sánh kỳ trước". Schools of the chosen
// tenants come from listCatalogSchools, every cursor page (a dropdown is never one page of 100).
import { SearchSelect } from '@eduworld/ops-ui';
import { DashboardRangePicker } from '@eduworld/ops-ui/dashboard';
import { Flex, Form, Switch } from 'antd';
import { useTranslation } from 'react-i18next';
import { useAllPages } from '../../../shared/api/fetchAllPages';
import { TenantFilter } from '../../../shell/TenantControls';
import { useLiveOpsApi, type Page, type Schemas } from '../api/common';
import { FIELDS, RANGE_LIMITS } from '../limits';
import { nameIn } from '../shared/ui';
import { DIMENSION_VALUES, DIMENSIONS, type AnalyticsFilters, type Dimension } from './model';

type School = Schemas['School'];

/** Schools of the selected tenants (listCatalogSchools, every page); only once at least one tenant is chosen. */
function useTenantSchools(tenantIds: readonly string[]) {
  const api = useLiveOpsApi();
  return useAllPages<School>(
    ['live-ops', 'analytics-schools', tenantIds],
    async (query) => {
      const r = await api.call<{ items: School[]; page: Page<School>['page'] }>(api.client.GET('/catalog/schools', { params: { query: { ...query, tenant_ids: [...tenantIds] } } } as never));
      return { data: r.items, page: r.page };
    },
    { enabled: tenantIds.length > 0, staleTime: 5 * 60_000 },
  );
}

export function AnalyticsFilterBar({ value, onChange }: { value: AnalyticsFilters; onChange: (next: AnalyticsFilters) => void }) {
  const { t, i18n } = useTranslation();
  const set = (p: Partial<AnalyticsFilters>) => onChange({ ...value, ...p });
  const schools = useTenantSchools(value.dimension === 'school' ? value.tenantIds : []);
  const valueOptions =
    value.dimension === 'school'
      ? schools.items.map((s) => ({ value: s.school_id, label: nameIn(s.name, i18n.language) || s.school_id }))
      : value.dimension
        ? DIMENSION_VALUES[value.dimension].map((v) => ({ value: v, label: t(`liveOps.analytics.dim.${value.dimension}.${v}`) }))
        : [];
  const schoolNeedsTenant = value.dimension === 'school' && value.tenantIds.length === 0;
  return (
    <Flex wrap gap="small" align="end" data-testid="analytics-filters">
      <DashboardRangePicker value={value.range} limits={RANGE_LIMITS} onChange={(range) => set({ range })} />
      <TenantFilter />
      <Form.Item label={t('liveOps.analytics.filter.dimension')} style={{ marginBottom: 0 }}>
        <SearchSelect<Dimension | undefined>
          allowClear
          aria-label={t('liveOps.analytics.filter.dimension')}
          placeholder={t('liveOps.analytics.filter.noDimension')}
          value={value.dimension}
          onChange={(dimension) => set({ dimension: dimension ?? undefined, values: [] })}
          options={DIMENSIONS.map((d) => ({ value: d, label: t(`liveOps.analytics.filter.dimensions.${d}`) }))}
          popupMatchSelectWidth={false}
        />
      </Form.Item>
      {value.dimension ? (
        <Form.Item label={t('liveOps.analytics.filter.values')} style={{ marginBottom: 0 }}>
          <SearchSelect<string[]>
            mode="multiple"
            allowClear
            aria-label={t('liveOps.analytics.filter.values')}
            placeholder={schoolNeedsTenant ? t('liveOps.analytics.filter.pickTenantFirst') : t('liveOps.analytics.filter.allValues')}
            disabled={schoolNeedsTenant}
            loading={schools.isFetching}
            truncated={schools.truncated}
            value={value.values}
            onChange={(values) => set({ values })}
            options={valueOptions}
            maxTagCount="responsive"
            style={{ minWidth: '14rem' }}
            popupMatchSelectWidth={false}
          />
        </Form.Item>
      ) : null}
      <Form.Item label={t('liveOps.analytics.filter.clientVersions')} style={{ marginBottom: 0 }}>
        <SearchSelect<string[]>
          mode="tags"
          aria-label={t('liveOps.analytics.filter.clientVersions')}
          placeholder={t('liveOps.analytics.filter.clientVersionsHint')}
          value={value.clientVersions}
          onChange={(v) => set({ clientVersions: v.filter((x) => x.length <= FIELDS.clientVersion.maxLength).slice(0, FIELDS.clientVersion.maxItems) })}
          options={[]}
          style={{ minWidth: '12rem' }}
        />
      </Form.Item>
      <Form.Item label={t('liveOps.analytics.filter.compare')} style={{ marginBottom: 0 }}>
        <Switch aria-label={t('liveOps.analytics.filter.compare')} checked={value.compare} onChange={(compare) => set({ compare })} />
      </Form.Item>
    </Flex>
  );
}
