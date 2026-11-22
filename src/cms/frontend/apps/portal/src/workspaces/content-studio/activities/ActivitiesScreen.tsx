// Activities screen (F058 T001, T024; ADR-0090: menu `menu.content_studio.activities` of the generated catalog): the
// list of learning activities (listCmsActivities, filter by type and status), step 1 "chọn loại" from the registry of
// the backend ∩ the editors of this portal (F058-AC-01), the editor of a new or existing activity.
import { Alert, Button, Card, Flex, List, Space, Table, Tag, Typography } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, Route, Routes, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import { ActionGate } from '../../../permissions/ActionGate';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { AssignedBadge } from '../shared/AssignedBadge';
import { useContentAccess } from '../shared/access';
import { CONTENT_STATUSES } from '../shared/contentTypes';
import { ActivityEditorPage } from './ActivityEditorPage';
import type { ActivityType } from './activityModel';
import { activityEditor, offeredTypes } from './editorRegistry';
import { useActivityList, useActivityTypes, type ActivityListFilter } from './useActivity';
import { SearchSelect } from '@eduworld/ops-ui';
import './editors';

const label = (v: { vi?: string; en?: string } | undefined, en: boolean) => (en ? (v?.en ?? v?.vi) : (v?.vi ?? v?.en)) ?? '';

export function ActivityTypeStep() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const types = useActivityTypes();
  if (types.error) return <ProblemMessage error={types.error} />;
  const offered = offeredTypes(types.data?.data ?? []);
  return (
    <Flex vertical gap="middle">
      <Typography.Title level={2}>{t('content_studio.activity.chooseType')}</Typography.Title>
      <List
        loading={types.isPending}
        data-testid="activity-type-step"
        grid={{ gutter: 16, column: 3 }}
        dataSource={offered}
        renderItem={(x) => (
          <List.Item>
            <Card
              hoverable
              role="button"
              aria-label={t(activityEditor(x.activity_type)!.labelKey)}
              onClick={() => navigate(`new?type=${x.activity_type}`)}
              title={t(activityEditor(x.activity_type)!.labelKey)}
              extra={x.status === 'preview' ? <Tag color="warning">{t('content_studio.activity.statusPreview')}</Tag> : null}
            >
              <Typography.Text type="secondary">{t('content_studio.activity.minClient', { version: x.min_client_version })}</Typography.Text>
            </Card>
          </List.Item>
        )}
      />
    </Flex>
  );
}

function ActivityList({ title }: { title: string }) {
  const { t, i18n } = useTranslation();
  const access = useContentAccess();
  const [type, setType] = useState<ActivityType | undefined>();
  const [status, setStatus] = useState<ActivityListFilter['status']>();
  const list = useActivityList({ activity_type: type, status });
  const types = useActivityTypes();
  const en = i18n.language.startsWith('en');
  return (
    <Flex vertical gap="middle">
      <Flex justify="space-between" align="center">
        <Typography.Title level={1}>{title}</Typography.Title>
        {access.activity.write && (
          <ActionGate actionKey="action.content_studio.activities.edit">
            <Link to="new">
              <Button type="primary">{t('content_studio.activity.create')}</Button>
            </Link>
          </ActionGate>
        )}
      </Flex>
      <Space>
        <SearchSelect allowClear aria-label={t('content_studio.activity.filterType')} placeholder={t('content_studio.activity.filterType')} style={{ minWidth: 200 }} value={type} options={(types.data?.data ?? []).map((x) => ({ value: x.activity_type, label: activityEditor(x.activity_type) ? t(activityEditor(x.activity_type)!.labelKey) : x.activity_type }))} onChange={(v?: ActivityType) => setType(v)} />
        <SearchSelect allowClear aria-label={t('content_studio.activity.filterStatus')} placeholder={t('content_studio.activity.filterStatus')} style={{ minWidth: 160 }} value={status} options={CONTENT_STATUSES.map((s) => ({ value: s, label: t(`content_studio.status.${s}`) }))} onChange={(v?: ActivityListFilter['status']) => setStatus(v)} />
      </Space>
      {list.error ? <ProblemMessage error={list.error} /> : null}
      <Table
        rowKey="activity_id"
        loading={list.isPending}
        data-testid="activity-list"
        dataSource={list.rows}
        pagination={false}
        columns={[
          { title: t('content_studio.builder.title'), render: (_, a) => <Link to={a.activity_id}>{label(a.title as never, en) || a.activity_id}</Link> },
          { title: t('content_studio.activity.type'), render: (_, a) => (activityEditor(a.activity_type) ? t(activityEditor(a.activity_type)!.labelKey) : a.activity_type) },
          { title: t('content_studio.activity.statusCol'), render: (_, a) => <Tag>{t(`content_studio.status.${a.status}`)}</Tag> },
          { title: t('content_studio.activity.versionCol'), render: (_, a) => `v${a.version}${a.current_published_version ? ` · ${t('content_studio.activity.publishedV', { version: a.current_published_version })}` : ''}` },
          { title: '', render: (_, a) => (a.assigned ? <AssignedBadge /> : null) },
        ]}
      />
      {list.hasNextPage && (
        <Button onClick={() => void list.fetchNextPage()} loading={list.isFetchingNextPage}>
          {t('content_studio.loadMore')}
        </Button>
      )}
      {!access.activity.write && <Alert type="info" showIcon message={t('content_studio.activity.readOnlyRole')} />}
    </Flex>
  );
}

function NewActivity() {
  const [params] = useSearchParams();
  const type = params.get('type') as ActivityType | null;
  return type ? <ActivityEditorPage type={type} /> : <ActivityTypeStep />;
}

function EditActivity() {
  const { id } = useParams();
  return <ActivityEditorPage id={id} />;
}

export function ActivitiesScreen({ title }: { title: string }) {
  return (
    <Routes>
      <Route index element={<ActivityList title={title} />} />
      <Route path="new" element={<NewActivity />} />
      <Route path=":id" element={<EditActivity />} />
    </Routes>
  );
}
