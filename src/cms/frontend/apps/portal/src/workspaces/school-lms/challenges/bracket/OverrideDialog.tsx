// "Điều chỉnh kết quả" of a bracket node (F108 T028; F108-AC-15): two steps — pick the winner among the two entries
// and write the mandatory reason (the next button stays locked until both are given; maxLength of
// BracketNodeOverride.reason), then confirm the summary; only that second click calls overrideBracketNode. The error
// of the last attempt stays in the dialog. Presentational only (AGENTS R1).
import { Alert, Button, Flex, Form, Input, Modal, Radio, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import type { BracketNode } from '../api/challenges';
import { ChallengeProblem } from '../ChallengeUi';
import { CHALLENGE_FIELDS } from '../model/challengeForm';

export interface OverrideDialogProps {
  node: BracketNode;
  nameOf: (entryId: string | null | undefined) => string | undefined;
  pending: boolean;
  error: unknown;
  onSubmit: (winnerEntryId: string, reason: string) => void;
  onCancel: () => void;
}

export function OverrideDialog({ node, nameOf, pending, error, onSubmit, onCancel }: OverrideDialogProps) {
  const { t } = useTranslation();
  const [winner, setWinner] = useState<string | undefined>();
  const [reason, setReason] = useState('');
  const [confirming, setConfirming] = useState(false);
  const ready = !!winner && reason.trim().length >= CHALLENGE_FIELDS.overrideReason.minLength;
  const label = (id: string | null | undefined) => nameOf(id) ?? t('lms.challenges.entries.unknownStudent');
  const entries = [node.entry_a_id, node.entry_b_id].filter((x): x is string => !!x);
  const footer = confirming ? (
    <Flex gap="small" justify="flex-end">
      <Button onClick={() => setConfirming(false)}>{t('lms.challenges.override.back')}</Button>
      <Button type="primary" danger loading={pending} onClick={() => onSubmit(winner!, reason.trim())}>
        {t('lms.challenges.override.submit')}
      </Button>
    </Flex>
  ) : (
    <Flex gap="small" justify="flex-end">
      <Button onClick={onCancel}>{t('lms.challenges.override.cancel')}</Button>
      <Button type="primary" disabled={!ready} onClick={() => setConfirming(true)}>
        {t('lms.challenges.override.next')}
      </Button>
    </Flex>
  );
  return (
    <Modal open title={t('lms.challenges.override.title')} onCancel={onCancel} footer={footer} destroyOnHidden>
      {confirming ? (
        <Flex vertical gap="small">
          <Typography.Paragraph strong>{t('lms.challenges.override.confirmText', { name: label(winner) })}</Typography.Paragraph>
          <Typography.Paragraph>{t('lms.challenges.override.reasonText', { reason: reason.trim() })}</Typography.Paragraph>
          <Alert type="warning" showIcon message={t('lms.challenges.override.warning')} />
        </Flex>
      ) : (
        <Form layout="vertical" component="div">
          <Form.Item label={t('lms.challenges.override.winner')} required>
            <Radio.Group value={winner} onChange={(e) => setWinner(e.target.value as string)} options={entries.map((id) => ({ value: id, label: label(id) }))} />
          </Form.Item>
          <Form.Item label={t('lms.challenges.override.reason')} htmlFor="override-reason" required>
            <Input.TextArea id="override-reason" value={reason} onChange={(e) => setReason(e.target.value)} maxLength={CHALLENGE_FIELDS.overrideReason.maxLength} showCount autoSize={{ minRows: 3, maxRows: 8 }} />
          </Form.Item>
        </Form>
      )}
      <ChallengeProblem error={error} testId="override-error" />
    </Modal>
  );
}
