// RollbackDialog (F054-AC-05, AC-06, AC-09, AC-29, ADR-0043): uses an older PUBLISHED version again for new
// sessions. Step 1: target (published versions of the same item; the current pointer is shown disabled) + reason
// (required, ≤ RollbackRequest.reason maxLength). Step 2: impact (sessions in progress keep their pinned version, new
// ones get the chosen version) and a second, warning-colored confirmation → rollbackContent (Idempotency-Key: a
// retry after a network error reuses the key, F054-AC-11). ROLLBACK_TARGET_INVALID / 403 are shown in the dialog.
import { Alert, App as AntApp, Flex, Form, Input, Modal, Select, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { LIMITS } from '../shared/config';
import { formatIct, rollbackTargets, type VersionSummary } from './releaseOptions';
import { useRelease, type ItemRef } from './useRelease';

export interface RollbackDialogProps {
  item: ItemRef;
  open: boolean;
  versions: readonly VersionSummary[];
  current: number | undefined;
  /** Pre-selected target (row action "Rollback về vN"). */
  initialTarget?: number;
  onClose: () => void;
}

export function RollbackDialog({ item, open, versions, current, initialTarget, onClose }: RollbackDialogProps) {
  const { t } = useTranslation();
  const { message } = AntApp.useApp();
  const { rollback } = useRelease();
  const [form] = Form.useForm<{ to_version?: number; reason?: string }>();
  const [step, setStep] = useState<1 | 2>(1);
  const [chosen, setChosen] = useState<{ to: number; reason: string } | null>(null);
  const targets = rollbackTargets(versions, current);
  const currentRow = versions.find((v) => v.version === current);
  const close = () => {
    setStep(1);
    setChosen(null);
    rollback.reset();
    onClose();
  };
  const options = [
    ...(currentRow ? [{ value: currentRow.version, label: t('content_studio.release.currentOption', { version: currentRow.version }), disabled: true }] : []),
    ...targets.map((v) => ({ value: v.version, label: t('content_studio.release.targetOption', { version: v.version, at: formatIct(v.published_at) }) })),
  ];
  return (
    <Modal
      open={open}
      title={t('content_studio.release.rollbackTitle')}
      okText={step === 1 ? t('content_studio.release.next') : t('content_studio.release.rollbackConfirm')}
      cancelText={step === 1 ? t('content_studio.cancel') : t('content_studio.release.back')}
      okButtonProps={{ danger: step === 2, disabled: step === 1 && targets.length === 0 }}
      confirmLoading={rollback.isPending}
      destroyOnHidden
      onCancel={() => (step === 2 ? setStep(1) : close())}
      onOk={async () => {
        if (step === 1) {
          const v = await form.validateFields().catch(() => null);
          if (!v?.to_version) return;
          setChosen({ to: v.to_version, reason: (v.reason ?? '').trim() });
          setStep(2);
          return;
        }
        if (!chosen) return;
        rollback.mutate(
          { ...item, toVersion: chosen.to, reason: chosen.reason },
          {
            onSuccess: () => {
              message.success(t('content_studio.release.rolledBack', { version: chosen.to }));
              close();
            },
          },
        );
      }}
    >
      {step === 1 ? (
        <Form form={form} layout="vertical" preserve initialValues={{ to_version: initialTarget && targets.some((x) => x.version === initialTarget) ? initialTarget : undefined }}>
          {targets.length === 0 && <Alert type="info" showIcon message={t('content_studio.release.noRollbackTarget')} />}
          <Form.Item name="to_version" label={t('content_studio.release.target')} rules={[{ required: true, message: t('content_studio.release.targetRequired') }]}>
            <Select aria-label={t('content_studio.release.target')} options={options} />
          </Form.Item>
          <Form.Item name="reason" label={t('content_studio.release.reason')} rules={[{ required: true, whitespace: true, message: t('content_studio.release.reasonRequired') }, { max: LIMITS.releaseReason }]}>
            <Input.TextArea aria-label={t('content_studio.release.reason')} maxLength={LIMITS.releaseReason} showCount autoSize={{ minRows: 2, maxRows: 6 }} />
          </Form.Item>
        </Form>
      ) : (
        <Flex vertical gap="small">
          <RollbackImpact from={current} to={chosen?.to} running={currentRow?.active_pinned_sessions ?? 0} reason={chosen?.reason ?? ''} />
          {rollback.error ? <ProblemMessage error={rollback.error} /> : null}
        </Flex>
      )}
    </Modal>
  );
}

/** Presentational part (stories): what a rollback changes, before the second confirmation (F054-AC-05). */
export function RollbackImpact({ from, to, running, reason }: { from: number | undefined; to: number | undefined; running: number; reason: string }) {
  const { t } = useTranslation();
  return (
    <Flex vertical gap="small" data-testid="rollback-impact">
      <Alert
        type="warning"
        showIcon
        message={t('content_studio.release.rollbackImpactTitle', { from: from ?? '—', to })}
        description={
          <ul>
            <li>{t('content_studio.release.rollbackImpactRunning', { from: from ?? '—', count: running })}</li>
            <li>{t('content_studio.release.rollbackImpactNew', { to })}</li>
            <li>{t('content_studio.release.rollbackImpactHistory')}</li>
          </ul>
        }
      />
      <Typography.Text>{t('content_studio.release.reasonShown', { reason })}</Typography.Text>
    </Flex>
  );
}
