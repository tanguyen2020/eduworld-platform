// Create wizard of a grade challenge / school tournament (F108 T028; F108-AC-14, AC-16): Template → Thông tin → Đối
// tượng → Xem lại. Checks before leaving a step are shape-only (required, maxLength, maxItems — challengeForm); every
// business rule is the server's. "Tạo thử thách" sends createChallenge with one Idempotency-Key per intent (a double
// click or a retry with the same data creates one challenge). Server errors: 403 RESOURCE_NOT_ASSIGNED → the template
// field (step 1); 422 VALIDATION_FAILED errors[] → their field and step; anything else → translated message + trace
// id. A System Admin works on the one tenant picked in the topbar, sent as X-Tenant-Id and `tenant_id`; the screen
// keys this wizard by that tenant, so switching it starts the wizard again. Desktop/tablet only (ADMIN_PORTAL §1.5).
import { Alert, Button, Descriptions, Flex, Form, Grid, Steps } from 'antd';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { ActionGate } from '../../../../permissions/ActionGate';
import { ApiError } from '../../../../shared/api/middleware/problem';
import { useSession } from '../../../../session/SessionProvider';
import { formatDateTime, localizedName } from '../../common/format';
import { useSchoolLmsScope } from '../../common/scope';
import { useSchoolGrades } from '../../lms/api/targets';
import { useChallengeTemplates, useCreateChallenge } from '../api/challenges';
import { ChallengeProblem } from '../ChallengeUi';
import {
  CHALLENGE_STEPS,
  EMPTY_CHALLENGE_FORM,
  fieldOf,
  paramsOf,
  stepOf,
  toCreatePayload,
  validateChallenge,
  type ChallengeField,
  type ChallengeFormValues,
  type FormIssue,
} from '../model/challengeForm';
import { StepParams, type FieldErrors } from './StepParams';
import { StepTargets } from './StepTargets';
import { StepTemplate } from './StepTemplate';

const REVIEW = CHALLENGE_STEPS.length - 1;

