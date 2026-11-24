// MOCK (test only, AGENTS R17): in-memory stand-in of the Game Operations Console operations of
// specs/openapi/eduworld-api.v1.yaml (`/game-admin/*`, `/game-media*` — F078 + the F079 deltas) used by the workspace A
// screens. It records requests, keeps a small state (configs, versions, releases), answers ETag / If-Match (412 when
// stale), the reviewer ≠ author rule (403) and cursor pages. It enforces NO real authorization or validation and is NOT
// a backend: tests add per-case answers (422 field errors, 403, VPN…) with `backend.overrides`.
import { WORKSPACE_CATALOG, type WorkspaceKey } from '../generated/menu-catalog';
import { ACCOUNTS, cursorPage, problem, type MockAccount, type MockPortalBackend, type RecordedRequest } from './mock-backend';

const json = (status: number, body: unknown, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', ...headers } });

/** Game permissions of the catalog (every `game.*` the menus of workspace A list). */
const GAME_PERMS = [...new Set(WORKSPACE_CATALOG.flatMap((w) => w.menus.flatMap((m) => [...m.requiredPermissions, ...m.actions.flatMap((a) => a.requiredPermissions)])).filter((p) => p.startsWith('game.')))];

function workspacesOf(perms: readonly string[]): WorkspaceKey[] {
  return WORKSPACE_CATALOG.filter((w) => w.menus.some((m) => m.requiredPermissions.some((x) => perms.includes(x)))).map((w) => w.key);
}

const platformAccount = (username: string, sub: string, displayName: string, permissions: string[]): MockAccount => ({
  username,
  email: `${username}@eduworld.test`,
  password: ACCOUNTS['sa.admin']!.password,
  platform: true,
  sub,
  displayName,
  tenants: [],
  perms: { platform: { workspaces: workspacesOf(permissions), permissions } },
});

/** Seed accounts of the F079 quickstart (roles of src/backend/pkg/authz/roles.go). */
export const GAME_ACCOUNTS: Record<string, MockAccount> = {
  'gd.thietke': platformAccount('gd.thietke', '0193a4d2-7000-7000-8000-0000000000d1', 'Game Designer', GAME_PERMS.filter((p) => p !== 'game.content.publish' && p !== 'game.economy.adjust')),
  'cm.duyet': platformAccount('cm.duyet', '0193a4d2-7000-7000-8000-0000000000d2', 'Content Reviewer', ['learning_content.read', 'learning_content.review', 'learning_content.publish', 'game.content.review']),
  'lo.vanhanh': platformAccount('lo.vanhanh', '0193a4d2-7000-7000-8000-0000000000d3', 'Live-ops', [
    'game.event.write', 'game.event.operate', 'game.season_track.write', 'game.content.publish', 'content.release.rollback', 'game.liveops.read', 'game.analytics.read', 'competition.config.season.write',
  ]),
};

export interface MockVersion {
  version: number;
  status: string;
  title?: string;
  payload: Record<string, unknown>;
  author_id?: string;
  reviewer_id?: string;
  revision: number;
  published_at?: string;
  publish_at?: string;
  thumbnail_url?: string;
  turntable_url?: string;
  render_status?: string;
  calendar_warnings?: unknown[];
  updated_at: string;
}

export interface MockConfig {
  config_id: string;
  type: string;
  versions: MockVersion[];
  current?: number;
  releases: Record<string, unknown>[];
}

export class GameOpsMock {
  readonly configs: MockConfig[] = [];
  manifest: Record<string, unknown> = { registered: false, platforms: [], areas: [], npcs: [], scene_refs: [], interactables: [] };
  counts: { config_type: string; status: string; count: number }[] = [];
  private n = 0;

  constructor(private readonly backend: MockPortalBackend) {
    backend.overrides.push((r) => this.route(r));
  }

  newId(): string {
    this.n++;
    return `0193a4d2-9000-7000-8000-${String(this.n).padStart(12, '0')}`;
  }

  add(type: string, versions: Partial<MockVersion>[], extra: Partial<MockConfig> = {}): MockConfig {
    const c: MockConfig = {
      config_id: extra.config_id ?? this.newId(),
      type,
      versions: versions.map((v, i) => ({ version: i + 1, status: 'draft', payload: {}, revision: 1, updated_at: '2026-11-20T03:00:00Z', ...v })),
      releases: [],
      ...extra,
    };
    this.configs.push(c);
    return c;
  }

  private view(c: MockConfig, v: MockVersion) {
    return {
      config_type: c.type,
      config_id: c.config_id,
      config_version_id: `${c.config_id.slice(0, 24)}${String(v.version).padStart(12, '0')}`,
      version: v.version,
      status: v.status,
      title: v.title,
      payload: v.payload,
      author_id: v.author_id,
      reviewer_id: v.reviewer_id,
      published_at: v.published_at,
      updated_at: v.updated_at,
      is_current: c.current === v.version,
      thumbnail_url: v.thumbnail_url,
      turntable_url: v.turntable_url,
      render_status: v.render_status,
      calendar_warnings: v.calendar_warnings,
    };
  }

  private user(): MockAccount | null {
    return this.backend.user;
  }

  private route(r: RecordedRequest): Response | undefined {
    if (!r.path.startsWith('/game-admin') && !r.path.startsWith('/game-media')) return undefined;
    if (!this.user()) return problem(401, 'UNAUTHENTICATED');
    const q = new URLSearchParams(r.search);
    const etag = (v: MockVersion) => ({ ETag: `"${v.revision}"` });
    if (r.method === 'GET' && r.path === '/game-admin/client-manifests/current') return json(200, this.manifest);
    if (r.method === 'GET' && r.path === '/game-admin/stats/config-counts') return json(200, { generated_at: '2026-11-24T01:00:00Z', data: this.counts });
    if (r.method === 'POST' && r.path === '/game-admin/preview/sessions') return json(201, { preview_token: 'preview-token-MOCK-0123456789', expires_at: '2026-11-24T02:00:00Z' });
    if (r.method === 'GET' && r.path === '/game-admin/preview/snapshot') {
      const c = this.configs.find((x) => x.config_id === q.get('config_id'));
      const v = c?.versions.find((x) => x.version === Number(q.get('version')));
      if (!c || !v || r.headers['x-preview-token'] !== 'preview-token-MOCK-0123456789') return problem(403, 'PERMISSION_DENIED');
      return json(200, { config_type: c.type, config_id: c.config_id, version: v.version, status: v.status, payload: v.payload, expires_at: '2026-11-24T02:00:00Z' });
    }
    if (r.method === 'POST' && r.path === '/game-media:presign-upload') return json(201, { asset_id: this.newId(), status: 'quarantined', upload_url: 'https://upload.test/put', upload_headers: {} });
    let m = /^\/game-media\/([^/]+)$/.exec(r.path);
    if (m && r.method === 'GET') return json(200, { asset_id: m[1], kind: 'image', status: 'ready', sha256: 'a'.repeat(64), original_filename: 'banner.png' });

    m = /^\/game-admin\/([^/]+)$/.exec(r.path);
    if (m) {
      const type = m[1]!;
      if (r.method === 'GET') {
        const status = q.get('status');
        // newest version of each config (of the status when filtered), keyset by config_id in the real backend
        const rows = this.configs
          .filter((c) => c.type === type)
          .map((c) => [...c.versions].reverse().find((v) => !status || v.status === status) && this.view(c, [...c.versions].reverse().find((v) => !status || v.status === status)!))
          .filter(Boolean);
        return json(200, cursorPage(rows, q, 'data'));
      }
      if (r.method === 'POST') {
        const body = r.body as { title?: string; payload: Record<string, unknown> };
        const c = this.add(type, [{ title: body.title, payload: body.payload, author_id: this.user()!.sub, status: 'draft' }]);
        return json(201, this.view(c, c.versions[0]!), etag(c.versions[0]!));
      }
    }
    m = /^\/game-admin\/([^/]+)\/([^/:]+)(.*)$/.exec(r.path);
    if (!m) return problem(404, 'NOT_FOUND');
    const [, type, id, rest] = m;
    const c = this.configs.find((x) => x.config_id === id && x.type === type);
    if (!c) return problem(404, 'NOT_FOUND');
    const byNo = (no: number) => c.versions.find((v) => v.version === no);
    let vm = /^\/versions\/(\d+)$/.exec(rest!);
    if (vm) {
      const v = byNo(Number(vm[1]));
      if (!v) return problem(404, 'NOT_FOUND');
      if (r.method === 'GET') return json(200, this.view(c, v), etag(v));
      if (r.method === 'PATCH') {
        if (r.headers['if-match'] !== `"${v.revision}"`) return problem(412, 'PRECONDITION_FAILED');
        if (v.status !== 'draft' && v.status !== 'rejected') return problem(409, 'CONTENT_NOT_EDITABLE');
        const body = r.body as { title?: string; payload: Record<string, unknown> };
        Object.assign(v, { payload: body.payload, title: body.title ?? v.title, revision: v.revision + 1, status: 'draft' });
        return json(200, this.view(c, v), etag(v));
      }
    }
    vm = /^\/versions\/(\d+):(transition|publish|diff|simulate)$/.exec(rest!);
    if (vm) {
      const v = byNo(Number(vm[1]));
      if (!v) return problem(404, 'NOT_FOUND');
      const op = vm[2];
      if (op === 'diff') return json(200, { from_version: Number(q.get('with')), to_version: v.version, changes: [{ path: '/name/vi', op: 'changed', before: 'Cũ', after: 'Mới' }] });
      if (op === 'simulate') {
        const curve = type === 'progression-curves';
        const weeks = Number(q.get('weeks'));
        const perDay = curve ? Math.min(Number(q.get('minutes_per_day')) * 14, 1000) : 100;
        return json(200, {
          config_type: type,
          config_id: c.config_id,
          version: v.version,
          assumptions: { per_play_day: perDay, days_per_week: Number(q.get('days_per_week')), weeks, daily_cap: curve ? 1000 : 100, ...(curve ? { minutes_per_day: Number(q.get('minutes_per_day')), minute_rate: 14 } : {}) },
          points: Array.from({ length: weeks }, (_, i) => ({ week: i + 1, level: 2 + i, total: perDay * Number(q.get('days_per_week')) * (i + 1) })),
          thresholds: [
            { level: 1, to_next: 120, total: 0 },
            { level: 2, to_next: 313, total: 120 },
            { level: 3, to_next: 539, total: 433 },
          ],
        });
      }
      if (r.headers['if-match'] !== `"${v.revision}"`) return problem(412, 'PRECONDITION_FAILED');
      if (op === 'transition') {
        const action = (r.body as { action: string }).action;
        if ((action === 'approve' || action === 'reject') && v.author_id === this.user()!.sub) return problem(403, 'PERMISSION_DENIED');
        v.status = { submit_review: 'in_review', approve: 'approved', reject: 'rejected', archive: 'archived' }[action] ?? v.status;
        if (action === 'approve' || action === 'reject') v.reviewer_id = this.user()!.sub;
        v.revision++;
        return json(200, this.view(c, v));
      }
      const body = (r.body ?? {}) as { publish_at?: string };
      const release = { release_id: this.newId(), item_type: type, item_id: c.config_id, action: 'publish', to_version: v.version, from_version: c.current, released_by: this.user()!.sub, released_at: '2026-11-24T01:00:00Z' };
      if (body.publish_at) {
        v.publish_at = body.publish_at;
        c.releases.unshift({ ...release, status: 'scheduled', scheduled_for: body.publish_at });
        return json(202, { ...release, status: 'scheduled', scheduled_for: body.publish_at });
      }
      v.status = 'published';
      v.published_at = '2026-11-24T01:00:00Z';
      v.revision++;
      c.current = v.version;
      c.releases.unshift({ ...release, status: 'applied' });
      return json(200, { ...release, status: 'applied' });
    }
    if (rest === '/versions' && r.method === 'GET') {
      const rows = [...c.versions].reverse().map((v) => ({ version: v.version, status: v.status, is_current: c.current === v.version, author_id: v.author_id, published_at: v.published_at, publish_at: v.publish_at }));
      return json(200, { item_id: c.config_id, ...(c.current ? { current_version: c.current } : {}), ...cursorPage(rows, q, 'data') });
    }
    if (rest === '/versions' && r.method === 'POST') {
      const base = byNo((r.body as { based_on_version: number }).based_on_version) ?? c.versions.at(-1)!;
      const nv: MockVersion = { ...base, version: c.versions.length + 1, status: 'draft', revision: 1, author_id: this.user()!.sub, published_at: undefined };
      c.versions.push(nv);
      return json(201, this.view(c, nv), etag(nv));
    }
    if (rest === '/releases' && r.method === 'GET') return json(200, cursorPage(c.releases, q, 'data'));
    const cancel = /^\/releases\/([^/]+):cancel$/.exec(rest!);
    if (cancel && r.method === 'POST') {
      const rel = c.releases.find((x) => x.release_id === cancel[1]);
      if (!rel) return problem(404, 'NOT_FOUND');
      rel.status = 'cancelled';
      return json(200, rel);
    }
    if (rest === ':rollback' && r.method === 'POST') {
      const to = (r.body as { to_version: number }).to_version;
      const rel = { release_id: this.newId(), item_type: type, item_id: c.config_id, action: 'rollback', from_version: c.current, to_version: to, reason: (r.body as { reason: string }).reason, status: 'applied', released_by: this.user()!.sub, released_at: '2026-11-24T01:00:00Z' };
      c.current = to;
      c.releases.unshift(rel);
      return json(200, rel);
    }
    if (rest === ':unpublish' && r.method === 'POST') {
      const rel = { release_id: this.newId(), item_type: type, item_id: c.config_id, action: 'unpublish', from_version: c.current, reason: (r.body as { reason: string }).reason, status: 'applied', released_by: this.user()!.sub, released_at: '2026-11-24T01:00:00Z' };
      c.current = undefined;
      c.releases.unshift(rel);
      return json(200, rel);
    }
    return problem(404, 'NOT_FOUND');
  }
}

export interface RenderGameOpsOptions {
  account?: string;
  disabledMenus?: string[];
  disabledActions?: string[];
  width?: number;
  setup?: (m: GameOpsMock, backend: MockPortalBackend) => void;
}

/** Renders the whole portal on a workspace A URL against the shell MOCK + this MOCK. */
export async function renderGameOps(url: string, opts: RenderGameOpsOptions = {}) {
  const { MockPortalBackend, renderPortal, setViewportWidth } = await import('./portal-harness');
  const name = opts.account ?? 'gd.thietke';
  const backend = new MockPortalBackend().withSession(GAME_ACCOUNTS[name] ? 'sa.admin' : name);
  if (GAME_ACCOUNTS[name]) backend.user = GAME_ACCOUNTS[name]!;
  backend.disabledMenus = opts.disabledMenus ?? [];
  backend.disabledActions = opts.disabledActions ?? [];
  const mock = new GameOpsMock(backend);
  opts.setup?.(mock, backend);
  if (opts.width) setViewportWidth(opts.width);
  const rendered = await renderPortal({ url, backend });
  return { ...rendered, mock };
}
