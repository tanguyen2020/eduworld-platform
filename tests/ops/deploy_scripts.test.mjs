// F130-AC-02, AC-03, AC-04, AC-05, AC-07, AC-10 (script logic, stubbed docker/curl/sops — TEST DOUBLES, R17):
// migrate jobs gate the rollout, a single-service deploy/rollback touches only that service, production refuses a digest
// not deployed on staging or a deploy outside the maintenance window, the canary aborts on an alert, secrets-sync creates
// a new version only when the value changed and never removes the previous one. Needs bash + jq (CI ubuntu, or the
// test image of tests/ops/integration/run.sh); skipped elsewhere.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { ROOT, tmpdir, stub, run, hasCmd } from './helpers.mjs';

const skip = hasCmd('jq') ? false : 'jq not installed (runs in CI / ops test image)';

function rig(extraEnv = {}) {
  const stubs = tmpdir('ew-f130-stubs-');
  const state = tmpdir('ew-f130-state-');
  fs.mkdirSync(path.join(state, 'labels'));
  fs.mkdirSync(path.join(state, 'values'));
  stub(stubs, 'docker', `exec bash "${ROOT.replace(/\\/g, '/')}/tests/ops/docker-stub.sh" "$@"`);
  const secrets = ['identity', 'learning', 'game', 'competition', 'chat'].map((m) => `${m}_migrator_dsn_v1`);
  // every secret referenced by the staging/production stacks exists at v1
  for (const env of ['staging', 'production']) {
    for (const f of ['app', 'infra', 'data', 'ops']) {
      const text = fs.readFileSync(path.join(ROOT, 'deploy', 'swarm', env, `${f}.stack.yml`), 'utf8');
      for (const m of text.matchAll(/SECRET_VER_([A-Z0-9_]+)/g)) secrets.push(`${m[1].toLowerCase()}_v1`);
    }
  }
  fs.writeFileSync(path.join(state, 'secrets'), [...new Set(secrets)].join('\n') + '\n');
  const env = { STUB_DIR: state, EW_MIGRATE_POLL_SECONDS: '0', ...extraEnv };
  return { stubs, state, env, calls: (r) => r.log.split('\n').filter(Boolean) };
}

test('F130-AC-02: a failed migrate job stops the workflow before `docker stack deploy`', { skip }, () => {
  const { stubs, env, calls } = rig({ STUB_FAIL_MIGRATE: 'learning' });
  const r = run('scripts/deploy/deploy-service.sh', ['--env=staging', '--stack=app', '--version=v1.4.0'], { env, stubs });
  assert.equal(r.code, 1, r.stderr);
  const c = calls(r);
  assert.ok(c.some((l) => /service create .*--mode replicated-job .*migrate-identity-/.test(l)), 'identity migrated first');
  assert.ok(c.some((l) => /migrate-learning-/.test(l)));
  assert.ok(!c.some((l) => /migrate-game-/.test(l)), 'no later module after the failure');
  assert.ok(!c.some((l) => /stack deploy/.test(l)), 'no rollout after a failed migration');
  assert.match(r.stdout, /"action":"migrate_failed".*"module":"learning"/);
  assert.match(r.stderr, /rollout stopped/);
});

test('F130-AC-02: migrate jobs use only <module>_migrator_dsn, in module order, then stack deploy', { skip }, () => {
  const { stubs, env, calls } = rig();
  const r = run('scripts/deploy/deploy-service.sh', ['--env=staging', '--stack=app', '--version=v1.4.0'], { env, stubs });
  assert.equal(r.code, 0, r.stderr);
  const c = calls(r);
  const creates = c.filter((l) => /service create/.test(l));
  assert.deepEqual(creates.map((l) => l.match(/migrate-([a-z]+)-/)[1]), ['identity', 'learning', 'game', 'competition', 'chat']);
  for (const l of creates) {
    const secretsUsed = [...l.matchAll(/--secret source=([a-z_]+)_v\d+/g)].map((m) => m[1]);
    const mod = l.match(/migrate-([a-z]+)-/)[1];
    assert.deepEqual(secretsUsed, [`${mod}_migrator_dsn`]);
    assert.match(l, /--restart-condition none .*--network data/);
    assert.match(l, new RegExp(`ghcr.io/tanguyen2020/eduworld/migrate:v1.4.0 --module=${mod}`));
  }
  const deployIdx = c.findIndex((l) => /stack deploy .*app.stack.yml eduworld/.test(l));
  assert.ok(deployIdx > c.lastIndexOf(creates.at(-1)), 'stack deploy after every migrate job');
});

