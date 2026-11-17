// Sinh ma trận truy vết yêu cầu (RTM): FR → tài liệu thiết kế → service → operation OpenAPI → TC → AC/US.
// Nguồn: docs/00-overview/PRODUCT_REQUIREMENTS.md (FR/US/AC), docs/**/*.md (tham chiếu FR, bảng TC),
// specs/openapi/eduworld-api.v1.yaml (operation). Liên kết "tường minh" = có nhắc mã/đường dẫn; "suy luận" = theo domain.
// Chạy: node scripts/docs/build-rtm.mjs            → ghi docs/12-project/TRACEABILITY_MATRIX.md
//       node scripts/docs/build-rtm.mjs --check    → exit 1 nếu file hiện có lệch với kết quả sinh
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import yaml from 'js-yaml';

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
const DOCS = path.join(ROOT, 'docs');
const PRD = 'docs/00-overview/PRODUCT_REQUIREMENTS.md';
const OUT = 'docs/12-project/TRACEABILITY_MATRIX.md';
const SPEC = 'specs/openapi/eduworld-api.v1.yaml';
const CHECK = process.argv.includes('--check');

// Domain → module sở hữu (ADR-0036, AGENTS.md §6), tài liệu sở hữu, mẫu path OpenAPI, tiền tố TC.
const DOMAINS = {
  AUTH: { svc: 'module identity', docs: ['09-identity/KEYCLOAK_SSO.md', '09-identity/IDENTITY_MAPPING.md'], ops: /^\/(auth|me)\//, tc: ['AUTH'] },
  ORG: { svc: 'module identity', docs: ['06-architecture/MULTITENANCY.md', '09-identity/IDENTITY_MAPPING.md'], ops: /^\/(tenant\/(settings|schools|students|users)|admin\/(tenants|tenant-|catalog|schools|school-imports)|catalog\/|schools\/|students\/)/, tc: ['ORG', 'TEN'] },
  SIS: { svc: 'module identity', docs: ['09-identity/SIS_INTEGRATION.md', '09-identity/ACCOUNT_PROVISIONING.md'], ops: /sis/, tc: ['SIS'] },
  CUR: { svc: 'module learning', docs: ['02-learning/CURRICULUM_MODEL.md'], ops: /^\/learning\/(curricula|subjects|topics|objectives|lessons)/, tc: ['LRN'] },
  CNT: { svc: 'module learning', docs: ['02-learning/CONTENT_MODEL.md'], ops: /^\/cms\/(learning\/(content|translations)|media)/, tc: ['CNT'] },
  QB: { svc: 'module learning', docs: ['02-learning/QUESTION_BANK.md'], ops: /question/, tc: ['LRN'] },
  EXR: { svc: 'module learning', docs: ['02-learning/ACTIVITY_ENGINE.md'], ops: /activit|attempt|simulation|hint|regrade/, tc: ['ACT', 'LRN', 'VID', 'SIM', 'MATH'] },
  EXM: { svc: 'module learning', docs: ['02-learning/EXAM_ENGINE.md'], ops: /^\/exams\//, tc: ['EXM'] },
  PRG: { svc: 'module learning', docs: ['02-learning/LEARNING_ANALYTICS.md'], ops: /progress|recommendation|wrong-questions/, tc: ['LRN'] },
  ANL: { svc: 'module learning · module game', docs: ['02-learning/LEARNING_ANALYTICS.md', '11-operations/GAME_ANALYTICS.md', '07-backend/DASHBOARD_DATA.md'], ops: /analytics|dashboard|telemetry|\/reports/, tc: ['ANL', 'DASH'] },
  GAME: { svc: 'module game', docs: ['01-game-design/WORLD_DESIGN.md', '01-game-design/CHARACTER_SYSTEM.md'], ops: /^\/game\/(profile|avatar|world|sessions|client-config|content-manifest|levels|level-sessions)/, tc: ['WLD', 'GAM'] },
  QST: { svc: 'module game', docs: ['01-game-design/QUEST_SYSTEM.md'], ops: /quest/, tc: ['GAM'] },
  MG: { svc: 'module learning · game client', docs: ['08-game-client/MINIGAME_ENGINE.md'], ops: /minigame/, tc: ['ACT'] },
  MP: { svc: 'module competition · realtime', docs: ['03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md', '06-architecture/REALTIME_ARCHITECTURE.md'], ops: /^\/multiplayer\/rooms|realtime-tickets|world\/friends/, tc: ['PTY', 'CMP'] },
  PTY: { svc: 'module competition', docs: ['03-social-multiplayer/PARTY_SYSTEM.md'], ops: /^\/multiplayer\/(parties|invitations)/, tc: ['PTY'] },
  MM: { svc: 'module competition', docs: ['03-social-multiplayer/MATCHMAKING.md'], ops: /matchmaking/, tc: ['MM'] },
  CMP: { svc: 'module competition', docs: ['01-game-design/COMPETITION_SYSTEM.md'], ops: /^\/competition\/(duels|matches)/, tc: ['CMP'] },
  RNK: { svc: 'module competition', docs: ['01-game-design/COMPETITION_SYSTEM.md'], ops: /ratings/, tc: ['CMP'] },
  LDB: { svc: 'module competition', docs: ['01-game-design/COMPETITION_SYSTEM.md'], ops: /^\/leaderboards/, tc: ['CMP'] },
  TRN: { svc: 'module competition', docs: ['01-game-design/COMPETITION_SYSTEM.md'], ops: /tournament/, tc: ['CMP'] },
  SOC: { svc: 'module chat', docs: ['03-social-multiplayer/SOCIAL_SYSTEM.md'], ops: /^\/social\//, tc: ['SOC'] },
  CHAT: { svc: 'module chat', docs: ['03-social-multiplayer/CHAT_SYSTEM.md'], ops: /^\/chat\//, tc: ['CHAT'] },
  MOD: { svc: 'module chat', docs: ['03-social-multiplayer/MODERATION.md', '10-security/CHILD_SAFETY.md'], ops: /sensitive|moderation|\/chat\/reports|\/social\/(blocks|mutes)|chat\/categories/, tc: ['CHAT'] },
  NTF: { svc: 'module chat', docs: ['03-social-multiplayer/SOCIAL_SYSTEM.md'], ops: /^\/notifications/, tc: ['SOC'] },
  GAM: { svc: 'module game', docs: ['01-game-design/PROGRESSION_SYSTEM.md'], ops: /^\/game\/(xp|progression|achievements)/, tc: ['GAM'] },
  RWD: { svc: 'module game', docs: ['01-game-design/PROGRESSION_SYSTEM.md', '01-game-design/INVENTORY_ECONOMY.md'], ops: /^\/game\/(xp|progression|wallet|inventory)|economy/, tc: ['GAM', 'ECO'] },
  ECO: { svc: 'module game', docs: ['01-game-design/INVENTORY_ECONOMY.md'], ops: /^\/game\/(inventory|wallet|shop|collections|achievements)|economy/, tc: ['ECO'] },
  CHL: { svc: 'module game', docs: ['01-game-design/CHALLENGE_BOSS_SYSTEM.md'], ops: /challenge|boss/, tc: ['CHL'] },
  EVT: { svc: 'module game', docs: ['01-game-design/EVENT_SEASON_SYSTEM.md'], ops: /^\/game\/(events|community-goals|season-track)|^\/game-admin\/events|school-events|event-templates/, tc: ['EVT'] },
  CMS: { svc: 'module learning · module game (game-admin)', docs: ['04-admin-portal/ADMIN_PORTAL.md'], ops: /^\/(cms|game-admin|game-media)/, tc: ['CMS'] },
  LMS: { svc: 'module learning', docs: ['04-admin-portal/SCHOOL_LMS.md'], ops: /^\/lms\//, tc: ['LMS', 'DASH'] },
  AUD: { svc: 'module identity', docs: ['10-security/SECURITY_ARCHITECTURE.md'], ops: /trash|login-events|roles|permissions|user-roles/, tc: ['AUD'] },
  OPS: { svc: 'src/backend/pkg/ (platform kit) · mọi module', docs: ['11-operations/DEPLOYMENT.md', '11-operations/MONITORING.md'], ops: /settings|student-cleanup|chat-retention|users\/\{user_id\}:lock/, tc: ['SVC', 'PERF'] },
};
// Mã domain chỉ dùng cho AC/US/TC, ánh xạ về domain FR.
const ALIAS = { GDD: 'GAME', WLD: 'GAME', TEN: 'ORG', LRN: 'EXR', ACT: 'EXR', SIM: 'EXR', VID: 'EXR', MATH: 'EXR', DASH: 'ANL' };
// Tên domain khi PRD gộp nhiều mã dưới một heading.
const NAMES = { CHL: 'Challenge & Boss', ECO: 'Kinh tế, vật phẩm, sưu tầm', GAM: 'XP, level, tiến trình', RWD: 'Phần thưởng', AUD: 'Audit', OPS: 'Vận hành' };
const dname = (d) => NAMES[d] ?? domainName[d] ?? '';

const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');
const rel = (abs) => path.relative(ROOT, abs).split(path.sep).join('/');
function walk(dir, out = []) {
  for (const n of fs.readdirSync(dir).sort()) {
    const p = path.join(dir, n);
    if (fs.statSync(p).isDirectory()) walk(p, out); else if (n.endsWith('.md')) out.push(rel(p));
  }
  return out;
}

// Trích mã, mở rộng dạng khoảng "FR-GAM-001…003" / "FR-GAM-001..003".
const ID_RE = /\b(FR|US|AC|TC)-([A-Z][A-Z0-9]*)-(\d{3,4})(?!\d)(?:\s*(?:…|\.\.\.?|–)\s*(?:(?:FR|US|AC|TC)-[A-Z][A-Z0-9]*-)?(\d{3,4})(?!\d))?/g;
function idsIn(text) {
  const out = [];
  for (const m of text.matchAll(ID_RE)) {
    const a = Number(m[3]); const b = m[4] ? Number(m[4]) : a; const w = m[3].length;
    for (let i = a; i <= Math.min(b, a + 50); i++) out.push(`${m[1]}-${m[2]}-${String(i).padStart(w, '0')}`);
  }
  return out;
}
const domOf = (id) => { const d = id.split('-')[1]; return ALIAS[d] ?? d; };
const cmpId = (a, b) => a.localeCompare(b, 'en');

// 1) PRD: FR/US/AC (cột đầu bảng).
const prd = read(PRD);
const reqs = { FR: new Map(), US: new Map(), AC: new Map() };
const domainName = {};
let section = '';
for (const line of prd.split('\n')) {
  const h = line.match(/^###\s+[\d.a-z]+\.\s+(.*?)\s+—\s+(.*)$/);
  if (h) { section = h[1]; for (const c of h[2].matchAll(/`([A-Z]+)`/g)) domainName[c[1]] ??= section; continue; }
  const r = line.match(/^\|\s*((FR|US|AC)-[A-Z][A-Z0-9]*-\d{3})\s*\|(.*)$/);
  if (!r) continue;
  const cells = r[3].split('|').map((s) => s.trim());
  const text = cells[0] ?? '';
  const phase = (r[2] === 'FR' ? cells[2] : cells[cells.length - 2]) ?? '';
  reqs[r[2]].set(r[1], { id: r[1], text, phase: phase || '—', line });
}

// 2) Tham chiếu FR trong tài liệu (trừ PRD, RTM, audit, TEST_STRATEGY) và bảng TC.
const designRefs = new Map(); // FR → Set(doc)
const defined = new Map(); // mã định nghĩa ở cột đầu bảng của tài liệu sở hữu → doc
const knownRefs = new Map(); // mã được nhắc → Set(doc)
const tcRows = new Map(); // TC → { doc, refs:Set }
const EXCL = new Set([PRD, OUT, 'docs/README.md', 'docs/12-project/DOCUMENTATION_AUDIT.md', 'docs/12-project/OPEN_QUESTIONS.md']);
for (const f of walk(DOCS)) {
  if (EXCL.has(f)) continue;
  const text = read(f);
  for (const line of text.split('\n')) {
    const tcm = line.match(/^\|\s*(TC-[A-Z][A-Z0-9]*-\d{3}(?:\s*(?:…|\.\.\.?|–)\s*\d{3})?)\s*\|/);
    const ids = idsIn(line);
    // Định nghĩa: ô đầu của dòng bảng, hoặc gạch đầu dòng "- AC-XXX-001: ..." (cho phép in đậm, dạng khoảng).
    const def = line.match(/^\|\s*\**((?:FR|US|AC)-[A-Z][A-Z0-9]*-\d{3,4}(?:\s*(?:…|\.\.\.?|–)\s*\d{3,4})?)\**\s*\|/)
      ?? line.match(/^\s*[-*]\s+\**((?:FR|US|AC)-[A-Z][A-Z0-9]*-\d{3,4}(?:\s*(?:…|\.\.\.?|–)\s*\d{3,4})?)\**\s*:/);
    if (def && !f.endsWith('TEST_STRATEGY.md')) for (const d of idsIn(def[1])) if (!defined.has(d)) defined.set(d, f);
    if (tcm) {
      const tcs = idsIn(tcm[1]);
      const refs = ids.filter((x) => !x.startsWith('TC-'));
      for (const t of tcs) {
        const e = tcRows.get(t) ?? { doc: f, refs: new Set() };
        refs.forEach((x) => e.refs.add(x)); tcRows.set(t, e);
      }
    }
    for (const id of ids) {
      if (id.startsWith('TC-')) continue;
      if (!knownRefs.has(id)) knownRefs.set(id, new Set());
      knownRefs.get(id).add(f);
      if (id.startsWith('FR-') && !f.endsWith('TEST_STRATEGY.md') && !tcm) {
        if (!designRefs.has(id)) designRefs.set(id, new Set());
        designRefs.get(id).add(f);
      }
    }
  }
}
const tcByPrefix = new Map();
for (const t of tcRows.keys()) { const p = t.split('-')[1]; tcByPrefix.set(p, (tcByPrefix.get(p) ?? 0) + 1); }
const tcExplicit = new Map(); // FR/AC → [TC]
for (const [t, e] of tcRows) for (const r of e.refs) { if (!tcExplicit.has(r)) tcExplicit.set(r, []); tcExplicit.get(r).push(t); }

// 3) OpenAPI operations.
const spec = yaml.load(read(SPEC));
const ops = [];
for (const [p, item] of Object.entries(spec.paths ?? {})) {
  for (const [m, op] of Object.entries(item)) {
    if (op && typeof op === 'object' && op.operationId) ops.push({ path: p, method: m.toUpperCase(), id: op.operationId, text: `${op.summary ?? ''} ${op.description ?? ''}` });
  }
}
const opIds = new Set(ops.map((o) => o.id));
const pathRe = (p) => new RegExp('^' + p.replace(/[.*+?^$()|[\]\\]/g, '\\$&').replace(/\\?\{[^}]+\\?\}/g, '[^/]+') + '$');
function explicitOps(fr, text) {
  const found = new Set();
  for (const o of ops) if (new RegExp(`\\b${fr}\\b`).test(o.text)) found.add(o.id);
  for (const m of text.matchAll(/(?:\b(GET|POST|PUT|PATCH|DELETE)\s+)?`?(?:\/api\/v1)?(\/[a-z][a-z0-9_\-{}:/.]*[a-z0-9}])/g)) {
    const p = m[2];
    for (const o of ops) {
      if (m[1] && o.method !== m[1]) continue;
      if (o.path === p || pathRe(o.path).test(p) || pathRe(p).test(o.path)) found.add(o.id);
    }
  }
  for (const m of text.matchAll(/`([a-z][A-Za-z0-9]+)`/g)) if (opIds.has(m[1])) found.add(m[1]);
  return [...found].sort(cmpId);
}
const inferredOps = (dom) => (DOMAINS[dom] ? ops.filter((o) => DOMAINS[dom].ops.test(o.path)) : []);

// 4) Lắp dữ liệu.
const link = (doc) => `[${path.basename(doc, '.md')}](${path.posix.relative('docs/12-project', doc)})`;
const docLink = (d) => link(d.startsWith('docs/') ? d : `docs/${d}`);
const cap = (arr, n, fmt) => (arr.length <= n ? arr.map(fmt).join(' · ') : arr.slice(0, n).map(fmt).join(' · ') + ` · +${arr.length - n}`);
const short = (s) => { const t = s.replace(/\[([^\]]*)\]\([^)]*\)/g, '$1').replace(/\*\*|`/g, '').replace(/\|/g, '/').replace(/\s+/g, ' ').trim(); return t.length > 110 ? t.slice(0, 107).trimEnd() + '…' : t; };
const inRelease = (r) => /^P\d/.test(r.phase);

const rows = [];
for (const r of [...reqs.FR.values()]) {
  const dom = domOf(r.id);
  const cfg = DOMAINS[dom];
  const dRefs = [...(designRefs.get(r.id) ?? [])].sort(cmpId);
  const eOps = explicitOps(r.id, r.text);
  const iOps = inferredOps(dom);
  const eTc = [...new Set(tcExplicit.get(r.id) ?? [])].sort(cmpId);
  const iTcCount = (cfg?.tc ?? []).reduce((s, p) => s + (tcByPrefix.get(p) ?? 0), 0);
  rows.push({ ...r, dom, cfg, dRefs, eOps, iOps, eTc, iTcCount });
}
const byDom = new Map();
for (const row of rows) { if (!byDom.has(row.dom)) byDom.set(row.dom, []); byDom.get(row.dom).push(row); }
const domOrder = [...byDom.keys()];

// 5) Báo cáo mồ côi.
const rel1 = rows.filter(inRelease);
const noDesign = rel1.filter((r) => r.dRefs.length === 0);
const noDesignHard = rel1.filter((r) => r.dRefs.length === 0 && !(r.cfg?.docs.length));
const noOpExplicit = rel1.filter((r) => r.eOps.length === 0);
const noOpHard = rel1.filter((r) => r.eOps.length === 0 && r.iOps.length === 0);
const noTcExplicit = rel1.filter((r) => r.eTc.length === 0);
const noTcHard = rel1.filter((r) => r.eTc.length === 0 && r.iTcCount === 0);
const frDomains = new Set(rows.map((r) => r.dom));
const acOrphans = [...reqs.AC.keys()].filter((a) => !frDomains.has(domOf(a))).sort(cmpId);
const usOrphans = [...reqs.US.keys()].filter((a) => !frDomains.has(domOf(a))).sort(cmpId);
const acNoTc = [...reqs.AC.keys()].filter((a) => !(tcExplicit.get(a)?.length)).sort(cmpId);
const allIds = new Set([...reqs.FR.keys(), ...reqs.US.keys(), ...reqs.AC.keys(), ...defined.keys()]);
const ext = [...defined.keys()].filter((id) => !reqs[id.slice(0, 2)].has(id)).sort(cmpId);
const extOf = (d, t) => ext.filter((id) => id.startsWith(t + '-') && domOf(id) === d);
const extAc = ext.filter((id) => id.startsWith('AC-'));
const extAcNoTc = extAc.filter((a) => !(tcExplicit.get(a)?.length));
const extOrphanDom = ext.filter((id) => !frDomains.has(domOf(id)));
const dangling = [...knownRefs.keys()].filter((id) => !allIds.has(id)).sort(cmpId);
const tcPrefixUnmapped = [...tcByPrefix.keys()].filter((p) => !Object.values(DOMAINS).some((d) => d.tc.includes(p))).sort(cmpId);
const groupIds = (list) => {
  if (!list.length) return 'Không có.';
  const g = new Map();
  for (const id of [...list].sort(cmpId)) { const d = id.split('-')[1]; if (!g.has(d)) g.set(d, []); g.get(d).push(id.split('-')[2]); }
  return [...g].map(([d, ns]) => `**${d}** ${ns.join(', ')}`).join(' · ');
};
const ids = (arr) => arr.map((r) => r.id);

// 6) Render.
const L = [];
L.push('# Ma trận truy vết yêu cầu (RTM)', '');
L.push('> Trạng thái: Draft · Owner: Product Manager + QA Lead · **File sinh tự động** bởi `scripts/docs/build-rtm.mjs`: không sửa tay; sửa nguồn rồi chạy `node scripts/docs/build-rtm.mjs` (CI chạy `--check`).');
L.push(`> Nguồn: [PRODUCT_REQUIREMENTS](../00-overview/PRODUCT_REQUIREMENTS.md) (FR/US/AC) · [TEST_STRATEGY](TEST_STRATEGY.md) và bảng \`TC-*\` trong tài liệu domain · \`${SPEC}\` · [AGENTS.md §6](../../AGENTS.md#6-domain--tài-liệu--module) (module sở hữu)`, '');
L.push('## 1. Cách đọc', '');
L.push('- **Tường minh**: tài liệu/operation/TC có nhắc đúng mã FR (hoặc FR nhắc đúng path/operationId). **Suy luận**: ghép theo domain của mã (tài liệu sở hữu, mẫu path, tiền tố TC) — cần rà khi review, không coi là bằng chứng.');
L.push('- FR có phase `—` (mở rộng sau, ngoài Release 1.0) vẫn liệt kê nhưng không tính vào báo cáo mồ côi.');
L.push('- AC/US gắn với FR theo domain (PRD chưa ghi FR cho từng AC). Muốn liên kết tường minh: ghi mã FR/AC trong cột kịch bản của bảng TC, hoặc trong `description` của operation OpenAPI.', '');
L.push('## 2. Tổng hợp theo domain', '');
L.push('| Domain | Module sở hữu | FR | FR có tài liệu tường minh | FR có operation tường minh | Operation suy luận | FR có TC tường minh | TC suy luận | US | AC | FR / AC mở rộng ở tài liệu sở hữu |');
L.push('|---|---|---|---|---|---|---|---|---|---|---|');
for (const d of domOrder) {
  const rs = byDom.get(d); const cfg = DOMAINS[d];
  const us = [...reqs.US.keys()].filter((x) => domOf(x) === d).length;
  const ac = [...reqs.AC.keys()].filter((x) => domOf(x) === d).length;
  const iTc = (cfg?.tc ?? []).map((p) => `${p} (${tcByPrefix.get(p) ?? 0})`).join(', ') || '—';
  L.push(`| \`${d}\` ${dname(d)} | ${cfg?.svc ?? '**?**'} | ${rs.length} | ${rs.filter((r) => r.dRefs.length).length} | ${rs.filter((r) => r.eOps.length).length} | ${inferredOps(d).length} | ${rs.filter((r) => r.eTc.length).length} | ${iTc} | ${us} | ${ac} | ${extOf(d, 'FR').length} / ${extOf(d, 'AC').length} |`);
}
const tot = (f) => rows.filter(f).length;
L.push(`| **Tổng** | | **${rows.length}** | ${tot((r) => r.dRefs.length)} | ${tot((r) => r.eOps.length)} | ${ops.length} op trong spec | ${tot((r) => r.eTc.length)} | ${tcRows.size} TC | ${reqs.US.size} | ${reqs.AC.size} | ${ext.filter((x) => x.startsWith('FR-')).length} / ${extAc.length} |`, '');

L.push('## 3. Ma trận FR', '');
for (const d of domOrder) {
  const cfg = DOMAINS[d];
  L.push(`### 3.${domOrder.indexOf(d) + 1}. \`${d}\` ${dname(d)}`, '');
  L.push(`Service: ${cfg?.svc ?? '**chưa ánh xạ**'} · Tài liệu sở hữu: ${cfg ? cfg.docs.map(docLink).join(' · ') : '—'}`, '');
  const acs = [...reqs.AC.keys()].filter((x) => domOf(x) === d).sort(cmpId);
  const uss = [...reqs.US.keys()].filter((x) => domOf(x) === d).sort(cmpId);
  if (acs.length || uss.length) L.push(`AC: ${acs.join(', ') || '—'} · US: ${uss.join(', ') || '—'}`, '');
  const ex = ext.filter((id) => domOf(id) === d);
  if (ex.length) {
    const docs = [...new Set(ex.map((id) => defined.get(id)))].sort(cmpId);
    L.push(`Mở rộng ở tài liệu sở hữu: ${extOf(d, 'FR').length} FR, ${extOf(d, 'AC').length} AC, ${extOf(d, 'US').length} US — ${docs.map(docLink).join(' · ')}`, '');
  }
  L.push('| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |', '|---|---|---|---|---|---|');
  for (const r of byDom.get(d)) {
    const docsCell = r.dRefs.length ? cap(r.dRefs, 4, docLink) : `_suy luận:_ ${cfg ? cfg.docs.map(docLink).join(' · ') : '—'}`;
    const opCell = r.eOps.length ? cap(r.eOps, 5, (x) => `\`${x}\``) : (r.iOps.length ? `_suy luận:_ ${r.iOps.length} op` : '—');
    const tcCell = r.eTc.length ? cap(r.eTc, 5, (x) => x) : (r.iTcCount ? `_suy luận:_ ${(cfg?.tc ?? []).map((p) => `TC-${p}`).join(', ')}` : '—');
    L.push(`| ${r.id} | ${short(r.text)} | ${r.phase} | ${docsCell} | ${opCell} | ${tcCell} |`);
  }
  L.push('');
}

L.push('## 4. Acceptance criteria → TC', '');
L.push('| AC | Domain FR | TC tường minh |', '|---|---|---|');
for (const a of [...reqs.AC.keys()].sort(cmpId)) {
  const t = [...new Set(tcExplicit.get(a) ?? [])].sort(cmpId);
  L.push(`| ${a} | ${frDomains.has(domOf(a)) ? `\`${domOf(a)}\`` : '**không có FR**'} | ${t.length ? cap(t, 6, (x) => x) : '—'} |`);
}
L.push('');

L.push('## 5. Báo cáo mồ côi', '');
L.push(`Tính trên ${rel1.length} FR thuộc Release 1.0 (phase P1–P6).`, '');
L.push('| Kiểm tra | Không có liên kết tường minh | Không có cả suy luận (mồ côi thật) |', '|---|---|---|');
L.push(`| FR không có tài liệu thiết kế | ${noDesign.length} | ${noDesignHard.length} |`);
L.push(`| FR không có operation OpenAPI | ${noOpExplicit.length} | ${noOpHard.length} |`);
L.push(`| FR không có TC | ${noTcExplicit.length} | ${noTcHard.length} |`);
L.push(`| AC không thuộc domain FR nào | — | ${acOrphans.length} |`);
L.push(`| US không thuộc domain FR nào | — | ${usOrphans.length} |`);
L.push(`| AC chưa có TC tường minh | ${acNoTc.length} | — |`);
L.push(`| AC mở rộng (tài liệu sở hữu) chưa có TC tường minh | ${extAcNoTc.length} / ${extAc.length} | — |`);
L.push(`| FR/AC mở rộng mang mã không phải domain FR | — | ${extOrphanDom.length} |`);
L.push(`| Mã FR/US/AC được nhắc nhưng không được định nghĩa ở đâu | — | ${dangling.length} |`);
L.push(`| Tiền tố TC không ánh xạ domain FR | — | ${tcPrefixUnmapped.length} |`, '');
L.push('### 5.1. Mồ côi thật', '');
L.push(`- **FR không có tài liệu thiết kế:** ${groupIds(ids(noDesignHard))}`);
L.push(`- **FR không có operation (kể cả suy luận):** ${groupIds(ids(noOpHard))} — có thể hợp lệ nếu yêu cầu thuần client/realtime/hạ tầng; khi đó ghi rõ trong tài liệu sở hữu.`);
L.push(`- **FR không có TC (kể cả suy luận):** ${groupIds(ids(noTcHard))}`);
L.push(`- **AC không thuộc FR nào:** ${groupIds(acOrphans)}`);
L.push(`- **US không thuộc FR nào:** ${groupIds(usOrphans)}`);
L.push(`- **FR/AC mở rộng mang mã không phải domain FR** (UX/NFR; cần gắn domain FR hoặc NFR): ${groupIds(extOrphanDom)}`);
L.push(`- **Mã được nhắc nhưng không được định nghĩa ở đâu:** ${dangling.length ? dangling.map((id) => `${id} (${[...knownRefs.get(id)].map((f) => path.basename(f, '.md')).join(', ')})`).join(' · ') : 'Không có.'}`);
L.push(`- **Tiền tố TC chưa ánh xạ domain:** ${tcPrefixUnmapped.length ? tcPrefixUnmapped.map((p) => `TC-${p} (${tcByPrefix.get(p)})`).join(', ') : 'Không có.'}`, '');
L.push('### 5.2. Chỉ có liên kết suy luận (cần bổ sung liên kết tường minh)', '');
L.push(`- **FR chưa được tài liệu thiết kế nhắc mã:** ${groupIds(ids(noDesign))}`);
L.push(`- **FR chưa có TC nhắc mã:** ${groupIds(ids(noTcExplicit))}`);
L.push(`- **AC (PRD) chưa có TC nhắc mã:** ${groupIds(acNoTc)}`);
L.push(`- **AC mở rộng chưa có TC nhắc mã:** ${groupIds(extAcNoTc)}`, '');
const out = L.join('\n');

const outAbs = path.join(ROOT, OUT);
if (CHECK) {
  const cur = fs.existsSync(outAbs) ? fs.readFileSync(outAbs, 'utf8') : '';
  if (cur !== out) { console.error(`RTM lệch: chạy node scripts/docs/build-rtm.mjs để sinh lại ${OUT}`); process.exit(1); }
  console.log(`RTM khớp (${rows.length} FR, ${reqs.US.size} US, ${reqs.AC.size} AC, ${tcRows.size} TC, ${ops.length} operation).`);
} else {
  fs.writeFileSync(outAbs, out);
  console.log(`Đã ghi ${OUT}: ${rows.length} FR (${rel1.length} Release 1.0), ${reqs.US.size} US, ${reqs.AC.size} AC, ${tcRows.size} TC, ${ops.length} operation.`);
  console.log(`Mở rộng: ${ext.length} mã (${extAc.length} AC, ${extAcNoTc.length} AC chưa TC), mã ngoài domain FR ${extOrphanDom.length}.`);
  console.log(`Mồ côi thật: design ${noDesignHard.length}, op ${noOpHard.length}, TC ${noTcHard.length}, AC ${acOrphans.length}, US ${usOrphans.length}, dangling ${dangling.length}, TC prefix ${tcPrefixUnmapped.length}.`);
  console.log(`Chỉ suy luận: design ${noDesign.length}, op ${noOpExplicit.length}, TC ${noTcExplicit.length}, AC không TC ${acNoTc.length}.`);
}
