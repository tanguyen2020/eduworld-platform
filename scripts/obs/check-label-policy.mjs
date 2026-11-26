// Label policy of metrics, rules and dashboards (F131 T018, F131-AC-06, MONITORING §2, AGENTS R14).
// Fails when a forbidden label (FORBIDDEN_LABELS: user/student/school ids, ip, email, username, message content…) is
//  - used by a Prometheus rule (by/without/on/ignoring/group_*, matchers, label_replace target, rule labels,
//    annotation templates $labels.<x>),
//  - used by a Grafana dashboard query (same PromQL extraction; LogQL label filters too),
//  - present on a scraped metric (exposition text given with --metrics=<file|url>, e.g. the /metrics of a binary),
// or when a `route` label holds a concrete path instead of a template (UUID or numeric segment).
// Usage: node scripts/obs/check-label-policy.mjs [--rules=<dir>] [--dashboards=<dir>] [--metrics=<file|url>]…
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { FORBIDDEN_LABELS, RULES_DIR, DASHBOARDS_DIR, loadRules, report } from './lib-monitoring-md.mjs';

const UUIDISH = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/i;

/** Label names referenced by a PromQL/LogQL expression. */
export function labelsInExpr(expr) {
  const names = new Set();
  const s = String(expr);
  for (const m of s.matchAll(/\b(?:by|without|on|ignoring|group_left|group_right)\s*\(([^)]*)\)/g)) {
    for (const n of m[1].split(',')) if (n.trim()) names.add(n.trim());
  }
  for (const m of s.matchAll(/\{([^{}]*)\}/g)) {
    for (const mm of m[1].matchAll(/([a-zA-Z_][a-zA-Z0-9_]*)\s*(?:=~|!~|!=|=)/g)) names.add(mm[1]);
  }
  for (const m of s.matchAll(/label_(?:replace|join)\s*\([^,]+,\s*"([a-zA-Z_][a-zA-Z0-9_]*)"/g)) names.add(m[1]);
  for (const m of s.matchAll(/\$labels\.([a-zA-Z_][a-zA-Z0-9_]*)/g)) names.add(m[1]);
  return names;
}

/** Concrete (non-template) route values used in matchers or samples. */
export function badRoutes(text) {
  const out = [];
  for (const m of String(text).matchAll(/\broute\s*(?:=|=~)\s*"([^"]*)"/g)) {
    const v = m[1];
    if (UUIDISH.test(v) || v.split('/').some((seg) => /^\d+$/.test(seg))) out.push(v);
  }
  return out;
}

/** Parses Prometheus exposition text: [{ name, labels: {k: v}, line }]. */
export function parseExposition(text) {
  const out = [];
  text.split(/\r?\n/).forEach((l, i) => {
    if (!l || l.startsWith('#')) return;
    const m = l.match(/^([a-zA-Z_:][a-zA-Z0-9_:]*)(\{(.*)\})?\s+\S+/);
    if (!m) return;
    const labels = {};
    for (const mm of (m[3] ?? '').matchAll(/([a-zA-Z_][a-zA-Z0-9_]*)="((?:[^"\\]|\\.)*)"/g)) labels[mm[1]] = mm[2];
    out.push({ name: m[1], labels, line: i + 1 });
  });
  return out;
}

function exprsOfDashboard(json) {
  const out = [];
  const walk = (o) => {
    if (Array.isArray(o)) return o.forEach(walk);
    if (o && typeof o === 'object') {
      for (const [k, v] of Object.entries(o)) {
        if ((k === 'expr' || k === 'query') && typeof v === 'string') out.push(v);
        else walk(v);
      }
    }
  };
  walk(json);
  return out;
}

export async function checkLabelPolicy({ rulesDir = RULES_DIR, dashboardsDir = DASHBOARDS_DIR, metrics = [] } = {}) {
  const v = [];
  const forbidden = new Set(FORBIDDEN_LABELS);
  for (const { file, rule } of loadRules(rulesDir)) {
    const name = rule.alert ?? rule.record;
    const used = new Set([...labelsInExpr(rule.expr), ...Object.keys(rule.labels ?? {})]);
    for (const a of Object.values(rule.annotations ?? {})) for (const n of labelsInExpr(a)) used.add(n);
    for (const n of used) if (forbidden.has(n)) v.push({ file, line: 1, rule: 'forbidden-label', message: `${name}: label ${n} is forbidden (MONITORING §2)` });
    for (const r of badRoutes(rule.expr)) v.push({ file, line: 1, rule: 'route-template', message: `${name}: route "${r}" is not a template` });
  }
  if (fs.existsSync(dashboardsDir)) {
    for (const f of fs.readdirSync(dashboardsDir).filter((x) => x.endsWith('.json'))) {
      const file = path.join(dashboardsDir, f);
      const json = JSON.parse(fs.readFileSync(file, 'utf8'));
      for (const e of exprsOfDashboard(json)) {
        for (const n of labelsInExpr(e)) if (forbidden.has(n)) v.push({ file, line: 1, rule: 'forbidden-label', message: `query uses forbidden label ${n}: ${e.slice(0, 80)}` });
        for (const r of badRoutes(e)) v.push({ file, line: 1, rule: 'route-template', message: `route "${r}" is not a template` });
      }
    }
  }
  for (const src of metrics) {
    const text = /^https?:\/\//.test(src) ? await (await fetch(src)).text() : fs.readFileSync(src, 'utf8');
    for (const s of parseExposition(text)) {
      for (const n of Object.keys(s.labels)) if (forbidden.has(n)) v.push({ file: src, line: s.line, rule: 'forbidden-label', message: `${s.name}: label ${n} is forbidden` });
      const r = s.labels.route;
      if (r && (UUIDISH.test(r) || r.split('/').some((seg) => /^\d+$/.test(seg)))) v.push({ file: src, line: s.line, rule: 'route-template', message: `${s.name}: route "${r}" is not a template` });
    }
  }
  return v;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const args = process.argv.slice(2);
  const get = (k) => args.filter((a) => a.startsWith(`--${k}=`)).map((a) => a.slice(k.length + 3));
  const v = await checkLabelPolicy({ rulesDir: get('rules')[0] ?? RULES_DIR, dashboardsDir: get('dashboards')[0] ?? DASHBOARDS_DIR, metrics: get('metrics') });
  process.exit(report('check-label-policy', v));
}
