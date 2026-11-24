// ICT time helpers of the release screens (F054-AC-04, F079-AC-03, AC-14; shared by Content Studio and the Game
// Operations Console): Vietnam time (UTC+7, no daylight saving) — times are shown and picked in ICT, sent in UTC.

/** Vietnam time offset (minutes). */
export const ICT_OFFSET_MINUTES = 7 * 60;

export interface WallTime {
  year: number;
  /** 1–12 */
  month: number;
  day: number;
  hour: number;
  minute: number;
}

/** An ICT wall-clock time → the UTC instant. */
export function ictToUtc(w: WallTime): Date {
  return new Date(Date.UTC(w.year, w.month - 1, w.day, w.hour, w.minute) - ICT_OFFSET_MINUTES * 60_000);
}

/** A UTC instant → its ICT wall-clock time. */
export function utcToIct(d: Date): WallTime {
  const x = new Date(d.getTime() + ICT_OFFSET_MINUTES * 60_000);
  return { year: x.getUTCFullYear(), month: x.getUTCMonth() + 1, day: x.getUTCDate(), hour: x.getUTCHours(), minute: x.getUTCMinutes() };
}

const pad = (n: number) => String(n).padStart(2, '0');

/** `dd/MM/yyyy HH:mm` in ICT. */
export function formatIct(iso: string | Date | undefined): string {
  if (!iso) return '';
  const d = typeof iso === 'string' ? new Date(iso) : iso;
  if (Number.isNaN(d.getTime())) return '';
  const w = utcToIct(d);
  return `${pad(w.day)}/${pad(w.month)}/${w.year} ${pad(w.hour)}:${pad(w.minute)}`;
}

/** `yyyy-MM-dd` of the ICT day of an instant. */
export function ictDate(d: Date): string {
  const w = utcToIct(d);
  return `${w.year}-${pad(w.month)}-${pad(w.day)}`;
}

/** Earliest allowed `publish_at` = now + `scheduled_publish_min_lead`. */
export function earliestPublishAt(now: Date, minLeadMinutes: number): Date {
  return new Date(now.getTime() + minLeadMinutes * 60_000);
}

export function isScheduleAllowed(at: Date, now: Date, minLeadMinutes: number): boolean {
  return at.getTime() >= earliestPublishAt(now, minLeadMinutes).getTime();
}
