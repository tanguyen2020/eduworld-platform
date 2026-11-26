// List of `menu.school_lms.challenges` (F108 T028; F108-AC-15, AC-16): challenges of the tenant (listChallenges, cursor
// pages with "Xem thêm"), filtered by grade (ADR-0095 — no class) and status; template names come from the assigned
// templates (listChallengeTemplates, whole list). A System Admin without one picked tenant sees the notice and a
// disabled create button, and no challenge API is called. Display only (AGENTS R1).
import { Alert, Button, Flex, Form } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useNavigate } from 'react-router-dom';
import { ActionGate } from '../../../../permissions/ActionGate';
import { formatDateTime } from '../../common/format';
import { ResponsiveTable } from '../../common/ResponsiveTable';
import { useSchoolLmsScope } from '../../common/scope';
import { useChallenges, useChallengeTemplates, type ChallengeFilters } from '../api/challenges';
import { ChallengeProblem, EnumTag } from '../ChallengeUi';
import { CHALLENGE_ENUMS, CHALLENGE_LIMITS, gradeOf, type Challenge, type ChallengeStatus } from '../model/challengeForm';
import { SearchSelect } from '@eduworld/ops-ui';

const GRADES = Array.from({ length: CHALLENGE_LIMITS.gradeLevel.maximum - CHALLENGE_LIMITS.gradeLevel.minimum + 1 }, (_, i) => CHALLENGE_LIMITS.gradeLevel.minimum + i);

function ChallengeTable() {
  const { t } = useTranslation();
  const [filters, setFilters] = useState<ChallengeFilters>({});
  const list = useChallenges(filters);
  const templates = useChallengeTemplates();
  const templateName = new Map(templates.items.map((x) => [x.template_id, x.name]));
  const grade = (c: Challenge) => {
    const g = gradeOf(c);
    return g ? t('lms.challenges.gradeN', { n: g }) : '';
  };
  const columns: ColumnType<Challenge>[] = [
    { key: 'name', title: t('lms.challenges.list.name'), render: (_, c) => <Link to={c.challenge_id}>{c.name}</Link> },
    { key: 'template', title: t('lms.challenges.list.template'), render: (_, c) => templateName.get(c.template_id) ?? '' },
    { key: 'kind', title: t('lms.challenges.list.kind'), render: (_, c) => <EnumTag group="kind" value={c.kind} /> },
    { key: 'grade', title: t('lms.challenges.list.grade'), render: (_, c) => grade(c) },
    { key: 'status', title: t('lms.challenges.list.status'), render: (_, c) => <EnumTag group="status" value={c.status} /> },
    {
      key: 'schedule',
      title: t('lms.challenges.list.schedule'),
      render: (_, c) => (c.ends_at ? t('lms.challenges.schedule', { from: formatDateTime(c.starts_at), to: formatDateTime(c.ends_at) }) : formatDateTime(c.starts_at)),
    },
    { key: 'audience', title: t('lms.challenges.list.audience'), render: (_, c) => c.audience_count ?? '' },
  ];
  return (
    <Flex vertical gap="middle">
      <Flex wrap="wrap" gap="small" align="end">
        <Form.Item label={t('lms.challenges.list.grade')} style={{ marginBottom: 0 }}>
          <SearchSelect
            allowClear
            aria-label={t('lms.challenges.list.grade')}
            placeholder={t('lms.challenges.list.grade')}
            value={filters.gradeLevel}
            onChange={(gradeLevel?: number) => setFilters({ ...filters, gradeLevel })}
            options={GRADES.map((n) => ({ value: n, label: t('lms.challenges.gradeN', { n }) }))}
            popupMatchSelectWidth={false}
          />
        </Form.Item>
        <Form.Item label={t('lms.challenges.list.status')} style={{ marginBottom: 0 }}>
          <SearchSelect
            allowClear
            aria-label={t('lms.challenges.list.status')}
            placeholder={t('lms.challenges.list.status')}
            value={filters.status}
            onChange={(status?: ChallengeStatus) => setFilters({ ...filters, status })}
            options={CHALLENGE_ENUMS.status.values.map((s) => ({ value: s, label: t(`lms.challenges.status.${s}`) }))}
            popupMatchSelectWidth={false}
          />
        </Form.Item>
      </Flex>
      <ChallengeProblem error={list.error} />
      <div data-testid="challenge-table">
        <ResponsiveTable<Challenge>
          rowKey="challenge_id"
          columns={columns}
          data={list.items}
          loading={list.isFetching}
          cardTitle={(c) => <Link to={c.challenge_id}>{c.name}</Link>}
          hasMore={list.hasMore}
          onLoadMore={list.loadMore}
        />
      </div>
    </Flex>
  );
}

export function ChallengeListPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const scope = useSchoolLmsScope();
  return (
    <Flex vertical gap="middle">
      <Flex justify="flex-end">
        <ActionGate actionKey="action.school_lms.challenges.create">
          <Button type="primary" disabled={!scope.ready} onClick={() => navigate('new')}>
            {t('lms.challenges.new')}
          </Button>
        </ActionGate>
      </Flex>
      {scope.ready ? (
        <ActionGate actionKey="action.school_lms.challenges.view">
          <ChallengeTable />
        </ActionGate>
      ) : (
        <Alert type="info" showIcon role="status" message={t('portal.school_lms.scope.pickOneTenant')} description={t('portal.school_lms.scope.pickOneTenantHelp')} />
      )}
    </Flex>
  );
}
