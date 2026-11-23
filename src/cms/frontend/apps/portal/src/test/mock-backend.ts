// MOCK (test and story only, AGENTS R17): an in-memory stand-in of the F012 portal session contract — /auth/portal/*,
// /me/tenants, /me/permissions, /auth/me, /admin/tenants — answering per specs/openapi/eduworld-api.v1.yaml so the
// shell can be tested before the backend binaries are wired (OQ F010-2). It simulates the HttpOnly refresh cookie
// (invisible to the page) and the readable CSRF cookie, demo accounts of the quickstart and their permission sets
// built from the menu catalog. Never imported by application code; NOT a real backend and no live integration.
import { WORKSPACE_CATALOG, type WorkspaceKey } from '../generated/menu-catalog';
import { TEST_CONFIG } from '../shared/api/test-utils';

/** Requests of the portal shell itself on every screen (x-portal-shell / banner, F122): not a screen API. */
export const SHELL_PATHS: readonly string[] = ['/tenant/chat/moderation-status', '/admin/chat/moderation-switches'];

export const TENANTS = {
  thDemoA: { tenant_id: '0193a4d2-0000-7000-8000-00000000000a', code: 'th-demo-a', name: 'TH Demo A', tenant_type: 'school', status: 'active' },
  pgdDemo: { tenant_id: '0193a4d2-0000-7000-8000-00000000000b', code: 'pgd-demo', name: 'Phòng GD Demo', tenant_type: 'unit', status: 'active' },
  thcsDemoB: { tenant_id: '0193a4d2-0000-7000-8000-00000000000c', code: 'thcs-demo-b', name: 'THCS Demo B', tenant_type: 'school', status: 'active' },
} as const;

type Tenant = (typeof TENANTS)[keyof typeof TENANTS];

/** Permissions of the menus of the given workspaces (any-of lists flattened), minus platform-only ones for tenants. */
export function permissionsOf(workspaces: WorkspaceKey[], { platform = false } = {}): string[] {
  const out = new Set<string>();
  for (const ws of WORKSPACE_CATALOG.filter((w) => workspaces.includes(w.key))) {
    for (const m of ws.menus) for (const p of m.requiredPermissions) if (platform || !/^(platform|game|competition\.config)\./.test(p)) out.add(p);
    for (const m of ws.menus) for (const a of m.actions) for (const p of a.requiredPermissions) if (platform || !/^(platform|game|competition\.config)\./.test(p)) out.add(p);
  }
  return [...out];
}

export interface PermissionFixture {
  workspaces: WorkspaceKey[];
  permissions: string[];
}

export interface MockAccount {
  username: string;
  email: string;
  password: string;
  sub: string;
  displayName: string;
  student?: boolean;
  platform?: boolean;
  tenants: Tenant[];
  /** Permission set per tenant id, or `platform`. */
  perms: Record<string, PermissionFixture>;
  setupRequired?: { steps: 'update_password'[] };
}

const SCHOOL = permissionsOf(['school_lms']);
const CONTENT = permissionsOf(['content_studio']);

