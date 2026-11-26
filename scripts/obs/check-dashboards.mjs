// Grafana dashboards vs MONITORING §3/§4 (F131 T032, F131-AC-14, AC-16). Fails when
//  - a dashboard of the §4 table has no JSON (uid = slug of the §4 name), or a JSON has no §4 row;
//  - a panel/target references a datasource UID that the provisioning file does not define;
//  - a Prometheus query uses a metric that is neither in MONITORING §3 (both tables), nor a recording rule of
//    prometheus/rules (eduworld:*), nor an exporter metric of MONITORING §1 (EXPORTER_PREFIXES);
//  - a dashboard tagged per-module / per-binary lacks the template variables `service` and `module`;
//  - Chat Safety (uid chat-safety) has a log/table panel, a non-Prometheus datasource, or a query that is not an
//    aggregate count/ratio (every expression must start with an aggregation) or groups by an identifying label.
// Usage: node scripts/obs/check-dashboards.mjs [--dashboards=<dir>] [--monitoring=<md>]
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import {
  MONITORING, DASHBOARDS_DIR, DATASOURCES_FILE, RULES_DIR, FORBIDDEN_LABELS, yaml, readMonitoring, parseMetrics,
  parseDashboards, inCatalog, loadRules, report,
} from './lib-monitoring-md.mjs';
import { labelsInExpr } from './check-label-policy.mjs';

/** Metric families of the exporters listed in MONITORING §1 (not catalogued one by one in §3). */
export const EXPORTER_PREFIXES = ['up', 'ALERTS', 'node_', 'container_', 'pg_', 'redis_', 'nats_', 'apisix_', 'keycloak_', 'jvm_',
  'probe_', 'swarm_', 'go_', 'process_', 'prometheus_', 'alertmanager_', 'traces_service_graph_'];

const FUNCS = new Set(['sum', 'min', 'max', 'avg', 'count', 'stddev', 'stdvar', 'topk', 'bottomk', 'quantile', 'count_values', 'group',
  'rate', 'irate', 'increase', 'delta', 'idelta', 'deriv', 'histogram_quantile', 'abs', 'ceil', 'floor', 'round', 'clamp_min', 'clamp_max',
  'clamp', 'label_replace', 'label_join', 'vector', 'scalar', 'time', 'timestamp', 'absent', 'absent_over_time', 'avg_over_time',
  'min_over_time', 'max_over_time', 'sum_over_time', 'count_over_time', 'last_over_time', 'predict_linear', 'resets', 'changes', 'sort',
  'sort_desc', 'hour', 'minute', 'day_of_week', 'by', 'without', 'on', 'ignoring', 'group_left', 'group_right', 'and', 'or', 'unless',
  'bool', 'offset', 'inf', 'nan', 'e']);
const AGG = /^\s*\(?\s*(sum|count|max|min|avg|histogram_quantile|topk)\b/;

/** Metric names used by a PromQL expression (strings, matchers, grouping lists, ranges removed). */
export function metricsInExpr(expr) {
  const s = String(expr)
    .replace(/"(?:[^"\\]|\\.)*"/g, '""')
    .replace(/\{[^{}]*\}/g, ' ')
    .replace(/\b(by|without|on|ignoring|group_left|group_right)\s*\([^)]*\)/g, ' ')
    .replace(/\[[^\]]*\]/g, ' ')
    .replace(/\$[a-zA-Z_]+/g, ' ');
  const out = new Set();
  for (const m of s.matchAll(/[a-zA-Z_:][a-zA-Z0-9_:]*/g)) {
    const t = m[0];
    if (FUNCS.has(t) || /^\d/.test(t)) continue;
    out.add(t);
  }
  return out;
}

function datasourceUids(file = DATASOURCES_FILE) {
  const doc = yaml.load(fs.readFileSync(file, 'utf8')) ?? {};
  return new Set((doc.datasources ?? []).map((d) => d.uid));
}

