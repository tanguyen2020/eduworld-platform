// ObjectiveForm (F053-AC-28, CURRICULUM_MODEL §3.3, §4): learning objective — code, description, Bloom level,
// `yccd_ref` (GDPT 2018), prerequisite LOs (at most `prerequisite_ids.maxItems` = 20). A prerequisite cycle refused
// by the server (422, errors[].code = PREREQUISITE_CYCLE) is shown on the prerequisites field.
import { Button, Form, Input } from 'antd';
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ApiError } from '../../../shared/api/middleware/problem';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { LIMITS } from '../shared/config';
import type { LearningObjective, LearningObjectiveInput } from './useCurriculum';
import { SearchSelect } from '@eduworld/ops-ui';

export const BLOOM_LEVELS = ['remember', 'understand', 'apply', 'analyze', 'evaluate', 'create'] as const;

export interface ObjectiveFormProps {
  /** Existing LO (edit) or undefined (create under `topicId`). */
  objective?: LearningObjective;
  topicId: string;
  /** LOs of the same curriculum version (prerequisite candidates). */
  candidates: readonly LearningObjective[];
  readOnly?: boolean;
  onSubmit: (input: LearningObjectiveInput) => Promise<unknown>;
}

type Values = Pick<LearningObjectiveInput, 'code' | 'description' | 'bloom_level' | 'yccd_ref' | 'prerequisite_ids'>;

export function ObjectiveForm({ objective, topicId, candidates, readOnly = false, onSubmit }: ObjectiveFormProps) {
  const { t } = useTranslation();
  const [form] = Form.useForm<Values>();
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    form.resetFields();
    form.setFieldsValue({
      code: objective?.code,
      description: objective?.description,
      bloom_level: objective?.bloom_level,
      yccd_ref: objective?.yccd_ref,
      prerequisite_ids: objective?.prerequisite_ids ?? [],
    });
    setError(null);
  }, [objective, form]);

  const submit = async (v: Values) => {
    setBusy(true);
    setError(null);
    try {
      await onSubmit({ ...v, topic_id: topicId, yccd_ref: v.yccd_ref?.trim() || undefined });
    } catch (e) {
      if (e instanceof ApiError && e.code === 'VALIDATION_FAILED') {
        const fields = Object.entries(e.fieldErrors).map(([field, fe]) => ({
          name: field.replace(/^\//, '').split(/[./]/)[0] as keyof Values,
          errors: [fe.code === 'PREREQUISITE_CYCLE' ? t('content_studio.curriculum.cycle', { path: fe.message ?? '' }) : t('content_studio.validation.invalid')],
        }));
        form.setFields(fields);
      }
      setError(e);
    } finally {
      setBusy(false);
    }
  };

  return (
    <Form form={form} layout="vertical" disabled={readOnly} onFinish={(v) => void submit(v)} aria-label={t('content_studio.curriculum.objective')}>
      <Form.Item name="code" label={t('content_studio.curriculum.loCode')} rules={[{ required: true, whitespace: true, message: t('content_studio.validation.required') }]}>
        <Input maxLength={LIMITS.objectiveCode} showCount />
      </Form.Item>
      <Form.Item name="description" label={t('content_studio.curriculum.loDescription')} rules={[{ required: true, whitespace: true, message: t('content_studio.validation.required') }]}>
        <Input.TextArea maxLength={LIMITS.objectiveDescription} showCount autoSize={{ minRows: 2, maxRows: 6 }} />
      </Form.Item>
      <Form.Item name="bloom_level" label={t('content_studio.curriculum.bloom')}>
        <SearchSelect allowClear options={BLOOM_LEVELS.map((b) => ({ value: b, label: t(`content_studio.curriculum.bloomLevels.${b}`) }))} />
      </Form.Item>
      <Form.Item name="yccd_ref" label={t('content_studio.curriculum.yccd')}>
        <Input maxLength={LIMITS.yccdRef} showCount />
      </Form.Item>
      <Form.Item name="prerequisite_ids" label={t('content_studio.curriculum.prerequisites', { max: LIMITS.prerequisites })}>
        <SearchSelect
          mode="multiple"
          aria-label={t('content_studio.curriculum.prerequisites', { max: LIMITS.prerequisites })}
          maxCount={LIMITS.prerequisites}
          optionFilterProp="label"
          options={candidates.filter((c) => c.objective_id !== objective?.objective_id).map((c) => ({ value: c.objective_id, label: `${c.code} — ${c.description}` }))}
        />
      </Form.Item>
      {error && !(error instanceof ApiError && error.code === 'VALIDATION_FAILED') ? <ProblemMessage error={error} /> : null}
      {!readOnly && (
        <Button type="primary" htmlType="submit" loading={busy}>
          {objective ? t('content_studio.save') : t('content_studio.curriculum.addObjective')}
        </Button>
      )}
    </Form>
  );
}
