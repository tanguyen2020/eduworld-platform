// Curfew windows (F052 T057, F052-AC-29, F052-AC-30, CHILD_SAFETY §4.1, ADR-0078) — pure helpers for the early
// warning only: the server decides ("chỉ siết thêm" → 422 VALIDATION_FAILED) and returns `effective_curfew`.
// A window `start–end` in ICT may cross midnight (21:30–06:30).
export interface CurfewWindow {
  start: string;
  end: string;
}

export const HHMM = /^([01][0-9]|2[0-3]):[0-5][0-9]$/;
const DAY = 24 * 60;

const minutes = (hhmm: string) => {
  const [h, m] = hhmm.split(':').map(Number);
  return h! * 60 + m!;
};

/** Length in minutes of a window (crossing midnight allowed); start = end is an empty window. */
export function windowLength(w: CurfewWindow): number {
  return (minutes(w.end) - minutes(w.start) + DAY) % DAY;
}

/** True when the school window contains the whole platform window (both may cross midnight). */
export function curfewCovers(school: CurfewWindow, platform: CurfewWindow): boolean {
  if (!HHMM.test(school.start) || !HHMM.test(school.end) || !HHMM.test(platform.start) || !HHMM.test(platform.end)) return false;
  const offset = (minutes(platform.start) - minutes(school.start) + DAY) % DAY;
  return offset + windowLength(platform) <= windowLength(school);
}

/** True when the window ends the next day (display "đến … hôm sau"). */
export function crossesMidnight(w: CurfewWindow): boolean {
  return HHMM.test(w.start) && HHMM.test(w.end) && minutes(w.end) <= minutes(w.start);
}
