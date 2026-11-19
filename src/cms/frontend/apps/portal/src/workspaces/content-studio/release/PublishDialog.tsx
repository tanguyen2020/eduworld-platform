// PublishDialog (F054-AC-04, AC-11, ADR-0043): publish an approved version now or at a time (chosen in ICT, sent in
// UTC, never earlier than now + `scheduled_publish_min_lead`), with a summary of the diff against the version students
// get now and a warning when media of the version is not `ready` yet. One user intent = one Idempotency-Key (clicking
// twice / retrying after a network error gives one release). A 409/412 (state changed elsewhere) refreshes the item.
import { useQueries } from '@tanstack/react-query';
import { Alert, App as AntApp, DatePicker, Flex, Input, Modal, Radio, Skeleton, Typography } from 'antd';
import type { Dayjs } from 'dayjs';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { usePortalRuntime } from '../../../app/runtime';
import { useSession } from '../../../session/SessionProvider';
import { ApiError } from '../../../shared/api/middleware/problem';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { mediaAssetKey, type MediaAsset } from '../media/useMediaUpload';
import { LIMITS, PUBLISH_TO_VISIBLE_MAX_S, SCHEDULED_PUBLISH_MIN_LEAD_MIN } from '../shared/config';
import { mediaRefs } from '../shared/gates';
import { earliestPublishAt, formatIct, groupDiff, ictToUtc, isScheduleAllowed, utcToIct } from './releaseOptions';
import { useDiff, useRelease, useVersionDetail, useVersions, type ItemRef } from './useRelease';

export interface PublishDialogProps {
  item: ItemRef;
  /** Version to publish (approved); null = closed. */
  version: number | null;
  /** Version students get now; read from listContentVersions when not given. */
  current?: number;
  onClose: () => void;
  /** Clock (tests). */
  now?: () => Date;
}

/** Ids of the media referenced by a version body (stem/blocks/options…). */
function useMediaWarnings(item: ItemRef, version: number | null) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  const detail = useVersionDetail(item, version ?? undefined);
  const ids = detail.data ? mediaRefs((detail.data.data.content ?? {}) as never) : [];
  const assets = useQueries({
    queries: ids.map((id) => ({
      queryKey: mediaAssetKey(scopeKey, id),
      queryFn: () => api.call<MediaAsset>(api.client.GET('/cms/media/{asset_id}', { params: { path: { asset_id: id } } })),
    })),
  });
  return assets.filter((a) => a.data && a.data.status !== 'ready').map((a) => a.data!);
}

