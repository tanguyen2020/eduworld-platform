// F130-AC-01, AC-06, AC-07, AC-08, AC-09, AC-24: the committed staging/production stacks pass the Swarm policy, and
// each class of violation makes the `swarm` validation (deploy-config.yml) fail with file:line. Mutations are applied to
// a temporary copy of deploy/.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { copyDeploy, mutate, ROOT } from './helpers.mjs';
import * as stacks from '../../scripts/deploy/check-stacks.mjs';
import * as secrets from '../../scripts/deploy/check-secret-refs.mjs';
import * as schedule from '../../scripts/deploy/check-scale-schedule.mjs';
import { loadPolicy } from '../../scripts/deploy/lib-policy.mjs';

const policy = loadPolicy();
const rules = (out) => out.map((v) => v.rule);

for (const env of ['staging', 'production']) {
  test(`committed ${env} stacks pass every rule`, () => {
    const dir = path.join(ROOT, 'deploy', 'swarm', env);
    assert.deepEqual(stacks.checkEnv(dir, policy), []);
    assert.deepEqual(secrets.checkEnv(dir, policy), []);
    assert.deepEqual(schedule.checkEnv(dir, policy), []);
  });
}

test('F130-AC-01: latest tag, missing placement/resources/update_config/healthcheck fail with a line', () => {
  const { env } = copyDeploy();
  const dir = env('production');
  mutate(path.join(dir, 'infra.stack.yml'), 'image: valkey/valkey:8.1.4', 'image: valkey/valkey:latest');
  mutate(path.join(dir, 'app.stack.yml'), '        constraints: [node.labels.role == app]\n        preferences: [{ spread: node.labels.zone }]\n      resources:\n        limits: { cpus: "4", memory: 4G }',
    '        preferences: [{ spread: node.labels.zone }]\n      resources:\n        limits: { cpus: "4", memory: 4G }');
  mutate(path.join(dir, 'ops.stack.yml'), '      resources: { limits: { cpus: "0.25", memory: 128M }, reservations: { cpus: "0.05", memory: 32M } }\n      update_config: { parallelism: 1, order: stop-first',
    '      update_config: { parallelism: 1, order: stop-first');
  mutate(path.join(dir, 'app.stack.yml'), 'image: ghcr.io/${GHCR_ORG}/eduworld/realtime:${BACKEND_VERSION}', 'image: ghcr.io/${GHCR_ORG}/eduworld/realtime:latest');
  const out = stacks.checkEnv(dir, policy);
  const r = rules(out);
  for (const want of ['image-latest', 'placement', 'resources', 'backend-image']) assert.ok(r.includes(want), `${want} not reported: ${JSON.stringify(out)}`);
  assert.ok(out.every((v) => v.line >= 1 && v.file));
  const latest = out.find((v) => v.rule === 'image-latest');
  // the image comes from the x-valkey anchor: the report points at the service using it
  assert.match(fs.readFileSync(latest.file, 'utf8').split(/\r?\n/)[latest.line - 1], /^ {2}valkey-[a-z-]+:/);
});

test('F130-AC-01: update_config must be start-first / rollback with monitor, rollback_config required', () => {
  const { env } = copyDeploy();
  const dir = env('staging');
  mutate(path.join(dir, 'app.stack.yml'), 'update_config: { parallelism: 1, delay: 10s, order: start-first, failure_action: rollback, monitor: 60s, max_failure_ratio: 0 }',
    'update_config: { parallelism: 1, delay: 10s, order: stop-first, failure_action: continue }');
  const r = rules(stacks.checkEnv(dir, policy));
  for (const want of ['update-order', 'update-failure', 'update-monitor']) assert.ok(r.includes(want), want);
});

test('F130-AC-06: backend healthcheck is `<binary> healthcheck --endpoint=/livez`, drain grace ≥ 40 s / 150 s', () => {
  const { env } = copyDeploy();
  const dir = env('production');
  mutate(path.join(dir, 'app.stack.yml'), 'test: ["CMD", "/app/realtime", "healthcheck", "--endpoint=/livez"]', 'test: ["CMD", "/app/realtime", "healthcheck"]');
  mutate(path.join(dir, 'app.stack.yml'), 'stop_grace_period: 150s', 'stop_grace_period: 30s');
  const out = stacks.checkEnv(dir, policy).filter((v) => /realtime/.test(v.message));
  assert.deepEqual(rules(out).sort(), ['healthcheck-livez', 'stop-grace']);
});

test('F130-AC-07: datastore with a published port or on edge, service on a forbidden network fail', () => {
  const { env } = copyDeploy();
  const dir = env('production');
  mutate(path.join(dir, 'data.stack.yml'), '    hostname: pg-1\n', '    hostname: pg-1\n    ports: ["5432:5432"]\n');
  mutate(path.join(dir, 'infra.stack.yml'), '    hostname: valkey-cache\n', '    hostname: valkey-cache\n    networks: [data, edge]\n');
  mutate(path.join(dir, 'app.stack.yml'), '    networks: [app, data]                                        # no edge: no inbound', '    networks: [edge, app, data]');
  const r = rules(stacks.checkEnv(dir, policy));
  for (const want of ['datastore-ports', 'datastore-edge', 'network-matrix']) assert.ok(r.includes(want), want);
});

test('placement pins must exist in the node inventory with the same role', () => {
  const { env } = copyDeploy();
  const dir = env('production');
  mutate(path.join(dir, 'data.stack.yml'), '[node.labels.role == data-pg, node.labels.name == pg-2] }\n      resources: { limits: { cpus: "30"', '[node.labels.role == data-pg, node.labels.name == pg-9] }\n      resources: { limits: { cpus: "30"');
  assert.ok(rules(stacks.checkEnv(dir, policy)).includes('placement-node'));
});

