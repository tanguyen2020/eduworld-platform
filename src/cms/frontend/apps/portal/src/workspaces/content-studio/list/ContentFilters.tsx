// ContentFilters (F054-AC-24, AC-29): filters of a content list — keyword `q` (≤ 200, the server ignores diacritics),
// subject, grade, topic, LO, status, language and translation status (OQ F054-2); for questions also type, difficulty
// and source (own / assigned / platform).
// Values only; the query is built by filtersToQuery.
import { Flex, Input, Select } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useObjectives, useSubjects, useTopics } from '../curriculum/useCurriculum';
import { LIMITS } from '../shared/config';
import { CONTENT_STATUSES } from '../shared/contentTypes';
import { CONTENT_LANGUAGES, TRANSLATION_STATUSES, type ContentLanguage, type TranslationStatus } from '../shared/fieldSchemas';
import type { ContentFiltersValue, ListType } from './useContentList';

const GRADES = [1, 2, 3, 4, 5, 6, 7, 8, 9];
const Q_TYPES = ['single_choice', 'multiple_choice', 'true_false', 'fill_in_blank', 'short_answer', 'numeric', 'matching', 'ordering', 'drag_drop', 'image_based', 'interactive'];

export function ContentFilters({ type, value, onChange, platform }: { type: ListType; value: ContentFiltersValue; onChange: (v: ContentFiltersValue) => void; platform: boolean }) {
  const { t, i18n } = useTranslation();
  const [q, setQ] = useState(value.q ?? '');
  const subjects = useSubjects();
  const topics = useTopics({ subject_id: value.subject_id, ...(value.grade_level ? { grade_level: value.grade_level } : {}) }, !!value.subject_id);
  const objectives = useObjectives(value.topic_id ? { topic_id: value.topic_id } : { subject_id: value.subject_id }, !!value.subject_id);
  const label = (n: { vi?: string; en?: string }) => (i18n.language.startsWith('en') ? (n.en ?? n.vi) : (n.vi ?? n.en)) ?? '';
  const set = (patch: Partial<ContentFiltersValue>) => onChange({ ...value, ...patch });
  const sel = { allowClear: true, style: { minWidth: 160 } } as const;
  return (
    <Flex gap="small" wrap align="center" data-testid="content-filters">
      <Input.Search aria-label={t('content_studio.list.search')} placeholder={t('content_studio.list.search')} maxLength={LIMITS.search} value={q} onChange={(e) => setQ(e.target.value)} onSearch={(v) => set({ q: v.trim() || undefined })} allowClear style={{ maxWidth: 320 }} />
      <Select {...sel} aria-label={t('content_studio.question.subject')} placeholder={t('content_studio.question.subject')} value={value.subject_id} options={(subjects.data?.data ?? []).map((s) => ({ value: s.subject_id, label: label(s.name) }))} onChange={(v?: string) => set({ subject_id: v, topic_id: undefined, objective_id: undefined })} />
      <Select {...sel} aria-label={t('content_studio.list.grade')} placeholder={t('content_studio.list.grade')} value={value.grade_level} options={GRADES.map((g) => ({ value: g, label: t('content_studio.curriculum.grade', { grade: g }) }))} onChange={(v?: number) => set({ grade_level: v })} />
      <Select {...sel} aria-label={t('content_studio.question.topic')} placeholder={t('content_studio.question.topic')} value={value.topic_id} options={(topics.data?.data ?? []).map((x) => ({ value: x.topic_id, label: `${x.code ?? ''} ${x.name}`.trim() }))} onChange={(v?: string) => set({ topic_id: v, objective_id: undefined })} />
      <Select {...sel} aria-label={t('content_studio.list.objective')} placeholder={t('content_studio.list.objective')} value={value.objective_id} options={(objectives.data?.data ?? []).map((o) => ({ value: o.objective_id, label: o.code }))} onChange={(v?: string) => set({ objective_id: v })} />
      <Select {...sel} aria-label={t('content_studio.release.col.status')} placeholder={t('content_studio.release.col.status')} value={value.status} options={CONTENT_STATUSES.map((s) => ({ value: s, label: t(`content_studio.status.${s}`) }))} onChange={(v?: string) => set({ status: v })} />
      <Select {...sel} aria-label={t('content_studio.list.language')} placeholder={t('content_studio.list.language')} value={value.content_language} options={CONTENT_LANGUAGES.map((l) => ({ value: l, label: t(`content_studio.preview.${l}`) }))} onChange={(v?: ContentLanguage) => set({ content_language: v })} />
      <Select {...sel} aria-label={t('content_studio.list.translationStatus')} placeholder={t('content_studio.list.translationStatus')} value={value.translation_status} options={TRANSLATION_STATUSES.map((x) => ({ value: x, label: t(`content_studio.translation.${x}`) }))} onChange={(v?: TranslationStatus) => set({ translation_status: v })} />
      {type === 'question' && (
        <>
          <Select {...sel} aria-label={t('content_studio.list.type')} placeholder={t('content_studio.list.type')} value={value.type} options={Q_TYPES.map((x) => ({ value: x, label: t(`ops-ui:question.types.${x}`) }))} onChange={(v?: string) => set({ type: v })} />
          <Select {...sel} mode="multiple" aria-label={t('content_studio.question.difficulty')} placeholder={t('content_studio.question.difficulty')} value={value.difficulty ?? []} options={[1, 2, 3, 4, 5].map((d) => ({ value: d, label: String(d) }))} onChange={(v: number[]) => set({ difficulty: v })} />
          <Select
            {...sel}
            aria-label={t('content_studio.list.source')}
            placeholder={t('content_studio.list.source')}
            value={value.source}
            options={(platform ? (['own', 'assigned', 'platform'] as const) : (['own', 'assigned'] as const)).map((s) => ({ value: s, label: t(`content_studio.list.sources.${s}`) }))}
            onChange={(v?: ContentFiltersValue['source']) => set({ source: v })}
          />
        </>
      )}
    </Flex>
  );
}
