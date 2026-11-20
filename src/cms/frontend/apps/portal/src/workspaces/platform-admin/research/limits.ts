// Input limits of the research screens (F035 US8, ADR-0041, AGENTS R28/R33): copied from
// specs/openapi/eduworld-api.v1.yaml with the pointer each comes from; `research.test.ts` fails when a value drifts.
import type { FieldLimit } from '../limits';

export const RESEARCH_LIMITS = {
  agreementRef: { source: 'components.schemas.ResearchAgreementCreate.properties.agreement_ref', minLength: 1, maxLength: 128 },
  revokeReason: { source: 'components.schemas.ResearchAgreementRevoke.properties.reason', minLength: 10, maxLength: 1000 },
  studyCode: { source: 'components.schemas.EfficacyStudyCreate.properties.code', maxLength: 64, pattern: '^[a-z0-9_-]{1,64}$' },
  studyTitle: { source: 'components.schemas.EfficacyStudyCreate.properties.title', minLength: 1, maxLength: 255 },
  studyObjectives: { source: 'components.schemas.EfficacyStudyCreate.properties.objective_ids', maxItems: 100 },
  minWeeks: { source: 'components.schemas.EfficacyStudyCreate.properties.min_intervention_weeks', minimum: 1, maximum: 52 },
  pairNo: { source: 'components.schemas.EfficacyStudyClassCreate.properties.pair_no', minimum: 1, maximum: 200 },
  waveDuration: { source: 'components.schemas.EfficacyWaveCreate.properties.duration_seconds', minimum: 60, maximum: 14400 },
  withdrawReason: { source: 'components.schemas.EfficacyWithdrawalCreate.properties.reason', minLength: 10, maxLength: 1000 },
  schoolRequestRef: { source: 'components.schemas.EfficacyWithdrawalCreate.properties.school_request_ref', maxLength: 128 },
  withdrawStudents: { source: 'components.schemas.EfficacyWithdrawalCreate.properties.student_ids', maxItems: 500 },
  excludedQuestions: { source: 'components.schemas.EfficacyReportCreate.properties.excluded_question_version_ids', maxItems: 100 },
  experimentCode: { source: 'components.schemas.LearningExperimentCreate.properties.code', maxLength: 64, pattern: '^[a-z0-9_-]{1,64}$' },
  experimentTenants: { source: 'components.schemas.LearningExperimentCreate.properties.tenant_ids', maxItems: 200 },
  armCode: { source: 'components.schemas.ExperimentArm.properties.code', maxLength: 32, pattern: '^[a-z0-9_]{1,32}$' },
  armAllocation: { source: 'components.schemas.ExperimentArm.properties.allocation_pct', minimum: 1, maximum: 99 },
  modelVersion: { source: 'components.schemas.ExperimentArm.properties.model_version', maxLength: 64 },
  stopReason: { source: 'components.schemas.LearningExperimentStop.properties.reason', minLength: 10, maxLength: 1000 },
} as const satisfies Record<string, FieldLimit>;
