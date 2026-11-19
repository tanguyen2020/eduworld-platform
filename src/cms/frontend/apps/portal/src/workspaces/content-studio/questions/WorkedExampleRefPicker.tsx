// WorkedExampleRefPicker (F053-AC-32, FR-CNT-020, CONTENT_MODEL §9.2): the question's `worked_example_ref` =
// {content_item_id, block_id} of a `worked_example` block of a lesson that shares one of the question's LOs.
// Lessons come from listCmsLessons (subject / grade filter) and are kept when their objective_ids meet the question's;
// the blocks are read with getCmsLesson.
import { Select, Space } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { usePortalRuntime } from '../../../app/runtime';
import { useSession } from '../../../session/SessionProvider';
import type { components } from '../../../shared/api/schema.gen';

type Lesson = components['schemas']['LessonAuthoring'];

export interface WorkedExampleRef {
  content_item_id: string;
  block_id: string;
}

/** Lessons sharing at least one LO with the question. */
export function lessonsForObjectives(lessons: readonly Lesson[], objectiveIds: readonly string[]): Lesson[] {
  if (objectiveIds.length === 0) return [];
  return lessons.filter((l) => (l.objective_ids ?? []).some((o) => objectiveIds.includes(o)));
}

/** `worked_example` blocks of a lesson (block `id`, CONTENT_MODEL §9.2 / lesson.schema.json; older drafts kept it in `content.id`). */
export function workedExampleBlocks(lesson: Lesson | undefined): { id: string; label: string }[] {
  return (lesson?.blocks ?? [])
    .filter((b) => b.kind === 'worked_example')
    .map((b, i) => {
      const id = (b as { id?: string }).id ?? ((b.content ?? {}) as { id?: string }).id ?? '';
      return { id, label: `${i + 1}. ${id}` };
    })
    .filter((b) => b.id);
}

export interface WorkedExampleRefPickerProps {
  value?: WorkedExampleRef;
  onChange: (v: WorkedExampleRef | undefined) => void;
  objectiveIds: readonly string[];
  subjectId?: string;
  disabled?: boolean;
}

export function WorkedExampleRefPicker({ value, onChange, objectiveIds, subjectId, disabled }: WorkedExampleRefPickerProps) {
  const { t, i18n } = useTranslation();
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  const [lessonId, setLessonId] = useState<string | undefined>(value?.content_item_id);
  const lessons = useQuery({
    queryKey: ['content-studio', 'lessons', scopeKey, { subjectId }],
    queryFn: () => api.call<{ data: Lesson[] }>(api.client.GET('/cms/learning/lessons', { params: { query: { ...(subjectId ? { subject_id: subjectId } : {}), limit: 100 } as never } })),
    enabled: objectiveIds.length > 0,
  });
  const lesson = useQuery({
    queryKey: ['content-studio', 'lesson', scopeKey, lessonId],
    queryFn: () => api.call<Lesson>(api.client.GET('/cms/learning/lessons/{lesson_id}', { params: { path: { lesson_id: lessonId! } } })),
    enabled: !!lessonId,
  });
  const candidates = lessonsForObjectives(lessons.data?.data ?? [], objectiveIds);
  const title = (l: Lesson) => (i18n.language.startsWith('en') ? (l.title.en ?? l.title.vi) : (l.title.vi ?? l.title.en)) ?? l.content_id;
  return (
    <Space wrap>
      <Select
        aria-label={t('content_studio.question.workedExampleLesson')}
        placeholder={objectiveIds.length ? t('content_studio.question.workedExampleLesson') : t('content_studio.question.workedExampleNeedsLo')}
        allowClear
        disabled={disabled || objectiveIds.length === 0}
        loading={lessons.isFetching}
        value={lessonId}
        onChange={(v?: string) => {
          setLessonId(v);
          if (!v) onChange(undefined);
        }}
        options={candidates.map((l) => ({ value: l.content_id, label: title(l) }))}
      />
      <Select
        aria-label={t('content_studio.question.workedExampleBlock')}
        placeholder={t('content_studio.question.workedExampleBlock')}
        allowClear
        disabled={disabled || !lessonId}
        loading={lesson.isFetching}
        value={value?.content_item_id === lessonId ? value?.block_id : undefined}
        onChange={(b?: string) => onChange(b && lessonId ? { content_item_id: lessonId, block_id: b } : undefined)}
        options={workedExampleBlocks(lesson.data).map((b) => ({ value: b.id, label: b.label }))}
      />
    </Space>
  );
}
