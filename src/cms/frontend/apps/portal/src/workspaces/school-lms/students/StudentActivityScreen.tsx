// Screen of `menu.school_lms.student_activity` (F052 T019, F052-AC-06): pick school → class → student (read-only class
// lists), or open `:studentId` directly (link from the class page) to see the game login history.
import { Flex, Form, Select, Space } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, Route, Routes, useParams } from 'react-router-dom';
import { useClassStudents, useSchoolClasses } from '../classes/useClasses';
import { SchoolSelect } from '../common/SchoolSelect';
import { SingleTenantGate } from '../common/scope';
import { PageTitle } from '../common/ui';
import { StudentLoginActivityPanel } from './StudentLoginActivityPanel';

function StudentPicker() {
  const { t } = useTranslation();
  const [schoolId, setSchoolId] = useState<string | undefined>();
  const [classId, setClassId] = useState<string | undefined>();
  const classes = useSchoolClasses(schoolId);
  const students = useClassStudents(schoolId, classId);
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Flex wrap="wrap" gap="middle">
        <Form.Item label={t('portal.school_lms.common.school')}>
          <SchoolSelect value={schoolId} onChange={(v) => (setSchoolId(v), setClassId(undefined))} />
        </Form.Item>
        <Form.Item label={t('portal.school_lms.classes.name')}>
          <Select
            aria-label={t('portal.school_lms.classes.name')}
            value={classId}
            onChange={setClassId}
            loading={classes.isFetching}
            options={classes.items.map((c) => ({ value: c.class_id, label: c.name }))}
            popupMatchSelectWidth={false}
          />
        </Form.Item>
      </Flex>
      <ul aria-label={t('portal.school_lms.classes.students')}>
        {students.items.map((s) => (
          <li key={s.student_id}>
            <Link to={s.student_id}>{s.display_name}</Link>
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
