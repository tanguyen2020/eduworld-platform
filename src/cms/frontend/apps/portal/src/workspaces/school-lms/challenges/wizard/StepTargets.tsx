// Step 3 "Đối tượng" (F108 T028; F108-AC-14; ADR-0095 mục 3): exactly one school of the user's scope
// (listTenantSchools), one grade the school teaches within the template's `grade_levels`, optional students of the same
// school to add and to leave out (≤ 500 each — contract limits; picked grade by grade from listTenantGradeStudents of
// that school, server search ≥ 2 characters), a hint when a student is in both lists (exclude wins on the server) and
// the list sizes — the effective number of students is computed by the server on create (no preview operation in the
// contract). No class picker (the class concept is gone). Presentational only (AGENTS R1).
import { Alert, Flex, Form, Typography } from 'antd';
import { useCallback, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { SchoolSelect } from '../../common/SchoolSelect';
import { useGradeStudents, useSchoolGrades } from '../../lms/api/targets';
import { ChallengeProblem } from '../ChallengeUi';
import { CHALLENGE_LIMITS, excludedIncludes, gradeOptions, type ChallengeFormValues, type ChallengeTemplate } from '../model/challengeForm';
import type { FieldErrors } from './StepParams';
import { SearchSelect } from '@eduworld/ops-ui';

type Patch = (p: Partial<ChallengeFormValues>) => void;
const status = (e: string | undefined) => (e ? ('error' as const) : undefined);

interface PickerProps {
  schoolId?: string;
  /** Grades the school teaches (the picker lists one grade at a time). */
  grades: number[];
  initialGrade?: number;
  value: string[];
  onChange: (ids: string[]) => void;
  label: string;
  max: number;
  testId: string;
  /** Display names of the students seen in a search (selected ids keep their label after the search changes). */
  labels: Map<string, string>;
}

/** Multi-select of students of one grade of the chosen school at a time, searchable on the server. */
function StudentPicker({ schoolId, grades, initialGrade, value, onChange, label, max, testId, labels }: PickerProps) {
  const { t } = useTranslation();
  const [search, setSearch] = useState('');
  const [picked, setPicked] = useState<number | undefined>();
  const grade = picked ?? initialGrade ?? grades[0];
  const found = useGradeStudents(schoolId, grade, search);
  for (const s of found.items) labels.set(s.student_id, `${s.display_name} (${s.username})`);
  const options = [...new Set([...value, ...found.items.map((s) => s.student_id)])].map((id) => ({ value: id, label: labels.get(id) ?? t('lms.challenges.wizard.pickedStudent') }));
  return (
    <Flex vertical gap="small">
      <Flex gap="small" wrap>
        <SearchSelect
          aria-label={t('lms.challenges.wizard.pickerGrade', { label })}
          value={grade}
          disabled={!schoolId}
          onChange={(g: number) => setPicked(g)}
          options={grades.map((g) => ({ value: g, label: t('lms.challenges.gradeN', { n: g }) }))}
          popupMatchSelectWidth={false}
        />
        <SearchSelect
          mode="multiple"
          showSearch
          filterOption={false}
          allowClear
          aria-label={label}
          placeholder={t('lms.challenges.wizard.searchStudent')}
          disabled={!schoolId}
          value={value}
          onSearch={setSearch}
          onChange={(ids: string[]) => onChange(ids)}
          loading={found.isFetching}
          maxCount={max}
          notFoundContent={t('lms.challenges.wizard.noStudent')}
          options={options}
          data-testid={testId}
          style={{ flex: 1, minWidth: 0 }}
        />
      </Flex>
      <ChallengeProblem error={found.error} />
    </Flex>
  );
}

export function StepTargets({ v, set, template, errors }: { v: ChallengeFormValues; set: Patch; template: ChallengeTemplate | undefined; errors: FieldErrors }) {
  const { t } = useTranslation();
  const school = useSchoolGrades(v.schoolId);
  const grades = gradeOptions(template?.grade_levels, school.grades);
  const labels = useRef(new Map<string, string>()).current;
  const previous = v.schoolId;
  const onSchool = useCallback(
    // another school has other grades and students: the audience starts again
    (schoolId: string | undefined) => set(previous && schoolId !== previous ? { schoolId, grade: undefined, includeIds: [], excludeIds: [] } : { schoolId }),
    [previous, set],
  );
  const conflicts = excludedIncludes(v);
  const includeGrade = school.grades.find((g) => g !== v.grade);
  return (
    <>
      <ChallengeProblem error={school.error} />
      <Form.Item label={t('portal.school_lms.common.school')} required validateStatus={status(errors.schoolId)} help={errors.schoolId}>
        <SchoolSelect value={v.schoolId} onChange={onSchool} />
      </Form.Item>
      <Form.Item label={t('lms.challenges.wizard.grade')} required validateStatus={status(errors.grade)} help={errors.grade}>
        <SearchSelect
          aria-label={t('lms.challenges.wizard.grade')}
          placeholder={t('lms.challenges.wizard.grade')}
          disabled={!v.schoolId}
          loading={school.isFetching}
          value={v.grade}
          onChange={(grade: number) => set({ grade })}
          options={grades.map((n) => ({ value: n, label: t('lms.challenges.gradeN', { n }) }))}
          data-testid="grade-select"
        />
      </Form.Item>
      <Form.Item label={t('lms.challenges.wizard.include')} validateStatus={status(errors.includeIds)} help={errors.includeIds}>
        <StudentPicker
          schoolId={v.schoolId}
          grades={school.grades}
          initialGrade={includeGrade}
          value={v.includeIds}
          onChange={(includeIds) => set({ includeIds })}
          label={t('lms.challenges.wizard.include')}
          max={CHALLENGE_LIMITS.includeStudents.maxItems}
          testId="pick-include"
          labels={labels}
        />
      </Form.Item>
      <Form.Item label={t('lms.challenges.wizard.exclude')} validateStatus={status(errors.excludeIds)} help={errors.excludeIds}>
        <StudentPicker
          schoolId={v.schoolId}
          grades={school.grades}
          initialGrade={v.grade}
          value={v.excludeIds}
          onChange={(excludeIds) => set({ excludeIds })}
          label={t('lms.challenges.wizard.exclude')}
          max={CHALLENGE_LIMITS.excludeStudents.maxItems}
          testId="pick-exclude"
          labels={labels}
        />
      </Form.Item>
      {conflicts.length ? <Alert type="warning" showIcon data-testid="exclude-wins" message={t('lms.challenges.wizard.excludeWins', { n: conflicts.length })} /> : null}
      <Flex vertical gap="small" data-testid="audience-preview">
        <Typography.Text strong>
          {v.grade
            ? t('lms.challenges.wizard.preview', { grade: v.grade, include: v.includeIds.length, exclude: v.excludeIds.length })
            : t('lms.challenges.wizard.previewNoGrade', { include: v.includeIds.length, exclude: v.excludeIds.length })}
        </Typography.Text>
        <Typography.Text type="secondary">{t('lms.challenges.wizard.previewServer')}</Typography.Text>
      </Flex>
    </>
  );
}
