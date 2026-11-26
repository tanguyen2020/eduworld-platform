// TranslationsPage (F082 T029, F082-AC-09, AC-12…AC-15; ADMIN_PORTAL §1.5): "Dịch nội dung game" of workspace A — shortcuts
// to the items to translate of each game type ("Cần dịch" / "Bản dịch lỗi thời", filtered by the server), the export
// dialog (game types only, items ≤ maxItems loaded from every page, target language, XLIFF 2.1 / CSV, only missing by
// default) followed until its signed link (with its expiry, ICT), and the import dialog (file checked by the server → dry
// run → report → confirm). Buttons inside ActionGate (`action.game_ops.translations.translate`). No machine translation.
import { TranslationExportDialog, TranslationImportDialog, TranslationJobProgress, type TranslationJobView } from '@eduworld/ops-ui/game-console';
import { Alert, Button, Card, Flex, List, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { useQueries } from '@tanstack/react-query';
import { usePortalRuntime } from '../../../app/runtime';
import { ActionGate } from '../../../permissions/ActionGate';
import { useSession } from '../../../session/SessionProvider';
import { fetchAllPages } from '../../../shared/api/fetchAllPages';
import { formatIct } from '../../../shared/release/ict';
import type { ConfigPage, GameConfigVersion } from '../api/gameConfigs';
import { CONFIG_TYPES, codeOf, menuPathOf, nameOf, translatableTypes, type ConfigType } from '../registry/configTypes';
import { GameOpsError } from '../shared/errors';
import { EXPORT_LIMITS, formatOf, useDocumentUpload, useTranslationJob, useTranslationWrites } from './api/gameTranslations';

const ACTION = 'action.game_ops.translations.translate' as const;

export function TranslationsPage({ title }: { title: string }) {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const lang = i18n.language.startsWith('en') ? 'en' : 'vi';
  const types = translatableTypes();
  const [exportOpen, setExportOpen] = useState(false);
  const [importOpen, setImportOpen] = useState(false);
  const [exportJobId, setExportJobId] = useState<string>();

  return (
    <Flex vertical gap="middle" data-testid="translations-page">
      <Typography.Title level={1}>{title}</Typography.Title>
      <Alert type="info" showIcon message={t('gameOps.translations.pageHint')} />
      <Flex gap="small" wrap>
        <ActionGate actionKey={ACTION}>
          <Button type="primary" onClick={() => setExportOpen(true)}>
            {t('gameOps.translations.exportButton')}
          </Button>
        </ActionGate>
        <ActionGate actionKey={ACTION}>
          <Button onClick={() => setImportOpen(true)}>{t('gameOps.translations.importButton')}</Button>
        </ActionGate>
      </Flex>
      {exportJobId && <ExportJob jobId={exportJobId} />}
      <Card size="small" title={t('gameOps.translations.shortcuts')}>
        <List
          size="small"
          dataSource={types}
          renderItem={(d) => (
            <List.Item key={d.type}>
              <Flex gap="small" align="center" wrap>
                <Typography.Text>{t(`gameOps.type.${d.type}`)}</Typography.Text>
                <Button size="small" type="link" onClick={() => navigate(`${menuPathOf(d.menu)}/${d.type}?translation_status=partial`)}>
                  {t('gameOps.translations.filters.partial')}
                </Button>
                <Button size="small" type="link" onClick={() => navigate(`${menuPathOf(d.menu)}/${d.type}?translation_status=outdated`)}>
                  {t('gameOps.translations.filters.outdated')}
                </Button>
              </Flex>
            </List.Item>
          )}
        />
      </Card>
      {exportOpen && <ExportDialog lang={lang} onClose={() => setExportOpen(false)} onStarted={(id) => (setExportJobId(id), setExportOpen(false))} />}
      {importOpen && <ImportDialog onClose={() => setImportOpen(false)} />}
    </Flex>
  );
}

function ExportJob({ jobId }: { jobId: string }) {
  const job = useTranslationJob(jobId);
  if (job.error) return <GameOpsError error={job.error} />;
  if (!job.data) return null;
  return (
    <Card size="small">
      <TranslationJobProgress job={job.data as TranslationJobView} formatTime={formatIct} />
    </Card>
  );
}

function ExportDialog({ lang, onClose, onStarted }: { lang: 'vi' | 'en'; onClose: () => void; onStarted: (jobId: string) => void }) {
  const { t } = useTranslation();
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  const { exportJob } = useTranslationWrites();
  const [types, setTypes] = useState<ConfigType[]>([]);
  // items of the chosen types: every cursor page of each type (dropdown — ADMIN_PORTAL §1.6)
  const lists = useQueries({
    queries: types.map((type) => ({
      queryKey: ['game-ops', 'list', scopeKey, type, { translation: 'export' }, 'all-pages'],
      queryFn: () => fetchAllPages<GameConfigVersion>((page) => api.call<ConfigPage>(api.client.GET('/game-admin/{config_type}', { params: { path: { config_type: type }, query: page as never } }))),
      staleTime: 60_000,
    })),
  });
  const itemOptions = lists.flatMap((q, i) => {
    const def = CONFIG_TYPES[types[i]!];
    return (q.data?.items ?? []).map((v) => ({ value: v.config_id, label: v.title || codeOf(def, v.payload) || nameOf(def, v.payload, lang) || v.config_id }));
  });
  return (
    <TranslationExportDialog
      open
      typeOptions={translatableTypes().map((d) => ({ value: d.type, label: t(`gameOps.type.${d.type}`) }))}
      itemOptions={itemOptions}
      itemsLoading={lists.some((q) => q.isPending)}
      itemsTruncated={lists.some((q) => q.data?.truncated)}
      maxItems={EXPORT_LIMITS.contentIds}
      maxTypes={EXPORT_LIMITS.contentTypes}
      onTypesChange={(v) => setTypes(v as ConfigType[])}
      submitting={exportJob.isPending}
      error={exportJob.error ? <GameOpsError error={exportJob.error} /> : null}
      onCancel={onClose}
      onSubmit={(v) =>
        exportJob.mutate(
          {
            ...(v.content_types.length ? { content_types: v.content_types } : {}),
            ...(v.content_ids.length ? { content_ids: v.content_ids } : {}),
            source_language: v.source_language,
            target_language: v.target_language,
            format: v.format,
            only_missing: v.only_missing,
          },
          { onSuccess: (job) => onStarted(job.job_id) },
        )
      }
    />
  );
}

function ImportDialog({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation();
  const file = useDocumentUpload();
  const { importJob } = useTranslationWrites();
  const [target, setTarget] = useState<'vi' | 'en'>('en');
  const [name, setName] = useState<string>();
  const [dryId, setDryId] = useState<string>();
  const [realId, setRealId] = useState<string>();
  const dry = useTranslationJob(dryId);
  const real = useTranslationJob(realId);
  const format = name ? formatOf(name) : null;
  const run = (dryRun: boolean) =>
    file.assetId &&
    format &&
    importJob.mutate({ asset_id: file.assetId, format, target_language: target, dry_run: dryRun }, { onSuccess: (job) => (dryRun ? setDryId(job.job_id) : setRealId(job.job_id)) });
  const uploadError =
    name && !format ? (
      <Alert type="error" showIcon message={t('gameOps.translations.badFormat')} />
    ) : file.state === 'error' ? (
      file.error ? <GameOpsError error={file.error} /> : <Alert type="error" showIcon message={t('gameOps.translations.fileRejected', { reason: file.rejectReason ?? '' })} />
    ) : null;
  return (
    <TranslationImportDialog
      open
      target={target}
      onTargetChange={(l) => (setTarget(l), setDryId(undefined))}
      onFile={(f) => {
        setName(f.name);
        setDryId(undefined);
        setRealId(undefined);
        void file.start(f);
      }}
      fileName={name}
      uploadState={name && !format ? 'error' : file.state}
      uploadError={uploadError}
      dryRunJob={dry.data as TranslationJobView | undefined}
      importJob={real.data as TranslationJobView | undefined}
      busy={importJob.isPending}
      error={importJob.error ? <GameOpsError error={importJob.error} /> : null}
      onDryRun={() => run(true)}
      onConfirm={() => run(false)}
      onClose={onClose}
      formatTime={formatIct}
    />
  );
}