test('F130-AC-02: a migrate job over the timeout stops the rollout', { skip }, () => {
  const { stubs, env, calls } = rig({ STUB_HANG_MIGRATE: 'identity', EW_MIGRATE_TIMEOUT_SECONDS: '1' });
  const r = run('scripts/deploy/run-migrate-job.sh', ['--env=staging', '--version=v1.4.0', '--timeout=1'], { env, stubs });
  assert.equal(r.code, 1);
  assert.match(r.stderr, /did not complete within 1s/);
  assert.ok(!calls(r).some((l) => /migrate-learning-/.test(l)));
});

test('edge case: lost manager quorum → no deploy at all', { skip }, () => {
  const { stubs, env, calls } = rig({ STUB_MANAGERS: 'Leader\\nUnreachable\\nUnreachable' });
  const r = run('scripts/deploy/deploy-service.sh', ['--env=staging', '--stack=app', '--version=v1.4.0'], { env, stubs });
  assert.equal(r.code, 1);
  assert.match(r.stderr, /quorum lost/);
  assert.ok(!calls(r).some((l) => /service create|stack deploy/.test(l)));
});

test('immutable tags only: latest or a branch name is refused', { skip }, () => {
  const { stubs, env } = rig();
  for (const v of ['latest', 'main', 'v1.4']) {
    const r = run('scripts/deploy/deploy-service.sh', ['--env=staging', '--stack=app', `--version=${v}`], { env, stubs });
    assert.equal(r.code, 1, v);
    assert.match(r.stderr, /immutable vX\.Y\.Z/);
  }
});

test('F130-AC-03: --service updates that one service only (no stack deploy, no other service)', { skip }, () => {
  const { stubs, env, calls } = rig();
  const r = run('scripts/deploy/deploy-service.sh', ['--env=staging', '--stack=app', '--version=v1.4.1', '--service=realtime'], { env, stubs });
  assert.equal(r.code, 0, r.stderr);
  const updates = calls(r).filter((l) => /service update|stack deploy|service rollback/.test(l));
  assert.deepEqual(updates, ['docker service update --detach=false --with-registry-auth --image ghcr.io/tanguyen2020/eduworld/realtime:v1.4.1 eduworld_realtime']);
});

test('F130-AC-03: rollback.sh rolls back exactly one service and audits it; unknown service refused', { skip }, () => {
  const { stubs, env, calls } = rig();
  const r = run('scripts/deploy/rollback.sh', ['--env=production', '--service=core-api', '--reason=drill'], { env, stubs });
  assert.equal(r.code, 0, r.stderr);
  assert.deepEqual(calls(r).filter((l) => /service (rollback|update)/.test(l)), ['docker service rollback --detach=false eduworld_core-api']);
  assert.match(r.stdout, /"action":"rollback_completed".*"service":"core-api"/);
  const bad = run('scripts/deploy/rollback.sh', ['--env=production', '--service=postgres'], { env, stubs });
  assert.equal(bad.code, 1);
  assert.match(bad.stderr, /unknown backend Swarm service/);
});

test('F130-AC-05: production refuses a digest not deployed on staging', { skip }, () => {
  const { stubs, env, calls } = rig({ STUB_DIGEST_CORE_API: 'sha256:rebuilt', EW_NOW_EPOCH: String(Date.UTC(2026, 10, 27, 16, 0) / 1000) });
  const expect = path.join(tmpdir(), 'staging-digests.txt');
  fs.writeFileSync(expect, ['core-api sha256:staging-core-api', 'realtime sha256:staging-realtime', 'worker sha256:staging-worker', 'migrate sha256:staging-migrate'].join('\n'));
  const r = run('scripts/deploy/deploy-service.sh', ['--env=production', '--stack=app', '--version=v1.4.0', `--expect-digests=${expect}`], { env, stubs });
  assert.equal(r.code, 1);
  assert.match(r.stderr, /digest of core-api:v1\.4\.0 \(sha256:rebuilt\) is not the one deployed on staging/);
  assert.ok(!calls(r).some((l) => /service create|stack deploy/.test(l)));
  const none = run('scripts/deploy/deploy-service.sh', ['--env=production', '--stack=app', '--version=v1.4.0'], { env, stubs });
  assert.match(none.stderr, /needs --expect-digests/);
});

