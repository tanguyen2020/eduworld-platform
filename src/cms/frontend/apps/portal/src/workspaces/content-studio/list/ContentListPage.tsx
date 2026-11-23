// ContentListPage (F054-AC-24, AC-25, AC-28, AC-30, ADMIN_PORTAL "Danh sách nội dung"): list + search of one content
// type — filters (ContentFilters, remembered per viewer), status Tag, version, "Từ EduWorld" for assigned platform
// content, the Tenant column for the System Admin (topbar filter → `tenant_ids[]`), and per row Mở / Xóa (soft delete,
// not for assigned content). Toolbar: Tạo mới (`*.edit`), Nhập (questions, `learning_content.import`). Cards < 768 px.
import { DomainTag } from '@eduworld/ops-ui';
import { Button, Flex, Space, Typography } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { ActionGate } from '../../../permissions/ActionGate';
import { useSession } from '../../../session/SessionProvider';
import { useTenantOptions } from '../../../shared/api/hooks/useTenantOptions';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { ResponsiveTable } from '../../../shell/ResponsiveTable';
import { useTenantColumn } from '../../../shell/TenantControls';
import { formatIct } from '../release/releaseOptions';
import { useContentAccess } from '../shared/access';
import { AssignedBadge } from '../shared/AssignedBadge';
import { CONTENT_TYPES } from '../shared/contentTypes';
import { ContentFilters } from './ContentFilters';
import { DeleteContentDialog } from './DeleteContentDialog';
import { filtersStorageKey, loadFilters, saveFilters, useContentList, type ContentFiltersValue, type ContentRow, type ListType } from './useContentList';

export function ContentListPage({ type, title }: { type: ListType; title: string }) {
  const { t, i18n } = useTranslation();
  const navigate = useNavigate();
  const { userKey } = useSession();
  const access = useContentAccess();
  const crossTenant = access.crossTenant;
  const storageKey = filtersStorageKey(userKey, type);
  const [filters, setFilters] = useState<ContentFiltersValue>(() => loadFilters(storageKey));
  const list = useContentList(type, filters);
  const tenants = useTenantOptions('', crossTenant);
  const tenantColumn = useTenantColumn<ContentRow>(new Map((tenants.data ?? []).map((x) => [x.tenant_id, x])));
  const [deleting, setDeleting] = useState<ContentRow | null>(null);
  const actions = CONTENT_TYPES[type].actions;
  const rows = list.rows;
  const name = (r: ContentRow) => {
    const title = r.title ? ((i18n.language.startsWith('en') ? (r.title.en ?? r.title.vi) : (r.title.vi ?? r.title.en)) ?? '') : '';
    return title || t('content_studio.list.untitled', { id: r.content_id.slice(-8), type: r.type ? t(`ops-ui:question.types.${r.type}`) : '' });
  };
  const change = (f: ContentFiltersValue) => {
    setFilters(f);
    saveFilters(storageKey, f);
  };

  const columns: ColumnType<ContentRow>[] = [
    {
      key: 'title',
      title: t('content_studio.list.col.title'),
      render: (_, r) => (
        <Space>
          <Button type="link" onClick={() => navigate(r.content_id)}>
            {name(r)}
          </Button>
          {r.assigned && <AssignedBadge />}
        </Space>
      ),
    },
    { key: 'status', title: t('content_studio.release.col.status'), render: (_, r) => <DomainTag kind="content_status" value={r.status} /> },
    { key: 'version', title: t('content_studio.release.col.version'), render: (_, r) => t('content_studio.release.v', { version: r.version }) },
    { key: 'updated', title: t('content_studio.list.col.updated'), render: (_, r) => formatIct(r.updated_at ?? r.published_at) || '—' },
    ...(tenantColumn ? [tenantColumn] : []),
    {
      key: 'actions',
      title: t('content_studio.release.col.actions'),
      render: (_, r) => (
        <Space>
          <Button size="small" onClick={() => navigate(r.content_id)}>
            {t('content_studio.list.open')}
          </Button>
          {!r.assigned && actions.edit && (
            <ActionGate actionKey={actions.edit}>
              <Button size="small" danger onClick={() => setDeleting(r)}>
                {t('content_studio.list.delete')}
              </Button>
            </ActionGate>
          )}
        </Space>
      ),
    },
  ];

  return (
    <section aria-labelledby={`ew-cs-list-${type}`}>
      <Flex vertical gap="middle">
        <Flex justify="space-between" align="center" wrap gap="small">
          <Typography.Title level={1} id={`ew-cs-list-${type}`}>
            {title}
          </Typography.Title>
          <Space wrap>
            {type === 'question' && (
              <ActionGate actionKey="action.content_studio.question_bank.import">
                <Button onClick={() => navigate('import')}>{t('content_studio.import.open')}</Button>
              </ActionGate>
            )}
            {actions.edit && (
              <ActionGate actionKey={actions.edit}>
                <Button type="primary" onClick={() => navigate('new')}>
                  {t('content_studio.list.create')}
                </Button>
              </ActionGate>
            )}
          </Space>
        </Flex>
        <Flex gap="small" wrap align="center">
          <ContentFilters type={type} value={filters} onChange={change} platform={access.platform} />
        </Flex>
        {list.error ? <ProblemMessage error={list.error} /> : null}
        <ResponsiveTable<ContentRow>
          rowKey="content_id"
          columns={columns}
          data={rows}
          loading={list.isFetching}
          cardTitle={(r) => (
            <Button type="link" onClick={() => navigate(r.content_id)}>
              {name(r)}
            </Button>
          )}
          hasMore={list.hasNextPage}
          onLoadMore={() => void list.fetchNextPage()}
          loadMoreLabel={t('content_studio.loadMore')}
        />
        {!list.isFetching && rows.length === 0 && <Typography.Text type="secondary">{t('content_studio.list.empty')}</Typography.Text>}
      </Flex>
      <DeleteContentDialog type={type} row={deleting} onClose={() => setDeleting(null)} />
    </section>
  );
}
