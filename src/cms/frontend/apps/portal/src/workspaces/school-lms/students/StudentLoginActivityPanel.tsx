// Game login activity of a student (F052 T019, F052-AC-06, F052-AC-07, FR-GAME-013): enter/leave time, platform,
// shortened device label, end reason, days without the game. Never an IP, `ip_hash` or raw device id (the view model
// drops them). Student outside the scope (404/403) → "Không tìm thấy".
import { Space, Statistic, Typography } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import { QueryError } from '../common/errors';
import { formatDateTime } from '../common/format';
import { ResponsiveTable } from '../common/ResponsiveTable';
import { daysSinceLastLogin, type LoginEventView } from './loginEventView';
import { useLoginEvents } from './useLoginEvents';

export interface StudentLoginTableProps {
  events: LoginEventView[];
  loading?: boolean;
  hasMore?: boolean;
  onLoadMore?: () => void;
}

/** Login history table (whitelisted view model only). */
export function StudentLoginTable({ events, loading, hasMore, onLoadMore }: StudentLoginTableProps) {
  const { t } = useTranslation();
  const columns: ColumnType<LoginEventView>[] = [
    { key: 'title', title: t('portal.school_lms.activity.loginAt'), render: (_, e) => formatDateTime(e.loginAt) },
    { key: 'logout', title: t('portal.school_lms.activity.logoutAt'), render: (_, e) => formatDateTime(e.logoutAt) },
    { key: 'platform', title: t('portal.school_lms.activity.platform'), render: (_, e) => t(`portal.school_lms.activity.platforms.${e.platform}`) },
    { key: 'device', title: t('portal.school_lms.activity.device'), render: (_, e) => e.deviceLabel ?? '' },
    { key: 'version', title: t('portal.school_lms.activity.clientVersion'), render: (_, e) => e.clientVersion ?? '' },
    { key: 'end', title: t('portal.school_lms.activity.endReason'), render: (_, e) => (e.endReason ? t(`portal.school_lms.activity.endReasons.${e.endReason}`) : '') },
  ];
  return <ResponsiveTable<LoginEventView> rowKey="id" columns={columns} data={events} loading={loading} cardTitle={(e) => formatDateTime(e.loginAt)} hasMore={hasMore} onLoadMore={onLoadMore} />;
}

export function StudentLoginActivityPanel({ studentId }: { studentId: string }) {
  const { t } = useTranslation();
  const events = useLoginEvents(studentId);
  if (events.error) return <QueryError error={events.error} resource />;
  const days = daysSinceLastLogin(events.items);
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }} data-testid="login-activity">
      {!events.isLoading ? (
        <Statistic
          title={t('portal.school_lms.activity.daysAway')}
          value={days === null ? t('portal.school_lms.students.never') : t('portal.school_lms.students.days', { count: days })}
        />
      ) : null}
      <StudentLoginTable events={events.items} loading={events.isFetching} hasMore={events.hasMore} onLoadMore={events.loadMore} />
      {!events.isFetching && events.items.length === 0 ? <Typography.Text type="secondary">{t('portal.school_lms.activity.none')}</Typography.Text> : null}
    </Space>
  );
}
