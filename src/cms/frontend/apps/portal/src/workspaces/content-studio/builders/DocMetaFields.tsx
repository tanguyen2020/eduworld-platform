// DocMetaFields (F054 US4): metadata shared by the builders — bilingual title (LearningName ≤ 255 per language,
// original language first), original language (chosen at creation), subject, grade(s), topic(s), LOs (curriculum
// lists of F053). Every text is capped by its OpenAPI maxLength (ADR-0041). The topics come from the current curriculum
// version of subject × khối (F053 T066, OQ F053-9); none → the picker explains why.
import { BilingualField } from '@eduworld/ops-ui/content';
import { Form } from 'antd';
import { useTranslation } from 'react-i18next';
import { topicPickerHelp, useCurrentTopics, useObjectives, useSubjects } from '../curriculum/useCurriculum';
import { LIMITS } from '../shared/config';
import { SearchSelect } from '@eduworld/ops-ui';

export interface DocMeta {
  title: { vi?: string; en?: string };
  content_language: 'vi' | 'en';
  subject_id?: string;
  grade_levels: number[];
  topic_ids: string[];
  objective_ids: string[];
}

const GRADES = [1, 2, 3, 4, 5, 6, 7, 8, 9];

/** Field ids (server error keys, jump targets). */
const F = { subject: 'subject_id', grades: 'grade_levels', topics: 'topic_ids', objectives: 'objective_ids' } as const;

export function DocMetaFields<T extends DocMeta>({ value, onChange, readOnly, languageLocked, singleGrade = false, singleTopic = false, errors = {} }: { value: T; onChange: (v: T) => void; readOnly: boolean; languageLocked: boolean; singleGrade?: boolean; singleTopic?: boolean; errors?: Readonly<Record<string, string>> }) {
  const { t, i18n } = useTranslation();
  const subjects = useSubjects();
  const grade = value.grade_levels.length === 1 ? value.grade_levels[0] : undefined;
  const current = useCurrentTopics(value.subject_id, grade);
  const topics = current.topics;
  const topicHelp = topicPickerHelp(t, current, i18n.language);
  const objectives = useObjectives(value.topic_ids.length === 1 ? { topic_id: value.topic_ids[0] } : { subject_id: value.subject_id }, !!value.subject_id);
  const label = (n: { vi?: string; en?: string }) => (i18n.language.startsWith('en') ? (n.en ?? n.vi) : (n.vi ?? n.en)) ?? '';
  const set = (patch: Partial<DocMeta>) => onChange({ ...value, ...patch });
  const err = (f: string) => ({ validateStatus: errors[f] || errors[`/${f}`] ? ('error' as const) : undefined, help: errors[f] ?? errors[`/${f}`] });
  return (
    <Form layout="vertical" component="div" disabled={readOnly}>
      <BilingualField id="title" label={t('content_studio.builder.title')} value={value.title} onChange={(v) => set({ title: (v ?? {}) as DocMeta['title'] })} contentLanguage={value.content_language} maxLength={LIMITS.name} required readOnly={readOnly} error={errors.title ?? errors['/title']} />
      <Form.Item label={t('content_studio.question.contentLanguage')}>
        <SearchSelect
          aria-label={t('content_studio.question.contentLanguage')}
          value={value.content_language}
          disabled={readOnly || languageLocked}
          options={[
            { value: 'vi', label: t('content_studio.preview.vi') },
            { value: 'en', label: t('content_studio.preview.en') },
          ]}
          onChange={(v: 'vi' | 'en') => set({ content_language: v })}
        />
      </Form.Item>
      <Form.Item label={t('content_studio.question.subject')} {...err(F.subject)}>
        <SearchSelect aria-label={t('content_studio.question.subject')} allowClear value={value.subject_id} loading={subjects.isFetching} options={(subjects.data?.data ?? []).map((s) => ({ value: s.subject_id, label: label(s.name) }))} onChange={(v?: string) => set({ subject_id: v, topic_ids: [], objective_ids: [] })} />
      </Form.Item>
      <Form.Item label={t('content_studio.question.gradeLevels')} {...err(F.grades)}>
        <SearchSelect
          mode={singleGrade ? undefined : 'multiple'}
          aria-label={t('content_studio.question.gradeLevels')}
          value={singleGrade ? value.grade_levels[0] : value.grade_levels}
          options={GRADES.map((g) => ({ value: g, label: t('content_studio.curriculum.grade', { grade: g }) }))}
          onChange={(v: number | number[] | undefined) => set({ grade_levels: v === undefined ? [] : Array.isArray(v) ? v : [v] })}
          allowClear={singleGrade}
        />
      </Form.Item>
      <Form.Item label={t('content_studio.question.topic')} {...err(F.topics)} extra={topicHelp}>
        <SearchSelect
          mode={singleTopic ? undefined : 'multiple'}
          allowClear
          aria-label={t('content_studio.question.topic')}
          value={singleTopic ? value.topic_ids[0] : value.topic_ids}
          loading={current.loading}
          notFoundContent={current.none ? topicHelp : undefined}
          options={(topics.data?.data ?? []).map((x) => ({ value: x.topic_id, label: `${x.code ?? ''} ${x.name}`.trim() }))}
          onChange={(v: string | string[] | undefined) => set({ topic_ids: v === undefined ? [] : Array.isArray(v) ? v : [v] })}
        />
      </Form.Item>
      <Form.Item label={t('content_studio.question.objectives')} {...err(F.objectives)}>
        <SearchSelect mode="multiple" aria-label={t('content_studio.question.objectives')} value={value.objective_ids} maxCount={50} loading={objectives.isFetching} options={(objectives.data?.data ?? []).map((o) => ({ value: o.objective_id, label: `${o.code} — ${o.description}` }))} onChange={(v: string[]) => set({ objective_ids: v })} />
      </Form.Item>
    </Form>
  );
}
