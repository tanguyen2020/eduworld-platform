// Students of a class, read-only (F052 T017, T059; F052-AC-04, F052-AC-06, F052-AC-07, F052-AC-31): listTenantClassStudents
// with the "SIS" tag, last game login and days without the game; no action editing identity or assigning a class
// (ADR-0060). Only actions: login history (ActionGate student_activity.view) and password reset (ActionGate
// classes.reset_student_password). Class outside the scope (404/403) → "Không tìm thấy".
import { Button, Input, Space, Typography } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useParams } from 'react-router-dom';
import { ActionGate } from '../../../permissions/ActionGate';
import { menuByKey } from '../../../permissions/menuRegistry';
import { QueryError } from '../common/errors';
import { FIELDS } from '../common/fieldSchemas';
import { formatDateTime } from '../common/format';
import { ResponsiveTable } from '../common/ResponsiveTable';
import { SourceTag, StatusTag } from '../common/ui';
import { ResetStudentPasswordDialog } from './ResetStudentPasswordDialog';
import { useClassName, useClassStudents, type ClassStudent } from './useClasses';

/** Path of a student's login history in the `menu.school_lms.student_activity` screen (path from the catalog). */
export function studentActivityPath(studentId: string): string {
  return `${menuByKey('menu.school_lms.student_activity')?.path ?? ''}/${studentId}`;
}

export function ClassStudentsPage() {
  const { t } = useTranslation();
  const { schoolId, classId } = useParams();
  const [search, setSearch] = useState('');
  const students = useClassStudents(schoolId, classId, search);
  const className = useClassName(schoolId, classId);
  const [resetting, setResetting] = useState<ClassStudent | null>(null);
  if (students.error) return <QueryError error={students.error} resource />;
  const columns: ColumnType<ClassStudent>[] = [
    { key: 'title', title: t('portal.school_lms.students.name'), render: (_, s) => s.display_name },
    { key: 'username', title: t('portal.school_lms.students.username'), dataIndex: 'username' },
    { key: 'code', title: t('portal.school_lms.students.code'), render: (_, s) => s.student_code ?? '' },
    { key: 'status', title: t('portal.school_lms.students.status'), render: (_, s) => <StatusTag group="user" value={s.status} /> },
    { key: 'source', title: t('portal.school_lms.common.source'), render: (_, s) => <SourceTag source={s.source} /> },
    { key: 'last', title: t('portal.school_lms.students.lastLogin'), render: (_, s) => (s.last_login_at ? formatDateTime(s.last_login_at) : t('portal.school_lms.students.never')) },
    {
      key: 'days',
      title: t('portal.school_lms.students.daysAway'),
      render: (_, s) => (s.days_since_last_login == null ? '' : t('portal.school_lms.students.days', { count: s.days_since_last_login })),
    },
    {
      key: 'actions',
      title: t('portal.school_lms.common.actions'),
      render: (_, s) => (
        <Space wrap>
          <ActionGate actionKey="action.school_lms.student_activity.view">
            <Link to={studentActivityPath(s.student_id)}>{t('portal.school_lms.students.activity')}</Link>
          </ActionGate>
          <ActionGate actionKey="action.school_lms.classes.reset_student_password">
            <Button size="small" onClick={() => setResetting(s)}>
              {t('portal.school_lms.passwordReset.action')}
            </Button>
          </ActionGate>
        </Space>
      ),
    },
  ];
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Link to="..">{t('portal.school_lms.common.back')}</Link>
      <Typography.Title level={2}>{className || t('portal.school_lms.classes.students')}</Typography.Title>
      <Typography.Text type="secondary">{t('portal.school_lms.classes.readOnlyHint')}</Typography.Text>
      <Input.Search
        allowClear
        aria-label={t('portal.school_lms.common.search')}
        placeholder={t('portal.school_lms.students.searchHint')}
        maxLength={FIELDS.classStudentSearch.maxLength}
        onSearch={setSearch}
        style={{ maxWidth: '100%' }}
      />
      <ResponsiveTable<ClassStudent>
        rowKey="student_id"
        columns={columns}
        data={students.items}
        loading={students.isFetching}
        cardTitle={(s) => s.display_name}
        hasMore={students.hasMore}
        onLoadMore={students.loadMore}
      />
      {resetting ? <ResetStudentPasswordDialog student={resetting} open onClose={() => setResetting(null)} /> : null}
    </Space>
  );
}
