// Form model of the "Thử thách khối / giải trường" wizard (F108 T028; F108-AC-14; ADR-0095 mục 3): limits mirrored from
// specs/openapi/eduworld-api.v1.yaml (each entry names its source — `challengeForm.test.ts` fails when they disagree,
// AGENTS R33), shape-only checks (required, maxLength, maxItems), the createChallenge body and the mapping of server
// error pointers back to wizard fields. Audience = one school of the scope, one grade, optional students of the same
// school to add and to leave out (the server applies "exclude wins" and computes the effective students). No class.
// The server decides every business rule (AGENTS R1): unknown template parameters, students of another school,
// unassigned templates all come back as 422 / 403 and are shown on their field.
import type { FieldSchema } from '../../common/fieldSchemas';
import type { components } from '../../../../shared/api/schema.gen';

type S = components['schemas'];
export type ChallengeTemplate = S['ChallengeTemplate'];
export type Challenge = S['Challenge'];
export type ChallengeCreate = S['ChallengeCreate'];
export type ChallengeStatus = Challenge['status'];

export const CHALLENGE_FIELDS = {
  name: { source: 'schemas/ChallengeCreate/properties/name', maxLength: 80 },
  cancelReason: { source: 'schemas/CancelReason/properties/reason', minLength: 1, maxLength: 1000 },
  overrideReason: { source: 'schemas/BracketNodeOverride/properties/reason', minLength: 1, maxLength: 1000 },
} as const satisfies Record<string, FieldSchema>;

export const CHALLENGE_LIMITS = {
  /** ChallengeCreate.targets[] item `student_ids` (students added to the grade). */
  includeStudents: { source: 'schemas/ChallengeCreate/properties/targets/items/properties/student_ids', maxItems: 500 },
  excludeStudents: { source: 'schemas/ChallengeCreate/properties/exclude_student_ids', maxItems: 500 },
  params: { source: 'schemas/ChallengeCreate/properties/params', maxProperties: 20 },
  gradeLevel: { source: 'schemas/GradeLevel', minimum: 1, maximum: 9 },
} as const;

/** "Giá trị chuỗi ≤ 255 ký tự" of ChallengeCreate.params (description of the contract). */
export const PARAM_STRING_MAX = 255;

export const CHALLENGE_ENUMS = {
  status: { source: 'schemas/Challenge/properties/status', values: ['scheduled', 'running', 'completed', 'cancelled'] },
  kind: { source: 'schemas/ChallengeTemplate/properties/kind', values: ['grade_challenge', 'school_tournament'] },
  seedingPolicy: { source: 'schemas/Tournament/properties/seeding_policy', values: ['rating', 'random', 'manual'] },
} as const;

/** Input of a template parameter. Keys named in the ChallengeCreate.params description get a typed input; any other allowed key a text box. */
export type ParamInput = 'integer' | 'datetime' | 'boolean' | 'seeding_policy' | 'subject' | 'text';
const PARAM_INPUTS: Record<string, ParamInput> = {
  max_entries: 'integer',
  seeding_policy: 'seeding_policy',
  registration_open_at: 'datetime',
  registration_close_at: 'datetime',
  auto_confirm: 'boolean',
  subject_id: 'subject',
};
export const paramInput = (key: string): ParamInput => PARAM_INPUTS[key] ?? 'text';

export type ParamValue = string | number | boolean | undefined;

export interface ChallengeFormValues {
  templateId?: string;
  name: string;
  startsAt?: string;
  endsAt?: string;
  params: Record<string, ParamValue>;
  schoolId?: string;
  grade?: number;
  includeIds: string[];
  excludeIds: string[];
}

export const EMPTY_CHALLENGE_FORM: ChallengeFormValues = { name: '', params: {}, includeIds: [], excludeIds: [] };

/** Field of the wizard: a form value or one template parameter (`params.<key>`). */
export type ChallengeField = Exclude<keyof ChallengeFormValues, 'params'> | `params.${string}`;

export interface FormIssue {
  field: ChallengeField;
  key: 'required' | 'tooLong' | 'tooMany';
  values?: Record<string, number>;
}

/** Wizard steps: 0 template · 1 details (name, schedule, parameters) · 2 audience · 3 review. */
export const CHALLENGE_STEPS = ['template', 'details', 'targets', 'review'] as const;

export function stepOf(field: ChallengeField): number {
  if (field === 'templateId') return 0;
  if (field === 'schoolId' || field === 'grade' || field === 'includeIds' || field === 'excludeIds') return 2;
  return 1;
}

