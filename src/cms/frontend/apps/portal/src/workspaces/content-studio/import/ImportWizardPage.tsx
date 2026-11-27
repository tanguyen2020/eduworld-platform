// ImportWizardPage (F054-AC-27, AC-28, AC-30, FR-QB-011, SC-003): question import from Excel / CSV in four steps —
// ① choose the file (read in the browser), ② formulas rendered with MathJax 4 and errors listed by row before anything
// is sent, ③ upload + dry run (`dry_run = true`): server errors by row (`row`, `field`, `code`, `message`; a full list
// of 500 says it was cut), ④ real run creating drafts. Needs `learning_content.import` (ActionGate) and ≥ 1024 px. The
// source "Word (.docx)" switches to the Word import of F028 (DocxImportPanel).
import { Alert, App as AntApp, Button, Card, Flex, Form, Input, Progress, Segmented, Steps, Table, Typography, Upload } from 'antd';
import { InboxOutlined } from '@ant-design/icons';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { LargeScreenRequired } from '../../../shell/StatusPages';
import { useMediaQuery } from '../../../shell/useMediaQuery';
import { EDITOR_MIN_WIDTH_QUERY, LIMITS } from '../shared/config';
import { ImportParseError, parseImportFile, type ImportTable } from './parseImportFile';
import { renderImportMath, type MathRenderResult, type RowIssue } from './renderImportMath';
import { IMPORT_ERRORS_MAX, useContentImport, type ContentImportJob } from './useContentImport';
import { DocxImportPanel } from './docx/DocxImportPanel';

type Phase = 'choose' | 'checked' | 'dry_run' | 'reviewed' | 'running' | 'done';

