// Screen of `menu.school_lms.students` — "Học sinh theo khối" (F052 T017, T059; F052-AC-04, F052-AC-06, F052-AC-07,
// F052-AC-31; ADR-0095): the class list, a class's students and the "học sinh chưa có lớp" tab are gone with the class
// concept. Students belong to one school and one grade; the school → grade → students list comes with the
// `listTenantGradeStudents` operation of WP-I1 (the catalog key is renamed `menu.school_lms.students` there). Until then
// the screen searches the students of one school (interim listTenantUsers, user_type=student — lms/api/targets.ts),
// read-only (SIS/CSV owns them, ADR-0060), with the same two actions as before: login history (ActionGate
// student_activity.view) and password reset (ActionGate classes.reset_student_password). No class anywhere.
import { Alert, Button, Form, Input, Space, Typography } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import { ActionGate } from '../../../permissions/ActionGate';
import { menuByKey } from '../../../permissions/menuRegistry';
import { ApiError } from '../../../shared/api/middleware/problem';
import { QueryError } from '../common/errors';
import { FIELDS } from '../common/fieldSchemas';
import { formatDateTime } from '../common/format';
import { ResponsiveTable } from '../common/ResponsiveTable';
import { SchoolSelect } from '../common/SchoolSelect';
import { SingleTenantGate } from '../common/scope';
import { PageTitle, SourceTag, StatusTag } from '../common/ui';
import { useSchoolStudentSearch, type StudentUser } from '../lms/api/targets';
import { ResetStudentPasswordDialog } from './ResetStudentPasswordDialog';

/** Path of a student's login history in the `menu.school_lms.student_activity` screen (path from the catalog). */
export function studentActivityPath(studentId: string): string {
  return `${menuByKey('menu.school_lms.student_activity')?.path ?? ''}/${studentId}`;
}

function StudentSearchPage() {
  const { t } = useTranslation();
  const [schoolId, setSchoolId] = useState<string | undefined>();
  const [search, setSearch] = useState('');
  const students = useSchoolStudentSearch(schoolId, search);
  const [resetting, setResetting] = useState<StudentUser | null>(null);
  const columns: ColumnType<StudentUser>[] = [
    { key: 'title', title: t('portal.school_lms.students.name'), render: (_, s) => s.display_name ?? s.username },
    { key: 'username', title: t('portal.school_lms.students.username'), dataIndex: 'username' },
    { key: 'status', title: t('portal.school_lms.students.status'), render: (_, s) => <StatusTag group="user" value={s.status} /> },
    { key: 'source', title: t('portal.school_lms.common.source'), render: (_, s) => (s.source ? <SourceTag source={s.source} /> : null) },
    { key: 'last', title: t('portal.school_lms.students.lastLogin'), render: (_, s) => (s.last_login_at ? formatDateTime(s.last_login_at) : t('portal.school_lms.students.never')) },
    {
      key: 'actions',
      title: t('portal.school_lms.common.actions'),
      render: (_, s) => (
        <Space wrap>
          <ActionGate actionKey="action.school_lms.student_activity.view">
            <Link to={studentActivityPath(s.user_id)}>{t('portal.school_lms.students.activity')}</Link>
          </ActionGate>
          <ActionGate actionKey="action.school_lms.students.reset_password">
            <Button size="small" onClick={() => setResetting(s)}>
              {t('portal.school_lms.passwordReset.action')}
            </Button>
          </ActionGate>
        </Space>
      ),
    },
  ];
  const forbidden = students.error instanceof ApiError && students.error.status === 403;
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }} data-testid="students-by-grade">
      <Typography.Text type="secondary">{t('portal.school_lms.students.readOnlyHint')}</Typography.Text>
      <Form layout="vertical" component="div">
        <Form.Item label={t('portal.school_lms.common.school')}>
          <SchoolSelect value={schoolId} onChange={setSchoolId} />
        </Form.Item>
        <Form.Item label={t('portal.school_lms.common.search')}>
          <Input.Search
            allowClear
            aria-label={t('portal.school_lms.common.search')}
            placeholder={t('portal.school_lms.students.searchHint')}
            maxLength={FIELDS.userSearch.maxLength}
            onSearch={setSearch}
            style={{ maxWidth: '100%' }}
          />
        </Form.Item>
      </Form>
      {search.trim().length < FIELDS.userSearch.minLength ? <Alert type="info" showIcon message={t('portal.school_lms.students.searchFirst')} /> : null}
      {forbidden ? <Alert type="info" showIcon data-testid="students-unavailable" message={t('portal.school_lms.students.searchUnavailable')} /> : students.error ? <QueryError error={students.error} /> : null}
      <ResponsiveTable<StudentUser>
        rowKey="user_id"
        columns={columns}
        data={students.items}
        loading={students.isFetching}
        cardTitle={(s) => s.display_name ?? s.username}
        hasMore={students.hasMore}
        onLoadMore={students.loadMore}
      />
      {resetting ? (
        <ResetStudentPasswordDialog student={{ student_id: resetting.user_id, display_name: resetting.display_name ?? resetting.username, username: resetting.username }} open onClose={() => setResetting(null)} />
      ) : null}
    </Space>
  );
}

export function StudentsScreen({ title }: { title: string }) {
  return (
    <section aria-labelledby="ew-menu-title">
      <PageTitle title={title} />
      <SingleTenantGate>{() => <StudentSearchPage />}</SingleTenantGate>
    </section>
  );
}
