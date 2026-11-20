// Content picker of the wizard and the path builder (F055 T017, F055-AC-01, AC-22): published, server-graded content
// only — activities of a type enabled in the Release (listActivityTypes), question sets, assessment blueprints,
// lessons (listCms*, status=published) and quests / mini-games of the game catalog (listGameActivities, read-only:
// no game configuration in workspace C). Written response / speaking / project types do not exist in the contract.
import { useQuery } from '@tanstack/react-query';
import type { ContentChoice } from '../model/assignmentForm';
import type { ContentType } from '../model/fieldSchemas';
import { LMS_PAGE_SIZE, lmsKey, useLmsContext, type DataPage, type S } from './lmsCommon';

export type PickerTab = 'activity' | 'question_set' | 'assessment_blueprint' | 'lesson' | 'game';

export interface PickerFilters {
  gradeLevel?: number;
  subjectId?: string;
  objectiveId?: string;
}

const CMS_PATH: Record<Exclude<PickerTab, 'game' | 'activity'>, string> = {
  question_set: '/cms/learning/question-sets',
  assessment_blueprint: '/cms/learning/assessment-blueprints',
  lesson: '/cms/learning/lessons',
};

type Named = { vi?: string; en?: string } | string | undefined;
const nameOf = (n: Named, locale: string): string => (typeof n === 'string' ? n : (locale.startsWith('en') && n?.en) || n?.vi || '');

/** Activity types the Release enables (status `enabled`). */
export function useEnabledActivityTypes(enabled = true) {
  const { api, scope } = useLmsContext();
  return useQuery({
    queryKey: lmsKey('activity-types', scope),
    queryFn: () => api.call<{ data: S['ActivityTypeInfo'][] }>(api.client.GET('/cms/learning/activity-types', { headers: scope.headers } as never)),
    select: (r) => r.data.filter((t) => t.status === 'enabled').map((t) => t.activity_type as string),
    enabled: scope.ready && enabled,
  });
}

/** One page of choices of a tab (published only). */
export function usePickerChoices(tab: PickerTab, f: PickerFilters, locale: string, enabled = true) {
  const { api, scope } = useLmsContext();
  const types = useEnabledActivityTypes(enabled && tab === 'activity');
  const query = {
    limit: LMS_PAGE_SIZE,
    ...(f.gradeLevel ? { grade_level: f.gradeLevel } : {}),
    ...(f.subjectId ? { subject_id: f.subjectId } : {}),
  };
  return useQuery({
    queryKey: lmsKey('picker', scope, tab, f, tab === 'activity' ? types.data : null),
    enabled: scope.ready && enabled && (tab !== 'activity' || !!types.data),
    queryFn: async (): Promise<ContentChoice[]> => {
      if (tab === 'game') {
        const r = await api.call<DataPage<S['GameActivityCatalogItem']>>(
          api.client.GET('/lms/catalog/game-activities', { params: { query: { ...query, ...(f.objectiveId ? { objective_id: f.objectiveId } : {}) } }, headers: scope.headers } as never),
        );
        return r.data.map((g) => ({ contentType: g.content_type as ContentType, contentId: g.content_id ?? g.content_version_id, versionId: g.content_version_id, title: g.title }));
      }
      if (tab === 'activity') {
        const r = await api.call<DataPage<S['ActivityDefinition']>>(
          api.client.GET('/cms/learning/activities', { params: { query: { ...query, status: 'published' } }, headers: scope.headers } as never),
        );
        const allowed = new Set(types.data ?? []);
        return r.data
          .filter((a) => a.status === 'published' && allowed.has(a.activity_type))
          .map((a) => ({ contentType: 'activity' as const, contentId: a.activity_id, versionId: a.activity_version_id, title: nameOf(a.title as Named, locale) }));
      }
      const r = await api.call<DataPage<S['ContentVersion'] & { title?: Named; name?: Named }>>(
        api.client.GET(CMS_PATH[tab] as '/cms/learning/lessons', { params: { query: { ...query, status: 'published', ...(f.objectiveId ? { objective_id: f.objectiveId } : {}) } }, headers: scope.headers } as never),
      );
      return r.data
        .filter((c) => c.status === 'published')
        .map((c) => ({ contentType: tab, contentId: c.content_id, versionId: c.content_version_id, title: nameOf(c.title ?? c.name, locale) }));
    },
  });
}
