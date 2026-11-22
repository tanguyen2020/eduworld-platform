// Filters of the assignment lists (F055 T019, F056 T032; ADR-0095): school, subject and grade — the listAssignments
// filters `school_id`, `subject_id`, `grade_level` (no course or class any more). Grades are those the chosen school
// teaches (1–9 when no school is chosen); subjects are those of the session's scope. Display only: the server filters.
import { Form } from 'antd';
import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { useMyPermissions } from '../../../../shared/api/hooks/useMyPermissions';
import { SchoolSelect } from '../../common/SchoolSelect';
import { subjectLabel, subjectsInScope } from '../../dashboard/model/filters';
import type { AssignmentFilters } from '../api/assignments';
import { useSchoolGrades, useTenantSubjects } from '../api/targets';
import { LMS_LIMITS } from '../model/fieldSchemas';
import { SearchSelect } from '@eduworld/ops-ui';

const ALL_GRADES = Array.from({ length: LMS_LIMITS.gradeLevel.maximum - LMS_LIMITS.gradeLevel.minimum + 1 }, (_, i) => LMS_LIMITS.gradeLevel.minimum + i);

export function AssignmentFilterBar({ value, onChange }: { value: AssignmentFilters; onChange: (f: AssignmentFilters) => void }) {
  const { t, i18n } = useTranslation();
  const perms = useMyPermissions();
  const subjects = useTenantSubjects();
  const school = useSchoolGrades(value.schoolId);
  const grades = value.schoolId ? school.grades : ALL_GRADES;
  const onSchool = useCallback((schoolId: string | undefined) => onChange({ ...value, schoolId, gradeLevel: undefined }), [value, onChange]);
  return (
    <>
      <Form.Item label={t('portal.school_lms.common.school')} style={{ marginBottom: 0 }}>
        <SchoolSelect value={value.schoolId} onChange={onSchool} allowClear autoSelect={false} />
      </Form.Item>
      <Form.Item label={t('lms.wizard.subject')} style={{ marginBottom: 0 }}>
        <SearchSelect
          allowClear
          aria-label={t('lms.wizard.subject')}
          placeholder={t('lms.filters.allSubjects')}
          value={value.subjectId}
          onChange={(subjectId?: string) => onChange({ ...value, subjectId })}
          loading={subjects.isFetching}
          options={subjectsInScope(subjects.data ?? [], perms.data?.roles ?? []).map((s) => ({ value: s.subject_id, label: subjectLabel(s, i18n.language) }))}
          popupMatchSelectWidth={false}
        />
      </Form.Item>
      <Form.Item label={t('lms.wizard.grade')} style={{ marginBottom: 0 }}>
        <SearchSelect
          allowClear
          aria-label={t('lms.wizard.grade')}
          placeholder={t('lms.filters.allGrades')}
          value={value.gradeLevel}
          onChange={(gradeLevel?: number) => onChange({ ...value, gradeLevel })}
          options={grades.map((g) => ({ value: g, label: t('lms.wizard.gradeN', { n: g }) }))}
          popupMatchSelectWidth={false}
        />
      </Form.Item>
    </>
  );
}
