// Emergency operations of an event run (F080 T040, F080-AC-19, ADMIN_PORTAL §3.15): Tạm dừng / Tiếp tục / Hủy behind
// `<ActionGate actionKey="action.game_ops.events.operate">` (game.event.operate). Each needs a reason (3–1000 characters
// with a counter — EventRunOperation.reason) then a second confirmation; the dialog is mounted per user action, so each
// action gets its own Idempotency-Key (kept only when the same action is retried inside the dialog). `409` (the run
// changed meanwhile, e.g. already suspended) closes the dialog with a notice and reloads the run; other errors stay in the
// dialog with their code + trace_id. Only operations the current status allows are offered (the server decides anyway).
import { App as AntApp, Alert, Button, Flex, Form, Input, Modal, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ApiError } from '../../../shared/api/middleware/problem';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { operationsFor, useRunOperation, type EventRunSummary, type RunOperation } from '../api/eventRuns';
import { FIELDS } from '../limits';

function RunOperationDialog({ run, op, name, onClose }: { run: EventRunSummary; op: RunOperation; name: string; onClose: () => void }) {
  const { t } = useTranslation();
  const { message } = AntApp.useApp();
  const [form] = Form.useForm<{ reason: string }>();
  const [reason, setReason] = useState<string | null>(null);
  const mutation = useRunOperation(run);
  const conflict = mutation.error instanceof ApiError && mutation.error.status === 409;
  const submit = () =>
    mutation.mutate(
      { op, reason: reason! },
      {
        onSuccess: () => {
          void message.success(t(`liveOps.events.op.${op}.done`));
          onClose();
        },
        onError: (e) => {
          if (e instanceof ApiError && e.status === 409) {
            void message.warning(t('liveOps.events.conflict'));
            onClose();
          }
        },
      },
    );
  const { minLength, maxLength } = FIELDS.runReason;
  return (
    <Modal open title={t(`liveOps.events.op.${op}.title`, { name })} onCancel={onClose} footer={null} destroyOnHidden>
      {reason === null ? (
        <Form form={form} layout="vertical" name={`event_run_${op}`} onFinish={(v) => setReason(v.reason.trim())}>
          <Form.Item
            name="reason"
            label={t('liveOps.events.reason')}
            rules={[
              { required: true, whitespace: true, message: t('liveOps.events.reasonRequired') },
              { min: minLength, message: t('liveOps.events.reasonMin', { min: minLength }) },
              { max: maxLength },
            ]}
          >
            <Input.TextArea maxLength={maxLength} showCount autoSize={{ minRows: 3, maxRows: 8 }} />
          </Form.Item>
          <Flex gap="small">
            <Button type="primary" danger={op !== 'resume'} htmlType="submit">
              {t('liveOps.events.next')}
            </Button>
            <Button onClick={onClose}>{t('liveOps.events.close')}</Button>
          </Flex>
        </Form>
      ) : (
        <Flex vertical gap="middle" data-testid="run-op-confirm">
          <Typography.Text strong>{t(`liveOps.events.op.${op}.confirmTitle`, { name })}</Typography.Text>
          <Alert type="warning" showIcon message={t(`liveOps.events.op.${op}.consequence`)} />
          <Typography.Paragraph type="secondary">{t('liveOps.events.reasonShown', { reason })}</Typography.Paragraph>
          {mutation.error && !conflict ? <ProblemMessage error={mutation.error} /> : null}
          <Flex gap="small">
            <Button type="primary" danger={op !== 'resume'} loading={mutation.isPending} onClick={submit}>
              {t(`liveOps.events.op.${op}.confirm`)}
            </Button>
            <Button onClick={() => setReason(null)}>{t('liveOps.events.back')}</Button>
          </Flex>
        </Flex>
      )}
    </Modal>
  );
}

export function EventRunActions({ run, name }: { run: EventRunSummary; name: string }) {
  const { t } = useTranslation();
  const [op, setOp] = useState<RunOperation | null>(null);
  const ops = operationsFor(run.status);
  if (!ops.length) return null;
  return (
    <ActionGate actionKey="action.game_ops.events.operate">
      <Flex wrap gap="small" data-testid="run-actions">
        {ops.map((o) => (
          <Button key={o} danger={o !== 'resume'} onClick={() => setOp(o)}>
            {t(`liveOps.events.op.${o}.button`)}
          </Button>
        ))}
        {op ? <RunOperationDialog key={op} run={run} op={op} name={name} onClose={() => setOp(null)} /> : null}
      </Flex>
    </ActionGate>
  );
}
