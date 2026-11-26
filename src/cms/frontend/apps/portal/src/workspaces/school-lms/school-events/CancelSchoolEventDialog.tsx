// "Hủy sự kiện" (F081-AC-08, T015): reason required (3–1000 characters, counter — EventRunOperation.reason), then a
// second confirmation that says "phần thưởng học sinh đã đạt được giữ nguyên"; cancelSchoolEvent with an Idempotency-Key
// per intent (kept on retry). Errors by `code` + trace_id. The server decides whether the event can still be cancelled.
import { Alert, Button, Flex, Form, Input, Modal, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ProblemMessage } from '../common/errors';
import { FIELDS } from '../common/fieldSchemas';
import { useFieldRules } from '../common/ui';
import { useCancelSchoolEvent } from './api';

export function CancelSchoolEventDialog({ eventId, eventName, onClose, onDone }: { eventId: string; eventName: string; onClose: () => void; onDone: () => void }) {
  const { t } = useTranslation();
  const field = useFieldRules();
  const [form] = Form.useForm<{ reason: string }>();
  const [reason, setReason] = useState<string | null>(null);
  const cancel = useCancelSchoolEvent(eventId);
  return (
    <Modal open title={t('portal.school_lms.schoolEvents.cancel.title', { name: eventName })} onCancel={onClose} footer={null} destroyOnHidden>
      {reason === null ? (
        <Form form={form} layout="vertical" name="school_event_cancel" onFinish={(v) => setReason(v.reason.trim())}>
          <Form.Item name="reason" label={t('portal.school_lms.schoolEvents.cancel.reason')} rules={[{ required: true, whitespace: true, message: t('portal.school_lms.schoolEvents.cancel.reasonRequired') }, ...field.rules(FIELDS.eventCancelReason, true)]}>
            <Input.TextArea autoSize={{ minRows: 3, maxRows: 8 }} {...field.input(FIELDS.eventCancelReason)} />
          </Form.Item>
          <Flex gap="small">
            <Button type="primary" danger htmlType="submit">
              {t('portal.school_lms.schoolEvents.cancel.next')}
            </Button>
            <Button onClick={onClose}>{t('portal.school_lms.common.close')}</Button>
          </Flex>
        </Form>
      ) : (
        <Flex vertical gap="middle" data-testid="cancel-confirm">
          <Typography.Text strong>{t('portal.school_lms.schoolEvents.cancel.confirmTitle')}</Typography.Text>
          <Alert type="warning" showIcon message={t('portal.school_lms.schoolEvents.cancel.rewardsKept')} />
          <Typography.Paragraph type="secondary">{t('portal.school_lms.schoolEvents.cancel.reasonShown', { reason })}</Typography.Paragraph>
          {cancel.error ? <ProblemMessage error={cancel.error} /> : null}
          <Flex gap="small">
            <Button type="primary" danger loading={cancel.isPending} onClick={() => cancel.mutate(reason, { onSuccess: onDone })}>
              {t('portal.school_lms.schoolEvents.cancel.confirm')}
            </Button>
            <Button onClick={() => setReason(null)}>{t('portal.school_lms.common.back')}</Button>
          </Flex>
        </Flex>
      )}
    </Modal>
  );
}
