// MOCK (test only, AGENTS R17): in-memory stand-in of the learning CMS endpoints used by Content Studio (F053) —
// questions + versions with row-version ETags (If-Match, 412), workflow transition / publish with Idempotency-Key
// replay, preview session, media, curriculum lists. Plugged into MockPortalBackend.overrides. NOT a backend: no
// validation beyond what the editor tests need; the real rules live in module learning (F021, F022, F025–F027).
import type { Handler, RecordedRequest } from './mock-backend';
import { problem } from './mock-backend';

export const U = (n: number) => `0193a4d2-0000-7000-8000-${String(n).padStart(12, '0')}`;

const json = (status: number, body: unknown, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', ...headers } });

export interface MockVersion {
  version: number;
  status: 'draft' | 'in_review' | 'approved' | 'published' | 'rejected' | 'archived';
  row: number;
  author_id: string;
  body: Record<string, unknown>;
}

export interface MockQuestion {
  id: string;
  tenant_id: string | null;
  owner_scope: 'platform' | 'tenant';
  assigned?: boolean;
  versions: MockVersion[];
}

export class MockContentBackend {
  readonly questions = new Map<string, MockQuestion>();
  readonly releases: { id: string; version: number; key: string }[] = [];
  readonly idem = new Map<string, Response>();
  readonly media = new Map<string, { asset_id: string; kind: string; status: string; original_filename: string; polls: number; alt_text?: string }>();
  topics: Record<string, unknown>[] = [];
  objectives: Record<string, unknown>[] = [];
  curricula: Record<string, unknown>[] = [];
  versions: Record<string, unknown>[] = [];
  lessons: Record<string, unknown>[] = [];
  /** Per-path failures to inject (first match answers). */
  readonly fail: { method: string; path: RegExp; response: () => Response; once?: boolean }[] = [];
  private n = 100;

  addQuestion(q: Omit<MockQuestion, 'versions'> & { versions: Partial<MockVersion>[] }): MockQuestion {
    const full: MockQuestion = { ...q, versions: q.versions.map((v, i) => ({ version: i + 1, status: 'draft', row: 1, author_id: U(900), body: {}, ...v })) };
    this.questions.set(q.id, full);
    return full;
  }

  view(q: MockQuestion, v: MockVersion) {
    return { content_type: 'question', content_id: q.id, version: v.version, status: v.status, owner_scope: q.owner_scope, tenant_id: q.tenant_id, assigned: !!q.assigned, author_id: v.author_id, content_language: 'vi', ...v.body };
  }

  private latest(q: MockQuestion) {
    return q.versions[q.versions.length - 1]!;
  }

  readonly handler: Handler = (r: RecordedRequest) => {
    for (const f of this.fail) {
      if (f.method === r.method && f.path.test(r.path)) {
        if (f.once) this.fail.splice(this.fail.indexOf(f), 1);
        return f.response();
      }
    }
    const key = r.headers['idempotency-key'];
    if (key && this.idem.has(`${r.path}:${key}`)) return this.idem.get(`${r.path}:${key}`)!.clone();
    const res = this.route(r);
    if (res && key && res.ok) this.idem.set(`${r.path}:${key}`, res.clone());
    return res;
  };

  private ifMatch(r: RecordedRequest, row: number): Response | null {
    const h = r.headers['if-match'];
    if (!h) return problem(428, 'PRECONDITION_REQUIRED');
    return h === `W/"${row}"` ? null : problem(412, 'VERSION_MISMATCH');
  }

  private route(r: RecordedRequest): Response | undefined {
    const p = r.path;
    let m: RegExpMatchArray | null;
    const body = (r.body ?? {}) as Record<string, unknown>;
    if (r.method === 'POST' && p === '/cms/learning/questions') {
      const id = U(this.n++);
      const q = this.addQuestion({ id, tenant_id: (body.tenant_id as string) ?? r.headers['x-tenant-id'] ?? null, owner_scope: (body.owner_scope as 'platform') ?? 'tenant', versions: [{ body }] });
      return json(201, this.view(q, this.latest(q)), { ETag: 'W/"1"' });
    }
    if ((m = p.match(/^\/cms\/learning\/questions\/([^/]+)$/)) && r.method === 'GET') {
      const q = this.questions.get(m[1]!);
      if (!q) return problem(404, 'NOT_FOUND');
      return json(200, this.view(q, this.latest(q)), { ETag: 'W/"77"' });
    }
    if ((m = p.match(/^\/cms\/learning\/questions\/([^/]+)\/versions\/(\d+)$/))) {
      const q = this.questions.get(m[1]!);
      const v = q?.versions.find((x) => x.version === Number(m![2]));
      if (!q || !v) return problem(404, 'NOT_FOUND');
      if (r.method === 'GET') return json(200, this.view(q, v), { ETag: `W/"${v.row}"` });
      if (r.method === 'PATCH') {
        if (v.status !== 'draft' && v.status !== 'rejected') return problem(409, 'CONTENT_NOT_EDITABLE');
        const bad = this.ifMatch(r, v.row);
        if (bad) return bad;
        v.body = { ...v.body, ...body };
        v.row++;
        return json(200, this.view(q, v), { ETag: `W/"${v.row}"` });
      }
    }
    if ((m = p.match(/^\/cms\/learning\/questions\/([^/]+)\/versions$/)) && r.method === 'POST') {
      const q = this.questions.get(m[1]!);
      if (!q) return problem(404, 'NOT_FOUND');
      const from = q.versions.find((x) => x.version === body.from_version)!;
      const v: MockVersion = { version: q.versions.length + 1, status: 'draft', row: 1, author_id: U(901), body: { ...from.body } };
      q.versions.push(v);
      return json(201, this.view(q, v), { ETag: 'W/"1"' });
    }
    if ((m = p.match(/^\/cms\/learning\/content\/question\/([^/]+)\/versions\/(\d+):(transition|publish)$/)) && r.method === 'POST') {
      const q = this.questions.get(m[1]!);
      const v = q?.versions.find((x) => x.version === Number(m![2]));
      if (!q || !v) return problem(404, 'NOT_FOUND');
      const bad = this.ifMatch(r, v.row);
      if (bad) return bad;
      if (m[3] === 'publish') {
        if (v.status !== 'approved') return problem(422, 'INVALID_STATE_TRANSITION');
        v.status = 'published';
        v.row++;
        this.releases.push({ id: q.id, version: v.version, key: r.headers['idempotency-key'] ?? '' });
        return json(200, { release_id: U(this.n++), item_type: 'question', item_id: q.id, action: 'publish', to_version: v.version, released_by: U(1), released_at: '2026-11-19T09:00:00Z' });
      }
      const next = { submit_review: 'in_review', approve: 'approved', reject: 'rejected' }[body.action as string] as MockVersion['status'];
      v.status = next;
      v.row++;
      return json(200, this.view(q, v), { ETag: `W/"${v.row}"` });
    }
    if (p === '/cms/learning/preview/sessions' && r.method === 'POST') {
      const q = this.questions.get(body.content_id as string)!;
      const v = q.versions.find((x) => x.version === body.version)!;
      const rp = JSON.parse(JSON.stringify(v.body.render_payload ?? {})) as { options?: Record<string, unknown>[] };
      rp.options?.forEach((o) => {
        delete o.feedback;
        delete o.misconception_tag;
      });
      return json(201, { preview_session_id: U(this.n++), expires_at: '2026-11-19T10:00:00Z', items: [{ position: 1, question_version_id: U(5), type: v.body.type, render_payload: rp }] });
    }
    if (p === '/cms/media:presign-upload' && r.method === 'POST') {
      const id = U(this.n++);
      this.media.set(id, { asset_id: id, kind: body.kind as string, status: 'quarantined', original_filename: body.filename as string, polls: 0, alt_text: body.alt_text as string | undefined });
      return json(201, { asset_id: id, status: 'quarantined', upload_url: `https://storage.mock.test/quarantine/${id}`, upload_headers: { 'Content-Type': body.mime_type as string } });
    }
    if ((m = p.match(/^\/cms\/media\/([^/:]+):complete-upload$/))) {
      const a = this.media.get(m[1]!)!;
      a.status = 'processing';
      return json(200, a);
    }
    if ((m = p.match(/^\/cms\/media\/([^/:]+)$/)) && r.method === 'GET') {
      const a = this.media.get(m[1]!);
      if (!a) return problem(404, 'NOT_FOUND');
      if (a.status === 'processing' && ++a.polls >= 2) a.status = 'ready';
      return json(200, { ...a, sha256: 'a'.repeat(64) });
    }
    if (p === '/cms/media' && r.method === 'GET') return json(200, { data: [...this.media.values()].map((a) => ({ ...a, sha256: 'a'.repeat(64) })), page: { has_more: false } });
    if (p === '/cms/learning/subjects') return json(200, { data: [{ subject_id: U(10), code: 'MATH', name: { vi: 'Toán', en: 'Math' } }], page: { has_more: false } });
    if (p === '/cms/learning/curricula' && r.method === 'GET') return json(200, { data: this.curricula, page: { has_more: false } });
    if (p === '/cms/learning/curricula' && r.method === 'POST') {
      const c = { curriculum_id: U(this.n++), code: body.code, name: body.name, owner_scope: 'tenant', latest_version_no: 1 };
      this.curricula.push(c);
      return json(201, c);
    }
    if ((m = p.match(/^\/cms\/learning\/curricula\/([^/]+)\/versions$/))) return json(200, { data: this.versions.filter((v) => v.curriculum_id === m![1]), page: { has_more: false } });
    if ((m = p.match(/^\/cms\/learning\/curricula\/([^/]+)\/versions\/(\d+)$/))) {
      const v = this.versions.find((x) => x.curriculum_id === m![1] && x.version === Number(m![2]));
      return v ? json(200, v, { ETag: 'W/"3"' }) : problem(404, 'NOT_FOUND');
    }
    if ((m = p.match(/^\/cms\/learning\/curricula\/([^/]+)\/versions\/(\d+):(transition|publish)$/))) {
      const v = this.versions.find((x) => x.curriculum_id === m![1] && x.version === Number(m![2]))!;
      if (m[3] === 'publish') {
        v.status = 'published';
        return json(200, v);
      }
      v.status = ({ submit_review: 'in_review', approve: 'approved', reject: 'draft' } as Record<string, string>)[body.action as string]!;
      return json(200, v);
    }
    if (p === '/cms/learning/topics' && r.method === 'GET') return json(200, { data: this.topics, page: { has_more: false } });
    if (p === '/cms/learning/topics' && r.method === 'POST') {
      const tp = { topic_id: U(this.n++), sort_order: 99, row_version: 1, ...body };
      this.topics.push(tp);
      return json(201, tp);
    }
    if ((m = p.match(/^\/cms\/learning\/topics\/([^/]+)$/)) && r.method === 'PATCH') {
      const tp = this.topics.find((x) => x.topic_id === m![1])!;
      const bad = this.ifMatch(r, tp.row_version as number);
      if (bad) return bad;
      Object.assign(tp, body, { row_version: (tp.row_version as number) + 1 });
      return json(200, tp);
    }
    if (p === '/cms/learning/objectives' && r.method === 'GET') return json(200, { data: this.objectives, page: { has_more: false } });
    if (p === '/cms/learning/objectives' && r.method === 'POST') {
      const lo = { objective_id: U(this.n++), row_version: 1, prerequisite_ids: [], competency_ids: [], ...body };
      this.objectives.push(lo);
      return json(201, lo);
    }
    if ((m = p.match(/^\/cms\/learning\/objectives\/([^/]+)$/)) && r.method === 'PATCH') {
      const lo = this.objectives.find((x) => x.objective_id === m![1])!;
      Object.assign(lo, body);
      return json(200, lo);
    }
    if (p === '/cms/learning/lessons' && r.method === 'GET') return json(200, { data: this.lessons, page: { has_more: false } });
    if ((m = p.match(/^\/cms\/learning\/lessons\/([^/]+)$/))) return json(200, this.lessons.find((l) => l.content_id === m![1]));
    return undefined;
  }
}
