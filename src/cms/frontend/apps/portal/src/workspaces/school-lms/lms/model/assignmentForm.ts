// Assignment form model of the wizard and the edit dialog (F055 T006; F055-AC-03, AC-04, AC-12). Pure functions:
// form values → AssignmentCreate / AssignmentUpdate payload, client-side checks with the limits of the contract
// (fieldSchemas.ts), and which fields an assignment in a given status may change. The server stays the authority
// (it applies `graded_assignment_release_default`, re-validates, answers ASSIGNMENT_NOT_EDITABLE); nothing here is a
// business decision (AGENTS R1).
import type { components } from '../../../../shared/api/schema.gen';
import { LMS_FIELDS, LMS_LIMITS, type ContentType, type TargetType } from './fieldSchemas';

export type AssignmentCreate = components['schemas']['AssignmentCreate'];
export type AssignmentUpdate = components['schemas']['AssignmentUpdate'];
export type Assignment = components['schemas']['Assignment'];
type FeedbackTiming = components['schemas']['FeedbackTiming'];
type ResultRelease = components['schemas']['ResultRelease'];

export interface ContentChoice {
  contentType: ContentType;
  contentId: string;
  /** Published version pinned at assignment time (absent for `follow_latest`). */
  versionId?: string;
  title: string;
}

export interface AssignmentFormValues {
  /** System Admin only: the one target tenant (F055-AC-07). */
  tenantId?: string;
  content?: ContentChoice;
  courseId?: string;
  targetType: TargetType;
  /** class ids, study group ids, or student ids according to `targetType`. */
  targetIds: string[];
  title: string;
  instructions?: string;
  openAt?: string;
  dueAt?: string;
  allowLate: boolean;
  lateWindowHours: number;
  mode: 'practice' | 'graded';
  maxAttempts: number;
  scorePolicy: 'best' | 'last' | 'first';
  followLatest: boolean;
  /** Only sent when the user changed it (F055-AC-03). */
  feedbackTiming?: FeedbackTiming;
  resultRelease?: ResultRelease;
}

export const EMPTY_FORM: AssignmentFormValues = {
  targetType: 'class',
  targetIds: [],
  title: '',
  allowLate: false,
  lateWindowHours: 0,
  mode: 'practice',
  maxAttempts: 1,
  scorePolicy: 'best',
  followLatest: false,
};

/** `follow_latest` is only possible in practice mode (CONTENT_MODEL §5.2): graded always pins the version. */
export function followLatestAllowed(mode: AssignmentFormValues['mode']): boolean {
  return mode === 'practice';
}

export type FormIssue = { field: keyof AssignmentFormValues; key: string; values?: Record<string, number> };

/** Client-side checks with the limits of AssignmentCreate (F055-AC-04); empty = OK. */
export function validateAssignment(v: AssignmentFormValues): FormIssue[] {
  const out: FormIssue[] = [];
  const t = v.title.trim();
  if (t.length < LMS_FIELDS.assignmentTitle.minLength) out.push({ field: 'title', key: 'required' });
  if (v.title.length > LMS_FIELDS.assignmentTitle.maxLength) out.push({ field: 'title', key: 'tooLong', values: { max: LMS_FIELDS.assignmentTitle.maxLength } });
  if ((v.instructions ?? '').length > LMS_FIELDS.assignmentInstructions.maxLength)
    out.push({ field: 'instructions', key: 'tooLong', values: { max: LMS_FIELDS.assignmentInstructions.maxLength } });
  if (!v.content) out.push({ field: 'content', key: 'required' });
  if (!v.courseId) out.push({ field: 'courseId', key: 'required' });
  const maxTargets = v.targetType === 'students' ? LMS_LIMITS.targetStudents.maxItems : LMS_LIMITS.targets.maxItems;
  if (v.targetIds.length < 1) out.push({ field: 'targetIds', key: 'required' });
  if (v.targetIds.length > maxTargets) out.push({ field: 'targetIds', key: 'tooMany', values: { max: maxTargets } });
  if (!v.openAt) out.push({ field: 'openAt', key: 'required' });
  if (!v.dueAt) out.push({ field: 'dueAt', key: 'required' });
  if (v.openAt && v.dueAt && !(Date.parse(v.dueAt) > Date.parse(v.openAt))) out.push({ field: 'dueAt', key: 'dueAfterOpen' });
  const lw = LMS_LIMITS.lateWindowHours;
  if (!Number.isInteger(v.lateWindowHours) || v.lateWindowHours < lw.minimum || v.lateWindowHours > lw.maximum)
    out.push({ field: 'lateWindowHours', key: 'range', values: { min: lw.minimum, max: lw.maximum } });
  const ma = LMS_LIMITS.maxAttempts;
  if (!Number.isInteger(v.maxAttempts) || v.maxAttempts < ma.minimum || v.maxAttempts > ma.maximum)
    out.push({ field: 'maxAttempts', key: 'range', values: { min: ma.minimum, max: ma.maximum } });
  return out;
}

