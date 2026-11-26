// Tests of check-label-policy (F131 T008: F131-AC-06). Run: node --test scripts/obs/
import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { checkLabelPolicy, labelsInExpr, badRoutes, parseExposition } from './check-label-policy.mjs';

const tmp = () => fs.mkdtempSync(path.join(os.tmpdir(), 'obs-labels-'));

test('extracts label names from grouping, matchers, label_replace and templates', () => {
  const l = labelsInExpr('sum by (module, school_id) (rate(x{user_id="1",route=~"/a"}[5m])) / on (tenant) y and label_replace(z, "email", "$1", "a", "b") {{ $labels.student_id }}');
  for (const n of ['module', 'school_id', 'user_id', 'route', 'tenant', 'email', 'student_id']) assert.ok(l.has(n), n);
});

test('route values must be templates', () => {
  assert.deepEqual(badRoutes('x{route="/api/v1/students/{student_id}"}'), []);
  assert.deepEqual(badRoutes('x{route="/api/v1/students/123"}'), ['/api/v1/students/123']);
  assert.equal(badRoutes('x{route="/a/0f8fad5b-d9cb-469f-a165-70867728950e"}').length, 1);
});

test('a rule using school_id fails (quickstart step 6)', async () => {
  const d = tmp();
  fs.mkdirSync(path.join(d, 'rules'));
  fs.writeFileSync(path.join(d, 'rules', 'x.yaml'), 'groups:\n  - name: g\n    rules:\n      - alert: A\n        expr: sum by (school_id) (rate(x[5m])) > 0\n        labels: { severity: warning }\n');
  const v = await checkLabelPolicy({ rulesDir: path.join(d, 'rules'), dashboardsDir: path.join(d, 'none') });
  assert.equal(v.length, 1);
  assert.match(v[0].message, /school_id is forbidden/);
});

test('a dashboard query with a forbidden label fails', async () => {
  const d = tmp();
  fs.mkdirSync(path.join(d, 'rules')); fs.mkdirSync(path.join(d, 'dash'));
  fs.writeFileSync(path.join(d, 'dash', 'x.json'), JSON.stringify({ panels: [{ targets: [{ expr: 'sum by (username) (rate(x[5m]))' }] }] }));
  const v = await checkLabelPolicy({ rulesDir: path.join(d, 'rules'), dashboardsDir: path.join(d, 'dash') });
  assert.equal(v.length, 1);
});

test('scraped metrics: forbidden label and concrete route fail, clean exposition passes', async () => {
  const d = tmp();
  fs.mkdirSync(path.join(d, 'rules'));
  const bad = path.join(d, 'bad.txt');
  fs.writeFileSync(bad, '# TYPE x counter\nx_total{module="chat",user_id="u1"} 1\neduworld_http_server_requests_total{module="game",route="/api/v1/players/42",method="GET",status_class="2xx",code=""} 3\n');
  const good = path.join(d, 'good.txt');
  fs.writeFileSync(good, 'eduworld_http_server_requests_total{module="game",route="/api/v1/players/{player_id}",method="GET",status_class="2xx",code=""} 3\n');
  const v = await checkLabelPolicy({ rulesDir: path.join(d, 'rules'), dashboardsDir: path.join(d, 'none'), metrics: [bad, good] });
  assert.deepEqual(v.map((x) => x.rule).sort(), ['forbidden-label', 'route-template']);
  assert.equal(parseExposition(fs.readFileSync(good, 'utf8'))[0].labels.route, '/api/v1/players/{player_id}');
});

test('the repository rules and dashboards follow the policy', async () => {
  assert.deepEqual(await checkLabelPolicy(), []);
});
