// useVersions / useRelease (F054 T025, data-model §1, ADR-0043): the release API of one content item —
// listContentVersions, listContentReleases, diffContentVersions, getContentVersion (reads, keys with the session
// scope) and the writes publishContentVersion (now / publish_at), cancelScheduledPublication, rollbackContent,
// unpublishContent, transitionContentVersion `archive`, create*Version ("Tạo bản nháp từ phiên bản này"). Every write
// is one user intent (useIdempotentMutation: retry = same Idempotency-Key, F054-AC-11) and the version writes send
// If-Match = the ETag of a fresh getContentVersion. Success invalidates the item's versions, releases, lists and queue.
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { usePortalRuntime } from '../../../app/runtime';
import { useSession } from '../../../session/SessionProvider';
import type { ApiClient } from '../../../shared/api/client';
import { TENANT_HEADER } from '../../../shared/api/middleware/tenant';
import { ApiError } from '../../../shared/api/middleware/problem';
import type { components } from '../../../shared/api/schema.gen';
import { fetchAllPages } from '../../../shared/api/fetchAllPages';
import { useCursorPages } from '../../../shared/api/useCursorPages';
import { useIdempotentMutation, type IntentHeaders } from '../../../shared/api/useIdempotentMutation';
import { withEtag } from '../questions/useQuestion';
import { CONTENT_TYPES, type ContentType } from '../shared/contentTypes';

export type VersionHistory = components['schemas']['ContentVersionHistory'];
export type ContentRelease = components['schemas']['ContentRelease'];
export type VersionDiff = components['schemas']['ContentVersionDiff'];
export type VersionDetail = components['schemas']['ContentVersionDetail'];

type Page<T> = { data: T[]; page?: { has_more?: boolean; next_cursor?: string | null } };

export interface ItemRef {
  contentType: ContentType;
  contentId: string;
  /**
   * Tenant of a tenant-owned item opened in platform scope (System Admin on a review task of lane `tenant`): sent as
   * X-Tenant-Id so the reads and the decision target that tenant (AGENTS R20: a System Admin write has one target
   * tenant). Ignored for tenant users — the session tenant always applies (middleware/tenant).
   */
  tenantId?: string;
}

const path = (r: ItemRef) => ({ content_type: r.contentType, content_id: r.contentId });

/** X-Tenant-Id of an item opened in platform scope for one tenant (`ItemRef.tenantId`); nothing otherwise. */
export const itemTenantHeaders = (r: ItemRef): Record<string, string> => (r.tenantId ? { [TENANT_HEADER]: r.tenantId } : {});

export const releaseKeys = {
  versions: (scope: string, r: ItemRef, status?: string) => ['content-studio', 'versions', scope, r.tenantId ?? null, r.contentType, r.contentId, status ?? null] as const,
  releases: (scope: string, r: ItemRef) => ['content-studio', 'releases', scope, r.tenantId ?? null, r.contentType, r.contentId] as const,
  diff: (scope: string, r: ItemRef, v: number, w: number) => ['content-studio', 'diff', scope, r.tenantId ?? null, r.contentType, r.contentId, v, w] as const,
  version: (scope: string, r: ItemRef, v: number) => ['content-studio', 'version', scope, r.tenantId ?? null, r.contentType, r.contentId, v] as const,
};

type ContentStatus = components['schemas']['ContentStatus'];
type VersionSummary = components['schemas']['ContentVersionSummary'];

/** Every version of the item (fetchAllPages): lookups, rollback targets, "current" — not a table. */
export function useVersions(r: ItemRef, status?: ContentStatus) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: releaseKeys.versions(scopeKey, r, status),
    queryFn: async (): Promise<VersionHistory & { truncated: boolean }> => {
      let first: VersionHistory | undefined;
      const all = await fetchAllPages<VersionSummary>(async (page) => {
        const p = await api.call<VersionHistory>(
          api.client.GET('/cms/learning/content/{content_type}/{content_id}/versions', { params: { path: path(r), query: { ...(status ? { status } : {}), ...page } }, headers: itemTenantHeaders(r) as never }),
        );
        first ??= p;
        return p;
      });
      return { ...first!, data: all.items, truncated: all.truncated };
    },
  });
}

/** Version history table: cursor pages, "Tải thêm" (owner rule 2026-11-22). */
export function useVersionPages(r: ItemRef, status?: ContentStatus) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useCursorPages<VersionSummary, VersionHistory>([...releaseKeys.versions(scopeKey, r, status), 'pages'], (cursor) =>
    api.call<VersionHistory>(
      api.client.GET('/cms/learning/content/{content_type}/{content_id}/versions', { params: { path: path(r), query: { ...(status ? { status } : {}), ...(cursor ? { cursor } : {}) } }, headers: itemTenantHeaders(r) as never }),
    ),
  );
}

/** Release timeline: cursor pages, "Tải thêm". */
export function useReleases(r: ItemRef) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useCursorPages<ContentRelease, Page<ContentRelease>>(releaseKeys.releases(scopeKey, r), (cursor) =>
    api.call<Page<ContentRelease>>(api.client.GET('/cms/learning/content/{content_type}/{content_id}/releases', { params: { path: path(r), query: cursor ? { cursor } : {} }, headers: itemTenantHeaders(r) as never })),
  );
}

/** Diff of version `v` against `w` (both of the same item). */
export function useDiff(r: ItemRef, v: number | undefined, w: number | undefined) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: releaseKeys.diff(scopeKey, r, v ?? 0, w ?? 0),
    queryFn: () =>
      api.call<VersionDiff>(api.client.GET('/cms/learning/content/{content_type}/{content_id}/versions/{version}:diff', { params: { path: { ...path(r), version: v! }, query: { with: w! } }, headers: itemTenantHeaders(r) as never })),
    enabled: !!v && !!w && v !== w,
  });
}

