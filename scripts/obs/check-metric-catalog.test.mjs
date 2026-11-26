// Tests of check-metric-catalog (F131 T007: F131-AC-02). Run: node --test scripts/obs/
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { checkCatalog, namingViolations } from './check-metric-catalog.mjs';
import { parseMetrics, readMonitoring } from './lib-monitoring-md.mjs';
import { parseExposition } from './check-label-policy.mjs';

const MD = `## 3. Metric bắt buộc

| Nhóm | Metric Prometheus | Loại | Label chính | Nguồn |
|---|---|---|---|---|
| Error Rate | \`eduworld_http_server_requests_total\` | Counter | \`service\`, \`module\`, \`route\` | \`core-api\` |
| Online | \`eduworld_players_online\` | Gauge | \`platform\` | realtime |
| Latency | \`eduworld_http_server_duration_seconds\` | Histogram | \`service\`, \`module\` | \`core-api\` |
| DB | \`pg_stat_database_*\` | — | | postgres_exporter |

## 4. Dashboard
`;
const catalog = parseMetrics(MD).main;
const samples = (text, extra = {}) => parseExposition(text).map((s) => ({ name: s.name, labels: { ...extra, ...s.labels } }));
const GOOD = `eduworld_http_server_requests_total{module="game",route="/x",method="GET",status_class="2xx",code=""} 1
eduworld_players_online{platform="windows",grade_band="g1_3"} 3
eduworld_http_server_duration_seconds_bucket{module="game",route="/x",method="GET",status_class="2xx",le="0.1"} 1
pg_stat_database_xact_commit{datname="eduworld_game"} 9
`;

test('parses the mandatory table with types and labels', () => {
  assert.deepEqual(catalog.map((m) => [m.name, m.type]), [
    ['eduworld_http_server_requests_total', 'counter'], ['eduworld_players_online', 'gauge'],
    ['eduworld_http_server_duration_seconds', 'histogram'], ['pg_stat_database_*', null]]);
  assert.deepEqual(catalog[0].labels, ['service', 'module', 'route']);
});

test('complete scrape with the labels Prometheus attaches passes', () => {
  assert.deepEqual(checkCatalog(catalog, samples(GOOD, { service: 'core-api' })), []);
});

test('missing metric, missing service/module label and wrong type are listed (F131-AC-02)', () => {
  const text = `eduworld_http_server_requests_total{route="/x"} 1
eduworld_http_server_duration_seconds{module="game"} 0.2
pg_stat_database_xact_commit{datname="a"} 1
`;
  const v = checkCatalog(catalog, samples(text));
  const msgs = v.map((x) => `${x.rule} ${x.message}`);
  assert.ok(msgs.some((m) => m.startsWith('label eduworld_http_server_requests_total: series without label service')));
  assert.ok(msgs.some((m) => m.startsWith('label eduworld_http_server_requests_total: series without label module')));
  assert.ok(msgs.some((m) => m.startsWith('missing mandatory metric eduworld_players_online')));
  assert.ok(msgs.some((m) => m.startsWith('type eduworld_http_server_duration_seconds is exposed but not as a histogram')));
});

test('wildcard entries need one matching series', () => {
  const v = checkCatalog(catalog, samples(GOOD.split('\n').filter((l) => !l.startsWith('pg_')).join('\n'), { service: 'core-api' }));
  assert.equal(v.length, 1);
  assert.match(v[0].message, /pg_stat_database_\*/);
});

test('origin filter checks one binary or exporter at a time', () => {
  assert.deepEqual(checkCatalog(catalog, samples('pg_stat_database_tup_fetched 1\n'), { origins: ['postgres'] }), []);
});

test('naming rules: counter _total, gauge without _total, histogram unit; exporter names exempt', () => {
  assert.deepEqual(namingViolations({ name: 'eduworld_x', type: 'counter' }), ['counter eduworld_x must end with _total']);
  assert.deepEqual(namingViolations({ name: 'eduworld_x_total', type: 'gauge' }), ['gauge eduworld_x_total must not end with _total']);
  assert.deepEqual(namingViolations({ name: 'eduworld_wait', type: 'histogram' }), ['histogram eduworld_wait needs a unit suffix (_seconds/_bytes)']);
  assert.deepEqual(namingViolations({ name: 'apisix_http_status', type: 'counter' }), []);
});

test('the MONITORING §3 catalogue itself follows the naming rules', () => {
  const { main } = parseMetrics(readMonitoring());
  assert.ok(main.length >= 50);
  assert.deepEqual(main.flatMap(namingViolations), []);
});
