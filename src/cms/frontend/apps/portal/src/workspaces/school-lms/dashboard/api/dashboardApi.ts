// Read hooks of the School Learning Dashboard (F056 T018, T027; F056-AC-01, AC-08, AC-11, AC-16, AC-21): the only code of
// the dashboard screens that calls the network (AGENTS R1). Every number comes from the dashboard API (rollups / read
// models, ADR-0062) — the portal never aggregates business lists. Query keys carry the tenant of the data
// (`['school-lms', 'dashboard', name, scope.key, …]`), so a tenant switch never shows another tenant's numbers; every
// request goes through the page's DashboardQueue (≤ dashboard_max_concurrent_per_user in flight, F056-AC-04).
import { useDashboardQueue } from '@eduworld/ops-ui/dashboard';
import { useInfiniteQuery, useQuery, type QueryKey } from '@tanstack/react-query';
import { useApi, type Schemas } from '../../common/api';
import { useSchoolLmsScope, type SchoolLmsScope } from '../../common/scope';
import { DASHBOARD_CONFIG, DASHBOARD_QUERY_TIMING } from '../model/limits';
import type { DashboardFilters } from '../model/filters';

export type SchoolDashboard = Schemas['SchoolDashboard'];
export type ClassDashboard = Schemas['ClassDashboard'];
export type StudentLearningProfile = Schemas['StudentLearningProfile'];
export type AttentionReason = Schemas['AttentionReason'];
export type Subject = Schemas['Subject'];
export type RegulationReferenceReport = Schemas['RegulationReferenceReport'];
export type AssessmentPeriod = Schemas['AssessmentPeriod'];

export const dashKey = (name: string, scope: SchoolLmsScope, ...rest: unknown[]): QueryKey => ['school-lms', 'dashboard', name, scope.key, ...rest];

/** Api, scope and the page queue of a dashboard hook. */
export function useDashboardContext() {
  const api = useApi();
  const scope = useSchoolLmsScope();
  const queue = useDashboardQueue();
  return { api, scope, queue };
}

const rangeQuery = (f: DashboardFilters) => ({ from: f.range.from, to: f.range.to });

/** getSchoolDashboard: the 5 widgets of the school (or of one class with `class_id`). */
export function useSchoolDashboard(f: DashboardFilters) {
  const { api, scope, queue } = useDashboardContext();
  const query = {
    school_id: f.schoolId!,
    ...rangeQuery(f),
    ...(f.grade ? { grade_level: f.grade } : {}),
    ...(f.classId ? { class_id: f.classId } : {}),
    ...(f.subjectId ? { subject_id: f.subjectId } : {}),
  };
  return useQuery({
    queryKey: dashKey('school', scope, query),
    queryFn: () => queue.run(() => api.call<SchoolDashboard>(api.client.GET('/lms/dashboards/school', { params: { query }, headers: scope.headers } as never))),
    enabled: scope.ready && !!f.schoolId,
    ...DASHBOARD_QUERY_TIMING,
  });
}

/** getClassDashboard: heatmap class × LO and the students of the class (cursor pages ≤ dashboard_page_size_max). */
export function useClassDashboard(classId: string | undefined, f: DashboardFilters) {
  const { api, scope, queue } = useDashboardContext();
  const query = { ...rangeQuery(f), ...(f.subjectId ? { subject_id: f.subjectId } : {}), ...(f.range.grain ? { grain: f.range.grain } : {}), page_size: DASHBOARD_CONFIG.dashboard_page_size_max };
  const q = useInfiniteQuery({
    queryKey: dashKey('class', scope, classId, query),
    queryFn: ({ pageParam }) =>
      queue.run(() =>
        api.call<ClassDashboard>(
          api.client.GET('/lms/classes/{class_id}/dashboard', {
            params: { path: { class_id: classId! }, query: { ...query, ...(pageParam ? { cursor: pageParam } : {}) } },
            headers: scope.headers,
          } as never),
        ),
      ),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last: ClassDashboard) => (last.students.page?.has_more && last.students.page.next_cursor ? last.students.page.next_cursor : undefined),
    enabled: scope.ready && !!classId,
    ...DASHBOARD_QUERY_TIMING,
  });
  return {
    first: q.data?.pages[0],
    students: q.data?.pages.flatMap((p) => p.students.data) ?? [],
    hasMore: !!q.hasNextPage,
    loadMore: () => void q.fetchNextPage(),
    isLoading: q.isPending && !!classId,
    isFetching: q.isFetching,
    error: q.error,
  };
}

