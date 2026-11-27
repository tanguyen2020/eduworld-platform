// "Tải bộ cài" of workspace C (F134 T035, T044, F134-AC-17, F134-AC-18, ADMIN_PORTAL §10, ADR-0101 mục 7–8): screen of
// the catalog menu `menu.school_lms.installers` (path, permission `school.installer.read` and the disabled state come from
// the generated registry — no hand-registered menu). For one school of the user's scope it lists the Android APK/MDM kits
// returned by listSchoolClientInstallers (platform, version, grade bands, minimum OS, size, SHA-256 with a copy button,
// download link), the "Mở bản Web" card (web_client_url + WebGL 2 check) and the `device.cfg` guide. School: the only
// school of the scope is used directly (School Admin, `school` tenant); with several (Tenant Admin of a `unit` tenant)
// the user picks one first in a searchable dropdown loaded from every page. A System Admin picks one tenant in the topbar.
// Errors by Problem `code`: 503 UNAVAILABLE (index not loadable / bad signature) → message and NO link; 403
// FEATURE_DISABLED → "tính năng tạm tắt" (the shell reloads getMyPermissions); 404 → school not in scope. Display only.
import { DownloadOutlined } from '@ant-design/icons';
import { SearchSelect } from '@eduworld/ops-ui';
import { Alert, App as AntApp, Button, Flex, Form, Space, Tag, Typography } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { ApiError } from '../../../shared/api/middleware/problem';
import { localizedName, formatDateTime } from '../common/format';
import { ResponsiveTable } from '../common/ResponsiveTable';
import { SingleTenantGate } from '../common/scope';
import { PageTitle } from '../common/ui';
import { useActionEnabled } from '../lms/LmsUi';
import { useSchoolInstallers, useScopeSchools, type SchoolClientInstaller } from './api';
import { DeviceConfigGuide } from './DeviceConfigGuide';
import { formatSize } from './format';
import { WebClientCard } from './WebClientCard';

/** Prefix of the keys of this screen (trailing dot: the i18n key check treats it as a prefix). */
const K = 'portal.school_lms.installers.';
const VIEW = 'action.school_lms.installers.view' as const;

/** Message key of an error of listSchoolClientInstallers, chosen by the Problem `code` (then the shared `error.<CODE>`). */
export function installerErrorKey(error: unknown): string {
  if (!(error instanceof ApiError)) return 'error.generic';
  if (error.code === 'UNAVAILABLE') return `${K}errors.UNAVAILABLE`;
  if (error.code === 'FEATURE_DISABLED') return 'portal.school_lms.errors.FEATURE_DISABLED';
  if (error.code === 'PERMISSION_DENIED') return 'portal.school_lms.errors.PERMISSION_DENIED';
  if (error.status === 404) return `${K}errors.NOT_FOUND`;
  return error.i18nKey;
}

function InstallerError({ error }: { error: unknown }) {
  const { t } = useTranslation();
  const traceId = error instanceof ApiError ? error.traceId : undefined;
  const disabled = error instanceof ApiError && error.code === 'FEATURE_DISABLED';
  return (
    <Alert
      type={disabled ? 'warning' : 'error'}
      showIcon
      role="alert"
      data-testid="installers-error"
      message={t(installerErrorKey(error))}
      description={traceId ? <Typography.Text type="secondary">{t('shell.problem.traceId', { traceId })}</Typography.Text> : undefined}
    />
  );
}

function CopySha({ sha }: { sha: string }) {
  const { t } = useTranslation();
  const { message } = AntApp.useApp();
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(sha);
      message.success(t(`${K}copied`));
    } catch {
      message.error(t(`${K}copyFailed`));
    }
  };
  return (
    <Space size="small" align="start">
      <Typography.Text code style={{ wordBreak: 'break-all' }} data-testid="installer-sha256">
        {sha}
      </Typography.Text>
      <Button size="small" aria-label={t(`${K}copySha`)} onClick={() => void copy()}>
        {t(`${K}copy`)}
      </Button>
    </Space>
  );
}

