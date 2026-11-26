// Grafana dashboards as code (F131 T031, F131-AC-14, AC-16): builds deploy/observability/grafana/dashboards/*.json —
// one dashboard per row of MONITORING §4, uid = slug of the §4 name. Queries use only the metrics of
// MONITORING §3 (and exporter metrics of §1, recording rules eduworld:*). Edit here, then
//   node scripts/obs/build-dashboards.mjs          # write
//   node scripts/obs/build-dashboards.mjs --check  # CI: generated files are fresh (job obs-dashboards)
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { DASHBOARDS_DIR } from './lib-monitoring-md.mjs';

const PROM = { type: 'prometheus', uid: 'prometheus' };
const TEMPO = { type: 'tempo', uid: 'tempo' };

const q = (expr, legend = '') => ({ expr, legendFormat: legend });
const ts = (title, targets, unit = 'short', w = 12) => ({ kind: 'timeseries', title, targets: [].concat(targets), unit, w });
const stat = (title, targets, unit = 'short', w = 6) => ({ kind: 'stat', title, targets: [].concat(targets), unit, w });
const p95 = (metric, by = '', sel = '', win = '5m') => `histogram_quantile(0.95, sum by (le${by ? `, ${by}` : ''}) (rate(${metric}_bucket${sel}[${win}])))`;

const svcVar = { name: 'service', query: 'label_values(eduworld_http_server_requests_total, service)', multi: true };
const modVar = { name: 'module', query: 'label_values(eduworld_http_server_requests_total, module)', multi: false };
const profVar = { name: 'profile', query: 'label_values(swarm_service_replicas_running{service="worker"}, profile)', multi: true };