function targets(v: AssignmentFormValues): AssignmentCreate['targets'] {
  if (v.targetType === 'students') return [{ target_type: 'students', student_ids: v.targetIds }];
  return v.targetIds.map((id) => ({ target_type: v.targetType, target_id: id }));
}

/** AssignmentCreate body; `feedback_timing`/`result_release` only when the user set them (F055-AC-03). */
export function toCreatePayload(v: AssignmentFormValues): AssignmentCreate {
  const follow = v.followLatest && followLatestAllowed(v.mode);
  const c = v.content!;
  return {
    ...(v.tenantId ? { tenant_id: v.tenantId } : {}),
    course_id: v.courseId!,
    title: v.title.trim(),
    ...(v.instructions?.trim() ? { instructions: v.instructions.trim() } : {}),
    content_ref: { content_type: c.contentType, content_id: c.contentId, ...(!follow && c.versionId ? { pinned_version_id: c.versionId } : {}) },
    follow_latest: follow,
    mode: v.mode,
    targets: targets(v),
    open_at: v.openAt!,
    due_at: v.dueAt!,
    late_policy: { allow_late: v.allowLate, late_window_hours: v.allowLate ? v.lateWindowHours : 0 },
    max_attempts: v.maxAttempts,
    score_policy: v.scorePolicy,
    ...(v.feedbackTiming ? { feedback_timing: v.feedbackTiming } : {}),
    ...(v.resultRelease ? { result_release: v.resultRelease } : {}),
  };
}

export type EditableField = 'title' | 'instructions' | 'open_at' | 'due_at' | 'late_policy' | 'max_attempts' | 'score_policy';

/** Fields the edit form offers per status (SCHOOL_LMS §4, F055-AC-12): open → due_at, instructions, late_policy only. */
export function editableFields(status: Assignment['status']): EditableField[] {
  if (status === 'draft' || status === 'scheduled') return ['title', 'instructions', 'open_at', 'due_at', 'late_policy', 'max_attempts', 'score_policy'];
  if (status === 'open') return ['due_at', 'instructions', 'late_policy'];
  return [];
}

/** Merge patch with only the editable fields that changed (no field the status does not allow is ever sent). */
export function toUpdatePayload(before: Assignment, after: Partial<AssignmentUpdate>): AssignmentUpdate {
  const allowed = new Set<string>(editableFields(before.status));
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(after)) {
    if (!allowed.has(k) || v === undefined) continue;
    const old = (before as Record<string, unknown>)[k];
    const sameInstant = typeof v === 'string' && typeof old === 'string' && /^\d{4}-\d\d-\d\dT/.test(v) && Date.parse(v) === Date.parse(old);
    if (!sameInstant && JSON.stringify(v) !== JSON.stringify(old)) out[k] = v;
  }
  return out as AssignmentUpdate;
}

const SERVER_FIELD: Record<string, keyof AssignmentFormValues> = {
  title: 'title',
  instructions: 'instructions',
  content_ref: 'content',
  course_id: 'courseId',
  targets: 'targetIds',
  open_at: 'openAt',
  due_at: 'dueAt',
  late_policy: 'lateWindowHours',
  max_attempts: 'maxAttempts',
  score_policy: 'scorePolicy',
  mode: 'mode',
  follow_latest: 'followLatest',
  feedback_timing: 'feedbackTiming',
  result_release: 'resultRelease',
  tenant_id: 'tenantId',
};

/** `errors[].field` of 422 VALIDATION_FAILED (JSON pointer or dotted path) → the form field it belongs to. */
export function formFieldOf(serverField: string): keyof AssignmentFormValues | undefined {
  const first = serverField.replace(/^\//, '').split(/[/.]/)[0] ?? '';
  return SERVER_FIELD[first];
}

/** Wizard step that holds a field (0 content · 1 targets · 2 schedule · 3 policy). */
export function stepOf(field: keyof AssignmentFormValues): number {
  if (field === 'content') return 0;
  if (field === 'courseId' || field === 'targetIds' || field === 'targetType' || field === 'tenantId') return 1;
  if (field === 'title' || field === 'instructions' || field === 'openAt' || field === 'dueAt' || field === 'allowLate' || field === 'lateWindowHours') return 2;
  return 3;
}
