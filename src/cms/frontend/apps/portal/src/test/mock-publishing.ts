// MOCK (test only, AGENTS R17): in-memory stand-in of the learning CMS release / review / media / builder / import
// endpoints and the tenant Thùng rác used by the F054 screens — versions with row-version ETags (If-Match 428/412),
// releases with Idempotency-Key replay (one release per key), scheduled publish + cancel, rollback / unpublish /
// archive, usages, review tasks by lane and tenant, preview sessions scored without attempts, media, question sets /
// lessons / blueprints with pool stats, imports (dry run / real run), tenant trash. Tenant isolation: another tenant's
// item answers 404; a missing permission answers 403. Plugged into MockPortalBackend.overrides. NOT a backend: the
// real rules live in module learning (F022–F027) and identity (F045).
import { cursorPage, problem, type MockAccount, type MockPortalBackend, type RecordedRequest } from './mock-backend';

export const P = (n: number) => `0193a4d2-5000-7000-8000-${String(n).padStart(12, '0')}`;

const json = (status: number, body: unknown, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json', ...headers } });

export type Status = 'draft' | 'in_review' | 'approved' | 'published' | 'archived' | 'rejected';

export interface PVersion {
  version: number;
  status: Status;
  row: number;
  author_id: string;
  reviewer_id?: string;
  published_by?: string;
  published_at?: string;
  publish_at?: string;
  change_note?: string;
  pinned?: number;
  audio_exempt_reason?: string;
  /** ReviewFieldComment[] of the latest rejection (OQ F054-1). */
  field_comments?: { path: string; comment: string }[];
  translation_status?: 'complete' | 'partial' | 'outdated';
  content: Record<string, unknown>;
}

export interface PItem {
  id: string;
  type: string;
  tenant_id: string | null;
  assigned?: boolean;
  title?: { vi?: string; en?: string };
  /** content_language of the item (default vi). */
  lang?: 'vi' | 'en';
  versions: PVersion[];
  current?: number;
  deleted?: boolean;
  usages?: Record<string, unknown>[];
  pool?: { rule_index: number; required: number; available: number; insufficient: boolean; warning: boolean; section_id?: string }[];
}

export interface PreviewItem {
  type: string;
  render_payload: Record<string, unknown>;
  correct: unknown;
  feedback?: Record<string, unknown>;
  hints?: Record<string, unknown>[];
  explanation?: Record<string, unknown>;
}

const DOC_BASES: Record<string, string> = { question_set: '/cms/learning/question-sets', lesson: '/cms/learning/lessons', assessment_blueprint: '/cms/learning/assessment-blueprints', question: '/cms/learning/questions' };

/** Account restricted to some permissions of one tenant (workspaces content_studio + school_lms). */
export function contentAccount(username: string, tenant: { tenant_id: string; code: string; name: string; tenant_type: string; status: string } | null, permissions: string[], sub = '0193a4d2-1111-7000-8000-0000000000e0'): MockAccount {
  return {
    username,
    email: `${username}@demo.edu.vn`,
    password: 'Demo@12345',
    sub,
    displayName: username,
    platform: !tenant,
    tenants: tenant ? [tenant as never] : [],
    perms: { [tenant ? tenant.tenant_id : 'platform']: { workspaces: ['content_studio', 'school_lms'], permissions } },
  };
}

export class MockPublishing {
  readonly items = new Map<string, PItem>();
  readonly releases = new Map<string, Record<string, unknown>[]>();
  readonly idem = new Map<string, Response>();
  /** Release rows really created (not replays) — SC-005. */
  created = 0;
  reviewTasks: Record<string, unknown>[] = [];
  media: Record<string, unknown>[] = [];
  previewItems: PreviewItem[] = [];
  readonly sessions = new Map<string, PreviewItem[]>();
  /** Attempts / learning events created by preview (must stay 0, ADR-0031). */
  attempts = 0;
  trash: Record<string, unknown>[] = [];
  /** listCmsCurricula / getCurriculumVersion of the topic pickers (F053 T066); none by default. */
  curricula: Record<string, unknown>[] = [];
  curriculumVersions: Record<string, unknown>[] = [];
  imports = new Map<string, Record<string, unknown>>();
  importErrors: Record<string, unknown>[] = [];
  /** Questions of a Word import (F028): ContentImportItem rows of listContentImportItems. */
  importItems: Record<string, unknown>[] = [];
  readonly fail: { method: string; path: RegExp; response: () => Response; once?: boolean }[] = [];
  private n = 500;
  /** Largest page the content lists answer (tests lower it to exercise the cursor; the backend allows 100). */
  listPageSize = 50;

  constructor(private readonly backend: MockPortalBackend) {
    backend.overrides.push((r) => this.handle(r));
  }

  add(item: Omit<PItem, 'versions'> & { versions: Partial<PVersion>[] }): PItem {
    const full: PItem = { ...item, versions: item.versions.map((v, i) => ({ version: i + 1, status: 'draft', row: 1, author_id: P(900), content: {}, ...v })) };
    this.items.set(item.id, full);
    return full;
  }

  private perms(r: RecordedRequest): string[] {
    const u = this.backend.user;
    if (!u) return [];
    return (u.perms[r.headers['x-tenant-id'] ?? 'platform'] ?? { permissions: [] }).permissions;
  }

  private visible(r: RecordedRequest, it: PItem | undefined): it is PItem {
    if (!it || it.deleted) return false;
    const tenant = r.headers['x-tenant-id'] ?? null;
    if (it.tenant_id === null) return tenant === null || !!it.assigned;
    if (tenant === null) return this.perms(r).includes('platform.cross_tenant.read');
    return it.tenant_id === tenant;
  }

  private ifMatch(r: RecordedRequest, row: number): Response | null {
    const h = r.headers['if-match'];
    if (!h) return problem(428, 'PRECONDITION_REQUIRED');
    return h === `W/"${row}"` ? null : problem(412, 'VERSION_MISMATCH');
  }

  private summary(it: PItem, v: PVersion) {
    return { version: v.version, status: v.status, is_current: it.current === v.version, change_note: v.change_note, author_id: v.author_id, reviewer_id: v.reviewer_id, published_by: v.published_by, published_at: v.published_at, publish_at: v.publish_at, active_pinned_sessions: v.pinned ?? 0, ...(v.field_comments ? { field_comments: v.field_comments } : {}) };
  }

  private detail(it: PItem, v: PVersion, canReview: boolean) {
    const content = { ...v.content };
    if (!canReview) delete content.answer_key;
    return { content_type: it.type, content_id: it.id, version: v.version, status: v.status, tenant_id: it.tenant_id, assigned: !!it.assigned, author_id: v.author_id, reviewer_id: v.reviewer_id, published_at: v.published_at, content_language: 'vi', audio_exempt_reason: v.audio_exempt_reason, content };
  }

  private doc(it: PItem, v: PVersion) {
    return { content_type: it.type, content_id: it.id, version: v.version, status: v.status, tenant_id: it.tenant_id, assigned: !!it.assigned, author_id: v.author_id, content_language: it.lang ?? 'vi', translation_status: v.translation_status, title: it.title, ...v.content };
  }

  private release(it: PItem, action: string, extra: Record<string, unknown>) {
    const rel = { release_id: P(this.n++), item_type: it.type, item_id: it.id, action, released_by: this.backend.user?.sub ?? P(1), released_at: '2026-11-20T03:00:00Z', status: 'applied', ...extra };
    this.releases.set(it.id, [rel, ...(this.releases.get(it.id) ?? [])]);
    this.created++;
    return rel;
  }

  async handle(r: RecordedRequest): Promise<Response | undefined> {
    for (const f of this.fail) {
      if (f.method === r.method && f.path.test(r.path)) {
        if (f.once) this.fail.splice(this.fail.indexOf(f), 1);
        return f.response();
      }
    }
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

    // ---- release API (generic by content type) ----
    if ((m = p.match(/^\/cms\/learning\/content\/([a-z_]+)\/([^/:]+)(.*)$/))) {
      const it = this.items.get(m[2]!);
      if (!this.visible(r, it) || it.type !== m[1]) return problem(404, 'NOT_FOUND');
      const rest = m[3]!;
      let v: PVersion | undefined;
      let mm: RegExpMatchArray | null;
      if (rest === '/versions' && r.method === 'GET') {
        const status = new URLSearchParams(r.search).get('status');
        const data = [...it.versions].reverse().filter((x) => !status || x.status === status).map((x) => this.summary(it, x));
        return json(200, { item_id: it.id, ...(it.current ? { current_version: it.current } : {}), data, page: { has_more: false, limit: 100 } });
      }
      if (rest === '/releases' && r.method === 'GET') return json(200, { data: this.releases.get(it.id) ?? [], page: { has_more: false, limit: 100 } });
      if (rest === '/usages' && r.method === 'GET') return json(200, { data: it.usages ?? [] });
      if ((mm = rest.match(/^\/versions\/(\d+)(.*)$/))) {
        v = it.versions.find((x) => x.version === Number(mm![1]));
        if (!v) return problem(404, 'NOT_FOUND');
        const action = mm[2];
        if (action === '' && r.method === 'GET') return json(200, this.detail(it, v, perms.includes('learning_content.review')), { ETag: `W/"${v.row}"` });
        if (action === ':diff') {
          const w = it.versions.find((x) => x.version === Number(new URLSearchParams(r.search).get('with')));
          if (!w) return problem(404, 'NOT_FOUND');
          const changes = [
            { path: '/stem/blocks/0/text', op: 'changed', before: { vi: 'Cũ' }, after: { vi: 'Mới' } },
            { path: '/media_refs/0', op: 'added', after: P(77) },
            { path: '/difficulty', op: 'changed', before: 2, after: 3 },
            ...(perms.includes('learning_content.review') ? [{ path: '/answer_key/correct_option_id', op: 'changed', before: 'o_aaaa', after: 'o_bbbb' }] : []),
          ];
          return json(200, { from_version: w.version, to_version: v.version, changes });
        }
        if (action === ':publish' && r.method === 'POST') {
          if (!perms.includes('learning_content.publish')) return problem(403, 'PERMISSION_DENIED');
          const bad = this.ifMatch(r, v.row);
          if (bad) return bad;
          if (v.status !== 'approved') return problem(422, 'INVALID_STATE_TRANSITION');
          if (body.publish_at) {
            v.publish_at = body.publish_at as string;
            v.row++;
            return json(202, this.release(it, 'publish', { to_version: v.version, scheduled_for: body.publish_at, status: 'scheduled' }));
          }
          const from = it.current;
          v.status = 'published';
          v.published_at = '2026-11-20T03:00:00Z';
          v.published_by = this.backend.user?.sub;
          v.row++;
          it.current = v.version;
          return json(200, this.release(it, 'publish', { from_version: from, to_version: v.version }));
        }
        if (action === ':transition' && r.method === 'POST') {
          const bad = this.ifMatch(r, v.row);
          if (bad) return bad;
          const a = body.action as string;
          if ((a === 'approve' || a === 'reject') && !perms.includes('learning_content.review')) return problem(403, 'PERMISSION_DENIED');
          if ((a === 'approve' || a === 'reject') && v.author_id === this.backend.user?.sub) return problem(403, 'PERMISSION_DENIED');
          if (a === 'reject' && !body.note) return problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'note', code: 'REQUIRED' }] });
          const fc = body.field_comments as { path: string; comment: string }[] | undefined;
          if (fc !== undefined && (a !== 'reject' || fc.length > 50 || fc.some((c) => c.path.length > 256 || !c.comment || c.comment.length > 1000))) return problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'field_comments', code: 'INVALID' }] });
          if (a === 'approve' && v.audio_exempt_reason && body.audio_exemption_acknowledged !== true) return problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'audio_exemption_acknowledged', code: 'REQUIRED' }] });
          if (a === 'archive' && it.current === v.version) return problem(422, 'INVALID_STATE_TRANSITION');
          const next = ({ submit_review: 'in_review', approve: 'approved', reject: 'rejected', archive: 'archived' } as Record<string, Status>)[a];
          if (!next) return problem(422, 'INVALID_STATE_TRANSITION');
          v.status = next;
          if (a === 'approve' || a === 'reject') v.reviewer_id = this.backend.user?.sub;
          if (a === 'reject') v.field_comments = fc ?? [];
          v.row++;
          this.reviewTasks = this.reviewTasks.filter((t) => !(t.content_id === it.id && t.version === v!.version) || a === 'archive');
          return json(200, this.detail(it, v, true), { ETag: `W/"${v.row}"` });
        }
      }
      if (rest === ':rollback' && r.method === 'POST') {
        if (!perms.includes('content.release.rollback')) return problem(403, 'PERMISSION_DENIED');
        const target = it.versions.find((x) => x.version === body.to_version);
        if (!target || target.status !== 'published' || target.version === it.current) return problem(422, 'ROLLBACK_TARGET_INVALID');
        const from = it.current;
        it.current = target.version;
        return json(200, this.release(it, 'rollback', { from_version: from, to_version: target.version, reason: body.reason }));
      }
      if (rest === ':unpublish' && r.method === 'POST') {
        if (!perms.includes('content.release.rollback')) return problem(403, 'PERMISSION_DENIED');
        const from = it.current;
        it.current = undefined;
        return json(200, this.release(it, 'unpublish', { from_version: from, reason: body.reason }));
      }
      if ((mm = rest.match(/^\/releases\/([^/:]+):cancel$/)) && r.method === 'POST') {
        const rel = (this.releases.get(it.id) ?? []).find((x) => x.release_id === mm![1]);
        if (!rel || rel.status !== 'scheduled') return problem(422, 'VALIDATION_FAILED');
        rel.status = 'cancelled';
        const v2 = it.versions.find((x) => x.version === rel.to_version);
        if (v2) delete v2.publish_at;
        return json(200, rel);
      }
      return undefined;
    }

    // ---- review queue ----
    if (p === '/cms/learning/review-tasks' && r.method === 'GET') {
      if (!perms.includes('learning_content.review')) return problem(403, 'PERMISSION_DENIED');
      const q = new URLSearchParams(r.search);
      const tenant = r.headers['x-tenant-id'] ?? null;
      const data = this.reviewTasks.filter((t) => (tenant ? t.lane === 'tenant' && t.tenant_id === tenant : true) && (!q.get('lane') || t.lane === q.get('lane')) && (!q.get('content_type') || t.content_type === q.get('content_type')));
      return json(200, { data, page: { has_more: false, limit: 50 } });
    }

    // ---- preview ----
    if (p === '/cms/learning/preview/sessions' && r.method === 'POST') {
      const it = this.items.get(body.content_id as string);
      if (!this.visible(r, it)) return problem(404, 'NOT_FOUND');
      if (it.type === 'assessment_blueprint' && it.pool?.some((x) => x.insufficient)) return problem(422, 'QUESTION_POOL_INSUFFICIENT');
      const id = P(this.n++);
      this.sessions.set(id, this.previewItems);
      const lang = body.display_language === 'en-US' ? 'en' : 'vi';
      return json(201, {
        preview_session_id: id,
        expires_at: '2026-11-20T04:00:00Z',
        items: this.previewItems.map((x, i) => ({ position: i + 1, question_version_id: P(800 + i), type: x.type, render_payload: { ...x.render_payload, stem: { blocks: [{ type: 'paragraph', text: { vi: `Câu ${i + 1} (vi)`, en: `Question ${i + 1} (en)` }[lang] }] } } })),
      });
    }
    if ((m = p.match(/^\/cms\/learning\/preview\/sessions\/([^/]+)\/responses$/)) && r.method === 'POST') {
      const items = this.sessions.get(m[1]!);
      const item = items?.[(body.position as number) - 1];
      if (!item) return problem(404, 'NOT_FOUND');
      const resp = body.response as Record<string, unknown>;
      const ok = JSON.stringify(resp) === JSON.stringify(item.correct);
      const chosen = (resp.option_id as string | undefined) ?? '';
      return json(200, {
        position: body.position,
        question_version_id: P(800),
        is_correct: ok,
        score: ok ? 1 : 0,
        max_score: 1,
        ...(item.feedback?.[chosen] ? { feedback: [{ option_id: chosen, feedback: item.feedback[chosen] }] } : {}),
        hint_levels: (item.hints ?? []).map((h, i) => ({ level: i + 1, hint: h })),
        ...(item.explanation ? { explanation: item.explanation } : {}),
      });
    }

    // ---- media ----
    if (p === '/cms/media' && r.method === 'GET') {
      const q = new URLSearchParams(r.search);
      const data = this.media.filter((a) => (!q.get('kind') || a.kind === q.get('kind')) && (!q.get('status') || a.status === q.get('status')) && (!q.get('q') || String(a.original_filename).includes(q.get('q')!)));
      return json(200, { data, page: { has_more: false, limit: 50 } });
    }
    if ((m = p.match(/^\/cms\/media\/([^/:]+)$/)) && r.method === 'GET') {
      const a = this.media.find((x) => x.asset_id === m![1]);
      if (!a || (a.tenant_id && a.tenant_id !== (r.headers['x-tenant-id'] ?? a.tenant_id))) return problem(404, 'NOT_FOUND');
      if (a.status === 'processing' && a.next) a.status = a.next;
      return json(200, a);
    }
    if (p === '/cms/media:presign-upload' && r.method === 'POST') {
      if (!perms.includes('media.upload')) return problem(403, 'PERMISSION_DENIED');
      if (String(body.filename).includes('.bat')) return problem(422, 'MEDIA_TYPE_NOT_ALLOWED');
      const id = P(this.n++);
      this.media.push({ asset_id: id, kind: body.kind, status: 'quarantined', original_filename: body.filename, sha256: body.sha256, license: body.license, alt_text: body.alt_text, tenant_id: r.headers['x-tenant-id'] ?? null, next: String(body.filename).includes('exe') ? 'rejected' : 'ready', reject_reason: String(body.filename).includes('exe') ? 'type_mismatch' : undefined });
      return json(201, { asset_id: id, status: 'quarantined', upload_url: `https://storage.mock.test/quarantine/${id}`, upload_headers: {} });
    }
    if ((m = p.match(/^\/cms\/media\/([^/:]+):complete-upload$/)) && r.method === 'POST') {
      const a = this.media.find((x) => x.asset_id === m![1])!;
      a.status = 'processing';
      return json(200, a);
    }

    // ---- documents (question sets, lessons, blueprints, questions list / delete) ----
    for (const [type, base] of Object.entries(DOC_BASES)) {
      if (p === base && r.method === 'GET') {
        const q = new URLSearchParams(r.search);
        const tenantIds = q.getAll('tenant_ids');
        const data = [...this.items.values()]
          .filter((it) => it.type === type && this.visible(r, it) && (!tenantIds.length || tenantIds.includes(it.tenant_id ?? '')) && (!q.get('status') || it.versions.at(-1)!.status === q.get('status')) && (!q.get('content_language') || (it.lang ?? 'vi') === q.get('content_language')) && (!q.get('translation_status') || it.versions.at(-1)!.translation_status === q.get('translation_status')) && (!q.get('q') || JSON.stringify(it.title ?? {}).toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/đ/g, 'd').includes(q.get('q')!.toLowerCase())))
          .map((it) => ({ ...this.doc(it, it.versions.at(-1)!), content_version_id: P(700 + it.versions.length) }));
        // one cursor page (the backend cuts it in SQL — owner rule 2026-11-23)
        const capped = new URLSearchParams(q);
        capped.set('limit', String(Math.min(Number(q.get('limit') ?? 50), this.listPageSize)));
        return json(200, cursorPage(data, capped, 'data', 50));
      }
      if (p === base && r.method === 'POST' && type !== 'question') {
        if (!perms.includes('learning_content.write')) return problem(403, 'PERMISSION_DENIED');
        const id = P(this.n++);
        const it = this.add({ id, type, tenant_id: r.headers['x-tenant-id'] ?? null, title: body.title as PItem['title'], versions: [{ content: body, author_id: this.backend.user?.sub }] });
        return json(201, this.doc(it, it.versions[0]!), { ETag: 'W/"1"' });
      }
      const re = new RegExp(`^${base.replace(/[/-]/g, (c) => `\\${c}`)}/([^/:]+)(.*)$`);
      if ((m = p.match(re))) {
        const it = this.items.get(m[1]!);
        if (!this.visible(r, it) || it.type !== type) return type === 'question' ? undefined : problem(404, 'NOT_FOUND');
        const rest = m[2]!;
        const latest = it.versions.at(-1)!;
        if (rest === '' && r.method === 'GET') return json(200, this.doc(it, latest), { ETag: `W/"${latest.row}"` });
        if (rest === '' && r.method === 'DELETE') {
          if (!perms.includes('learning_content.write')) return problem(403, 'PERMISSION_DENIED');
          const bad = this.ifMatch(r, latest.row);
          if (bad) return bad;
          if (it.versions.some((x) => x.status === 'published') && !body.reason) return problem(422, 'VALIDATION_FAILED', { errors: [{ field: 'reason', code: 'REQUIRED' }] });
          it.deleted = true;
          this.trash.push({ resource_type: type, resource_id: it.id, tenant_id: it.tenant_id, title: it.title?.vi ?? it.id, deleted_at: '2026-11-20T03:00:00Z', deleted_by: this.backend.user?.sub, delete_reason: body.reason, restorable: true });
          return new Response(null, { status: 204 });
        }
        if ((m = rest.match(/^\/versions\/(\d+)$/)) && r.method === 'GET') {
          const v = it.versions.find((x) => x.version === Number(m![1]));
          return v ? json(200, this.doc(it, v), { ETag: `W/"${v.row}"` }) : problem(404, 'NOT_FOUND');
        }
        if ((m = rest.match(/^\/versions\/(\d+)$/)) && r.method === 'PATCH') {
          const v = it.versions.find((x) => x.version === Number(m![1]))!;
          if (v.status !== 'draft' && v.status !== 'rejected') return problem(409, 'CONTENT_NOT_EDITABLE');
          const bad = this.ifMatch(r, v.row);
          if (bad) return bad;
          v.content = { ...v.content, ...body };
          if (body.title) it.title = body.title as PItem['title'];
          v.row++;
          return json(200, this.doc(it, v), { ETag: `W/"${v.row}"` });
        }
        if (rest === '/versions' && r.method === 'POST') {
          const from = it.versions.find((x) => x.version === body.from_version)!;
          const v: PVersion = { version: it.versions.length + 1, status: 'draft', row: 1, author_id: this.backend.user?.sub ?? P(1), content: { ...from.content } };
          it.versions.push(v);
          return json(201, this.doc(it, v), { ETag: 'W/"1"' });
        }
        if (/^\/versions\/\d+:pool-stats$/.test(rest)) return json(200, { rules: it.pool ?? [] });
      }
    }

    // ---- imports ----
    if (p === '/cms/learning/imports' && r.method === 'POST') {
      if (!perms.includes('learning_content.import')) return problem(403, 'PERMISSION_DENIED');
      const id = P(this.n++);
      const job = { import_id: id, kind: body.kind, status: 'queued', dry_run: body.dry_run, created_at: '2026-11-20T03:00:00Z', polls: 0 };
      this.imports.set(id, job);
      return json(202, job);
    }
    if ((m = p.match(/^\/cms\/learning\/imports\/([^/]+)\/items$/)) && r.method === 'GET') {
      if (!perms.includes('learning_content.import')) return problem(403, 'PERMISSION_DENIED');
      if (!this.imports.get(m[1]!)) return problem(404, 'NOT_FOUND');
      const q = new URLSearchParams(r.search);
      const rows = this.importItems.filter((it) => !q.get('status') || it.status === q.get('status'));
      const start = Number(q.get('cursor') ?? 0);
      const limit = Number(q.get('limit') ?? 20);
      const more = start + limit < rows.length;
      return json(200, { data: rows.slice(start, start + limit), page: { has_more: more, limit, ...(more ? { next_cursor: String(start + limit) } : {}) } });
    }
    if ((m = p.match(/^\/cms\/learning\/imports\/([^/]+)\/items\/(\d+):create-draft$/)) && r.method === 'POST') {
      if (!perms.includes('learning_content.import')) return problem(403, 'PERMISSION_DENIED');
      const it = this.importItems.find((x) => x.item_no === Number(m![2]));
      if (!it) return problem(404, 'NOT_FOUND');
      if (it.status === 'created' || it.status === 'error') {
        return problem(409, 'CONFLICT', { errors: [{ field: 'item_no', code: it.status === 'created' ? 'IMPORT_ITEM_ALREADY_CREATED' : 'IMPORT_ITEM_NOT_IMPORTABLE' }] });
      }
      const sent = ((body.math_assets as { latex: string }[] | undefined) ?? []).map((a) => a.latex).sort();
      const want = ((it.formulas as { latex: string }[] | undefined) ?? []).map((f) => f.latex).sort();
      if (JSON.stringify(sent) !== JSON.stringify(want)) return problem(422, 'MATH_RENDER_FAILED');
      it.status = 'created';
      it.question_id = P(this.n++);
      return json(201, it);
    }
    if ((m = p.match(/^\/cms\/learning\/imports\/([^/]+)$/)) && r.method === 'GET') {
      const job = this.imports.get(m[1]!);
      if (!job) return problem(404, 'NOT_FOUND');
      if (job.kind === 'lessons_docx') {
        const ok = this.importItems.filter((it) => it.status === 'ready' || it.status === 'warning').length;
        const n = this.importItems.length;
        Object.assign(job, { status: n ? 'succeeded' : 'failed', created_count: ok, rejected_count: n - ok, result: { items_total: n, items_importable: ok },
          errors: n ? [] : [{ row: 1, field: 'file', code: 'NO_QUESTIONS', message: 'no question' }] });
        return json(200, job);
      }
      job.status = 'succeeded';
      job.errors = this.importErrors;
      job.rejected_count = new Set(this.importErrors.map((e) => e.row)).size;
      job.created_count = job.dry_run ? 0 : 18;
      return json(200, job);
    }

    // ---- tenant trash ----
    if (p === '/tenant/trash' && r.method === 'GET') {
      if (!perms.includes('tenant.trash.read')) return problem(403, 'PERMISSION_DENIED');
      const tenant = r.headers['x-tenant-id'];
      const type = new URLSearchParams(r.search).get('resource_type');
      return json(200, { data: this.trash.filter((x) => x.tenant_id === tenant && (!type || x.resource_type === type)), page: { has_more: false, limit: 50 } });
    }
    if ((m = p.match(/^\/tenant\/trash\/([^/]+)\/([^/:]+):restore$/)) && r.method === 'POST') {
      if (!perms.includes('tenant.trash.restore')) return problem(403, 'PERMISSION_DENIED');
      const x = this.trash.find((t) => t.resource_id === m![2] && t.tenant_id === r.headers['x-tenant-id']);
      if (!x) return problem(404, 'NOT_FOUND');
      if (x.conflict) return problem(409, 'CONFLICT');
      this.trash = this.trash.filter((t) => t !== x);
      const it = this.items.get(x.resource_id as string);
      if (it) it.deleted = false;
      return json(200, { restored: true });
    }

    // ---- curriculum lists used by the filters / builders ----
    if (p === '/cms/learning/subjects') return json(200, { data: [{ subject_id: P(10), code: 'MATH', name: { vi: 'Toán', en: 'Math' } }], page: { has_more: false, limit: 100 } });
    if (p === '/cms/learning/curricula' && r.method === 'GET') return json(200, { data: this.curricula, page: { has_more: false, limit: 100 } });
    if ((m = p.match(/^\/cms\/learning\/curricula\/([^/]+)\/versions\/(\d+)$/)) && r.method === 'GET') {
      const v = this.curriculumVersions.find((x) => x.curriculum_id === m![1] && x.version === Number(m![2]));
      return v ? json(200, v) : problem(404, 'NOT_FOUND');
    }
    if (p === '/cms/learning/topics') return json(200, { data: [{ topic_id: P(20), code: 'T1', name: 'Phép nhân', sort_order: 1 }], page: { has_more: false, limit: 200 } });
    if (p === '/cms/learning/objectives')
      return json(200, {
        data: [
          { objective_id: P(31), code: 'LO1', description: 'Nhân 2' },
          { objective_id: P(32), code: 'LO2', description: 'Nhân 3' },
          { objective_id: P(33), code: 'LO3', description: 'Nhân 4' },
        ],
        page: { has_more: false, limit: 200 },
      });
    return undefined;
  }
}
