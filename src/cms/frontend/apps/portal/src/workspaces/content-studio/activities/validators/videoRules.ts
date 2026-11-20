// videoRules (F058 T042, F058-AC-15…AC-20, VIDEO_LEARNING §3–§7, pure): early checks of a video_lesson config in the
// portal — chapters inside the video without overlap, unique cues inside [0, duration], subtitles with exactly one
// default (and `vi` when the video has speech), branches with 2–4 options targeting existing segments and an ending.
// The same rules run in module learning (videolesson.Config.Validate); the server decides.
export interface Chapter {
  chapter_id: string;
  title?: { vi?: string; en?: string };
  start_ms: number;
  end_ms: number;
}
export interface Cue {
  cue_id: string;
  at_ms: number;
  question_version_id?: string;
  required?: boolean;
  allow_skip_after_wrong?: boolean;
}
export interface Subtitle {
  language: string;
  subtitle_asset_id?: string;
  is_default?: boolean;
}
export interface Segment {
  segment_id: string;
  video_asset_id?: string;
  is_ending?: boolean;
}
export interface Branch {
  branch_point_id: string;
  at_ms: number;
  prompt?: { vi?: string; en?: string };
  options: { option_id: string; label?: { vi?: string; en?: string }; target_segment_id: string }[];
}
export interface VideoContent {
  video_asset_id?: string;
  duration_ms?: number;
  chapters?: Chapter[];
  subtitles?: Subtitle[];
  embedded_questions?: Cue[];
  segments?: Segment[];
  branches?: Branch[];
}

export interface VideoIssue {
  pointer: string;
  key: string;
  values?: Record<string, unknown>;
}

const LANGUAGE = /^[a-z]{2}(-[A-Z]{2})?$/;

/** Issues of a video config; `speech` = the form flag "video có lời" (vi subtitle required, ADMIN_PORTAL §3.13). */
export function videoIssues(c: VideoContent, opts: { speech: boolean }): VideoIssue[] {
  const out: VideoIssue[] = [];
  const add = (pointer: string, key: string, values?: Record<string, unknown>) => out.push({ pointer: `/config/content/${pointer}`, key, values });
  const d = c.duration_ms ?? 0;
  if (!c.video_asset_id) add('video_asset_id', 'video.noVideo');
  const chapters = [...(c.chapters ?? [])];
  chapters.forEach((ch, i) => {
    if (!(ch.start_ms < ch.end_ms)) add(`chapters/${i}/end_ms`, 'video.chapterOrder', { n: i + 1 });
    else if (d > 0 && ch.end_ms > d) add(`chapters/${i}/end_ms`, 'video.afterEnd', { n: i + 1 });
  });
  const sorted = chapters.map((ch, i) => ({ ...ch, i })).sort((a, b) => a.start_ms - b.start_ms);
  for (let k = 1; k < sorted.length; k++) {
    if (sorted[k]!.start_ms < sorted[k - 1]!.end_ms) add(`chapters/${sorted[k]!.i}/start_ms`, 'video.chapterOverlap', { a: sorted[k - 1]!.i + 1, b: sorted[k]!.i + 1 });
  }
  const cueIds = new Set<string>();
  (c.embedded_questions ?? []).forEach((q, i) => {
    if (cueIds.has(q.cue_id)) add(`embedded_questions/${i}/cue_id`, 'video.cueDuplicate', { id: q.cue_id });
    cueIds.add(q.cue_id);
    if (q.at_ms < 0 || (d > 0 && q.at_ms >= d)) add(`embedded_questions/${i}/at_ms`, 'video.cueOutside', { id: q.cue_id });
    if (!q.question_version_id) add(`embedded_questions/${i}/question_version_id`, 'video.cueQuestion_missing', { id: q.cue_id });
  });
  const subs = c.subtitles ?? [];
  subs.forEach((s, i) => {
    if (!LANGUAGE.test(s.language)) add(`subtitles/${i}/language`, 'video.subtitleLanguage', { lang: s.language });
    if (!s.subtitle_asset_id) add(`subtitles/${i}/subtitle_asset_id`, 'video.subtitleFile', { lang: s.language });
  });
  if (subs.length > 0 && subs.filter((s) => s.is_default).length !== 1) add('subtitles', 'video.subtitleDefaultIssue');
  if (opts.speech && !subs.some((s) => s.language === 'vi')) add('subtitles', 'video.subtitleViRequired');
  const segIds = new Set((c.segments ?? []).map((s) => s.segment_id));
  (c.branches ?? []).forEach((b, i) => {
    if (b.options.length < 2 || b.options.length > 4) add(`branches/${i}/options`, 'video.branchOptions', { n: i + 1 });
    b.options.forEach((o, j) => {
      if (!segIds.has(o.target_segment_id)) add(`branches/${i}/options/${j}/target_segment_id`, 'video.branchTarget', { n: i + 1 });
    });
    if (d > 0 && b.at_ms >= d) add(`branches/${i}/at_ms`, 'video.afterEnd', { n: i + 1 });
  });
  (c.segments ?? []).forEach((s, i) => {
    if (!s.video_asset_id) add(`segments/${i}/video_asset_id`, 'video.segmentVideo', { id: s.segment_id });
  });
  if ((c.branches ?? []).length > 0 && !(c.segments ?? []).some((s) => s.is_ending)) add('segments', 'video.noEnding');
  return out;
}

/** video_recommended_length (CONFIG_DEFAULTS, VIDEO_LEARNING §3): ≤ 8 minutes for g1_5, ≤ 15 for g6_9 (warning only). */
export const VIDEO_RECOMMENDED_MINUTES = { g1_5: 8, g6_9: 15 } as const;

/** Recommended length in minutes for the grades of the activity (the highest grade decides), null without grade. */
export function recommendedMinutes(grades: number[]): number | null {
  if (grades.length === 0) return null;
  return Math.max(...grades) <= 5 ? VIDEO_RECOMMENDED_MINUTES.g1_5 : VIDEO_RECOMMENDED_MINUTES.g6_9;
}
