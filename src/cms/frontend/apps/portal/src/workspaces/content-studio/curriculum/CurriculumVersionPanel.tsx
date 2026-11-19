// CurriculumVersionPanel (F053-AC-30, CURRICULUM_MODEL §10): status of the selected curriculum version and its
// workflow — Gửi duyệt (curriculum.write), Duyệt / Từ chối (curriculum.publish, reason required to reject), Xuất bản
// (curriculum.publish). When the server refuses because LOs lack `yccd_ref` (422 VALIDATION_FAILED on `yccd_ref`,
// F021-AC-14) the LO codes are listed.
import { Alert, Button, Space, Typography } from 'antd';
import { DomainTag } from '@eduworld/ops-ui';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ApiError } from '../../../shared/api/middleware/problem';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { ReasonDialog } from '../../../shell/ReasonDialog';
import { LIMITS } from '../shared/config';
import type { CurriculumVersion } from './useCurriculum';

export interface CurriculumVersionPanelProps {
  version: CurriculumVersion;
  can: { write: boolean; publish: boolean };
  readOnly: boolean;
  onTransition: (action: 'submit_review' | 'approve' | 'reject', note?: string) => Promise<unknown>;
  onPublish: () => Promise<unknown>;
}

/** LO codes of a "missing yccd_ref" refusal (`errors[].field = yccd_ref`, message `…: CODE1, CODE2`). */
export function missingYccd(err: unknown): string[] | null {
  if (!(err instanceof ApiError) || err.code !== 'VALIDATION_FAILED') return null;
  const fe = err.fieldErrors.yccd_ref ?? err.fieldErrors['/yccd_ref'];
  if (!fe) return null;
  const list = fe.message?.split(':').slice(1).join(':') ?? '';
  return list
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean);
}

export function CurriculumVersionPanel({ version, can, readOnly, onTransition, onPublish }: CurriculumVersionPanelProps) {
  const { t } = useTranslation();
  const [error, setError] = useState<unknown>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [rejecting, setRejecting] = useState(false);
  const run = (name: string, fn: () => Promise<unknown>) => {
    setBusy(name);
    setError(null);
    fn()
      .catch(setError)
      .finally(() => setBusy(null));
  };
  const missing = missingYccd(error);
  return (
    <Space direction="vertical" style={{ width: '100%' }} data-testid="curriculum-version-panel">
      <Space wrap>
        <DomainTag kind="content_status" value={version.status} />
        <Typography.Text strong>{t('content_studio.curriculum.version', { label: version.version_label })}</Typography.Text>
        {!readOnly && version.status === 'draft' && can.write && (
          <ActionGate actionKey="action.content_studio.curriculum.edit" mode="disable">
            <Button loading={busy === 'submit'} onClick={() => run('submit', () => onTransition('submit_review'))}>
              {t('content_studio.curriculum.submit')}
            </Button>
          </ActionGate>
        )}
        {!readOnly && version.status === 'in_review' && can.publish && (
          <>
            <ActionGate actionKey="action.content_studio.curriculum.edit" mode="disable">
              <Button type="primary" loading={busy === 'approve'} onClick={() => run('approve', () => onTransition('approve'))}>
                {t('content_studio.curriculum.approve')}
              </Button>
            </ActionGate>
            <ActionGate actionKey="action.content_studio.curriculum.edit" mode="disable">
              <Button danger onClick={() => setRejecting(true)}>
                {t('content_studio.curriculum.reject')}
              </Button>
            </ActionGate>
          </>
        )}
        {!readOnly && version.status === 'approved' && can.publish && (
          <ActionGate actionKey="action.content_studio.curriculum.publish" mode="disable">
            <Button type="primary" loading={busy === 'publish'} onClick={() => run('publish', onPublish)}>
              {t('content_studio.curriculum.publish')}
            </Button>
          </ActionGate>
        )}
      </Space>
      {missing && missing.length > 0 ? (
        <Alert
          type="error"
          showIcon
          role="alert"
          message={t('content_studio.curriculum.missingYccd')}
          description={
            <ul data-testid="missing-yccd">
              {missing.map((c) => (
                <li key={c}>{c}</li>
              ))}
            </ul>
          }
        />
      ) : error ? (
        <ProblemMessage error={error} />
      ) : null}
      <ReasonDialog
        open={rejecting}
        title={t('content_studio.curriculum.rejectTitle')}
        maxLength={LIMITS.curriculumNote}
        confirmLoading={busy === 'reject'}
        onCancel={() => setRejecting(false)}
        onSubmit={(note) => {
          setRejecting(false);
          run('reject', () => onTransition('reject', note));
        }}
      />
    </Space>
  );
}
