// Test of alert-drill.sh (F131 T022, F131-AC-11) with a stubbed curl (MOCK Alertmanager, AGENTS R17): the drill posts
// the critical alert with the static labels of its rule and drill="true", waits for a notification counted by
// Alertmanager, resolves it and appends a row to the report. Run: node --test scripts/obs/
import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
const bash = spawnSync('bash', ['-c', 'echo ok']).status === 0;

test('drill of one critical alert: labels of the rule, delivery measured, resolved, report row', { skip: !bash }, () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'obs-drill-'));
  const log = path.join(dir, 'calls.log').replace(/\\/g, '/');
  const state = path.join(dir, 'n').replace(/\\/g, '/');
  const stub = path.join(dir, 'curl');
  fs.writeFileSync(stub, `#!/usr/bin/env bash
# MOCK Alertmanager: /metrics counts one more notification at every call; POST /api/v2/alerts is logged.
echo "$*" >> "${log}"
case "$*" in
  *"/metrics"*) n=$(cat "${state}" 2>/dev/null || echo 0); n=$((n + 1)); echo "$n" > "${state}"
    echo "alertmanager_notifications_total{integration=\\"webhook\\"} $n" ;;
  *"/api/v2/alerts"*) echo '{}' ;;
  *) exit 7 ;;
esac
`, { mode: 0o755 });
  const report = path.join(dir, 'drill.md').replace(/\\/g, '/');
  const r = spawnSync('bash', ['scripts/obs/alert-drill.sh', '--env=staging', '--alert=ModerationP1Unclaimed', '--yes'], {
    cwd: repo, encoding: 'utf8',
    env: { ...process.env, CURL_BIN: stub.replace(/\\/g, '/'), EW_ALERTMANAGER_URL: 'http://am.invalid', EW_DRILL_REPORT: report, EW_ACTOR: 'drill-test' },
  });
  assert.equal(r.status, 0, r.stderr + r.stdout);
  const calls = fs.readFileSync(log, 'utf8');
  const posts = calls.split('\n').filter((l) => l.includes('/api/v2/alerts'));
  assert.equal(posts.length, 2, 'fire then resolve');
  assert.match(posts[0], /"alertname":"ModerationP1Unclaimed"/);
  assert.match(posts[0], /"team":"safety"/, 'static labels of the rule route it like the real alert');
  assert.match(posts[0], /"drill":"true"/);
  assert.match(posts[1], /"endsAt"/);
  const md = fs.readFileSync(report, 'utf8');
  assert.match(md, /\| `ModerationP1Unclaimed` \| synthetic \(alert-drill\.sh\) \|.*\| drill-test \| đạt \|/);
});

test('unknown alert name: exit 1 without posting', { skip: !bash }, () => {
  const r = spawnSync('bash', ['scripts/obs/alert-drill.sh', '--env=staging', '--alert=NoSuchAlert', '--yes'], {
    cwd: repo, encoding: 'utf8', env: { ...process.env, EW_ALERTMANAGER_URL: 'http://am.invalid', CURL_BIN: 'false' },
  });
  assert.equal(r.status, 1);
  assert.match(r.stderr, /no critical alert matches NoSuchAlert/);
});
