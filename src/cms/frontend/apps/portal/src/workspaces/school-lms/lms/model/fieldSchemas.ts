// Field limits and enums of the F055 LMS forms, copied from specs/openapi/eduworld-api.v1.yaml (ADR-0041, F055-AC-04).
// Each entry names the OpenAPI location it mirrors; `fieldSchemas.test.ts` fails when the spec and this file disagree,
// so the spec stays the single source (AGENTS R33). Used with `openapiRules` / `openapiInputProps` of ops-ui.
import type { FieldSchema } from '../../common/fieldSchemas';

export const LMS_FIELDS = {
  assignmentTitle: { source: 'schemas/AssignmentCreate/properties/title', minLength: 1, maxLength: 120 },
  assignmentInstructions: { source: 'schemas/AssignmentCreate/properties/instructions', maxLength: 2000 },
  extendReason: { source: 'schemas/AssignmentExtendRequest/properties/reason', minLength: 3, maxLength: 500 },
  reason: { source: 'schemas/ReasonRequest/properties/reason', minLength: 1, maxLength: 500 },
  pathTitle: { source: 'schemas/LearningPathInput/properties/title', minLength: 1, maxLength: 255 },
  studyGroupName: { source: 'schemas/StudyGroupInput/properties/name', minLength: 1, maxLength: 64 },
  scheduleNote: { source: 'schemas/ScheduleItemInput/properties/note', maxLength: 500 },
  accommodationReason: { source: 'schemas/ExamAccommodationsUpdate/properties/reason', minLength: 3, maxLength: 500 },
} as const satisfies Record<string, FieldSchema>;

/** Numeric / array limits mirrored from the spec (same check). */
export const LMS_LIMITS = {
  lateWindowHours: { source: 'schemas/AssignmentCreate/properties/late_policy/properties/late_window_hours', minimum: 0, maximum: 168 },
  maxAttempts: { source: 'schemas/AssignmentCreate/properties/max_attempts', minimum: 1, maximum: 10 },
  targets: { source: 'schemas/AssignmentCreate/properties/targets', minItems: 1, maxItems: 50 },
  targetStudents: { source: 'schemas/AssignmentTarget/properties/student_ids', maxItems: 200 },
  pathNodes: { source: 'schemas/LearningPathInput/properties/nodes', minItems: 1, maxItems: 200 },
  groupMembers: { source: 'schemas/StudyGroupInput/properties/member_ids', minItems: 1, maxItems: 100 },
  schedulePeriod: { source: 'schemas/ScheduleItemInput/properties/period', minimum: 1, maximum: 15 },
  extraTimePercent: { source: 'schemas/ExamAccommodation/properties/extra_time_percent', minimum: 1, maximum: 100 },
  accommodations: { source: 'schemas/ExamAccommodationsUpdate/properties/accommodations', maxItems: 200 },
  regradeReason: { source: 'schemas/RegradeRequestCreate/properties/reason', minLength: 10, maxLength: 1000 },
} as const;

/** Enums mirrored from the spec (same check). */
export const LMS_ENUMS = {
  contentType: { source: 'schemas/AssignmentCreate/properties/content_ref/properties/content_type', values: ['activity', 'question_set', 'assessment_blueprint', 'lesson', 'quest', 'minigame_config'] },
  mode: { source: 'schemas/AssignmentCreate/properties/mode', values: ['practice', 'graded'] },
  scorePolicy: { source: 'schemas/AssignmentCreate/properties/score_policy', values: ['best', 'last', 'first'] },
  feedbackTiming: { source: 'schemas/FeedbackTiming', values: ['immediate', 'after_submit', 'after_close', 'never'] },
  resultRelease: { source: 'schemas/ResultRelease', values: ['immediate', 'after_close', 'manual'] },
  targetType: { source: 'schemas/AssignmentTarget/properties/target_type', values: ['class', 'study_group', 'students'] },
  assignmentStatus: { source: 'schemas/AssignmentStatus', values: ['draft', 'scheduled', 'open', 'closed', 'results_released', 'archived', 'cancelled'] },
  submissionStatus: { source: 'schemas/AssignmentSubmissionStatus', values: ['not_started', 'in_progress', 'submitted', 'late', 'missed', 'excused'] },
  courseStatus: { source: 'schemas/Course/properties/status', values: ['active', 'hidden', 'archived'] },
  unlockMode: { source: 'schemas/LearningPathInput/properties/unlock_mode', values: ['sequential', 'free', 'mastery_gate'] },
  pathContentType: { source: 'schemas/PathNode/properties/content_ref/properties/content_type', values: ['lesson', 'activity', 'question_set', 'assessment_blueprint', 'quest'] },
  unlockRule: { source: 'schemas/PathNode/properties/unlock_rule/properties/kind', values: ['always', 'previous_completed', 'mastery_gate', 'date'] },
  scheduleKind: { source: 'schemas/ScheduleItemInput/properties/kind', values: ['lesson', 'assignment_due', 'exam', 'event'] },
  regradeAction: { source: 'schemas/RegradeRequestCreate/properties/action', values: ['accept_all', 'use_new_key', 'drop_item'] },
} as const;

export type ContentType = (typeof LMS_ENUMS.contentType.values)[number];
export type TargetType = (typeof LMS_ENUMS.targetType.values)[number];
