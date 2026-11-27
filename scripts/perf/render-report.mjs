// Renders the perf report of one load run (F132-AC-06, T002/T009, data-model §1): configuration, environment and
// sizing, backend tag + digest of every image, k6 thresholds, loadbot latencies, Prometheus checks, load-generator
// saturation, verdict and bottlenecks → docs/12-project/perf-reports/<YYYY-MM-DD>-<run>.md.
//
// Verdict rules (ADR-0045 §7: never claim a capacity that was not measured):
//   - invalid  when a load generator ran above generator_cpu_max (the limit measured would be the generator's) or a
//              required input is missing;
//   - failed   when any k6 threshold or Prometheus check failed, or a check has no data;
//   - passed   otherwise. Only a passed TC-PERF-014 run on staging may print "đạt `target_ccu`"; any other run never
//              prints that phrase (F132-AC-06).
//
// Usage: node scripts/perf/render-report.mjs --dir=<run dir> --out=<file.md>
//   <run dir>/meta.json               {run_id, run, tc, env, started, ended, commit, backend_tag, images:{svc:digest},
//                                      sizing:{…}, warmup:{peak_ok, checked_at}, grafana_url?, scale?}
//   <run dir>/k6-summary.json         k6 --summary-export (optional when the run is loadbot only)
//   <run dir>/loadbot-summary.json    loadbot -summary (optional)
//   <run dir>/prom.json               scripts/perf/prom-checks.mjs output (optional when the scenario has no prom check)
//   <run dir>/generator.json          scripts/perf/check-generator-saturation.sh output
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const GEN = path.join(ROOT, 'tests/load/k6/scenarios.gen.json');

const readJSON = (f) => (fs.existsSync(f) ? JSON.parse(fs.readFileSync(f, 'utf8')) : null);

/** k6 --summary-export threshold value: legacy boolean (true = crossed = failed) or {ok}. */
function thresholdOk(v) {
  if (typeof v === 'boolean') return !v;
  if (v && typeof v === 'object' && 'ok' in v) return Boolean(v.ok);
  return false;
}

/** Flattens the k6 thresholds into rows {metric, expr, ok}. */
export function k6Rows(summary) {
  const rows = [];
  for (const [metric, m] of Object.entries((summary && summary.metrics) || {})) {
    for (const [expr, v] of Object.entries(m.thresholds || {})) rows.push({ metric, expr, ok: thresholdOk(v) });
  }
  return rows.sort((a, b) => a.metric.localeCompare(b.metric));
}

/** The verdict of a run: { status: passed|failed|invalid, reasons[] }. */
export function verdict({ meta, k6, prom, generator, cpuMax }) {
  const reasons = [];
  if (!meta) return { status: 'invalid', reasons: ['meta.json is missing'] };
  if (!generator || typeof generator.max_cpu !== 'number') reasons.push('no load-generator CPU measurement (saturation unknown)');
  else if (generator.max_cpu > cpuMax) reasons.push(`load generator CPU ${(generator.max_cpu * 100).toFixed(0)}% > ${(cpuMax * 100).toFixed(0)}% (perf_generator_cpu_max): the limit measured is the generator's`);
  if (!k6 && !prom) reasons.push('no measurement (neither k6 summary nor Prometheus checks)');
  if (reasons.length) return { status: 'invalid', reasons };
  for (const r of k6Rows(k6)) if (!r.ok) reasons.push(`k6 threshold ${r.metric} ${r.expr} failed`);
  for (const c of prom || []) {
    if (c.observed === null || c.observed === undefined || Number.isNaN(c.observed)) reasons.push(`Prometheus check ${c.name}: no data`);
    else if (!c.ok) reasons.push(`Prometheus check ${c.name}: ${c.observed} ${c.op} ${c.value} is false (${c.ref})`);
  }
  if (meta.tc === 'TC-PERF-002' || meta.tc === 'TC-PERF-014') {
    if (!meta.warmup || meta.warmup.peak_ok !== true) reasons.push('ops-scheduler was not at `peak` before the run (TC-PERF-017)');
  }
  return { status: reasons.length ? 'failed' : 'passed', reasons };
}

