// Assignment form model of the wizard and the edit dialog (F055 T006; F055-AC-02…AC-04, AC-12). Pure functions:
// form values → AssignmentCreate / AssignmentUpdate payload, client-side checks with the limits of the contract
// (fieldSchemas.ts), and which fields an assignment in a given status may change. Targets follow ADR-0095: one school
// and one subject, then either grades of the school (one `grade` target per grade, plus students of other grades of the
// same school included, minus excluded students — exclude wins) or one `students` target; no class, study group or
// course. The server stays the authority (it applies `graded_assignment_release_default`, re-validates, answers 422 for
// a student of another school and ASSIGNMENT_NOT_EDITABLE); nothing here is a business decision (AGENTS R1).
import type { components } from '../../../../shared/api/schema.gen';
import { LMS_FIELDS, LMS_LIMITS, type ContentType } from './fieldSchemas';

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

/** `grades`: grades of the school ± students; `students`: only a list of students of the school. */
export type TargetMode = 'grades' | 'students';

export interface AssignmentFormValues {
  /** System Admin only: the one target tenant (F055-AC-07). */
  tenantId?: string;
  content?: ContentChoice;
  schoolId?: string;
  subjectId?: string;
  targetMode: TargetMode;
  /** Grades (1–9) of the school, one `grade` target each. */
  grades: number[];
  /** Students of other grades of the same school added to the grades (`include_student_ids`). */
  includeIds: string[];
  /** Students left out of the grades (`exclude_student_ids`, wins over include). */
  excludeIds: string[];
  /** Students of the `students` target. */
  studentIds: string[];
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
  targetMode: 'grades',
  grades: [],
  includeIds: [],
  excludeIds: [],
  studentIds: [],
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

/** Client-side checks with the limits of AssignmentCreate / AssignmentTarget (F055-AC-04); empty = OK. */
export function validateAssignment(v: AssignmentFormValues): FormIssue[] {
  const out: FormIssue[] = [];
  const t = v.title.trim();
  if (t.length < LMS_FIELDS.assignmentTitle.minLength) out.push({ field: 'title', key: 'required' });
  if (v.title.length > LMS_FIELDS.assignmentTitle.maxLength) out.push({ field: 'title', key: 'tooLong', values: { max: LMS_FIELDS.assignmentTitle.maxLength } });
  if ((v.instructions ?? '').length > LMS_FIELDS.assignmentInstructions.maxLength)
    out.push({ field: 'instructions', key: 'tooLong', values: { max: LMS_FIELDS.assignmentInstructions.maxLength } });
  if (!v.content) out.push({ field: 'content', key: 'required' });
  if (!v.schoolId) out.push({ field: 'schoolId', key: 'required' });
  if (!v.subjectId) out.push({ field: 'subjectId', key: 'required' });
  if (v.targetMode === 'students') {
    const max = LMS_LIMITS.targetStudents.maxItems;
    if (v.studentIds.length < 1) out.push({ field: 'studentIds', key: 'required' });
    if (v.studentIds.length > max) out.push({ field: 'studentIds', key: 'tooMany', values: { max } });
  } else {
    const g = LMS_LIMITS.gradeLevel;
    if (v.grades.length < 1) out.push({ field: 'grades', key: 'required' });
    else if (v.grades.length > LMS_LIMITS.targets.maxItems) out.push({ field: 'grades', key: 'tooMany', values: { max: LMS_LIMITS.targets.maxItems } });
    else if (v.grades.some((x) => !Number.isInteger(x) || x < g.minimum || x > g.maximum)) out.push({ field: 'grades', key: 'range', values: { min: g.minimum, max: g.maximum } });
    if (v.includeIds.length > LMS_LIMITS.includeStudents.maxItems) out.push({ field: 'includeIds', key: 'tooMany', values: { max: LMS_LIMITS.includeStudents.maxItems } });
    if (v.excludeIds.length > LMS_LIMITS.excludeStudents.maxItems) out.push({ field: 'excludeIds', key: 'tooMany', values: { max: LMS_LIMITS.excludeStudents.maxItems } });
  }
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

/** Students both included and excluded: exclude wins (ADR-0095), shown as a hint before sending. */
export function excludedIncludes(v: Pick<AssignmentFormValues, 'includeIds' | 'excludeIds'>): string[] {
  const ex = new Set(v.excludeIds);
  return v.includeIds.filter((id) => ex.has(id));
}

/**
 * `targets[]` of AssignmentCreate: one `grade` target per grade (ascending, no duplicate). Included students ride on the
 * first grade target (they belong to other grades of the same school; the server counts each student once); the exclude
 * list goes on every grade target so a left-out student never enters the snapshot, whatever their grade.
 */
export function toTargets(v: AssignmentFormValues): AssignmentCreate['targets'] {
  if (v.targetMode === 'students') return [{ target_type: 'students', student_ids: [...new Set(v.studentIds)] }];
  const grades = [...new Set(v.grades)].sort((a, b) => a - b);
  const include = [...new Set(v.includeIds)];
  const exclude = [...new Set(v.excludeIds)];
  return grades.map((grade, i) => ({
    target_type: 'grade' as const,
    grade_level: grade,
    ...(i === 0 && include.length ? { include_student_ids: include } : {}),
    ...(exclude.length ? { exclude_student_ids: exclude } : {}),
  }));
}

/** AssignmentCreate body; `feedback_timing`/`result_release` only when the user set them (F055-AC-03). */
export function toCreatePayload(v: AssignmentFormValues): AssignmentCreate {
  const follow = v.followLatest && followLatestAllowed(v.mode);
  const c = v.content!;
  return {
    ...(v.tenantId ? { tenant_id: v.tenantId } : {}),
    school_id: v.schoolId!,
    subject_id: v.subjectId!,
    title: v.title.trim(),
    ...(v.instructions?.trim() ? { instructions: v.instructions.trim() } : {}),
    content_ref: { content_type: c.contentType, content_id: c.contentId, ...(!follow && c.versionId ? { pinned_version_id: c.versionId } : {}) },
    follow_latest: follow,
    mode: v.mode,
    targets: toTargets(v),
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
  school_id: 'schoolId',
  subject_id: 'subjectId',
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

/** Field of a target pointer (`/targets/0/include_student_ids/2` → includeIds); the target itself → grades. */
const TARGET_FIELD: Record<string, keyof AssignmentFormValues> = {
  include_student_ids: 'includeIds',
  exclude_student_ids: 'excludeIds',
  student_ids: 'studentIds',
  grade_level: 'grades',
};

/** `errors[].field` of 422 VALIDATION_FAILED (JSON pointer or dotted path) → the form field it belongs to. */
export function formFieldOf(serverField: string): keyof AssignmentFormValues | undefined {
  const parts = serverField.replace(/^\//, '').split(/[/.]/);
  if (parts[0] === 'targets') return parts.map((p) => TARGET_FIELD[p]).find(Boolean) ?? 'grades';
  return SERVER_FIELD[parts[0] ?? ''];
}

const TARGET_STEP: (keyof AssignmentFormValues)[] = ['schoolId', 'subjectId', 'targetMode', 'grades', 'includeIds', 'excludeIds', 'studentIds', 'tenantId'];
const SCHEDULE_STEP: (keyof AssignmentFormValues)[] = ['title', 'instructions', 'openAt', 'dueAt', 'allowLate', 'lateWindowHours'];

/** Wizard step that holds a field (0 content · 1 targets · 2 schedule · 3 policy). */
export function stepOf(field: keyof AssignmentFormValues): number {
  if (field === 'content') return 0;
  if (TARGET_STEP.includes(field)) return 1;
  if (SCHEDULE_STEP.includes(field)) return 2;
  return 3;
}
