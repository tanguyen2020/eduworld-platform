// Parameters of the Content Studio editors (AGENTS R33): values come from CONFIG_DEFAULTS rows whose scope is
// "code" (no runtime setting exists for them) or from contract `maxItems` / `maxLength`; named once here.
import { FIELDS } from './fieldSchemas';

/** CONFIG_DEFAULTS `content_autosave_interval` = 10 s (ADMIN_PORTAL §1.4, scope code). */
export const CONTENT_AUTOSAVE_INTERVAL_MS = 10_000;

/** Pause in typing that saves earlier than the interval [Proposed]. */
export const CONTENT_AUTOSAVE_DEBOUNCE_MS = 2_000;

/**
 * CONFIG_DEFAULTS `assets_per_version_max` = 50 (ADR-0044); the platform setting is not readable by content editors,
 * so the portal uses the same value as the contract cap `QuestionAuthoringInput.media_refs.maxItems` (server decides).
 */
export const ASSETS_PER_VERSION_MAX = 50;

/** Poll period of a media asset in the pipeline (quarantined / processing) [Proposed]. */
export const MEDIA_POLL_MS = 2_000;

/** Poll period of a translation export/import job [Proposed]. */
export const TRANSLATION_JOB_POLL_MS = 2_000;

/** ADMIN_PORTAL §1.5: editors need ≥ 1024 px; below, content is read only. */
export const EDITOR_MIN_WIDTH_QUERY = '(min-width: 1024px)';

/** Grades whose stem must be spoken (CONTENT_MODEL §8.1, `g1_3`, [Proposed] PED-07 / OQ-23). */
export const SPOKEN_STEM_GRADES: readonly number[] = [1, 2, 3];

/**
 * CONFIG_DEFAULTS `scheduled_publish_min_lead` = 5 min (CONTENT_MODEL, platform setting not readable by content
 * staff): earliest `publish_at` offered by the scheduler (F054-AC-04); the server checks again.
 */
export const SCHEDULED_PUBLISH_MIN_LEAD_MIN = 5;

/** CONFIG_DEFAULTS `publish_to_visible_max` ≤ 30 s (ADR-0043): "Đang chuẩn bị cache → Đã xuất bản" notice. */
export const PUBLISH_TO_VISIBLE_MAX_S = 30;

/** CONFIG_DEFAULTS `content_review_sla_days` = 3 (ADMIN_PORTAL §4): older open tasks show "Quá hạn" (F054-AC-12). */
export const CONTENT_REVIEW_SLA_DAYS = 3;

/** CONFIG_DEFAULTS `content_review_bulk_max` = 50 (ADMIN_PORTAL §4, scope code): bulk approve cap (F054-AC-14). */
export const CONTENT_REVIEW_BULK_MAX = 50;

/**
 * CONFIG_DEFAULTS `media_max_bytes_by_kind` (CONTENT_MODEL §7) and the accepted inputs of each kind: a pre-check
 * before presignMediaUpload (F054-AC-17); the media pipeline decides (magic bytes, ClamAV). Documents (import files)
 * have no registered cap: 20 MB [Proposed].
 */
export const MEDIA_RULES = {
  image: { maxBytes: 10 * 1024 * 1024, ext: ['png', 'jpg', 'jpeg', 'webp', 'svg'] },
  audio: { maxBytes: 20 * 1024 * 1024, ext: ['mp3', 'wav', 'ogg'] },
  video: { maxBytes: 300 * 1024 * 1024, ext: ['mp4', 'webm'] },
  model3d: { maxBytes: 50 * 1024 * 1024, ext: ['glb', 'pck'] },
  document: { maxBytes: 20 * 1024 * 1024, ext: ['csv', 'xlsx', 'json', 'yaml', 'yml', 'xml', 'zip'] },
} as const;

/** Rows of one question import (importer `MaxImportRows`, F024). */
export const IMPORT_MAX_ROWS = 5000;

/** OpenAPI limits used by the forms (ADR-0041); new limits go to fieldSchemas.ts (checked against the spec). */
export const LIMITS = {
  /** RollbackRequest.reason, ReasonRequest.reason, delete* reason. */
  releaseReason: 500,
  /** RestoreRequest.reason (minLength 3). */
  restoreReason: 1000,
  restoreReasonMin: 3,
  /** `q` of the lists and the media search. */
  search: 200,
  /** LearningName (vi / en). */
  name: 255,
  changeNote: 500,
  /** transitionContentVersion note (mirrored and checked in fieldSchemas.ts). */
  transitionNote: FIELDS.transitionNote.maxLength,
  audioExemptReason: 255,
  tag: 50,
  tags: 20,
  license: 255,
  altText: 500,
  topicName: 255,
  topicCode: 64,
  objectiveCode: 64,
  objectiveDescription: 1000,
  yccdRef: 64,
  prerequisites: 20,
  curriculumNote: 2000,
} as const;
