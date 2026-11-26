// Tests of check-alerts (F131 T020: F131-AC-08, AC-12, AC-13). Run: node --test scripts/obs/
import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { checkAlerts } from './check-alerts.mjs';
import { parseAlerts, splitRow } from './lib-monitoring-md.mjs';

const MD = `# M
## 5. Alert rules — [Confirmed]

| Alert | Điều kiện | Mức | Hành động |
|---|---|---|---|
| \`FooDown\` | \`up{job="foo"} == 0\` | Critical | x |
| \`BarSlow\` / \`BarStuck\` | p95 \`a|b\` > 1 s | Warning (Critical nếu …) | y |

## 6. SLO
`;
const SECTIONS = ['Triệu chứng', 'Tác động', 'Chẩn đoán', 'Xử lý', 'Leo thang', 'Liên quan'];

function fixture({ rules, runbooks = {}, tests = '' }) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'obs-alerts-'));
  fs.writeFileSync(path.join(dir, 'MONITORING.md'), MD);
  for (const d of ['rules', 'runbooks', 'tests']) fs.mkdirSync(path.join(dir, d));
  fs.writeFileSync(path.join(dir, 'rules', 'x.yaml'), rules);
  for (const [n, body] of Object.entries(runbooks)) fs.writeFileSync(path.join(dir, 'runbooks', `${n}.md`), body);
  fs.writeFileSync(path.join(dir, 'tests', 'x.test.yaml'), tests);
  return { monitoring: path.join(dir, 'MONITORING.md'), rulesDir: path.join(dir, 'rules'), runbooksDir: path.join(dir, 'runbooks'), testsDir: path.join(dir, 'tests') };
}
const rule = (name, sev = 'critical', extra = '') => `      - alert: ${name}
        expr: vector(1)
        labels: { severity: ${sev} }
        annotations:
          summary: "s"
          runbook_url: "docs/11-operations/runbooks/${name}.md"
          source: "docs/11-operations/MONITORING.md#5-alert-rules--confirmed"${extra}
`;
const runbook = (secs = SECTIONS) => `# R\n\n${secs.map((s) => `## ${s}\n\nx\n`).join('\n')}`;
const tcase = (name, fire) => `      - eval_time: 1m\n        alertname: ${name}\n        exp_alerts: ${fire ? '[{exp_labels: {severity: critical}}]' : '[]'}\n`;

test('parses names, severities and pipes inside backticks', () => {
  assert.deepEqual(splitRow('| `a|b` | c \\| d |'), ['`a|b`', 'c | d']);
  const a = parseAlerts(MD);
  assert.deepEqual(a.map((x) => x.name), ['FooDown', 'BarSlow', 'BarStuck']);
  assert.deepEqual([...a[1].severities].sort(), ['critical', 'warning']);
});

test('reports alerts of the table without rule and rules outside the table (F131-AC-08)', () => {
  const f = fixture({ rules: `groups:\n  - name: g\n    rules:\n${rule('FooDown')}${rule('Extra')}` });
  const rules = checkAlerts(f).violations.map((v) => `${v.rule}:${v.message.split(':')[0]}`);
  assert.ok(rules.includes('missing-rule:alert BarSlow of MONITORING §5 has no rule'));
  assert.ok(rules.includes('not-in-table:Extra'));
});

test('checks severity, annotations and runbook_url (F131-AC-08, AC-12)', () => {
  const bad = `groups:\n  - name: g\n    rules:\n      - alert: FooDown\n        expr: vector(1)\n        labels: { severity: warning }\n        annotations: { runbook_url: "x" }\n`;
  const v = checkAlerts({ ...fixture({ rules: bad }), only: 'rules' }).violations.map((x) => x.rule);
  for (const r of ['severity-table', 'summary', 'runbook-url', 'source']) assert.ok(v.includes(r), r);
});

test('requires a firing and a non-firing promtool case per alert', () => {
  const rules = `groups:\n  - name: g\n    rules:\n${rule('FooDown')}${rule('BarSlow', 'warning')}${rule('BarStuck', 'warning')}`;
  const tests = `tests:\n  - name: t\n    alert_rule_test:\n${tcase('FooDown', true)}${tcase('FooDown', false)}${tcase('BarSlow', true)}`;
  const v = checkAlerts({ ...fixture({ rules, tests }), only: 'rules' }).violations.map((x) => `${x.rule}:${x.message.split(':')[0]}`);
  assert.ok(v.includes('test-quiet:BarSlow'));
  assert.ok(v.includes('test-firing:BarStuck'));
  assert.ok(!v.some((x) => x.endsWith(':FooDown')));
});

test('runbooks: missing file, missing or misordered section, orphan (F131-AC-13)', () => {
  const rules = `groups:\n  - name: g\n    rules:\n${rule('FooDown')}${rule('BarSlow', 'warning')}${rule('BarStuck', 'warning')}`;
  const noEscalation = SECTIONS.filter((s) => s !== 'Leo thang');
  const swapped = [...SECTIONS]; [swapped[0], swapped[1]] = [swapped[1], swapped[0]];
  const f = fixture({ rules, runbooks: { FooDown: runbook(noEscalation), BarSlow: runbook(swapped), Ghost: runbook() } });
  const v = checkAlerts({ ...f, only: 'runbooks' }).violations.map((x) => x.rule);
  assert.ok(v.includes('runbook-missing'));     // BarStuck
  assert.ok(v.includes('runbook-section'));     // FooDown without "Leo thang"
  assert.ok(v.includes('runbook-order'));       // BarSlow
  assert.ok(v.includes('runbook-orphan'));      // Ghost
});

test('a complete fixture passes', () => {
  const rules = `groups:\n  - name: g\n    rules:\n${rule('FooDown')}${rule('BarSlow', 'warning')}${rule('BarStuck', 'critical')}`;
  const tests = `tests:\n  - name: t\n    alert_rule_test:\n${['FooDown', 'BarSlow', 'BarStuck'].map((n) => tcase(n, true) + tcase(n, false)).join('')}`;
  const f = fixture({ rules, tests, runbooks: { FooDown: runbook(), BarSlow: runbook(), BarStuck: runbook() } });
  assert.deepEqual(checkAlerts(f).violations, []);
});

test('the repository: MONITORING §5 ↔ rules ↔ runbooks ↔ promtool tests', () => {
  const r = checkAlerts();
  assert.deepEqual(r.violations.map((v) => `${v.rule} ${v.message}`), []);
  assert.ok(r.alerts >= 100);
});
