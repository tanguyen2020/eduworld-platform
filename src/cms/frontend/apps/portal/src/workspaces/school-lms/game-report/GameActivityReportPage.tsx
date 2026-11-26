// "Hoạt động game" of workspace C (F080 T055, T061, F080-AC-27…AC-30, GAME_ANALYTICS §7.2, ADMIN_PORTAL §3.14): route
// `/school-lms/reports/game-activity` inside the `menu.school_lms.reports` screen (no hand-registered menu, ADR-0090).
// Filters: range presets / custom ≤ dashboard_max_range_days, schools (only for a `unit` tenant, only the schools of the
// user's scope — every page, searchable) and grade bands. Five independent widgets on one shared getTenantGameSummary
// query; `suppressed_cells` explains how many cells < 5 students are hidden; dashboard rules of §3.14.1 (grain label,
// "Cập nhật lúc", "Tạm tính", "Chưa có số liệu trước ngày …", RANGE_TOO_LARGE / BUSY / QUERY_TIMEOUT). On a phone only
// Tổng quan and Chơi lành mạnh are shown, read-only (no filter). A System Admin works on the one tenant picked in the topbar.
import { SearchSelect } from '@eduworld/ops-ui';
import { DashboardRangePicker, type DashboardRange } from '@eduworld/ops-ui/dashboard';
import { Alert, Col, Flex, Form, Row } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { usePermissions } from '../../../permissions/usePermissions';
import { useSession } from '../../../session/SessionProvider';
import { useTenantOptions } from '../../../shared/api/hooks/useTenantOptions';
import { useMediaQuery } from '../../../shell/useMediaQuery';
import { useTenantFilter } from '../../../shell/tenantFilterStore';
import { localizedName } from '../common/format';
import { SingleTenantGate } from '../common/scope';
import { useWidgetError } from '../dashboard/DashboardUi';
import { defaultRange, todayIso } from '../dashboard/model/filters';
import { DASHBOARD_CONFIG, RANGE_LIMITS } from '../dashboard/model/limits';
import { GRADE_BANDS, useScopeSchools, useTenantGameSummary, type GradeBand } from './api/tenantGameSummary';
import { DeviceWidget, OverviewWidget, ProgressionWidget, SocialWidget, WellbeingWidget } from './widgets/widgets';

/** Prefix of the keys of this screen (trailing dot: the i18n key check treats it as a prefix). */
const K = 'portal.school_lms.game_report.';
/** Phone layout limit (ADMIN_PORTAL §1.5: `md` 768 px). */
const DESKTOP_QUERY = '(min-width: 768px)';

/** True when the tenant in scope is a `unit` (đơn vị nhiều trường): only then the school filter is offered. */
function useUnitTenant(): boolean {
  const { state } = useSession();
  const { crossTenant } = usePermissions();
  const filter = useTenantFilter();
  const tenants = useTenantOptions('', crossTenant);
  if (state.scope?.kind === 'tenant') return state.scope.tenantType === 'unit';
  return crossTenant && tenants.items.find((x) => x.tenant_id === filter[0])?.tenant_type === 'unit';
}

function Report() {
  const { t, i18n } = useTranslation();
  const desktop = useMediaQuery(DESKTOP_QUERY);
  const unit = useUnitTenant();
  const [range, setRange] = useState<DashboardRange>(() => defaultRange(todayIso()));
  const [schoolIds, setSchoolIds] = useState<string[]>([]);
  const [gradeBands, setGradeBands] = useState<GradeBand[]>([]);
  const schools = useScopeSchools(unit && desktop);
  const q = useTenantGameSummary({ from: range.from, to: range.to, grain: range.grain, schoolIds: unit && desktop ? schoolIds : [], gradeBands: desktop ? gradeBands : [] });
  const error = useWidgetError(q.error);
  const props = { data: q.data, loading: q.isPending, error };
  const schoolName = (id?: string) => localizedName(schools.items.find((s) => s.school_id === id)?.name, i18n.language) || (id ?? '');
  const hidden = q.data?.suppressed_cells ?? 0;
  return (
    <Flex vertical gap="middle" data-testid="game-activity-report">
      {desktop ? (
        <Flex wrap gap="small" align="end" data-testid="game-report-filters">
          <DashboardRangePicker value={range} limits={RANGE_LIMITS} onChange={setRange} />
          {unit ? (
            <Form.Item label={t(`${K}filter.schools`)} style={{ marginBottom: 0 }}>
              <SearchSelect<string[]>
                mode="multiple"
                allowClear
                aria-label={t(`${K}filter.schools`)}
                placeholder={t(`${K}filter.allSchools`)}
                value={schoolIds}
                onChange={(ids) => setSchoolIds(ids.slice(0, DASHBOARD_CONFIG.dashboard_page_size_max))}
                loading={schools.isFetching}
                truncated={schools.truncated}
                options={schools.items.map((s) => ({ value: s.school_id, label: localizedName(s.name, i18n.language) }))}
                maxTagCount="responsive"
                style={{ minWidth: '14rem' }}
                popupMatchSelectWidth={false}
              />
            </Form.Item>
          ) : null}
          <Form.Item label={t(`${K}filter.gradeBand`)} style={{ marginBottom: 0 }}>
            <SearchSelect<GradeBand[]>
              mode="multiple"
              allowClear
              aria-label={t(`${K}filter.gradeBand`)}
              placeholder={t(`${K}filter.allGradeBands`)}
              value={gradeBands}
              onChange={setGradeBands}
              options={GRADE_BANDS.map((g) => ({ value: g, label: t(`${K}gradeBand.${g}`) }))}
              style={{ minWidth: '12rem' }}
              popupMatchSelectWidth={false}
            />
          </Form.Item>
        </Flex>
      ) : null}
      {hidden > 0 ? <Alert type="info" showIcon data-testid="game-report-suppressed" message={t(`${K}suppressedNote`, { count: hidden })} /> : null}
      <Row gutter={[16, 16]}>
        <Col xs={24} xl={12}>
          <OverviewWidget {...props} />
        </Col>
        <Col xs={24} xl={12}>
          <WellbeingWidget {...props} />
        </Col>
        {desktop ? (
          <>
            <Col xs={24} xl={12}>
              <ProgressionWidget {...props} schoolName={schoolName} />
            </Col>
            <Col xs={24} xl={12}>
              <SocialWidget {...props} />
            </Col>
            <Col xs={24}>
              <DeviceWidget {...props} />
            </Col>
          </>
        ) : null}
      </Row>
    </Flex>
  );
}

export function GameActivityReportPage() {
  return <SingleTenantGate>{() => <Report />}</SingleTenantGate>;
}

export default GameActivityReportPage;
