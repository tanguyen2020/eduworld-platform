// MOCK (test only, AGENTS R17): in-memory stand-in of the identity/game operations used by F052 screens, answering per
// specs/openapi/eduworld-api.v1.yaml so workspace C can be tested before the backend endpoints of F010/F011/F015 exist
// (listTenantClassStudents, listSchoolReconciliationIssues, resolveSchoolReconciliationIssue are contract-only).
// Tenant isolation is simulated: data belongs to one tenant, another tenant's X-Tenant-Id gets 404 (AC-ORG-001).
// Never imported by application code; NOT a real backend and no live integration.
import { permissionsOf, problem, TENANTS, type MockAccount, type MockPortalBackend, type RecordedRequest } from './mock-backend';

const json = (status: number, body: unknown, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', ...headers } });

export const IDS = {
  schoolA: '0193a4d2-2000-7000-8000-00000000000a',
  schoolPending: '0193a4d2-2000-7000-8000-00000000000b',
  schoolB: '0193a4d2-2000-7000-8000-00000000000c',
  class3A: '0193a4d2-3000-7000-8000-0000000003aa',
  studentAn: '0193a4d2-4000-7000-8000-0000000000a1',
  studentSapBiDon: '0193a4d2-4000-7000-8000-0000000000a2',
  issueDup: '0193a4d2-5000-7000-8000-0000000000d1',
  importId: '0193a4d2-6000-7000-8000-0000000000e1',
  roleSchoolAdmin: '0193a4d2-7000-7000-8000-000000000001',
  roleAcademic: '0193a4d2-7000-7000-8000-000000000002',
  roleCustom: '0193a4d2-7000-7000-8000-000000000003',
  userQlcm: '0193a4d2-1111-7000-8000-000000000002',
  userRole1: '0193a4d2-8000-7000-8000-000000000001',
  subjectMath: '0193a4d2-9000-7000-8000-000000000001',
} as const;

const T_A = TENANTS.thDemoA.tenant_id;
const T_PGD = TENANTS.pgdDemo.tenant_id;
const T_B = TENANTS.thcsDemoB.tenant_id;
const page = (items: unknown[], key = 'items') => ({ [key]: items, page: { has_more: false, limit: 50, next_cursor: null } });

const school = (id: string, tenant: string, vi: string, status: string, province: boolean, source = 'sis') => ({
  school_id: id,
  tenant_id: tenant,
  code: vi.toLowerCase().replace(/\s+/g, '-'),
  name: { vi },
  province: province ? { code: 'vn.ha-noi', name: { vi: 'Hà Nội', en: 'Hanoi' } } : undefined,
  school_level: 'primary',
  grade_min: 1,
  grade_max: 5,
  status,
  is_virtual: false,
  source,
  version: 1,
});

/** Accounts of the F052 quickstart missing from the shell MOCK (portal-harness): read-only academic manager, school B admin. */
export function accountWith(username: string, tenant: (typeof TENANTS)[keyof typeof TENANTS], permissions: string[], sub = '0193a4d2-1111-7000-8000-0000000000f0'): MockAccount {
  return {
    username,
    email: `${username}@demo.edu.vn`,
    password: 'Demo@12345',
    sub,
    displayName: username,
    tenants: [tenant],
    perms: { [tenant.tenant_id]: { workspaces: ['school_lms'], permissions } },
  };
}

export const SCHOOL_PERMS = permissionsOf(['school_lms']);
/** Read-only permissions (academic manager in the quickstart): no write permission of F052. */
export const READ_ONLY_PERMS = SCHOOL_PERMS.filter((p) => !/\.(write|import|exclude|configure|password_reset|lock)$/.test(p) && p !== 'tenant.user_role.write');

export interface SchoolLmsMockState {
  tenantSettingsVersion: number;
  schoolSettingsVersion: number;
  schoolSettings: Record<string, unknown>;
  tenantSettings: Record<string, unknown>;
  issueStatus: string;
  resolveCalls: number;
}

/** Installs the MOCK F052 handlers on a MockPortalBackend (as the first override). */
export function installSchoolLmsMock(backend: MockPortalBackend): SchoolLmsMockState {
  const state: SchoolLmsMockState = {
    tenantSettingsVersion: 1,
    schoolSettingsVersion: 1,
    tenantSettings: { inactive_student_cleanup_days: 180, effective_cleanup_days: 180, gameplay_chat_retention_days: 7, default_locale: 'vi-VN', allowed_hours: { windows: [] } },
    schoolSettings: {
      school_id: IDS.schoolA,
      class_leaderboard_enabled: true,
      school_leaderboard_enabled: true,
      student_club_creation: false,
      allowed_hours: { windows: [] },
      curfew_window: null,
      daily_play_limit_minutes: null,
      platform_curfew: { start: '21:30', end: '06:30' },
      effective_curfew: { start: '21:30', end: '06:30' },
    },
    issueStatus: 'open',
    resolveCalls: 0,
  };
  const schools = [school(IDS.schoolA, T_A, 'TH Demo A', 'active', true), school(IDS.schoolPending, T_PGD, 'TH Chờ Duyệt', 'pending_review', false), school(IDS.schoolB, T_B, 'THCS Demo B', 'active', true)];
  const tenantOf = (r: RecordedRequest) => r.headers['x-tenant-id'];
  const schoolIn = (r: RecordedRequest, id: string) => schools.find((s) => s.school_id === id && s.tenant_id === tenantOf(r));
  const etag = (v: number) => ({ ETag: `"v${v}"` });

  backend.overrides.unshift((r) => {
    const p = r.path;
    const m = r.method;
    let x: RegExpExecArray | null;
    if (m === 'GET' && p === '/catalog/provinces') {
      return json(200, { items: [{ province_id: '0193a4d2-0000-7000-8000-0000000000a1', code: 'vn.ha-noi', country_code: 'VN', name: { vi: 'Hà Nội', en: 'Hanoi' }, short_name: { vi: 'HN', en: 'HN' }, province_type: 'municipality', status: 'active' }] });
    }
    if (m === 'GET' && p === '/tenant/schools') {
      const status = new URLSearchParams(r.search).get('status');
      return json(200, page(schools.filter((s) => s.tenant_id === tenantOf(r) && (!status || s.status === status))));
    }
    if ((x = /^\/tenant\/schools\/([^/:]+)$/.exec(p))) {
      const s = schoolIn(r, x[1]!);
      if (!s) return problem(404, 'NOT_FOUND');
      if (m === 'GET') return json(200, s, etag(s.version));
      if (m === 'PATCH') {
        if (r.headers['if-match'] !== `"v${s.version}"`) return problem(412, 'VERSION_MISMATCH');
        const b = r.body as Record<string, unknown>;
        if (b.province_code) s.province = { code: String(b.province_code), name: { vi: 'Hà Nội', en: 'Hanoi' } };
        if (b.name) s.name = b.name as { vi: string };
        s.version++;
        return json(200, s, etag(s.version));
      }
    }
    if ((x = /^\/tenant\/schools\/([^/:]+):approve$/.exec(p)) && m === 'POST') {
      const s = schoolIn(r, x[1]!);
      if (!s) return problem(404, 'NOT_FOUND');
      if (!s.province) return problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'province_code', code: 'REQUIRED' }] });
      s.status = 'active';
      s.version++;
      return json(200, s, etag(s.version));
    }
    if ((x = /^\/tenant\/schools\/([^/]+)\/classes$/.exec(p)) && m === 'GET') {
      if (!schoolIn(r, x[1]!)) return problem(404, 'NOT_FOUND');
      return json(200, page(x[1] === IDS.schoolA ? [{ class_id: IDS.class3A, school_id: IDS.schoolA, grade_level: 3, name: '3A', student_count: 2 }] : []));
    }
    if ((x = /^\/tenant\/schools\/([^/]+)\/classes\/([^/]+)\/students$/.exec(p)) && m === 'GET') {
      if (!schoolIn(r, x[1]!) || x[2] !== IDS.class3A) return problem(404, 'NOT_FOUND');
      return json(
        200,
        page([
          { student_id: IDS.studentAn, display_name: 'Nguyễn An', student_code: 'HS001', username: 'hs.an.3a', status: 'active', source: 'sis', last_login_at: '2026-11-17T08:00:00Z', days_since_last_login: 2 },
          { student_id: IDS.studentSapBiDon, display_name: 'Trần Sắp Bị Dọn', student_code: 'HS002', username: 'hs.sapbidon', status: 'active', source: 'sis', last_login_at: null, days_since_last_login: null },
        ]),
      );
    }
    if (m === 'GET' && p === '/tenant/students/unassigned') {
      return json(200, page(tenantOf(r) === T_A ? [{ student_id: '0193a4d2-4000-7000-8000-0000000000a9', display_name: 'Lê Chưa Lớp', username: 'hs.chualop', school_id: IDS.schoolA, grade_level: 3, reason: 'missing_class' }] : []));
    }
    if ((x = /^\/tenant\/students\/([^/]+)\/login-events$/.exec(p)) && m === 'GET') {
      if (tenantOf(r) !== T_A || x[1] !== IDS.studentAn) return problem(404, 'NOT_FOUND');
      return json(
        200,
        page(
          [
            // MOCK: extra fields a buggy server might leak — the view model must drop them (SC-004)
            { login_event_id: '0193a4d2-a000-7000-8000-000000000001', tenant_id: T_A, student_id: IDS.studentAn, platform: 'windows', device_label: 'Windows · PC', client_version: '1.0.0', login_at: '2026-11-17T08:00:00Z', logout_at: '2026-11-17T09:00:00Z', end_reason: 'replaced_by_other_device', ip: '203.0.113.7', ip_hash: 'deadbeefcafe', device_id: 'raw-device-123' },
          ],
          'data',
        ),
      );
    }
    if ((x = /^\/tenant\/students\/([^/]+)\/password:reset$/.exec(p)) && m === 'POST') {
      if (tenantOf(r) !== T_A) return problem(404, 'NOT_FOUND');
      return json(200, { student_id: x[1], username: 'hs.an.3a', initial_password: 'Mock-Pass-0001', password_temporary: true, password_redacted: false, reset_at: '2026-11-19T08:00:00Z' });
    }
    if ((x = /^\/schools\/([^/]+)\/sis-imports$/.exec(p)) && m === 'POST') {
      if (!schoolIn(r, x[1]!)) return problem(404, 'NOT_FOUND');
      return json(202, { import_id: IDS.importId, school_id: x[1], status: 'awaiting_upload', upload_url: 'https://storage.test/upload/sis.csv?sig=mock', upload_expires_at: '2099-01-01T00:00:00Z', created_at: '2026-11-19T08:00:00Z' });
    }
    if ((x = /^\/schools\/([^/]+)\/sis-imports\/([^/]+)$/.exec(p)) && m === 'GET') {
      if (!schoolIn(r, x[1]!)) return problem(404, 'NOT_FOUND');
      return json(200, { import_id: x[2], school_id: x[1], status: 'completed_with_issues', dry_run: false, rows_total: 3, rows_applied: 2, rows_rejected: 1, reconciliation_issue_count: 1, created_at: '2026-11-19T08:00:00Z', errors: [{ row_no: 3, column_name: 'username', error_code: 'DUPLICATE_IDENTIFIER', message: 'duplicate', occurred_at: '2026-11-19T08:00:01Z' }] });
    }
    if ((x = /^\/schools\/([^/]+)\/reconciliation-issues$/.exec(p)) && m === 'GET') {
      if (!schoolIn(r, x[1]!)) return problem(404, 'NOT_FOUND');
      return json(200, page([{ issue_id: IDS.issueDup, tenant_id: T_A, school_id: x[1], issue_type: 'duplicate_identifier', entity_refs: { external_id: 'HS003' }, suggested_action: 'fix_in_source', status: state.issueStatus, created_at: '2026-11-19T08:00:01Z' }], 'data'));
    }
    if ((x = /^\/schools\/([^/]+)\/reconciliation-issues\/([^/:]+):resolve$/.exec(p)) && m === 'POST') {
      if (!schoolIn(r, x[1]!)) return problem(404, 'NOT_FOUND');
      state.resolveCalls++;
      state.issueStatus = (r.body as { resolution: string }).resolution === 'ignore' ? 'ignored' : 'resolved';
      return json(200, { issue_id: x[2], tenant_id: T_A, school_id: x[1], issue_type: 'duplicate_identifier', status: state.issueStatus, created_at: '2026-11-19T08:00:01Z' });
    }
    if ((x = /^\/tenant\/users\/([^/]+)$/.exec(p)) && m === 'PATCH') {
      if (r.headers['if-match'] !== 'W/"1"') return problem(412, 'VERSION_MISMATCH');
      const b = r.body as { email?: string };
      if (b.email?.toLowerCase() === 'ad.a@thdemo.edu.vn') return problem(409, 'EMAIL_TAKEN');
      return json(200, { user_id: x[1], user_type: 'staff', status: 'active', username: 'qlcm.truong.a', email: b.email, display_name: 'Cán bộ B', version: 2 });
    }
    if (p === '/tenant/users') {
      if (m === 'POST') {
        const b = r.body as { email: string; username: string };
        if (b.email.toLowerCase() === 'ad.a@thdemo.edu.vn') return problem(409, 'EMAIL_TAKEN');
        if (b.username === 'ad.truong.a') return problem(409, 'USERNAME_TAKEN');
        return json(201, { user_id: '0193a4d2-1111-7000-8000-0000000000ff', user_type: 'staff', status: 'pending', username: b.username, version: 1 });
      }
      return json(
        200,
        page([
          { user_id: IDS.userQlcm, user_type: 'staff', status: 'active', username: 'qlcm.truong.a', email: 'qlcm.a@thdemo.edu.vn', display_name: 'Cán bộ B', source: 'manual', user_roles: [], version: 1 },
          { user_id: IDS.studentAn, user_type: 'student', status: 'active', username: 'hs.an.3a', display_name: 'Nguyễn An', source: 'sis', version: 1 },
        ]),
      );
    }
    if (m === 'GET' && p === '/tenant/permissions') {
      return json(200, {
        catalog_version: 'mock',
        items: [
          { code: 'tenant.class.read', scope_kind: 'tenant', group_code: 'school_lms', name: { vi: 'Xem lớp', en: 'Read classes' } },
          { code: 'lms.report.read', scope_kind: 'tenant', group_code: 'school_lms', name: { vi: 'Xem báo cáo', en: 'Read reports' } },
          { code: 'game.quest.write', scope_kind: 'platform', group_code: 'game_ops', name: { vi: 'Sửa quest', en: 'Edit quests' } },
        ],
      });
    }
    const roles = [
      { role_id: IDS.roleSchoolAdmin, code: 'school_admin', name: { vi: 'Quản trị trường' }, role_scope_kind: 'tenant', is_system: true, permission_codes: ['tenant.class.read'], active_user_role_count: 12, version: 1 },
      { role_id: IDS.roleAcademic, code: 'academic_manager', name: { vi: 'Quản lý chuyên môn' }, role_scope_kind: 'tenant', is_system: true, permission_codes: ['tenant.class.read', 'lms.report.read'], active_user_role_count: 3, version: 1 },
      { role_id: IDS.roleCustom, code: 'to_truong_toan', name: { vi: 'Tổ trưởng Toán' }, role_scope_kind: 'tenant', is_system: false, permission_codes: ['tenant.class.read'], active_user_role_count: 5, version: 2, updated_at: '2026-11-12T08:00:00Z' },
    ];
    if (m === 'GET' && p === '/tenant/roles') return json(200, page(roles));
    if (m === 'POST' && p === '/tenant/roles') {
      const b = r.body as { permission_codes: string[] };
      const bad = b.permission_codes.findIndex((c) => c.startsWith('game.'));
      if (bad >= 0) return problem(422, 'VALIDATION_FAILED', { errors: [{ field: `permission_codes.${bad}`, code: 'PERMISSION_NOT_ALLOWED_FOR_SCOPE' }] });
      return json(201, { ...roles[2], role_id: '0193a4d2-7000-7000-8000-0000000000ff' });
    }
    if ((x = /^\/tenant\/roles\/([^/]+)$/.exec(p))) {
      const role = roles.find((ro) => ro.role_id === x![1]);
      if (!role) return problem(404, 'NOT_FOUND');
      if (m === 'GET') return json(200, role, etag(role.version));
      if (m === 'PATCH') return r.headers['if-match'] === `"v${role.version}"` ? json(200, { ...role, version: role.version + 1 }, etag(role.version + 1)) : problem(412, 'VERSION_MISMATCH');
      if (m === 'DELETE') return role.active_user_role_count ? problem(409, 'CONFLICT') : new Response(null, { status: 204 });
    }
    if (m === 'GET' && (p === '/tenant/user-roles' || /^\/schools\/[^/]+\/user-roles$/.test(p))) {
      return json(200, page([{ user_role_id: IDS.userRole1, user_id: IDS.userQlcm, role_id: IDS.roleAcademic, role_code: 'academic_manager', scope_type: 'subject', scope_id: IDS.subjectMath, source: 'manual', valid_from: '2026-11-01T00:00:00Z' }]));
    }
    if (m === 'POST' && (p === '/tenant/user-roles' || /^\/schools\/[^/]+\/user-roles$/.test(p))) {
      const b = r.body as { role_id: string; scope_id?: string };
      if (b.role_id === IDS.roleSchoolAdmin && p.startsWith('/schools/')) return problem(403, 'PERMISSION_DENIED');
      return json(201, { user_role_id: '0193a4d2-8000-7000-8000-0000000000ff', user_id: IDS.userQlcm, role_id: b.role_id, role_code: 'academic_manager', scope_type: 'subject', source: 'manual', valid_from: '2026-11-19T00:00:00Z' });
    }
    if (m === 'DELETE' && /^\/tenant\/user-roles\/[^/]+$/.test(p)) return new Response(null, { status: 204 });
    if (m === 'GET' && p === '/learning/subjects') return json(200, { items: [{ subject_id: IDS.subjectMath, code: 'math', name: { vi: 'Toán' } }] });
    if (m === 'GET' && p === '/tenant/audit-logs') {
      // MOCK of the server filters: action_prefix[] (dotted group, OR) AND action (exact), F059-3 / OQ F052-2
      const q = new URLSearchParams(r.search);
      const prefixes = q.getAll('action_prefix');
      const action = q.get('action');
      const rows: (Record<string, unknown> & { action: string })[] = [
        { audit_id: '0193a4d2-b000-7000-8000-000000000001', tenant_id: tenantOf(r), actor_id: IDS.userQlcm, actor_type: 'staff', action: 'user_role.granted', target_type: 'user', target_id: IDS.userQlcm, result: 'success', reason: 'Phân công tổ Toán', changes: { role_code: 'academic_manager' }, trace_id: 'trace-audit-1', occurred_at: '2026-11-18T08:00:00Z' },
        { audit_id: '0193a4d2-b000-7000-8000-000000000003', tenant_id: tenantOf(r), actor_id: IDS.userQlcm, actor_type: 'staff', action: 'role.created', target_type: 'role', target_id: IDS.roleSchoolAdmin, result: 'success', reason: 'Tạo role tổ trưởng', trace_id: 'trace-audit-3', occurred_at: '2026-11-18T09:00:00Z' },
        { audit_id: '0193a4d2-b000-7000-8000-000000000004', tenant_id: tenantOf(r), actor_id: IDS.userQlcm, actor_type: 'staff', action: 'school.updated', target_type: 'school', target_id: IDS.schoolA, result: 'success', reason: 'Đổi địa chỉ', trace_id: 'trace-audit-4', occurred_at: '2026-11-18T10:00:00Z' },
        // MOCK: a moderation row the server must not return — the client drops it too (R16)
        { audit_id: '0193a4d2-b000-7000-8000-000000000002', tenant_id: tenantOf(r), actor_type: 'staff', action: 'moderation.case.opened', target_type: 'moderation_case', target_id: 'x', result: 'success', occurred_at: '2026-11-18T08:00:00Z' },
      ];
      return json(
        200,
        page(
          rows.filter((e) => (!prefixes.length || prefixes.some((x) => e.action === x || e.action.startsWith(`${x}.`))) && (!action || e.action === action)),
          'data',
        ),
      );
    }
    if (p === '/tenant/settings') {
      if (m === 'GET') return json(200, state.tenantSettings, etag(state.tenantSettingsVersion));
      if (m === 'PATCH') {
        if (r.headers['if-match'] !== `"v${state.tenantSettingsVersion}"`) return problem(412, 'VERSION_MISMATCH');
        const b = r.body as Record<string, unknown>;
        if (typeof b.gameplay_chat_retention_days === 'number' && b.gameplay_chat_retention_days < 3) return problem(422, 'RETENTION_BELOW_MINIMUM');
        state.tenantSettings = { ...state.tenantSettings, ...b };
        state.tenantSettingsVersion++;
        return json(200, state.tenantSettings, etag(state.tenantSettingsVersion));
      }
    }
    if ((x = /^\/schools\/([^/]+)\/settings$/.exec(p))) {
      if (!schoolIn(r, x[1]!)) return problem(404, 'NOT_FOUND');
      if (m === 'GET') return json(200, state.schoolSettings, etag(state.schoolSettingsVersion));
      if (m === 'PATCH') {
        if (r.headers['if-match'] !== `"v${state.schoolSettingsVersion}"`) return problem(412, 'VERSION_MISMATCH');
        const b = r.body as Record<string, unknown>;
        const w = b.curfew_window as { start: string; end: string } | null | undefined;
        if (w && w.start > '21:30' && w.start < '23:59') return problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'curfew_window.start', code: 'CURFEW_NOT_STRICTER' }] });
        const { reason: _reason, ...rest } = b;
        void _reason;
        state.schoolSettings = { ...state.schoolSettings, ...rest, ...(w !== undefined ? { effective_curfew: w ?? state.schoolSettings.platform_curfew } : {}) };
        state.schoolSettingsVersion++;
        return json(200, state.schoolSettings, etag(state.schoolSettingsVersion));
      }
    }
    if (m === 'GET' && p === '/tenant/student-cleanup/preview') {
      return json(200, page([{ student_id: IDS.studentSapBiDon, display_name: 'Trần Sắp Bị Dọn', school_id: IDS.schoolA, class_name: '3A', last_login_at: null, scheduled_cleanup_at: '2026-12-01T02:00:00Z', excluded_until: null }], 'data'));
    }
    if (p === '/tenant/student-cleanup/exclusions') return new Response(null, { status: 204 });
    return undefined;
  });
  return state;
}
