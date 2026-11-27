// MOCK (test only, AGENTS R17): in-memory stand-in of the workspace D operations of specs/openapi/eduworld-api.v1.yaml
// (identity module, F010/F016/F045) plugged into MockPortalBackend as overrides — tenants (cursor, `version` on list rows,
// create/update with ETag/If-Match), tenant assignments, clone jobs, trash, catalog (countries/provinces with `version`,
// listCatalogSchools with X-Tenant-Id or `tenant_ids[]` + `status`), school imports, platform settings, cleanup runs
// (approve, cancel with reason + Idempotency-Key replay).
// It records requests only; it is NOT a backend, enforces no authorization and is never imported by application code.
import { cursorPage, MockPortalBackend, problem, TENANTS, type RecordedRequest } from './mock-backend';

/**
 * MOCK (test only): jsdom's Blob/File has neither arrayBuffer() nor stream(), which Node's fetch Request needs to read
 * a multipart body built from jsdom's FormData (the school CSV upload); browsers have both. Filled from FileReader.
 */
function polyfillBlobReading(): void {
  if (typeof Blob === 'undefined' || typeof FileReader === 'undefined') return;
  if (typeof Blob.prototype.arrayBuffer !== 'function') {
    Blob.prototype.arrayBuffer = function arrayBuffer(this: Blob) {
      return new Promise<ArrayBuffer>((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(reader.result as ArrayBuffer);
        reader.onerror = () => reject(reader.error);
        reader.readAsArrayBuffer(this);
      });
    };
  }
  if (typeof Blob.prototype.stream !== 'function') {
    Blob.prototype.stream = function stream(this: Blob) {
      const read = () => this.arrayBuffer();
      return new ReadableStream<Uint8Array>({
        async start(controller) {
          controller.enqueue(new Uint8Array(await read()));
          controller.close();
        },
      });
    } as Blob['stream'];
  }
}
polyfillBlobReading();

