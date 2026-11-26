// Game config API hooks of workspace A (F079 T016, plan "application"): `/api/v1/game-admin/{config_type}` of F078 —
// list (cursor pages for tables, every page for dropdowns), version (with ETag), versions, diff, releases, and the writes:
// create, draft update (If-Match — autosave), transition (submit_review / approve / reject / archive), publish (now or
// publish_at), cancel of a scheduled publish, rollback, unpublish and new draft from a version. Each write is one user
// intent (useIdempotentMutation: a retry reuses the Idempotency-Key, F079-AC-20); version writes send If-Match. No
// business rule here: the server decides (AGENTS R1).
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { usePortalRuntime } from '../../../app/runtime';
import { useSession } from '../../../session/SessionProvider';
import type { ApiClient } from '../../../shared/api/client';
import { useAllPages, type PageQuery } from '../../../shared/api/fetchAllPages';
import { ApiError } from '../../../shared/api/middleware/problem';
import type { components } from '../../../shared/api/schema.gen';
import { useCursorPages } from '../../../shared/api/useCursorPages';
import { INTENT_HEADER } from '../../../shared/api/middleware/idempotency';
import { useIdempotentMutation } from '../../../shared/api/useIdempotentMutation';
import { CONFIG_TYPES, type ConfigType } from '../registry/configTypes';

export type GameConfigVersion = components['schemas']['GameConfigVersion'];
export type ContentStatus = components['schemas']['ContentStatus'];
export type VersionSummary = components['schemas']['ContentVersionSummary'];
export type VersionHistory = components['schemas']['ContentVersionHistory'];
export type VersionDiff = components['schemas']['ContentVersionDiff'];
export type ContentRelease = components['schemas']['ContentRelease'];
export type PageInfo = components['schemas']['PageInfo'];
export type TranslationStatus = components['schemas']['TranslationStatus'];

/** Server filters of listGameConfigs (F079 status; F082 translation_status for game types, tag / author_id for GIFs). */
export interface ConfigListFilters {
  status?: ContentStatus;
  translation_status?: TranslationStatus;
  tag?: string;
  author_id?: string;
}

const filterQuery = (f: ConfigListFilters): Record<string, string> => Object.fromEntries(Object.entries(f).filter(([, v]) => v !== undefined && v !== '')) as Record<string, string>;
export interface ConfigPage {
  data: GameConfigVersion[];
  page: PageInfo;
}

export interface ItemRef {
  configType: ConfigType;
  configId: string;
}

export interface Loaded<T> {
  data: T;
  etag: string | null;
}

const GA = '/game-admin/{config_type}' as const;
const ITEM = '/game-admin/{config_type}/{config_id}/versions/{version}' as const;

export const gameKeys = {
  all: ['game-ops'] as const,
  list: (scope: string, t: ConfigType, f: Record<string, unknown>) => ['game-ops', 'list', scope, t, f] as const,
  version: (scope: string, r: ItemRef, v: number) => ['game-ops', 'version', scope, r.configType, r.configId, v] as const,
  versions: (scope: string, r: ItemRef) => ['game-ops', 'versions', scope, r.configType, r.configId] as const,
  releases: (scope: string, r: ItemRef) => ['game-ops', 'releases', scope, r.configType, r.configId] as const,
  diff: (scope: string, r: ItemRef, v: number, w: number) => ['game-ops', 'diff', scope, r.configType, r.configId, v, w] as const,
};

async function withEtag<T>(api: ApiClient, p: Promise<{ data?: T; error?: unknown; response: Response }>): Promise<Loaded<T>> {
  const r = await p;
  const data = await api.call(Promise.resolve(r));
  return { data, etag: r.response.headers.get('ETag') };
}

const listPage = (api: ApiClient, t: ConfigType, query: Record<string, unknown>) =>
  api.call<ConfigPage>(api.client.GET(GA, { params: { path: { config_type: t }, query: query as never } }));

/** Table / grid rows of a type: cursor pages ("Tải thêm") paged by the server, with its filters (F079-AC-01, F082-AC-05, AC-09). */
export function useConfigPages(t: ConfigType, filters: ContentStatus | ConfigListFilters | undefined, enabled = true) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  const f = filterQuery(typeof filters === 'string' ? { status: filters } : (filters ?? {}));
  return useCursorPages<GameConfigVersion, ConfigPage>(gameKeys.list(scopeKey, t, { ...f, pages: true }), (cursor) => listPage(api, t, { ...f, ...(cursor ? { cursor } : {}) }), { enabled });
}

/** Every config of a type (dropdowns / reference pickers): all cursor pages (ADMIN_PORTAL §1.6). */
export function useAllConfigs(t: ConfigType, status?: ContentStatus, enabled = true) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useAllPages<GameConfigVersion>(gameKeys.list(scopeKey, t, { status: status ?? null }), (page: PageQuery) => listPage(api, t, { ...(status ? { status } : {}), ...page }), {
    enabled,
  });
}

