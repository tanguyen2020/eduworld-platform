// "Học sinh chưa có lớp" (F052 T018, F052-AC-05, TC-SIS-103): listUnassignedStudents (`missing_class`, …) with the
// guidance to add the class in SIS/CSV. Deliberately no "assign class" action on the portal (ADR-0060).
import { Alert, Form, Space } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useUnassignedStudents, type UnassignedStudent } from '../classes/useClasses';
import { QueryError } from '../common/errors';
import { formatDateTime } from '../common/format';
import { ResponsiveTable } from '../common/ResponsiveTable';
import { SchoolSelect } from '../common/SchoolSelect';

export function UnassignedStudentsPage({ schoolId: fixedSchool }: { schoolId?: string }) {
  const { t } = useTranslation();
  const [picked, setPicked] = useState<string | undefined>();
  const schoolId = fixedSchool ?? picked;
  const list = useUnassignedStudents(schoolId);
  const columns: ColumnType<UnassignedStudent>[] = [
    { key: 'title', title: t('portal.school_lms.students.name'), dataIndex: 'display_name' },
    { key: 'username', title: t('portal.school_lms.students.username'), render: (_, s) => s.username ?? '' },
    { key: 'grade', title: t('portal.school_lms.classes.grade'), render: (_, s) => s.grade_level ?? '' },
    { key: 'reason', title: t('portal.school_lms.unassigned.reason'), render: (_, s) => t(`portal.school_lms.unassigned.reasons.${s.reason}`) },
    { key: 'detected', title: t('portal.school_lms.unassigned.detectedAt'), render: (_, s) => formatDateTime(s.detected_at) },
  ];
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }} data-testid="unassigned-students">
      <Alert type="info" showIcon message={t('portal.school_lms.unassigned.title')} description={t('portal.school_lms.unassigned.guidance')} />
      {fixedSchool ? null : (
        <Form.Item label={t('portal.school_lms.common.school')}>
          <SchoolSelect value={picked} onChange={setPicked} allowClear autoSelect={false} />
        </Form.Item>
      )}
      {list.error ? <QueryError error={list.error} /> : null}
      <ResponsiveTable<UnassignedStudent>
        rowKey="student_id"
        columns={columns}
        data={list.items}
        loading={list.isFetching}
        cardTitle={(s) => s.display_name}
        hasMore={list.hasMore}
        onLoadMore={list.loadMore}
      />
    </Space>
  );
}