test('F130-AC-05: production deploys only 22:00–06:00 ICT or at the weekend, unless --hotfix', { skip }, () => {
  const expect = path.join(tmpdir(), 'staging-digests.txt');
  fs.writeFileSync(expect, ['core-api', 'realtime', 'worker', 'migrate'].map((i) => `${i} sha256:staging-${i}`).join('\n'));
  // Thursday 2026-11-26 10:00 ICT = 03:00 UTC: school hours
  const school = String(Date.UTC(2026, 10, 26, 3, 0) / 1000);
  const { stubs, env, calls } = rig({ EW_NOW_EPOCH: school });
  const r = run('scripts/deploy/deploy-service.sh', ['--env=production', '--stack=app', '--version=v1.4.0', `--expect-digests=${expect}`], { env, stubs });
  assert.equal(r.code, 1);
  assert.match(r.stderr, /maintenance_window/);
  assert.ok(!calls(r).some((l) => /stack deploy/.test(l)));
  const hot = run('scripts/deploy/deploy-service.sh', ['--env=production', '--stack=app', '--version=v1.4.0', `--expect-digests=${expect}`, '--hotfix'], { env, stubs });
  assert.equal(hot.code, 0, hot.stderr);
  // Thursday 23:00 ICT = 16:00 UTC: inside the window
  const night = rig({ EW_NOW_EPOCH: String(Date.UTC(2026, 10, 26, 16, 0) / 1000) });
  const ok = run('scripts/deploy/deploy-service.sh', ['--env=production', '--stack=app', '--version=v1.4.0', `--expect-digests=${expect}`, `--record-digests=${expect}.out`], { env: night.env, stubs: night.stubs });
  assert.equal(ok.code, 0, ok.stderr);
  assert.match(fs.readFileSync(`${expect}.out`, 'utf8'), /^core-api sha256:staging-core-api$/m);
});

test('stack deploy exports SECRET_VER_* and CFG_* for every reference (infra stack)', { skip }, () => {
  const { stubs, state, env } = rig();
  fs.appendFileSync(path.join(state, 'secrets'), 'apisix_admin_key_v2\napisix_admin_key_v10\n');
  // docker-env: dumps the environment `stack deploy` receives, then behaves like the docker stub
  stub(stubs, 'docker-env', `if [ "$1 $2" = "stack deploy" ]; then env | grep -E '^(SECRET_VER|CFG)_' | sort > "${state.replace(/\\/g, '/')}/deploy.env"; exit 0; fi\nexec bash "${ROOT.replace(/\\/g, '/')}/tests/ops/docker-stub.sh" "$@"`);
  const r2 = run('scripts/deploy/deploy-service.sh', ['--env=production', '--stack=infra'], { env: { ...env, DOCKER_BIN: path.join(stubs, 'docker-env') }, stubs });
  assert.equal(r2.code, 0, r2.stderr);
  const dumped = fs.readFileSync(path.join(state, 'deploy.env'), 'utf8');
  assert.match(dumped, /^SECRET_VER_APISIX_ADMIN_KEY=10$/m, 'highest version wins (numeric)');
  assert.match(dumped, /^CFG_APISIX_YAML=[0-9a-f]{12}$/m);
  assert.match(dumped, /^CFG_SENTINEL_CONF_TPL=[0-9a-f]{12}$/m);
});

function canaryRig({ alerts, rulesLoaded = true, routes = '{"list":[{"value":{"id":"r1","upstream_id":"core-api"}},{"value":{"id":"r2","upstream_id":"realtime"}}]}' }) {
  const r = rig({ EW_APISIX_ADMIN_URL: 'http://apisix-admin:9180', EW_PROMETHEUS_URL: 'http://prometheus:9090' });
  const key = path.join(r.state, 'admin_key');
  fs.writeFileSync(key, 'test-admin-key');
  r.env.EW_APISIX_ADMIN_KEY_FILE = key;
  const rules = rulesLoaded ? '{"data":{"groups":[{"rules":[{"name":"ModuleHighErrorRate"},{"name":"APILatencyP95High"}]}]}}' : '{"data":{"groups":[]}}';
  const result = alerts ? '[{"metric":{"alertname":"ModuleHighErrorRate","module":"learning","service":"core-api-canary"}}]' : '[]';
  stub(r.stubs, 'curl', `case "$*" in
  *"/api/v1/rules"*) echo '${rules}' ;;
  *"/api/v1/query"*) echo '{"data":{"result":${result}}}' ;;
  *"/apisix/admin/routes"*"-X PATCH"*|*"-X PATCH"*) echo '{}' ;;
  *"/apisix/admin/upstreams/"*) echo '{}' ;;
  *"/apisix/admin/routes"*) echo '${routes}' ;;
  *) echo "curl-stub unhandled $*" >&2; exit 7 ;;
esac`);
  return r;
}

