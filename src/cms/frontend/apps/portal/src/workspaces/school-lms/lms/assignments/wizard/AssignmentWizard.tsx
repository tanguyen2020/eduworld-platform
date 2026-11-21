// Assignment wizard (F055 T018; F055-AC-01…AC-07): 5 steps Nội dung → Đối tượng → Thời gian → Chính sách → Xem lại.
// Checks of each step use the contract limits (assignmentForm.validateAssignment); "Giao bài" runs createAssignment +
// publishAssignment as one intent (useCreateAndPublishAssignment: same Idempotency-Keys on a retry or double click).
// Server errors: VALIDATION_FAILED errors[] → their field and step; CONTENT_NOT_PUBLISHED / RESOURCE_NOT_ASSIGNED →
// back to the content step; others → translated message + trace id. A System Admin works on the one tenant picked in
// the topbar (SingleTenantGate) which is also sent as `tenant_id` (F055-AC-07). Prefill from F056 shortcuts:
// `?objective_id=&school_id=&subject_id=&grade=&student_ids=&mode=` (ADR-0095: grade of a school, no class). Wizard is
// desktop/tablet only (ADMIN_PORTAL §1.5).
import { Alert, Button, Descriptions, Flex, Form, Grid, Steps } from 'antd';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { ActionGate } from '../../../../../permissions/ActionGate';
import { ApiError } from '../../../../../shared/api/middleware/problem';
import { useSession } from '../../../../../session/SessionProvider';
import { useSchoolLmsScope } from '../../../common/scope';
import { formatDateTime } from '../../../common/format';
import { useCreateAndPublishAssignment } from '../../api/assignments';
import { EMPTY_FORM, formFieldOf, stepOf, toCreatePayload, validateAssignment, type AssignmentFormValues, type FormIssue } from '../../model/assignmentForm';
import { LMS_LIMITS } from '../../model/fieldSchemas';
import { isContentError } from '../../model/problemMessages';
import { LmsProblem } from '../../LmsUi';
import { StepContent } from './StepContent';
import { StepPolicy, StepSchedule } from './StepDetails';
import { StepTargets } from './StepTargets';

const STEP_FIELDS: (keyof AssignmentFormValues)[][] = [
  ['content'],
  ['schoolId', 'subjectId', 'grades', 'includeIds', 'excludeIds', 'studentIds'],
  ['title', 'instructions', 'openAt', 'dueAt', 'lateWindowHours'],
  ['maxAttempts'],
];
const STEPS = ['content', 'targets', 'schedule', 'policy', 'review'] as const;

function initialValues(params: URLSearchParams): AssignmentFormValues {
  const mode = params.get('mode');
  const students = params.get('student_ids')?.split(',').filter(Boolean) ?? [];
  const grade = Number(params.get('grade'));
  const { minimum, maximum } = LMS_LIMITS.gradeLevel;
  return {
    ...EMPTY_FORM,
    ...(mode === 'practice' || mode === 'graded' ? { mode } : {}),
    ...(params.get('school_id') ? { schoolId: params.get('school_id')! } : {}),
    ...(params.get('subject_id') ? { subjectId: params.get('subject_id')! } : {}),
    ...(Number.isInteger(grade) && grade >= minimum && grade <= maximum ? { grades: [grade] } : {}),
    ...(students.length ? { targetMode: 'students' as const, studentIds: students } : {}),
  };
}

/** Text of the targets on the review step: "Khối 3, Khối 4 · thêm 2 · bớt 1" or "Học sinh cụ thể: 5". */
function targetSummary(v: AssignmentFormValues, t: (k: string, o?: Record<string, unknown>) => string): string {
  if (v.targetMode === 'students') return t('lms.wizard.targetCount', { type: t('lms.targetMode.students'), n: v.studentIds.length });
  const grades = [...v.grades].sort((a, b) => a - b).map((n) => t('lms.wizard.gradeN', { n }));
  return [grades.join(', '), v.includeIds.length ? t('lms.wizard.includeCount', { n: v.includeIds.length }) : '', v.excludeIds.length ? t('lms.wizard.excludeCount', { n: v.excludeIds.length }) : '']
    .filter(Boolean)
    .join(' · ');
}

