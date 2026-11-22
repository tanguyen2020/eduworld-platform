// Step 2 "Đối tượng" (F055 T018; F055-AC-02, AC-04; ADR-0095 mục 3): the school (schools of the user's scope), the
// subject (subjects of the tenant, only those of a subject-scoped role), then either "Theo khối" — one or more grades the
// school teaches, optional students of other grades of the same school to add, optional students to leave out (exclude
// wins) — or "Học sinh cụ thể" (a list of students of the school). Students are picked grade by grade from
// listTenantGradeStudents (search ≥ 2 characters on the server). Options only come from the API; limits come from
// the contract (fieldSchemas). No class, study group or course (removed by ADR-0095). Presentational only (AGENTS R1).
import { Alert, Flex, Form, Radio, Typography } from 'antd';
import { useCallback, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useMyPermissions } from '../../../../../shared/api/hooks/useMyPermissions';
import { SchoolSelect } from '../../../common/SchoolSelect';
import { subjectLabel, subjectsInScope } from '../../../dashboard/model/filters';
import { useGradeStudents, useSchoolGrades, useTenantSubjects } from '../../api/targets';
import { excludedIncludes, type AssignmentFormValues, type TargetMode } from '../../model/assignmentForm';
import { LMS_LIMITS } from '../../model/fieldSchemas';
import { LmsProblem } from '../../LmsUi';
import { SearchSelect } from '@eduworld/ops-ui';

type Patch = (p: Partial<AssignmentFormValues>) => void;
type Errors = Partial<Record<keyof AssignmentFormValues, string>>;
type StudentField = 'includeIds' | 'excludeIds' | 'studentIds';

const MODES: TargetMode[] = ['grades', 'students'];
/** Student lists of the step and their contract limit (AssignmentTarget maxItems). */
const STUDENT_MAX: Record<StudentField, number> = {
  includeIds: LMS_LIMITS.includeStudents.maxItems,
  excludeIds: LMS_LIMITS.excludeStudents.maxItems,
  studentIds: LMS_LIMITS.targetStudents.maxItems,
};
const [INCLUDE, EXCLUDE, STUDENTS] = Object.keys(STUDENT_MAX) as StudentField[];
const status = (e: string | undefined) => (e ? ('error' as const) : undefined);

interface PickerProps {
  schoolId?: string;
  /** Grades the school teaches (the picker lists one grade at a time). */
  grades: number[];
  /** Grade shown first (include: a grade not targeted; exclude: a targeted grade). */
  initialGrade?: number;
  value: string[];
  onChange: (ids: string[]) => void;
  label: string;
  max: number;
  testId: string;
  /** Display names of the students seen in a search (selected ids keep their label after the search changes). */
  labels: Map<string, string>;
}

/** Multi-select of students of one grade of the school at a time (listTenantGradeStudents), searchable on the server. */
function StudentPicker({ schoolId, grades, initialGrade, value, onChange, label, max, testId, labels }: PickerProps) {
  const { t } = useTranslation();
  const [search, setSearch] = useState('');
  const [picked, setPicked] = useState<number | undefined>();
  const grade = picked ?? initialGrade ?? grades[0];
  const found = useGradeStudents(schoolId, grade, search);
  for (const s of found.items) labels.set(s.student_id, `${s.display_name} (${s.username})`);
  const options = [...new Set([...value, ...found.items.map((s) => s.student_id)])].map((id) => ({ value: id, label: labels.get(id) ?? t('lms.wizard.studentPrefilled') }));
  return (
    <Flex vertical gap="small">
      <Flex gap="small" wrap>
        <SearchSelect
          aria-label={t('lms.wizard.pickerGrade', { label })}
          value={grade}
          disabled={!schoolId}
          onChange={(g: number) => setPicked(g)}
          options={grades.map((g) => ({ value: g, label: t('lms.wizard.gradeN', { n: g }) }))}
          popupMatchSelectWidth={false}
          data-testid={`${testId}-grade`}
        />
        <SearchSelect
          mode="multiple"
          showSearch
          filterOption={false}
          allowClear
          aria-label={label}
          placeholder={t('lms.wizard.searchStudent')}
          disabled={!schoolId}
          value={value}
          onSearch={setSearch}
          onChange={(ids: string[]) => onChange(ids)}
          loading={found.isFetching}
          maxCount={max}
          notFoundContent={t('lms.wizard.noStudent')}
          options={options}
          data-testid={testId}
          style={{ flex: 1, minWidth: 0 }}
        />
      </Flex>
      <LmsProblem error={found.error} />
    </Flex>
  );
}