test('F130-AC-04: canary aborts on ModuleHighErrorRate of core-api-canary: split removed, canary removed, core-api unchanged', { skip }, () => {
  const { stubs, env, calls } = canaryRig({ alerts: true });
  const r = run('scripts/deploy/canary.sh', ['--env=production', '--version=v1.5.0', '--duration=2', '--interval=1'], { env, stubs });
  assert.equal(r.code, 1);
  const c = calls(r);
  assert.ok(c.some((l) => /docker service create .*--name eduworld_core-api-canary --replicas 2 .*core-api:v1\.5\.0/.test(l)));
  assert.ok(c.some((l) => /curl .*-X PATCH .*routes\/r1 .*"weight":10/.test(l)), 'traffic-split 10 % on core-api routes');
  assert.ok(!c.some((l) => /routes\/r2/.test(l)), 'realtime routes untouched');
  assert.ok(c.some((l) => /curl .*-X PATCH .*routes\/r1 .*traffic-split":null/.test(l)), 'split removed');
  assert.ok(c.some((l) => /docker service rm eduworld_core-api-canary/.test(l)));
  assert.ok(!c.some((l) => /service update .*eduworld_core-api$/.test(l)), 'core-api not updated');
  assert.match(r.stdout, /"action":"canary_aborted".*ModuleHighErrorRate\/learning/);
});

test('F130-AC-04: canary without alerts promotes core-api then removes the canary', { skip }, () => {
  const { stubs, env, calls } = canaryRig({ alerts: false });
  const r = run('scripts/deploy/canary.sh', ['--env=production', '--version=v1.5.0', '--duration=1', '--interval=1'], { env, stubs });
  assert.equal(r.code, 0, r.stderr);
  const c = calls(r);
  const upd = c.findIndex((l) => /service update .*core-api:v1\.5\.0 eduworld_core-api$/.test(l));
  const rm = c.findIndex((l) => /service rm eduworld_core-api-canary/.test(l));
  assert.ok(upd >= 0 && rm > upd);
});

test('F130-AC-04: no alert rule loaded → refuse a blind canary before creating anything', { skip }, () => {
  const { stubs, env, calls } = canaryRig({ alerts: false, rulesLoaded: false });
  const r = run('scripts/deploy/canary.sh', ['--env=production', '--version=v1.5.0', '--duration=1', '--interval=1'], { env, stubs });
  assert.equal(r.code, 1);
  assert.match(r.stderr, /alert rule ModuleHighErrorRate is not loaded/);
  assert.ok(!calls(r).some((l) => /service create/.test(l)));
});

test('F130-AC-10: secrets-sync creates _v<N+1> only when the value changes and keeps _v<N>', { skip }, () => {
  const { stubs, state, env, calls } = rig();
  fs.writeFileSync(path.join(state, 'secrets'), 'learning_db_dsn_v1\n');
  const input = path.join(tmpdir(), 'staging.json');
  fs.writeFileSync(input, JSON.stringify({ learning_db_dsn: 'postgres://learning_app:one@pg/eduworld_learning', game_db_dsn: 'g1' }));
  // v1 currently holds another value: label hash differs
  fs.writeFileSync(path.join(state, 'labels', 'learning_db_dsn_v1'), 'old-hash');
  let r = run('scripts/deploy/secrets-sync.sh', ['--env=staging', input], { env, stubs });
  assert.equal(r.code, 0, r.stderr);
  assert.ok(calls(r).some((l) => /secret create .*ew\.secret=learning_db_dsn .*learning_db_dsn_v2 -/.test(l)));
  assert.ok(calls(r).some((l) => /secret create .*game_db_dsn_v1 -/.test(l)));
  assert.equal(fs.readFileSync(path.join(state, 'values', 'learning_db_dsn_v2'), 'utf8'), 'postgres://learning_app:one@pg/eduworld_learning');
  assert.ok(!r.stdout.includes('one@pg') && !r.stderr.includes('one@pg') && !r.log.includes('one@pg'), 'value never in output/argv');
  assert.match(r.stdout, /"secret":"learning_db_dsn","version":"2","previous_kept":"learning_db_dsn_v1"/);
  // unchanged on a second run
  r = run('scripts/deploy/secrets-sync.sh', ['--env=staging', input], { env, stubs });
  assert.equal(r.code, 0);
  assert.ok(!calls(r).some((l) => /secret create/.test(l)));
  assert.match(r.stderr, /0 created, 2 unchanged/);
  assert.ok(!r.log.includes('secret rm'));
});

test('F130-AC-10: Keycloak client secret change needs --keycloak-rotated (old and new valid during rollout)', { skip }, () => {
  const { stubs, state, env, calls } = rig();
  fs.writeFileSync(path.join(state, 'secrets'), 'kc_game_backend_v1\n');
  fs.writeFileSync(path.join(state, 'labels', 'kc_game_backend_v1'), 'previous');
  const input = path.join(tmpdir(), 'kc.json');
  fs.writeFileSync(input, JSON.stringify({ kc_game_backend: 'new-client-secret' }));
  const r = run('scripts/deploy/secrets-sync.sh', ['--env=production', input], { env, stubs });
  assert.equal(r.code, 1);
  assert.match(r.stderr, /--keycloak-rotated/);
  assert.ok(!calls(r).some((l) => /secret create/.test(l)));
  const ok = run('scripts/deploy/secrets-sync.sh', ['--env=production', '--keycloak-rotated', input], { env, stubs });
  assert.equal(ok.code, 0, ok.stderr);
  assert.ok(calls(ok).some((l) => /secret create .*kc_game_backend_v2/.test(l)));
});

test('F130-AC-07: networks.sh creates app/data/admin encrypted and edge plain', { skip }, () => {
  const { stubs, env, calls } = rig();
  const r = run('deploy/swarm/networks.sh', [], { env, stubs });
  assert.equal(r.code, 0, r.stderr);
  const c = calls(r).filter((l) => /network create/.test(l));
  assert.deepEqual(c, [
    'docker network create --driver overlay --scope swarm edge',
    'docker network create --driver overlay --scope swarm --opt encrypted app',
    'docker network create --driver overlay --scope swarm --opt encrypted data',
    'docker network create --driver overlay --scope swarm --opt encrypted admin',
  ]);
});

test('run-ops-job copies networks/env/secrets of a service and adds the restore credential only on request', { skip }, () => {
  const { stubs, state, env, calls } = rig();
  fs.appendFileSync(path.join(state, 'secrets'), 'backup_restore_access_key_v4\nbackup_restore_secret_key_v4\n');
  const r = run('scripts/ops/run-ops-job.sh', ['--env=staging', '--like=core-api', '--restore', '--', 'restore_test.sh', '--mode=full'], { env, stubs });
  assert.equal(r.code, 0, r.stderr);
  const create = calls(r).find((l) => /service create/.test(l));
  assert.match(create, /--mode replicated-job --restart-condition none/);
  assert.match(create, /--network edge --network data/);
  assert.match(create, /--secret source=learning_db_dsn_v3,target=learning_db_dsn/);
  assert.match(create, /--secret source=backup_restore_access_key_v4,target=\/run\/restore-credentials\/backup_restore_access_key/);
  assert.match(create, /--constraint node\.labels\.role == app/);
  const plain = run('scripts/ops/run-ops-job.sh', ['--env=staging', '--like=core-api', '--', 'true'], { env, stubs });
  assert.ok(!calls(plain).filter((l) => /service create/.test(l)).at(-1).includes('backup_restore'));
});

test("secret rotation: production --redeploy of the running version needs no digest file, another version is refused", { skip }, () => {
  const night = String(Date.UTC(2026, 10, 26, 16, 0) / 1000);
  const { stubs, env, calls } = rig({ EW_NOW_EPOCH: night, STUB_RUNNING_IMAGE: "ghcr.io/tanguyen2020/eduworld/core-api:v1.4.0@sha256:abc" });
  const ok = run("scripts/deploy/deploy-service.sh", ["--env=production", "--stack=app", "--version=v1.4.0", "--skip-migrate", "--redeploy"], { env, stubs });
  assert.equal(ok.code, 0, ok.stderr);
  assert.ok(calls(ok).some((l) => /stack deploy/.test(l)));
  assert.ok(!calls(ok).some((l) => /migrate-/.test(l)), "no migration on a redeploy");
  const bad = run("scripts/deploy/deploy-service.sh", ["--env=production", "--stack=app", "--version=v1.5.0", "--skip-migrate", "--redeploy"], { env, stubs });
  assert.equal(bad.code, 1);
  assert.match(bad.stderr, /only redeploys the running version \(v1\.4\.0\)/);
});