export const ACCOUNTS: Record<string, MockAccount> = {
  'ad.truong.a': {
    username: 'ad.truong.a',
    email: 'ad.truong.a@thdemo.edu.vn',
    password: 'Demo@12345',
    sub: '0193a4d2-1111-7000-8000-000000000001',
    displayName: 'Quản trị TH Demo A',
    tenants: [TENANTS.thDemoA],
    perms: { [TENANTS.thDemoA.tenant_id]: { workspaces: ['school_lms'], permissions: SCHOOL } },
  },
  'qlcm.truong.a': {
    username: 'qlcm.truong.a',
    email: 'qlcm.truong.a@thdemo.edu.vn',
    password: 'Demo@12345',
    sub: '0193a4d2-1111-7000-8000-000000000002',
    displayName: 'Quản lý chuyên môn',
    tenants: [TENANTS.thDemoA, TENANTS.pgdDemo],
    perms: {
      [TENANTS.thDemoA.tenant_id]: { workspaces: ['school_lms', 'content_studio'], permissions: [...SCHOOL, ...CONTENT] },
      [TENANTS.pgdDemo.tenant_id]: { workspaces: ['school_lms'], permissions: ['lms.student.read', 'tenant.student.read'] },
    },
  },
  'sa.admin': {
    username: 'sa.admin',
    email: 'sa.admin@eduworld.test',
    password: 'Demo@12345',
    platform: true,
    sub: '0193a4d2-1111-7000-8000-000000000003',
    displayName: 'System Admin',
    tenants: [],
    perms: { platform: { workspaces: ['school_lms', 'content_studio', 'game_ops', 'live_ops', 'platform_admin'], permissions: permissionsOf(['school_lms', 'content_studio', 'game_ops', 'live_ops', 'platform_admin'], { platform: true }) } },
  },
  'cm.noidung': {
    username: 'cm.noidung',
    email: 'cm.noidung@eduworld.test',
    password: 'Demo@12345',
    platform: true,
    sub: '0193a4d2-1111-7000-8000-000000000004',
    displayName: 'Content Admin',
    tenants: [],
    perms: { platform: { workspaces: ['content_studio'], permissions: CONTENT } },
  },
  'hs.an.3a': {
    username: 'hs.an.3a',
    email: 'hs.an.3a@thdemo.edu.vn',
    password: 'Demo@12345',
    student: true,
    sub: '0193a4d2-1111-7000-8000-000000000005',
    displayName: 'Học sinh',
    tenants: [TENANTS.thDemoA],
    perms: {},
  },
  'gv.moi': {
    username: 'gv.moi',
    email: 'gv.moi@thdemo.edu.vn',
    password: 'Temp@12345',
    sub: '0193a4d2-1111-7000-8000-000000000006',
    displayName: 'Giáo viên mới',
    tenants: [],
    perms: {},
  },
};

export interface RecordedRequest {
  method: string;
  path: string;
  search: string;
  gateway: 'public' | 'internal';
  headers: Record<string, string>;
  body: unknown;
}

