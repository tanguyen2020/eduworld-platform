// Classes of a school (F052 T017, F052-AC-04): listTenantSchoolClasses by academic year / grade with head count.
// Read-only (classes come from SIS/CSV, ADR-0060). Row → the class's students.
import { Flex, Form, Input, Select, Space } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useSearchParams } from 'react-router-dom';
import { QueryError } from '../common/errors';
import { ResponsiveTable } from '../common/ResponsiveTable';
import { SchoolSelect } from '../common/SchoolSelect';
import { useSchoolClasses, type SchoolClass } from './useClasses';

const ACADEMIC_YEAR = /^[0-9]{4}-[0-9]{4}$/;
const GRADES = [1, 2, 3, 4, 5, 6, 7, 8, 9];

export function ClassListPage() {
  const { t } = useTranslation();
  const [params, setParams] = useSearchParams();
  const schoolId = params.get('school') ?? undefined;
  const setSchool = (id: string | undefined) => setParams(id ? { school: id } : {}, { replace: true });
  const [grade, setGrade] = useState<number | undefined>();
  const [year, setYear] = useState('');
  const classes = useSchoolClasses(schoolId, { grade, academicYear: ACADEMIC_YEAR.test(year) ? year : undefined });
  const columns: ColumnType<SchoolClass>[] = [
    { key: 'title', title: t('portal.school_lms.classes.name'), render: (_, c) => <Link to={`${c.school_id}/${c.class_id}`}>{c.name}</Link> },
    { key: 'grade', title: t('portal.school_lms.classes.grade'), dataIndex: 'grade_level' },
    { key: 'count', title: t('portal.school_lms.classes.studentCount'), render: (_, c) => c.student_count ?? '' },
  ];
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Flex wrap="wrap" gap="middle">
        <Form.Item label={t('portal.school_lms.common.school')}>
          <SchoolSelect value={schoolId} onChange={setSchool} />
        </Form.Item>
        <Form.Item label={t('portal.school_lms.classes.academicYear')} validateStatus={year && !ACADEMIC_YEAR.test(year) ? 'error' : undefined}>
          <Input value={year} onChange={(e) => setYear(e.target.value)} maxLength={9} placeholder={t('portal.school_lms.classes.academicYearHint')} />
        </Form.Item>
        <Form.Item label={t('portal.school_lms.classes.grade')}>
          <Select allowClear value={grade} onChange={setGrade} options={GRADES.map((g) => ({ value: g, label: String(g) }))} aria-label={t('portal.school_lms.classes.grade')} />
        </Form.Item>
      </Flex>
      {classes.error ? <QueryError error={classes.error} resource /> : null}
      <ResponsiveTable<SchoolClass>
        rowKey="class_id"
        columns={columns}
        data={classes.items}
        loading={classes.isFetching}
        cardTitle={(c) => <Link to={`${c.school_id}/${c.class_id}`}>{c.name}</Link>}
        hasMore={classes.hasMore}
        onLoadMore={classes.loadMore}
      />
    </Space>
  );
}