export const DASHBOARDS = [
  { slug: 'eduworld-overview', title: 'EduWorld Overview', tags: ['overview'], panels: [
    stat('CCU', q('sum(eduworld_players_online)')),
    stat('Phiên game hoạt động', q('sum(eduworld_game_sessions_active)')),
    stat('Availability 5xx (5m)', q('eduworld:sli_availability_errors:ratio_rate5m'), 'percentunit'),
    stat('Burn rate 1h', q('eduworld:sli_availability_errors:ratio_rate1h / (1 - eduworld:slo_availability_target)')),
    ts('RPS gateway', q('sum by (code) (rate(apisix_http_status{job="apisix"}[5m]))', '{{code}}'), 'reqps'),
    ts('p95 latency core-api', q(p95('eduworld_http_server_duration_seconds', 'service'), '{{service}}'), 's'),
    ts('5xx theo module', q('sum by (module) (rate(eduworld_http_server_requests_total{status_class="5xx"}[5m]))', '{{module}}'), 'reqps'),
    ts('Replica Swarm service', [q('sum by (swarm_service) (swarm_service_replicas_running{service=~"core-api|realtime|worker"})', '{{swarm_service}} running'), q('sum by (swarm_service) (swarm_service_replicas_desired{service=~"core-api|realtime|worker"})', '{{swarm_service}} desired')]),
    ts('Alert đang bắn', q('count by (severity) (ALERTS{alertstate="firing"})', '{{severity}}')),
  ] },
  { slug: 'module-map', title: 'Module Map', tags: ['tracing'], panels: [
    { kind: 'nodeGraph', title: 'Service graph (Tempo)', datasource: TEMPO, targets: [{ queryType: 'serviceMap', refId: 'A' }], w: 24, h: 12 },
    ts('Facade RPS caller → callee', q('sum by (caller_module, callee_module) (rate(eduworld_internal_client_requests_total[5m]))', '{{caller_module}} → {{callee_module}}'), 'reqps'),
    ts('Facade lỗi caller → callee', q('sum by (caller_module, callee_module) (rate(eduworld_internal_client_requests_total{outcome!="ok"}[5m]))', '{{caller_module}} → {{callee_module}}'), 'reqps'),
    ts('Facade p95 caller → callee', q(p95('eduworld_internal_client_duration_seconds', 'caller_module, callee_module'), '{{caller_module}} → {{callee_module}}'), 's'),
    ts('Circuit breaker mở (tích hợp ngoài)', q('max by (module, integration) (eduworld_external_client_circuit_state) == 2', '{{module}} → {{integration}}')),
  ] },
  { slug: 'module', title: 'Module', tags: ['per-module'], vars: [svcVar, modVar], panels: [
    ts('RPS theo route', q('sum by (route) (rate(eduworld_http_server_requests_total{service=~"$service",module="$module"}[5m]))', '{{route}}'), 'reqps'),
    ts('5xx theo route', q('sum by (route) (rate(eduworld_http_server_requests_total{service=~"$service",module="$module",status_class="5xx"}[5m]))', '{{route}}'), 'reqps'),
    ts('p95 theo route', q(p95('eduworld_http_server_duration_seconds', 'route', '{service=~"$service",module="$module"}'), '{{route}}'), 's'),
    ts('p95 command/query handler', q(p95('eduworld_usecase_duration_seconds', 'kind, usecase', '{module="$module"}'), '{{kind}} {{usecase}}'), 's'),
    ts('DB pool acquire p95', q(p95('eduworld_db_pool_acquire_duration_seconds', 'service, database', '{module="$module"}'), '{{service}} {{database}}'), 's'),
    ts('Query chậm (p95)', q(`topk(10, ${p95('eduworld_db_query_duration_seconds', 'query', '{module="$module"}')})`, '{{query}}'), 's'),
    ts('Outbox lag', q('max(eduworld_outbox_lag_seconds{module="$module"})', 'outbox'), 's'),
    ts('Consumer theo kết quả', q('sum by (result) (rate(eduworld_events_consumed_total{module="$module"}[5m]))', '{{result}}'), 'ops'),
    ts('Projection lag', q('max by (projection) (eduworld_projection_lag_seconds{module="$module"})', '{{projection}}'), 's'),
    ts('Lỗi tích hợp ngoài', q('sum by (integration) (rate(eduworld_external_client_requests_total{module="$module",outcome!="ok"}[5m]))', '{{integration}}'), 'ops'),
    ts('Panic đã recover', q('sum by (service, entrypoint) (increase(eduworld_panics_recovered_total{module="$module"}[5m]))', '{{service}} {{entrypoint}}')),
  ] },
  { slug: 'binary', title: 'Binary', tags: ['per-binary'], vars: [svcVar, profVar, modVar], panels: [
    ts('CPU (core)', q('sum by (name) (rate(container_cpu_usage_seconds_total{service=~"$service",profile=~"$profile|"}[5m]))', '{{name}}')),
    ts('RAM working set', q('sum by (name) (container_memory_working_set_bytes{service=~"$service",profile=~"$profile|"})', '{{name}}'), 'bytes'),
    ts('Replica', [q('sum by (swarm_service) (swarm_service_replicas_running{service=~"$service"})', '{{swarm_service}} running'), q('sum by (swarm_service) (swarm_service_replicas_desired{service=~"$service"})', '{{swarm_service}} desired')]),
    ts('Restart task (15m)', q('sum by (swarm_service) (increase(swarm_task_restarts_total{service=~"$service"}[15m]))', '{{swarm_service}}')),
    ts('GC pause p95', q('max by (instance) (go_gc_duration_seconds{service=~"$service",quantile="0.75"})', '{{instance}}'), 's'),
    ts('Goroutine', q('sum by (instance) (go_goroutines{service=~"$service"})', '{{instance}}')),
    ts('Pool DB tổng', q('sum by (module, state) (eduworld_db_pool_connections{service=~"$service",module=~"$module"})', '{{module}} {{state}}')),
    ts('Panic đã recover', q('sum by (module, entrypoint) (increase(eduworld_panics_recovered_total{service=~"$service"}[5m]))', '{{module}} {{entrypoint}}')),
  ] },
  { slug: 'api', title: 'API', tags: ['api'], vars: [{ ...modVar, multi: true }], panels: [
    ts('RPS theo route', q('sum by (route) (rate(eduworld_http_server_requests_total{module=~"$module"}[5m]))', '{{route}}'), 'reqps'),
    ts('Top route chậm (p95)', q(`topk(10, ${p95('eduworld_http_server_duration_seconds', 'route', '{module=~"$module"}')})`, '{{route}}'), 's'),
    ts('Lỗi theo code', q('sum by (code) (rate(eduworld_http_server_requests_total{module=~"$module",status_class=~"4xx|5xx"}[5m]))', '{{code}}'), 'reqps'),
    ts('Rate-limit bị từ chối', q('sum by (policy) (rate(eduworld_ratelimit_rejected_total[5m]))', '{{policy}}'), 'reqps'),
    ts('Gateway theo status', q('sum by (code) (rate(apisix_http_status[5m]))', '{{code}}'), 'reqps'),
    ts('Gateway p95 (ms)', q(`histogram_quantile(0.95, sum by (le) (rate(apisix_http_latency_bucket{type="request"}[5m])))`, 'p95'), 'ms'),
  ] },
  { slug: 'realtime', title: 'Realtime', tags: ['realtime'], panels: [
    ts('Kết nối WS theo node', q('sum by (node) (eduworld_ws_connections)', '{{node}}')),
    ts('Kết nối WS theo kết quả', q('sum by (result) (rate(eduworld_ws_connect_total[5m]))', '{{result}}'), 'ops'),
    ts('Lệnh NATS p95', q(p95('eduworld_nats_request_duration_seconds', 'subject_group'), '{{subject_group}}'), 's'),
    ts('Lệnh NATS lỗi', q('sum by (module, reason) (rate(eduworld_nats_request_errors_total[5m]))', '{{module}} {{reason}}'), 'ops'),
    ts('Hub tick p95', q(p95('eduworld_hub_tick_duration_seconds'), 'p95'), 's'),
    ts('Hub instance', q('sum(eduworld_hub_instances)', 'hub')),
    ts('Replica realtime', q('sum(swarm_service_replicas_running{service="realtime"})', 'running')),
  ] },
  { slug: 'matchmaking-competition', title: 'Matchmaking & Competition', tags: ['competition'], panels: [
    ts('Hàng đợi', q('sum by (queue) (eduworld_mm_queue_size)', '{{queue}}')),
    ts('Thời gian chờ p50/p95', [q('histogram_quantile(0.5, sum by (le, mode) (rate(eduworld_mm_wait_seconds_bucket{result="matched"}[5m])))', 'p50 {{mode}}'), q(p95('eduworld_mm_wait_seconds', 'mode', '{result="matched"}'), 'p95 {{mode}}')], 's'),
    ts('Tỷ lệ chấp nhận proposal', q('sum by (mode) (rate(eduworld_mm_proposals_total{result="accepted"}[15m])) / sum by (mode) (rate(eduworld_mm_proposals_total[15m]))', '{{mode}}'), 'percentunit'),
    ts('Thời lượng trận p95', q(p95('eduworld_match_duration_seconds', 'mode'), '{{mode}}'), 's'),
    ts('Kết quả trùng bị từ chối', q('sum by (mode) (increase(eduworld_match_results_total{result="duplicate_rejected"}[5m]))', '{{mode}}')),
  ] },
  { slug: 'learning', title: 'Learning', tags: ['learning'], panels: [
    ts('Hoạt động hoàn thành', q('sum by (activity_type) (rate(eduworld_learning_activities_completed_total[5m]))', '{{activity_type}}'), 'ops'),
    ts('Attempt nộp theo kết quả', q('sum by (result) (rate(eduworld_attempts_submitted_total[5m]))', '{{result}}'), 'ops'),
    ts('Bài thi nộp theo kết quả', q('sum by (exam_type, result) (rate(eduworld_exam_submissions_total[5m]))', '{{exam_type}} {{result}}'), 'ops'),
    ts('Thời gian nộp bài thi p95', q(p95('eduworld_exam_submit_duration_seconds', 'exam_type'), '{{exam_type}}'), 's'),
    ts('SLI submit < 400 ms', q('eduworld:sli_latency_submit:ratio_rate5m', 'SLI'), 'percentunit'),
  ] },
  { slug: 'dashboards', title: 'Dashboards', tags: ['dashboards'], panels: [
    ts('p95 truy vấn theo query_id', q(p95('eduworld_dashboard_query_duration_seconds', 'query_id'), '{{query_id}}'), 's'),
    ts('Cache hit', q('sum(rate(eduworld_dashboard_cache_requests_total{result="hit"}[5m])) / sum(rate(eduworld_dashboard_cache_requests_total[5m]))', 'hit'), 'percentunit'),
    ts('Truy vấn bị từ chối', q('sum by (reason) (rate(eduworld_dashboard_rejected_total[5m]))', '{{reason}}'), 'ops'),
    ts('Rollup lag', q('max by (module, rollup) (eduworld_rollup_lag_seconds)', '{{module}} {{rollup}}'), 's'),
    ts('Thời gian chốt ngày', q('max(eduworld_rollup_day_close_duration_seconds)', 'day close'), 's'),
    ts('Replica lag', q('max by (instance) (pg_replication_lag_seconds)', '{{instance}}'), 's'),
    ts('Export theo trạng thái', q('sum by (status) (increase(eduworld_export_jobs_total[1h]))', '{{status}}')),
  ] },
  // Counts and ratios by group only: no message content, no student or school identifier (AC-16, R14, R16).
  { slug: 'chat-safety', title: 'Chat Safety', tags: ['safety'], panels: [
    ts('Tin nhắn theo quyết định', q('sum by (decision) (rate(eduworld_chat_messages_total[5m]))', '{{decision}}'), 'ops'),
    ts('Tỷ lệ blocked/masked/held', q('sum by (decision) (rate(eduworld_chat_messages_total{decision=~"blocked|masked|held"}[15m])) / ignoring (decision) group_left sum(rate(eduworld_chat_messages_total[15m]))', '{{decision}}'), 'percentunit'),
    ts('Phát hiện PII', q('sum(rate(eduworld_chat_sensitive_matches_total{category="pii"}[5m]))', 'pii'), 'ops'),
    ts('Báo cáo theo lý do', q('sum by (reason) (rate(eduworld_reports_total[1h]))', '{{reason}}'), 'ops'),
    ts('Hàng đợi kiểm duyệt', q('sum by (priority) (eduworld_moderation_queue_size)', '{{priority}}')),
    ts('Tuổi case chưa nhận lâu nhất', q('max by (severity) (eduworld_moderation_case_unclaimed_age_seconds)', '{{severity}}'), 's'),
    ts('Thông báo trường quá hạn', q('sum(eduworld_moderation_school_notifications_overdue)', 'quá hạn')),
    stat('Bước kiểm duyệt đang tắt', q('count(eduworld_chat_policy_disabled == 1) or vector(0)')),
  ] },
  { slug: 'data-stores', title: 'Data stores', tags: ['datastore'], panels: [
    ts('PostgreSQL TPS', q('sum by (instance) (rate(pg_stat_database_xact_commit[5m]) + rate(pg_stat_database_xact_rollback[5m]))', '{{instance}}'), 'ops'),
    ts('Kết nối PostgreSQL', q('sum by (instance) (pg_stat_database_numbackends)', '{{instance}}')),
    ts('Replication lag', q('max by (instance) (pg_replication_lag_seconds)', '{{instance}}'), 's'),
    ts('Disk node data-pg', q('1 - min by (node) (node_filesystem_avail_bytes{role="data-pg",fstype!~"tmpfs|overlay"} / node_filesystem_size_bytes{role="data-pg",fstype!~"tmpfs|overlay"})', '{{node}}'), 'percentunit'),
    ts('Valkey memory', q('max by (instance) (redis_memory_used_bytes)', '{{instance}}'), 'bytes'),
    ts('Valkey ops/s', q('sum by (instance) (rate(redis_commands_processed_total[5m]))', '{{instance}}'), 'ops'),
    ts('Valkey evictions', q('sum by (instance) (rate(redis_evicted_keys_total[5m]))', '{{instance}}'), 'ops'),
    ts('Pool acquire p95', q(p95('eduworld_db_pool_acquire_duration_seconds', 'service, module'), '{{service}} {{module}}'), 's'),
  ] },
  { slug: 'async-integration', title: 'Async & Integration', tags: ['async'], panels: [
    ts('Outbox lag theo module', q('max by (module) (eduworld_outbox_lag_seconds)', '{{module}}'), 's'),
    ts('NATS pending theo stream/consumer', q('max by (stream, consumer) (nats_consumer_num_pending{stream=~"EVT_.*|CHAT_MESSAGES"})', '{{stream}} {{consumer}}')),
    ts('DLQ', q('max by (stream) (nats_stream_total_messages{stream=~"DLQ_.*"})', '{{stream}}')),
    ts('Projection lag', q('max by (module, projection) (eduworld_projection_lag_seconds)', '{{module}} {{projection}}'), 's'),
    ts('Saga theo kết quả / kẹt', [q('sum by (saga, result) (increase(eduworld_saga_runs_total[1h]))', '{{saga}} {{result}}'), q('sum(eduworld_saga_stuck)', 'stuck')]),
    ts('SIS sync theo trạng thái', q('sum by (status) (increase(eduworld_sis_sync_runs_total[1h]))', '{{status}}')),
    ts('SIS reconciliation issues', q('sum(eduworld_sis_reconciliation_issues)', 'issues')),
  ] },
  { slug: 'identity', title: 'Identity', tags: ['identity'], panels: [
    ts('Đăng nhập theo kết quả', q('sum by (result) (rate(eduworld_auth_logins_total[5m]))', '{{result}}'), 'ops'),
    ts('Đăng nhập game theo kết quả', q('sum by (result) (rate(eduworld_auth_game_login_total[5m]))', '{{result}}'), 'ops'),
    ts('Keycloak events', q('sum by (event) (rate(keycloak_user_events_total[5m]))', '{{event}}'), 'ops'),
    ts('JWKS refresh', q('sum by (result) (rate(eduworld_jwks_refresh_total[15m]))', '{{result}}'), 'ops'),
    ts('Hàng chờ đăng nhập', q('max(eduworld_auth_waiting_room_queued)', 'queued')),
    ts('SLI đăng nhập < 3 s', q('eduworld:sli_login:ratio_rate5m', 'SLI'), 'percentunit'),
  ] },
  { slug: 'client', title: 'Client', tags: ['client'], panels: [
    ts('FPS p50 theo hạng máy', q('histogram_quantile(0.5, sum by (le, device_tier) (rate(eduworld_client_fps_bucket[1h])))', '{{device_tier}}')),
    ts('Crash rate theo phiên bản', q('sum by (client_version) (increase(eduworld_client_crashes_total[1h])) / sum by (client_version) (increase(eduworld_client_sessions_total[1h]))', '{{client_version}}'), 'percentunit'),
    ts('Tải nội dung theo nguồn', q('sum by (source) (rate(eduworld_client_content_download_bytes_total[1h]))', '{{source}}'), 'Bps'),
  ] },
];

