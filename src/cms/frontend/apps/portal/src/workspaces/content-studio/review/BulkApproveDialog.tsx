// BulkApproveDialog (F054-AC-14, ADMIN_PORTAL §4): approves up to `content_review_bulk_max` selected tasks, one call per
// item with its own Idempotency-Key (stable for the dialog, so "Thử lại" reuses it), and reports the result of each
// item — including the server refusal when the reviewer authored the version (reviewer ≠ author, 403). Tasks whose
// version carries an audio exemption need the explicit acknowledgement of the reviewer (F025-AC-16).
import { useQueryClient } from '@tanstack/react-query';
import { Alert, Button, Checkbox, Flex, List, Modal, Tag, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { usePortalRuntime } from '../../../app/runtime';
import { useMe } from '../../../shared/api/hooks/useMe';
import { ApiError } from '../../../shared/api/middleware/problem';
import { problemOf } from '../../../shell/ProblemMessage';
import { useContentAccess } from '../shared/access';
import { CONTENT_REVIEW_BULK_MAX } from '../shared/config';
import { bulkAllowed, taskKey, type ReviewTask } from './reviewRules';
import { bulkIntent, decide } from './useReviewTasks';

type ItemResult = { state: 'pending' | 'ok' | 'error' | 'skipped'; message?: string };

export function BulkApproveDialog({ open, tasks, onClose }: { open: boolean; tasks: readonly ReviewTask[]; onClose: () => void }) {
  const { t } = useTranslation();
  const { api } = usePortalRuntime();
  const me = useMe();
  const qc = useQueryClient();
  const [runId, setRunId] = useState(() => crypto.randomUUID());
  const [ack, setAck] = useState(false);
  const [running, setRunning] = useState(false);
  const [results, setResults] = useState<Record<string, ItemResult>>({});
  const { platform } = useContentAccess();
  const needsAck = tasks.some((x) => x.audio_exempt_reason);
  const allowed = bulkAllowed(tasks.length);

  const run = async () => {
    setRunning(true);
    for (const task of tasks) {
      const k = taskKey(task);
      if (results[k]?.state === 'ok') continue;
      if (task.audio_exempt_reason && !ack) {
        setResults((r) => ({ ...r, [k]: { state: 'skipped', message: t('content_studio.review.ackNeeded') } }));
        continue;
      }
      setResults((r) => ({ ...r, [k]: { state: 'pending' } }));
      try {
        await decide(api, { contentType: task.content_type, contentId: task.content_id, ...(platform && task.tenant_id ? { tenantId: task.tenant_id } : {}), version: task.version, action: 'approve', audioExemptionAcknowledged: !!task.audio_exempt_reason && ack }, bulkIntent(runId, task));
        setResults((r) => ({ ...r, [k]: { state: 'ok' } }));
      } catch (e) {
        const own = e instanceof ApiError && e.status === 403 && !!task.author_id && task.author_id === me.data?.user_id;
        setResults((r) => ({ ...r, [k]: { state: 'error', message: own ? t('content_studio.review.ownContent') : t(problemOf(e).key) } }));
      }
    }
    setRunning(false);
    void qc.invalidateQueries({ queryKey: ['content-studio'] });
  };

  const close = () => {
    setResults({});
    setAck(false);
    setRunId(crypto.randomUUID());
    onClose();
  };
  const done = tasks.length > 0 && tasks.every((x) => results[taskKey(x)]?.state === 'ok');
  return (
    <Modal
      open={open}
      title={t('content_studio.review.bulkTitle', { count: tasks.length })}
      onCancel={close}
      destroyOnHidden
      footer={
        <Flex justify="end" gap="small">
          <Button
            onClick={close}
          >
            {t('content_studio.review.close')}
          </Button>
          <Button type="primary" disabled={!allowed || done} loading={running} onClick={() => void run()}>
            {Object.keys(results).length ? t('content_studio.review.retryFailed') : t('content_studio.review.approveAll', { count: tasks.length })}
          </Button>
        </Flex>
      }
    >
      <Flex vertical gap="small">
        {!allowed && <Alert type="error" showIcon message={t('content_studio.review.bulkLimit', { max: CONTENT_REVIEW_BULK_MAX })} />}
        {needsAck && (
          <Checkbox checked={ack} onChange={(e) => setAck(e.target.checked)}>
            {t('content_studio.review.ackAudio')}
          </Checkbox>
        )}
        <List
          size="small"
          data-testid="bulk-results"
          dataSource={[...tasks]}
          renderItem={(task) => {
            const r = results[taskKey(task)];
            return (
              <List.Item key={taskKey(task)} data-result={r?.state ?? 'waiting'}>
                <Flex vertical>
                  <Typography.Text>{t('content_studio.review.itemLabel', { title: task.title ?? task.content_id, version: task.version })}</Typography.Text>
                  {r?.message ? <Typography.Text type={r.state === 'error' ? 'danger' : 'secondary'}>{r.message}</Typography.Text> : null}
                </Flex>
                {r ? <Tag color={r.state === 'ok' ? 'success' : r.state === 'error' ? 'error' : 'default'}>{t(`content_studio.review.result.${r.state}`)}</Tag> : null}
              </List.Item>
            );
          }}
        />
      </Flex>
    </Modal>
  );
}
