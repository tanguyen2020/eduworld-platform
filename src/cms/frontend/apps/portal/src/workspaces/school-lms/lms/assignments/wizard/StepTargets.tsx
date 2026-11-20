// Step 2 "Đối tượng" (F055 T018, F055-AC-02, AC-24): course (listCourses — only the courses of the user's scope), then
// target type class (one or more classes of the course's school), study_group (listStudyGroups; hidden when the study
// group menu is turned off) or students (students of the course's class). Options only come from the API (scope).
import { Form, Radio, Select } from 'antd';
import { useTranslation } from 'react-i18next';
import { useClassStudents, useSchoolClasses } from '../../../classes/useClasses';
import { useCourses, type Course } from '../../api/courses';
import { useStudyGroups } from '../../api/schedules';
import type { AssignmentFormValues } from '../../model/assignmentForm';
import type { TargetType } from '../../model/fieldSchemas';
import { LmsProblem, useActionEnabled } from '../../LmsUi';

export function courseLabel(c: Course, locale: string): string {
  const subject = (locale.startsWith('en') && c.subject_name?.en) || c.subject_name?.vi || '';
  return [c.class_name, subject].filter(Boolean).join(' · ');
}

type Patch = (p: Partial<AssignmentFormValues>) => void;

export function StepTargets({ v, set, errors }: { v: AssignmentFormValues; set: Patch; errors: Partial<Record<keyof AssignmentFormValues, string>> }) {
  const { t, i18n } = useTranslation();
  const courses = useCourses({ status: 'active' });
  const course = courses.items.find((c) => c.course_id === v.courseId);
  const groupsOn = useActionEnabled('action.school_lms.study_groups.view');
  const classes = useSchoolClasses(v.targetType === 'class' ? course?.school_id : undefined);
  const groups = useStudyGroups(course?.school_id, undefined, v.targetType === 'study_group' && groupsOn);
  const students = useClassStudents(v.targetType === 'students' ? course?.school_id : undefined, course?.class_id);
  const types: TargetType[] = groupsOn ? ['class', 'study_group', 'students'] : ['class', 'students'];
  const options =
    v.targetType === 'class'
      ? classes.items.map((c) => ({ value: c.class_id, label: c.name }))
      : v.targetType === 'study_group'
        ? groups.items.map((g) => ({ value: g.study_group_id, label: g.name }))
        : students.items.map((s) => ({ value: s.student_id, label: s.display_name }));
  return (
    <>
      <LmsProblem error={courses.error ?? classes.error ?? groups.error ?? students.error} />
      <Form.Item label={t('lms.wizard.course')} required validateStatus={errors.courseId ? 'error' : undefined} help={errors.courseId}>
        <Select
          showSearch
          optionFilterProp="label"
          aria-label={t('lms.wizard.course')}
          placeholder={t('lms.wizard.course')}
          loading={courses.isFetching}
          value={v.courseId}
          onChange={(id: string) => {
            const c = courses.items.find((x) => x.course_id === id);
            set({ courseId: id, targetIds: c && v.targetType === 'class' ? [c.class_id] : [] });
          }}
          options={courses.items.map((c) => ({ value: c.course_id, label: courseLabel(c, i18n.language) }))}
        />
      </Form.Item>
      <Form.Item label={t('lms.wizard.targetType')}>
        <Radio.Group
          aria-label={t('lms.wizard.targetType')}
          value={v.targetType}
          onChange={(e) => set({ targetType: e.target.value as TargetType, targetIds: [] })}
          options={types.map((k) => ({ value: k, label: t(`lms.targetType.${k}`) }))}
        />
      </Form.Item>
      <Form.Item label={t(`lms.wizard.targets.${v.targetType}`)} required validateStatus={errors.targetIds ? 'error' : undefined} help={errors.targetIds}>
        <Select
          mode="multiple"
          showSearch
          optionFilterProp="label"
          aria-label={t(`lms.wizard.targets.${v.targetType}`)}
          disabled={!course}
          value={v.targetIds}
          onChange={(ids: string[]) => set({ targetIds: ids })}
          options={options}
          data-testid="target-select"
        />
      </Form.Item>
    </>
  );
}