const json = (status: number, body: unknown, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', ...headers } });

export const problem = (status: number, code: string, extra: Record<string, unknown> = {}) =>
  new Response(JSON.stringify({ type: 'about:blank', title: code, status, code, trace_id: `trace-${code.toLowerCase()}`, ...extra }), {
    status,
    headers: { 'Content-Type': 'application/problem+json' },
  });

/**
 * MOCK cursor page (test only) of a list the backend reads as a keyset page in SQL (owner rule 2026-11-23): honours
 * `limit` (default `defaultLimit`, max 100) and `cursor` (an opaque "c<offset>" here), returns `{ [key]: rows, page }`.
 */
export function cursorPage(rows: readonly unknown[], q: URLSearchParams, key: 'items' | 'data' = 'items', defaultLimit = 20) {
  const limit = Math.min(Number(q.get('limit') ?? defaultLimit) || defaultLimit, 100);
  const cur = q.get('cursor');
  const from = cur ? Number(cur.replace(/^c/, '')) : 0;
  const slice = rows.slice(from, from + limit);
  const more = from + limit < rows.length;
  return { [key]: slice, page: { has_more: more, next_cursor: more ? `c${from + limit}` : null, limit } };
}

const b64url = (s: string) => btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');

/** MOCK JWT (unsigned, test only) carrying `sub` so the shell can key per-viewer storage. */
export const fakeJwt = (sub: string, n: number) => `${b64url('{"alg":"none"}')}.${b64url(JSON.stringify({ sub, n }))}.mock-signature-${n}`;

/** JSON bodies are parsed; anything else (multipart CSV upload of F057) is kept as raw text. */
function parseBody(text: string, contentType: string | undefined): unknown {
  if (!text) return undefined;
  if (contentType && !/json/i.test(contentType)) return text;
  return JSON.parse(text);
}

export type Handler = (req: RecordedRequest) => Response | undefined | Promise<Response | undefined>;

export class MockPortalBackend {
  readonly requests: RecordedRequest[] = [];
  /** Readable CSRF cookie value (null = no cookie). */
  csrf: string | null = null;
  /** Simulated HttpOnly refresh cookie: the page never sees it. */
  refreshAlive = false;
  user: MockAccount | null = null;
  disabledMenus: string[] = [];
  disabledActions: string[] = [];
  /** Tokens the backend now answers with `401 TOKEN_EXPIRED`. */
  readonly expiredTokens = new Set<string>();
  /** First matching override answers (per-test behaviours: errors, network failures…). */
  readonly overrides: Handler[] = [];
  private n = 0;
  private csrfN = 0;
  private currentToken: string | null = null;

  /** Starts with a live session of `username` (as after an earlier sign-in in another page load). */
  withSession(username: string): this {
    this.user = ACCOUNTS[username]!;
    this.refreshAlive = true;
    this.csrf = this.newCsrf();
    return this;
  }

  calls(method: string, path: string): RecordedRequest[] {
    return this.requests.filter((r) => r.method === method && r.path === path);
  }

  private newCsrf() {
    this.csrfN++;
    return `csrf${String(this.csrfN).padStart(2, '0')}`.padEnd(43, 'x');
  }

  private issue(account: MockAccount) {
    this.n++;
    this.currentToken = fakeJwt(account.sub, this.n);
    this.refreshAlive = true;
    this.csrf = this.newCsrf();
    return json(200, { access_token: this.currentToken, token_type: 'Bearer', expires_in: 300, refresh_expires_in: 1800, auth_time: '2026-11-19T08:00:00Z' }, { 'Cache-Control': 'no-store' });
  }

  readonly fetch = async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
    const request = input instanceof Request ? input : new Request(input, init);
    const url = new URL(request.url);
    const internal = url.origin === new URL(TEST_CONFIG.apiInternalBaseUrl).origin;
    const base = new URL(internal ? TEST_CONFIG.apiInternalBaseUrl : TEST_CONFIG.apiBaseUrl).pathname;
    const headers: Record<string, string> = {};
    request.headers.forEach((v, k) => (headers[k.toLowerCase()] = v));
    const text = request.method === 'GET' ? '' : await request.text();
    const rec: RecordedRequest = {
      method: request.method,
      path: url.pathname.slice(base.length),
      search: url.search,
      gateway: internal ? 'internal' : 'public',
      headers,
      body: parseBody(text, headers['content-type']),
    };
    this.requests.push(rec);
    for (const o of this.overrides) {
      const r = await o(rec);
      if (r) return r;
    }
    return this.route(rec);
  };

  private authorized(rec: RecordedRequest): MockAccount | Response {
    const token = rec.headers.authorization?.replace(/^Bearer /, '');
    if (!token || !this.user) return problem(401, 'UNAUTHENTICATED');
    if (this.expiredTokens.has(token)) return problem(401, 'TOKEN_EXPIRED');
    if (this.user.student) return problem(403, 'PORTAL_ACCESS_STAFF_ONLY');
    return this.user;
  }

  private route(rec: RecordedRequest): Response {
    const body = (rec.body ?? {}) as Record<string, string>;
    switch (`${rec.method} ${rec.path}`) {
      case 'POST /auth/portal/login': {
        const id = String(body.username ?? '').trim().toLowerCase();
        const account = Object.values(ACCOUNTS).find((a) => a.username === id || a.email === id);
        if (!account || account.password !== body.password) return problem(401, 'INVALID_CREDENTIALS');
        if (account.student) return problem(403, 'PORTAL_ACCESS_STAFF_ONLY');
        if (account.setupRequired) {
          return problem(403, 'ACCOUNT_SETUP_REQUIRED', {
            setup_ticket: 'setup-ticket-0123456789abcdef0123456789abcdef',
            required_steps: account.setupRequired.steps,
          });
        }
        this.user = account;
        return this.issue(account);
      }
      case 'POST /auth/portal/token:refresh': {
        if (!this.csrf || rec.headers['x-ew-csrf'] !== this.csrf) return problem(403, 'CSRF_TOKEN_INVALID');
        if (!this.refreshAlive || !this.user) return problem(401, 'REFRESH_TOKEN_INVALID');
        return this.issue(this.user);
      }
      case 'POST /auth/portal/logout': {
        if (this.refreshAlive && rec.headers['x-ew-csrf'] !== this.csrf) return problem(403, 'CSRF_TOKEN_INVALID');
        this.refreshAlive = false;
        this.csrf = null;
        this.user = null;
        return new Response(null, { status: 204 });
      }
      case 'POST /auth/portal/setup:password': {
        if (body.setup_ticket !== 'setup-ticket-0123456789abcdef0123456789abcdef') return problem(401, 'SETUP_TICKET_INVALID');
        if (String(body.new_password).length < 10) return problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'new_password', code: 'PASSWORD_POLICY_VIOLATION' }] });
        return json(200, { remaining_steps: [] });
      }
      case 'POST /auth/portal/password:forgot':
        return new Response(null, { status: 202 });
      case 'POST /auth/portal/password:reset': {
        if (body.token !== 'reset-token-0123456789abcdef0123456789abcdef') return problem(401, 'PASSWORD_RESET_TOKEN_INVALID');
        if (String(body.new_password).length < 10) return problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'new_password', code: 'PASSWORD_POLICY_VIOLATION' }] });
        return new Response(null, { status: 204 });
      }
      case 'GET /me/tenants': {
        const a = this.authorized(rec);
        if (a instanceof Response) return a;
        return json(200, { has_platform_role: !!a.platform, items: a.tenants.map((t) => ({ ...t, role_codes: [] })) });
      }
      case 'GET /me/permissions': {
        const a = this.authorized(rec);
        if (a instanceof Response) return a;
        const tenant = rec.headers['x-tenant-id'];
        if (tenant && !a.tenants.some((t) => t.tenant_id === tenant)) return problem(403, 'TENANT_MISMATCH');
        const fx = a.perms[tenant ?? 'platform'] ?? { workspaces: [], permissions: [] };
        return json(200, { roles: [], permissions: fx.permissions, workspaces: fx.workspaces, tenant_id: tenant ?? null, disabled_menus: this.disabledMenus, disabled_actions: this.disabledActions });
      }
      case 'GET /auth/me': {
        const a = this.authorized(rec);
        if (a instanceof Response) return a;
        return json(200, { user_id: a.sub, user_type: 'staff', status: 'active', roles: [], display_name: a.displayName });
      }
      // Banner "Đang tắt kiểm duyệt" of every screen (F122): every step on by default, so a suite that does not install the
      // chat-safety MOCK still gets the answers the shell asks for (getTenantModerationStatus, listModerationSwitches).
      case 'GET /tenant/chat/moderation-status': {
        const a = this.authorized(rec);
        if (a instanceof Response) return a;
        return json(200, { tenant_id: rec.headers['x-tenant-id'] ?? null, enabled_all: true, disabled_steps: [], expires_at: null, source_scope: null });
      }
      case 'GET /admin/chat/moderation-switches': {
        const a = this.authorized(rec);
        if (a instanceof Response) return a;
        if (!(a.perms.platform?.permissions ?? []).includes('chat.policy.toggle')) return problem(403, 'PERMISSION_DENIED');
        const steps = { sensitive_terms: true, spam: true, bullying: true, async_analysis: true };
        const platform = { scope: 'platform', tenant_id: null, configured: null, effective: { enabled_all: true, steps }, disabled_steps: [], pending_requests: [] };
        const filtered = new URLSearchParams(rec.search).getAll('tenant_ids').length > 0;
        return json(200, { platform, tenants: [], page: { has_more: false, next_cursor: null, limit: 20 }, ...(filtered ? {} : { disabled_tenant_count: 0 }) });
      }
      case 'GET /admin/tenants': {
        const a = this.authorized(rec);
        if (a instanceof Response) return a;
        const q = new URLSearchParams(rec.search).get('q')?.toLowerCase() ?? '';
        const data = Object.values(TENANTS).filter((t) => !q || t.code.includes(q) || t.name.toLowerCase().includes(q));
        return json(200, { data, page: { has_more: false } });
      }
      default:
        return problem(404, 'NOT_FOUND');
    }
  }
}

