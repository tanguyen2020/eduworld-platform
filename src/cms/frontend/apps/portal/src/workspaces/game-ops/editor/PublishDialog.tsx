// PublishDialog of a game config (F079-AC-03, AC-11, ADR-0043): publish an approved version now or at an ICT time (sent in
// UTC, not earlier than now + `scheduled_publish_min_lead`) with a change note; one intent = one Idempotency-Key, If-Match
// of a fresh read. A progression curve takes effect from the publish moment (`effective_from` = publish time or
// `publish_at`) and its confirmation states that no level is lowered and no granted XP is taken back (PROGRESSION_SYSTEM
// §4.2). A competition configuration (F108-AC-05, AC-10) is published with an effective_from instead (now or an ICT time,
// sent in UTC): the server refuses a past time and applies it from then; the pointer moves at once. Server errors are
// shown in the dialog (field errors by path, code + trace_id).
import { Alert, App as AntApp, DatePicker, Flex, Input, Modal, Radio, Typography } from 'antd';
import type { Dayjs } from 'dayjs';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { SCHEDULED_PUBLISH_MIN_LEAD_MIN } from '../../../shared/release/config';
import { earliestPublishAt, formatIct, isScheduleAllowed } from '../../../shared/release/ict';
import { useConfigWrites, type ItemRef } from '../api/gameConfigs';
import { pickedUtcIso } from '../builders/fields';
import { CONFIG_TYPES } from '../registry/configTypes';
import { GameOpsError, fieldErrorsOf } from '../shared/errors';

/** PublishRequest.change_note maxLength (OpenAPI). */
export const CHANGE_NOTE_MAX = 500;

export interface PublishDialogProps {
  item: ItemRef;
  version: number | null;
  onClose: () => void;
  now?: () => Date;
}

export function PublishDialog({ item, version, onClose, now = () => new Date() }: PublishDialogProps) {
  const { t, i18n } = useTranslation();
  const { message } = AntApp.useApp();
  const { publish } = useConfigWrites();
  const [mode, setMode] = useState<'now' | 'schedule'>('now');
  const [at, setAt] = useState<Dayjs | null>(null);
  const [note, setNote] = useState('');
  const [timeError, setTimeError] = useState<string | null>(null);
  const curve = item.configType === 'progression-curves';
  // Competition configurations take effect at effective_from (no scheduled publish, no client-side lead rule: the server
  // checks the time — AGENTS R1).
  const effective = CONFIG_TYPES[item.configType].publishMode === 'effective';
  const earliest = earliestPublishAt(now(), SCHEDULED_PUBLISH_MIN_LEAD_MIN);
  const fieldErrors = fieldErrorsOf(publish.error, t, (k) => i18n.exists(k));
  const close = () => {
    setMode('now');
    setAt(null);
    setNote('');
    setTimeError(null);
    publish.reset();
    onClose();
  };
  return (
    <Modal
      open={version !== null}
      title={t('gameOps.publish.title', { version })}
      okText={mode === 'now' || effective ? t('gameOps.publish.now') : t('gameOps.publish.schedule')}
      cancelText={t('gameOps.common.cancel')}
      confirmLoading={publish.isPending}
      destroyOnHidden
      onCancel={close}
      onOk={() => {
        if (version === null) return;
        let publishAt: string | undefined;
        let effectiveFrom: string | undefined;
        if (mode === 'schedule' && effective) {
          const iso = at ? pickedUtcIso(at) : null;
          if (!iso) {
            setTimeError(t('gameOps.publish.effectiveRequired'));
            return;
          }
          effectiveFrom = iso;
        } else if (mode === 'schedule') {
          const iso = at ? pickedUtcIso(at) : null;
          if (!iso || !isScheduleAllowed(new Date(iso), now(), SCHEDULED_PUBLISH_MIN_LEAD_MIN)) {
            setTimeError(t('gameOps.publish.tooEarly', { at: formatIct(earliest) }));
            return;
          }
          publishAt = iso;
        }
        publish.mutate(
          { ...item, version, ...(publishAt ? { publishAt } : {}), ...(effectiveFrom ? { effectiveFrom } : {}), ...(note.trim() ? { changeNote: note.trim() } : {}) },
          {
            onSuccess: (r) => {
              message.success(
                effectiveFrom
                  ? t('gameOps.publish.effectiveDone', { version, at: formatIct(effectiveFrom) })
                  : r.status === 'scheduled' || publishAt
                    ? t('gameOps.publish.scheduled', { at: formatIct(r.scheduled_for ?? publishAt) })
                    : t('gameOps.publish.done', { version }),
              );
              close();
            },
          },
        );
      }}
    >
      <Flex vertical gap="middle">
        {curve && <Alert type="warning" showIcon data-testid="publish-curve-notice" message={t('gameOps.publish.curveNotice')} />}
        <Radio.Group value={mode} onChange={(e) => setMode(e.target.value as 'now' | 'schedule')} aria-label={t('gameOps.publish.when')}>
          <Radio value="now">{effective ? t('gameOps.publish.effectiveNow') : t('gameOps.publish.whenNow')}</Radio>
          <Radio value="schedule">{effective ? t('gameOps.publish.effectiveAt') : t('gameOps.publish.whenSchedule')}</Radio>
        </Radio.Group>
        {mode === 'schedule' && (
          <Flex vertical gap="small">
            <DatePicker aria-label={effective ? t('gameOps.publish.effectiveAt') : t('gameOps.publish.at')} showTime={{ format: 'HH:mm' }} format="DD/MM/YYYY HH:mm" needConfirm={false} value={at} onChange={(d) => (setAt(d), setTimeError(null))} status={timeError ? 'error' : undefined} />
            <Typography.Text type="secondary">{effective ? t('gameOps.publish.effectiveHint') : t('gameOps.publish.ictHint', { at: formatIct(earliest) })}</Typography.Text>
            {timeError && (
              <Typography.Text type="danger" role="alert">
                {timeError}
              </Typography.Text>
            )}
          </Flex>
        )}
        <Input.TextArea aria-label={t('gameOps.publish.changeNote')} placeholder={t('gameOps.publish.changeNote')} maxLength={CHANGE_NOTE_MAX} showCount value={note} onChange={(e) => setNote(e.target.value)} />
        {Object.entries(fieldErrors).map(([f, m]) => (
          <Typography.Text key={f} type="danger" data-field={f}>
            {`${f}: ${m}`}
          </Typography.Text>
        ))}
        {publish.error ? <GameOpsError error={publish.error} /> : null}
      </Flex>
    </Modal>
  );
}