export function PublishDialog({ item, version, current: currentProp, onClose, now = () => new Date() }: PublishDialogProps) {
  const history = useVersions(item);
  const current = currentProp ?? history.data?.current_version;
  const { t } = useTranslation();
  const { message } = AntApp.useApp();
  const { publish } = useRelease();
  const [mode, setMode] = useState<'now' | 'schedule'>('now');
  const [at, setAt] = useState<Dayjs | null>(null);
  const [note, setNote] = useState('');
  const [timeError, setTimeError] = useState<string | null>(null);
  const diff = useDiff(item, version ?? undefined, current);
  const notReady = useMediaWarnings(item, version);
  const earliest = earliestPublishAt(now(), SCHEDULED_PUBLISH_MIN_LEAD_MIN);
  const earliestWall = utcToIct(earliest);
  const close = () => {
    setMode('now');
    setAt(null);
    setNote('');
    setTimeError(null);
    publish.reset();
    onClose();
  };
  /** The picker shows wall-clock ICT: its fields are read as ICT. */
  const pickedUtc = (d: Dayjs) => ictToUtc({ year: d.year(), month: d.month() + 1, day: d.date(), hour: d.hour(), minute: d.minute() });
  const stale = publish.error instanceof ApiError && (publish.error.status === 409 || publish.error.status === 412);
  const groups = diff.data ? groupDiff(diff.data.changes, true) : null;

  return (
    <Modal
      open={version !== null}
      title={t('content_studio.release.publishTitle', { version })}
      okText={mode === 'now' ? t('content_studio.release.publishNow') : t('content_studio.release.schedule')}
      cancelText={t('content_studio.cancel')}
      confirmLoading={publish.isPending}
      destroyOnHidden
      onCancel={close}
      onOk={() => {
        if (version === null) return;
        let publishAt: string | undefined;
        if (mode === 'schedule') {
          const utc = at ? pickedUtc(at) : null;
          if (!utc || !isScheduleAllowed(utc, now(), SCHEDULED_PUBLISH_MIN_LEAD_MIN)) {
            setTimeError(t('content_studio.release.scheduleTooEarly', { at: formatIct(earliest) }));
            return;
          }
          publishAt = utc.toISOString();
        }
        publish.mutate(
          { ...item, version, ...(publishAt ? { publishAt } : {}), ...(note.trim() ? { changeNote: note.trim() } : {}) },
          {
            onSuccess: (r) => {
              if (r.status === 'scheduled' || publishAt) message.success(t('content_studio.release.scheduled', { at: formatIct(r.scheduled_for ?? publishAt) }));
              else message.success(t('content_studio.release.published', { seconds: PUBLISH_TO_VISIBLE_MAX_S }));
              close();
            },
          },
        );
      }}
    >
      <Flex vertical gap="middle">
        <Radio.Group value={mode} onChange={(e) => setMode(e.target.value as 'now' | 'schedule')} aria-label={t('content_studio.release.when')}>
          <Radio value="now">{t('content_studio.release.whenNow')}</Radio>
          <Radio value="schedule">{t('content_studio.release.whenSchedule')}</Radio>
        </Radio.Group>
        {mode === 'schedule' && (
          <Flex vertical gap="small">
            <DatePicker
              aria-label={t('content_studio.release.publishAt')}
              showTime={{ format: 'HH:mm' }}
              format="DD/MM/YYYY HH:mm"
              needConfirm={false}
              value={at}
              onChange={(d) => {
                setAt(d);
                setTimeError(null);
              }}
              disabledDate={(d) => d.year() * 10000 + (d.month() + 1) * 100 + d.date() < earliestWall.year * 10000 + earliestWall.month * 100 + earliestWall.day}
              status={timeError ? 'error' : undefined}
            />
            <Typography.Text type="secondary">{t('content_studio.release.ictHint', { at: formatIct(earliest), minutes: SCHEDULED_PUBLISH_MIN_LEAD_MIN })}</Typography.Text>
            {timeError && (
              <Typography.Text type="danger" role="alert">
                {timeError}
              </Typography.Text>
            )}
          </Flex>
        )}
        <section aria-label={t('content_studio.release.diffSummary')} data-testid="publish-diff-summary">
          <Typography.Text strong>{t('content_studio.release.diffSummary')}</Typography.Text>
          {current === undefined || current === version ? (
            <Typography.Paragraph type="secondary">{t('content_studio.release.firstPublish')}</Typography.Paragraph>
          ) : diff.isPending ? (
            <Skeleton active paragraph={{ rows: 1 }} />
          ) : groups ? (
            <Typography.Paragraph>
              {t('content_studio.release.diffCounts', { current, text: groups.text.length, answer: groups.answer.length, media: groups.media.length, metadata: groups.metadata.length })}
            </Typography.Paragraph>
          ) : null}
        </section>
        {notReady.length > 0 && <Alert type="warning" showIcon message={t('content_studio.release.mediaNotReady', { count: notReady.length, names: notReady.map((a) => a.original_filename).join(', ') })} />}
        <Input.TextArea aria-label={t('content_studio.question.changeNote')} placeholder={t('content_studio.question.changeNote')} maxLength={LIMITS.changeNote} showCount value={note} onChange={(e) => setNote(e.target.value)} />
        {publish.error ? <ProblemMessage error={publish.error} /> : null}
        {stale && <Alert type="info" showIcon message={t('content_studio.release.stale')} />}
      </Flex>
    </Modal>
  );
}
