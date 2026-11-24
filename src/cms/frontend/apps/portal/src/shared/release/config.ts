// Parameters of the versioned editors shared by Content Studio and the Game Operations Console (AGENTS R33): values of
// CONFIG_DEFAULTS rows whose scope is "code" (no runtime setting is readable by content staff), named once here.

/** CONFIG_DEFAULTS `content_autosave_interval` = 10 s (ADMIN_PORTAL §1.4, scope code). */
export const CONTENT_AUTOSAVE_INTERVAL_MS = 10_000;

/** Pause in typing that saves earlier than the interval [Proposed]. */
export const CONTENT_AUTOSAVE_DEBOUNCE_MS = 2_000;

/** ADMIN_PORTAL §1.5: editors need ≥ 1024 px; below, content is read only. */
export const EDITOR_MIN_WIDTH_QUERY = '(min-width: 1024px)';

/**
 * CONFIG_DEFAULTS `scheduled_publish_min_lead` = 5 min (CONTENT_MODEL, platform setting not readable by content
 * staff): earliest `publish_at` offered by the scheduler (F054-AC-04, F079-AC-03); the server checks again.
 */
export const SCHEDULED_PUBLISH_MIN_LEAD_MIN = 5;
