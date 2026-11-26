// Tests of check-dashboards and build-dashboards (F131 T028: F131-AC-14, AC-16). Run: node --test scripts/obs/
import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { checkDashboards, metricsInExpr } from './check-dashboards.mjs';
import { renderAll } from './build-dashboards.mjs';
import { DASHBOARDS_DIR } from './lib-monitoring-md.mjs';

const PROM = { type: 'prometheus', uid: 'prometheus' };

function withDashboards(list) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'obs-dash-'));
  for (const [name, text] of Object.entries(renderAll())) fs.writeFileSync(path.join(dir, name), text);
  for (const d of list) fs.writeFileSync(path.join(dir, `${d.uid}.json`), JSON.stringify(d));
  return dir;
}
const panel = (expr, extra = {}) => ({ type: 'timeseries', title: 'p', datasource: PROM, targets: [{ refId: 'A', datasource: PROM, expr }], ...extra });

test('extracts metric names, not functions, labels or variables', () => {
  assert.deepEqual([...metricsInExpr('histogram_quantile(0.95, sum by (le, module) (rate(eduworld_x_seconds_bucket{module="$module"}[5m]))) / on() group_left eduworld:slo_availability_target')].sort(),
    ['eduworld:slo_availability_target', 'eduworld_x_seconds_bucket']);
});

test('the generated dashboards cover MONITORING §4 and pass every rule', () => {
  const r = checkDashboards({ dashboardsDir: withDashboards([]) });
  assert.deepEqual(r.violations, []);
  assert.equal(r.dashboards, r.wanted);
});

test('the committed JSON is what build-dashboards generates', () => {
  for (const [name, text] of Object.entries(renderAll())) assert.equal(fs.readFileSync(path.join(DASHBOARDS_DIR, name), 'utf8'), text, name);
});

test('unknown datasource uid and metric outside MONITORING §3 fail (F131-AC-14)', () => {
  const dir = withDashboards([]);
  const api = JSON.parse(fs.readFileSync(path.join(dir, 'api.json'), 'utf8'));
  api.panels.push(panel('sum(rate(eduworld_made_up_total[5m]))'));
  api.panels.push({ ...panel('sum(up)'), datasource: { type: 'prometheus', uid: 'nope' } });
  fs.writeFileSync(path.join(dir, 'api.json'), JSON.stringify(api));
  const v = checkDashboards({ dashboardsDir: dir }).violations.map((x) => x.rule).sort();
  assert.deepEqual(v, ['datasource', 'metric']);
});

test('per-module dashboard without service/module variables fails; unknown dashboard fails', () => {
  const dir = withDashboards([{ uid: 'extra', title: 'Extra', tags: ['per-module'], templating: { list: [] }, panels: [] }]);
  const v = checkDashboards({ dashboardsDir: dir }).violations.map((x) => x.rule).sort();
  assert.deepEqual(v, ['not-in-table', 'variables', 'variables']);
});

test('Chat Safety: logs panel, raw series and identifying labels fail (F131-AC-16)', () => {
  const dir = withDashboards([]);
  const cs = JSON.parse(fs.readFileSync(path.join(dir, 'chat-safety.json'), 'utf8'));
  cs.panels.push({ type: 'logs', title: 'raw', datasource: { type: 'loki', uid: 'loki' }, targets: [{ refId: 'A', expr: '{service="core-api"}' }] });
  cs.panels.push(panel('rate(eduworld_chat_messages_total[5m])'));
  cs.panels.push(panel('sum by (tenant) (rate(eduworld_chat_messages_total[5m]))'));
  fs.writeFileSync(path.join(dir, 'chat-safety.json'), JSON.stringify(cs));
  const v = checkDashboards({ dashboardsDir: dir }).violations.filter((x) => x.rule === 'chat-safety');
  assert.equal(v.length, 3);
});

test('a dashboard of MONITORING §4 without JSON fails', () => {
  const dir = withDashboards([]);
  fs.rmSync(path.join(dir, 'client.json'));
  assert.deepEqual(checkDashboards({ dashboardsDir: dir }).violations.map((x) => x.rule), ['dashboard-missing']);
});