export function StepTargets({ v, set, errors }: { v: AssignmentFormValues; set: Patch; errors: Errors }) {
  const { t, i18n } = useTranslation();
  const perms = useMyPermissions();
  const subjects = useTenantSubjects();
  const school = useSchoolGrades(v.schoolId);
  const labels = useRef(new Map<string, string>()).current;
  const previous = v.schoolId;
  const onSchool = useCallback(
    // another school has other grades and students: the target lists start again (the automatic first pick keeps prefilled students)
    (schoolId: string | undefined) => set(previous && schoolId !== previous ? { schoolId, grades: [], includeIds: [], excludeIds: [], studentIds: [] } : { schoolId }),
    [previous, set],
  );
  const subjectOptions = subjectsInScope(subjects.data ?? [], perms.data?.roles ?? []).map((s) => ({ value: s.subject_id, label: subjectLabel(s, i18n.language) }));
  const conflicts = excludedIncludes(v);
  // include lists another grade than the targeted ones first; exclude starts on a targeted grade
  const initialGrade = (field: StudentField) => (field === INCLUDE ? school.grades.find((g) => !v.grades.includes(g)) : field === EXCLUDE ? [...v.grades].sort((a, b) => a - b)[0] : undefined);
  const picker = (field: StudentField) => (
    <Form.Item label={t(`lms.wizard.${field}`)} required={field === 'studentIds'} validateStatus={status(errors[field])} help={errors[field]}>
      <StudentPicker schoolId={v.schoolId} grades={school.grades} initialGrade={initialGrade(field)} value={v[field]} onChange={(ids) => set({ [field]: ids })} label={t(`lms.wizard.${field}`)} max={STUDENT_MAX[field]} testId={`pick-${field}`} labels={labels} />
    </Form.Item>
  );
  return (
    <>
      <LmsProblem error={subjects.error ?? school.error} />
      <Form.Item label={t('portal.school_lms.common.school')} required validateStatus={status(errors.schoolId)} help={errors.schoolId}>
        <SchoolSelect value={v.schoolId} onChange={onSchool} />
      </Form.Item>
      <Form.Item label={t('lms.wizard.subject')} required validateStatus={status(errors.subjectId)} help={errors.subjectId}>
        <SearchSelect
          showSearch
          optionFilterProp="label"
          aria-label={t('lms.wizard.subject')}
          placeholder={t('lms.wizard.subject')}
          loading={subjects.isFetching}
          value={v.subjectId}
          onChange={(subjectId: string) => set({ subjectId })}
          options={subjectOptions}
        />
      </Form.Item>
      <Form.Item label={t('lms.wizard.targetType')}>
        <Radio.Group
          aria-label={t('lms.wizard.targetType')}
          value={v.targetMode}
          onChange={(e) => set({ targetMode: e.target.value as TargetMode })}
          options={MODES.map((k) => ({ value: k, label: t(`lms.targetMode.${k}`) }))}
        />
      </Form.Item>
      {v.targetMode === 'grades' ? (
        <>
          <Form.Item label={t('lms.wizard.grades')} required validateStatus={status(errors.grades)} help={errors.grades}>
            <SearchSelect
              mode="multiple"
              aria-label={t('lms.wizard.grades')}
              placeholder={t('lms.wizard.grades')}
              disabled={!v.schoolId}
              loading={school.isFetching}
              value={v.grades}
              maxCount={LMS_LIMITS.targets.maxItems}
              onChange={(grades: number[]) => set({ grades })}
              options={school.grades.map((g) => ({ value: g, label: t('lms.wizard.gradeN', { n: g }) }))}
              data-testid="grade-select"
            />
          </Form.Item>
          {picker(INCLUDE!)}
          {picker(EXCLUDE!)}
          {conflicts.length ? (
            <Alert type="warning" showIcon data-testid="exclude-wins" message={t('lms.wizard.excludeWins', { n: conflicts.length })} />
          ) : (
            <Typography.Text type="secondary">{t('lms.wizard.newStudentsJoin')}</Typography.Text>
          )}
        </>
      ) : (
        picker(STUDENTS!)
      )}
    </>
  );
}