/** getStudentLearningProfile (audited `learning.profile_viewed` by the server). */
export function useStudentProfile(studentId: string | undefined, f: DashboardFilters) {
  const { api, scope, queue } = useDashboardContext();
  const query = { ...rangeQuery(f), ...(f.subjectId ? { subject_id: f.subjectId } : {}) };
  return useQuery({
    queryKey: dashKey('student', scope, studentId, query),
    queryFn: () =>
      queue.run(() =>
        api.call<StudentLearningProfile>(api.client.GET('/lms/students/{student_id}/analytics', { params: { path: { student_id: studentId! }, query }, headers: scope.headers } as never)),
      ),
    enabled: scope.ready && !!studentId,
    ...DASHBOARD_QUERY_TIMING,
  });
}

/** Subjects of the tenant (listSubjects) for the subject filter. */
export function useSubjects() {
  const { api, scope } = useDashboardContext();
  return useQuery({
    queryKey: dashKey('subjects', scope),
    queryFn: async () => (await api.call<{ items: Subject[] }>(api.client.GET('/learning/subjects', { headers: scope.headers } as never))).items,
    enabled: scope.ready,
    staleTime: 10 * 60_000,
  });
}

export interface ReferenceFilters {
  schoolId?: string;
  subjectId?: string;
  classId?: string;
  period: AssessmentPeriod;
}

/** getRegulationReferenceReport: reference levels TT27/TT22 (always with the server disclaimer, F056-AC-16). */
export function useRegulationReference(f: ReferenceFilters) {
  const { api, scope, queue } = useDashboardContext();
  const query = { subject_id: f.subjectId!, period: f.period, ...(f.classId ? { class_id: f.classId } : {}), limit: DASHBOARD_CONFIG.dashboard_page_size_max };
  const q = useInfiniteQuery({
    queryKey: dashKey('reference', scope, f.schoolId, query),
    queryFn: ({ pageParam }) =>
      queue.run(() =>
        api.call<RegulationReferenceReport>(
          api.client.GET('/lms/schools/{school_id}/reference-levels', {
            params: { path: { school_id: f.schoolId! }, query: { ...query, ...(pageParam ? { cursor: pageParam } : {}) } },
            headers: scope.headers,
          } as never),
        ),
      ),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last: RegulationReferenceReport) => (last.page?.has_more && last.page.next_cursor ? last.page.next_cursor : undefined),
    enabled: scope.ready && !!f.schoolId && !!f.subjectId,
  });
  return {
    first: q.data?.pages[0],
    items: q.data?.pages.flatMap((p) => p.items) ?? [],
    hasMore: !!q.hasNextPage,
    loadMore: () => void q.fetchNextPage(),
    isLoading: q.isPending && !!f.schoolId && !!f.subjectId,
    error: q.error,
  };
}

/** listSchoolAcademicYears (module identity, F010 T047): school years and terms of the "Năm học này" / "Học kỳ này" presets. */
export function useSchoolAcademicYears(schoolId: string | undefined) {
  const { api, scope } = useDashboardContext();
  return useQuery({
    queryKey: dashKey('academic-years', scope, schoolId),
    queryFn: async () =>
      (
        await api.call<{ data: Schemas['SchoolAcademicYear'][] }>(
          api.client.GET('/schools/{school_id}/academic-years', { params: { path: { school_id: schoolId! } }, headers: scope.headers } as never),
        )
      ).data,
    enabled: scope.ready && !!schoolId,
    staleTime: 60 * 60_000,
  });
}
