// DocxImportPanel (F028-AC-15, FR-QB-011, CONTENT_PRODUCTION_PLAN §8): Word import of the import screen —
// ① choose the .docx (template "Câu n.", options "A.", bold answer…) and the license, ② upload + analysis job
// (`lessons_docx`, nothing is created), ③ per-question report: summary + server-paged table filtered by status,
// ④ "Tạo bản nháp": the formulas of every importable question are rendered with MathJax 4 and its draft is created
// (one request and one Idempotency-Key per question, progress, result per question). Drafts only, never published.
import { Alert, App as AntApp, Button, Card, Flex, Form, Input, Progress, Table, Tag, Typography, Upload } from 'antd';
import { SearchSelect } from '@eduworld/ops-ui';
import { FileWordOutlined } from '@ant-design/icons';
import { useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../../permissions/ActionGate';
import { useSession } from '../../../../session/SessionProvider';
import { ProblemMessage } from '../../../../shell/ProblemMessage';
import { IMPORT_FILE_MAX_BYTES, LIMITS } from '../../shared/config';
import { useContentImport, type ContentImportJob } from '../useContentImport';
import { importItemsKey, useDocxDrafts, useImportItems, type DraftResult, type ImportItem, type ImportItemStatus } from './useDocxImport';

const STATUSES: readonly ImportItemStatus[] = ['ready', 'warning', 'error', 'created', 'draft_failed'];
const TAG: Record<ImportItemStatus, string> = { ready: 'success', warning: 'warning', error: 'error', created: 'processing', draft_failed: 'error' };

type Phase = 'choose' | 'analysing' | 'report' | 'creating';

export function DocxImportPanel({ pollMs }: { pollMs?: number }) {
  const { t } = useTranslation();
  const { message } = AntApp.useApp();
  const qc = useQueryClient();
  const { scopeKey } = useSession();
  const imp = useContentImport({ pollMs });
  const drafts = useDocxDrafts();
  const [form] = Form.useForm<{ license: string }>();
  const [file, setFile] = useState<File | null>(null);
  const [tooLarge, setTooLarge] = useState(false);
  const [phase, setPhase] = useState<Phase>('choose');
  const [job, setJob] = useState<ContentImportJob | null>(null);
  const [status, setStatus] = useState<ImportItemStatus | undefined>();
  const [error, setError] = useState<unknown>(null);
  const [progress, setProgress] = useState<{ done: number; total: number } | null>(null);
  const [results, setResults] = useState<DraftResult[]>([]);
  const items = useImportItems(job?.status === 'succeeded' ? job.import_id : undefined, status);

  const choose = (f: File) => {
    setFile(f);
    setTooLarge(f.size > IMPORT_FILE_MAX_BYTES);
    setJob(null);
    setResults([]);
    setError(null);
    setPhase('choose');
  };

  const analyse = async () => {
    if (!file) return;
    const v = await form.validateFields().catch(() => null);
    if (!v) return;
    setPhase('analysing');
    setError(null);
    try {
      const assetId = await imp.uploadFile(file, v.license.trim());
      setJob(await imp.run(assetId, true, 'lessons_docx'));
      setPhase('report');
    } catch (e) {
      setError(e);
      setPhase('choose');
    }
  };

  const createAll = async () => {
    if (!job) return;
    setPhase('creating');
    setError(null);
    setResults([]);
    try {
      const todo = await drafts.importable(job.import_id);
      setProgress({ done: 0, total: todo.length });
      const out: DraftResult[] = [];
      for (const it of todo) {
        out.push(await drafts.createOne(job.import_id, it));
        setProgress({ done: out.length, total: todo.length });
      }
      setResults(out);
      const ok = out.filter((r) => r.ok).length;
      if (ok > 0) message.success(t('content_studio.import.docx.created', { count: ok }));
    } catch (e) {
      setError(e);
    } finally {
      setPhase('report');
      await qc.invalidateQueries({ queryKey: importItemsKey(scopeKey, job.import_id, '').slice(0, 4) });
    }
  };

  const total = Number((job?.result as { items_total?: number } | undefined)?.items_total ?? 0);
  const importable = Number((job?.result as { items_importable?: number } | undefined)?.items_importable ?? 0);
  const fileErrors = (job?.errors ?? []).filter((e) => e.field === 'file');
  const failed = results.filter((r) => !r.ok);
  const issueText = (it: ImportItem) =>
    it.issues.map((i) => t(`content_studio.import.docx.issue.${i.code}`, { defaultValue: i.code })).join(' · ');

  const columns = [
    { key: 'no', title: t('content_studio.import.docx.col.question'), render: (_: unknown, r: ImportItem) => r.source_label ?? r.item_no },
    {
      key: 'status',
      title: t('content_studio.import.docx.col.status'),
      render: (_: unknown, r: ImportItem) => <Tag color={TAG[r.status]}>{t(`content_studio.import.docx.status.${r.status}`)}</Tag>,
    },
    { key: 'type', title: t('content_studio.import.docx.col.type'), render: (_: unknown, r: ImportItem) => (r.question_type ? t(`content_studio.import.docx.type.${r.question_type}`) : null) },
    { key: 'stem', title: t('content_studio.import.docx.col.stem'), render: (_: unknown, r: ImportItem) => <Typography.Text ellipsis>{r.stem_preview}</Typography.Text> },
    { key: 'formulas', title: t('content_studio.import.docx.col.formulas'), render: (_: unknown, r: ImportItem) => r.formulas.length },
    { key: 'issues', title: t('content_studio.import.docx.col.issues'), render: (_: unknown, r: ImportItem) => issueText(r) },
  ];

  return (
    <Card>
      <Flex vertical gap="middle">
        <Upload.Dragger
          accept=".docx"
          multiple={false}
          showUploadList={false}
          disabled={phase === 'analysing' || phase === 'creating'}
          beforeUpload={(f) => {
            choose(f);
            return false;
          }}
        >
          <p>
            <FileWordOutlined />
          </p>
          <Typography.Paragraph>{file ? file.name : t('content_studio.import.docx.dropHint')}</Typography.Paragraph>
          <Typography.Text type="secondary">{t('content_studio.import.docx.formatHint')}</Typography.Text>
        </Upload.Dragger>
        {tooLarge && <Alert type="error" showIcon message={t('content_studio.import.docx.tooLarge')} />}
        {file && !tooLarge && (
          <Form form={form} layout="vertical" initialValues={{ license: t('content_studio.import.defaultLicense') }}>
            <Form.Item
              name="license"
              label={t('content_studio.media.license')}
              rules={[{ required: true, whitespace: true, message: t('content_studio.media.licenseRequired') }, { max: LIMITS.license }]}
            >
              <Input aria-label={t('content_studio.media.license')} maxLength={LIMITS.license} showCount />
            </Form.Item>
          </Form>
        )}
        <ActionGate actionKey="action.content_studio.question_bank.import">
          <Flex gap="small">
            <Button type="primary" disabled={!file || tooLarge || phase !== 'choose'} loading={phase === 'analysing'} onClick={() => void analyse()}>
              {t('content_studio.import.docx.analyse')}
            </Button>
            <Button type="primary" danger disabled={phase !== 'report' || importable === 0} loading={phase === 'creating'} onClick={() => void createAll()}>
              {t('content_studio.import.docx.createDrafts')}
            </Button>
          </Flex>
        </ActionGate>
        {error ? <ProblemMessage error={error} /> : null}
        {progress && phase === 'creating' && (
          <Progress percent={progress.total ? Math.round((progress.done / progress.total) * 100) : 100} aria-label={t('content_studio.import.docx.creating')} />
        )}
        {job && (
          <Flex vertical gap="small" data-testid="docx-report">
            <Alert
              type={job.status === 'succeeded' && total === importable ? 'success' : job.status === 'succeeded' ? 'warning' : 'error'}
              showIcon
              message={
                job.status === 'succeeded'
                  ? t('content_studio.import.docx.summary', { total, importable, rejected: total - importable })
                  : t('content_studio.import.docx.failed')
              }
              description={
                fileErrors.length ? fileErrors.map((e) => t(`content_studio.import.docx.issue.${e.code}`, { defaultValue: e.message ?? e.code })).join(' · ') : undefined
              }
            />
            {results.length > 0 && (
              <Alert
                type={failed.length ? 'warning' : 'success'}
                showIcon
                data-testid="docx-results"
                message={t('content_studio.import.docx.results', { created: results.length - failed.length, failed: failed.length })}
                description={
                  failed.length
                    ? failed.map((r) => `${r.label}: ${t(`content_studio.import.docx.issue.${r.code ?? 'ERROR'}`, { defaultValue: r.code })}`).join(' · ')
                    : undefined
                }
              />
            )}
            {job.status === 'succeeded' && (
              <>
                <SearchSelect<ImportItemStatus>
                  allowClear
                  aria-label={t('content_studio.import.docx.filter')}
                  placeholder={t('content_studio.import.docx.filter')}
                  value={status}
                  onChange={(v) => setStatus(v)}
                  options={STATUSES.map((s) => ({ value: s, label: t(`content_studio.import.docx.status.${s}`) }))}
                />
                <Table<ImportItem>
                  size="small"
                  data-testid="docx-items"
                  rowKey={(r) => String(r.item_no)}
                  loading={items.isFetching}
                  columns={columns}
                  dataSource={items.rows}
                  pagination={false}
                />
                {items.hasNextPage && (
                  <Button onClick={() => void items.fetchNextPage()} loading={items.isFetchingNextPage}>
                    {t('content_studio.import.docx.more')}
                  </Button>
                )}
                {importable > 0 && <Typography.Text type="secondary">{t('content_studio.import.docx.draftNote')}</Typography.Text>}
              </>
            )}
          </Flex>
        )}
      </Flex>
    </Card>
  );
}
