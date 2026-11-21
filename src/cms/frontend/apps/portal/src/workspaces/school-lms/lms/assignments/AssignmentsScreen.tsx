// Screen of `menu.school_lms.assignments` (F055 T018, T019, T026): list, wizard (`new`), detail (`:assignmentId`).
// One tenant at a time (SingleTenantGate: a System Admin must pick the target tenant first — F055-AC-07). The list filters
// by school, subject, grade (ADR-0095) and status and shows the targets of each assignment (grades or students).
import { Button, Flex, Form, Select, Typography } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, Route, Routes } from 'react-router-dom';
import { ActionGate } from '../../../../permissions/ActionGate';
import { formatDateTime } from '../../common/format';
import { ResponsiveTable } from '../../common/ResponsiveTable';
import { SingleTenantGate } from '../../common/scope';
import { PageTitle } from '../../common/ui';
import { useAssignments, type Assignment, type AssignmentFilters, type AssignmentStatus } from '../api/assignments';
import { LMS_ENUMS } from '../model/fieldSchemas';
import { LmsProblem, LmsTag, TargetsText } from '../LmsUi';
import { AssignmentDetailPage } from './AssignmentDetailPage';
import { AssignmentFilterBar } from './AssignmentFilterBar';
import { AssignmentWizard } from './wizard/AssignmentWizard';

function AssignmentListPage() {
  const { t } = useTranslation();
  const [filters, setFilters] = useState<AssignmentFilters>({});
  const list = useAssignments(filters);
  const columns: ColumnType<Assignment>[] = [
    { key: 'title', title: t('lms.assignment.title'), render: (_, a) => <Link to={a.assignment_id}>{a.title}</Link> },
    { key: 'targets', title: t('lms.wizard.steps.targets'), render: (_, a) => <TargetsText targets={a.targets} /> },
    { key: 'status', title: t('lms.assignment.status'), render: (_, a) => <LmsTag group="assignmentStatus" value={a.status} /> },
    { key: 'mode', title: t('lms.wizard.mode'), render: (_, a) => t(`lms.mode.${a.mode}`) },
    { key: 'due', title: t('lms.wizard.dueAt'), render: (_, a) => formatDateTime(a.due_at) },
    { key: 'done', title: t('lms.assignment.completion'), render: (_, a) => (a.completion ? `${a.completion.submitted ?? 0}/${a.completion.assigned ?? 0}` : '') },
  ];
  return (
    <Flex vertical gap="middle">
      <Flex wrap="wrap" gap="middle" justify="space-between" align="end">
        <Flex wrap="wrap" gap="small" align="end" data-testid="assignment-filters">
          <AssignmentFilterBar value={filters} onChange={setFilters} />
          <Form.Item label={t('lms.assignment.status')} style={{ marginBottom: 0 }}>
            <Select
              allowClear
              aria-label={t('lms.assignment.status')}
              placeholder={t('lms.assignment.status')}
              value={filters.status}
              onChange={(status?: AssignmentStatus) => setFilters({ ...filters, status })}
              options={LMS_ENUMS.assignmentStatus.values.map((s) => ({ value: s, label: t(`lms.assignmentStatus.${s}`) }))}
              popupMatchSelectWidth={false}
            />
          </Form.Item>
        </Flex>
        <ActionGate actionKey="action.school_lms.assignments.edit">
          <Link to="new">
            <Button type="primary">{t('lms.assignment.new')}</Button>
          </Link>
        </ActionGate>
      </Flex>
      <LmsProblem error={list.error} />
      <ResponsiveTable<Assignment>
        rowKey="assignment_id"
        columns={columns}
        data={list.items}
        loading={list.isFetching}
        cardTitle={(a) => <Link to={a.assignment_id}>{a.title}</Link>}
        hasMore={list.hasMore}
        onLoadMore={list.loadMore}
      />
    </Flex>
  );
}

export function AssignmentsScreen({ title }: { title: string }) {
  const { t } = useTranslation();
  return (
    <section aria-labelledby="ew-menu-title">
      <PageTitle title={title} />
      <SingleTenantGate>
        {() => (
          <Routes>
            <Route index element={<AssignmentListPage />} />
            <Route
              path="new"
              element={
                <>
                  <Typography.Title level={2}>{t('lms.wizard.heading')}</Typography.Title>
                  <AssignmentWizard />
                </>
              }
            />
            <Route path=":assignmentId" element={<AssignmentDetailPage />} />
          </Routes>
        )}
      </SingleTenantGate>
    </section>
  );
}
