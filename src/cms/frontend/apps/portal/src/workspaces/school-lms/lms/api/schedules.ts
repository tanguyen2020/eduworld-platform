// Schedule and study group hooks (F055 T034; F055-AC-16, AC-17): listScheduleItems (date range, cursor),
// create (Idempotency-Key) / update (If-Match = row version, delta F055-D1) / delete (soft-delete on the server);
// study groups of a school: list, get (members + ETag), create (Idempotency-Key), update (If-Match), delete.
import { useMutation, useQuery } from '@tanstack/react-query';
import { useIdempotentMutation } from '../../../../shared/api/useIdempotentMutation';
import { ifMatchOf } from './courses';
import { LMS_PAGE_SIZE, lmsKey, MERGE_PATCH, useDataList, useLmsContext, type DataPage, type S } from './lmsCommon';

export type ScheduleItem = S['ScheduleItem'];
export type ScheduleItemInput = S['ScheduleItemInput'];
export type StudyGroup = S['StudyGroup'];
export type StudyGroupInput = S['StudyGroupInput'];

export function useScheduleItems(range: { from: string; to: string; courseId?: string }) {
  const { api, scope } = useLmsContext();
  return useDataList<ScheduleItem>(
    lmsKey('schedules', scope, range.from, range.to, range.courseId ?? ''),
    (cursor) =>
      api.call<DataPage<ScheduleItem>>(
        api.client.GET('/lms/schedules', {
          params: { query: { from: range.from, to: range.to, limit: LMS_PAGE_SIZE, ...(cursor ? { cursor } : {}), ...(range.courseId ? { course_id: range.courseId } : {}) } },
          headers: scope.headers,
        } as never),
      ),
    scope.ready,
  );
}

export function useSaveScheduleItem() {
  const { api, scope, invalidate } = useLmsContext();
  return useIdempotentMutation<ScheduleItem, { item?: ScheduleItem; body: ScheduleItemInput }>(
    (v, intent) =>
      v.item
        ? api.call<ScheduleItem>(
            api.client.PATCH('/lms/schedules/{schedule_item_id}', {
              params: { path: { schedule_item_id: v.item.schedule_item_id } },
              body: v.body,
              headers: { ...scope.headers, ...MERGE_PATCH, ...ifMatchOf((v.item as { version?: number }).version) },
            } as never),
          )
        : api.call<ScheduleItem>(api.client.POST('/lms/schedules', { body: v.body, headers: { ...scope.headers, ...intent } } as never)),
    { onSuccess: () => void invalidate('schedules') },
  );
}

export function useDeleteScheduleItem() {
  const { api, scope, invalidate } = useLmsContext();
  return useMutation({
    mutationFn: (id: string) => api.call<void>(api.client.DELETE('/lms/schedules/{schedule_item_id}', { params: { path: { schedule_item_id: id } }, headers: scope.headers } as never)),
    onSuccess: () => void invalidate('schedules'),
  });
}

export function useStudyGroups(schoolId: string | undefined, classId?: string, enabled = true) {
  const { api, scope } = useLmsContext();
  return useDataList<StudyGroup>(
    lmsKey('study-groups', scope, schoolId, classId ?? ''),
    (cursor) =>
      api.call<DataPage<StudyGroup>>(
        api.client.GET('/lms/schools/{school_id}/study-groups', {
          params: { path: { school_id: schoolId! }, query: { limit: LMS_PAGE_SIZE, ...(cursor ? { cursor } : {}), ...(classId ? { class_id: classId } : {}) } },
          headers: scope.headers,
        } as never),
      ),
    scope.ready && !!schoolId && enabled,
  );
}

/** One group with its members; the apiClient remembers its ETag for the If-Match of updateStudyGroup. */
export function useStudyGroup(schoolId: string | undefined, id: string | undefined) {
  const { api, scope } = useLmsContext();
  return useQuery({
    queryKey: lmsKey('study-group', scope, schoolId, id),
    queryFn: () =>
      api.call<StudyGroup>(api.client.GET('/lms/schools/{school_id}/study-groups/{study_group_id}', { params: { path: { school_id: schoolId!, study_group_id: id! } }, headers: scope.headers } as never)),
    enabled: scope.ready && !!schoolId && !!id,
  });
}

export function useSaveStudyGroup(schoolId: string) {
  const { api, scope, invalidate } = useLmsContext();
  return useIdempotentMutation<StudyGroup, { id?: string; body: StudyGroupInput }>(
    (v, intent) =>
      v.id
        ? api.call<StudyGroup>(
            api.client.PATCH('/lms/schools/{school_id}/study-groups/{study_group_id}', {
              params: { path: { school_id: schoolId, study_group_id: v.id } },
              body: v.body,
              headers: { ...scope.headers, ...MERGE_PATCH },
            } as never),
          )
        : api.call<StudyGroup>(api.client.POST('/lms/schools/{school_id}/study-groups', { params: { path: { school_id: schoolId } }, body: v.body, headers: { ...scope.headers, ...intent } } as never)),
    {
      onSuccess: () => {
        void invalidate('study-groups');
        void invalidate('study-group');
      },
    },
  );
}

export function useDeleteStudyGroup(schoolId: string) {
  const { api, scope, invalidate } = useLmsContext();
  return useMutation({
    mutationFn: (id: string) =>
      api.call<void>(api.client.DELETE('/lms/schools/{school_id}/study-groups/{study_group_id}', { params: { path: { school_id: schoolId, study_group_id: id } }, headers: scope.headers } as never)),
    onSuccess: () => void invalidate('study-groups'),
  });
}
