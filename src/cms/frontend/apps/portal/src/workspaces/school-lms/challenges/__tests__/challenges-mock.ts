// MOCK (test only, AGENTS R17): in-memory stand-in of the F107 operations used by the "Thử thách khối / giải trường"
// screen of workspace C (F108 US3) — listChallengeTemplates, listChallenges, createChallenge, getChallenge,
// cancelChallenge, getTournament, publishTournament, cancelTournament, registerTournamentEntry,
// withdrawTournamentEntry, confirmTournamentEntries, generateTournamentBracket, getTournamentBracket,
// confirmBracketNode, overrideBracketNode — answering per specs/openapi/eduworld-api.v1.yaml, plus the shell MOCK
// backend and the F052 MOCK (schools, students by grade). Writes check Idempotency-Key (same key → first answer, no
// duplicate). Data of TH Demo A; THCS Demo B has its own template and no challenge; any other tenant → 404.
// Never imported by application code; NOT a real backend and no live integration.
import { configure } from '@testing-library/react';
import { vi } from 'vitest';
import { MockPortalBackend, problem, renderPortal, setViewportWidth, TENANTS, type MockAccount, type RecordedRequest } from '../../../../test/portal-harness';
import { accountWith, GRADE_STUDENTS, IDS, installSchoolLmsMock, SCHOOL_PERMS } from '../../../../test/school-lms-mock';

configure({ asyncUtilTimeout: 30_000 });
vi.setConfig({ testTimeout: 480_000 });

