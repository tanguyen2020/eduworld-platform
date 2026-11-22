// Edit an assignment (F055 T027, F055-AC-12): only the fields its status allows (editableFields: open → due_at,
// instructions, late_policy), merge patch of the changed fields with If-Match (ETag of getAssignment, remembered by the
// apiClient). 412 / VERSION_MISMATCH / ASSIGNMENT_NOT_EDITABLE → message, reload of the fresh copy, nothing overwritten.
import { DatePicker, Form, Input, InputNumber, Modal, Switch } from 'antd';
import dayjs, { type Dayjs } from 'dayjs';
import { useTranslation } from 'react-i18next';
import { useUpdateAssignment, type Assignment } from '../api/assignments';
import { editableFields, toUpdatePayload } from '../model/assignmentForm';
import { LMS_ENUMS, LMS_FIELDS, LMS_LIMITS } from '../model/fieldSchemas';
import { isStale } from '../model/problemMessages';
import { LmsProblem } from '../LmsUi';
import { SearchSelect } from '@eduworld/ops-ui';

interface EditValues {
  title?: string;
  instructions?: string;
  open_at?: Dayjs;
  due_at?: Dayjs;
  allow_late?: boolean;
  late_window_hours?: number;
  max_attempts?: number;
  score_policy?: Assignment['score_policy'];
}

/** The late policy is sent only when the user changed it (merge patch of changed fields). */
function latePolicyChanged(a: Assignment, v: EditValues): boolean {
  return !!v.allow_late !== (a.late_policy?.allow_late ?? false) || (!!v.allow_late && (v.late_window_hours ?? 0) !== (a.late_policy?.late_window_hours ?? 0));
}

export function AssignmentEditDialog({ assignment: a, onClose, onStale }: { assignment: Assignment; onClose: () => void; onStale: () => void }) {
  const { t } = useTranslation();
  const [form] = Form.useForm<EditValues>();
  const update = useUpdateAssignment(a.assignment_id);
  const fields = new Set(editableFields(a.status));
  const stale = isStale(update.error);
  return (
    <Modal
      open
      title={t('lms.assignment.editTitle')}
      okText={t('portal.school_lms.common.save')}
      cancelText={t('portal.school_lms.common.cancel')}
      confirmLoading={update.isPending}
      okButtonProps={{ disabled: stale }}
      destroyOnHidden
      onCancel={onClose}
      onOk={async () => {
        const v = await form.validateFields().catch(() => null);
        if (!v) return;
        const body = toUpdatePayload(a, {
          title: v.title,
          instructions: v.instructions,
          open_at: v.open_at?.toISOString(),
          due_at: v.due_at?.toISOString(),
          late_policy: latePolicyChanged(a, v) ? { allow_late: !!v.allow_late, late_window_hours: v.allow_late ? (v.late_window_hours ?? 0) : 0 } : undefined,
          max_attempts: v.max_attempts,
          score_policy: v.score_policy,
        });
        update.mutate(body, {
          onSuccess: onClose,
          onError: (e) => {
            if (isStale(e)) onStale();
          },
        });
      }}
    >
      {stale ? <LmsProblem error={update.error} testId="edit-stale" /> : <LmsProblem error={update.error} />}
      <Form
        form={form}
        layout="vertical"
        initialValues={{
          title: a.title,
          instructions: a.instructions,
          open_at: a.open_at ? dayjs(a.open_at) : undefined,
          due_at: a.due_at ? dayjs(a.due_at) : undefined,
          allow_late: a.late_policy?.allow_late ?? false,
          late_window_hours: a.late_policy?.late_window_hours ?? 0,
          max_attempts: a.max_attempts,
          score_policy: a.score_policy,
        }}
      >
        <Form.Item name="title" label={t('lms.wizard.title')} rules={[{ required: true, message: t('lms.form.required') }]}>
          <Input disabled={!fields.has('title')} showCount maxLength={LMS_FIELDS.assignmentTitle.maxLength} />
        </Form.Item>
        <Form.Item name="instructions" label={t('lms.wizard.instructions')}>
          <Input.TextArea disabled={!fields.has('instructions')} showCount maxLength={LMS_FIELDS.assignmentInstructions.maxLength} autoSize={{ minRows: 2, maxRows: 6 }} />
        </Form.Item>
        <Form.Item name="open_at" label={t('lms.wizard.openAt')}>
          <DatePicker showTime disabled={!fields.has('open_at')} aria-label={t('lms.wizard.openAt')} />
        </Form.Item>
        <Form.Item
          name="due_at"
          label={t('lms.wizard.dueAt')}
          dependencies={['open_at']}
          rules={[
            ({ getFieldValue }) => ({
              validator: (_, d?: Dayjs) => {
                const o = getFieldValue('open_at') as Dayjs | undefined;
                return !d || !o || d.isAfter(o) ? Promise.resolve() : Promise.reject(new Error(t('lms.form.dueAfterOpen')));
              },
            }),
          ]}
        >
          <DatePicker showTime disabled={!fields.has('due_at')} aria-label={t('lms.wizard.dueAt')} />
        </Form.Item>
        <Form.Item name="allow_late" label={t('lms.wizard.allowLate')} valuePropName="checked">
          <Switch disabled={!fields.has('late_policy')} aria-label={t('lms.wizard.allowLate')} />
        </Form.Item>
        <Form.Item name="late_window_hours" label={t('lms.wizard.lateWindowHours')}>
          <InputNumber disabled={!fields.has('late_policy')} min={LMS_LIMITS.lateWindowHours.minimum} max={LMS_LIMITS.lateWindowHours.maximum} precision={0} />
        </Form.Item>
        <Form.Item name="max_attempts" label={t('lms.wizard.maxAttempts')}>
          <InputNumber disabled={!fields.has('max_attempts')} min={LMS_LIMITS.maxAttempts.minimum} max={LMS_LIMITS.maxAttempts.maximum} precision={0} />
        </Form.Item>
        <Form.Item name="score_policy" label={t('lms.wizard.scorePolicy')}>
          <SearchSelect disabled={!fields.has('score_policy')} options={LMS_ENUMS.scorePolicy.values.map((s) => ({ value: s, label: t(`lms.scorePolicy.${s}`) }))} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
