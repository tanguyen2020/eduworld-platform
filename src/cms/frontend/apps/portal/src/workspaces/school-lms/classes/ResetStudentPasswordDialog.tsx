// Reset a student's password (F052 T059, F052-AC-31, ACCOUNT_PROVISIONING §6d): reason required (OpenAPI 3–500),
// resetStudentPassword with one Idempotency-Key per confirmation (reused when retried with the same reason), the new
// password shown ONCE with a printable login card. The password lives only in this component's state: never written
// to storage, the query cache or logs (AGENTS R14); closing the dialog forgets it. `password_redacted` (replayed
// response) → the password cannot be shown again, suggest resetting once more.
import { Alert, App, Button, Card, Descriptions, Form, Input, Modal, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { writeErrorKey } from '../common/errors';
import { FIELDS } from '../common/fieldSchemas';
import { useFieldRules } from '../common/ui';
import { useResetStudentPassword, type PasswordResetResult } from './useClasses';

export interface ResetStudentPasswordDialogProps {
  student: { student_id: string; display_name: string; username: string };
  open: boolean;
  onClose: () => void;
  /** Print hook (window.print in the browser; a spy in tests). */
  print?: () => void;
}

export function ResetStudentPasswordDialog({ student, open, onClose, print = () => window.print() }: ResetStudentPasswordDialogProps) {
  const { t } = useTranslation();
  const { message } = App.useApp();
  const [form] = Form.useForm<{ reason: string }>();
  const reset = useResetStudentPassword();
  const field = useFieldRules();
  const [result, setResult] = useState<PasswordResetResult | null>(null);
  const close = () => {
    setResult(null);
    reset.reset();
    onClose();
  };
  const confirm = async () => {
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    try {
      setResult(await reset.mutateAsync({ studentId: student.student_id, reason: v.reason.trim() }));
    } catch (e) {
      message.error(t(writeErrorKey(e)));
    }
  };
  return (
    <Modal
      open={open}
      title={t('portal.school_lms.passwordReset.title', { name: student.display_name })}
      onCancel={close}
      destroyOnHidden
      footer={
        result
          ? [
              !result.password_redacted && result.initial_password ? (
                <Button key="print" onClick={print}>
                  {t('portal.school_lms.passwordReset.print')}
                </Button>
              ) : null,
              <Button key="close" type="primary" onClick={close}>
                {t('portal.school_lms.common.close')}
              </Button>,
            ]
          : [
              <Button key="cancel" onClick={close}>
                {t('portal.school_lms.common.cancel')}
              </Button>,
              <Button key="ok" type="primary" danger loading={reset.isPending} onClick={confirm}>
                {t('portal.school_lms.passwordReset.confirm')}
              </Button>,
            ]
      }
    >
      {result ? (
        result.password_redacted || !result.initial_password ? (
          <Alert type="warning" showIcon role="alert" message={t('portal.school_lms.passwordReset.redacted')} />
        ) : (
          <Card size="small" title={t('portal.school_lms.passwordReset.cardTitle')} data-testid="login-card">
            <Descriptions column={1} size="small">
              <Descriptions.Item label={t('portal.school_lms.passwordReset.username')}>{result.username}</Descriptions.Item>
              <Descriptions.Item label={t('portal.school_lms.passwordReset.password')}>
                <Typography.Text code data-testid="initial-password">
                  {result.initial_password}
                </Typography.Text>
              </Descriptions.Item>
            </Descriptions>
            <Typography.Text type="secondary">{t('portal.school_lms.passwordReset.onceOnly')}</Typography.Text>
          </Card>
        )
      ) : (
        <Form form={form} layout="vertical" preserve={false}>
          <Typography.Paragraph>{t('portal.school_lms.passwordReset.description', { username: student.username })}</Typography.Paragraph>
          <Form.Item name="reason" label={t('portal.school_lms.common.reason')} rules={field.rules(FIELDS.passwordResetReason, true)}>
            <Input.TextArea autoSize={{ minRows: 2, maxRows: 5 }} {...field.input(FIELDS.passwordResetReason)} />
          </Form.Item>
        </Form>
      )}
    </Modal>
  );
}