export function ChallengeWizard() {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const screens = Grid.useBreakpoint();
  const scope = useSchoolLmsScope();
  const { state } = useSession();
  const platform = state.scope?.kind !== 'tenant';
  const templates = useChallengeTemplates();
  const create = useCreateChallenge();
  const [v, setV] = useState<ChallengeFormValues>(EMPTY_CHALLENGE_FORM);
  const [step, setStep] = useState(0);
  const [shown, setShown] = useState<Set<number>>(new Set());
  const [serverErrors, setServerErrors] = useState<FieldErrors>({});
  const template = templates.items.find((x) => x.template_id === v.templateId);
  const school = useSchoolGrades(v.schoolId);
  const issues = useMemo(() => validateChallenge(v), [v]);
  const issueText = (i: FormIssue) => t(`lms.challenges.form.${i.key}`, i.values);
  const errors: FieldErrors = { ...serverErrors };
  for (const i of issues) if (shown.has(stepOf(i.field)) && !errors[i.field]) errors[i.field] = issueText(i);

  const set = (p: Partial<ChallengeFormValues>) => {
    setV((old) => ({ ...old, ...p }));
    setServerErrors((old) => {
      const next = { ...old };
      for (const k of Object.keys(p) as (keyof ChallengeFormValues)[]) {
        if (k === 'params') for (const name of Object.keys(p.params ?? {})) if (p.params?.[name] !== v.params[name]) delete next[`params.${name}`];
        if (k !== 'params') delete next[k];
      }
      return next;
    });
  };
  const stepValid = (s: number) => !issues.some((i) => stepOf(i.field) === s);
  const next = () => {
    setShown((old) => new Set(old).add(step));
    if (stepValid(step)) setStep(step + 1);
  };
  const submit = () => {
    setShown(new Set([0, 1, 2]));
    if (issues.length) {
      setStep(Math.min(...issues.map((i) => stepOf(i.field))));
      return;
    }
    const body = toCreatePayload(v, template, platform ? scope.key : undefined);
    create.mutate(body, {
      onSuccess: (c) => navigate(`../${c.challenge_id}`, { replace: true }),
      onError: (e) => {
        if (!(e instanceof ApiError)) return;
        if (e.code === 'RESOURCE_NOT_ASSIGNED') {
          setServerErrors({ templateId: t('lms.challenges.errors.RESOURCE_NOT_ASSIGNED') });
          void templates.refetch();
          setStep(0);
          return;
        }
        if (e.code === 'VALIDATION_FAILED') {
          const mapped: FieldErrors = {};
          for (const [pointer, fe] of Object.entries(e.fieldErrors)) {
            const f = fieldOf(pointer, body);
            if (f && !mapped[f]) mapped[f] = t('lms.challenges.errors.field', { code: fe.code });
          }
          const fields = Object.keys(mapped) as ChallengeField[];
          setServerErrors(mapped);
          if (fields.length) setStep(Math.min(...fields.map(stepOf)));
        }
      },
    });
  };

  if (!screens.md) return <Alert type="info" showIcon message={t('lms.challenges.wizard.desktopOnly')} />;
  const errorStep = create.error instanceof ApiError && create.error.code === 'RESOURCE_NOT_ASSIGNED' ? 0 : REVIEW;
  const params = Object.entries(paramsOf(v, template));
  const schoolName = school.school ? localizedName(school.school.name, i18n.language) : '';
  return (
    <Form layout="vertical" component="div" data-testid="challenge-wizard">
      <Steps current={step} items={CHALLENGE_STEPS.map((s) => ({ title: t(`lms.challenges.wizard.steps.${s}`) }))} />
      <Flex vertical gap="middle">
        {step === errorStep ? <ChallengeProblem error={create.error} testId="wizard-error" /> : null}
        {step === 0 && (
          <StepTemplate
            templates={templates.items}
            loading={templates.isFetching}
            loadError={templates.error}
            value={v.templateId}
            // another template allows other parameters and grades: they start again
            onChange={(templateId) => set(templateId !== v.templateId ? { templateId, params: {}, grade: undefined } : { templateId })}
            error={errors.templateId}
          />
        )}
        {step === 1 && <StepParams v={v} set={set} template={template} errors={errors} />}
        {step === 2 && <StepTargets v={v} set={set} template={template} errors={errors} />}
        {step === REVIEW && (
          <Descriptions bordered size="small" column={1} data-testid="challenge-review">
            <Descriptions.Item label={t('lms.challenges.wizard.template')}>{template?.name}</Descriptions.Item>
            <Descriptions.Item label={t('lms.challenges.wizard.name')}>{v.name.trim()}</Descriptions.Item>
            <Descriptions.Item label={t('lms.challenges.wizard.startsAt')}>{formatDateTime(v.startsAt)}</Descriptions.Item>
            {v.endsAt ? <Descriptions.Item label={t('lms.challenges.wizard.endsAt')}>{formatDateTime(v.endsAt)}</Descriptions.Item> : null}
            {params.map(([k, value]) => (
              <Descriptions.Item key={k} label={t(`lms.challenges.params.${k}`, { defaultValue: k })}>
                {typeof value === 'boolean' ? t(value ? 'portal.school_lms.common.yes' : 'portal.school_lms.common.no') : String(value)}
              </Descriptions.Item>
            ))}
            <Descriptions.Item label={t('portal.school_lms.common.school')}>{schoolName}</Descriptions.Item>
            <Descriptions.Item label={t('lms.challenges.wizard.audience')}>
              {t('lms.challenges.wizard.preview', { grade: v.grade, include: v.includeIds.length, exclude: v.excludeIds.length })}
            </Descriptions.Item>
          </Descriptions>
        )}
        <Flex gap="small" justify="flex-end">
          <Button onClick={() => (step === 0 ? navigate('..') : setStep(step - 1))}>{step === 0 ? t('portal.school_lms.common.cancel') : t('lms.challenges.wizard.back')}</Button>
          {step < REVIEW ? (
            <Button type="primary" onClick={next}>
              {t('lms.challenges.wizard.next')}
            </Button>
          ) : (
            <ActionGate actionKey="action.school_lms.challenges.create" mode="disable">
              <Button type="primary" loading={create.isPending} disabled={create.isPending} onClick={submit}>
                {t('lms.challenges.wizard.create')}
              </Button>
            </ActionGate>
          )}
        </Flex>
      </Flex>
    </Form>
  );
}
