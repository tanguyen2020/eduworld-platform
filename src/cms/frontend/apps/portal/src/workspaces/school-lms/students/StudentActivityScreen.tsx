// Screen of `menu.school_lms.student_activity` (F052 T019, F052-AC-06): pick school → grade → student (listTenantGradeStudents;
// ADR-0095: no class), or open `:studentId` directly (link from "Học sinh theo khối" or the dashboard) to see the game
// login history.
import { Flex, Form, Input, Space } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, Route, Routes, useParams } from 'react-router-dom';
import { QueryError } from '../common/errors';
import { FIELDS } from '../common/fieldSchemas';
import { SingleTenantGate } from '../common/scope';
import { PageTitle } from '../common/ui';
import { useGradeStudents } from '../lms/api/targets';
import { StudentLoginActivityPanel } from './StudentLoginActivityPanel';
import { SchoolGradeFields, useSchoolGradePicker } from './StudentsScreen';

function StudentPicker() {
  const { t } = useTranslation();
  const picker = useSchoolGradePicker();
  const [search, setSearch] = useState('');
  const students = useGradeStudents(picker.schoolId, picker.grade, search);
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Flex wrap="wrap" gap="small" align="end">
        <SchoolGradeFields p={picker} />
        <Form.Item label={t('portal.school_lms.common.search')} style={{ marginBottom: 0 }}>
          <Input.Search allowClear aria-label={t('portal.school_lms.common.search')} placeholder={t('portal.school_lms.students.searchHint')} maxLength={FIELDS.gradeStudentSearch.maxLength} onSearch={setSearch} />
        </Form.Item>
      </Flex>
      {students.error ? <QueryError error={students.error} resource /> : null}
      <ul aria-label={t('portal.school_lms.students.found')}>
        {students.items.map((x) => (
          <li key={x.student_id}>
            <Link to={x.student_id}>{x.display_name}</Link>
          </li>
        ))}
      </ul>
    </Space>
  );
}

function StudentActivity() {
  const { t } = useTranslation();
  const { studentId } = useParams();
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Link to="..">{t('portal.school_lms.common.back')}</Link>
      <StudentLoginActivityPanel studentId={studentId!} />
    </Space>
  );
}

export function StudentActivityScreen({ title }: { title: string }) {
  return (
    <section aria-labelledby="ew-menu-title">
      <PageTitle title={title} />
      <SingleTenantGate>
        {() => (
          <Routes>
            <Route index element={<StudentPicker />} />
            <Route path=":studentId" element={<StudentActivity />} />
          </Routes>
        )}
      </SingleTenantGate>
    </section>
  );
}
