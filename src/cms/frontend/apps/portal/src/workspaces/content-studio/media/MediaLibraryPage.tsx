// MediaLibraryPage (F054-AC-16, AC-17, AC-18, AC-30, CONTENT_MODEL §7): "Video · Audio · Media" — listMediaAssets
// filtered by kind / status / `q` (cursor), status with the rejection reason, variants count, license, alt text,
// the System Admin Tenant column; detail drawer with "used in"; the multi-file upload zone (`media.upload`).
import { Button, Flex, Input, Select, Tag, Typography } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useInfiniteQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { usePortalRuntime } from '../../../app/runtime';
import { ActionGate } from '../../../permissions/ActionGate';
import { useSession } from '../../../session/SessionProvider';
import { useTenantOptions } from '../../../shared/api/hooks/useTenantOptions';
import { useCrossTenantScope } from '../../../shared/tenancy/useCrossTenantScope';
import { ProblemMessage } from '../../../shell/ProblemMessage';
import { ResponsiveTable } from '../../../shell/ResponsiveTable';
import { useTenantColumn } from '../../../shell/TenantControls';
import { LIMITS } from '../shared/config';
import { MediaDetailDrawer } from './MediaDetailDrawer';
import { MediaUploadDropzone } from './MediaUploadDropzone';
import type { MediaAsset, MediaKind, MediaStatus } from './useMediaUpload';

const KINDS: MediaKind[] = ['image', 'audio', 'video', 'model3d', 'document'];
const STATUSES: MediaStatus[] = ['quarantined', 'processing', 'ready', 'rejected'];
const COLOR: Record<MediaStatus, string> = { quarantined: 'processing', processing: 'processing', ready: 'success', rejected: 'error' };

type Page = { data: MediaAsset[]; page?: { has_more?: boolean; next_cursor?: string | null } };

export function useMediaLibrary(filters: { kind?: MediaKind; status?: MediaStatus; q?: string }) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  const scope = useCrossTenantScope();
  const query = scope.listQuery({ ...(filters.kind ? { kind: filters.kind } : {}), ...(filters.status ? { status: filters.status } : {}), ...(filters.q ? { q: filters.q } : {}), limit: 50 });
  return useInfiniteQuery({
    queryKey: ['content-studio', 'media', scopeKey, query],
    initialPageParam: undefined as string | undefined,
    queryFn: ({ pageParam }) => api.call<Page>(api.client.GET('/cms/media', { params: { query: { ...query, ...(pageParam ? { cursor: pageParam } : {}) } as never } })),
    getNextPageParam: (p) => (p.page?.has_more ? (p.page.next_cursor ?? undefined) : undefined),
  });
}

export function MediaLibraryPage({ title }: { title: string }) {
  const { t } = useTranslation();
  const scope = useCrossTenantScope();
  const [kind, setKind] = useState<MediaKind | undefined>();
  const [status, setStatus] = useState<MediaStatus | undefined>();
  const [q, setQ] = useState('');
  const [search, setSearch] = useState('');
  const [open, setOpen] = useState<string | null>(null);
  const [uploading, setUploading] = useState(false);
  const list = useMediaLibrary({ kind, status, q: search });
  const tenants = useTenantOptions('', scope.crossTenant);
  const tenantColumn = useTenantColumn<MediaAsset>(new Map((tenants.data ?? []).map((x) => [x.tenant_id, x])));
  const rows = (list.data?.pages ?? []).flatMap((p) => p.data);

  const columns: ColumnType<MediaAsset>[] = [
    {
      key: 'title',
      title: t('content_studio.media.col.name'),
      render: (_, a) => (
        <Button type="link" onClick={() => setOpen(a.asset_id)}>
          {a.original_filename}
        </Button>
      ),
    },
    { key: 'kind', title: t('content_studio.media.col.kind'), render: (_, a) => t(`content_studio.media.kind.${a.kind}`) },
    {
      key: 'status',
      title: t('content_studio.media.col.status'),
      render: (_, a) => (
        <span>
          <Tag color={COLOR[a.status]} data-media-status={a.status}>
            {t(`content_studio.media.status.${a.status}`)}
          </Tag>
          {a.status === 'rejected' && a.reject_reason ? <Typography.Text type="danger">{a.reject_reason}</Typography.Text> : null}
        </span>
      ),
    },
    { key: 'variants', title: t('content_studio.media.variants'), render: (_, a) => (a.variants ?? []).length },
    { key: 'license', title: t('content_studio.media.license'), render: (_, a) => a.license ?? '—' },
    { key: 'alt', title: t('content_studio.media.altText'), render: (_, a) => a.alt_text ?? '—' },
    ...(tenantColumn ? [tenantColumn] : []),
  ];

  return (
    <section aria-labelledby="ew-cs-media">
      <Flex vertical gap="middle">
        <Flex justify="space-between" align="center" wrap gap="small">
          <Typography.Title level={1} id="ew-cs-media">
            {title}
          </Typography.Title>
          <ActionGate actionKey="action.content_studio.media.upload">
            <Button type="primary" onClick={() => setUploading((x) => !x)}>
              {t('content_studio.media.upload')}
            </Button>
          </ActionGate>
        </Flex>
        {uploading && <MediaUploadDropzone onUploaded={() => void list.refetch()} />}
        <Flex gap="small" wrap align="center">
          <Select allowClear aria-label={t('content_studio.media.col.kind')} placeholder={t('content_studio.media.col.kind')} value={kind} onChange={setKind} options={KINDS.map((k) => ({ value: k, label: t(`content_studio.media.kind.${k}`) }))} />
          <Select allowClear aria-label={t('content_studio.media.col.status')} placeholder={t('content_studio.media.col.status')} value={status} onChange={setStatus} options={STATUSES.map((s) => ({ value: s, label: t(`content_studio.media.status.${s}`) }))} />
          <Input.Search aria-label={t('content_studio.media.search')} placeholder={t('content_studio.media.search')} maxLength={LIMITS.search} value={q} onChange={(e) => setQ(e.target.value)} onSearch={(v) => setSearch(v.trim())} allowClear />
        </Flex>
        {list.error ? <ProblemMessage error={list.error} /> : null}
        <ResponsiveTable<MediaAsset>
          rowKey="asset_id"
          columns={columns}
          data={rows}
          loading={list.isFetching}
          cardTitle={(a) => a.original_filename}
          hasMore={list.hasNextPage}
          onLoadMore={() => void list.fetchNextPage()}
          loadMoreLabel={t('content_studio.loadMore')}
        />
      </Flex>
      <MediaDetailDrawer assetId={open} onClose={() => setOpen(null)} />
    </section>
  );
}
