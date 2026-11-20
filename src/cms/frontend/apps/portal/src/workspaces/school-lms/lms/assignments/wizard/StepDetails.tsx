// Steps 3 "Thời gian" and 4 "Chính sách" of the wizard (F055 T018; F055-AC-03, AC-04): limits and counters from the
// contract (fieldSchemas), `follow_latest` disabled in graded mode, feedback timing / result release left empty =
// "mặc định của hệ thống" (the server applies graded_assignment_release_default). Presentational only.
import { Checkbox, DatePicker, Form, Input, InputNumber, Radio, Select, Switch } from 'antd';
import dayjs from 'dayjs';
import { useTranslation } from 'react-i18next';
import { followLatestAllowed, type AssignmentFormValues } from '../../model/assignmentForm';
import { LMS_ENUMS, LMS_FIELDS, LMS_LIMITS } from '../../model/fieldSchemas';

type Patch = (p: Partial<AssignmentFormValues>) => void;
type Errors = Partial<Record<keyof AssignmentFormValues, string>>;

const status = (e: string | undefined) => (e ? ('error' as const) : undefined);

export function StepSchedule({ v, set, errors }: { v: AssignmentFormValues; set: Patch; errors: Errors }) {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item label={t('lms.wizard.title')} htmlFor="asg-title" required validateStatus={status(errors.title)} help={errors.title}>
        <Input id="asg-title" value={v.title} onChange={(e) => set({ title: e.target.value })} showCount maxLength={LMS_FIELDS.assignmentTitle.maxLength} />
      </Form.Item>
      <Form.Item label={t('lms.wizard.instructions')} htmlFor="asg-instructions" validateStatus={status(errors.instructions)} help={errors.instructions}>
        <Input.TextArea
          id="asg-instructions"
          value={v.instructions}
          onChange={(e) => set({ instructions: e.target.value })}
          showCount
          maxLength={LMS_FIELDS.assignmentInstructions.maxLength}
          autoSize={{ minRows: 2, maxRows: 6 }}
        />
      </Form.Item>
      <Form.Item label={t('lms.wizard.openAt')} required validateStatus={status(errors.openAt)} help={errors.openAt}>
        <DatePicker showTime aria-label={t('lms.wizard.openAt')} value={v.openAt ? dayjs(v.openAt) : null} onChange={(d) => set({ openAt: d ? d.toISOString() : undefined })} />
      </Form.Item>
      <Form.Item label={t('lms.wizard.dueAt')} required validateStatus={status(errors.dueAt)} help={errors.dueAt}>
        <DatePicker showTime aria-label={t('lms.wizard.dueAt')} value={v.dueAt ? dayjs(v.dueAt) : null} onChange={(d) => set({ dueAt: d ? d.toISOString() : undefined })} />
      </Form.Item>
      <Form.Item label={t('lms.wizard.allowLate')}>
        <Switch aria-label={t('lms.wizard.allowLate')} checked={v.allowLate} onChange={(c) => set({ allowLate: c })} />
      </Form.Item>
      {v.allowLate ? (
        <Form.Item label={t('lms.wizard.lateWindowHours')} htmlFor="asg-late" validateStatus={status(errors.lateWindowHours)} help={errors.lateWindowHours}>
          <InputNumber
            id="asg-late"
            min={LMS_LIMITS.lateWindowHours.minimum}
            max={LMS_LIMITS.lateWindowHours.maximum}
            precision={0}
            value={v.lateWindowHours}
            onChange={(n) => set({ lateWindowHours: n ?? 0 })}
          />
        </Form.Item>
      ) : null}
    </>
  );
}

export function StepPolicy({ v, set, errors }: { v: AssignmentFormValues; set: Patch; errors: Errors }) {
  const { t } = useTranslation();
  const followOk = followLatestAllowed(v.mode);
  return (
    <>
      <Form.Item label={t('lms.wizard.mode')}>
        <Radio.Group
          aria-label={t('lms.wizard.mode')}
          value={v.mode}
          onChange={(e) => {
            const mode = e.target.value as AssignmentFormValues['mode'];
            set({ mode, followLatest: followLatestAllowed(mode) ? v.followLatest : false });
          }}
          options={LMS_ENUMS.mode.values.map((m) => ({ value: m, label: t(`lms.mode.${m}`) }))}
        />
      </Form.Item>
      <Form.Item>
        <Checkbox checked={v.followLatest && followOk} disabled={!followOk} onChange={(e) => set({ followLatest: e.target.checked })} data-testid="follow-latest">
          {t('lms.wizard.followLatest')}
        </Checkbox>
      </Form.Item>
      <Form.Item label={t('lms.wizard.maxAttempts')} htmlFor="asg-attempts" validateStatus={status(errors.maxAttempts)} help={errors.maxAttempts}>
        <InputNumber id="asg-attempts" min={LMS_LIMITS.maxAttempts.minimum} max={LMS_LIMITS.maxAttempts.maximum} precision={0} value={v.maxAttempts} onChange={(n) => set({ maxAttempts: n ?? 1 })} />
      </Form.Item>
      <Form.Item label={t('lms.wizard.scorePolicy')}>
        <Select
          aria-label={t('lms.wizard.scorePolicy')}
          value={v.scorePolicy}
          onChange={(s: AssignmentFormValues['scorePolicy']) => set({ scorePolicy: s })}
          options={LMS_ENUMS.scorePolicy.values.map((s) => ({ value: s, label: t(`lms.scorePolicy.${s}`) }))}
        />
      </Form.Item>
      <Form.Item label={t('lms.wizard.feedbackTiming')} extra={t('lms.wizard.serverDefault')}>
        <Select
          allowClear
          aria-label={t('lms.wizard.feedbackTiming')}
          placeholder={t('lms.wizard.serverDefaultShort')}
          value={v.feedbackTiming}
          onChange={(s) => set({ feedbackTiming: s ?? undefined })}
          options={LMS_ENUMS.feedbackTiming.values.map((s) => ({ value: s, label: t(`lms.feedbackTiming.${s}`) }))}
        />
      </Form.Item>
      <Form.Item label={t('lms.wizard.resultRelease')}>
        <Select
          allowClear
          aria-label={t('lms.wizard.resultRelease')}
          placeholder={t('lms.wizard.serverDefaultShort')}
          value={v.resultRelease}
          onChange={(s) => set({ resultRelease: s ?? undefined })}
          options={LMS_ENUMS.resultRelease.values.map((s) => ({ value: s, label: t(`lms.resultRelease.${s}`) }))}
        />
      </Form.Item>
    </>
  );
}
