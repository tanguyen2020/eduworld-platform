// Parser of docs/11-operations/MONITORING.md (F131 T002) — the only source of the metric catalogue (§3), dashboards
// (§4), alerts (§5) and SLOs (§6) (AGENTS R33). The checks in scripts/obs/ read the tables here instead of keeping
// their own copies.
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
export const REPO_ROOT = path.resolve(here, '..', '..');
const require = createRequire(import.meta.url);
export const yaml = require('../node_modules/js-yaml');
export const MONITORING = path.join(REPO_ROOT, 'docs', '11-operations', 'MONITORING.md');
export const RULES_DIR = path.join(REPO_ROOT, 'deploy', 'observability', 'prometheus', 'rules');
export const TESTS_DIR = path.join(REPO_ROOT, 'deploy', 'observability', 'prometheus', 'tests');
export const RUNBOOKS_DIR = path.join(REPO_ROOT, 'docs', '11-operations', 'runbooks');
export const DASHBOARDS_DIR = path.join(REPO_ROOT, 'deploy', 'observability', 'grafana', 'dashboards');
export const DATASOURCES_FILE = path.join(REPO_ROOT, 'deploy', 'observability', 'grafana', 'provisioning', 'datasources', 'datasources.yaml');

/** Labels that never go into a metric, a rule or a dashboard (MONITORING §2, AGENTS R14). */
export const FORBIDDEN_LABELS = ['user_id', 'student_id', 'school_id', 'ip', 'email', 'username', 'message', 'content', 'text', 'body', 'token'];
/** Mandatory runbook sections, in order (data-model.md, F131-AC-13). */
export const RUNBOOK_SECTIONS = ['Triệu chứng', 'Tác động', 'Chẩn đoán', 'Xử lý', 'Leo thang', 'Liên quan'];

/** Split one Markdown table row into cells; `|` inside backticks or escaped (\|) does not split. */
export function splitRow(line) {
  const cells = [];
  let cur = '';
  let tick = false;
  const body = line.trim().replace(/^\|/, '').replace(/\|$/, '');
  for (let i = 0; i < body.length; i++) {
    const c = body[i];
    if (c === '\\' && body[i + 1] === '|') { cur += '|'; i++; continue; }
    if (c === '`') tick = !tick;
    if (c === '|' && !tick) { cells.push(cur.trim()); cur = ''; continue; }
    cur += c;
  }
  cells.push(cur.trim());
  return cells;
}

