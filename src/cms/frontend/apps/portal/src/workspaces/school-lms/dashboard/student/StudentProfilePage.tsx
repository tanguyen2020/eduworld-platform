// Student profile (F056 T028; F056-AC-11; SCHOOL_LMS §8.5): getStudentLearningProfile (audited by the server) — subject
// overview, mastery by topic (MasteryHeatmap), 8-week trend, assignment history (no answers), recommendations shown to
// the student — and the tab "Hoạt động đăng nhập" from listTenantStudentLoginEvents (F052 panel, needs
// lms.student_activity.read). Never shown: Game Rating, position in the game, friends, chat, IP, device id (none of
// them is in the contract; the login panel's view model drops unknown fields). Out of scope → "Không tìm thấy". The name
// comes with the link from the overview (router state, never in the URL); there is no class level (ADR-0095).
import { DashboardChart, DashboardMetric, DashboardWidget, MASTERY_LEVELS, MasteryHeatmap, type HeatmapRow } from '@eduworld/ops-ui/dashboard';
import { Breadcrumb, Col, Descriptions, Flex, List, Row, Table, Tabs, Tag, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { Link, useLocation, useParams } from 'react-router-dom';
import { useActionEnabled } from '../../lms/LmsUi';
import { formatDate, formatDateTime } from '../../common/format';
import { StudentLoginActivityPanel } from '../../students/StudentLoginActivityPanel';
import { useStudentProfile, useSubjects, type StudentLearningProfile } from '../api/dashboardApi';
import { DashboardPageError, DASHBOARD_BASE, useWidgetError } from '../DashboardUi';
import { useDashboardFilters } from '../DashboardFilterBar';
import { filtersToSearch, subjectLabel } from '../model/filters';

type Attempt = StudentLearningProfile['recent_attempts'][number];

export function StudentProfilePage() {
  const { t, i18n } = useTranslation();
  const { studentId } = useParams();
  const linked = (useLocation().state as { name?: string } | null)?.name;
  const { filters } = useDashboardFilters();
  const q = useStudentProfile(studentId, filters);
  const error = useWidgetError(q.error);
  const subjects = useSubjects();
  const p = q.data;
  if (q.error && !error?.code.startsWith('DASHBOARD_')) return <DashboardPageError error={q.error} />;
  const name = linked || t('dashboard.student.unnamed');
  const subjectName = (id?: string) => {
    const s = subjects.data?.find((x) => x.subject_id === id);
    return s ? subjectLabel(s, i18n.language) : '';
  };
  const pct = (v: number | null | undefined) => (v === null || v === undefined ? '' : new Intl.NumberFormat(i18n.language, { style: 'percent', maximumFractionDigits: 0 }).format(v));
  const levelLabel = (l: (typeof MASTERY_LEVELS)[number]) => t(`dashboard.mastery.${l}`);

  const masteryTab = (
    <Flex vertical gap="middle">
      {(p?.subjects ?? []).map((s) => {
        const rows: HeatmapRow[] = (s.mastery_by_topic ?? []).map((tp) => {
          const topic = tp.topic_name ?? '';
          return {
            key: tp.topic_id,
            header: topic,
            cells: MASTERY_LEVELS.map((level, i) => {
              const n = tp.level_dist[i] ?? 0;
              return { level, text: String(n), muted: n === 0, description: t('dashboard.heatmap.topicCell', { topic, level: levelLabel(level), count: n }) };
            }),
          };
        });
        return (
          <Flex key={s.subject_id} vertical gap="small" data-testid="profile-subject">
            <Descriptions
              size="small"
              column={{ xs: 1, md: 4 }}
              title={subjectName(s.subject_id)}
              items={[
                { key: 'accuracy', label: t('dashboard.student.accuracy'), children: pct(s.accuracy) },
                { key: 'minutes', label: t('dashboard.student.minutes'), children: s.active_minutes ?? '' },
                { key: 'activities', label: t('dashboard.student.activities'), children: s.activities_completed ?? '' },
                { key: 'lessons', label: t('dashboard.student.lessons'), children: s.lessons_completed ?? '' },
              ]}
            />
            {rows.length ? (
              <MasteryHeatmap caption={t('dashboard.profile.masteryCaption', { subject: subjectName(s.subject_id) })} rowHeader={t('dashboard.profile.topic')} columns={MASTERY_LEVELS.map((l) => ({ key: l, label: levelLabel(l) }))} rows={rows} />
            ) : null}
          </Flex>
        );
      })}
    </Flex>
  );

  const trendBySubject = Object.entries(
    (p?.trend_weeks ?? []).reduce<Record<string, NonNullable<StudentLearningProfile['trend_weeks']>>>((acc, w) => {
      (acc[w.subject_id] ??= []).push(w);
      return acc;
    }, {}),
  );
  const trendTab = (
    <Flex vertical gap="middle">
      {trendBySubject.map(([subjectId, weeks]) => (
        <Flex key={subjectId} vertical gap="small">
          <Typography.Text strong>{subjectName(subjectId)}</Typography.Text>
          <DashboardChart
            kind="line"
            categories={weeks.map((w) => formatDate(w.week_start))}
            values={weeks.map((w) => w.mastered_count ?? null)}
            ariaLabel={t('dashboard.profile.trendAria', { subject: subjectName(subjectId), values: weeks.map((w) => `${formatDate(w.week_start)}: ${w.mastered_count ?? '—'}`).join('; ') })}
          />
        </Flex>
      ))}
      {trendBySubject.length === 0 ? <Typography.Text type="secondary">{t('dashboard.metric.noData')}</Typography.Text> : null}
    </Flex>
  );

  const historyTab = (
    <Table<Attempt>
      size="small"
      rowKey="assignment_id"
      pagination={false}
      dataSource={p?.recent_attempts ?? []}
      locale={{ emptyText: t('dashboard.metric.noData') }}
      columns={[
        { key: 'title', title: t('dashboard.profile.assignment'), render: (_, a) => a.title ?? '' },
        { key: 'subject', title: t('dashboard.filter.subject'), render: (_, a) => subjectName(a.subject_id) },
        { key: 'status', title: t('dashboard.profile.status'), render: (_, a) => <Tag>{t(`dashboard.submission.${a.status}`)}</Tag> },
        { key: 'score', title: t('dashboard.profile.score'), render: (_, a) => (a.score === null || a.score === undefined ? '' : t('dashboard.profile.scoreOf', { score: a.score, max: a.max_score ?? '' })) },
        { key: 'due', title: t('dashboard.profile.due'), render: (_, a) => formatDateTime(a.due_at) },
        { key: 'submitted', title: t('dashboard.profile.submittedAt'), render: (_, a) => formatDateTime(a.submitted_at) },
      ]}
    />
  );

  const recommendationsTab = (
    <List
      size="small"
      dataSource={p?.active_recommendations ?? []}
      locale={{ emptyText: t('dashboard.profile.noRecommendation') }}
      renderItem={(r) => (
        <List.Item key={r.recommendation_id}>
          <Flex gap="small" wrap align="center">
            <Tag>{t(`dashboard.reco.${r.rule_code}`)}</Tag>
            <Typography.Text>{subjectName(r.subject_id)}</Typography.Text>
          </Flex>
        </List.Item>
      )}
    />
  );

  const tabs = [
    { key: 'mastery', label: t('dashboard.profile.tab.mastery'), children: masteryTab },
    { key: 'trend', label: t('dashboard.profile.tab.trend'), children: trendTab },
    { key: 'history', label: t('dashboard.profile.tab.history'), children: historyTab },
    { key: 'recommendations', label: t('dashboard.profile.tab.recommendations'), children: recommendationsTab },
  ];
  return (
    <Flex vertical gap="middle" data-testid="dashboard-student">
      <Breadcrumb
        items={[
          { title: <Link to={`${DASHBOARD_BASE}?${filtersToSearch(filters).toString()}`}>{t('dashboard.breadcrumb.school')}</Link> },
          ...(filters.grade ? [{ title: t('dashboard.filter.gradeN', { n: filters.grade }) }] : []),
          { title: name },
        ]}
      />
      <DashboardWidget testId="widget-profile" title={t('dashboard.profile.title', { name })} meta={p} loading={q.isPending && !!studentId} error={error}>
        {p ? (
          <Flex vertical gap="small">
            <Row gutter={[16, 8]}>
              {p.subjects.map((s) => (
                <Col key={s.subject_id} xs={12} md={6}>
                  <DashboardMetric label={t('dashboard.profile.subjectAccuracy', { subject: subjectName(s.subject_id) })} value={s.accuracy} format="percent" partial={p.is_partial} />
                </Col>
              ))}
            </Row>
            <Typography.Text type="secondary">{t('dashboard.profile.period', { from: formatDate(p.from ?? filters.range.from), to: formatDate(p.to ?? filters.range.to) })}</Typography.Text>
            <ProfileTabs tabs={tabs} studentId={studentId!} />
          </Flex>
        ) : null}
      </DashboardWidget>
    </Flex>
  );
}

function ProfileTabs({ tabs, studentId }: { tabs: { key: string; label: string; children: JSX.Element }[]; studentId: string }) {
  const { t } = useTranslation();
  // the login tab follows the catalog action (lms.student_activity.read, can be turned off — ADR-0090)
  const canSeeLogins = useActionEnabled('action.school_lms.student_activity.view');
  const login = { key: 'login', label: t('dashboard.profile.tab.login'), children: <StudentLoginActivityPanel studentId={studentId} /> };
  return <Tabs items={canSeeLogins ? [...tabs, login] : tabs} />;
}
