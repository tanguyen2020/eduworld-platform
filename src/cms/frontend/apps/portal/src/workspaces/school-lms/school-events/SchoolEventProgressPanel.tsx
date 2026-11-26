// "Tiến độ sự kiện" (F081 US3: F081-AC-10, AC-11; T022): number of participating students, the community goal as a bar
// with the milestones of the server (25/50/75/100 % …, reached or not), the contributions by grade sorted by grade
// (GradeContributionTable: no rank column, no highlighted leader — R19; grades already suppressed by the server below
// analytics_min_cell_size), "Cập nhật lúc" and a manual refresh (no polling). Aggregates only: no student name or id (R16).
// An event of several schools (or the whole tenant) asks for the school to show.
import { SearchSelect } from '@eduworld/ops-ui';
import { Button, Card, Flex, Progress, Statistic, Table, Tag, Typography } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ApiError } from '../../../shared/api/middleware/problem';
import { formatIct } from '../../../shared/release/ict';
import { ProblemMessage } from '../common/errors';
import { localizedName } from '../common/format';
import { useSchoolEventProgress, useTenantSchools, type SchoolEvent, type SchoolEventProgress } from './api';

type GradeRow = SchoolEventProgress['by_grade'][number];

export function GradeContributionTable({ rows }: { rows: GradeRow[] }) {
  const { t } = useTranslation();
  const columns: ColumnType<GradeRow>[] = [
    { key: 'grade', title: t('portal.school_lms.schoolEvents.progress.grade'), render: (_, r) => t('portal.school_lms.schoolEvents.form.grade', { grade: r.grade_level }) },
    { key: 'total', title: t('portal.school_lms.schoolEvents.progress.contributionTotal'), render: (_, r) => r.contribution_total },
    { key: 'contributors', title: t('portal.school_lms.schoolEvents.progress.contributors'), render: (_, r) => r.contributors_count },
  ];
  // the server sorts by grade; sorted again so the table never reads as a ranking
  const sorted = [...rows].sort((a, b) => a.grade_level - b.grade_level);
  return (
    <Table<GradeRow>
      rowKey="grade_level"
      size="small"
      columns={columns}
      dataSource={sorted}
      pagination={false}
      caption={t('portal.school_lms.schoolEvents.progress.byGrade')}
      locale={{ emptyText: t('portal.school_lms.schoolEvents.progress.noGrades') }}
    />
  );
}

export function SchoolEventProgressView({ data, onRefresh, refreshing }: { data: SchoolEventProgress; onRefresh: () => void; refreshing: boolean }) {
  const { t } = useTranslation();
  const goal = data.community_goal;
  const pct = goal ? Math.min(100, Math.floor((goal.total * 100) / Math.max(goal.target, 1))) : 0;
  return (
    <Flex vertical gap="middle" data-testid="event-progress">
      <Flex gap="large" wrap align="center">
        <Statistic title={t('portal.school_lms.schoolEvents.progress.participants')} value={data.participants_count} />
        <Flex vertical>
          <Typography.Text type="secondary">{t('portal.school_lms.schoolEvents.progress.asOf', { at: formatIct(data.data_as_of) })}</Typography.Text>
          <Typography.Text type="secondary">{t('portal.school_lms.schoolEvents.progress.delayHint')}</Typography.Text>
        </Flex>
        <Button onClick={onRefresh} loading={refreshing}>
          {t('portal.school_lms.schoolEvents.progress.refresh')}
        </Button>
      </Flex>
      {goal ? (
        <Flex vertical gap="small">
          <Typography.Text strong>
            {t('portal.school_lms.schoolEvents.progress.goal')} · {t(`portal.school_lms.schoolEvents.progress.metric.${goal.metric}`)}
          </Typography.Text>
          <Progress percent={pct} aria-label={t('portal.school_lms.schoolEvents.progress.goal')} />
          <Typography.Text>{t('portal.school_lms.schoolEvents.progress.total', { total: goal.total, target: goal.target })}</Typography.Text>
          <Flex gap="small" wrap data-testid="goal-milestones">
            {goal.milestones.map((m) => (
              <Tag key={m.pct} color={m.reached ? 'success' : 'default'} data-reached={m.reached}>
                {t('portal.school_lms.schoolEvents.progress.milestone', { pct: m.pct })} · {m.reached ? t('portal.school_lms.schoolEvents.progress.reached') : t('portal.school_lms.schoolEvents.progress.notReached')}
              </Tag>
            ))}
          </Flex>
          <GradeContributionTable rows={data.by_grade} />
          <Typography.Text type="secondary">{t('portal.school_lms.schoolEvents.progress.gradeNote')}</Typography.Text>
        </Flex>
      ) : (
        <Typography.Text type="secondary">{t('portal.school_lms.schoolEvents.progress.noGoal')}</Typography.Text>
      )}
    </Flex>
  );
}

export function SchoolEventProgressPanel({ event, tenantHeaders, tenantKey }: { event: SchoolEvent; tenantHeaders: Record<string, string>; tenantKey: string }) {
  const { t, i18n } = useTranslation();
  const audience = event.audience?.school_ids ?? [];
  const several = audience.length !== 1;
  const [school, setSchool] = useState<string | undefined>();
  const schools = useTenantSchools(tenantHeaders, tenantKey, several);
  const options = schools.items.filter((s) => !audience.length || audience.includes(s.school_id));
  const progress = useSchoolEventProgress(event.event_id, school, event.status !== 'draft');
  const needsSchool = progress.error instanceof ApiError && progress.error.status === 422;
  return (
    <Card title={t('portal.school_lms.schoolEvents.progress.title')}>
      <Flex vertical gap="middle">
        {several ? (
          <SearchSelect<string>
            aria-label={t('portal.school_lms.schoolEvents.progress.school')}
            placeholder={t('portal.school_lms.schoolEvents.progress.pickSchool')}
            value={school}
            onChange={(v?: string) => setSchool(v)}
            loading={schools.isFetching}
            optionFilterProp="label"
            options={options.map((s) => ({ value: s.school_id, label: localizedName(s.name, i18n.language) }))}
            style={{ minWidth: '16rem' }}
          />
        ) : null}
        {needsSchool ? <Typography.Text type="secondary">{t('portal.school_lms.schoolEvents.progress.pickSchool')}</Typography.Text> : progress.error ? <ProblemMessage error={progress.error} /> : null}
        {progress.data ? <SchoolEventProgressView data={progress.data} onRefresh={() => void progress.refetch()} refreshing={progress.isFetching} /> : null}
      </Flex>
    </Card>
  );
}