const json = (status: number, body: unknown, headers: Record<string, string> = {}) =>
  new Response(body === undefined ? null : JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', ...headers } });

export const IDS = {
  question: '0193a4d2-2222-7000-8000-000000000001',
  questionSet: '0193a4d2-2222-7000-8000-000000000002',
  trashQuestion: '0193a4d2-3333-7000-8000-000000000001',
  trashConflict: '0193a4d2-3333-7000-8000-000000000002',
  trashPlatform: '0193a4d2-3333-7000-8000-000000000003',
  deletedBy: '0193a4d2-1111-7000-8000-000000000099',
  job: '0193a4d2-4444-7000-8000-000000000001',
  cloneCopy: '0193a4d2-4444-7000-8000-000000000002',
  importDry: '0193a4d2-5555-7000-8000-000000000001',
  importApply: '0193a4d2-5555-7000-8000-000000000002',
  runAwaiting: '0193a4d2-6666-7000-8000-000000000001',
  runDone: '0193a4d2-6666-7000-8000-000000000002',
  provinceHn: '0193a4d2-7777-7000-8000-000000000001',
  provinceOld: '0193a4d2-7777-7000-8000-000000000002',
  school: '0193a4d2-8888-7000-8000-000000000001',
} as const;

export const DISABLED_TENANT = { tenant_id: '0193a4d2-0000-7000-8000-00000000000d', code: 'th-ngung', name: 'TH Ngừng HĐ', tenant_type: 'school', status: 'disabled' } as const;

type Tenant = { tenant_id: string; code: string; name: string; tenant_type: string; status: string; school_count?: number; is_qa?: boolean };

export interface PlatformState {
  tenants: Tenant[];
  /** Page size the mock uses for listTenants (to exercise cursor paging). */
  tenantPageSize: number;
  versions: Map<string, number>;
  assignments: Map<string, { tenant_id: string; tenant_name: string; assigned_at: string }[]>;
  cloneFinal: 'succeeded' | 'partially_failed' | 'failed';
  clonePolls: number;
  trash: Record<string, unknown>[];
  cleanup: Record<string, unknown>;
  chat: Record<string, unknown>;
  curfew: Record<string, unknown>;
  runs: Record<string, unknown>[];
  /** Idempotency-Key → run id of a cancelStudentCleanupRun already applied (replay answers 204 again). */
  cancels: Map<string, string>;
  importPolls: number;
  /** Catalog rows served as cursor pages (keyset lists in the backend, owner rule 2026-11-23). */
  countries: Record<string, unknown>[];
  provinces: Record<string, unknown>[];
}

export function platformState(): PlatformState {
  return {
    tenants: [...Object.values(TENANTS), DISABLED_TENANT].map((t) => ({ ...t, school_count: 1 })),
    tenantPageSize: 100,
    versions: new Map(),
    assignments: new Map(),
    cloneFinal: 'succeeded',
    clonePolls: 0,
    trash: [
      { resource_type: 'question', resource_id: IDS.trashQuestion, tenant_id: TENANTS.thDemoA.tenant_id, title: 'Câu hỏi bảng nhân 7', deleted_at: '2026-11-18T08:00:00Z', deleted_by: IDS.deletedBy, delete_reason: 'Xóa nhầm', restorable: true },
      { resource_type: 'question', resource_id: IDS.trashConflict, tenant_id: TENANTS.thDemoA.tenant_id, title: 'Câu hỏi trùng mã', deleted_at: '2026-11-17T08:00:00Z', deleted_by: IDS.deletedBy, restorable: false },
      { resource_type: 'question', resource_id: IDS.trashPlatform, tenant_id: null, title: 'Câu hỏi platform', deleted_at: '2026-11-16T08:00:00Z', deleted_by: IDS.deletedBy, restorable: true },
    ],
    cleanup: { default_days: 180, notice_days: 14, allow_tenant_override: true, approval_threshold_percent: 10 },
    chat: { gameplay_chat_retention_days: 7, minimum_days: 3, ephemeral_retention_hours: 24, allow_tenant_override: true },
    curfew: { start: '22:00', end: '06:00', timezone: 'Asia/Ho_Chi_Minh', warning_minutes: 10, is_default: true, updated_at: null },
    runs: [
      { run_id: IDS.runAwaiting, tenant_id: TENANTS.thDemoA.tenant_id, status: 'awaiting_approval', candidates: 42, cleaned: 0, excluded: 3, skipped_active: 1, started_at: '2026-11-19T02:00:00Z', finished_at: null },
      { run_id: IDS.runDone, tenant_id: TENANTS.pgdDemo.tenant_id, status: 'completed', candidates: 5, cleaned: 5, excluded: 0, skipped_active: 0, started_at: '2026-11-18T02:00:00Z', finished_at: '2026-11-18T02:05:00Z' },
    ],
    cancels: new Map(),
    importPolls: 0,
    countries: [{ country_code: 'VN', name: { vi: 'Việt Nam', en: 'Viet Nam' }, default_locale: 'vi-VN', status: 'active' }],
    provinces: PROVINCES.map((p) => ({ ...p })),
  };
}

const PROVINCES = [
  { province_id: IDS.provinceHn, code: 'vn.ha-noi', country_code: 'VN', official_code: null, name: { vi: 'Hà Nội', en: 'Hanoi' }, short_name: { vi: 'Hà Nội', en: 'Hanoi' }, province_type: 'municipality', effective_from: '2025-07-01', effective_to: null, status: 'active', successor_code: null },
  { province_id: IDS.provinceOld, code: 'vn.old.ha-tay', country_code: 'VN', official_code: '20', name: { vi: 'Hà Tây', en: 'Ha Tay' }, short_name: { vi: 'Hà Tây', en: 'Ha Tay' }, province_type: 'province', effective_from: null, effective_to: '2008-08-01', status: 'inactive', successor_code: 'vn.ha-noi' },
];

const ifMatchOk = (rec: RecordedRequest, path: string, s: PlatformState) => {
  const v = rec.headers['if-match'];
  if (!v) return problem(428, 'PRECONDITION_REQUIRED');
  if (v !== `W/"${s.versions.get(path) ?? 1}"`) return problem(412, 'VERSION_MISMATCH');
  return null;
};

/** Installs the MOCK platform routes on `backend`; returns the mutable state for assertions. */
export function withPlatformMock(backend: MockPortalBackend, state: PlatformState = platformState()): PlatformState {
  const s = state;
  backend.overrides.push((rec) => {
    const q = new URLSearchParams(rec.search);
    const body = (rec.body ?? {}) as Record<string, unknown>;
    const route = `${rec.method} ${rec.path}`;
    // tenants (cursor = index)
    if (route === 'GET /admin/tenants') {
      const text = q.get('q')?.toLowerCase() ?? '';
      const type = q.get('tenant_type');
      const all = s.tenants.filter((t) => (!text || t.code.includes(text) || t.name.toLowerCase().includes(text)) && (!type || t.tenant_type === type));
      const from = Number(q.get('cursor') ?? 0);
      const size = Math.min(Number(q.get('limit') ?? s.tenantPageSize), s.tenantPageSize);
      const data = all.slice(from, from + size).map((t) => ({ ...t, version: s.versions.get(`/admin/tenants/${t.tenant_id}`) ?? 1 }));
      const more = from + size < all.length;
      return json(200, { data, page: { has_more: more, next_cursor: more ? String(from + size) : null, limit: size } });
    }
    if (route === 'POST /admin/tenants') {
      if (s.tenants.some((t) => t.code === body.code)) return problem(409, 'CONFLICT');
      const t = { tenant_id: `0193a4d2-0000-7000-8000-${String(s.tenants.length + 100).padStart(12, '0')}`, code: String(body.code), name: String(body.name), tenant_type: String(body.tenant_type), status: 'active' };
      s.tenants.push(t);
      s.versions.set(`/admin/tenants/${t.tenant_id}`, 1);
      return json(201, { ...t, keycloak_org_alias: body.keycloak_org_alias, created_at: '2026-11-19T08:00:00Z', updated_at: '2026-11-19T08:00:00Z' }, { ETag: 'W/"1"' });
    }
    const tenantPatch = /^\/admin\/tenants\/([^/]+)$/.exec(rec.path);
    if (rec.method === 'PATCH' && tenantPatch) {
      const bad = ifMatchOk(rec, rec.path, s);
      if (bad) return bad;
      const t = s.tenants.find((x) => x.tenant_id === tenantPatch[1]);
      if (!t) return problem(404, 'NOT_FOUND');
      // F134: is_qa of the merge patch; the MOCK mirrors the 422 of the common tenant (F134-AC-07) to exercise the screen
      if (typeof body.is_qa === 'boolean' && t.tenant_type === 'common') return problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'is_qa', code: 'invalid', message: 'common tenant' }] });
      Object.assign(t, body.name ? { name: body.name } : {}, body.status ? { status: body.status } : {}, typeof body.is_qa === 'boolean' ? { is_qa: body.is_qa } : {});
      const v = (s.versions.get(rec.path) ?? 1) + 1;
      s.versions.set(rec.path, v);
      return json(200, { ...t, keycloak_org_alias: 'x-alias', created_at: '2026-11-19T08:00:00Z', updated_at: '2026-11-19T09:00:00Z' }, { ETag: `W/"${v}"` });
    }
    // assignments
    const aKey = (rt: unknown, id: unknown) => `${rt}:${id}`;
    if (route === 'GET /admin/tenant-assignments') {
      const data = (s.assignments.get(aKey(q.get('resource_type'), q.get('resource_id'))) ?? []).map((a) => ({ ...a, resource_type: q.get('resource_type'), resource_id: q.get('resource_id') }));
      return json(200, { data, page: { has_more: false, limit: 50 } });
    }
    if (route === 'POST /admin/tenant-assignments') {
      const k = aKey(body.resource_type, body.resource_id);
      const ids = body.apply_to_all_tenants ? s.tenants.map((t) => t.tenant_id) : ((body.tenant_ids as string[]) ?? []);
      const list = s.assignments.get(k) ?? [];
      for (const id of ids) if (!list.some((a) => a.tenant_id === id)) list.push({ tenant_id: id, tenant_name: s.tenants.find((t) => t.tenant_id === id)?.name ?? id, assigned_at: '2026-11-19T08:00:00Z' });
      s.assignments.set(k, list);
      return json(200, { data: list.map((a) => ({ ...a, resource_type: body.resource_type, resource_id: body.resource_id })) });
    }
    if (route === 'DELETE /admin/tenant-assignments') {
      const k = aKey(body.resource_type, body.resource_id);
      s.assignments.set(k, (s.assignments.get(k) ?? []).filter((a) => !(body.tenant_ids as string[]).includes(a.tenant_id)));
      return new Response(null, { status: 204 });
    }
    // clone jobs
    if (route === 'POST /admin/tenant-clones') {
      return json(202, { job_id: IDS.job, status: 'queued', source_tenant_id: body.source_tenant_id, target_tenant_id: body.target_tenant_id, resource_type: body.resource_type });
    }
    if (route === `GET /admin/tenant-clones/${IDS.job}`) {
      s.clonePolls++;
      const status = s.clonePolls < 2 ? 'running' : s.cloneFinal;
      const done = status !== 'running';
      return json(200, {
        job_id: IDS.job,
        status,
        source_tenant_id: TENANTS.thDemoA.tenant_id,
        target_tenant_id: TENANTS.pgdDemo.tenant_id,
        resource_type: 'question_set',
        ...(done ? { cloned_count: 1, id_map: [{ resource_type: 'question_set', source_id: IDS.questionSet, clone_id: IDS.cloneCopy }] } : {}),
        ...(done && status !== 'succeeded' ? { errors: ['learning step 2: media missing'] } : {}),
      });
    }
    // trash
    if (route === 'GET /admin/trash') {
      const ids = q.getAll('tenant_ids');
      const data = s.trash.filter((x) => x.resource_type === q.get('resource_type') && (!ids.length || ids.includes(String(x.tenant_id))));
      return json(200, { data, page: { has_more: false, limit: 50 } });
    }
    const restore = /^\/admin\/trash\/([^/]+)\/([^/:]+):restore$/.exec(rec.path);
    if (rec.method === 'POST' && restore) {
      const item = s.trash.find((x) => x.resource_id === restore[2]);
      if (!item) return problem(404, 'NOT_FOUND');
      if (item.restorable === false) return problem(409, 'CONFLICT');
      s.trash = s.trash.filter((x) => x !== item);
      return new Response(null, { status: 204 });
    }
    // catalog
    const version = (path: string) => s.versions.get(path) ?? 1;
    const bump = (path: string) => {
      const v = version(path) + 1;
      s.versions.set(path, v);
      return v;
    };
    if (route === 'GET /catalog/countries') {
      const rows = s.countries.map((c) => ({ ...c, version: version(`/admin/catalog/countries/${String(c.country_code)}`) }));
      return json(200, cursorPage(rows, q), { ETag: 'W/"catalog-1"' });
    }
    if (rec.method === 'PATCH' && rec.path.startsWith('/admin/catalog/countries/')) {
      const bad = ifMatchOk(rec, rec.path, s);
      if (bad) return bad;
      const v = bump(rec.path);
      return json(200, { country_code: 'VN', name: { vi: 'Việt Nam', en: 'Viet Nam' }, default_locale: 'vi-VN', status: body.status ?? 'active', version: v }, { ETag: `W/"${v}"` });
    }
    if (route === 'GET /catalog/provinces') {
      const st = q.get('status') ?? 'active';
      const rows = s.provinces.filter((p) => st === 'all' || p.status === st).map((p) => ({ ...p, version: version(`/admin/catalog/provinces/${String(p.province_id)}`) }));
      return json(200, cursorPage(rows, q));
    }
    if (route === 'POST /admin/catalog/provinces') {
      const p = { ...PROVINCES[0], ...body, province_id: '0193a4d2-7777-7000-8000-000000000099' };
      s.versions.set(`/admin/catalog/provinces/${p.province_id}`, 1);
      return json(201, p, { ETag: 'W/"1"' });
    }
    if (rec.method === 'PATCH' && rec.path.startsWith('/admin/catalog/provinces/')) {
      const bad = ifMatchOk(rec, rec.path, s);
      if (bad) return bad;
      const v = bump(rec.path);
      return json(200, { ...PROVINCES[0], ...body, version: v }, { ETag: `W/"${v}"` });
    }
    if (route === 'POST /admin/catalog/province-successions') return json(201, body);
    if (route === 'GET /catalog/schools') {
      // X-Tenant-Id (tenant session) or tenant_ids[] (System Admin, CrossTenantScope); neither → 403 (OQ F057-4)
      const header = rec.headers['x-tenant-id'];
      const ids = header ? [header] : q.getAll('tenant_ids');
      if (!ids.length) return problem(403, 'PERMISSION_DENIED');
      const status = q.get('status');
      const items = ids
        .map((tenant, i) => ({
          school_id: `${IDS.school.slice(0, -2)}${String(i + 1).padStart(2, '0')}`,
          tenant_id: tenant,
          code: `th-${i + 1}`,
          name: { vi: i === 0 ? 'Trường TH A' : `Trường TH ${i + 1}` },
          school_level: 'primary',
          grade_min: 1,
          grade_max: 5,
          status: i === 0 ? 'pending_review' : 'active',
          is_virtual: false,
          source: 'import',
          province: { code: 'vn.ha-noi', name: { vi: 'Hà Nội', en: 'Hanoi' } },
        }))
        .filter((x) => !status || x.status === status);
      return json(200, { items, page: { has_more: false, limit: 50 } });
    }
    if (route === 'POST /admin/schools:import') {
      const dry = String(rec.body).includes('name="dry_run"\r\n\r\ntrue');
      return json(202, { import_id: dry ? IDS.importDry : IDS.importApply, status: 'queued', dry_run: dry, created_at: '2026-11-19T08:00:00Z' });
    }
    const imp = /^\/admin\/school-imports\/([^/]+)$/.exec(rec.path);
    if (rec.method === 'GET' && imp) {
      const dry = imp[1] === IDS.importDry;
      return json(200, {
        import_id: imp[1],
        status: 'succeeded',
        dry_run: dry,
        total_rows: 3,
        created_rows: dry ? 0 : 2,
        updated_rows: 0,
        error_rows: 1,
        errors: [{ row: 3, field: 'province_code', code: 'PROVINCE_INACTIVE', message: 'province vn.old.ha-tay' }],
        created_at: '2026-11-19T08:00:00Z',
      });
    }
    // settings (GET → ETag, PUT needs If-Match)
    const settings: Record<string, keyof PlatformState> = { '/admin/settings/student-cleanup': 'cleanup', '/admin/settings/chat-retention': 'chat', '/admin/settings/curfew': 'curfew' };
    const sk = settings[rec.path];
    if (sk && rec.method === 'GET') return json(200, s[sk], { ETag: `W/"${s.versions.get(rec.path) ?? 1}"` });
    if (sk && rec.method === 'PUT') {
      const bad = ifMatchOk(rec, rec.path, s);
      if (bad) return bad;
      if (sk === 'chat' && Number(body.gameplay_chat_retention_days) < 3) return problem(422, 'RETENTION_BELOW_MINIMUM');
      const { reason: _r, ...rest } = body;
      void _r;
      Object.assign(s[sk] as object, rest, sk === 'curfew' ? { is_default: false } : {});
      const v = (s.versions.get(rec.path) ?? 1) + 1;
      s.versions.set(rec.path, v);
      return json(200, s[sk], { ETag: `W/"${v}"` });
    }
    if (route === 'GET /admin/student-cleanup/runs') {
      const ids = q.getAll('tenant_ids');
      return json(200, { data: s.runs.filter((r) => !ids.length || ids.includes(String(r.tenant_id))), page: { has_more: false, limit: 50 } });
    }
    const approve = /^\/admin\/student-cleanup\/runs\/([^/:]+):approve$/.exec(rec.path);
    if (rec.method === 'POST' && approve) {
      const run = s.runs.find((r) => r.run_id === approve[1]);
      if (!run) return problem(404, 'NOT_FOUND');
      run.status = 'approved';
      return new Response(null, { status: 202 });
    }
    const cancel = /^\/admin\/student-cleanup\/runs\/([^/:]+):cancel$/.exec(rec.path);
    if (rec.method === 'POST' && cancel) {
      const key = rec.headers['idempotency-key'] ?? '';
      if (s.cancels.get(key) === cancel[1]) return new Response(null, { status: 204, headers: { 'Idempotent-Replayed': 'true' } });
      const reason = typeof body.reason === 'string' ? body.reason : '';
      if (reason.length < 3 || reason.length > 500) return problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'reason', code: 'INVALID' }] });
      const run = s.runs.find((r) => r.run_id === cancel[1]);
      if (!run) return problem(404, 'NOT_FOUND');
      if (run.status !== 'awaiting_approval') return problem(409, 'INVALID_STATE_TRANSITION');
      run.status = 'cancelled';
      s.cancels.set(key, String(run.run_id));
      return new Response(null, { status: 204 });
    }
    return undefined;
  });
  return s;
}

/** Renders the portal at `url` for `account` (default `sa.admin`) with the MOCK platform routes installed. */
export async function renderPlatform(url: string, opts: { account?: string; state?: PlatformState; disabledActions?: string[]; disabledMenus?: string[] } = {}) {
  const { renderPortal } = await import('./portal-harness');
  const backend = new MockPortalBackend().withSession(opts.account ?? 'sa.admin');
  backend.disabledActions = opts.disabledActions ?? [];
  backend.disabledMenus = opts.disabledMenus ?? [];
  const state = withPlatformMock(backend, opts.state);
  const rendered = await renderPortal({ url, backend });
  return { ...rendered, state };
}
