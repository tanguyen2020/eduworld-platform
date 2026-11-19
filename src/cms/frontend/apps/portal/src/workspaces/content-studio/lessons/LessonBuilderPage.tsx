// LessonBuilderPage (F054-AC-21, AC-23, ADMIN_PORTAL §3.2): Lesson Builder on the shared builder frame — metadata
// (one grade, one topic, LOs), estimated minutes, completion rule, the block list; formulas of the blocks rendered in
// the browser and sent as `math_assets` (ADR-0055, ≤ maxItems); formula errors block the save.
import { buildMathAssets, collectMath } from '@eduworld/ops-ui/content';
import { Flex, Form, InputNumber, Select } from 'antd';
import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { DocMetaFields } from '../builders/DocMetaFields';
import { DocumentEditorPage, type BuilderArgs } from '../builders/DocumentEditorPage';
import type { DocAuthoring, LessonAuthoring } from '../builders/useDocument';
import { LessonBlockList } from './LessonBlockList';
import { emptyLesson, fromLesson, LESSON_LIMITS, lessonIssues, mathPayload, toLessonBody, type CompletionRule, type LessonState } from './lessonModel';

const RULES: CompletionRule[] = ['all_required_activities', 'any_activity', 'view_only'];

/** Authoring body with the formulas rendered (a formula error rejects the save with MATH_RENDER_FAILED). */
export async function lessonBody(s: LessonState): Promise<Record<string, unknown>> {
  const built = await buildMathAssets(collectMath(mathPayload(s), s.content_language));
  if (built.errors.length) throw new Error(`MATH_RENDER_FAILED: ${built.errors.map((e) => e.latex).join(', ')}`);
  return toLessonBody(s, built.assets.slice(0, LESSON_LIMITS.mathAssets));
}

function LessonBody({ state, onChange, readOnly, errors, id }: BuilderArgs<LessonState> & { id?: string }) {
  const { t } = useTranslation();
  return (
    <Flex vertical gap="middle">
      <DocMetaFields value={state} onChange={onChange} readOnly={readOnly} languageLocked={!!id} singleGrade singleTopic errors={errors} />
      <Form layout="vertical" component="div" disabled={readOnly}>
        <Flex gap="middle" wrap>
          <Form.Item label={t('content_studio.lesson.minutes')}>
            <InputNumber aria-label={t('content_studio.lesson.minutes')} min={1} max={180} value={state.estimated_minutes} onChange={(v) => onChange({ ...state, estimated_minutes: v ?? undefined })} />
          </Form.Item>
          <Form.Item label={t('content_studio.lesson.completion')} data-field="completion_rule">
            <Select aria-label={t('content_studio.lesson.completion')} value={state.completion_rule} options={RULES.map((r) => ({ value: r, label: t(`content_studio.lesson.completionRules.${r}`) }))} onChange={(completion_rule: CompletionRule) => onChange({ ...state, completion_rule })} />
          </Form.Item>
        </Flex>
      </Form>
      <LessonBlockList state={state} onChange={onChange} readOnly={readOnly} />
    </Flex>
  );
}

export function LessonBuilderPage({ id }: { id?: string }) {
  const { t } = useTranslation();
  const blockers = useCallback((s: LessonState) => lessonIssues(s, t), [t]);
  return (
    <DocumentEditorPage<LessonState>
      type="lesson"
      id={id}
      title={{ create: t('content_studio.lesson.newTitle'), edit: t('content_studio.lesson.editTitle') }}
      empty={emptyLesson}
      fromServer={(d: DocAuthoring) => fromLesson(d as LessonAuthoring)}
      toBody={lessonBody}
      blockers={blockers}
      body={(a) => <LessonBody {...a} id={id} />}
    />
  );
}
