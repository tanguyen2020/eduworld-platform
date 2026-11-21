// Screen of `menu.school_lms.students` — "Học sinh theo khối" (F052 T017, T059; F052-AC-04, F052-AC-06, F052-AC-07,
// F052-AC-27, F052-AC-31; ADR-0095): school → grade (the grades the school teaches) → students of that grade
// (listTenantGradeStudents, cursor, optional search), read-only with the "SIS" tag (SIS/CSV owns them, ADR-0060), last
// game login and days without the game. Actions: login history (ActionGate student_activity.view) and password reset
// (ActionGate students.reset_password). No class, no "học sinh chưa có lớp" (gone with the class concept).
import { Button, Flex, Form, Input, Select, Space, Typography } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useCallback, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router-dom';
import { ActionGate } from '../../../permissions/ActionGate';
import { menuByKey } from '../../../permissions/menuRegistry';
import { QueryError } from '../common/errors';
import { FIELDS } from '../common/fieldSchemas';
import { formatDateTime } from '../common/format';
import { ResponsiveTable } from '../common/ResponsiveTable';
import { SchoolSelect } from '../common/SchoolSelect';
import { SingleTenantGate } from '../common/scope';
import { PageTitle, SourceTag, StatusTag } from '../common/ui';
import { useGradeStudents, useSchoolGrades, type GradeStudent } from '../lms/api/targets';
import { ResetStudentPasswordDialog } from './ResetStudentPasswordDialog';

/** Path of a student's login history in the `menu.school_lms.student_activity` screen (path from the catalog). */
export function studentActivityPath(studentId: string): string {
  return `${menuByKey('menu.school_lms.student_activity')?.path ?? ''}/${studentId}`;
}

/** School + grade pickers shared by "Học sinh theo khối" and "Hoạt động học sinh" (grade = first one taught by default). */
export function useSchoolGradePicker() {
  const [schoolId, setSchoolIdState] = useState<string | undefined>();
  const [picked, setPicked] = useState<number | undefined>();
  const school = useSchoolGrades(schoolId);
  const grade = picked !== undefined && school.grades.includes(picked) ? picked : school.grades[0];
  const setSchoolId = useCallback((id: string | undefined) => {
    setSchoolIdState(id);
    setPicked(undefined);
  }, []);
  return { schoolId, setSchoolId, grade, setGrade: setPicked, grades: school.grades };
}

export function SchoolGradeFields({ p }: { p: ReturnType<typeof useSchoolGradePicker> }) {
  const { t } = useTranslation();
  return (
    <>
      <Form.Item label={t('portal.school_lms.common.school')} style={{ marginBottom: 0 }}>
        <SchoolSelect value={p.schoolId} onChange={p.setSchoolId} />
      </Form.Item>
      <Form.Item label={t('portal.school_lms.students.grade')} style={{ marginBottom: 0 }}>
        <Select
          aria-label={t('portal.school_lms.students.grade')}
          value={p.grade}
          onChange={(g: number) => p.setGrade(g)}
          options={p.grades.map((g) => ({ value: g, label: t('portal.school_lms.students.gradeN', { n: g }) }))}
          popupMatchSelectWidth={false}
        />
      </Form.Item>
    </>
  );
}

function StudentsByGradePage() {
  const { t } = useTranslation();
  const picker = useSchoolGradePicker();
  const [search, setSearch] = useState('');
  const students = useGradeStudents(picker.schoolId, picker.grade, search);
  const [resetting, setResetting] = useState<GradeStudent | null>(null);
  const columns: ColumnType<GradeStudent>[] = [
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
          <ActionGate actionKey="action.school_lms.students.reset_password">
            <Button size="small" onClick={() => setResetting(s)}>
              {t('portal.school_lms.passwordReset.action')}
            </Button>
          </ActionGate>
        </Space>
      ),
    },
  ];
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }} data-testid="students-by-grade">
      <Typography.Text type="secondary">{t('portal.school_lms.students.readOnlyHint')}</Typography.Text>
      <Flex wrap gap="small" align="end">
        <SchoolGradeFields p={picker} />
        <Form.Item label={t('portal.school_lms.common.search')} style={{ marginBottom: 0 }}>
          <Input.Search
            allowClear
            aria-label={t('portal.school_lms.common.search')}
            placeholder={t('portal.school_lms.students.searchHint')}
            maxLength={FIELDS.gradeStudentSearch.maxLength}
            onSearch={setSearch}
          />
        </Form.Item>
      </Flex>
      {students.error ? <QueryError error={students.error} resource /> : null}
      <ResponsiveTable<GradeStudent>
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

export function StudentsScreen({ title }: { title: string }) {
  return (
    <section aria-labelledby="ew-menu-title">
      <PageTitle title={title} />
      <SingleTenantGate>{() => <StudentsByGradePage />}</SingleTenantGate>
    </section>
  );
}