const LABEL = { passed: 'đạt', failed: 'không đạt', invalid: 'không hợp lệ' };

function fmt(n) {
  if (n === null || n === undefined) return '—';
  return typeof n === 'number' ? (Math.abs(n) >= 100 ? n.toFixed(0) : n.toPrecision(3)) : String(n);
}

/** Renders the Markdown report. */
export function render({ meta, k6, loadbot, prom, generator, cpuMax, targetCcu }) {
  const v = verdict({ meta, k6, prom, generator, cpuMax });
  const m = meta || {};
  const L = [];
  L.push(`# Perf report ${m.run_id || '?'} — ${m.tc || '?'} \`${m.run || '?'}\``, '');
  L.push(`> Sinh bởi \`scripts/perf/render-report.mjs\` (F132-AC-06). Môi trường: **${m.env || '?'}**. Kết luận: **${LABEL[v.status]}**.`, '');
  if (m.env !== 'staging') {
    L.push(`**Không phải bằng chứng năng lực:** lần chạy trên \`${m.env}\` chỉ kiểm tra kịch bản (smoke); năng lực chỉ được công bố từ staging tương đương production (ADR-0045 §7).`, '');
  }
  L.push('## 1. Cấu hình', '');
  L.push('| Mục | Giá trị |', '|---|---|');
  L.push(`| run_id | \`${m.run_id || '?'}\` |`);
  L.push(`| Kịch bản | ${m.tc || '?'} \`${m.run || '?'}\` |`);
  L.push(`| Thời gian | ${m.started || '?'} → ${m.ended || '?'} |`);
  L.push(`| Backend | tag \`${m.backend_tag || '?'}\`, commit \`${m.commit || '?'}\` |`);
  L.push(`| Warm-up (\`ops-scheduler\` ở \`peak\`) | ${m.warmup ? (m.warmup.peak_ok ? `có (${m.warmup.checked_at || ''})` : 'không') : 'không áp dụng'} |`);
  if (m.scale) L.push(`| Scale trong lúc tải | \`${m.scale}\` |`);
  if (m.grafana_url) L.push(`| Biểu đồ | [Grafana "Load test" theo run_id](${m.grafana_url}) |`);
  L.push('');
  L.push('### Image', '', '| Swarm service | Digest |', '|---|---|');
  const images = Object.entries(m.images || {});
  if (images.length) for (const [svc, d] of images) L.push(`| ${svc} | \`${d}\` |`);
  else L.push('| — | (không ghi nhận) |');
  L.push('', '### Sizing', '', '| Vai trò node | Cấu hình |', '|---|---|');
  const sizing = Object.entries(m.sizing || {});
  if (sizing.length) for (const [k, s] of sizing) L.push(`| ${k} | ${typeof s === 'object' ? JSON.stringify(s) : s} |`);
  else L.push('| — | (không ghi nhận) |');
  L.push('', '## 2. Ngưỡng k6', '');
  const rows = k6Rows(k6);
  if (rows.length) {
    L.push('| Metric | Ngưỡng | Giá trị (p95 hoặc tỷ lệ/đếm) | Kết quả |', '|---|---|---|---|');
    for (const r of rows) {
      const mm = k6.metrics[r.metric] || {};
      const p95 = mm['p(95)'] ?? (mm.values && mm.values['p(95)']) ?? mm.value ?? mm.rate ?? mm.count;
      L.push(`| \`${r.metric}\` | \`${r.expr}\` | ${fmt(p95)} | ${r.ok ? 'đạt' : '**không đạt**'} |`);
    }
  } else L.push('(không có k6 trong lần chạy này)');
  L.push('', '## 3. Bot (loadbot)', '');
  const lat = Object.entries((loadbot && loadbot.latencies) || {});
  if (lat.length) {
    L.push('| Thao tác | Số mẫu | p50 ms | p95 ms | p99 ms |', '|---|---|---|---|---|');
    for (const [op, q] of lat.sort()) L.push(`| ${op} | ${q.count} | ${fmt(q.p50_ms)} | ${fmt(q.p95_ms)} | ${fmt(q.p99_ms)} |`);
    const c = loadbot.counts || {};
    L.push('', `Đăng nhập: ${c.login_ok || 0} vào được, ${c.login_failed || 0} thất bại, ${c.login_queue_busy || 0} lượt \`LOGIN_QUEUE_BUSY\`, ${c.login_server_error || 0} lỗi 5xx.`);
  } else L.push('(không có loadbot trong lần chạy này)');
  L.push('', '## 4. Kiểm tra Prometheus', '');
  if (prom && prom.length) {
    L.push('| Kiểm tra | Quan sát | Ngưỡng | Nguồn | Kết quả |', '|---|---|---|---|---|');
    for (const c of prom) L.push(`| ${c.name} | ${fmt(c.observed)} | ${c.op} ${c.value} | ${c.ref} | ${c.observed === null || c.observed === undefined ? '**thiếu số liệu**' : c.ok ? 'đạt' : '**không đạt**'} |`);
  } else L.push('(không có)');
  L.push('', '## 5. Máy tải', '');
  if (generator && typeof generator.max_cpu === 'number') {
    L.push(`CPU cao nhất của máy tải: ${(generator.max_cpu * 100).toFixed(0)}% (ngưỡng \`perf_generator_cpu_max\` ${(cpuMax * 100).toFixed(0)}%).`);
  } else L.push('Không đo được CPU máy tải → lần chạy không hợp lệ.');
  L.push('', '## 6. Kết luận', '');
  L.push(`**${LABEL[v.status]}**${v.reasons.length ? ':' : '.'}`);
  for (const r of v.reasons) L.push(`- ${r}`);
  if (v.status === 'passed' && m.tc === 'TC-PERF-014' && m.env === 'staging') {
    L.push('', `Lần chạy này đạt \`target_ccu\` (${targetCcu} CCU) trên staging; cập nhật bảng sizing DEPLOYMENT §4.2 theo số đo.`);
  } else if (m.tc === 'TC-PERF-014') {
    L.push('', 'Năng lực `target_ccu` **chưa được chứng minh** bởi lần chạy này; không tài liệu nào được ghi là đã đạt.');
  }
  L.push('', '## 7. Điểm nghẽn', '');
  const worst = (prom || []).filter((c) => c.observed !== null && c.observed !== undefined && !c.ok);
  if (worst.length) for (const c of worst) L.push(`- ${c.name}: ${fmt(c.observed)} (ngưỡng ${c.op} ${c.value}) — ${c.ref}`);
  else L.push('- (Tech Lead ghi phân tích: tài nguyên bão hòa đầu tiên, pool DB, lag consumer, …)');
  L.push('');
  return { text: L.join('\n'), verdict: v };
}

function arg(name) {
  const a = process.argv.find((x) => x.startsWith(`--${name}=`));
  return a ? a.slice(name.length + 3) : undefined;
}

if (process.argv[1] && fileURLToPath(import.meta.url) === path.resolve(process.argv[1])) {
  const dir = arg('dir');
  const out = arg('out');
  if (!dir || !out) {
    console.error('usage: render-report.mjs --dir=<run dir> --out=<report.md>');
    process.exit(2);
  }
  const gen = readJSON(GEN);
  const { text, verdict: v } = render({
    meta: readJSON(path.join(dir, 'meta.json')),
    k6: readJSON(path.join(dir, 'k6-summary.json')),
    loadbot: readJSON(path.join(dir, 'loadbot-summary.json')),
    prom: readJSON(path.join(dir, 'prom.json')),
    generator: readJSON(path.join(dir, 'generator.json')),
    cpuMax: gen.parameters.generator_cpu_max,
    targetCcu: gen.parameters.target_ccu,
  });
  fs.mkdirSync(path.dirname(out), { recursive: true });
  fs.writeFileSync(out, text);
  console.log(`render-report: ${v.status} → ${out}`);
  process.exit(v.status === 'passed' ? 0 : v.status === 'failed' ? 1 : 3);
}