function build(d) {
  let x = 0;
  let y = 0;
  let rowH = 0;
  const panels = d.panels.map((p, i) => {
    const w = p.w ?? 12;
    const h = p.h ?? (p.kind === 'stat' ? 4 : 8);
    if (x + w > 24) { x = 0; y += rowH; rowH = 0; }
    const gridPos = { h, w, x, y };
    x += w;
    rowH = Math.max(rowH, h);
    const ds = p.datasource ?? PROM;
    const panel = {
      id: i + 1, type: p.kind, title: p.title, datasource: ds, gridPos,
      targets: p.targets.map((t, j) => ({ datasource: ds, refId: String.fromCharCode(65 + j), ...t })),
    };
    if (p.kind !== 'nodeGraph') panel.fieldConfig = { defaults: { unit: p.unit ?? 'short' }, overrides: [] };
    if (p.kind === 'timeseries') panel.options = { legend: { displayMode: 'list', placement: 'bottom' }, tooltip: { mode: 'multi' } };
    if (p.kind === 'stat') panel.options = { reduceOptions: { calcs: ['lastNotNull'] }, colorMode: 'value' };
    return panel;
  });
  return {
    uid: d.slug, title: d.title, tags: ['eduworld', ...d.tags], timezone: 'browser', schemaVersion: 39, version: 1,
    editable: false, refresh: '30s', time: { from: 'now-6h', to: 'now' },
    templating: { list: (d.vars ?? []).map((v) => ({
      name: v.name, type: 'query', datasource: PROM, query: { query: v.query, refId: 'PrometheusVariableQueryEditor-VariableQuery' },
      refresh: 2, includeAll: Boolean(v.multi), multi: Boolean(v.multi), current: {}, sort: 1,
    })) },
    annotations: { list: [] },
    panels,
  };
}

export function renderAll() {
  return Object.fromEntries(DASHBOARDS.map((d) => [`${d.slug}.json`, `${JSON.stringify(build(d), null, 2)}\n`]));
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const check = process.argv.includes('--check');
  const files = renderAll();
  let stale = 0;
  if (!check) fs.mkdirSync(DASHBOARDS_DIR, { recursive: true });
  for (const [name, text] of Object.entries(files)) {
    const p = path.join(DASHBOARDS_DIR, name);
    if (check) {
      if (!fs.existsSync(p) || fs.readFileSync(p, 'utf8') !== text) { console.error(`stale: ${p} (run node scripts/obs/build-dashboards.mjs)`); stale++; }
    } else fs.writeFileSync(p, text);
  }
  if (check) {
    for (const f of fs.existsSync(DASHBOARDS_DIR) ? fs.readdirSync(DASHBOARDS_DIR) : []) if (f.endsWith('.json') && !files[f]) { console.error(`not generated: ${f}`); stale++; }
  }
  console.log(`build-dashboards: ${Object.keys(files).length} dashboards${check ? `, ${stale} stale` : ' written'}`);
  process.exit(stale ? 1 : 0);
}