export function AssignmentWizard() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const screens = Grid.useBreakpoint();
  const scope = useSchoolLmsScope();
  const { state } = useSession();
  const platform = state.scope?.kind !== 'tenant';
  const [v, setV] = useState<AssignmentFormValues>(() => initialValues(params));
  const [step, setStep] = useState(0);
  const [shown, setShown] = useState<Set<number>>(new Set());
  const [serverErrors, setServerErrors] = useState<Partial<Record<keyof AssignmentFormValues, string>>>({});
  const create = useCreateAndPublishAssignment();
  const values = useMemo(() => ({ ...v, tenantId: platform ? scope.key : undefined }), [v, platform, scope.key]);
  const issues = validateAssignment(values);
  const issueText = (i: FormIssue) => t(`lms.form.${i.key}`, i.values);
  const errors: Partial<Record<keyof AssignmentFormValues, string>> = { ...serverErrors };
  for (const i of issues) if (shown.has(stepOf(i.field)) && !errors[i.field]) errors[i.field] = issueText(i);
  const set = (p: Partial<AssignmentFormValues>) => {
    setV((old) => ({ ...old, ...p }));
    setServerErrors((old) => {
      const next = { ...old };
      for (const k of Object.keys(p)) delete next[k as keyof AssignmentFormValues];
      return next;
    });
  };
  const stepValid = (s: number) => !issues.some((i) => STEP_FIELDS[s]?.includes(i.field));
  const next = () => {
    setShown((old) => new Set(old).add(step));
    if (stepValid(step)) setStep(step + 1);
  };
  const submit = () => {
    setShown(new Set([0, 1, 2, 3]));
    if (issues.length) {
      setStep(Math.min(...issues.map((i) => stepOf(i.field))));
      return;
    }
    create.mutate(toCreatePayload(values), {
      onSuccess: (a) => navigate(`../${a.assignment_id}`, { replace: true }),
      onError: (e) => {
        if (isContentError(e)) return setStep(0);
        if (e instanceof ApiError && e.code === 'VALIDATION_FAILED') {
          const mapped: Partial<Record<keyof AssignmentFormValues, string>> = {};
          for (const [field, fe] of Object.entries(e.fieldErrors)) {
            const f = formFieldOf(field);
            if (f) mapped[f] = t(`error.field.${fe.code}`, { defaultValue: t('error.VALIDATION_FAILED') });
          }
          const fields = Object.keys(mapped) as (keyof AssignmentFormValues)[];
          setServerErrors(mapped);
          if (fields.length) setStep(Math.min(...fields.map(stepOf)));
        }
      },
    });
  };
  if (!screens.md) return <Alert type="info" showIcon message={t('lms.wizard.desktopOnly')} />;
  const contentError = create.error && isContentError(create.error) ? undefined : errors.content;
  return (
    <Form layout="vertical" component="div" data-testid="assignment-wizard">
      <Steps current={step} items={STEPS.map((s) => ({ title: t(`lms.wizard.steps.${s}`) }))} />
      <Flex vertical gap="middle">
        {step === 4 || isContentError(create.error) ? <LmsProblem error={create.error} testId="wizard-error" /> : null}
        {step === 0 && <StepContent value={v.content} onChange={(content) => set({ content })} objectiveId={params.get('objective_id') ?? undefined} error={contentError} />}
        {step === 1 && <StepTargets v={v} set={set} errors={errors} />}
        {step === 2 && <StepSchedule v={v} set={set} errors={errors} />}
        {step === 3 && <StepPolicy v={v} set={set} errors={errors} />}
        {step === 4 && (
          <Descriptions bordered size="small" column={1} data-testid="wizard-review">
            <Descriptions.Item label={t('lms.wizard.steps.content')}>{v.content?.title}</Descriptions.Item>
            <Descriptions.Item label={t('lms.wizard.title')}>{v.title}</Descriptions.Item>
            <Descriptions.Item label={t('lms.wizard.targetType')}>{targetSummary(v, t)}</Descriptions.Item>
            <Descriptions.Item label={t('lms.wizard.openAt')}>{formatDateTime(v.openAt)}</Descriptions.Item>
            <Descriptions.Item label={t('lms.wizard.dueAt')}>{formatDateTime(v.dueAt)}</Descriptions.Item>
            <Descriptions.Item label={t('lms.wizard.mode')}>{t(`lms.mode.${v.mode}`)}</Descriptions.Item>
            <Descriptions.Item label={t('lms.wizard.followLatest')}>{v.followLatest ? t('portal.school_lms.common.yes') : t('portal.school_lms.common.no')}</Descriptions.Item>
          </Descriptions>
        )}
        <Flex gap="small" justify="flex-end">
          <Button onClick={() => (step === 0 ? navigate('..') : setStep(step - 1))}>{step === 0 ? t('portal.school_lms.common.cancel') : t('lms.wizard.back')}</Button>
          {step < 4 ? (
            <Button type="primary" onClick={next}>
              {t('lms.wizard.next')}
            </Button>
          ) : (
            <ActionGate actionKey="action.school_lms.assignments.edit" mode="disable">
              <Button type="primary" loading={create.isPending} disabled={create.isPending} onClick={submit}>
                {t('lms.wizard.assign')}
              </Button>
            </ActionGate>
          )}
        </Flex>
      </Flex>
    </Form>
  );
}
