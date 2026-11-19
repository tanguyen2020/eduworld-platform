// "Xung đột đồng bộ" (F052 T026, F052-AC-10): listSchoolReconciliationIssues by type and status with the suggested
// action; "Xử lý" (ActionGate sis_sync.import) on open issues. `entity_refs` are ids only (no raw PII, SIS_INTEGRATION).
import { Button, Flex, Form, Select, Space, Typography } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ActionGate } from '../../../permissions/ActionGate';
import { QueryError } from '../common/errors';
import { formatDateTime } from '../common/format';
import { ResponsiveTable } from '../common/ResponsiveTable';
import { StatusTag } from '../common/ui';
import { ResolveIssueDialog } from './ResolveIssueDialog';
import { useReconciliationIssues, type IssueStatus, type IssueType, type ReconciliationIssue } from './useReconciliation';

const TYPES: IssueType[] = ['duplicate_identifier', 'missing_class', 'school_province_missing', 'school_duplicate', 'possible_duplicate', 'transfer_confirmation', 'class_school_mismatch', 'orphan_enrollment', 'multi_school_enrollment', 'grade_mismatch', 'deactivation_review', 'cleaned_up_student_in_sis'];
const STATUSES: IssueStatus[] = ['open', 'resolved', 'ignored'];

/** i18n key of the suggested action: the server code when translated, else the default advice of the issue type. */
export function suggestionKey(issue: Pick<ReconciliationIssue, 'issue_type' | 'suggested_action'>, exists: (k: string) => boolean): string {
  const own = issue.suggested_action ? `portal.school_lms.recon.suggestions.${issue.suggested_action}` : '';
  return own && exists(own) ? own : `portal.school_lms.recon.typeAdvice.${issue.issue_type}`;
}

export function ReconciliationIssuesPage({ schoolId }: { schoolId: string }) {
  const { t, i18n } = useTranslation();
  const [issueType, setIssueType] = useState<IssueType | undefined>();
  const [status, setStatus] = useState<IssueStatus | undefined>('open');
  const [resolving, setResolving] = useState<ReconciliationIssue | null>(null);
  const issues = useReconciliationIssues(schoolId, { issueType, status });
  const exists = (k: string) => i18n.exists(k);
  const columns: ColumnType<ReconciliationIssue>[] = [
    { key: 'title', title: t('portal.school_lms.recon.type'), render: (_, x) => t(`portal.school_lms.recon.types.${x.issue_type}`) },
    { key: 'refs', title: t('portal.school_lms.recon.refs'), render: (_, x) => Object.entries(x.entity_refs ?? {}).map(([k, v]) => `${k}: ${v}`).join(', ') },
    { key: 'suggested', title: t('portal.school_lms.recon.suggested'), render: (_, x) => t(suggestionKey(x, exists)) },
    { key: 'status', title: t('portal.school_lms.recon.status'), render: (_, x) => <StatusTag group="issue" value={x.status} /> },
    { key: 'created', title: t('portal.school_lms.recon.createdAt'), render: (_, x) => formatDateTime(x.created_at) },
    {
      key: 'actions',
      title: t('portal.school_lms.common.actions'),
      render: (_, x) =>
        x.status === 'open' ? (
          <ActionGate actionKey="action.school_lms.sis_sync.import">
            <Button size="small" onClick={() => setResolving(x)}>
              {t('portal.school_lms.recon.resolve')}
            </Button>
          </ActionGate>
        ) : (
          <Typography.Text type="secondary">{x.resolved_at ? formatDateTime(x.resolved_at) : ''}</Typography.Text>
        ),
    },
  ];
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Flex wrap="wrap" gap="middle">
        <Form.Item label={t('portal.school_lms.recon.type')}>
          <Select allowClear aria-label={t('portal.school_lms.recon.type')} value={issueType} onChange={setIssueType} popupMatchSelectWidth={false} options={TYPES.map((x) => ({ value: x, label: t(`portal.school_lms.recon.types.${x}`) }))} />
        </Form.Item>
        <Form.Item label={t('portal.school_lms.recon.status')}>
          <Select allowClear aria-label={t('portal.school_lms.recon.status')} value={status} onChange={setStatus} options={STATUSES.map((x) => ({ value: x, label: t(`portal.school_lms.status.issue.${x}`) }))} />
        </Form.Item>
      </Flex>
      {issues.error ? <QueryError error={issues.error} resource /> : null}
      <ResponsiveTable<ReconciliationIssue>
        rowKey="issue_id"
        columns={columns}
        data={issues.items}
        loading={issues.isFetching}
        cardTitle={(x) => t(`portal.school_lms.recon.types.${x.issue_type}`)}
        hasMore={issues.hasMore}
        onLoadMore={issues.loadMore}
      />
      {resolving ? <ResolveIssueDialog schoolId={schoolId} issue={resolving} onClose={() => setResolving(null)} /> : null}
    </Space>
  );
}