/** Shape checks only (required fields, maxLength, maxItems of the contract) — business rules stay on the server. */
export function validateChallenge(v: ChallengeFormValues): FormIssue[] {
  const out: FormIssue[] = [];
  if (!v.templateId) out.push({ field: 'templateId', key: 'required' });
  if (!v.name.trim()) out.push({ field: 'name', key: 'required' });
  else if (v.name.trim().length > CHALLENGE_FIELDS.name.maxLength) out.push({ field: 'name', key: 'tooLong', values: { max: CHALLENGE_FIELDS.name.maxLength } });
  if (!v.startsAt) out.push({ field: 'startsAt', key: 'required' });
  for (const [k, value] of Object.entries(v.params)) {
    if (typeof value === 'string' && value.length > PARAM_STRING_MAX) out.push({ field: `params.${k}`, key: 'tooLong', values: { max: PARAM_STRING_MAX } });
  }
  if (!v.schoolId) out.push({ field: 'schoolId', key: 'required' });
  if (!v.grade) out.push({ field: 'grade', key: 'required' });
  if (v.includeIds.length > CHALLENGE_LIMITS.includeStudents.maxItems) out.push({ field: 'includeIds', key: 'tooMany', values: { max: CHALLENGE_LIMITS.includeStudents.maxItems } });
  if (v.excludeIds.length > CHALLENGE_LIMITS.excludeStudents.maxItems) out.push({ field: 'excludeIds', key: 'tooMany', values: { max: CHALLENGE_LIMITS.excludeStudents.maxItems } });
  return out;
}

/** Students both added and left out (display hint only: the server applies "exclude wins"). */
export function excludedIncludes(v: Pick<ChallengeFormValues, 'includeIds' | 'excludeIds'>): string[] {
  const ex = new Set(v.excludeIds);
  return v.includeIds.filter((id) => ex.has(id));
}

/** Values of the parameters the template allows, empty ones dropped (no other key is ever sent). */
export function paramsOf(v: ChallengeFormValues, template: Pick<ChallengeTemplate, 'allowed_params'> | undefined): Record<string, string | number | boolean> {
  const out: Record<string, string | number | boolean> = {};
  for (const key of template?.allowed_params ?? []) {
    const value = v.params[key];
    if (value !== undefined && value !== '') out[key] = typeof value === 'string' ? value.trim() : value;
  }
  return out;
}

/** createChallenge body: one grade target, the added students as a `students` target, the exclude list, the allowed parameters. */
export function toCreatePayload(v: ChallengeFormValues, template: Pick<ChallengeTemplate, 'allowed_params'> | undefined, tenantId?: string): ChallengeCreate {
  const params = paramsOf(v, template);
  const targets: ChallengeCreate['targets'] = [{ target_type: 'grade', grade_level: v.grade! }];
  if (v.includeIds.length) targets.push({ target_type: 'students', student_ids: [...v.includeIds] });
  return {
    template_id: v.templateId!,
    name: v.name.trim(),
    school_id: v.schoolId!,
    starts_at: v.startsAt!,
    ...(v.endsAt ? { ends_at: v.endsAt } : {}),
    targets,
    ...(v.excludeIds.length ? { exclude_student_ids: [...v.excludeIds] } : {}),
    ...(Object.keys(params).length ? { params } : {}),
    ...(tenantId ? { tenant_id: tenantId } : {}),
  };
}

const SERVER_FIELD: Record<string, ChallengeField> = {
  template_id: 'templateId',
  name: 'name',
  starts_at: 'startsAt',
  ends_at: 'endsAt',
  school_id: 'schoolId',
  exclude_student_ids: 'excludeIds',
};

/** Field of a server error pointer (`/params/max_entries`, `/targets/1/student_ids/0`, `name`) given the body that was sent. */
export function fieldOf(pointer: string, sent: Pick<ChallengeCreate, 'targets'> | undefined): ChallengeField | undefined {
  const parts = pointer.replace(/^\//, '').split(/[/.]/);
  const head = parts[0] ?? '';
  if (head === 'params') return parts[1] ? `params.${parts[1]}` : undefined;
  if (head === 'targets') {
    const target = parts[1] !== undefined ? sent?.targets?.[Number(parts[1])] : undefined;
    return target?.target_type === 'students' || parts.includes('student_ids') ? 'includeIds' : 'grade';
  }
  return SERVER_FIELD[head];
}

/** Grades offered by the wizard: the grades the school teaches within the template's `grade_levels` (all of them when the template names none). */
export function gradeOptions(templateGrades: readonly number[] | undefined, schoolGrades: readonly number[]): number[] {
  return templateGrades?.length ? schoolGrades.filter((g) => templateGrades.includes(g)) : [...schoolGrades];
}

/** Grade of a challenge (its `grade` target). */
export const gradeOf = (c: Pick<ChallengeCreate, 'targets'> | undefined): number | undefined => c?.targets?.find((x) => x.target_type === 'grade')?.grade_level;

/** Students added to the grade of a challenge. */
export const includedOf = (c: Pick<ChallengeCreate, 'targets'> | undefined): string[] => (c?.targets ?? []).filter((x) => x.target_type === 'students').flatMap((x) => x.student_ids ?? []);