/** Events of a date window (ICT days, F079-AC-14): every page of the filtered list (calendar, not a table). */
export function useEventWindow(from: string, to: string) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useAllPages<GameConfigVersion>(gameKeys.list(scopeKey, 'events', { from, to }), (page: PageQuery) => listPage(api, 'events', { from, to, ...page }));
}

export function useConfigVersion(r: ItemRef, version: number) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: gameKeys.version(scopeKey, r, version),
    queryFn: () => withEtag<GameConfigVersion>(api, api.client.GET(ITEM, { params: { path: { config_type: r.configType, config_id: r.configId, version } } })),
    refetchOnWindowFocus: false,
  });
}

/** Fresh ETag of a version (If-Match of a workflow write). */
export async function versionEtag(api: ApiClient, r: ItemRef, version: number): Promise<string | null> {
  const loaded = await withEtag<GameConfigVersion>(api, api.client.GET(ITEM, { params: { path: { config_type: r.configType, config_id: r.configId, version } } }));
  return loaded.etag;
}

/** Version history table: cursor pages. */
export function useVersionPages(r: ItemRef) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useCursorPages<VersionSummary, VersionHistory>(gameKeys.versions(scopeKey, r), (cursor) =>
    api.call<VersionHistory>(
      api.client.GET('/game-admin/{config_type}/{config_id}/versions', { params: { path: { config_type: r.configType, config_id: r.configId }, query: cursor ? { cursor } : {} } }),
    ),
  );
}

/** Every version (rollback targets): all pages. */
export function useAllVersions(r: ItemRef) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useAllPages<VersionSummary>([...gameKeys.versions(scopeKey, r), 'all'], (page) =>
    api.call<VersionHistory>(
      api.client.GET('/game-admin/{config_type}/{config_id}/versions', { params: { path: { config_type: r.configType, config_id: r.configId }, query: page } }),
    ),
  );
}

export function useReleasePages(r: ItemRef) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useCursorPages<ContentRelease, { data: ContentRelease[]; page: PageInfo }>(gameKeys.releases(scopeKey, r), (cursor) =>
    api.call(api.client.GET('/game-admin/{config_type}/{config_id}/releases', { params: { path: { config_type: r.configType, config_id: r.configId }, query: cursor ? { cursor } : {} } })),
  );
}

export function useDiff(r: ItemRef, version: number | undefined, against: number | undefined) {
  const { api } = usePortalRuntime();
  const { scopeKey } = useSession();
  return useQuery({
    queryKey: gameKeys.diff(scopeKey, r, version ?? 0, against ?? 0),
    queryFn: () =>
      api.call<VersionDiff>(
        api.client.GET('/game-admin/{config_type}/{config_id}/versions/{version}:diff', {
          params: { path: { config_type: r.configType, config_id: r.configId, version: version! }, query: { with: against! } },
        }),
      ),
    enabled: !!version && !!against && version !== against,
  });
}

const ifMatch = (etag: string | null): Record<string, string> => (etag ? { 'If-Match': etag } : {});

export interface DraftInput {
  title?: string;
  payload: Record<string, unknown>;
}

/** Draft save of the autosave (If-Match = last ETag); resolves with the new ETag. */
export async function saveDraft(api: ApiClient, r: ItemRef, version: number, input: DraftInput, etag: string | null): Promise<Loaded<GameConfigVersion>> {
  return withEtag<GameConfigVersion>(
    api,
    api.client.PATCH(ITEM, {
      params: { path: { config_type: r.configType, config_id: r.configId, version } },
      body: { ...(input.title ? { title: input.title } : {}), payload: input.payload },
      headers: ifMatch(etag) as never,
    }),
  );
}

/** `412` or `409 CONTENT_VERSION_CONFLICT`: someone else saved the version (F079-AC-02). */
export const isConflict = (e: unknown) => e instanceof ApiError && (e.status === 412 || e.code === 'CONTENT_VERSION_CONFLICT' || e.code === 'PRECONDITION_FAILED');

export type TransitionAction = 'submit_review' | 'approve' | 'reject' | 'archive';