export function useVersionDetail(r: ItemRef, v: number | undefined) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: releaseKeys.version(scopeKey, r, v ?? 0),
    queryFn: () => withEtag<VersionDetail>(api, api.client.GET('/cms/learning/content/{content_type}/{content_id}/versions/{version}', { params: { path: { ...path(r), version: v! } }, headers: itemTenantHeaders(r) as never })),
    enabled: !!v,
  });
}

/** ETag of a version, read fresh before a write (If-Match). */
export async function versionEtag(api: ApiClient, r: ItemRef, version: number): Promise<string | null> {
  const loaded = await withEtag<VersionDetail>(api, api.client.GET('/cms/learning/content/{content_type}/{content_id}/versions/{version}', { params: { path: { ...path(r), version } }, headers: itemTenantHeaders(r) as never }));
  return loaded.etag;
}

const ifMatch = (etag: string | null): Record<string, string> => (etag ? { 'If-Match': etag } : {});

/** Path of create*Version for a content type (absent = the contract has none). */
const NEW_VERSION_PATH: Partial<Record<ContentType, string>> = {
  question: '/cms/learning/questions/{question_id}/versions',
  question_set: '/cms/learning/question-sets/{question_set_id}/versions',
  assessment_blueprint: '/cms/learning/assessment-blueprints/{assessment_blueprint_id}/versions',
  lesson: '/cms/learning/lessons/{lesson_id}/versions',
  learning_material: '/cms/learning/learning-materials/{learning_material_id}/versions',
  activity: '/cms/learning/activities/{activity_id}/versions',
};

/** Path parameter name of the item id in the authoring API of a type (`question_set_id`…). */
export const idParamOf = (t: ContentType) => `${t}_id`;

export function canCreateVersion(t: ContentType): boolean {
  return !!NEW_VERSION_PATH[t] && !!CONTENT_TYPES[t].base;
}

export interface PublishVars extends ItemRef {
  version: number;
  /** UTC ISO; absent = publish now. */
  publishAt?: string;
  changeNote?: string;
}

export function useRelease() {
  const { api } = usePortalRuntime();
  const qc = useQueryClient();
  const done = () => qc.invalidateQueries({ queryKey: ['content-studio'] });
  // success, or the state changed elsewhere (409 / 412): refresh versions, releases, lists and queue (edge case AC-04)
  const opts = { onSuccess: done, onError: (e: unknown) => void (e instanceof ApiError && (e.status === 409 || e.status === 412) && done()) };

  const publish = useIdempotentMutation(async (v: PublishVars, intent) => {
    const etag = await versionEtag(api, v, v.version);
    return api.call<ContentRelease>(
      api.client.POST('/cms/learning/content/{content_type}/{content_id}/versions/{version}:publish', {
        params: { path: { ...path(v), version: v.version } },
        body: { ...(v.publishAt ? { publish_at: v.publishAt } : {}), ...(v.changeNote ? { change_note: v.changeNote } : {}) },
        headers: { ...intent, ...ifMatch(etag), ...itemTenantHeaders(v) } as never,
      }),
    );
  }, opts);

  const cancelSchedule = useIdempotentMutation(
    (v: ItemRef & { releaseId: string }, intent) =>
      api.call<ContentRelease>(
        api.client.POST('/cms/learning/content/{content_type}/{content_id}/releases/{release_id}:cancel', { params: { path: { ...path(v), release_id: v.releaseId } }, headers: { ...intent, ...itemTenantHeaders(v) } as never }),
      ),
    opts,
  );

  const rollback = useIdempotentMutation(
    (v: ItemRef & { toVersion: number; reason: string }, intent) =>
      api.call<ContentRelease>(
        api.client.POST('/cms/learning/content/{content_type}/{content_id}:rollback', { params: { path: path(v) }, body: { to_version: v.toVersion, reason: v.reason }, headers: { ...intent, ...itemTenantHeaders(v) } as never }),
      ),
    opts,
  );

  const unpublish = useIdempotentMutation(
    (v: ItemRef & { reason: string }, intent) =>
      api.call<ContentRelease>(api.client.POST('/cms/learning/content/{content_type}/{content_id}:unpublish', { params: { path: path(v) }, body: { reason: v.reason }, headers: { ...intent, ...itemTenantHeaders(v) } as never })),
    opts,
  );

  const archive = useIdempotentMutation(async (v: ItemRef & { version: number; reason: string }, intent) => {
    const etag = await versionEtag(api, v, v.version);
    return api.call(
      api.client.POST('/cms/learning/content/{content_type}/{content_id}/versions/{version}:transition', {
        params: { path: { ...path(v), version: v.version } },
        body: { action: 'archive', note: v.reason },
        headers: { ...intent, ...ifMatch(etag), ...itemTenantHeaders(v) } as never,
      }),
    );
  }, opts);

  const newDraft = useIdempotentMutation((v: ItemRef & { fromVersion: number }, intent: IntentHeaders) => {
    const p = NEW_VERSION_PATH[v.contentType];
    if (!p) throw new Error(`no create-version operation for ${v.contentType}`);
    return api.call<{ version: number }>(
      api.client.POST(p as never, { params: { path: { [idParamOf(v.contentType)]: v.contentId } }, body: { from_version: v.fromVersion }, headers: { ...intent, ...itemTenantHeaders(v) } } as never),
    );
  }, opts);

  return { publish, cancelSchedule, rollback, unpublish, archive, newDraft };
}
