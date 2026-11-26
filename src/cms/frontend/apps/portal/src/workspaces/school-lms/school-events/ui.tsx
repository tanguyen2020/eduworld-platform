// Presentational pieces of "Sự kiện trường" (F081): status tag (antd status presets = theme tokens, no hardcoded color —
// R32), ICT date helpers (shown and picked in Vietnam time, sent in UTC — ADR-0056), the error codes the screen maps to
// its own messages (F081-AC-04, AC-07, AC-09, AC-20). No data logic.
import { Tag } from 'antd';
import dayjs, { type Dayjs } from 'dayjs';
import { useTranslation } from 'react-i18next';
import { ApiError } from '../../../shared/api/middleware/problem';
import { ictToUtc, utcToIct } from '../../../shared/release/ict';
import type { SchoolEventStatus } from './api';

const STATUS_COLOR: Record<SchoolEventStatus, string> = {
  draft: 'default',
  scheduled: 'processing',
  live: 'success',
  suspended: 'warning',
  ended: 'default',
  cancelled: 'error',
  archived: 'default',
};

export function EventStatusTag({ status }: { status: SchoolEventStatus }) {
  const { t } = useTranslation();
  return (
    <Tag color={STATUS_COLOR[status]} data-event-status={status}>
      {t(`portal.school_lms.schoolEvents.status.${status}`)}
    </Tag>
  );
}

/** Editable only before the run starts (F081-AC-07): the server answers 409 anyway. */
export const isEditable = (status: SchoolEventStatus) => status === 'scheduled';
/** Cancelable while scheduled or live; never while suspended by EduWorld (edge case) — the server decides. */
export const isCancelable = (status: SchoolEventStatus) => status === 'scheduled' || status === 'live';

/** A UTC ISO instant → a dayjs holding its ICT wall time (value of the DatePicker). */
export function ictDayjs(iso: string | undefined | null): Dayjs | null {
  if (!iso) return null;
  const w = utcToIct(new Date(iso));
  return dayjs(new Date(w.year, w.month - 1, w.day, w.hour, w.minute));
}

/** The ICT wall time picked → the UTC ISO instant sent to the API. */
export const ictIso = (d: Dayjs) => ictToUtc({ year: d.year(), month: d.month() + 1, day: d.date(), hour: d.hour(), minute: d.minute() }).toISOString();

/** Whole days between two picked times (shown next to the template maximum; the server checks). */
export const durationDays = (from: Dayjs | null | undefined, to: Dayjs | null | undefined) =>
  from && to ? Math.ceil(to.diff(from, 'minute') / (24 * 60)) : 0;

/** 404 NOT_FOUND or 403 TENANT_MISMATCH on one event: "Không tìm thấy", nothing about the event (F081-AC-09). */
export const isHidden = (e: unknown) => e instanceof ApiError && (e.status === 404 || (e.status === 403 && e.code === 'TENANT_MISMATCH'));
/** 412 VERSION_MISMATCH or 409 CONFLICT on update: the data changed, reload (F081-AC-07). */
export const isStale = (e: unknown) => e instanceof ApiError && (e.status === 412 || e.status === 409);
