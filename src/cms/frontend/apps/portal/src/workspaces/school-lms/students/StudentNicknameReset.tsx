// Reset a student's nickname (F136 T033, F136-AC-22, MODERATION §7): the dialog names the student, shows the current
// nickname when the caller has it (the grade student list carries none — no backend field is added for it) and explains
// the effect; reason required (OpenAPI 3–500); resetStudentNickname with one Idempotency-Key per confirmation, reused
// when retried with the same reason. Opened from an ActionGate (display only): module game decides with
// lms.student.nickname_reset and answers 403/404 otherwise (AGENTS R1, R7).
import { Alert, App, Button, Descriptions, Form, Input, Modal, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { writeErrorKey } from '../common/errors';
import { FIELDS } from '../common/fieldSchemas';
import { useFieldRules } from '../common/ui';
import { useResetStudentNickname } from './useStudentNickname';

export interface StudentNicknameResetProps {
  student: { student_id: string; display_name: string; username: string; nickname?: string | null };
  open: boolean;
  onClose: () => void;
}

export function StudentNicknameReset({ student, open, onClose }: StudentNicknameResetProps) {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const [form] = Form.useForm<{ reason: string }>();
  const reset = useResetStudentNickname();
  const field = useFieldRules();
  const close = () => {
    reset.reset();
    onClose();
  };
  const confirm = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    try {
      await reset.mutateAsync({ studentId: student.student_id, reason: v.reason.trim() });
      message.success(t('portal.school_lms.nicknameReset.done', { name: student.display_name }));
      close();
    } catch {
      // shown below by its code (writeErrorKey); the same reason retried keeps the Idempotency-Key
    }
  };
  return (
    <Modal
      open={open}
      title={t('portal.school_lms.nicknameReset.title', { name: student.display_name })}
      onCancel={close}
      destroyOnHidden
      footer={[
        <Button key="cancel" onClick={close}>
          {t('portal.school_lms.common.cancel')}
        </Button>,
        <Button key="ok" type="primary" danger loading={reset.isPending} onClick={confirm}>
          {t('portal.school_lms.nicknameReset.confirm')}
        </Button>,
      ]}
    >
      <Form form={form} layout="vertical" preserve={false}>
        {student.nickname ? (
          <Descriptions column={1} size="small">
            <Descriptions.Item label={t('portal.school_lms.nicknameReset.current')}>
              <Typography.Text data-testid="current-nickname">{student.nickname}</Typography.Text>
            </Descriptions.Item>
          </Descriptions>
        ) : null}
        <Typography.Paragraph>{t('portal.school_lms.nicknameReset.description', { name: student.display_name, username: student.username })}</Typography.Paragraph>
        <Form.Item name="reason" label={t('portal.school_lms.common.reason')} rules={field.rules(FIELDS.nicknameResetReason, true)}>
          <Input.TextArea autoSize={{ minRows: 2, maxRows: 5 }} {...field.input(FIELDS.nicknameResetReason)} />
        </Form.Item>
      </Form>
      {reset.error ? <Alert type="error" showIcon role="alert" message={t(writeErrorKey(reset.error))} /> : null}
    </Modal>
  );
}
