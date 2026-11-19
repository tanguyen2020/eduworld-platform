// Exclude students from the cleanup or postpone it to a date (F052 T043, F052-AC-21): at most `maxItems` of
// CleanupExclusionRequest per call, reason required (3–500), createStudentCleanupExclusions with an Idempotency-Key per
// intent (reused when retried with the same data).
import { Alert, App, DatePicker, Form, Input, Modal, Radio } from 'antd';
import dayjs, { type Dayjs } from 'dayjs';
import { useTranslation } from 'react-i18next';
import { writeErrorKey } from '../common/errors';
import { FIELDS, LIMITS } from '../common/fieldSchemas';
import { useFieldRules } from '../common/ui';
import { useExcludeStudents } from './useCleanup';

interface ExcludeForm {
  mode: 'exclude' | 'postpone';
  until?: Dayjs;
  reason: string;
}

export const MAX_EXCLUDE = LIMITS.cleanupMaxStudents.maxItems;

export function ExcludeDialog({ studentIds, onClose }: { studentIds: string[]; onClose: (done: boolean) => void }) {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const [form] = Form.useForm<ExcludeForm>();
  const exclude = useExcludeStudents();
  const field = useFieldRules();
  const mode = Form.useWatch('mode', form);
  const tooMany = studentIds.length > MAX_EXCLUDE;
  const ok = async () => {
    if (tooMany) return;
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    try {
      await exclude.mutateAsync({ student_ids: studentIds, reason: v.reason.trim(), until: v.mode === 'postpone' && v.until ? v.until.endOf('day').toISOString() : null });
      message.success(t('portal.school_lms.cleanup.excluded', { count: studentIds.length }));
      onClose(true);
    } catch (e) {
      message.error(t(writeErrorKey(e)));
    }
  };
  return (
    <Modal open title={t('portal.school_lms.cleanup.excludeTitle', { count: studentIds.length })} okText={t('portal.school_lms.common.confirm')} cancelText={t('portal.school_lms.common.cancel')} okButtonProps={{ disabled: tooMany }} confirmLoading={exclude.isPending} onOk={ok} onCancel={() => onClose(false)} destroyOnHidden>
      {tooMany ? <Alert type="error" showIcon role="alert" message={t('portal.school_lms.cleanup.tooMany', { max: MAX_EXCLUDE })} /> : null}
      <Form<ExcludeForm> form={form} layout="vertical" preserve={false} initialValues={{ mode: 'exclude' }} data-testid="exclude-form">
        <Form.Item name="mode" label={t('portal.school_lms.cleanup.mode')}>
          <Radio.Group options={(['exclude', 'postpone'] as const).map((m) => ({ value: m, label: t(`portal.school_lms.cleanup.modes.${m}`) }))} />
        </Form.Item>
        {mode === 'postpone' ? (
          <Form.Item name="until" label={t('portal.school_lms.cleanup.until')} rules={[{ required: true, message: t('portal.school_lms.cleanup.untilRequired') }]}>
            <DatePicker disabledDate={(d) => d.isBefore(dayjs(), 'day')} />
          </Form.Item>
        ) : null}
        <Form.Item name="reason" label={t('portal.school_lms.common.reason')} rules={field.rules(FIELDS.cleanupReason, true)}>
          <Input.TextArea autoSize={{ minRows: 2, maxRows: 5 }} {...field.input(FIELDS.cleanupReason)} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