/** Text of the section whose heading starts with `## <num>.` (up to the next `## `). */
export function section(md, num) {
  const lines = md.split(/\r?\n/);
  const start = lines.findIndex((l) => l.startsWith(`## ${num}.`));
  if (start < 0) return { text: '', line: 0 };
  let end = lines.findIndex((l, i) => i > start && /^## /.test(l));
  if (end < 0) end = lines.length;
  return { text: lines.slice(start, end).join('\n'), line: start + 1 };
}

/** Tables of a section: [{ header: [...], rows: [{ cells, line }] }]. */
export function tables(sectionText, firstLine = 1) {
  const out = [];
  const lines = sectionText.split('\n');
  let cur = null;
  lines.forEach((l, i) => {
    if (/^\s*\|/.test(l)) {
      const cells = splitRow(l);
      if (!cur) { cur = { header: cells, rows: [] }; out.push(cur); return; }
      if (cells.every((c) => /^:?-{2,}:?$/.test(c))) return;
      cur.rows.push({ cells, line: firstLine + i });
    } else cur = null;
  });
  return out;
}

const ticks = (cell) => [...cell.matchAll(/`([^`]+)`/g)].map((m) => m[1]);

export function readMonitoring(file = MONITORING) {
  return fs.readFileSync(file, 'utf8');
}

/** §5 alerts: [{ name, severities: Set('critical'|'warning'), condition, line }]. */
export function parseAlerts(md) {
  const sec = section(md, 5);
  const out = [];
  for (const t of tables(sec.text, sec.line)) {
    if (!/^Alert$/i.test(t.header[0])) continue;
    for (const r of t.rows) {
      const names = ticks(r.cells[0]).filter((n) => /^[A-Z][A-Za-z0-9]+$/.test(n));
      const sevText = (r.cells[2] ?? '').toLowerCase();
      const severities = new Set();
      if (sevText.includes('critical')) severities.add('critical');
      if (sevText.includes('warning')) severities.add('warning');
      for (const name of names) out.push({ name, severities, condition: r.cells[1] ?? '', action: r.cells[3] ?? '', line: r.line });
    }
  }
  return out;
}

const TYPE_RE = /counter|gauge|histogram|summary/gi;

/** §3 metrics. main = mandatory table (Master Prompt §18); extra = "Metric bổ sung". */
export function parseMetrics(md) {
  const sec = section(md, 3);
  const ts = tables(sec.text, sec.line);
  const main = [];
  const extra = [];
  for (const t of ts) {
    if (/^Nhóm$/i.test(t.header[0])) {
      for (const r of t.rows) {
        const [, metricCell, typeCell, labelCell, sourceCell] = r.cells;
        const names = ticks(metricCell).filter((n) => /^[a-z_][a-z0-9_]*\*?$/.test(n));
        const types = (typeCell.match(TYPE_RE) ?? []).map((x) => x.toLowerCase());
        const labels = ticks(labelCell).filter((n) => /^[a-z_][a-z0-9_]*$/.test(n));
        names.forEach((name, i) => main.push({
          name, type: types.length === names.length ? types[i] : (types[0] ?? null), labels, source: sourceCell ?? '', line: r.line,
        }));
      }
    } else if (/^Metric$/i.test(t.header[0])) {
      for (const r of t.rows) for (const n of ticks(r.cells[0])) if (/^[a-z_][a-z0-9_]*\*?$/.test(n)) extra.push({ name: n, line: r.line });
    }
  }
  return { main, extra };
}

/** Where a mandatory metric comes from: an exporter job or a backend binary. */
export function metricOrigin(m) {
  const src = m.source;
  if (/APISIX/.test(src) || m.name.startsWith('apisix_')) return 'apisix';
  if (/postgres_exporter/.test(src) || m.name.startsWith('pg_')) return 'postgres';
  if (m.name.startsWith('nats_')) return 'nats';
  return 'backend';
}

/** §4 dashboards: [{ name, slug, line }]; slug = uid of the JSON dashboard. */
export function parseDashboards(md) {
  const sec = section(md, 4);
  const out = [];
  for (const t of tables(sec.text, sec.line)) {
    if (!/^Dashboard$/i.test(t.header[0])) continue;
    for (const r of t.rows) {
      const m = r.cells[0].match(/\*\*(.+?)\*\*/);
      if (!m) continue;
      const name = m[1].replace(/`/g, '');
      out.push({ name, slug: dashboardSlug(name), line: r.line });
    }
  }
  return out;
}

export function dashboardSlug(name) {
  return name.replace(/<[^>]*>/g, '').toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '')
    .replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');
}

/** True when `name` is in the catalogue (exact or wildcard prefix `pg_stat_database_*`), histogram suffixes stripped. */
export function inCatalog(name, names) {
  const base = name.replace(/_(bucket|sum|count)$/, '');
  return names.some((n) => (n.endsWith('*') ? name.startsWith(n.slice(0, -1)) : n === name || n === base));
}

/** Loads every rule file: [{ file, group, rule }]. */
export function loadRules(dir = RULES_DIR) {
  const out = [];
  for (const f of fs.readdirSync(dir).filter((x) => /\.ya?ml$/.test(x)).sort()) {
    const doc = yaml.load(fs.readFileSync(path.join(dir, f), 'utf8')) ?? {};
    for (const g of doc.groups ?? []) for (const r of g.rules ?? []) out.push({ file: path.join(dir, f), group: g.name, rule: r });
  }
  return out;
}

/** Violation formatter shared by the checks. */
export function report(name, violations) {
  for (const v of violations) console.error(`${path.relative(REPO_ROOT, v.file ?? '') || '-'}:${v.line ?? 1}: [${v.rule}] ${v.message}`);
  console.log(`${name}: ${violations.length} violation(s)`);
  return violations.length ? 1 : 0;
}
