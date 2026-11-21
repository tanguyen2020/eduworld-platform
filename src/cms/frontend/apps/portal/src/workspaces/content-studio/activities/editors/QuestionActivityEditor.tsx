// QuestionActivityEditor (F058 T022, F058-AC-03, AC-04): the config of `quiz` and `interactive_exercise` — question
// source (fixed published questions via the Question Bank picker, a question set pinned or current, or a selection
// rule by topic / LO / difficulty / type / count), allowed question types of the schema, evaluation strategies limited
// to the registry list of the type, max score, pass threshold, FeedbackTiming, completion rule ("Hoàn thành khác Đạt"),
// and the timer of a timed quiz. Display and intents only: module learning validates and grades (AGENTS R1, R5).
import { DeleteOutlined } from '@ant-design/icons';
import { Alert, Button, Card, Checkbox, Flex, Form, Input, InputNumber, List, Segmented, Select, Switch, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { topicPickerHelp, useCurrentTopics, useObjectives } from '../../curriculum/useCurriculum';
import { QuestionPicker } from '../../question-sets/SelectionRuleEditor';
import { allowedTypesOf, COMPLETION_MODES, FEEDBACK_TIMINGS, type ActivityType, type Config } from '../activityModel';
import type { ActivityEditorProps } from '../editorRegistry';
import { canAddQuestion, MAX_QUESTIONS, offeredStrategies, sourceIssues, type QuestionSource, type SourceKind } from '../validators/questionSourceRules';

const obj = (v: unknown): Record<string, unknown> => (v && typeof v === 'object' ? (v as Record<string, unknown>) : {});

/** Default config of a question-based type (allowed types = schema default; immediate feedback; submit completion). */
export function questionConfig(type: ActivityType, strategies: string[] = ['auto']): Config {
  return {
    content: { question_source: { kind: 'questions', questions: [] }, allowed_question_types: allowedTypesOf(type).defaults },
    evaluation: { strategies, max_score: 10 },
    feedback: { timing: 'immediate' },
    completion_rule: { mode: 'submit' },
  };
}

export function QuestionActivityEditor({ config, onChange, readOnly, errors, typeInfo, subjectId, gradeLevels }: ActivityEditorProps) {
  const { t, i18n } = useTranslation();
  const type = (typeInfo?.activity_type ?? 'quiz') as ActivityType;
  const content = obj(config.content);
  const source = (content.question_source as QuestionSource | undefined) ?? { kind: 'questions', questions: [] };
  const allowed = (content.allowed_question_types as string[] | undefined) ?? allowedTypesOf(type).defaults;
  const evaluation = obj(config.evaluation);
  const feedback = obj(config.feedback);
  const completion = obj(config.completion_rule);
  const timed = content.timed as { total_seconds: number; show_timer?: boolean } | undefined;
  const [picking, setPicking] = useState(false);
  const [refused, setRefused] = useState<string | null>(null);
  // F053 T066: topics of the current curriculum version of subject × khối (one grade → that grade, else the subject)
  const currentTopics = useCurrentTopics(subjectId, gradeLevels.length === 1 ? gradeLevels[0] : undefined, source.kind === 'selection');
  const topics = currentTopics.topics;
  const topicHelp = topicPickerHelp(t, currentTopics, i18n.language);
  const objectives = useObjectives({ subject_id: subjectId }, !!subjectId && source.kind === 'selection');
  const setContent = (patch: Record<string, unknown>) => onChange({ ...config, content: { ...content, ...patch } });
  const setSource = (s: QuestionSource) => setContent({ question_source: s });
  const err = (p: string) => errors[p] ?? errors[`${p}/`];
  const issues = sourceIssues(source, allowed);
  const strategies = offeredStrategies(typeInfo?.evaluation_strategies);
  const selection = source.selection ?? { count: 10 };

  const switchKind = (kind: SourceKind) => {
    if (kind === 'questions') setSource({ kind, questions: source.questions ?? [] });
    else if (kind === 'question_set') setSource({ kind, question_set_id: source.question_set_id, question_set_version: source.question_set_version ?? null });
    else setSource({ kind, selection });
  };

  return (
    <Flex vertical gap="middle" data-testid="question-activity-editor">
      <Card size="small" title={t('content_studio.activity.source.title')}>
        <Form layout="vertical" component="div" disabled={readOnly}>
          <Segmented
            aria-label={t('content_studio.activity.source.kind')}
            value={source.kind}
            disabled={readOnly}
            onChange={(v) => switchKind(v as SourceKind)}
            options={(['questions', 'question_set', 'selection'] as SourceKind[]).map((k) => ({ value: k, label: t(`content_studio.activity.source.${k}`) }))}
          />
          {source.kind === 'questions' && (
            <Flex vertical gap="small">
              <List
                size="small"
                bordered
                data-testid="activity-questions"
                locale={{ emptyText: t('content_studio.activity.source.empty') }}
                dataSource={source.questions ?? []}
                renderItem={(q, i) => (
                  <List.Item
                    actions={
                      readOnly
                        ? []
                        : [
                            <Button
                              key="rm"
                              size="small"
                              icon={<DeleteOutlined />}
                              aria-label={t('content_studio.activity.source.remove', { n: i + 1 })}
                              onClick={() => setSource({ ...source, questions: (source.questions ?? []).filter((_, k) => k !== i) })}
                            />,
                          ]
                    }
                  >
                    {t('content_studio.activity.source.item', { n: i + 1, id: q.question_version_id.slice(-8), type: q.type ? t(`ops-ui:question.types.${q.type}`) : '' })}
                  </List.Item>
                )}
              />
              <Typography.Text type="secondary">{t('content_studio.activity.source.counter', { n: (source.questions ?? []).length, max: MAX_QUESTIONS })}</Typography.Text>
              {refused && <Alert type="warning" showIcon message={refused} closable onClose={() => setRefused(null)} />}
              {!readOnly && <Button onClick={() => setPicking(true)}>{t('content_studio.activity.source.add')}</Button>}
              <QuestionPicker
                open={picking}
                onClose={() => setPicking(false)}
                onPick={(q) => {
                  const id = q.content_version_id ?? '';
                  const verdict = canAddQuestion(source, { question_version_id: id, type: q.type ?? '' }, allowed);
                  if (verdict !== 'ok') {
                    setRefused(t(`content_studio.activity.source.refused.${verdict}`, { type: t(`ops-ui:question.types.${q.type}`) }));
                    return;
                  }
                  setRefused(null);
                  setSource({ ...source, questions: [...(source.questions ?? []), { question_version_id: id, type: q.type }] });
                }}
              />
            </Flex>
          )}
          {source.kind === 'question_set' && (
            <Flex vertical gap="small">
              <Form.Item label={t('content_studio.activity.source.setId')} validateStatus={err('/config/content/question_source/question_set_id') ? 'error' : undefined} help={err('/config/content/question_source/question_set_id')}>
                <Input aria-label={t('content_studio.activity.source.setId')} maxLength={36} value={source.question_set_id ?? ''} onChange={(e) => setSource({ ...source, question_set_id: e.target.value.trim() || undefined })} />
              </Form.Item>
              <Form.Item label={t('content_studio.activity.source.setVersion')} extra={t('content_studio.activity.source.setVersionHelp')}>
                <InputNumber aria-label={t('content_studio.activity.source.setVersion')} min={1} value={source.question_set_version ?? null} onChange={(v) => setSource({ ...source, question_set_version: v ?? null })} />
              </Form.Item>
            </Flex>
          )}
          {source.kind === 'selection' && (
            <Flex vertical gap="small">
              <Form.Item label={t('content_studio.question.topic')} extra={topicHelp}>
                <Select mode="multiple" aria-label={t('content_studio.question.topic')} maxCount={20} loading={currentTopics.loading} notFoundContent={currentTopics.none ? topicHelp : undefined} value={selection.topic_ids ?? []} options={(topics.data?.data ?? []).map((x) => ({ value: x.topic_id, label: x.name }))} onChange={(v: string[]) => setSource({ ...source, selection: { ...selection, topic_ids: v } })} />
              </Form.Item>
              <Form.Item label={t('content_studio.question.objectives')}>
                <Select mode="multiple" aria-label={t('content_studio.question.objectives')} maxCount={20} value={selection.objective_ids ?? []} options={(objectives.data?.data ?? []).map((o) => ({ value: o.objective_id, label: `${o.code} — ${o.description}` }))} onChange={(v: string[]) => setSource({ ...source, selection: { ...selection, objective_ids: v } })} />
              </Form.Item>
              <Flex gap="small">
                <Form.Item label={t('content_studio.activity.source.difficultyMin')}>
                  <InputNumber aria-label={t('content_studio.activity.source.difficultyMin')} min={1} max={5} value={selection.difficulty_min} onChange={(v) => setSource({ ...source, selection: { ...selection, difficulty_min: v ?? undefined } })} />
                </Form.Item>
                <Form.Item label={t('content_studio.activity.source.difficultyMax')}>
                  <InputNumber aria-label={t('content_studio.activity.source.difficultyMax')} min={1} max={5} value={selection.difficulty_max} onChange={(v) => setSource({ ...source, selection: { ...selection, difficulty_max: v ?? undefined } })} />
                </Form.Item>
                <Form.Item label={t('content_studio.activity.source.selectionCount')} validateStatus={issues.some((i) => i.key === 'source.count') ? 'error' : undefined}>
                  <InputNumber aria-label={t('content_studio.activity.source.selectionCount')} min={1} max={MAX_QUESTIONS} value={selection.count} onChange={(v) => setSource({ ...source, selection: { ...selection, count: v ?? 0 } })} />
                </Form.Item>
              </Flex>
              <Form.Item label={t('content_studio.questionSet.types')}>
                <Select mode="multiple" aria-label={t('content_studio.questionSet.types')} value={selection.question_types ?? []} options={allowed.map((x) => ({ value: x, label: t(`ops-ui:question.types.${x}`) }))} onChange={(v: string[]) => setSource({ ...source, selection: { ...selection, question_types: v.length ? v : undefined } })} />
              </Form.Item>
            </Flex>
          )}
          <Form.Item label={t('content_studio.activity.allowedTypes')}>
            <Select mode="multiple" aria-label={t('content_studio.activity.allowedTypes')} value={allowed} options={allowedTypesOf(type).values.map((x) => ({ value: x, label: t(`ops-ui:question.types.${x}`) }))} onChange={(v: string[]) => setContent({ allowed_question_types: v })} />
          </Form.Item>
          {type === 'quiz' && (
            <Flex gap="small" align="center">
              <Switch aria-label={t('content_studio.activity.timed')} checked={!!timed} onChange={(on) => setContent({ timed: on ? { total_seconds: 600, show_timer: true } : undefined })} />
              <Typography.Text>{t('content_studio.activity.timed')}</Typography.Text>
              {timed && (
                <InputNumber
                  aria-label={t('content_studio.activity.timedSeconds')}
                  min={10}
                  max={3600}
                  status={timed.total_seconds < 10 || timed.total_seconds > 3600 ? 'error' : undefined}
                  value={timed.total_seconds}
                  onChange={(v) => setContent({ timed: { ...timed, total_seconds: v ?? 0 } })}
                />
              )}
            </Flex>
          )}
        </Form>
      </Card>
      <Card size="small" title={t('content_studio.activity.grading.title')}>
        <Form layout="vertical" component="div" disabled={readOnly}>
          <Form.Item label={t('content_studio.activity.grading.strategies')} validateStatus={err('/config/evaluation/strategies') ? 'error' : undefined} help={err('/config/evaluation/strategies')}>
            <Checkbox.Group
              aria-label={t('content_studio.activity.grading.strategies')}
              value={(evaluation.strategies as string[] | undefined) ?? []}
              options={strategies.map((s) => ({ value: s, label: t(`content_studio.activity.strategy.${s}`, { defaultValue: s }) }))}
              onChange={(v) => onChange({ ...config, evaluation: { ...evaluation, strategies: v } })}
            />
          </Form.Item>
          <Flex gap="small">
            <Form.Item label={t('content_studio.activity.grading.maxScore')}>
              <InputNumber aria-label={t('content_studio.activity.grading.maxScore')} min={0} max={1000} value={evaluation.max_score as number | undefined} onChange={(v) => onChange({ ...config, evaluation: { ...evaluation, max_score: v ?? 0 } })} />
            </Form.Item>
            <Form.Item label={t('content_studio.activity.grading.passThreshold')}>
              <InputNumber aria-label={t('content_studio.activity.grading.passThreshold')} min={0} max={1} step={0.05} value={evaluation.pass_threshold as number | undefined} onChange={(v) => onChange({ ...config, evaluation: { ...evaluation, pass_threshold: v ?? undefined } })} />
            </Form.Item>
          </Flex>
          <Form.Item label={t('content_studio.activity.grading.feedback')}>
            <Select aria-label={t('content_studio.activity.grading.feedback')} value={feedback.timing as string | undefined} options={FEEDBACK_TIMINGS.map((x) => ({ value: x, label: t(`content_studio.activity.feedback.${x}`) }))} onChange={(v: string) => onChange({ ...config, feedback: { ...feedback, timing: v } })} />
          </Form.Item>
          <Form.Item label={t('content_studio.activity.grading.completion')} extra={t('content_studio.activity.grading.completionHelp')}>
            <Select aria-label={t('content_studio.activity.grading.completion')} value={completion.mode as string | undefined} options={COMPLETION_MODES.filter((m) => m === 'submit' || m === 'graded').map((x) => ({ value: x, label: t(`content_studio.activity.completion.${x}`) }))} onChange={(v: string) => onChange({ ...config, completion_rule: { ...completion, mode: v } })} />
          </Form.Item>
          <Typography.Text type="secondary">{t('content_studio.activity.grading.note')}</Typography.Text>
        </Form>
      </Card>
    </Flex>
  );
}
