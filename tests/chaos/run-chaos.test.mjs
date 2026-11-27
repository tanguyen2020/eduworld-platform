// Unit tests of the chaos runner (F132 T016, T021): envguard, recovery that always runs, failed marking, report fields,
// scenario schema. docker / curl / psql / sleep are TEST DOUBLES (R17, tests/ops/helpers.mjs): nothing reaches a real
// Swarm, Prometheus or database. The behaviour on staging is evidenced by the weekly chaos.yml run (pending environment).
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { stub, run, tmpdir } from '../ops/helpers.mjs';
import { load, validate, alertNames, compare } from './lib/scenario.mjs';

const SCRIPT = 'tests/chaos/run-chaos.sh';
const SCN = path.resolve('tests/chaos/scenarios');

function rig({ replicas = '2/2', prom = {}, psqlOut = '10|10|0|0', execFail = '' } = {}) {
  const dir = tmpdir('ew-f132-chaos-');
  const reports = path.join(dir, 'reports');
  // docker: Swarm calls of run-chaos.sh; `service ls` answers STUB_REPLICAS (e.g. 0/2 = never converges).
  stub(dir, 'docker', `
case "$1 $2" in
  "service inspect") if [[ "$*" == *Replicas* ]]; then echo 2; else echo '["EW_KC_ADMIN_BASE_URL=http://keycloak:8080/admin/realms/eduworld"]'; fi ;;
  "service ls") echo "\${STUB_REPLICAS:-2/2}" ;;
  "service ps") echo app-1 ;;
  "exec "*) if [ -n "\${STUB_EXEC_FAIL:-}" ] && [[ "$*" == *"\${STUB_EXEC_FAIL}"* ]]; then echo "exec failed" >&2; exit 1; fi; echo t ;;
  "inspect -f") echo true ;;
  *) : ;;
esac`);
  stub(dir, 'curl', `
q=""; for a in "$@"; do case "$a" in query=*) q="\${a#query=}" ;; esac; done
v="\${STUB_PROM_DEFAULT:-0}"
if [ -n "\${STUB_PROM_MAP:-}" ]; then
  while IFS=$'\\t' read -r k val; do [ -n "$k" ] && [[ "$q" == *"$k"* ]] && { v="$val"; break; }; done < "$STUB_PROM_MAP"
fi
printf '{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[0,"%s"]}]}}' "$v"`);
  stub(dir, 'psql', 'echo "${STUB_PSQL_OUT:-0|0|0|0}"');
  stub(dir, 'sleep', ':');
  const map = path.join(dir, 'prom.tsv');
  fs.writeFileSync(map, Object.entries(prom).map(([k, v]) => `${k}\t${v}`).join('\n') + '\n');
  const ledger = path.join(dir, 'ledger');
  fs.mkdirSync(ledger);
  const dsn = path.join(dir, 'dsn');
  fs.writeFileSync(dsn, 'postgres://reader@localhost/eduworld_chat');
  const env = {
    EW_ENV: 'staging', EW_PROMETHEUS_URL: 'http://prom.test', EW_CHAOS_REPORT_DIR: reports, STUB_REPLICAS: replicas,
    STUB_PROM_MAP: map, STUB_PSQL_OUT: psqlOut, STUB_EXEC_FAIL: execFail, EW_CHAOS_SETTLE: '1',
    EW_CHAOS_LEDGER_DIR: ledger, EW_CHAOS_RECONCILE_DSN_CHAT_FILE: dsn, EW_CHAOS_RECONCILE_DSN_LEARNING_FILE: dsn,
  };
  const report = () => {
    const files = fs.existsSync(reports) ? fs.readdirSync(reports) : [];
    return files.length ? fs.readFileSync(path.join(reports, files[0]), 'utf8') : '';
  };
  return { dir, env, ledger, report };
}

// Healthy worker-chat-down: REST 5xx ratio 0, persister backlog 50 during (> 0) and after (< 100), alerts seen.
const healthyProm = { ALERTS: 1, 'consumer=~': 50 };