export function checkDashboards({ dashboardsDir = DASHBOARDS_DIR, monitoring = MONITORING, datasources = DATASOURCES_FILE, rulesDir = RULES_DIR } = {}) {
  const v = [];
  const md = readMonitoring(monitoring);
  const { main, extra } = parseMetrics(md);
  const catalog = [...main, ...extra].map((m) => m.name);
  const records = new Set(loadRules(rulesDir).filter((r) => r.rule.record).map((r) => r.rule.record));
  const uids = datasourceUids(datasources);
  const wanted = parseDashboards(md);
  const files = fs.existsSync(dashboardsDir) ? fs.readdirSync(dashboardsDir).filter((f) => f.endsWith('.json')) : [];
  const seen = new Set();
  for (const f of files) {
    const file = path.join(dashboardsDir, f);
    let d;
    try { d = JSON.parse(fs.readFileSync(file, 'utf8')); } catch (e) { v.push({ file, line: 1, rule: 'json', message: `invalid JSON: ${e.message}` }); continue; }
    const add = (rule, message) => v.push({ file, line: 1, rule, message: `${d.uid ?? f}: ${message}` });
    seen.add(d.uid);
    if (!wanted.some((w) => w.slug === d.uid)) add('not-in-table', 'dashboard uid matches no row of MONITORING §4');
    const vars = new Set((d.templating?.list ?? []).map((x) => x.name));
    if ((d.tags ?? []).some((t) => t === 'per-module' || t === 'per-binary')) {
      for (const n of ['service', 'module']) if (!vars.has(n)) add('variables', `per binary/module dashboard needs the variable ${n}`);
    }
    const safety = d.uid === 'chat-safety';
    for (const p of d.panels ?? []) {
      const dsList = [p.datasource, ...(p.targets ?? []).map((t) => t.datasource)].filter(Boolean);
      for (const ds of dsList) if (!uids.has(ds.uid)) add('datasource', `panel "${p.title}" uses unknown datasource uid ${ds.uid}`);
      if (safety && (['logs', 'table'].includes(p.type) || dsList.some((ds) => ds.type !== 'prometheus'))) add('chat-safety', `panel "${p.title}" must be a Prometheus count/ratio panel`);
      for (const t of p.targets ?? []) {
        if (!t.expr || (t.datasource ?? p.datasource)?.type !== 'prometheus') continue;
        for (const m of metricsInExpr(t.expr)) {
          const ok = inCatalog(m, catalog) || records.has(m) || EXPORTER_PREFIXES.some((pre) => (pre.endsWith('_') ? m.startsWith(pre) : m === pre));
          if (!ok) add('metric', `panel "${p.title}" queries ${m}, which is not in MONITORING §3 nor a recording rule`);
        }
        for (const l of labelsInExpr(t.expr)) if (FORBIDDEN_LABELS.includes(l)) add('forbidden-label', `panel "${p.title}" uses label ${l}`);
        if (safety) {
          if (!AGG.test(t.expr)) add('chat-safety', `panel "${p.title}": query must be an aggregate count/ratio`);
          for (const l of labelsInExpr(t.expr)) if (['tenant', 'channel_id', 'case_id', 'user_id', 'student_id'].includes(l)) add('chat-safety', `panel "${p.title}" groups/filters by identifying label ${l}`);
        }
      }
    }
  }
  for (const w of wanted) if (!seen.has(w.slug)) v.push({ file: monitoring, line: w.line, rule: 'dashboard-missing', message: `dashboard "${w.name}" of MONITORING §4 has no JSON (uid ${w.slug})` });
  return { violations: v, dashboards: files.length, wanted: wanted.length };
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const opt = Object.fromEntries(process.argv.slice(2).map((a) => a.replace(/^--/, '').split('=')));
  const r = checkDashboards({ dashboardsDir: opt.dashboards ?? DASHBOARDS_DIR, monitoring: opt.monitoring ?? MONITORING });
  console.log(`check-dashboards: ${r.dashboards} JSON dashboards, ${r.wanted} in MONITORING §4`);
  process.exit(report('check-dashboards', r.violations));
}
