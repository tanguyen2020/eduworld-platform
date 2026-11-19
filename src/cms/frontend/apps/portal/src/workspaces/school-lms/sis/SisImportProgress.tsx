// Progress/result of a CSV import (F052 T025, F052-AC-09): status, rows_total / rows_applied / rows_rejected /
// reconciliation_issue_count, row errors; `failed`/`expired` → message + "Tải lại".
import { Alert, Button, Descriptions, Progress, Space, Table } from 'antd';
import { useTranslation } from 'react-i18next';
import { StatusTag } from '../common/ui';
import { isFinal, type SisImport } from './useSisImport';

type RowError = NonNullable<SisImport['errors']>[number];

export function SisImportProgress({ value, onRetry }: { value: SisImport; onRetry: () => void }) {
  const { t } = useTranslation();
  const done = (value.rows_applied ?? 0) + (value.rows_rejected ?? 0);
  const percent = value.rows_total ? Math.min(100, Math.round((done / value.rows_total) * 100)) : 0;
  const broken = value.status === 'failed' || value.status === 'expired';
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }} data-testid="sis-import-progress">
      <Descriptions column={{ xs: 1, md: 2 }} size="small">
        <Descriptions.Item label={t('portal.school_lms.sis.status')}>
          <StatusTag group="import" value={value.status} />
        </Descriptions.Item>
        <Descriptions.Item label={t('portal.school_lms.sis.dryRun')}>{value.dry_run ? t('portal.school_lms.common.yes') : t('portal.school_lms.common.no')}</Descriptions.Item>
        <Descriptions.Item label={t('portal.school_lms.sis.rowsTotal')}>{value.rows_total ?? 0}</Descriptions.Item>
        <Descriptions.Item label={t('portal.school_lms.sis.rowsApplied')}>{value.rows_applied ?? 0}</Descriptions.Item>
        <Descriptions.Item label={t('portal.school_lms.sis.rowsRejected')}>{value.rows_rejected ?? 0}</Descriptions.Item>
        <Descriptions.Item label={t('portal.school_lms.sis.issues')}>{value.reconciliation_issue_count ?? 0}</Descriptions.Item>
      </Descriptions>
      {!isFinal(value.status) ? <Progress percent={percent} status="active" aria-label={t('portal.school_lms.sis.progress')} /> : null}
      {broken ? (
        <Alert
          type="error"
          showIcon
          role="alert"
          message={t(`portal.school_lms.sis.final.${value.status}`)}
          action={
            <Button onClick={onRetry} size="small">
              {t('portal.school_lms.sis.uploadAgain')}
            </Button>
          }
        />
      ) : null}
      {value.status === 'completed' || value.status === 'completed_with_issues' ? <Alert type={value.status === 'completed' ? 'success' : 'warning'} showIcon message={t(`portal.school_lms.sis.final.${value.status}`)} /> : null}
      {value.errors?.length ? (
        <Table<RowError>
          size="small"
          rowKey={(e) => `${e.row_no ?? ''}-${e.column_name ?? ''}-${e.error_code}`}
          pagination={{ pageSize: 20 }}
          dataSource={value.errors}
          columns={[
            { key: 'row', title: t('portal.school_lms.sis.row'), render: (_, e) => e.row_no ?? '' },
            { key: 'col', title: t('portal.school_lms.sis.column'), render: (_, e) => e.column_name ?? '' },
            { key: 'code', title: t('portal.school_lms.sis.errorCode'), dataIndex: 'error_code' },
            { key: 'msg', title: t('portal.school_lms.sis.errorMessage'), render: (_, e) => e.message ?? '' },
          ]}
        />
      ) : null}
    </Space>
  );
}
