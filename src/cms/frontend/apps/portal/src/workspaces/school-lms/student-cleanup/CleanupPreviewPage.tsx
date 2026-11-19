// "Học sinh sắp bị dọn" (F052 T043, F052-AC-21, F052-AC-27): previewTenantStudentCleanup (scheduled date, last login,
// exclusion) with multi-select → Loại trừ / Gia hạn (ActionGate cleanup_exclude). No "Xóa vĩnh viễn" anywhere (R21).
import { Button, Flex, Space, Tag } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { actionStatus } from '../../../permissions/menuRegistry';
import { usePermissions } from '../../../permissions/usePermissions';
import { QueryError } from '../common/errors';
import { formatDate, formatDateTime } from '../common/format';
import { ResponsiveTable } from '../common/ResponsiveTable';
import { ExcludeDialog } from './ExcludeDialog';
import { useCleanupPreview, type CleanupCandidate } from './useCleanup';

export function CleanupPreviewPage() {
  const { t } = useTranslation();
  const { view } = usePermissions();
  const canExclude = !!view && actionStatus('action.school_lms.settings.cleanup_exclude', view) === 'enabled';
  const list = useCleanupPreview();
  const [selected, setSelected] = useState<string[]>([]);
  const [open, setOpen] = useState(false);
  const columns: ColumnType<CleanupCandidate>[] = [
    { key: 'title', title: t('portal.school_lms.students.name'), dataIndex: 'display_name' },
    { key: 'class', title: t('portal.school_lms.classes.name'), render: (_, c) => c.class_name ?? '' },
    { key: 'last', title: t('portal.school_lms.students.lastLogin'), render: (_, c) => (c.last_login_at ? formatDateTime(c.last_login_at) : t('portal.school_lms.students.never')) },
    { key: 'scheduled', title: t('portal.school_lms.cleanup.scheduledAt'), render: (_, c) => formatDate(c.scheduled_cleanup_at) },
    { key: 'excluded', title: t('portal.school_lms.cleanup.exclusion'), render: (_, c) => (c.excluded_until ? <Tag>{t('portal.school_lms.cleanup.excludedUntil', { date: formatDate(c.excluded_until) })}</Tag> : null) },
  ];
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }} data-testid="cleanup-preview">
      {canExclude ? (
        <Flex gap="small" wrap="wrap">
          <Button type="primary" disabled={!selected.length} onClick={() => setOpen(true)}>
            {t('portal.school_lms.cleanup.excludeSelected', { count: selected.length })}
          </Button>
        </Flex>
      ) : null}
      {list.error ? <QueryError error={list.error} /> : null}
      <ResponsiveTable<CleanupCandidate>
        rowKey="student_id"
        columns={columns}
        data={list.items}
        loading={list.isFetching}
        cardTitle={(c) => c.display_name}
        hasMore={list.hasMore}
        onLoadMore={list.loadMore}
        rowSelection={canExclude ? { selectedRowKeys: selected, onChange: (keys) => setSelected(keys.map(String)) } : undefined}
      />
      {open ? (
        <ExcludeDialog
          studentIds={selected}
          onClose={(done) => {
            setOpen(false);
            if (done) setSelected([]);
          }}
        />
      ) : null}
    </Space>
  );
}
