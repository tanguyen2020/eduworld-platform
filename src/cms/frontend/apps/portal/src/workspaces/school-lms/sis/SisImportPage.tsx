// "Nhập CSV" (F052 T025, F052-AC-08, F052-AC-09, F052-AC-11, F052-AC-27): school, data kind (one per file, SisImportCreate
// `entity_kinds` maxItems 1), dry run, file → useSisImport. Columns follow the EduWorld SIS Contract (specs/sis); the
// automatic SIS schedule (`sis_sync_schedule`) is set by EduWorld, not here. Large screens only (< 768 px → notice).
import { Alert, Button, Form, Select, Space, Switch, Typography, Upload } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { LargeScreenRequired } from '../../../shell/StatusPages';
import { useMediaQuery } from '../../../shell/useMediaQuery';
import { ProblemMessage } from '../common/errors';
import { MOBILE_QUERY } from '../common/ResponsiveTable';
import { checkCsvFile } from './fileCheck';
import { SisImportProgress } from './SisImportProgress';
import { useSisImport, type EntityKind } from './useSisImport';

const KINDS: EntityKind[] = ['students', 'classes', 'enrollments'];

export function SisImportPage({ schoolId }: { schoolId: string }) {
  const { t } = useTranslation();
  const desktop = useMediaQuery(MOBILE_QUERY);
  const sis = useSisImport(schoolId);
  const [file, setFile] = useState<File | null>(null);
  const [kind, setKind] = useState<EntityKind>('students');
  const [dryRun, setDryRun] = useState(true);
  if (!desktop) return <LargeScreenRequired />;
  const fileProblem = file ? checkCsvFile(file) : null;
  const { state } = sis;
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Typography.Paragraph type="secondary">{t('portal.school_lms.sis.help')}</Typography.Paragraph>
      <Form layout="vertical">
        <Form.Item label={t('portal.school_lms.sis.kind')}>
          <Select aria-label={t('portal.school_lms.sis.kind')} value={kind} onChange={setKind} options={KINDS.map((k) => ({ value: k, label: t(`portal.school_lms.sis.kinds.${k}`) }))} />
        </Form.Item>
        <Form.Item label={t('portal.school_lms.sis.dryRun')} help={t('portal.school_lms.sis.dryRunHelp')}>
          <Switch checked={dryRun} onChange={setDryRun} aria-label={t('portal.school_lms.sis.dryRun')} />
        </Form.Item>
        <Form.Item label={t('portal.school_lms.sis.file')} validateStatus={fileProblem ? 'error' : undefined} help={fileProblem ? t(`portal.school_lms.sis.fileProblems.${fileProblem}`) : undefined}>
          <Upload
            accept=".csv,text/csv"
            maxCount={1}
            beforeUpload={(f) => {
              setFile(f);
              sis.reset();
              return false;
            }}
            onRemove={() => setFile(null)}
          >
            <Button>{t('portal.school_lms.sis.pickFile')}</Button>
          </Upload>
        </Form.Item>
        <ActionGate actionKey="action.school_lms.sis_sync.import">
          <Button type="primary" disabled={!file || !!fileProblem || sis.busy} loading={sis.busy} onClick={() => file && void sis.start(file, kind, dryRun)}>
            {t('portal.school_lms.sis.start')}
          </Button>
        </ActionGate>
      </Form>
      {sis.busy ? <Alert type="info" showIcon message={t(`portal.school_lms.sis.phases.${state.phase}`)} /> : null}
      {state.phase === 'failed' ? (
        state.problem === 'api' ? (
          <ProblemMessage error={state.error} />
        ) : (
          <Alert type="error" showIcon role="alert" message={t(`portal.school_lms.sis.problems.${state.problem}`)} />
        )
      ) : null}
      {sis.trackingError ? <ProblemMessage error={sis.trackingError} /> : null}
      {sis.current ? (
        <SisImportProgress
          value={sis.current}
          onRetry={() => {
            sis.reset();
            if (file) void sis.start(file, kind, dryRun);
          }}
        />
      ) : null}
    </Space>
  );
}
