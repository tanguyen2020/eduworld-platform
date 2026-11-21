// Filter bar of the School Learning Dashboard (F056 T018, T052, F056-AC-01, F056-AC-03, F056-AC-08, F056-AC-19): school,
// grade, class, subject (only the subjects of the session's scope) and the range picker of ops-ui. Filters live in the
// URL (drill-down keeps them) and the last grade/subject/preset is remembered per viewer and tenant.
import { DashboardRangePicker } from '@eduworld/ops-ui/dashboard';
import { Flex, Form, Select } from 'antd';
import { useCallback, useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { useSearchParams } from 'react-router-dom';
import { useMyPermissions } from '../../../shared/api/hooks/useMyPermissions';
import { useSession } from '../../../session/SessionProvider';
import { useSchoolClasses } from '../classes/useClasses';
import { SchoolSelect } from '../common/SchoolSelect';
import { useSchoolLmsScope } from '../common/scope';
import { useSchoolAcademicYears, useSubjects } from './api/dashboardApi';
import { schoolCalendar } from './model/calendar';
import { filtersFromSearch, filtersToSearch, recalledSearch, rememberFilters, subjectLabel, subjectsInScope, todayIso, type DashboardFilters } from './model/filters';
import { RANGE_LIMITS } from './model/limits';

const GRADES = Array.from({ length: 12 }, (_, i) => i + 1);

/** Filters of the URL + setter (replace, so Back leaves the dashboard instead of undoing a filter). */
export function useDashboardFilters() {
  const [params, setParams] = useSearchParams();
  const { userKey } = useSession();
  const scope = useSchoolLmsScope();
  const today = todayIso();
  const filters = useMemo(() => filtersFromSearch(params, today), [params, today]);
  useEffect(() => {
    if (params.has('preset')) return;
    const recalled = recalledSearch(userKey, scope.key, today);
    if (recalled && [...recalled.keys()].length) {
      for (const [k, v] of params) recalled.set(k, v);
      setParams(recalled, { replace: true });
    }
    // once per mount: only fills the URL when it has no range yet
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  const setFilters = useCallback(
    (f: DashboardFilters) => {
      setParams(filtersToSearch(f), { replace: true });
      rememberFilters(userKey, scope.key, f);
    },
    [setParams, userKey, scope.key],
  );
  return { filters, setFilters, search: filtersToSearch(filters).toString() };
}

export function DashboardFilterBar({ filters, onChange, showClass = true }: { filters: DashboardFilters; onChange: (f: DashboardFilters) => void; showClass?: boolean }) {
  const { t, i18n } = useTranslation();
  const perms = useMyPermissions();
  const subjects = useSubjects();
  const years = useSchoolAcademicYears(filters.schoolId);
  const calendar = useMemo(() => schoolCalendar(years.data, todayIso()), [years.data]);
  const classes = useSchoolClasses(filters.schoolId, filters.grade ? { grade: filters.grade } : {});
  const subjectOptions = subjectsInScope(subjects.data ?? [], perms.data?.roles ?? []).map((s) => ({ value: s.subject_id, label: subjectLabel(s, i18n.language) }));
  const set = (p: Partial<DashboardFilters>) => onChange({ ...filters, ...p });
  const onSchool = useCallback((schoolId: string | undefined) => onChange({ ...filters, schoolId, classId: undefined }), [filters, onChange]);
  return (
    <Flex wrap gap="small" align="end" data-testid="dashboard-filters">
      <Form.Item label={t('portal.school_lms.common.school')} style={{ marginBottom: 0 }}>
        <SchoolSelect value={filters.schoolId} onChange={onSchool} />
      </Form.Item>
      <Form.Item label={t('dashboard.filter.grade')} style={{ marginBottom: 0 }}>
        <Select
          allowClear
          aria-label={t('dashboard.filter.grade')}
          placeholder={t('dashboard.filter.allGrades')}
          value={filters.grade}
          onChange={(grade?: number) => set({ grade, classId: undefined })}
          options={GRADES.map((g) => ({ value: g, label: t('dashboard.filter.gradeN', { n: g }) }))}
          popupMatchSelectWidth={false}
        />
      </Form.Item>
      {showClass ? (
        <Form.Item label={t('dashboard.filter.class')} style={{ marginBottom: 0 }}>
          <Select
            allowClear
            aria-label={t('dashboard.filter.class')}
            placeholder={t('dashboard.filter.allClasses')}
            value={filters.classId}
            onChange={(classId?: string) => set({ classId })}
            loading={classes.isFetching}
            options={classes.items.map((c) => ({ value: c.class_id, label: c.name }))}
            popupMatchSelectWidth={false}
          />
        </Form.Item>
      ) : null}
      <Form.Item label={t('dashboard.filter.subject')} style={{ marginBottom: 0 }}>
        <Select
          allowClear
          aria-label={t('dashboard.filter.subject')}
          placeholder={t('dashboard.filter.allSubjects')}
          value={filters.subjectId}
          onChange={(subjectId?: string) => set({ subjectId })}
          loading={subjects.isFetching}
          options={subjectOptions}
          popupMatchSelectWidth={false}
        />
      </Form.Item>
      <DashboardRangePicker value={filters.range} limits={RANGE_LIMITS} calendar={calendar} onChange={(range) => set({ range })} />
    </Flex>
  );
}
