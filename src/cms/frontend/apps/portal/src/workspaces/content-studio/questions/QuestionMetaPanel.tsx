// QuestionMetaPanel (F053-AC-04, AC-14, AC-32; data-model "Metadata"): grade levels (several, 1–9), subject, topic,
// LOs, difficulty, original language, tags, spoken-stem exemption (grades 1–3), worked example and change note.
// Every text is capped by its OpenAPI maxLength (ADR-0041).
import { Form, Input, InputNumber } from 'antd';
import { useTranslation } from 'react-i18next';
import { useObjectives, useSubjects, useTopics } from '../curriculum/useCurriculum';
import { LIMITS } from '../shared/config';
import { needsSpokenStem } from '../shared/gates';
import type { QuestionMeta } from './useQuestion';
import { EfficacyInstrumentField } from './EfficacyInstrumentField';
import { WorkedExampleRefPicker } from './WorkedExampleRefPicker';
import { SearchSelect } from '@eduworld/ops-ui';

export interface QuestionMetaPanelProps {
  value: QuestionMeta;
  onChange: (v: QuestionMeta) => void;
  readOnly: boolean;
  /** The original language is chosen once, when the question is created. */
  languageLocked: boolean;
  errors: Readonly<Record<string, string>>;
}

const GRADES = [1, 2, 3, 4, 5, 6, 7, 8, 9];

/** Field ids (jump targets, error keys). */
const F = {
  grades: 'grade_levels',
  subject: 'subject_id',
  topic: 'topic_id',
  objectives: 'objective_ids',
  difficulty: 'difficulty',
  tags: 'tags',
  exempt: 'audio_exempt_reason',
  worked: 'worked_example_ref',
  note: 'change_note',
} as const;

export function QuestionMetaPanel({ value, onChange, readOnly, languageLocked, errors }: QuestionMetaPanelProps) {
  const { t, i18n } = useTranslation();
  const set = (patch: Partial<QuestionMeta>) => onChange({ ...value, ...patch });
  const subjects = useSubjects();
  const grade = value.grade_levels?.length === 1 ? value.grade_levels[0] : undefined;
  const topics = useTopics({ subject_id: value.subject_id, ...(grade ? { grade_level: grade } : {}) }, !!value.subject_id);
  const objectives = useObjectives({ topic_id: value.topic_id }, !!value.topic_id);
  const label = (n: { vi?: string; en?: string }) => (i18n.language.startsWith('en') ? (n.en ?? n.vi) : (n.vi ?? n.en)) ?? '';
  const item = (field: string) => ({ validateStatus: errors[field] ? ('error' as const) : undefined, help: errors[field], 'data-field': field });
  return (
    <Form layout="vertical" component="div" disabled={readOnly} aria-label={t('content_studio.question.meta')}>
      <Form.Item label={t('content_studio.question.gradeLevels')} required {...item(F.grades)}>
        <SearchSelect mode="multiple" aria-label={t('content_studio.question.gradeLevels')} value={value.grade_levels ?? []} maxCount={9} options={GRADES.map((g) => ({ value: g, label: t('content_studio.curriculum.grade', { grade: g }) }))} onChange={(v: number[]) => set({ grade_levels: v })} />
      </Form.Item>
      <Form.Item label={t('content_studio.question.subject')} {...item(F.subject)}>
        <SearchSelect aria-label={t('content_studio.question.subject')} allowClear value={value.subject_id} loading={subjects.isFetching} options={(subjects.data?.data ?? []).map((s) => ({ value: s.subject_id, label: label(s.name) }))} onChange={(v?: string) => set({ subject_id: v, topic_id: undefined, objective_ids: [] })} />
      </Form.Item>
      <Form.Item label={t('content_studio.question.topic')} {...item(F.topic)}>
        <SearchSelect aria-label={t('content_studio.question.topic')} allowClear value={value.topic_id} loading={topics.isFetching} options={(topics.data?.data ?? []).map((x) => ({ value: x.topic_id, label: `${x.code ?? ''} ${x.name}`.trim() }))} onChange={(v?: string) => set({ topic_id: v, objective_ids: [] })} />
      </Form.Item>
      <Form.Item label={t('content_studio.question.objectives')} {...item(F.objectives)}>
        <SearchSelect mode="multiple" aria-label={t('content_studio.question.objectives')} value={value.objective_ids ?? []} maxCount={50} loading={objectives.isFetching} options={(objectives.data?.data ?? []).map((o) => ({ value: o.objective_id, label: `${o.code} — ${o.description}` }))} onChange={(v: string[]) => set({ objective_ids: v })} />
      </Form.Item>
      <Form.Item label={t('content_studio.question.difficulty')} {...item(F.difficulty)}>
        <InputNumber min={1} max={5} value={value.difficulty} onChange={(v) => set({ difficulty: v ?? undefined })} />
      </Form.Item>
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
      <Form.Item label={t('content_studio.question.tags')} {...item(F.tags)}>
        <SearchSelect mode="tags" open={false} aria-label={t('content_studio.question.tags')} value={value.tags ?? []} maxCount={LIMITS.tags} onChange={(v: string[]) => set({ tags: v.map((x) => x.slice(0, LIMITS.tag)) })} />
      </Form.Item>
      {needsSpokenStem(value.grade_levels) && (
        <Form.Item label={t('content_studio.question.audioExempt')} extra={t('content_studio.question.audioExemptHelp')} {...item(F.exempt)}>
          <Input.TextArea aria-label={t('content_studio.question.audioExempt')} value={value.audio_exempt_reason ?? ''} maxLength={LIMITS.audioExemptReason} showCount autoSize={{ minRows: 1, maxRows: 4 }} onChange={(e) => set({ audio_exempt_reason: e.target.value })} />
        </Form.Item>
      )}
      <Form.Item label={t('content_studio.question.workedExample')} {...item(F.worked)}>
        <WorkedExampleRefPicker value={value.worked_example_ref} onChange={(v) => set({ worked_example_ref: v })} objectiveIds={value.objective_ids ?? []} subjectId={value.subject_id} disabled={readOnly} />
      </Form.Item>
      <EfficacyInstrumentField kind="question" actionKey="action.content_studio.question_bank.edit" checked={!!value.efficacy_instrument} disabled={readOnly} onChange={(v) => set({ efficacy_instrument: v })} />
      <Form.Item label={t('content_studio.question.changeNote')} {...item(F.note)}>
        <Input.TextArea aria-label={t('content_studio.question.changeNote')} value={value.change_note ?? ''} maxLength={LIMITS.changeNote} showCount autoSize={{ minRows: 1, maxRows: 4 }} onChange={(e) => set({ change_note: e.target.value })} />
      </Form.Item>
    </Form>
  );
}