test('production and unnamed environments are refused before any docker call (F132-AC-07)', () => {
  for (const e of ['production', 'prod', '']) {
    const r = rig();
    const out = run(SCRIPT, ['--scenario=worker-chat-down'], { env: { ...r.env, EW_ENV: e }, stubs: r.dir });
    assert.equal(out.code, 2, out.stderr);
    assert.match(out.stderr, /refused/);
    assert.doesNotMatch(out.log, /^docker /m);
  }
});

test('compose mode never targets the owner stack eduworld-*', () => {
  const r = rig();
  const out = run(SCRIPT, ['--scenario=worker-chat-down'], {
    env: { ...r.env, EW_ENV: 'local', EW_CHAOS_CONTAINER_PREFIX: 'eduworld-' }, stubs: r.dir,
  });
  assert.equal(out.code, 2);
  assert.match(out.stderr, /owner/);
});

test('worker-chat-down passes: scale 0, scale back, reconcile, report with every field (F132-AC-25, AC-14)', () => {
  const r = rig({ prom: healthyProm });
  fs.writeFileSync(path.join(r.ledger, 'worker-chat-down-chat.txt'), '11111111-1111-4111-8111-111111111111\n');
  const out = run(SCRIPT, ['--scenario=worker-chat-down'], { env: r.env, stubs: r.dir });
  assert.equal(out.code, 0, out.stderr);
  const down = out.log.indexOf('docker service scale -d eduworld_worker-chat=0');
  const up = out.log.indexOf('docker service scale -d eduworld_worker-chat=2');
  assert.ok(down >= 0 && up > down, out.log);
  const rep = r.report();
  for (const want of ['## worker-chat-down (WORKER-CHAT-DOWN) — passed', '| Acceptance | F132-AC-25 |', '| Load | 0.3x target_ccu',
    '`swarm_service_down` on `worker-chat` for 300s', 'Injected / recovered (UTC)', '| Recovery | clean',
    '| Reconciliation | db_messages=10 bot_acked=10 missing=0 duplicates=0 |', '| ServiceDown | yes | [runbook](../../11-operations/runbooks/ServiceDown.md) |',
    '| during | core-api REST keeps serving']) {
    assert.ok(rep.includes(want), `report lacks ${want}\n${rep}`);
  }
});

test('a scenario that does not recover: fault removed, recovery forced, scenario failed (T016)', () => {
  const r = rig({ replicas: '0/2', prom: healthyProm });
  fs.writeFileSync(path.join(r.ledger, 'worker-chat-down-chat.txt'), '11111111-1111-4111-8111-111111111111\n');
  const out = run(SCRIPT, ['--scenario=worker-chat-down'], { env: r.env, stubs: r.dir });
  assert.equal(out.code, 1);
  assert.match(out.log, /docker service scale -d eduworld_worker-chat=2/); // the injected fault is removed
  assert.match(out.log, /docker service update --detach --force eduworld_core-api/);
  const rep = r.report();
  assert.match(rep, /— failed/);
  assert.match(rep, /recovery needed: FAILED|recovery needed: FORCED/);
});

test('injection that breaks half-way still runs its recovery (module_db_down, compose throwaway stack)', () => {
  const r = rig({ execFail: 'pg_terminate_backend' });
  const out = run(SCRIPT, ['--scenario=learning-db-down'], {
    env: { ...r.env, EW_ENV: 'local', EW_CHAOS_PG_CONTAINER: 'ew-f132-chaos-pg' }, stubs: r.dir,
  });
  assert.equal(out.code, 1);
  assert.match(out.log, /ALLOW_CONNECTIONS false/);
  assert.match(out.log, /docker exec ew-f132-chaos-pg psql .*ALLOW_CONNECTIONS true/);
  assert.match(r.report(), /injection failed/);
});

