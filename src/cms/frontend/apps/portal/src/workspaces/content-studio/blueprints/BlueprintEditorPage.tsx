// BlueprintEditorPage (F054-AC-22, AC-23, ADMIN_PORTAL §3.2, §5): Assessment Blueprint editor on the shared builder
// frame — metadata, exam type, duration, total points, attempts, result release and the LO × difficulty matrix with
// the pool of each cell; "Sinh đề mẫu" draws a sample exam through createLearningPreviewSession (nothing saved, no
// exam session); QUESTION_POOL_INSUFFICIENT refreshes the pool stats and highlights the cells lacking questions.
import { QuestionPreview, type RichContent } from '@eduworld/ops-ui/content';
import { useMutation } from '@tanstack/react-query';
import { Alert, Button, Card, Flex, Form, InputNumber, List, Select, Typography } from 'antd';
import { useCallback, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { usePortalRuntime } from '../../../app/runtime';
import { ActionGate } from '../../../permissions/ActionGate';
import { ApiError } from '../../../shared/api/middleware/problem';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { DocMetaFields } from '../builders/DocMetaFields';
import { DocumentEditorPage, type BuilderArgs } from '../builders/DocumentEditorPage';
import { usePoolStats, type BlueprintAuthoring, type DocAuthoring } from '../builders/useDocument';
import { useObjectives } from '../curriculum/useCurriculum';
import { EfficacyInstrumentField } from '../questions/EfficacyInstrumentField';
import type { PreviewSession } from '../preview/usePreview';
import { cellPools } from './matrixModel';
import { BlueprintMatrix } from './BlueprintMatrix';
import { blueprintIssues, emptyBlueprint, EXAM_TYPES, fromBlueprint, toBlueprintBody, type BlueprintState, type ExamType, type ReleaseMode } from './blueprintModel';

function SampleExam({ id, version, onInsufficient }: { id: string; version: number; onInsufficient: () => void }) {
  const { t } = useTranslation();
  const { api } = usePortalRuntime();
  const sample = useMutation({
    mutationFn: () => api.call<PreviewSession>(api.client.POST('/cms/learning/preview/sessions', { body: { content_type: 'assessment_blueprint', content_id: id, version } })),
    onError: (e) => {
      if (e instanceof ApiError && e.code === 'QUESTION_POOL_INSUFFICIENT') onInsufficient();
    },
  });
  return (
    <Card size="small" title={t('content_studio.blueprint.sample')}>
      <ActionGate actionKey="action.content_studio.reviews.preview">
        <Button loading={sample.isPending} onClick={() => sample.mutate()}>
          {t('content_studio.blueprint.generate')}
        </Button>
      </ActionGate>
      <Typography.Paragraph type="secondary">{t('content_studio.blueprint.sampleHint')}</Typography.Paragraph>
      {sample.error ? <ProblemMessage error={sample.error} /> : null}
      {sample.data && (
        <Flex vertical gap="small" data-testid="sample-exam">
          <Alert type="success" showIcon message={t('content_studio.blueprint.sampleCount', { count: sample.data.items?.length ?? 0 })} />
          <List
            size="small"
            dataSource={sample.data.items ?? []}
            renderItem={(it) => (
              <List.Item key={it.position}>
                <QuestionPreview type={it.type} stem={(it.render_payload as { stem?: RichContent }).stem} renderPayload={it.render_payload} language="vi" contentLanguage="vi" />
              </List.Item>
            )}
          />
        </Flex>
      )}
    </Card>
  );
}

function BlueprintBody({ state, onChange, readOnly, errors, server, id }: BuilderArgs<BlueprintState> & { id?: string }) {
  const { t } = useTranslation();
  const [highlight, setHighlight] = useState(false);
  const pool = usePoolStats('assessment_blueprint', id, server?.version);
  const los = useObjectives(state.topic_ids.length === 1 ? { topic_id: state.topic_ids[0] } : { subject_id: state.subject_id }, !!state.subject_id);
  const loOptions = (los.data?.data ?? []).map((o) => ({ value: o.objective_id, label: `${o.code} — ${o.description}` }));
  const labels = new Map(loOptions.map((o) => [o.value, o.label]));
  return (
    <Flex vertical gap="middle">
      <DocMetaFields value={state} onChange={onChange} readOnly={readOnly} languageLocked={!!id} errors={errors} />
      <Form layout="vertical" component="div" disabled={readOnly}>
        <Flex gap="middle" wrap>
          <Form.Item label={t('content_studio.blueprint.examType')}>
            <Select aria-label={t('content_studio.blueprint.examType')} value={state.exam_type} options={EXAM_TYPES.map((x) => ({ value: x, label: t(`content_studio.blueprint.examTypes.${x}`) }))} onChange={(exam_type: ExamType) => onChange({ ...state, exam_type })} />
          </Form.Item>
          <Form.Item label={t('content_studio.blueprint.duration')} data-field="duration">
            <InputNumber aria-label={t('content_studio.blueprint.duration')} min={1} max={240} value={state.duration_min} onChange={(v) => onChange({ ...state, duration_min: v ?? 1 })} />
          </Form.Item>
          <Form.Item label={t('content_studio.blueprint.totalPoints')}>
            <InputNumber aria-label={t('content_studio.blueprint.totalPoints')} min={0.25} step={0.25} value={state.total_points} onChange={(v) => onChange({ ...state, total_points: v ?? 10 })} />
          </Form.Item>
          <Form.Item label={t('content_studio.blueprint.maxAttempts')}>
            <InputNumber aria-label={t('content_studio.blueprint.maxAttempts')} min={1} max={20} value={state.max_attempts} onChange={(v) => onChange({ ...state, max_attempts: v ?? 1 })} />
          </Form.Item>
          <Form.Item label={t('content_studio.blueprint.release')}>
            <Select aria-label={t('content_studio.blueprint.release')} value={state.result_release} options={(['immediate', 'after_close', 'manual'] as const).map((x) => ({ value: x, label: t(`content_studio.blueprint.releases.${x}`) }))} onChange={(result_release: ReleaseMode) => onChange({ ...state, result_release })} />
          </Form.Item>
        </Flex>
      </Form>
      <EfficacyInstrumentField kind="blueprint" actionKey="action.content_studio.assessments.edit" checked={!!state.efficacy_instrument} disabled={readOnly} onChange={(v) => onChange({ ...state, efficacy_instrument: v })} />
      <BlueprintMatrix
        matrix={state.matrix}
        onChange={(matrix) => onChange({ ...state, matrix })}
        objectiveLabels={labels}
        objectiveOptions={loOptions}
        targetPoints={state.total_points}
        pools={cellPools(state.matrix, pool.data, state.section.id)}
        highlightInsufficient={highlight}
        readOnly={readOnly}
      />
      {id && server && (
        <SampleExam
          id={id}
          version={server.version}
          onInsufficient={() => {
            setHighlight(true);
            void pool.refetch();
          }}
        />
      )}
    </Flex>
  );
}

export function BlueprintEditorPage({ id }: { id?: string }) {
  const { t } = useTranslation();
  const blockers = useCallback((s: BlueprintState) => blueprintIssues(s, t), [t]);
  const empty = useCallback(() => emptyBlueprint(t('content_studio.blueprint.defaultSection')), [t]);
  return (
    <DocumentEditorPage<BlueprintState>
      type="assessment_blueprint"
      id={id}
      title={{ create: t('content_studio.blueprint.newTitle'), edit: t('content_studio.blueprint.editTitle') }}
      empty={empty}
      fromServer={(d: DocAuthoring) => fromBlueprint(d as BlueprintAuthoring)}
      toBody={async (s) => toBlueprintBody(s)}
      blockers={blockers}
      body={(a) => <BlueprintBody {...a} id={id} />}
    />
  );
}
