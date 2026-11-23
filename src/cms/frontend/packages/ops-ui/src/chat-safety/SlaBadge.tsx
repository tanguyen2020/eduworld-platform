// SlaBadge (F122 T007, F122-AC-01, F122-AC-18): deadline of a case or a school notification computed by the server
// (`sla_due_at`, `ack_due_at`). Overdue is shown with the error status token AND the word "Quá hạn" — color is never the
// only channel. No rule here: the badge only compares the server deadline with the clock it is given.
import { Tag } from 'antd';
import dayjs from 'dayjs';
import { useTranslation } from 'react-i18next';
import { OPS_UI_NS } from '../i18n';

export interface SlaBadgeProps {
  /** Deadline from the API (ISO-8601); nothing is shown without one. */
  dueAt?: string | null;
  /** Done (acknowledged, resolved): no deadline state any more. */
  done?: boolean;
  /** Current time (tests, stories); default now. */
  now?: Date;
}

export function SlaBadge({ dueAt, done, now }: SlaBadgeProps) {
  const { t } = useTranslation(OPS_UI_NS);
  if (!dueAt) return null;
  const due = dayjs(dueAt);
  const label = due.format('DD/MM/YYYY HH:mm');
  if (done) return <Tag data-sla="done">{t('chatSafety.sla.done', { at: label })}</Tag>;
  const overdue = !due.isAfter(dayjs(now ?? new Date()));
  return (
    <Tag color={overdue ? 'error' : 'processing'} data-sla={overdue ? 'overdue' : 'pending'}>
      {overdue ? t('chatSafety.sla.overdue', { at: label }) : t('chatSafety.sla.due', { at: label })}
    </Tag>
  );
}