export function useConfigWrites() {
  const { api } = usePortalRuntime();
  const qc = useQueryClient();
  const done = () => qc.invalidateQueries({ queryKey: gameKeys.all });
  const opts = { onSuccess: done, onError: (e: unknown) => void (e instanceof ApiError && (e.status === 409 || e.status === 412) && done()) };

  const create = useIdempotentMutation(
    (v: { configType: ConfigType } & DraftInput, intent) =>
      withEtag<GameConfigVersion>(
        api,
        api.client.POST(GA, { params: { path: { config_type: v.configType } }, body: { ...(v.title ? { title: v.title } : {}), payload: v.payload }, headers: intent as never }),
      ),
    opts,
  );

  const transition = useIdempotentMutation(async (v: ItemRef & { version: number; action: TransitionAction; note?: string }, intent) => {
    const etag = await versionEtag(api, v, v.version);
    return api.call<GameConfigVersion>(
      api.client.POST('/game-admin/{config_type}/{config_id}/versions/{version}:transition', {
        params: { path: { config_type: v.configType, config_id: v.configId, version: v.version } },
        body: { action: v.action, ...(v.note ? { note: v.note } : {}) },
        headers: { ...intent, ...ifMatch(etag) } as never,
      }),
    );
  }, opts);

  const publish = useIdempotentMutation(async (v: ItemRef & { version: number; publishAt?: string; effectiveFrom?: string; changeNote?: string }, intent) => {
    const etag = await versionEtag(api, v, v.version);
    return api.call<ContentRelease>(
      api.client.POST('/game-admin/{config_type}/{config_id}/versions/{version}:publish', {
        params: { path: { config_type: v.configType, config_id: v.configId, version: v.version } },
        body: { ...(v.publishAt ? { publish_at: v.publishAt } : {}), ...(v.effectiveFrom ? { effective_from: v.effectiveFrom } : {}), ...(v.changeNote ? { change_note: v.changeNote } : {}) },
        headers: { ...intent, ...ifMatch(etag) } as never,
      }),
    );
  }, opts);

  const cancelSchedule = useIdempotentMutation(
    (v: ItemRef & { releaseId: string; reason?: string }, intent) =>
      api.call<ContentRelease>(
        api.client.POST('/game-admin/{config_type}/{config_id}/releases/{release_id}:cancel', {
          params: { path: { config_type: v.configType as never, config_id: v.configId, release_id: v.releaseId } },
          body: v.reason ? { reason: v.reason } : {},
          headers: intent as never,
        }),
      ),
    opts,
  );

  const rollback = useIdempotentMutation(
    (v: ItemRef & { toVersion: number; reason: string }, intent) =>
      api.call<ContentRelease>(
        api.client.POST('/game-admin/{config_type}/{config_id}:rollback', {
          params: { path: { config_type: v.configType, config_id: v.configId } },
          body: { to_version: v.toVersion, reason: v.reason },
          headers: intent as never,
        }),
      ),
    opts,
  );

  const unpublish = useIdempotentMutation(
    (v: ItemRef & { reason: string }, intent) =>
      api.call<ContentRelease>(
        api.client.POST('/game-admin/{config_type}/{config_id}:unpublish', { params: { path: { config_type: v.configType, config_id: v.configId } }, body: { reason: v.reason }, headers: intent as never }),
      ),
    opts,
  );

  const newVersion = useIdempotentMutation(async (v: ItemRef & { basedOn: number }, intent) => {
    if (CONFIG_TYPES[v.configType].newDraft === 'create') {
      // Module competition (F108, Clarifications 2026-11-16): createGameConfig with config_id + based_on_version_id; the
      // new draft starts from the source content, the source version never changes.
      const src = await withEtag<GameConfigVersion>(api, api.client.GET(ITEM, { params: { path: { config_type: v.configType, config_id: v.configId, version: v.basedOn } } }));
      return api.call<GameConfigVersion>(
        api.client.POST(GA, {
          params: { path: { config_type: v.configType } },
          body: { config_id: v.configId, based_on_version_id: src.data.config_version_id, payload: src.data.payload, ...(src.data.title ? { title: src.data.title } : {}) },
          headers: intent as never,
        }),
      );
    }
    return api.call<GameConfigVersion>(
      api.client.POST('/game-admin/{config_type}/{config_id}/versions', {
        params: { path: { config_type: v.configType as never, config_id: v.configId } },
        body: { based_on_version: v.basedOn },
        headers: intent as never,
      }),
    );
  }, opts);

  return { create, transition, publish, cancelSchedule, rollback, unpublish, newVersion };
}

export type ChatAssetPresignResponse = { asset_key: string; upload_url: string; expires_at: string };

/**
 * presignChatAssetUpload (module chat, F082-AC-03): one sticker/GIF file of a draft. `intent` is the per-file intent of the
 * upload queue (Idempotency-Key kept on retry).
 */
export function presignChatAsset(api: ApiClient, r: ItemRef, input: { fileName: string; contentType: string; size: number; sha256: string; intent: string }) {
  return api.call<ChatAssetPresignResponse>(
    api.client.POST('/game-admin/{config_type}/{config_id}/assets:presign', {
      params: { path: { config_type: r.configType as 'sticker-packs' | 'gifs', config_id: r.configId } },
      body: { file_name: input.fileName, content_type: input.contentType as 'image/webp', size_bytes: input.size, sha256: input.sha256 },
      headers: { [INTENT_HEADER]: input.intent } as never,
    }),
  );
}
