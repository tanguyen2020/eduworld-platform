// MOCK (test only, AGENTS R17): in-memory stand-in of the CMS activity API (F030/F058) and of the regrade requests
// (F031/F058) for the Activity Builder screens — activity types of the registry, activities stored as items of
// MockPublishing (so the generic release / preview endpoints apply), create with Idempotency-Key replay, draft PATCH
// with If-Match (428/412), CONTENT_NOT_EDITABLE on a published version, new version, a configurable ACTIVITY_CONFIG_INVALID
// answer, regrade preview / list / create / get, simulation templates (F066). NOT a backend: the rules live in module learning.
import { problem, type RecordedRequest } from './mock-backend';
import { P, type MockPublishing, type PItem, type PVersion } from './mock-publishing';

const json = (status: number, body: unknown, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', ...headers } });

export const ACTIVITY_TYPES = [
  { activity_type: 'interactive_exercise', label: 'Interactive exercise', schema_url: 'https://schemas.eduworld.vn/content/activities/interactive-exercise.schema.json', schema_version: 1, evaluation_strategies: ['auto'], min_client_version: '0.1.0', status: 'enabled' },
  { activity_type: 'quiz', label: 'Quiz', schema_url: 'https://schemas.eduworld.vn/content/activities/quiz.schema.json', schema_version: 1, evaluation_strategies: ['auto'], min_client_version: '0.1.0', status: 'enabled' },
  { activity_type: 'video_lesson', label: 'Video lesson', schema_url: 'https://schemas.eduworld.vn/content/activities/video-lesson.schema.json', schema_version: 1, evaluation_strategies: ['completion', 'auto'], min_client_version: '0.1.0', status: 'enabled' },
  { activity_type: 'audio_lesson', label: 'Audio lesson', schema_url: 'https://schemas.eduworld.vn/content/activities/audio-lesson.schema.json', schema_version: 1, evaluation_strategies: ['completion', 'auto'], min_client_version: '0.1.0', status: 'enabled' },
  { activity_type: 'simulation', label: 'Simulation', schema_url: 'https://schemas.eduworld.vn/content/activities/simulation.schema.json', schema_version: 1, evaluation_strategies: ['deterministic_check', 'auto'], min_client_version: '0.1.0', status: 'enabled' },
  { activity_type: 'speaking', label: 'Speaking', schema_url: 'https://schemas.eduworld.vn/content/activities/speaking.schema.json', schema_version: 1, evaluation_strategies: ['auto'], min_client_version: '9.9.9', status: 'enabled' },
  { activity_type: 'minigame', label: 'Minigame', schema_url: 'https://schemas.eduworld.vn/content/activities/minigame.schema.json', schema_version: 1, evaluation_strategies: ['auto'], min_client_version: '0.1.0', status: 'disabled' },
];

/** MOCK copy of the physics.lever v1 descriptor (module learning owns the real one) for listSimulationTemplates. */
export const LEVER_TEMPLATE = {
  template_code: 'physics.lever',
  template_version: 1,
  title: 'Cân bằng đòn bẩy',
  simulation_kind: 'physics',
  status: 'active',
  checker_available: true,
  min_client_version: '0.1.0',
  parameter_schema: {
    type: 'object',
    required: ['arm_length_m', 'slots_per_side', 'mass_1_kg', 'mass_2_kg'],
    properties: {
      arm_length_m: { type: 'number', minimum: 0.2, maximum: 5, title: 'Chiều dài mỗi cánh tay đòn (m)' },
      slots_per_side: { type: 'integer', minimum: 1, maximum: 10, title: 'Số ô đặt vật mỗi bên' },
      mass_1_kg: { type: 'number', minimum: 0.1, maximum: 50, title: 'Khối lượng vật 1 (kg)' },
      mass_2_kg: { type: 'number', minimum: 0.1, maximum: 50, title: 'Khối lượng vật 2 (kg)' },
    },
  },
  actions: [{ action: 'place_mass' }, { action: 'release' }],
  metrics: [
    { metric: 'torque_diff', unit: 'N·m' },
    { metric: 'balanced', unit: '' },
  ],
};

export interface MockRegrade {
  regrade_request_id: string;
  scope: string;
  action: string;
  status: string;
  affected_attempts: number;
  re_evaluated_attempts: number;
  reason: string;
  requested_by: string;
  created_at: string;
  question_version_id?: string;
  activity_version_id?: string;
}

export class MockActivities {
  types: Record<string, unknown>[] = ACTIVITY_TYPES.map((x) => ({ ...x }));
  /** Questions returned by GET /cms/learning/questions (the Question Bank picker). */
  questions: Record<string, unknown>[] = [];
  /** Next answer of a create / PATCH (e.g. 422 ACTIVITY_CONFIG_INVALID with errors[]). */
  nextWrite: Response | null = null;
  /** Attempts affected by a regrade preview, by version id. */
  affected: Record<string, number> = {};
  regrades: MockRegrade[] = [];
  /** Templates returned by listSimulationTemplates. */
  simulationTemplates: Record<string, unknown>[] = [LEVER_TEMPLATE];
  /** Status the next getRegradeRequest moves a request to (queued → running → completed by default). */
  progress: string[] = ['running', 'completed'];
  private readonly idem = new Map<string, Response>();
  private n = 800;

  constructor(private readonly pub: MockPublishing) {
    (pub as unknown as { backend: { overrides: ((r: RecordedRequest) => Promise<Response | undefined>)[] } }).backend.overrides.unshift((r) => this.handle(r));
  }

  /** Adds an activity (item of type activity whose version content is the ActivityDefinitionInput body). */
  add(id: string, versions: Partial<PVersion>[], extra: Partial<PItem> = {}): PItem {
    return this.pub.add({ id, type: 'activity', tenant_id: null, ...extra, versions });
  }

  private perms(r: RecordedRequest): string[] {
    return (this.pub as unknown as { perms: (r: RecordedRequest) => string[] }).perms(r);
  }

  private definition(r: RecordedRequest, it: PItem, v: PVersion): Record<string, unknown> {
    const c = v.content as Record<string, unknown>;
    const tenant = r.headers['x-tenant-id'] ?? null;
    return {
      activity_id: it.id,
      activity_version_id: `${it.id.slice(0, -4)}${String(v.version).padStart(4, '0')}`,
      version: v.version,
      status: v.status,
      owner_scope: it.tenant_id ? 'tenant' : 'platform',
      ...(it.tenant_id ? { tenant_id: it.tenant_id } : {}),
      current_published_version: it.current ?? null,
      author_id: v.author_id,
      assigned: !!it.assigned || (tenant !== null && it.tenant_id === null),
      ...c,
    };
  }

  async handle(r: RecordedRequest): Promise<Response | undefined> {
    const key = r.headers['idempotency-key'];
    const idemKey = key ? `${r.method} ${r.path}:${key}` : '';
    if (key && this.idem.has(idemKey)) return this.idem.get(idemKey)!.clone();
    const res = this.route(r);
    if (res && key && res.ok) this.idem.set(idemKey, res.clone());
    return res;
  }

  private route(r: RecordedRequest): Response | undefined {
    const p = r.path;
    const body = (r.body ?? {}) as Record<string, unknown>;
    const perms = this.perms(r);
    let m: RegExpMatchArray | null;
    if (p === '/cms/learning/activity-types' && r.method === 'GET') return json(200, { data: this.types });
    if (p === '/cms/learning/simulation-templates' && r.method === 'GET') return json(200, { data: this.simulationTemplates });
    if (p === '/cms/learning/questions' && r.method === 'GET' && this.questions.length > 0) return json(200, { data: this.questions, page: { has_more: false, limit: 50 } });
    if (p === '/cms/learning/activities' && r.method === 'GET') {
      const q = new URLSearchParams(r.search);
      const data = [...this.pub.items.values()]
        .filter((it) => it.type === 'activity' && !it.deleted)
        .map((it) => this.definition(r, it, it.versions.at(-1)!))
        .filter((d) => (!q.get('activity_type') || d.activity_type === q.get('activity_type')) && (!q.get('status') || d.status === q.get('status')));
      return json(200, { data, page: { has_more: false, limit: 50 } });
    }
    if (p === '/cms/learning/activities' && r.method === 'POST') {
      if (!perms.includes('learning_content.activity.write')) return problem(403, 'PERMISSION_DENIED');
      if (!r.headers['idempotency-key']) return problem(428, 'PRECONDITION_REQUIRED');
      if (this.nextWrite) {
        const w = this.nextWrite;
        this.nextWrite = null;
        return w;
      }
      const id = P(this.n++);
      const it = this.add(id, [{ status: 'draft', author_id: this.pub_user(), content: { ...body } }], { tenant_id: r.headers['x-tenant-id'] ?? (body.tenant_id as string) ?? null });
      return json(201, this.definition(r, it, it.versions[0]!), { ETag: 'W/"1"' });
    }
    if ((m = p.match(/^\/cms\/learning\/activities\/([^/:]+)$/))) {
      const it = this.pub.items.get(m[1]!);
      if (!it || it.type !== 'activity' || it.deleted) return problem(404, 'NOT_FOUND');
      const v = it.versions.at(-1)!;
      if (r.method === 'GET') return json(200, this.definition(r, it, v), { ETag: `W/"${v.row}"` });
      if (r.method === 'PATCH') {
        if (!perms.includes('learning_content.activity.write')) return problem(403, 'PERMISSION_DENIED');
        const h = r.headers['if-match'];
        if (!h) return problem(428, 'PRECONDITION_REQUIRED');
        if (h !== `W/"${v.row}"`) return problem(412, 'VERSION_MISMATCH');
        if (v.status !== 'draft' && v.status !== 'rejected') return problem(409, 'CONTENT_NOT_EDITABLE');
        if (this.nextWrite) {
          const w = this.nextWrite;
          this.nextWrite = null;
          return w;
        }
        v.content = { ...body };
        v.row++;
        return json(200, this.definition(r, it, v), { ETag: `W/"${v.row}"` });
      }
    }
    if ((m = p.match(/^\/cms\/learning\/activities\/([^/:]+)\/versions$/)) && r.method === 'POST') {
      const it = this.pub.items.get(m[1]!);
      if (!it) return problem(404, 'NOT_FOUND');
      const from = it.versions.find((x) => x.version === body.from_version) ?? it.versions.at(-1)!;
      const v: PVersion = { version: it.versions.length + 1, status: 'draft', row: 1, author_id: this.pub_user(), content: { ...from.content } };
      it.versions.push(v);
      return json(201, this.definition(r, it, v), { ETag: 'W/"1"' });
    }
    // ---- regrade ----
    if (p.startsWith('/cms/learning/regrade-requests')) {
      if (!perms.includes('learning_content.re_evaluate')) return problem(403, 'PERMISSION_DENIED');
      const target = (body.question_version_id ?? body.activity_version_id) as string | undefined;
      if (p === '/cms/learning/regrade-requests:preview' && r.method === 'POST') return json(200, { affected_attempts: this.affected[target ?? ''] ?? 0 });
      if (p === '/cms/learning/regrade-requests' && r.method === 'GET') {
        const q = new URLSearchParams(r.search);
        const v = q.get('question_version_id') ?? q.get('activity_version_id');
        if (!v) return problem(422, 'VALIDATION_FAILED');
        return json(200, { data: this.regrades.filter((x) => x.question_version_id === v || x.activity_version_id === v) });
      }
      if (p === '/cms/learning/regrade-requests' && r.method === 'POST') {
        if (!r.headers['idempotency-key']) return problem(428, 'PRECONDITION_REQUIRED');
        const req: MockRegrade = {
          regrade_request_id: P(this.n++),
          scope: body.scope as string,
          action: body.action as string,
          status: 'queued',
          affected_attempts: this.affected[target ?? ''] ?? 0,
          re_evaluated_attempts: 0,
          reason: body.reason as string,
          requested_by: this.pub_user(),
          created_at: '2026-11-20T03:00:00Z',
          ...(body.question_version_id ? { question_version_id: body.question_version_id as string } : {}),
          ...(body.activity_version_id ? { activity_version_id: body.activity_version_id as string } : {}),
        };
        this.regrades.unshift(req);
        return json(202, req);
      }
      if ((m = p.match(/^\/cms\/learning\/regrade-requests\/([^/:]+)$/)) && r.method === 'GET') {
        const req = this.regrades.find((x) => x.regrade_request_id === m![1]);
        if (!req) return problem(404, 'NOT_FOUND');
        const next = this.progress.shift();
        if (next) req.status = next;
        if (req.status === 'completed') req.re_evaluated_attempts = req.affected_attempts;
        return json(200, req);
      }
    }
    return undefined;
  }

  private pub_user(): string {
    return (this.pub as unknown as { backend: { user?: { sub: string } } }).backend.user?.sub ?? P(900);
  }
}