function InstallerTable({ rows }: { rows: readonly SchoolClientInstaller[] }) {
  const { t, i18n } = useTranslation();
  const columns: ColumnType<SchoolClientInstaller>[] = [
    { key: 'platform', title: t(`${K}col.platform`), render: (_, r) => t(`${K}platform.${r.platform}`) },
    { key: 'version', title: t(`${K}col.version`), dataIndex: 'client_version' },
    {
      key: 'grades',
      title: t(`${K}col.gradeBands`),
      render: (_, r) => (
        <Space size="small" wrap>
          {r.grade_bands.map((g) => (
            <Tag key={g}>{t(`portal.school_lms.game_report.gradeBand.${g}`)}</Tag>
          ))}
        </Space>
      ),
    },
    { key: 'min_os', title: t(`${K}col.minOs`), dataIndex: 'min_os' },
    { key: 'size', title: t(`${K}col.size`), render: (_, r) => formatSize(r.size_bytes, i18n.language) },
    { key: 'sha256', title: t(`${K}col.sha256`), render: (_, r) => <CopySha sha={r.sha256} /> },
    { key: 'released', title: t(`${K}col.releasedAt`), render: (_, r) => formatDateTime(r.released_at) },
    {
      key: 'download',
      title: t(`${K}col.download`),
      render: (_, r) => (
        <ActionGate actionKey={VIEW}>
          <Button type="primary" icon={<DownloadOutlined />} href={r.url} download aria-label={t(`${K}downloadOf`, { version: r.client_version, grades: r.grade_bands.map((g) => t(`portal.school_lms.game_report.gradeBand.${g}`)).join(", ") })}>
            {t(`${K}download`)}
          </Button>
        </ActionGate>
      ),
    },
  ];
  return (
    <ResponsiveTable<SchoolClientInstaller>
      rowKey="kit_id"
      columns={columns}
      data={rows}
      cardTitle={(r) => `${t(`${K}platform.${r.platform}`)} ${r.client_version}`}
      caption={t(`${K}tableCaption`)}
    />
  );
}

function Installers() {
  const { t, i18n } = useTranslation();
  const canView = useActionEnabled(VIEW);
  const schools = useScopeSchools(canView);
  const [picked, setPicked] = useState<string | undefined>();
  const several = schools.items.length > 1;
  const schoolId = picked ?? (schools.items.length === 1 ? schools.items[0]!.school_id : undefined);
  const q = useSchoolInstallers(schoolId, canView);

  if (!canView) return <Alert type="warning" showIcon role="status" data-testid="installers-disabled" message={t('portal.school_lms.errors.FEATURE_DISABLED')} />;
  return (
    <Flex vertical gap="middle">
      <Typography.Paragraph type="secondary">{t(`${K}intro`)}</Typography.Paragraph>
      {several ? (
        <Form layout="vertical" name="installer_school">
          <Form.Item label={t('portal.school_lms.common.school')} style={{ marginBottom: 0 }}>
            <SearchSelect<string>
              aria-label={t('portal.school_lms.common.school')}
              placeholder={t('portal.school_lms.common.pickSchool')}
              value={picked}
              onChange={(v?: string) => setPicked(v)}
              loading={schools.isFetching}
              truncated={schools.truncated}
              popupMatchSelectWidth={false}
              options={schools.items.map((s) => ({ value: s.school_id, label: localizedName(s.name, i18n.language) }))}
              style={{ minWidth: '16rem' }}
            />
          </Form.Item>
        </Form>
      ) : null}
      {schools.error ? <InstallerError error={schools.error} /> : null}
      {!schools.isFetching && !schools.error && schools.items.length === 0 ? <Alert type="info" showIcon role="status" message={t(`${K}noSchool`)} /> : null}
      {several && !schoolId ? <Alert type="info" showIcon role="status" data-testid="installers-pick-school" message={t(`${K}pickSchool`)} /> : null}
      {q.error ? <InstallerError error={q.error} /> : null}
      {q.isFetching && !q.data ? <Typography.Text type="secondary">{t(`${K}loading`)}</Typography.Text> : null}
      {q.data && !q.error ? (
        <>
          <Typography.Text type="secondary" data-testid="installers-index-time">
            {t(`${K}indexGeneratedAt`, { time: formatDateTime(q.data.index_generated_at) })}
          </Typography.Text>
          {q.data.data.length ? <InstallerTable rows={q.data.data} /> : <Alert type="info" showIcon role="status" message={t(`${K}empty`)} />}
          <Typography.Text type="secondary">{t(`${K}shaHelp`)}</Typography.Text>
          {q.data.web_client_url ? (
            <ActionGate actionKey={VIEW}>
              <WebClientCard url={q.data.web_client_url} />
            </ActionGate>
          ) : null}
        </>
      ) : null}
      <DeviceConfigGuide />
    </Flex>
  );
}

export function InstallerListPage({ title }: { title: string }) {
  return (
    <section data-testid="installers-page">
      <PageTitle title={title} />
      <SingleTenantGate>{() => <Installers />}</SingleTenantGate>
    </section>
  );
}

export default InstallerListPage;
