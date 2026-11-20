// audioRules (F066 T045, F066-AC-25, AUDIO_LANGUAGE_LEARNING §2, pure): early checks of an audio_lesson config in
// the portal — segments with start < end inside the audio and without overlap, question times before the end, a
// transcript before Gửi duyệt (required to publish). The same rules run in module learning (audiolesson.Config).
export interface AudioSegment {
  segment_id: string;
  title?: { vi?: string; en?: string };
  start_ms: number;
  end_ms: number;
}
export interface AudioQuestion {
  question_version_id: string;
  at_ms?: number;
}
export interface AudioContent {
  audio_asset_id?: string;
  transcript_asset_id?: string;
  segments?: AudioSegment[];
  questions?: AudioQuestion[];
}
export interface AudioIssue {
  pointer: string;
  key: string;
  values?: Record<string, unknown>;
}

/** maxItems of audio-lesson.schema.json. */
export const MAX_SEGMENTS = 50;
export const MAX_QUESTIONS = 30;

export function audioIssues(c: AudioContent, durationMs: number | undefined): AudioIssue[] {
  const out: AudioIssue[] = [];
  const add = (pointer: string, key: string, values?: Record<string, unknown>) => out.push({ pointer: `/config/content/${pointer}`, key, values });
  const d = durationMs ?? 0;
  if (!c.audio_asset_id) add('audio_asset_id', 'audio.noAudio');
  if (!c.transcript_asset_id) add('transcript_asset_id', 'audio.noTranscript');
  const segs = c.segments ?? [];
  const ids = new Set<string>();
  segs.forEach((s, i) => {
    if (!(s.start_ms < s.end_ms)) add(`segments/${i}/end_ms`, 'audio.segmentOrder', { n: i + 1 });
    else if (d > 0 && s.end_ms > d) add(`segments/${i}/end_ms`, 'audio.afterEnd', { n: i + 1 });
    if (ids.has(s.segment_id)) add(`segments/${i}/segment_id`, 'audio.segmentDuplicate', { id: s.segment_id });
    ids.add(s.segment_id);
  });
  const sorted = segs.map((s, i) => ({ ...s, i })).sort((a, b) => a.start_ms - b.start_ms);
  for (let k = 1; k < sorted.length; k++) {
    if (sorted[k]!.start_ms < sorted[k - 1]!.end_ms) add(`segments/${sorted[k]!.i}/start_ms`, 'audio.segmentOverlap', { a: sorted[k - 1]!.i + 1, b: sorted[k]!.i + 1 });
  }
  (c.questions ?? []).forEach((q, i) => {
    if (q.at_ms !== undefined && (q.at_ms < 0 || (d > 0 && q.at_ms >= d))) add(`questions/${i}/at_ms`, 'audio.questionOutside', { n: i + 1 });
  });
  return out;
}