test('compose mode refuses an eduworld-* PostgreSQL container', () => {
  const r = rig();
  const out = run(SCRIPT, ['--scenario=learning-db-down'], {
    env: { ...r.env, EW_ENV: 'local', EW_CHAOS_PG_CONTAINER: 'eduworld-postgres-1' }, stubs: r.dir,
  });
  assert.equal(out.code, 1);
  assert.doesNotMatch(out.log, /docker exec eduworld/);
});

test('a failed expectation or a missing reconciliation fails the scenario', () => {
  const r = rig({ prom: { ...healthyProm, 'status_class="5xx"': 0.2 } });
  const out = run(SCRIPT, ['--scenario=worker-chat-down'], { env: r.env, stubs: r.dir });
  assert.equal(out.code, 1);
  const rep = r.report();
  assert.match(rep, /\| FAIL \|/);
  assert.match(rep, /reconciliation not run/);
});

test('lost attempts fail the reconciliation (F132-AC-12: 0 lost)', () => {
  const r = rig({ prom: { ALERTS: 1 }, psqlOut: '99|100|1' });
  fs.writeFileSync(path.join(r.ledger, 'valkey-down-attempts.txt'), '22222222-2222-4222-8222-222222222222\n');
  const out = run(SCRIPT, ['--scenario=valkey-down'], { env: r.env, stubs: r.dir });
  assert.equal(out.code, 1);
  assert.match(r.report(), /reconciliation: db_submitted=99 bot_acked=100 missing=1/);
});

test('external-fault repoints core-api to Toxiproxy and restores the original value', () => {
  const r = rig({ prom: { ALERTS: 1, circuit_state: 2, duration_seconds_bucket: 0.1 } });
  const out = run(SCRIPT, ['--scenario=external-fault', '--target=keycloak-admin'], { env: r.env, stubs: r.dir });
  assert.equal(out.code, 0, out.stderr + r.report());
  assert.match(out.log, /--env-add EW_KC_ADMIN_BASE_URL=http:\/\/chaos-toxiproxy:18080\/admin\/realms\/eduworld/);
  assert.match(out.log, /--env-add EW_KC_ADMIN_BASE_URL=http:\/\/keycloak:8080\/admin\/realms\/eduworld/);
  assert.match(out.log, /docker service rm eduworld_chaos-toxiproxy/);
});

test('dry-run of all lists the scenarios, skips the ones needing --target and touches nothing', () => {
  const r = rig();
  const out = run(SCRIPT, ['--scenario=all', '--dry-run'], { env: r.env, stubs: r.dir });
  assert.equal(out.code, 0, out.stderr);
  assert.match(out.stderr, /dry-run: learning-db-down/);
  assert.match(out.stderr, /skip node-drain \(needs --target\)/);
  assert.doesNotMatch(out.log, /^docker /m);
});

test('every scenario is valid: fault enum, TC id, F131 alert names and runbooks exist (data-model §1)', () => {
  const names = alertNames();
  const files = fs.readdirSync(SCN).filter((f) => f.endsWith('.yaml'));
  assert.ok(files.length >= 11);
  for (const f of files) assert.deepEqual(validate(load(path.join(SCN, f), 'x'), { alertNames: names, file: f }), []);
  const bad = load(path.join(SCN, 'nats-down.yaml'));
  bad.fault.type = 'kill_everything';
  bad.alerts = [{ name: 'NoSuchAlert', runbook: 'docs/11-operations/runbooks/NoSuchAlert.md' }];
  const errs = validate(bad, { alertNames: names, file: 'nats-down.yaml' }).join('\n');
  assert.match(errs, /fault.type/);
  assert.match(errs, /NoSuchAlert is not an alert/);
});

test('compare never passes on NaN (no Prometheus answer is not a pass)', () => {
  assert.equal(compare('NaN', '<', 1), false);
  assert.equal(compare('0', '==', 0), true);
  assert.equal(compare('0.004', '<', 0.005), true);
});
