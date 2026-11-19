// Parameters of the Content Studio editors (AGENTS R33): values come from CONFIG_DEFAULTS rows whose scope is
// "code" (no runtime setting exists for them) or from contract `maxItems` / `maxLength`; named once here.

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

/** OpenAPI limits used by the forms (ADR-0041). */
export const LIMITS = {
  changeNote: 500,
  transitionNote: 2000,
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