export function ImportWizardPage({ pollMs }: { pollMs?: number }) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { message } = AntApp.useApp();
  const wide = useMediaQuery(EDITOR_MIN_WIDTH_QUERY);
  const imp = useContentImport({ pollMs });
  const [form] = Form.useForm<{ license: string }>();
  const [phase, setPhase] = useState<Phase>('choose');
  const [file, setFile] = useState<File | null>(null);
  const [table, setTable] = useState<ImportTable | null>(null);
  const [math, setMath] = useState<MathRenderResult | null>(null);
  const [progress, setProgress] = useState(0);
  const [assetId, setAssetId] = useState<string | null>(null);
  const [dry, setDry] = useState<ContentImportJob | null>(null);
  const [real, setReal] = useState<ContentImportJob | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<unknown>(null);
  const [parseError, setParseError] = useState<string | null>(null);
  const [source, setSource] = useState<'sheet' | 'docx'>('sheet');

  if (!wide) return <LargeScreenRequired />;

  const choose = async (f: File) => {
    setFile(f);
    setParseError(null);
    setError(null);
    setTable(null);
    setMath(null);
    setDry(null);
    setReal(null);
    setAssetId(null);
    setBusy(true);
    try {
      const tb = await parseImportFile(f);
      setTable(tb);
      const m = await renderImportMath(tb, (d, n) => setProgress(Math.round((d / n) * 100)));
      setMath(m);
      setPhase('checked');
    } catch (e) {
      setParseError(e instanceof ImportParseError ? t(`content_studio.import.parse.${e.code}`, { detail: e.detail }) : t('content_studio.import.parse.INVALID_FORMAT', { detail: '' }));
      setPhase('choose');
    } finally {
      setBusy(false);
    }
  };

  const dryRun = async () => {
    if (!math || !file) return;
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    setBusy(true);
    setError(null);
    try {
      const normalized = new File([math.csv], file.name.replace(/\.(xlsx|csv)$/i, '') + '.csv', { type: 'text/csv' });
      const id = assetId ?? (await imp.uploadFile(normalized, v.license.trim()));
      setAssetId(id);
      setPhase('dry_run');
      const job = await imp.run(id, true);
      setDry(job);
      setPhase('reviewed');
    } catch (e) {
      setError(e);
      setPhase('checked');
    } finally {
      setBusy(false);
    }
  };

  const runReal = async () => {
    if (!assetId) return;
    setBusy(true);
    setError(null);
    setPhase('running');
    try {
      const job = await imp.run(assetId, false);
      setReal(job);
      setPhase('done');
      if (job.status === 'succeeded') message.success(t('content_studio.import.created', { count: job.created_count ?? 0 }));
    } catch (e) {
      setError(e);
      setPhase('reviewed');
    } finally {
      setBusy(false);
    }
  };

  const step = { choose: 0, checked: 1, dry_run: 2, reviewed: 2, running: 3, done: 3 }[phase];
  const mathIssues = math?.issues ?? [];
  const serverErrors = (real ?? dry)?.errors ?? [];
  const issueColumns = [
    { key: 'row', title: t('content_studio.import.col.row'), dataIndex: 'row' },
    { key: 'field', title: t('content_studio.import.col.field'), dataIndex: 'field' },
    { key: 'code', title: t('content_studio.import.col.code'), render: (_: unknown, r: RowIssue) => t(`content_studio.import.issue.${r.code}`, { defaultValue: r.code }) },
    { key: 'latex', title: t('content_studio.import.col.formula'), render: (_: unknown, r: RowIssue) => (r.latex ? <Typography.Text code>{r.latex.slice(0, 80)}</Typography.Text> : null) },
  ];
  const serverColumns = [
    { key: 'row', title: t('content_studio.import.col.row'), dataIndex: 'row' },
    { key: 'field', title: t('content_studio.import.col.field'), dataIndex: 'field' },
    { key: 'code', title: t('content_studio.import.col.code'), dataIndex: 'code' },
    { key: 'message', title: t('content_studio.import.col.message'), dataIndex: 'message' },
  ];

  return (
    <section aria-labelledby="ew-cs-import">
      <Flex vertical gap="middle">
        <Flex justify="space-between" align="center">
          <Typography.Title level={1} id="ew-cs-import">
            {t('content_studio.import.title')}
          </Typography.Title>
          <Button onClick={() => navigate('..')}>{t('content_studio.import.back')}</Button>
        </Flex>
        <Segmented
          aria-label={t('content_studio.import.source')}
          value={source}
          onChange={(v) => setSource(v as 'sheet' | 'docx')}
          options={[
            { value: 'sheet', label: t('content_studio.import.sourceSheet') },
            { value: 'docx', label: t('content_studio.import.sourceDocx') },
          ]}
        />
        {source === 'docx' ? <DocxImportPanel pollMs={pollMs} /> : null}
        {source === 'sheet' && <Steps current={step} items={(['choose', 'math', 'dryRun', 'run'] as const).map((s) => ({ title: t(`content_studio.import.steps.${s}`) }))} />}
        {source === 'sheet' && (
        <Card>
          <Flex vertical gap="middle">
            <Upload.Dragger
              accept=".csv,.xlsx"
              multiple={false}
              showUploadList={false}
              disabled={busy || phase === 'running'}
              beforeUpload={(f) => {
                void choose(f);
                return false;
              }}
            >
              <p>
                <InboxOutlined />
              </p>
              <Typography.Paragraph>{file ? file.name : t('content_studio.import.dropHint')}</Typography.Paragraph>
              <Typography.Text type="secondary">{t('content_studio.import.formatHint')}</Typography.Text>
            </Upload.Dragger>
            {busy && phase === 'choose' && <Progress percent={progress} aria-label={t('content_studio.import.rendering')} />}
            {parseError && <Alert type="error" showIcon message={parseError} />}
            {table && math && (
              <Alert
                type={mathIssues.length ? 'warning' : 'success'}
                showIcon
                data-testid="import-math-summary"
                message={t('content_studio.import.checked', { rows: table.rows.length, formulas: math.formulas, issues: mathIssues.length })}
              />
            )}
            {mathIssues.length > 0 && <Table size="small" data-testid="import-math-issues" rowKey={(r) => `${r.row}:${r.field}:${r.latex ?? r.code}`} columns={issueColumns} dataSource={mathIssues} pagination={{ pageSize: 20 }} />}
            {phase !== 'choose' && (
              <Form form={form} layout="vertical" initialValues={{ license: t('content_studio.import.defaultLicense') }}>
                <Form.Item name="license" label={t('content_studio.media.license')} rules={[{ required: true, whitespace: true, message: t('content_studio.media.licenseRequired') }, { max: LIMITS.license }]}>
                  <Input aria-label={t('content_studio.media.license')} maxLength={LIMITS.license} showCount />
                </Form.Item>
              </Form>
            )}
            <ActionGate actionKey="action.content_studio.question_bank.import">
              <Flex gap="small">
                <Button type="primary" disabled={phase !== 'checked' || mathIssues.length > 0} loading={busy && phase !== 'running'} onClick={() => void dryRun()}>
                  {t('content_studio.import.dryRun')}
                </Button>
                <Button type="primary" danger disabled={phase !== 'reviewed' || dry?.status !== 'succeeded'} loading={phase === 'running'} onClick={() => void runReal()}>
                  {t('content_studio.import.run')}
                </Button>
              </Flex>
            </ActionGate>
            {mathIssues.length > 0 && <Typography.Text type="danger">{t('content_studio.import.fixFirst')}</Typography.Text>}
            {error ? <ProblemMessage error={error} /> : null}
            {(dry || real) && (
              <Flex vertical gap="small" data-testid="import-report">
                <Alert
                  type={(real ?? dry)!.status === 'succeeded' && serverErrors.length === 0 ? 'success' : 'warning'}
                  showIcon
                  message={
                    real
                      ? t('content_studio.import.realReport', { status: t(`content_studio.import.status.${real.status}`), created: real.created_count ?? 0, rejected: real.rejected_count ?? 0 })
                      : t('content_studio.import.dryReport', { status: t(`content_studio.import.status.${dry!.status}`), ok: dry!.created_count ?? 0, rejected: dry!.rejected_count ?? 0 })
                  }
                />
                {serverErrors.length >= IMPORT_ERRORS_MAX && <Alert type="info" showIcon message={t('content_studio.import.truncated', { max: IMPORT_ERRORS_MAX, rejected: (real ?? dry)!.rejected_count ?? '?' })} />}
                {serverErrors.length > 0 && <Table size="small" data-testid="import-server-errors" rowKey={(r, i) => `${r.row}:${r.field}:${i}`} columns={serverColumns} dataSource={serverErrors} pagination={{ pageSize: 20 }} />}
              </Flex>
            )}
          </Flex>
        </Card>
        )}
      </Flex>
    </section>
  );
}