const json = (status: number, body: unknown) =>
  new Response(status === 204 ? null : JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
const page = (data: unknown[]) => ({ data, page: { has_more: false, limit: 50, next_cursor: null } });

const T_A = TENANTS.thDemoA.tenant_id;
const T_B = TENANTS.thcsDemoB.tenant_id;

export const CH_IDS = {
  tplTournament: '0193a4d2-c000-7000-8000-000000000001',
  tplQuiz: '0193a4d2-c000-7000-8000-000000000002',
  tplB: '0193a4d2-c000-7000-8000-000000000003',
  tplUnassigned: '0193a4d2-c000-7000-8000-0000000000ff',
  challengeTournament: '0193a4d2-c100-7000-8000-000000000001',
  challengeQuiz: '0193a4d2-c100-7000-8000-000000000002',
  tournament: '0193a4d2-c200-7000-8000-000000000001',
  studentCuong4: '0193a4d2-4000-7000-8000-0000000000c4',
  studentBinh: '0193a4d2-4000-7000-8000-0000000000b2',
  otherSchoolStudent: '0193a4d2-4000-7000-8000-0000000000f9',
} as const;

/** Nicknames of the players (TournamentPlayer only carries nickname/avatar — R16). */
export const NICKNAMES: Record<string, string> = {
  [IDS.studentAn]: 'Mèo Xanh',
  [CH_IDS.studentBinh]: 'Cáo Đỏ',
  [IDS.studentSapBiDon]: 'Thỏ Trắng',
  [CH_IDS.studentCuong4]: 'Gấu Nâu',
};

type Row = Record<string, unknown>;

export const TEMPLATES: Record<string, Row[]> = {
  [T_A]: [
    {
      template_id: CH_IDS.tplTournament,
      version: 2,
      template_version_id: '0193a4d2-c000-7000-8000-0000000000a1',
      kind: 'school_tournament',
      format: 'tournament',
      name: 'Giải Toán khối',
      description: 'Loại trực tiếp 1v1',
      mode: 'battle_1v1',
      tournament_format: 'single_elim',
      participant_type: 'individual',
      grade_levels: [3, 4],
      allowed_params: ['max_entries', 'seeding_policy', 'registration_close_at', 'auto_confirm'],
      reward_summary: 'Huy hiệu Vô địch khối',
    },
    {
      template_id: CH_IDS.tplQuiz,
      version: 1,
      kind: 'grade_challenge',
      format: 'timed_quiz',
      name: 'Thử thách Timed Quiz',
      grade_levels: [1, 2, 3],
      allowed_params: ['theme_code'],
    },
  ],
  [T_B]: [{ template_id: CH_IDS.tplB, version: 1, kind: 'grade_challenge', format: 'timed_quiz', name: 'Thử thách THCS B', grade_levels: [1, 2], allowed_params: [] }],
};

export interface ChallengesMockState {
  challenges: Map<string, Row>;
  tournament: Row;
  entries: Row[];
  nodes: Row[];
  /** Idempotency-Key → recorded answer. */
  idem: Map<string, { status: number; body: unknown }>;
}

let seq = 0;
const newId = () => `0193a4d2-c999-7000-8000-${String(++seq).padStart(12, '0')}`;

export function tournamentChallenge(over: Row = {}): Row {
  return {
    challenge_id: CH_IDS.challengeTournament,
    tenant_id: T_A,
    template_id: CH_IDS.tplTournament,
    name: 'Giải Toán khối 3',
    kind: 'school_tournament',
    format: 'tournament',
    school_id: IDS.schoolA,
    targets: [
      { target_type: 'grade', grade_level: 3 },
      { target_type: 'students', student_ids: [CH_IDS.studentCuong4] },
    ],
    exclude_student_ids: [IDS.studentSapBiDon],
    starts_at: '2026-12-01T01:00:00Z',
    ends_at: '2026-12-05T10:00:00Z',
    status: 'scheduled',
    audience_count: 3,
    participant_count: 0,
    created_by: '0193a4d2-1111-7000-8000-000000000001',
    tournament_id: CH_IDS.tournament,
    ...over,
  };
}

export function quizChallenge(over: Row = {}): Row {
  return {
    challenge_id: CH_IDS.challengeQuiz,
    tenant_id: T_A,
    template_id: CH_IDS.tplQuiz,
    name: 'Đua nhanh khối 2',
    kind: 'grade_challenge',
    format: 'timed_quiz',
    school_id: IDS.schoolA,
    targets: [{ target_type: 'grade', grade_level: 2 }],
    starts_at: '2026-12-02T01:00:00Z',
    status: 'scheduled',
    audience_count: 40,
    created_by: '0193a4d2-1111-7000-8000-000000000001',
    tournament_id: null,
    ...over,
  };
}

const entryOf = (studentId: string, status = 'registered'): Row => ({
  entry_id: `0193a4d2-c300-7000-8000-${studentId.slice(-12)}`,
  tournament_id: CH_IDS.tournament,
  status,
  checked_in_at: null,
  seed: null,
  final_rank: null,
  players: [{ student_id: studentId, nickname: NICKNAMES[studentId] ?? null }],
});

/** Audience of the fixture challenge: grade 3 of TH Demo A ∪ Cường − Sắp Bị Dọn (MOCK of the server rule). */
export const AUDIENCE = [IDS.studentAn, CH_IDS.studentBinh, CH_IDS.studentCuong4];
export const ENTRY = Object.fromEntries(AUDIENCE.map((s) => [s, entryOf(s).entry_id as string])) as Record<string, string>;
export const NODE = { semi1: '0193a4d2-c400-7000-8000-000000000001', semi2: '0193a4d2-c400-7000-8000-000000000002', final: '0193a4d2-c400-7000-8000-000000000003' } as const;

/** Bracket of 3 confirmed entries (single elimination): An vs Bình awaiting confirmation, Cường bye, final pending. */
export function generatedNodes(): Row[] {
  return [
    { node_id: NODE.semi1, bracket: 'winners', round: 1, position: 1, entry_a_id: ENTRY[IDS.studentAn], entry_b_id: ENTRY[CH_IDS.studentBinh], status: 'awaiting_confirm', winner_entry_id: ENTRY[IDS.studentAn], next_win_node_id: NODE.final },
    { node_id: NODE.semi2, bracket: 'winners', round: 1, position: 2, entry_a_id: ENTRY[CH_IDS.studentCuong4], entry_b_id: null, status: 'bye', winner_entry_id: ENTRY[CH_IDS.studentCuong4], next_win_node_id: NODE.final },
    { node_id: NODE.final, bracket: 'winners', round: 2, position: 1, entry_a_id: null, entry_b_id: ENTRY[CH_IDS.studentCuong4], status: 'pending' },
  ];
}

export function confirmedEntries(): Row[] {
  return AUDIENCE.map((s) => entryOf(s, 'confirmed'));
}

/** Installs the MOCK handlers (first override) and returns their state. */
export function installChallengesMock(backend: MockPortalBackend): ChallengesMockState {
  const state: ChallengesMockState = {
    challenges: new Map([
      [CH_IDS.challengeTournament, tournamentChallenge()],
      [CH_IDS.challengeQuiz, quizChallenge()],
    ]),
    tournament: {
      tournament_id: CH_IDS.tournament,
      challenge_id: CH_IDS.challengeTournament,
      name: 'Giải Toán khối 3',
      format: 'single_elim',
      scope_type: 'school',
      scope_id: IDS.schoolA,
      school_id: IDS.schoolA,
      grade_level: 3,
      subject_id: IDS.subjectMath,
      template_id: CH_IDS.tplTournament,
      registration_closes_at: '2026-11-30T10:00:00Z',
      starts_at: '2026-12-01T01:00:00Z',
      seeding_policy: 'rating',
      participant_type: 'individual',
      mode: 'battle_1v1',
      reward_summary: 'Huy hiệu Vô địch khối',
      status: 'draft',
      entry_count: 0,
    },
    entries: [],
    nodes: [],
    idem: new Map(),
  };
  const tenantOf = (r: RecordedRequest) => r.headers['x-tenant-id'] ?? T_A;
  /** Same Idempotency-Key → the first answer again (no second write). */
  const once = (r: RecordedRequest, run: () => { status: number; body: unknown } | Response): Response => {
    const key = r.headers['idempotency-key'];
    if (!key) return problem(400, 'IDEMPOTENCY_KEY_REQUIRED');
    const prev = state.idem.get(`${r.path}|${key}`);
    if (prev) return json(prev.status, prev.body);
    const res = run();
    if (res instanceof Response) return res;
    state.idem.set(`${r.path}|${key}`, res);
    return json(res.status, res.body);
  };
  const tour = () => ({ ...state.tournament, entry_count: state.entries.filter((e) => e.status !== 'withdrawn').length });
  const bracket = () => ({ tournament_id: CH_IDS.tournament, format: 'single_elim', status: state.tournament.status, nodes: state.nodes, groups: [], entries: state.entries });

  backend.overrides.unshift((r) => {
    const p = r.path;
    const m = r.method;
    if (!/^\/(lms\/catalog\/challenge-templates|lms\/challenges|competition\/tournaments)/.test(p)) return undefined;
    const tenant = tenantOf(r);
    if (tenant !== T_A && tenant !== T_B) return problem(404, 'NOT_FOUND');
    const b = (r.body ?? {}) as Row;
    let x: RegExpExecArray | null;
    if (m === 'GET' && p === '/lms/catalog/challenge-templates') return json(200, page(TEMPLATES[tenant] ?? []));
    if (p === '/lms/challenges' && m === 'GET') {
      const q = new URLSearchParams(r.search);
      const grade = q.get('grade_level');
      const status = q.get('status');
      const rows = tenant === T_A ? [...state.challenges.values()] : [];
      return json(200, page(rows.filter((c) => (!grade || (c.targets as Row[]).some((t) => t.grade_level === Number(grade))) && (!status || c.status === status))));
    }
    if (p === '/lms/challenges' && m === 'POST') {
      return once(r, () => {
        const tpl = (TEMPLATES[tenant] ?? []).find((t) => t.template_id === b.template_id);
        if (!tpl) return problem(403, 'RESOURCE_NOT_ASSIGNED');
        const errors: Row[] = [];
        for (const k of Object.keys((b.params ?? {}) as Row)) if (!(tpl.allowed_params as string[]).includes(k)) errors.push({ field: `/params/${k}`, code: 'NOT_ALLOWED' });
        if (b.school_id !== IDS.schoolA && tenant === T_A) errors.push({ field: '/school_id', code: 'NOT_IN_SCOPE' });
        const inSchool = new Set(GRADE_STUDENTS.map((s) => s.student_id));
        (b.targets as Row[]).forEach((t, i) => ((t.student_ids as string[] | undefined) ?? []).forEach((s, j) => !inSchool.has(s) && errors.push({ field: `/targets/${i}/student_ids/${j}`, code: 'NOT_IN_SCHOOL' })));
        ((b.exclude_student_ids as string[] | undefined) ?? []).forEach((s, j) => !inSchool.has(s) && errors.push({ field: `/exclude_student_ids/${j}`, code: 'NOT_IN_SCHOOL' }));
        if (errors.length) return problem(422, 'VALIDATION_FAILED', { errors });
        const grade = (b.targets as Row[]).find((t) => t.target_type === 'grade')?.grade_level;
        const include = (b.targets as Row[]).flatMap((t) => (t.student_ids as string[] | undefined) ?? []);
        const exclude = new Set((b.exclude_student_ids as string[] | undefined) ?? []);
        const audience = new Set([...GRADE_STUDENTS.filter((s) => s.grade_level === grade).map((s) => s.student_id), ...include].filter((s) => !exclude.has(s)));
        const c = { ...b, challenge_id: newId(), tenant_id: tenant, kind: tpl.kind, format: tpl.format, status: 'scheduled', audience_count: audience.size, created_by: 'me', tournament_id: null };
        state.challenges.set(String(c.challenge_id), c);
        return { status: 201, body: c };
      });
    }
    if ((x = /^\/lms\/challenges\/([^/:]+)$/.exec(p)) && m === 'GET') {
      const c = tenant === T_A ? state.challenges.get(x[1]!) : undefined;
      return c ? json(200, c) : problem(404, 'NOT_FOUND');
    }
    if ((x = /^\/lms\/challenges\/([^/:]+):cancel$/.exec(p)) && m === 'POST') {
      const c = tenant === T_A ? state.challenges.get(x[1]!) : undefined;
      if (!c) return problem(404, 'NOT_FOUND');
      return once(r, () => {
        if (!b.reason) return problem(422, 'VALIDATION_FAILED', { errors: [{ field: '/reason', code: 'REQUIRED' }] });
        Object.assign(c, { status: 'cancelled', cancel_reason: b.reason });
        return { status: 200, body: c };
      });
    }
    // tournaments of TH Demo A only
    const t = /^\/competition\/tournaments\/([^/:]+)/.exec(p);
    if (!t || t[1] !== CH_IDS.tournament || tenant !== T_A) return problem(404, 'NOT_FOUND');
    const rest = p.slice(t[0].length);
    if (rest === '' && m === 'GET') return json(200, tour());
    if (rest === '/bracket' && m === 'GET') return json(200, bracket());
    if (rest === ':publish' && m === 'POST') return once(r, () => ((state.tournament.status = 'registration_open'), { status: 200, body: tour() }));
    if (rest === ':cancel' && m === 'POST')
      return once(r, () => (b.reason ? (Object.assign(state.tournament, { status: 'cancelled', cancel_reason: b.reason }), { status: 200, body: tour() }) : problem(422, 'VALIDATION_FAILED', { errors: [{ field: '/reason', code: 'REQUIRED' }] })));
    if (rest === '/entries' && m === 'POST')
      return once(r, () => {
        // organiser with an empty body = the whole audience of the challenge (TournamentEntryCreate)
        const ids = ((b.student_ids as string[] | undefined) ?? AUDIENCE).filter((s) => !state.entries.some((e) => (e.players as Row[])[0]!.student_id === s));
        const created = ids.map((s) => entryOf(s));
        state.entries.push(...created);
        return { status: 201, body: { entries: created, skipped: [{ student_id: IDS.studentSapBiDon, reason: 'not_eligible' }] } };
      });
    if (rest === '/entries:confirm' && m === 'POST')
      return once(r, () => {
        for (const e of state.entries) if (e.status === 'registered') e.status = 'confirmed';
        return { status: 200, body: { entries: state.entries } };
      });
    if ((x = /^\/entries\/([^/:]+)$/.exec(rest)) && m === 'DELETE') {
      const e = state.entries.find((y) => y.entry_id === x![1]);
      if (!e) return problem(404, 'NOT_FOUND');
      e.status = 'withdrawn';
      return json(204, null);
    }
    if (rest === ':generate-bracket' && m === 'POST')
      return once(r, () => {
        state.nodes = generatedNodes();
        state.tournament.status = 'scheduled';
        return { status: 200, body: bracket() };
      });
    if ((x = /^\/nodes\/([^/:]+):confirm$/.exec(rest)) && m === 'POST') {
      const n = state.nodes.find((y) => y.node_id === x![1]);
      if (!n) return problem(404, 'NOT_FOUND');
      n.status = 'confirmed';
      return json(200, n);
    }
    if ((x = /^\/nodes\/([^/:]+):override$/.exec(rest)) && m === 'POST') {
      const n = state.nodes.find((y) => y.node_id === x![1]);
      if (!n) return problem(404, 'NOT_FOUND');
      return once(r, () => {
        if (!b.reason) return problem(422, 'VALIDATION_FAILED', { errors: [{ field: '/reason', code: 'REQUIRED' }] });
        if (b.winner_entry_id !== n.entry_a_id && b.winner_entry_id !== n.entry_b_id) return problem(422, 'VALIDATION_FAILED', { errors: [{ field: '/winner_entry_id', code: 'NOT_IN_MATCH' }] });
        Object.assign(n, { status: 'overridden', winner_entry_id: b.winner_entry_id });
        return { status: 200, body: n };
      });
    }
    return problem(404, 'NOT_FOUND');
  });
  return state;
}

/** School admin of TH Demo A (role school_admin: every permission of workspace C, incl. lms.challenge.write). */
export const adTruongA = (): MockAccount => accountWith('ad.truong.a', TENANTS.thDemoA, SCHOOL_PERMS, '0193a4d2-1111-7000-8000-000000000001');
/** academic_manager: workspace C without lms.challenge.write (ADMIN_PORTAL §2.3, ADR-0072). */
export const academicManager = (): MockAccount =>
  accountWith(
    'qlcm.truong.a',
    TENANTS.thDemoA,
    SCHOOL_PERMS.filter((p) => p !== 'lms.challenge.write'),
    '0193a4d2-1111-7000-8000-000000000002',
  );
/** tenant_admin (ad.donvi): read-only LMS, no lms.challenge.write. */
export const tenantAdmin = (): MockAccount =>
  accountWith('ad.donvi', TENANTS.thDemoA, ['lms.student.read', 'lms.assignment.read', 'tenant.school.read', 'tenant.student.read', 'learning_content.read'], '0193a4d2-1111-7000-8000-000000000007');

export interface RenderChallengesOptions {
  as?: string;
  account?: MockAccount | null;
  disabledMenus?: string[];
  disabledActions?: string[];
  setup?: (s: ChallengesMockState) => void;
}

export async function renderChallenges(url: string, opts: RenderChallengesOptions = {}) {
  const backend = new MockPortalBackend().withSession(opts.as ?? 'ad.truong.a');
  if (opts.account !== null) backend.user = opts.account ?? adTruongA();
  backend.disabledMenus = opts.disabledMenus ?? [];
  backend.disabledActions = opts.disabledActions ?? [];
  installSchoolLmsMock(backend);
  // subjects of the tenant (subject_id parameter)
  backend.overrides.unshift((r) => (r.method === 'GET' && r.path === '/learning/subjects' ? json(200, { items: [{ subject_id: IDS.subjectMath, code: 'math', name: { vi: 'Toán' } }] }) : undefined));
  const state = installChallengesMock(backend);
  opts.setup?.(state);
  setViewportWidth(1440);
  const r = await renderPortal({ url, backend });
  return { ...r, state };
}

export { IDS, TENANTS, problem };
export { stopPortals } from '../../../../test/portal-harness';
