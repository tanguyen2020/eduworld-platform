// MOCK (test only, AGENTS R17): in-memory stand-in of the chat-safety operations of specs/openapi/eduworld-api.v1.yaml
// used by the F122 screens — moderation cases (F121), sanctions, school safety notifications, sensitive-term catalogue,
// tenant terms/categories and moderation switches (F116), getTenantModerationStatus (F122), school chat policy and clubs
// (F120) — plugged into MockPortalBackend as overrides. It records requests, keeps a little state and simulates the few
// answers the screens must handle (412, 409, 403, 422 codes); it enforces NO authorization and is NOT a backend.
// Catalogue fixtures are placeholders only (`zq…`), never a real sensitive word (AGENTS R15).
import { WORKSPACE_CATALOG, type WorkspaceKey } from '../generated/menu-catalog';
import { ACCOUNTS, permissionsOf, problem, TENANTS, type MockAccount, type MockPortalBackend, type RecordedRequest } from './mock-backend';
import { IDS as LMS_IDS, installSchoolLmsMock } from './school-lms-mock';

const json = (status: number, body: unknown, headers: Record<string, string> = {}) =>
  new Response(body === undefined ? null : JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', ...headers } });

const T_A = TENANTS.thDemoA.tenant_id;
const T_B = TENANTS.thcsDemoB.tenant_id;
/** A tenant `common` (ADR-0078): school notifications are not offered for its cases. */
export const T_COMMON = '0193a4d2-0000-7000-8000-0000000000cc';

export const CS = {
  caseA: '0193a4d2-c000-7000-8000-00000000000a',
  caseB: '0193a4d2-c000-7000-8000-00000000000b',
  caseCommon: '0193a4d2-c000-7000-8000-00000000000c',
  schoolA: LMS_IDS.schoolA,
  schoolOther: '0193a4d2-2000-7000-8000-0000000000ff',
  studentA: '0193a4d2-4000-7000-8000-0000000000a1',
  studentOther: '0193a4d2-4000-7000-8000-0000000000f2',
  msg1: '0193a4d2-d000-7000-8000-000000000001',
  msg2: '0193a4d2-d000-7000-8000-000000000002',
  notifA: '0193a4d2-e000-7000-8000-000000000001',
  notifAcked: '0193a4d2-e000-7000-8000-000000000002',
  notifOld: '0193a4d2-e000-7000-8000-000000000003',
  sanction1: '0193a4d2-f000-7000-8000-000000000001',
  catInsult: '0193a4d2-a100-7000-8000-000000000001',
  catPolitics: '0193a4d2-a100-7000-8000-000000000002',
  term1: '0193a4d2-a200-7000-8000-000000000001',
  term2: '0193a4d2-a200-7000-8000-000000000002',
  memberOwner: '0193a4d2-4000-7000-8000-0000000000b1',
  memberBinh: '0193a4d2-4000-7000-8000-0000000000b2',
  memberChi: '0193a4d2-4000-7000-8000-0000000000b3',
  tenantTerm1: '0193a4d2-a200-7000-8000-000000000099',
  allow1: '0193a4d2-a300-7000-8000-000000000001',
  request1: '0193a4d2-a400-7000-8000-000000000001',
  clubPending: '0193a4d2-b000-7000-8000-000000000001',
  clubActive: '0193a4d2-b000-7000-8000-000000000002',
  moderator: '0193a4d2-1111-7000-8000-0000000000e1',
} as const;

/** Permissions of the platform role `safety_moderator` (src/backend/pkg/authz/roles.go, SystemRoles). */
export const SAFETY_PERMS = [
  'moderation.case.read', 'moderation.case.act', 'moderation.case.messages.read', 'moderation.sanction.read', 'moderation.sanction.write',
  'moderation.school_notification.send', 'chat.sensitive_terms.read', 'chat.tenant_terms.write', 'chat.tenant_category.toggle',
];

/**
 * Workspaces getMyPermissions returns for a permission set — the rule of module identity (portalcatalog.Workspaces): a
 * workspace is visible when one of its menus' any-of permissions is held. Used so the Safety Moderator principal of the
 * tests is the real one (role permissions + catalog), not a hand-picked workspace list.
 */
export function catalogWorkspaces(perms: readonly string[]): WorkspaceKey[] {
  return WORKSPACE_CATALOG.filter((w) => w.menus.some((m) => m.requiredPermissions.some((x) => perms.includes(x)))).map((w) => w.key);
}

/** Platform accounts of the F122 quickstart missing from the shell MOCK. */
export const SAFETY_ACCOUNTS: Record<string, MockAccount> = {
  'sm.antoan': {
    username: 'sm.antoan', email: 'sm.antoan@eduworld.test', password: ACCOUNTS['sa.admin']!.password, platform: true, sub: CS.moderator,
    displayName: 'Safety Moderator', tenants: [], perms: { platform: { workspaces: catalogWorkspaces(SAFETY_PERMS), permissions: SAFETY_PERMS } },
  },
  'sa.admin2': {
    ...ACCOUNTS['sa.admin']!, username: 'sa.admin2', email: 'sa.admin2@eduworld.test', sub: '0193a4d2-1111-7000-8000-0000000000e2', displayName: 'System Admin 2',
  },
  'ad.truong.b': {
    username: 'ad.truong.b', email: 'ad.truong.b@thcsdemo.edu.vn', password: ACCOUNTS['ad.truong.a']!.password, sub: '0193a4d2-1111-7000-8000-0000000000e3',
    displayName: 'Quản trị THCS Demo B', tenants: [TENANTS.thcsDemoB],
    perms: { [T_B]: { workspaces: ['school_lms'], permissions: permissionsOf(['school_lms']) } },
  },
};

const iso = (offsetHours: number) => new Date(Date.now() + offsetHours * 3_600_000).toISOString();
const steps = (off: string[] = []) => Object.fromEntries(['sensitive_terms', 'spam', 'bullying', 'async_analysis'].map((s) => [s, !off.includes(s)]));

export function chatSafetyState() {
  return {
    cases: [
      { case_id: CS.caseA, tenant_id: T_A, tenant_name: 'TH Demo A', tenant_type: 'school', status: 'open', severity: 'p1', source: 'report', category_code: 'insult', target_type: 'message', target_id: CS.msg1,
        channel_type: 'party', school_ids: [CS.schoolA, CS.schoolOther], report_count: 2, sla_due_at: iso(-1), version: 3, created_at: iso(-5),
        subject_user_ids: [CS.studentA, CS.studentOther], evidence_message_ids: [CS.msg1], actions: [{ action: 'created', actor_kind: 'system', at: iso(-5) }],
        reports: [], evidence: [], school_notifications: [] },
      { case_id: CS.caseB, tenant_id: T_B, tenant_name: 'THCS Demo B', tenant_type: 'school', status: 'in_review', severity: 'p2', source: 'filter_alert', category_code: 'spam', target_type: 'user',
        school_ids: [], report_count: 1, sla_due_at: iso(20), assignee_moderator_id: CS.moderator, version: 1, created_at: iso(-2), subject_user_ids: [], actions: [] },
      { case_id: CS.caseCommon, tenant_id: T_COMMON, tenant_name: 'Tenant chung', tenant_type: 'common', status: 'open', severity: 'p3', source: 'report', category_code: 'insult', target_type: 'user',
        school_ids: [], report_count: 1, sla_due_at: iso(40), version: 1, created_at: iso(-1), subject_user_ids: [CS.studentA], actions: [] },
    ] as Record<string, unknown>[],
    casePageSize: 2,
    /** Page size of listModerationCaseTenants (the dropdown walks every page). */
    caseTenantPageSize: 2,
    messages: [
      { message_id: CS.msg1, channel_id: CS.msg1, channel_type: 'party', seq: 10, sender_id: CS.studentOther, sender_nickname: 'MeoCon', sender_school_id: CS.schoolOther,
        kind: 'text', body: 'zqgoc ban dau', body_display: 'zq*** ban dau', revision: 2, revisions: [{ revision: 1, body: 'zqgoc lan 1', decision: 'mask', edited_at: iso(-6) }],
        moderation_state: 'masked', created_at: iso(-6), in_snapshot: true },
      { message_id: CS.msg2, channel_id: CS.msg1, channel_type: 'party', seq: 11, sender_id: CS.studentA, sender_nickname: 'CaVang', sender_school_id: CS.schoolA,
        kind: 'text', body: 'tin da thu hoi', revision: 1, moderation_state: 'allowed', recalled_at: iso(-5), created_at: iso(-6), in_snapshot: false },
    ] as Record<string, unknown>[],
    sanctions: [
      { sanction_id: CS.sanction1, tenant_id: T_A, user_id: CS.studentOther, case_id: CS.caseA, sanction_type: 'mute', issued_by_kind: 'moderator', starts_at: iso(-1), ends_at: iso(23) },
    ] as Record<string, unknown>[],
    platformNotifications: [
      { notification_id: CS.notifA, case_id: CS.caseA, tenant_id: T_A, school_id: CS.schoolA, severity: 'p1', school_notified_at: iso(-30), ack_due_at: iso(-6) },
      { notification_id: CS.notifAcked, case_id: CS.caseB, tenant_id: T_B, school_id: CS.schoolOther, severity: 'p2', school_notified_at: iso(-40), ack_due_at: iso(-16),
        school_ack_at: iso(-20), contact_outcome: 'parent_contacted' },
    ] as Record<string, unknown>[],
    schoolNotifications: [
      { notification_id: CS.notifA, case_id: CS.caseA, tenant_id: T_A, school_id: CS.schoolA, severity: 'p1', category_code: 'bullying',
        students: [{ student_id: CS.studentA, display_name: 'Nguyễn An', grade_level: 3, role: 'affected' }], measures_taken: 'Đã khóa chat 24 giờ',
        summary: 'Học sinh bị bạn trêu chọc lặp lại trong trận', school_notified_at: iso(-30), ack_due_at: iso(-6), created_at: iso(-30) },
      { notification_id: CS.notifAcked, case_id: CS.caseB, tenant_id: T_A, school_id: CS.schoolA, severity: 'p3', category_code: 'spam',
        students: [{ student_id: CS.studentA, display_name: 'Nguyễn An', grade_level: 3, role: 'offender' }], summary: 'Gửi tin lặp nhiều lần',
        school_notified_at: iso(-60), ack_due_at: iso(-36), school_ack_at: iso(-40), contact_outcome: 'not_needed', created_at: iso(-60) },
      { notification_id: CS.notifOld, case_id: CS.caseB, tenant_id: T_A, school_id: CS.schoolA, severity: 'p3', category_code: 'spam',
        students: [{ student_id: CS.studentA, display_name: 'Nguyễn An', grade_level: 3, role: 'offender' }], summary: 'Vụ việc cũ hơn 30 ngày',
        school_notified_at: iso(-24 * 45), ack_due_at: iso(-24 * 44), created_at: iso(-24 * 45) },
    ] as Record<string, unknown>[],
    categories: [
      { category_id: CS.catInsult, code: 'insult', name: 'Xúc phạm', severity: 'medium', default_action: 'mask', enabled: true, allow_tenant_toggle: false, term_count: 1, updated_at: iso(-100) },
      { category_id: CS.catPolitics, code: 'politics_religion', name: 'Chính trị, tôn giáo', severity: 'low', default_action: 'hold_for_review', enabled: true, allow_tenant_toggle: true, term_count: 0, updated_at: iso(-100) },
    ] as Record<string, unknown>[],
    terms: [
      { term_id: CS.term1, category_id: CS.catInsult, term: 'zqplaceholder', match_type: 'word', locale: 'vi', variants: ['zqp1aceholder'], enabled: true, scope: 'platform', status: 'published', updated_at: iso(-100) },
      { term_id: CS.term2, category_id: CS.catInsult, term: 'zqenglish', match_type: 'phrase', locale: 'en', variants: [], enabled: true, scope: 'platform', status: 'draft', updated_at: iso(-50) },
    ] as Record<string, unknown>[],
    tenantTerms: [
      { term_id: CS.tenantTerm1, category_id: CS.catInsult, term: 'zqtenantonly', match_type: 'word', locale: 'vi', enabled: true, scope: 'tenant', tenant_id: T_A, status: 'draft', updated_at: iso(-10) },
    ] as Record<string, unknown>[],
    allowlist: [{ entry_id: CS.allow1, phrase: 'zqok', locale: 'vi', reason: 'Tên địa danh', tenant_id: null }] as Record<string, unknown>[],
    versions: [
      { catalog_version: 7, scope: 'platform', tenant_id: null, published_at: iso(-200), published_by: CS.moderator, change_summary: 'Bản đầu', term_count: 1, regression: { recall: 0.97, false_positive_rate: 0.01 } },
    ] as Record<string, unknown>[],
    tenantCategoryOff: new Set<string>(),
    /** MOCK page size of listModerationSwitches (the screen asks 50; tests force small pages). */
    switchPageSize: 0,
    switches: {
      platform: { scope: 'platform', tenant_id: null, configured: null, effective: { enabled_all: true, steps: steps() }, disabled_steps: [] as string[], pending_requests: [] as unknown[] },
      tenants: {
        [T_A]: { scope: 'tenant', tenant_id: T_A, tenant_name: 'TH Demo A', configured: { enabled_all: true, steps: steps(['spam']), expires_at: iso(2) },
          effective: { enabled_all: true, steps: steps(['spam']) }, disabled_steps: ['spam'], updated_at: iso(-1),
          pending_requests: [{ request_id: CS.request1, scope: 'tenant', tenant_id: T_A, enabled_all: true, steps: steps(['spam', 'bullying']), reason: 'Điều tra lọc nhầm bắt nạt',
            expires_at: iso(3), status: 'pending_approval', proposed_by: ACCOUNTS['sa.admin']!.sub, created_at: iso(-1), request_expires_at: iso(23), version: 1 }] },
        [T_B]: { scope: 'tenant', tenant_id: T_B, tenant_name: 'THCS Demo B', configured: null, effective: { enabled_all: true, steps: steps() }, disabled_steps: [], pending_requests: [] },
      } as Record<string, Record<string, unknown>>,
    },
    /** Tenants with a step off at the tenant level (listModerationSwitches without tenant_ids, F122 T065). */
    disabledTenantCount: 1,
    history: [
      { change_id: '0193a4d2-a500-7000-8000-000000000001', scope: 'tenant', tenant_id: T_A, before: steps(), after: steps(['spam']), enabled_all_before: true, enabled_all_after: true,
        reason: 'Sự kiện thi đấu cấp trường', expires_at: iso(2), actor_id: ACCOUNTS['sa.admin']!.sub, approved_by: '0193a4d2-1111-7000-8000-0000000000e2', source: 'approved_request', changed_at: iso(-1) },
      { change_id: '0193a4d2-a500-7000-8000-000000000002', scope: 'platform', tenant_id: null, before: steps(['bullying']), after: steps(), enabled_all_before: true, enabled_all_after: true,
        reason: 'Hết hạn tắt', expires_at: null, actor_id: null, source: 'auto_expire', changed_at: iso(-30) },
    ] as Record<string, unknown>[],
    /** Moderation status served by getTenantModerationStatus per tenant (banner). */
    status: {
      [T_A]: { tenant_id: T_A, enabled_all: true, disabled_steps: ['spam'], expires_at: iso(2), source_scope: 'tenant' },
      [T_B]: { tenant_id: T_B, enabled_all: true, disabled_steps: [], expires_at: null, source_scope: null },
    } as Record<string, Record<string, unknown>>,
    /** School chat policy document (null: none, every value inherited). */
    schoolPolicy: null as Record<string, unknown> | null,
    clubs: [
      { club_id: CS.clubPending, club_type: 'interest_group', name: 'Nhà thám hiểm', status: 'pending_approval', member_count: 3, max_members: 50, school_id: CS.schoolA, version: 1 },
      { club_id: CS.clubActive, club_type: 'school_club', name: 'CLB Cờ vua', status: 'active', member_count: 3, max_members: 50, school_id: CS.schoolA, version: 2 },
    ] as Record<string, unknown>[],
    clubMembers: {
      [CS.clubActive]: [
        { club_id: CS.clubActive, user_id: CS.memberOwner, nickname: 'CaVang', role: 'owner', joined_at: iso(-300) },
        { club_id: CS.clubActive, user_id: CS.memberBinh, nickname: 'MeoCon', role: 'member', joined_at: iso(-200) },
        { club_id: CS.clubActive, user_id: CS.memberChi, nickname: 'ThoNau', role: 'member', joined_at: iso(-100) },
      ],
    } as Record<string, Record<string, unknown>[]>,
    clubMemberPageSize: 2,
  };
}

export type ChatSafetyState = ReturnType<typeof chatSafetyState>;

const EFFECTIVE: Record<string, Record<string, unknown>> = {
  direct: { channel_type: 'direct', enabled: true, allowed_kinds: ['text', 'emoji', 'quick'], max_text_length: 200, rate: { messages_per_minute: 10, burst: 2 }, slow_mode_seconds: 0,
    allowed_hours: [{ days: 'mon-sun', from: '06:30', to: '21:30' }], sources: [{ scope_type: 'platform', version: 5 }] },
  world: { channel_type: 'world', enabled: true, allowed_kinds: ['text', 'emoji', 'quick'], max_text_length: 120, rate: { messages_per_minute: 6, burst: 2 }, slow_mode_seconds: 5,
    allowed_hours: [{ days: 'mon-sun', from: '06:30', to: '21:30' }], sources: [{ scope_type: 'platform', version: 5 }] },
  area: { channel_type: 'area', enabled: true, allowed_kinds: ['text', 'emoji', 'quick'], max_text_length: 120, rate: { messages_per_minute: 6, burst: 2 }, slow_mode_seconds: 5,
    allowed_hours: [], sources: [{ scope_type: 'platform', version: 5 }] },
  team: { channel_type: 'team', enabled: true, allowed_kinds: ['text', 'emoji', 'quick', 'sticker', 'gif'], max_text_length: 200, rate: { messages_per_minute: 20, burst: 5 }, slow_mode_seconds: 0, allowed_hours: [], sources: [{ scope_type: 'platform', version: 5 }] },
  match: { channel_type: 'match', enabled: true, allowed_kinds: ['text', 'emoji', 'quick', 'sticker', 'gif'], max_text_length: 200, rate: { messages_per_minute: 20, burst: 5 }, slow_mode_seconds: 0, allowed_hours: [], sources: [{ scope_type: 'platform', version: 5 }] },
  party: { channel_type: 'party', enabled: true, allowed_kinds: ['text', 'emoji', 'quick', 'sticker', 'gif'], max_text_length: 200, rate: { messages_per_minute: 20, burst: 5 }, slow_mode_seconds: 0, allowed_hours: [], sources: [{ scope_type: 'platform', version: 5 }] },
};

/** Cursor page of a list (`cursor` = start index). */
function paged(rows: unknown[], search: URLSearchParams, size: number) {
  const from = Number(search.get('cursor') ?? 0);
  const data = rows.slice(from, from + size);
  const more = from + size < rows.length;
  return { data, page: { has_more: more, next_cursor: more ? String(from + size) : null, limit: size } };
}

/** Installs the chat-safety MOCK on a backend (with the workspace C MOCK for schools); returns its mutable state. */
export function installChatSafetyMock(backend: MockPortalBackend, state: ChatSafetyState = chatSafetyState()): ChatSafetyState {
  installSchoolLmsMock(backend);
  const handler = (rec: RecordedRequest): Response | undefined => {
    const q = new URLSearchParams(rec.search);
    const body = (rec.body ?? {}) as Record<string, unknown>;
    const tenantHeader = rec.headers['x-tenant-id'];
    const p = rec.path;
    const m = (re: RegExp) => re.exec(p);
    const key = `${rec.method} ${p}`;
    let x: RegExpExecArray | null;

    // tenant list of the Safety Team filters: the shell MOCK tenants + a tenant `common`
    if (key === 'GET /admin/tenants') {
      return json(200, { data: [...Object.values(TENANTS), { tenant_id: T_COMMON, code: 'chung', name: 'Tenant chung', tenant_type: 'common', status: 'active' }], page: { has_more: false, next_cursor: null } });
    }
    // ---- tenants having cases (F122 T064): source of the moderation tenant filter (keyset pages) ----
    if (key === 'GET /admin/moderation/case-tenants') {
      const seen = new Map<string, Record<string, unknown>>();
      for (const c of state.cases) if (!seen.has(String(c.tenant_id))) seen.set(String(c.tenant_id), { tenant_id: c.tenant_id, tenant_name: c.tenant_name, tenant_type: c.tenant_type });
      return json(200, paged([...seen.values()], q, state.caseTenantPageSize));
    }
    // ---- moderation cases (F121) ----
    if (key === 'GET /admin/moderation/cases') {
      const tenants = q.getAll('tenant_ids');
      let rows = state.cases.filter((c) => !tenants.length || tenants.includes(String(c.tenant_id)));
      for (const f of ['status', 'severity', 'category_code'] as const) if (q.get(f)) rows = rows.filter((c) => c[f] === q.get(f));
      if (q.get('assignee') === 'unassigned') rows = rows.filter((c) => !c.assignee_moderator_id);
      if (q.get('assignee') === 'me') rows = rows.filter((c) => c.assignee_moderator_id === backend.user?.sub);
      return json(200, paged(rows, q, state.casePageSize));
    }
    if ((x = m(/^\/admin\/moderation\/cases\/([^/:]+)$/)) && rec.method === 'GET') {
      const c = state.cases.find((r) => r.case_id === x![1]);
      return c ? json(200, c, { ETag: `W/"${String(c.version)}"` }) : problem(404, 'NOT_FOUND');
    }
    if ((x = m(/^\/admin\/moderation\/cases\/([^/:]+)\/messages$/)) && rec.method === 'GET') {
      const needle = (q.get('q') ?? '').toLowerCase();
      const rows = state.messages.filter((r) => !needle || String(r.body ?? '').toLowerCase().includes(needle) || String(r.sender_nickname ?? '').toLowerCase().includes(needle));
      return json(200, paged(rows, q, 50));
    }
    if ((x = m(/^\/admin\/moderation\/cases\/([^/:]+):(assign|act|notify-school)$/)) && rec.method === 'POST') {
      const c = state.cases.find((r) => r.case_id === x![1]);
      if (!c) return problem(404, 'NOT_FOUND');
      if (x[2] === 'assign') {
        c.assignee_moderator_id = body.assignee_moderator_id;
        c.status = 'in_review';
        c.version = Number(c.version) + 1;
        return json(200, c, { ETag: `W/"${String(c.version)}"` });
      }
      if (x[2] === 'act') {
        if (rec.headers['if-match'] !== `W/"${String(c.version)}"`) return problem(412, 'VERSION_MISMATCH');
        if (c.status === 'resolved' && body.action !== 'reopen') return problem(409, 'MODERATION_CASE_CLOSED');
        c.status = body.action === 'close' || body.action === 'dismiss' ? 'resolved' : body.action === 'escalate' ? 'escalated' : body.action === 'reopen' ? 'reopened' : 'in_review';
        c.version = Number(c.version) + 1;
        (c.actions as unknown[]).push({ action: body.action, actor_kind: 'moderator', actor_id: backend.user?.sub, note: body.note, at: new Date().toISOString() });
        return json(200, c, { ETag: `W/"${String(c.version)}"` });
      }
      const n = { notification_id: '0193a4d2-e000-7000-8000-0000000000ff', case_id: c.case_id, tenant_id: c.tenant_id, school_id: body.school_id, severity: c.severity, school_notified_at: new Date().toISOString(), ack_due_at: iso(24) };
      (c.school_notifications as unknown[]).push(n);
      return json(201, n);
    }
    // ---- sanctions ----
    if (key === 'GET /admin/moderation/sanctions') {
      const tenants = q.getAll('tenant_ids');
      const rows = state.sanctions.filter((s) => (!tenants.length || tenants.includes(String(s.tenant_id))) && (q.get('active') !== 'true' || !s.revoked_at));
      return json(200, paged(rows, q, 50));
    }
    if (key === 'POST /admin/moderation/sanctions') {
      if (!tenantHeader) return problem(400, 'TENANT_HEADER_REQUIRED');
      const s = { sanction_id: `0193a4d2-f000-7000-8000-${String(state.sanctions.length + 1).padStart(12, '0')}`, tenant_id: tenantHeader, ...body, issued_by_kind: 'moderator', starts_at: new Date().toISOString() };
      state.sanctions.unshift(s);
      return json(201, s);
    }
    if ((x = m(/^\/admin\/moderation\/sanctions\/([^/:]+):revoke$/)) && rec.method === 'POST') {
      const s = state.sanctions.find((r) => r.sanction_id === x![1]);
      if (!s) return problem(404, 'NOT_FOUND');
      Object.assign(s, { revoked_at: new Date().toISOString(), revoke_reason: body.reason });
      return json(200, s);
    }
    // ---- school safety notifications ----
    if (key === 'GET /admin/safety/school-notifications') {
      const tenants = q.getAll('tenant_ids');
      let rows = state.platformNotifications.filter((n) => !tenants.length || tenants.includes(String(n.tenant_id)));
      if (q.get('acknowledged') === 'true') rows = rows.filter((n) => n.school_ack_at);
      if (q.get('acknowledged') === 'false') rows = rows.filter((n) => !n.school_ack_at);
      if (q.get('overdue') === 'true') rows = rows.filter((n) => !n.school_ack_at && String(n.ack_due_at) < new Date().toISOString());
      return json(200, paged(rows, q, 50));
    }
    if (key === 'GET /tenant/safety/school-notifications') {
      let rows = state.schoolNotifications.filter((n) => n.tenant_id === tenantHeader);
      const from = q.get('created_from');
      if (from) rows = rows.filter((n) => String(n.created_at) >= new Date(from).toISOString());
      if (q.get('acknowledged') === 'true') rows = rows.filter((n) => n.school_ack_at);
      if (q.get('acknowledged') === 'false') rows = rows.filter((n) => !n.school_ack_at);
      return json(200, paged(rows, q, 50));
    }
    if ((x = m(/^\/tenant\/safety\/school-notifications\/([^/:]+):acknowledge$/)) && rec.method === 'POST') {
      const n = state.schoolNotifications.find((r) => r.notification_id === x![1] && r.tenant_id === tenantHeader);
      if (!n) return problem(404, 'NOT_FOUND');
      if (n.school_ack_at) return problem(409, 'CONFLICT');
      Object.assign(n, { school_ack_at: new Date().toISOString(), contact_outcome: body.contact_outcome, ack_note: body.note });
      return json(200, n);
    }
    // ---- sensitive-term catalogue (F116) ----
    // cursor pages (keyset (code, category_id) in SQL on the server — owner rule 2026-11-23)
    if (key === 'GET /admin/chat/sensitive-categories') return json(200, paged(state.categories, q, Number(q.get('limit') ?? 20)));
    if (key === 'POST /admin/chat/sensitive-categories') {
      const c = { category_id: '0193a4d2-a100-7000-8000-0000000000aa', term_count: 0, updated_at: new Date().toISOString(), ...body };
      state.categories.push(c);
      return json(201, c);
    }
    if ((x = m(/^\/admin\/chat\/sensitive-categories\/([^/]+)$/))) {
      const i = state.categories.findIndex((r) => r.category_id === x![1]);
      if (i < 0) return problem(404, 'NOT_FOUND');
      if (rec.method === 'DELETE') {
        state.categories.splice(i, 1);
        return new Response(null, { status: 204 });
      }
      Object.assign(state.categories[i]!, body);
      return json(200, state.categories[i]);
    }
    if (key === 'GET /admin/chat/sensitive-terms') {
      // filters applied by the server (SQL) — locale, match_type, status (F122 T063)
      const rows = state.terms.filter((r) => (['category_id', 'locale', 'match_type', 'status'] as const).every((f) => !q.get(f) || r[f] === q.get(f)));
      return json(200, paged(rows, q, 50));
    }
    if (key === 'POST /admin/chat/sensitive-terms:test') {
      const text = String(body.text ?? '');
      const matches = [...text.matchAll(/zq[a-z0-9]*/g)].map((r) => ({ category_code: 'insult', severity: 'medium', action: 'mask', start: r.index!, end: r.index! + r[0].length }));
      return json(200, { decision: matches.length ? 'mask' : 'allow', masked_text: text.replace(/zq[a-z0-9]*/g, (w) => '*'.repeat(w.length)), matches });
    }
    if (key === 'POST /admin/chat/sensitive-terms') {
      const t = { term_id: '0193a4d2-a200-7000-8000-0000000000aa', scope: 'platform', status: 'draft', updated_at: new Date().toISOString(), ...body };
      state.terms.push(t);
      return json(201, t);
    }
    if ((x = m(/^\/admin\/chat\/sensitive-terms\/([^/:]+)$/))) {
      const i = state.terms.findIndex((r) => r.term_id === x![1]);
      if (i < 0) return problem(404, 'NOT_FOUND');
      if (rec.method === 'DELETE') {
        state.terms.splice(i, 1);
        return new Response(null, { status: 204 });
      }
      Object.assign(state.terms[i]!, body);
      return json(200, state.terms[i]);
    }
    if (key === 'GET /admin/chat/sensitive-allowlist') return json(200, paged(state.allowlist, q, 50));
    if (key === 'POST /admin/chat/sensitive-allowlist') {
      const a = { entry_id: '0193a4d2-a300-7000-8000-0000000000aa', tenant_id: null, ...body };
      state.allowlist.push(a);
      return json(201, a);
    }
    if ((x = m(/^\/admin\/chat\/sensitive-allowlist\/([^/]+)$/)) && rec.method === 'DELETE') {
      state.allowlist = state.allowlist.filter((r) => r.entry_id !== x![1]);
      return new Response(null, { status: 204 });
    }
    if (key === 'POST /admin/chat/sensitive-catalog:publish' || key === 'POST /admin/chat/tenant-terms:publish') {
      const tenant = key.includes('tenant-terms');
      const v = { catalog_version: state.versions.length + 7, scope: tenant ? 'tenant' : 'platform', tenant_id: tenant ? tenantHeader : null, published_at: new Date().toISOString(),
        published_by: backend.user?.sub, change_summary: body.change_summary, term_count: 2, regression: { recall: 0.98, false_positive_rate: 0.005 } };
      state.versions.unshift(v);
      return json(201, v);
    }
    if (key === 'GET /admin/chat/sensitive-catalog/versions') return json(200, paged(state.versions, q, 50));
    // ---- tenant terms and categories (Safety Team, X-Tenant-Id = target tenant) ----
    if (key === 'GET /admin/chat/tenant-terms') return json(200, paged(state.tenantTerms.filter((r) => r.tenant_id === tenantHeader), q, 50));
    if (key === 'POST /admin/chat/tenant-terms') {
      const t = { term_id: '0193a4d2-a200-7000-8000-0000000000bb', scope: 'tenant', tenant_id: tenantHeader, status: 'draft', updated_at: new Date().toISOString(), ...body };
      state.tenantTerms.push(t);
      return json(201, t);
    }
    if ((x = m(/^\/admin\/chat\/tenant-terms\/([^/:]+)$/))) {
      const i = state.tenantTerms.findIndex((r) => r.term_id === x![1] && r.tenant_id === tenantHeader);
      if (i < 0) return problem(404, 'NOT_FOUND');
      if (rec.method === 'DELETE') {
        state.tenantTerms.splice(i, 1);
        return new Response(null, { status: 204 });
      }
      Object.assign(state.tenantTerms[i]!, body);
      return json(200, state.tenantTerms[i]);
    }
    if (key === 'GET /tenant/chat/categories') {
      const data = state.categories.map((c) => ({ code: c.code, name: c.name, severity: c.severity, enabled_platform: c.enabled, allow_tenant_toggle: c.allow_tenant_toggle,
        enabled_in_tenant: !state.tenantCategoryOff.has(`${tenantHeader}:${String(c.code)}`), tenant_term_count: state.tenantTerms.filter((t) => t.tenant_id === tenantHeader && t.category_id === c.category_id).length }));
      return json(200, { ...paged(data, q, Number(q.get('limit') ?? 20)), moderation_switches: state.switches.tenants[tenantHeader ?? ''] ?? state.switches.platform });
    }
    if ((x = m(/^\/admin\/chat\/tenant-categories\/([^/:]+):toggle$/)) && rec.method === 'POST') {
      const c = state.categories.find((r) => r.code === x![1]);
      if (!c) return problem(404, 'NOT_FOUND');
      if (!c.allow_tenant_toggle) return problem(403, 'TENANT_TOGGLE_NOT_ALLOWED');
      const k = `${tenantHeader}:${String(c.code)}`;
      if (body.enabled) state.tenantCategoryOff.delete(k);
      else state.tenantCategoryOff.add(k);
      return json(200, { code: c.code, name: c.name, severity: c.severity, enabled_platform: true, enabled_in_tenant: !!body.enabled, allow_tenant_toggle: true });
    }
    // ---- moderation switches ----
    if (key === 'GET /admin/chat/moderation-switches') {
      // chat.policy.toggle only (System Admin) — the Safety Moderator is refused like by module chat (no banner then)
      if (!(backend.user?.perms.platform?.permissions ?? []).includes('chat.policy.toggle')) return problem(403, 'PERMISSION_DENIED');
      // one cursor page of the tenants of tenant_ids, the platform level on every page (API_DESIGN §5); without
      // tenant_ids the number of tenants off (disabled_tenant_count, F122 T065)
      const ids = q.getAll('tenant_ids');
      const tenants = ids.map((t) => state.switches.tenants[t]).filter(Boolean);
      const { data, page } = paged(tenants, q, state.switchPageSize || Number(q.get('limit') ?? 20));
      const count = ids.length ? {} : { disabled_tenant_count: state.disabledTenantCount };
      return json(200, { platform: state.switches.platform, tenants: data, page, ...count });
    }
    if ((x = m(/^\/admin\/chat\/moderation-switches\/requests\/([^/:]+):reject$/)) && rec.method === 'POST') {
      for (const level of [state.switches.platform, ...Object.values(state.switches.tenants)]) {
        const list = level.pending_requests as Record<string, unknown>[];
        const r = list.find((y) => y.request_id === x![1]);
        if (!r) continue;
        if (r.proposed_by === backend.user?.sub) return problem(403, 'MODERATION_SWITCH_SELF_APPROVAL');
        if (rec.headers['if-match'] !== `W/"${String(r.version)}"`) return problem(412, 'VERSION_MISMATCH');
        level.pending_requests = list.filter((y) => y !== r);
        return json(200, { ...r, status: 'rejected', decided_at: new Date().toISOString(), version: Number(r.version) + 1 });
      }
      return problem(404, 'NOT_FOUND');    }
    if (key === 'PUT /admin/chat/moderation-switches') {
      if (!body.reason || String(body.reason).length < 10) return problem(422, 'MODERATION_SWITCH_REASON_REQUIRED', { errors: [{ field: 'reason', code: 'MODERATION_SWITCH_REASON_REQUIRED' }] });
      const s = body.steps as Record<string, boolean>;
      const disabling = body.enabled_all === false || Object.values(s).some((v) => !v);
      if (disabling && !body.expires_at) return problem(422, 'MODERATION_SWITCH_EXPIRY_REQUIRED');
      const level = body.scope === 'tenant' ? state.switches.tenants[String(body.tenant_id)] : state.switches.platform;
      if (!level) return problem(404, 'NOT_FOUND');
      if (disabling) {
        const r = { request_id: '0193a4d2-a400-7000-8000-0000000000aa', scope: body.scope, tenant_id: body.tenant_id ?? null, enabled_all: body.enabled_all, steps: s,
          reason: body.reason, expires_at: body.expires_at, status: 'pending_approval', proposed_by: backend.user?.sub, created_at: new Date().toISOString(), request_expires_at: iso(24) };
        (level.pending_requests as unknown[]).push(r);
        return json(202, r);
      }
      Object.assign(level, { configured: { enabled_all: true, steps: s, expires_at: null }, effective: { enabled_all: true, steps: s }, disabled_steps: [] });
      return json(200, level);
    }
    if ((x = m(/^\/admin\/chat\/moderation-switches\/requests\/([^/:]+):approve$/)) && rec.method === 'POST') {
      for (const level of [state.switches.platform, ...Object.values(state.switches.tenants)]) {
        const list = level.pending_requests as Record<string, unknown>[];
        const r = list.find((y) => y.request_id === x![1]);
        if (!r) continue;
        if (r.proposed_by === backend.user?.sub) return problem(403, 'MODERATION_SWITCH_SELF_APPROVAL');
        level.pending_requests = list.filter((y) => y !== r);
        const off = Object.entries(r.steps as Record<string, boolean>).filter(([, v]) => !v).map(([k]) => k);
        Object.assign(level, { configured: { enabled_all: r.enabled_all, steps: r.steps, expires_at: r.expires_at }, effective: { enabled_all: r.enabled_all, steps: r.steps }, disabled_steps: off });
        return json(200, level);
      }
      return problem(404, 'NOT_FOUND');
    }
    if (key === 'GET /admin/chat/moderation-switches/history') return json(200, paged(state.history, q, 50));
    if (key === 'GET /tenant/chat/moderation-status') {
      const s = state.status[tenantHeader ?? ''];
      return s ? json(200, s) : problem(403, 'PERMISSION_DENIED');
    }
    // ---- school chat policy (F120) ----
    if ((x = m(/^\/tenant\/chat-policies\/school\/([^/]+)$/))) {
      if (rec.method === 'GET') return state.schoolPolicy ? json(200, state.schoolPolicy, { ETag: `"${String(state.schoolPolicy.version)}"` }) : problem(404, 'NOT_FOUND');
      const expected = state.schoolPolicy ? `"${String(state.schoolPolicy.version)}"` : '"0"';
      if (rec.headers['if-match'] !== expected) return problem(412, 'VERSION_MISMATCH');
      state.schoolPolicy = { policy_id: '0193a4d2-a600-7000-8000-000000000001', scope_type: 'school', scope_id: x[1], version: (Number(state.schoolPolicy?.version ?? 0)) + 1, ...body };
      return json(200, state.schoolPolicy, { ETag: `"${String(state.schoolPolicy.version)}"` });
    }
    if (key === 'GET /tenant/chat-policies:effective') return json(200, EFFECTIVE[q.get('channel_type') ?? ''] ?? EFFECTIVE.direct);
    // ---- club members for staff (F122 T061) ----
    if ((x = m(/^\/tenant\/social\/clubs\/([^/:]+)\/members$/)) && rec.method === 'GET') {
      const rows = state.clubMembers[x[1]!];
      return rows ? json(200, paged(rows, q, state.clubMemberPageSize)) : problem(404, 'NOT_FOUND');
    }
    if ((x = m(/^\/tenant\/social\/clubs\/([^/:]+)\/members\/([^/:]+):remove$/)) && rec.method === 'POST') {
      const rows = state.clubMembers[x[1]!];
      const i = rows?.findIndex((r) => r.user_id === x![2] && r.role !== 'owner') ?? -1;
      if (!rows || i < 0) return problem(404, 'NOT_FOUND');
      rows.splice(i, 1);
      return new Response(null, { status: 204 });
    }
    // ---- clubs (F120) ----
    if (key === 'GET /social/clubs') {
      const rows = state.clubs.filter((c) => (!q.get('school_id') || c.school_id === q.get('school_id')) && (!q.get('status') || c.status === q.get('status')));
      return json(200, paged(rows, q, 50));
    }
    if ((x = m(/^\/social\/clubs\/([^/:]+):(approve|archive)$/)) && rec.method === 'POST') {
      const c = state.clubs.find((r) => r.club_id === x![1]);
      if (!c) return problem(404, 'NOT_FOUND');
      if (x[2] === 'approve') {
        if (c.status !== 'pending_approval') return problem(409, 'CONFLICT');
        c.status = 'active';
        return json(200, c);
      }
      c.status = 'archived';
      return new Response(null, { status: 204 });
    }
    return undefined;
  };
  backend.overrides.push(handler);
  return state;
}

export interface RenderSafetyOptions {
  account?: string;
  state?: ChatSafetyState;
  disabledMenus?: string[];
  disabledActions?: string[];
  width?: number;
}

/** MOCK (test only): renders the whole portal at `url` signed in as `account` with the chat-safety MOCK installed. */
export async function renderSafety(url: string, opts: RenderSafetyOptions = {}) {
  const { MockPortalBackend, renderPortal, setViewportWidth } = await import('./portal-harness');
  const name = opts.account ?? 'sa.admin';
  const backend = new MockPortalBackend().withSession(SAFETY_ACCOUNTS[name] ? 'sa.admin' : name);
  if (SAFETY_ACCOUNTS[name]) backend.user = SAFETY_ACCOUNTS[name]!;
  backend.disabledMenus = opts.disabledMenus ?? [];
  backend.disabledActions = opts.disabledActions ?? [];
  const state = installChatSafetyMock(backend, opts.state);
  if (opts.width) setViewportWidth(opts.width);
  const rendered = await renderPortal({ url, backend });
  return { ...rendered, state };
}

export { T_A, T_B };
