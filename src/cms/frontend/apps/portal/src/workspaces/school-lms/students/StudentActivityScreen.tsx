// Screen of `menu.school_lms.student_activity` (F052 T019, F052-AC-06): pick school → search a student (ADR-0095: no
// class; interim student search of lms/api/targets.ts until the school → grade → students list of WP-I1), or open
// `:studentId` directly (link from "Học sinh theo khối" or the dashboard) to see the game login history.
import { Alert, Flex, Form, Input, Space } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, Route, Routes, useParams } from 'react-router-dom';
import { ApiError } from '../../../shared/api/middleware/problem';
import { FIELDS } from '../common/fieldSchemas';
import { SchoolSelect } from '../common/SchoolSelect';
import { SingleTenantGate } from '../common/scope';
import { PageTitle } from '../common/ui';
import { useSchoolStudentSearch } from '../lms/api/targets';
import { StudentLoginActivityPanel } from './StudentLoginActivityPanel';

function StudentPicker() {
  const { t } = useTranslation();
  const [schoolId, setSchoolId] = useState<string | undefined>();
  const [search, setSearch] = useState('');
  const students = useSchoolStudentSearch(schoolId, search);
  const forbidden = students.error instanceof ApiError && students.error.status === 403;
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Flex wrap="wrap" gap="middle">
        <Form.Item label={t('portal.school_lms.common.school')}>
          <SchoolSelect value={schoolId} onChange={setSchoolId} />
        </Form.Item>
        <Form.Item label={t('portal.school_lms.common.search')}>
          <Input.Search allowClear aria-label={t('portal.school_lms.common.search')} placeholder={t('portal.school_lms.students.searchHint')} maxLength={FIELDS.userSearch.maxLength} onSearch={setSearch} />
        </Form.Item>
      </Flex>
      {forbidden ? <Alert type="info" showIcon message={t('portal.school_lms.students.searchUnavailable')} /> : null}
      <ul aria-label={t('portal.school_lms.students.found')}>
        {students.items.map((s) => (
          <li key={s.user_id}>
            <Link to={s.user_id}>{s.display_name ?? s.username}</Link>
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