test('F130-AC-09: secrets mounted outside their owner fail (DEPLOYMENT §13)', () => {
  const cases = [
    ['core-api', 'chat_purger_dsn', /only be mounted by worker-chat/],
    ['worker-events', 'kc_game_backend', /only be mounted by core-api/],
    ['realtime', 'learning_db_dsn', /may only be mounted by core-api, worker-events/],
    ['worker-chat', 'game_db_dsn', /module game, not loaded by worker-chat/],
    ['core-api', 'learning_migrator_dsn', /never be mounted by a stack service/],
    ['worker-jobs', 'core_nats_creds', /binary core, not to worker-jobs/],
    ['core-api', 'unknown_secret', /matches no rule/],
  ];
  for (const [svc, secret, re] of cases) assert.match(secrets.checkMount(policy, svc, secret) ?? 'null', re, `${svc} ← ${secret}`);
  assert.equal(secrets.checkMount(policy, 'worker-chat', 'chat_purger_dsn'), null);
  assert.equal(secrets.checkMount(policy, 'core-api', 'kc_svc_identity'), null);
  assert.equal(secrets.checkMount(policy, 'worker-jobs', 'identity_partition_admin_dsn'), null);

  const { env } = copyDeploy();
  const dir = env('production');
  mutate(path.join(dir, 'app.stack.yml'), 'secrets: [realtime_nats_creds, realtime_valkey_password]', 'secrets: [realtime_nats_creds, realtime_valkey_password, learning_db_dsn]');
  mutate(path.join(dir, 'app.stack.yml'), '       core_nats_creds, core_valkey_password, s3_core_access_key, s3_core_secret_key]', '       core_nats_creds, core_valkey_password, s3_core_access_key, s3_core_secret_key, chat_purger_dsn]');
  const out = secrets.checkEnv(dir, policy);
  assert.ok(out.some((v) => v.rule === 'realtime-db'));
  assert.ok(out.some((v) => v.rule === 'secret-owner' && /core-api: chat_purger_dsn/.test(v.message)));
});

test('F130-AC-08: secret values in env files or stacks, unversioned or file secrets fail', () => {
  const { env } = copyDeploy();
  const dir = env('staging');
  fs.appendFileSync(path.join(dir, 'core-api.env'), 'EW_KC_GAME_CLIENT_SECRET=s3cr3t-value\nEW_EXTRA_URL=postgres://learning_app:hunter2@pg-router:5432/x\n');
  mutate(path.join(dir, 'app.stack.yml'), '      EW_WORKER_PROFILE: chat\n', '      EW_WORKER_PROFILE: chat\n      EW_VALKEY_PASSWORD: plain\n');
  mutate(path.join(dir, 'app.stack.yml'), 'kc_portal: { external: true, name: "kc_portal_v${SECRET_VER_KC_PORTAL}" }', 'kc_portal: { file: ./kc_portal.txt }');
  const out = secrets.checkEnv(dir, policy);
  const msgs = out.map((v) => `${v.rule} ${v.message}`).join('\n');
  assert.match(msgs, /secret-value core-api\.env: EW_KC_GAME_CLIENT_SECRET carries a value/);
  assert.match(msgs, /secret-pattern core-api\.env: EW_EXTRA_URL looks like a credentials embedded in a URL/);
  assert.match(msgs, /secret-value worker-chat: EW_VALKEY_PASSWORD carries a value/);
  assert.match(msgs, /secret-external secret kc_portal/);
  const envHit = out.find((v) => /EW_KC_GAME_CLIENT_SECRET/.test(v.message));
  assert.match(envHit.file, /core-api\.env$/);
});

test('a *_FILE variable must point to a mounted secret', () => {
  const { env } = copyDeploy();
  const dir = env('production');
  mutate(path.join(dir, 'app.stack.yml'), '      EW_DB_CHAT_PURGER_URL_FILE: /run/secrets/chat_purger_dsn     # gameplay-chat-purge only (ADR-0033)\n',
    '      EW_DB_CHAT_PURGER_URL_FILE: /run/secrets/chat_purger_dsn     # gameplay-chat-purge only (ADR-0033)\n      EW_KC_GAME_CLIENT_SECRET_FILE: /run/secrets/kc_game_backend\n');
  assert.ok(secrets.checkEnv(dir, policy).some((v) => v.rule === 'secret-file-unmounted'));
});

test('F130-AC-24: invalid scale schedule fails with file:line and JSON path', () => {
  const { env } = copyDeploy();
  const dir = env('production');
  const f = path.join(dir, 'scale-schedule.yaml');
  mutate(f, '  - { name: core-api, baseline: 6, peak: 16 }', '  - { name: core-api, baseline: 6, peak: 4 }');
  mutate(f, '  - { name: realtime, baseline: 4, peak: 6 }', `  - { name: realtime, peak: 6 }\n  - { name: ghost, baseline: 1, peak: 2 }\n  - { name: ${'x'.repeat(65)}, baseline: 1, peak: 2 }`);
  const out = schedule.checkEnv(dir, policy);
  const text = out.map((v) => `${v.line} ${v.path} ${v.message}`).join('\n');
  assert.match(text, /\/services\/2\/peak peak 4 is lower than baseline 6/);
  assert.match(text, /\/services\/3 must have required property 'baseline'/);
  assert.match(text, /service "ghost" is not in the stack files/);
  assert.match(text, /\/services\/5\/name must NOT have more than 64 characters/);
  const peak = out.find((v) => v.path === '/services/2/peak');
  assert.match(fs.readFileSync(f, 'utf8').split(/\r?\n/)[peak.line - 1], /core-api/);
});
