// MOCK (test only, AGENTS R17): in-memory stand-in of the F059 operations of specs/openapi/eduworld-api.v1.yaml —
// permission catalog, platform roles (ETag/If-Match), UserRoles (Idempotency-Key replay), tenant accounts,
// listPlatformUsers (platform staff only without `tenant_ids`, members of the filtered tenants with `User.tenant_ids`
// otherwise), createPlatformUser (Idempotency-Key replay), account lock/unlock/deactivate, platform audit (`action`,
// `action_prefix[]`), cross-tenant login events, feature flags (If-Match), getIntegrationOverview and SIS connections —
// plugged into MockPortalBackend after the workspace D mock of platform-backend.ts. It mimics the contract answers the
// screens must display (422/409/412/403 codes); it is NOT a backend, enforces no authorization and is never imported
// by application code.
import { MockPortalBackend, problem, TENANTS, type RecordedRequest } from './mock-backend';
import { withPlatformMock } from './platform-backend';

const json = (status: number, body: unknown, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', ...headers } });

export const AIDS = {
  roleSystemAdmin: '0193a4d2-9000-7000-8000-000000000001',
  roleLiveops: '0193a4d2-9000-7000-8000-000000000002',
  roleSafety: '0193a4d2-9000-7000-8000-000000000003',
  roleTenantAdmin: '0193a4d2-9000-7000-8000-000000000004',
  roleShared: '0193a4d2-9000-7000-8000-000000000005',
  roleTenantCustom: '0193a4d2-9000-7000-8000-000000000006',
  roleUnused: '0193a4d2-9000-7000-8000-000000000007',
  cmDuyet: '0193a4d2-9100-7000-8000-000000000001',
  qlcm: '0193a4d2-1111-7000-8000-000000000002',
  pgdStaff: '0193a4d2-9100-7000-8000-000000000003',
  hsAn: '0193a4d2-9100-7000-8000-000000000004',
  urPlatform: '0193a4d2-9200-7000-8000-000000000001',
  urTenant: '0193a4d2-9200-7000-8000-000000000002',
  urPgd: '0193a4d2-9200-7000-8000-000000000003',
  school: '0193a4d2-9300-7000-8000-000000000001',
  sis: '0193a4d2-9400-7000-8000-000000000001',
  run: '0193a4d2-9400-7000-8000-000000000002',
  audit1: '0193a4d2-9500-7000-8000-000000000001',
  audit2: '0193a4d2-9500-7000-8000-000000000002',
} as const;

type Obj = Record<string, unknown>;

const PERMISSIONS = [
  { code: 'platform.role.read', scope_kind: 'platform', group_code: 'platform_access', name: { vi: 'Xem role', en: 'Read roles' } },
  { code: 'platform.role.write', scope_kind: 'platform', group_code: 'platform_access', name: { vi: 'Sửa role', en: 'Write roles' } },
  { code: 'game.quest.write', scope_kind: 'platform', group_code: 'game_ops', name: { vi: 'Sửa quest', en: 'Write quests' } },
  { code: 'game.event.write', scope_kind: 'platform', group_code: 'game_ops', name: { vi: 'Sửa sự kiện', en: 'Write events' } },
  { code: 'tenant.user.read', scope_kind: 'tenant', group_code: 'school_access', name: { vi: 'Xem người dùng', en: 'Read users' } },
  { code: 'lms.class.read', scope_kind: 'tenant', group_code: 'school_lms', name: { vi: 'Xem lớp', en: 'Read classes' } },
];

const role = (id: string, code: string, kind: string, extra: Obj = {}) => ({
  role_id: id,
  code,
  name: { vi: `Role ${code}` },
  role_scope_kind: kind,
  is_system: false,
  tenant_id: null,
  permission_codes: [] as string[],
  active_user_role_count: 0,
  version: 1,
  updated_at: '2026-11-18T08:00:00Z',
  ...extra,
});

export interface AccessState {
  catalogVersion: string;
  roles: ReturnType<typeof role>[];
  userRoles: Obj[];
  users: Obj[];
  audit: Obj[];
  loginEvents: Obj[];
  loginPageSize: number;
  flags: Obj[];
  /** Idempotency-Key → created UserRole (replay returns the same record). */
  grants: Map<string, Obj>;
  /** user_id → tenants the account belongs to (source of `User.tenant_ids` in listPlatformUsers). */
  memberships: Map<string, string[]>;
  /** Idempotency-Key → platform user created by createPlatformUser (replay returns the same record). */
  createdPlatformUsers: Map<string, Obj>;
}

export function accessState(): AccessState {
  return {
    catalogVersion: '2026.11.19.1',
    roles: [
      role(AIDS.roleSystemAdmin, 'system_admin', 'platform', { is_system: true, permission_codes: ['platform.role.read', 'platform.role.write'], active_user_role_count: 1 }),
      role(AIDS.roleLiveops, 'liveops', 'platform', { is_system: true, permission_codes: ['game.quest.write', 'game.event.write'], active_user_role_count: 2 }),
      role(AIDS.roleSafety, 'safety_moderator', 'platform', { is_system: true, permission_codes: ['platform.role.read'] }),
      role(AIDS.roleTenantAdmin, 'tenant_admin', 'tenant', { is_system: true, permission_codes: ['tenant.user.read', 'lms.class.read'], active_user_role_count: 3 }),
      role(AIDS.roleShared, 'liveops_lead', 'platform', { permission_codes: ['game.quest.write'], active_user_role_count: 1, version: 3 }),
      role(AIDS.roleTenantCustom, 'gv_chu_nhiem', 'tenant', { tenant_id: TENANTS.thDemoA.tenant_id, permission_codes: ['lms.class.read'] }),
      role(AIDS.roleUnused, 'qa_reader', 'platform', { permission_codes: ['platform.role.read'], version: 2 }),
    ],
    userRoles: [
      { user_role_id: AIDS.urPlatform, user_id: AIDS.cmDuyet, role_id: AIDS.roleShared, role_code: 'liveops_lead', tenant_id: null, scope_type: 'platform', scope_id: null, valid_from: '2026-11-01T00:00:00Z', valid_to: null, source: 'manual', granted_by: '0193a4d2-1111-7000-8000-000000000003', created_at: '2026-11-01T00:00:00Z' },
      { user_role_id: AIDS.urTenant, user_id: AIDS.qlcm, role_id: AIDS.roleTenantAdmin, role_code: 'tenant_admin', tenant_id: TENANTS.thDemoA.tenant_id, scope_type: 'tenant', scope_id: null, valid_from: '2026-09-01T00:00:00Z', valid_to: '2027-06-30T00:00:00Z', source: 'sis', granted_by: null, created_at: '2026-09-01T00:00:00Z' },
      { user_role_id: AIDS.urPgd, user_id: AIDS.qlcm, role_id: AIDS.roleTenantAdmin, role_code: 'tenant_admin', tenant_id: TENANTS.pgdDemo.tenant_id, scope_type: 'school', scope_id: AIDS.school, valid_from: '2026-09-01T00:00:00Z', valid_to: null, source: 'manual', granted_by: null, created_at: '2026-09-01T00:00:00Z' },
    ],
    users: [
      { user_id: AIDS.cmDuyet, user_type: 'staff', primary_tenant_id: null, status: 'active', username: 'cm.duyet', email: 'cm.duyet@eduworld.vn', display_name: 'Cán bộ duyệt nội dung', source: 'manual', version: 1 },
      { user_id: AIDS.qlcm, user_type: 'staff', primary_tenant_id: TENANTS.thDemoA.tenant_id, status: 'active', username: 'qlcm.truong.a', email: 'qlcm.truong.a@thdemo.edu.vn', display_name: 'Quản lý chuyên môn', source: 'manual', last_login_at: '2026-11-18T07:00:00Z', version: 1 },
      { user_id: AIDS.pgdStaff, user_type: 'staff', primary_tenant_id: TENANTS.pgdDemo.tenant_id, status: 'active', username: 'cv.pgd', email: 'cv.pgd@pgd.edu.vn', display_name: 'Chuyên viên PGD', source: 'manual', version: 1 },
      { user_id: AIDS.hsAn, user_type: 'student', primary_tenant_id: TENANTS.thDemoA.tenant_id, status: 'active', username: 'hs.an.3a', email: null, display_name: 'Nguyễn An', source: 'sis', version: 1 },
    ],
    audit: [
      { audit_id: AIDS.audit1, tenant_id: TENANTS.thDemoA.tenant_id, actor_id: '0193a4d2-1111-7000-8000-000000000003', actor_type: 'staff', action: 'user_role.granted', target_type: 'user', target_id: AIDS.qlcm, result: 'success', reason: 'Bổ nhiệm', changes: { role_code: 'tenant_admin', note: '<img src=x onerror=alert(1)>' }, trace_id: 'trace-audit-1', occurred_at: '2026-11-19T07:00:00Z' },
      { audit_id: AIDS.audit2, tenant_id: null, actor_id: null, actor_type: 'system', action: 'platform_settings.changed', target_type: 'feature_flag', target_id: 'menu.platform_admin.reports', result: 'success', reason: 'Tắt báo cáo', changes: { before: true, after: false }, trace_id: 'trace-audit-2', occurred_at: '2026-11-19T06:00:00Z' },
    ],
    loginEvents: [
      { login_event_id: '0193a4d2-9600-7000-8000-000000000001', tenant_id: TENANTS.thDemoA.tenant_id, student_id: AIDS.hsAn, platform: 'windows', device_label: 'Windows · máy lớp 3A', client_version: '1.4.0', login_at: '2026-11-19T01:00:00Z', logout_at: '2026-11-19T01:20:00Z', end_reason: 'replaced_by_other_device' },
      { login_event_id: '0193a4d2-9600-7000-8000-000000000002', tenant_id: TENANTS.thDemoA.tenant_id, student_id: AIDS.hsAn, platform: 'android', device_label: 'Android · tablet', client_version: '1.4.0', login_at: '2026-11-19T01:20:00Z', logout_at: null, end_reason: null },
      { login_event_id: '0193a4d2-9600-7000-8000-000000000003', tenant_id: TENANTS.pgdDemo.tenant_id, student_id: '0193a4d2-9100-7000-8000-000000000009', platform: 'web', device_label: 'Web', client_version: '1.3.9', login_at: '2026-11-18T01:00:00Z', logout_at: '2026-11-18T02:00:00Z', end_reason: 'curfew' },
    ],
    loginPageSize: 2,
    flags: [
      { flag_key: 'portal_new_dashboard', flag_kind: 'feature', parent_key: null, description: 'Màn tổng quan mới', default_enabled: false, tenant_override: null, effective_enabled: false, can_disable: true, version: 4, updated_at: '2026-11-18T08:00:00Z', updated_by: null },
      { flag_key: 'menu.platform_admin.student_activity', flag_kind: 'menu', parent_key: null, default_enabled: true, tenant_override: null, effective_enabled: true, can_disable: true, version: 1 },
      { flag_key: 'menu.platform_admin.audit_logs', flag_kind: 'menu', parent_key: null, default_enabled: true, tenant_override: null, effective_enabled: true, can_disable: false, version: 0 },
    ],
    grants: new Map(),
    memberships: new Map([
      [AIDS.qlcm, [TENANTS.thDemoA.tenant_id, TENANTS.pgdDemo.tenant_id]],
      [AIDS.pgdStaff, [TENANTS.pgdDemo.tenant_id]],
      [AIDS.hsAn, [TENANTS.thDemoA.tenant_id]],
    ]),
    createdPlatformUsers: new Map(),
  };
}

const page = (data: unknown[], key = 'items') => ({ [key]: data, page: { has_more: false, next_cursor: null, limit: 100 } });

function ifMatch(rec: RecordedRequest, version: number): Response | null {
  const v = rec.headers['if-match'];
  if (!v) return problem(428, 'PRECONDITION_REQUIRED');
  if (v !== `W/"${version}"`) return problem(412, 'VERSION_MISMATCH');
  return null;
}

/** Installs the MOCK F059 routes on `backend` (after the workspace D mock); returns the mutable state. */
export function withAccessMock(backend: MockPortalBackend, state: AccessState = accessState()): AccessState {
  const s = state;
  backend.overrides.push((rec) => {
    const q = new URLSearchParams(rec.search);
    const tenantIds = q.getAll('tenant_ids');
    const body = (rec.body ?? {}) as Obj;
    const route = `${rec.method} ${rec.path}`;
    if (route === 'GET /admin/permissions') return json(200, { items: PERMISSIONS, catalog_version: s.catalogVersion });
    if (route === 'GET /admin/roles') {
      const kind = q.get('role_scope_kind');
      const rows = s.roles.filter((r) => (!kind || r.role_scope_kind === kind) && (!r.tenant_id || !tenantIds.length || tenantIds.includes(r.tenant_id)));
      return json(200, page(rows));
    }
    if (route === 'POST /admin/roles') {
      const codes = (body.permission_codes as string[]) ?? [];
      if (body.role_scope_kind === 'tenant') {
        const bad = codes.map((c, i) => [c, i] as const).filter(([c]) => /^(platform|game|competition\.config)\./.test(c));
        if (bad.length) return problem(422, 'VALIDATION_FAILED', { errors: bad.map(([, i]) => ({ field: `permission_codes[${i}]`, code: 'PERMISSION_NOT_ALLOWED_FOR_SCOPE' })) });
      }
      if (s.roles.some((r) => r.code === body.code)) return problem(409, 'CONFLICT');
      const r = role(`0193a4d2-9000-7000-8000-0000000001${String(s.roles.length).padStart(2, '0')}`, String(body.code), String(body.role_scope_kind ?? 'platform'), {
        name: body.name,
        description: body.description,
        permission_codes: codes,
      });
      s.roles.push(r);
      return json(201, r, { ETag: 'W/"1"' });
    }
    const one = /^\/admin\/roles\/([^/]+)$/.exec(rec.path);
    if (one) {
      const r = s.roles.find((x) => x.role_id === one[1]);
      if (!r) return problem(404, 'NOT_FOUND');
      if (rec.method === 'GET') return json(200, r, { ETag: `W/"${r.version}"` });
      if (r.is_system) return problem(403, 'PERMISSION_DENIED');
      const bad = ifMatch(rec, r.version);
      if (bad) return bad;
      if (rec.method === 'PATCH') {
        if (!body.reason) return problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'reason', code: 'REQUIRED' }] });
        Object.assign(r, { name: body.name, permission_codes: body.permission_codes, description: body.description, version: r.version + 1 });
        return json(200, r, { ETag: `W/"${r.version}"` });
      }
      if (rec.method === 'DELETE') {
        if (r.active_user_role_count > 0) return problem(409, 'CONFLICT');
        s.roles = s.roles.filter((x) => x !== r);
        return new Response(null, { status: 204 });
      }
    }
    if (route === 'GET /admin/user-roles') {
      const user = q.get('user_id');
      const roleId = q.get('role_id');
      return json(200, page(s.userRoles.filter((u) => (!user || u.user_id === user) && (!roleId || u.role_id === roleId) && (!tenantIds.length || tenantIds.includes(String(u.tenant_id))))));
    }
    if (route === 'POST /admin/user-roles') {
      const key = rec.headers['idempotency-key'] ?? '';
      const replay = s.grants.get(key);
      if (replay) return json(201, replay);
      const r = s.roles.find((x) => x.role_id === body.role_id);
      if (!r) return problem(422, 'ROLE_NOT_FOUND');
      const tenant = rec.headers['x-tenant-id'] ?? null;
      if (r.role_scope_kind === 'platform' && (tenant || body.scope_type !== 'platform')) return problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'scope_type', code: 'INVALID' }] });
      if (r.code === 'safety_moderator' && !body.training_completed_at) return problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'training_completed_at', code: 'REQUIRED' }] });
      if (s.userRoles.some((u) => u.user_id === body.user_id && u.role_id === body.role_id && u.tenant_id === tenant)) return problem(409, 'CONFLICT');
      const ur = { user_role_id: `0193a4d2-9200-7000-8000-0000000001${String(s.userRoles.length).padStart(2, '0')}`, user_id: body.user_id, role_id: r.role_id, role_code: r.code, tenant_id: tenant, scope_type: body.scope_type, scope_id: body.scope_id ?? null, valid_from: body.valid_from ?? '2026-11-19T08:00:00Z', valid_to: body.valid_to ?? null, source: 'manual', granted_by: '0193a4d2-1111-7000-8000-000000000003' };
      s.userRoles.push(ur);
      s.grants.set(key, ur);
      return json(201, ur);
    }
    const revoke = /^\/admin\/user-roles\/([^/]+)$/.exec(rec.path);
    if (rec.method === 'DELETE' && revoke) {
      if (!q.get('reason')) return problem(422, 'VALIDATION_FAILED');
      s.userRoles = s.userRoles.filter((u) => u.user_role_id !== revoke[1]);
      return new Response(null, { status: 204 });
    }
    if (route === 'GET /tenant/users') {
      const tenant = rec.headers['x-tenant-id'];
      if (!tenant) return problem(400, 'TENANT_HEADER_REQUIRED');
      const type = q.get('user_type');
      const text = q.get('q')?.toLowerCase() ?? '';
      return json(200, page(s.users.filter((u) => u.primary_tenant_id === tenant && (!type || u.user_type === type) && (!text || String(u.username).includes(text) || String(u.display_name).toLowerCase().includes(text)))));
    }
    if (route === 'POST /tenant/users') {
      if (s.users.some((u) => u.email === body.email)) return problem(409, 'EMAIL_TAKEN');
      if (s.users.some((u) => u.username === body.username)) return problem(409, 'USERNAME_TAKEN');
      const u = { user_id: `0193a4d2-9100-7000-8000-0000000001${String(s.users.length).padStart(2, '0')}`, user_type: 'staff', primary_tenant_id: rec.headers['x-tenant-id'], status: 'pending', username: body.username, email: body.email, display_name: body.display_name, source: 'manual', version: 1 };
      s.users.push(u);
      return json(201, u);
    }
    if (route === 'GET /admin/users') {
      // without tenant_ids: platform staff only (AGENTS R8); with tenant_ids: members of those tenants + their tenant_ids
      const type = q.get('user_type');
      const status = q.get('status');
      const text = q.get('q')?.toLowerCase() ?? '';
      const match = (u: Obj) =>
        (!type || u.user_type === type) && (!status || u.status === status) && (!text || [u.username, u.email, u.display_name].some((x) => String(x ?? '').toLowerCase().includes(text)));
      const rows = tenantIds.length
        ? s.users.flatMap((u) => {
            const members = (s.memberships.get(String(u.user_id)) ?? []).filter((id) => tenantIds.includes(id));
            return members.length && match(u) ? [{ ...u, tenant_ids: members }] : [];
          })
        : s.users.filter((u) => !u.primary_tenant_id && match(u));
      return json(200, page(rows));
    }
    if (route === 'POST /admin/users') {
      const key = rec.headers['idempotency-key'] ?? '';
      const replay = s.createdPlatformUsers.get(key);
      if (replay) return json(201, replay, { 'Idempotent-Replayed': 'true' });
      if (rec.headers['x-tenant-id']) return problem(422, 'VALIDATION_FAILED');
      if (s.users.some((u) => u.email === body.email)) return problem(409, 'EMAIL_TAKEN');
      if (s.users.some((u) => u.username === body.username)) return problem(409, 'USERNAME_TAKEN');
      const roles = (body.initial_role_ids as string[] | undefined) ?? [];
      if (!roles.length || roles.some((id) => s.roles.find((r) => r.role_id === id)?.role_scope_kind !== 'platform')) {
        return problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'initial_role_ids', code: 'INVALID' }] });
      }
      const u = {
        user_id: `0193a4d2-9100-7000-8000-0000000002${String(s.users.length).padStart(2, '0')}`,
        user_type: 'staff',
        primary_tenant_id: null,
        status: 'pending',
        username: body.username,
        email: body.email,
        display_name: body.display_name,
        locale: body.locale,
        source: 'manual',
        version: 1,
      };
      s.users.push(u);
      s.createdPlatformUsers.set(key, u);
      return json(201, u);
    }
    if (route === 'GET /admin/integrations') {
      return json(200, {
        generated_at: '2026-11-20T08:00:00Z',
        keycloak: {
          issuer_url: 'https://sso.eduworld.local/realms/eduworld',
          realm: 'eduworld',
          clients: [
            { client_id: 'eduworld-portal', purpose: 'portal' },
            { client_id: 'eduworld-game-backend', purpose: 'game' },
            { client_id: 'eduworld-identity-admin', purpose: 'admin_api' },
          ],
          platform_auth_max_age_seconds: 900,
          // outside the schema (must never be rendered, F059-AC-23)
          client_secret: 'kc-secret-should-not-show',
        },
        apisix: {
          public_base_url: 'https://api.eduworld.local',
          internal_base_url: null,
          route_groups: [
            { prefix: '/api/v1/auth/', gateway: 'public', service: 'core-api' },
            { prefix: '/realtime/v1', gateway: 'public', service: 'realtime' },
            { prefix: '/api/v1/admin/', gateway: 'internal', service: 'core-api' },
          ],
        },
        sis: { connections_total: 3, connections_by_status: { active: 2, auth_error: 1 } },
      });
    }
    if (rec.method === 'POST' && /^\/admin\/users\/[^/]+:(lock|unlock|deactivate)$/.test(rec.path)) return new Response(null, { status: 204 });
    const schools = /^\/admin\/tenants\/([^/]+)\/schools$/.exec(rec.path);
    if (rec.method === 'GET' && schools) {
      return json(200, page([{ school_id: AIDS.school, tenant_id: schools[1], code: 'th-a', name: { vi: 'Trường TH A' }, school_level: 'primary', grade_min: 1, grade_max: 5, status: 'active', is_virtual: false }]));
    }
    if (route === 'GET /admin/audit-logs') {
      const from = q.get('from');
      const to = q.get('to');
      if (!from || !to) return problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'from', code: 'REQUIRED' }] });
      if (new Date(to).getTime() - new Date(from).getTime() > 93 * 86_400_000) return problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'from', code: 'INVALID' }] });
      const action = q.get('action');
      const prefixes = q.getAll('action_prefix');
      if (prefixes.length > 10) return problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'action_prefix', code: 'INVALID' }] });
      const inGroup = (a: string) => !prefixes.length || prefixes.some((p) => a === p || a.startsWith(`${p}.`));
      const rows = s.audit.filter((a) => (!action || a.action === action) && inGroup(String(a.action)) && (tenantIds.length ? tenantIds.includes(String(a.tenant_id)) : true));
      return json(200, page(rows, 'data'));
    }
    if (route === 'GET /admin/login-events') {
      const student = q.get('student_id');
      const rows = s.loginEvents.filter((e) => (!student || e.student_id === student) && (!tenantIds.length || tenantIds.includes(String(e.tenant_id))));
      const from = Number(q.get('cursor') ?? 0);
      const data = rows.slice(from, from + s.loginPageSize);
      const more = from + s.loginPageSize < rows.length;
      return json(200, { data, page: { has_more: more, next_cursor: more ? String(from + s.loginPageSize) : null, limit: s.loginPageSize } });
    }
    if (route === 'GET /admin/feature-flags') {
      const kind = q.get('flag_kind');
      return json(200, page(s.flags.filter((f) => !kind || f.flag_kind === kind), 'data'));
    }
    const flag = /^\/admin\/feature-flags\/([^/]+)$/.exec(rec.path);
    if (rec.method === 'PUT' && flag) {
      const f = s.flags.find((x) => x.flag_key === flag[1]);
      if (!f) return problem(404, 'NOT_FOUND');
      const bad = ifMatch(rec, Number(f.version));
      if (bad) return bad;
      if (body.state === 'disabled' && !f.can_disable) return problem(422, 'FEATURE_NOT_DISABLEABLE');
      if (body.scope === 'platform') Object.assign(f, { default_enabled: body.state === 'enabled', effective_enabled: body.state === 'enabled' });
      else Object.assign(f, { tenant_override: body.state === 'inherit' ? null : body.state === 'enabled' });
      f.version = Number(f.version) + 1;
      return json(200, f, { ETag: `W/"${String(f.version)}"` });
    }
    if (route === 'GET /sis/connections') {
      const conn = {
        sis_connection_id: AIDS.sis,
        tenant_id: TENANTS.thDemoA.tenant_id,
        provider_kind: 'contract_v1',
        contract_version: '1.2',
        status: 'active',
        school_count: 1,
        schedule: null,
        has_secret: true,
        last_sync_run: { sync_run_id: AIDS.run, kind: 'delta', status: 'partially_failed', started_at: '2026-11-19T02:00:00Z', finished_at: '2026-11-19T02:05:00Z', stats: { rows_rejected: 3 } },
        created_at: '2026-10-01T00:00:00Z',
        // outside the schema (must never be rendered, F059-AC-23)
        secret_ref: 'vault://sis/th-demo-a/api-key',
        api_token: 'tok-should-not-show',
      };
      return json(200, page([conn].filter((c) => !tenantIds.length || tenantIds.includes(c.tenant_id)), 'data'));
    }
    if (rec.method === 'GET' && rec.path === `/admin/sis/connections/${AIDS.sis}/schools`) {
      return json(200, { data: [{ sis_connection_id: AIDS.sis, school_id: AIDS.school, tenant_id: TENANTS.thDemoA.tenant_id, school_name: 'Trường TH A', active: true, assigned_at: '2026-10-01T00:00:00Z' }] });
    }
    if (rec.method === 'GET' && rec.path === `/sis/connections/${AIDS.sis}/sync-runs`) {
      return json(200, page([{ sync_run_id: AIDS.run, kind: 'delta', status: 'partially_failed', started_at: '2026-11-19T02:00:00Z', finished_at: '2026-11-19T02:05:00Z', stats: { rows_rejected: 3 } }], 'data'));
    }
    return undefined;
  });
  return s;
}

/** Renders the portal at `url` for `account` (default `sa.admin`) with the MOCK workspace D + F059 routes. */
export async function renderAccess(url: string, opts: { account?: string; state?: AccessState; disabledActions?: string[]; disabledMenus?: string[] } = {}) {
  const { renderPortal } = await import('./portal-harness');
  const backend = new MockPortalBackend().withSession(opts.account ?? 'sa.admin');
  backend.disabledActions = opts.disabledActions ?? [];
  backend.disabledMenus = opts.disabledMenus ?? [];
  withPlatformMock(backend);
  const state = withAccessMock(backend, opts.state);
  const rendered = await renderPortal({ url, backend });
  return { ...rendered, state };
}
