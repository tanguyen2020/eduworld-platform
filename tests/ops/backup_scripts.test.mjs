// F130-AC-13, AC-14, AC-16, AC-17, AC-19 (script logic with stubbed pgbackrest/rclone/curl/etcdctl/nats — TEST DOUBLES,
// R17). The same scripts run for real against throwaway containers in tests/ops/integration/run.sh.
// Needs bash + jq + sha256sum (CI ubuntu, or the ops test image); skipped elsewhere.
import test from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { tmpdir, stub, run, hasCmd } from './helpers.mjs';

const skip = hasCmd('jq') ? false : 'jq not installed (runs in CI / ops test image)';

function secretsDir(names) {
  const d = tmpdir('ew-f130-secrets-');
  for (const n of names) fs.writeFileSync(path.join(d, n), `test-only-${n}\n`);
  return d;
}
const repoSecrets = ['backup_repo_write_access_key', 'backup_repo_write_secret_key', 'backup_repo_cipher_pass'];
const metric = (dir, file) => fs.readFileSync(path.join(dir, file), 'utf8');

test('F130-AC-13: pg_backup.sh writes the four metrics per target/type, check runs after the backup', { skip }, () => {
  const stubs = tmpdir(); const metrics = tmpdir();
  stub(stubs, 'pgbackrest', 'exit 0');
  const env = { METRIC_DIR: metrics, EW_SECRETS_DIR: secretsDir(repoSecrets) };
  const r = run('scripts/backup/pg_backup.sh', ['diff'], { env, stubs });
  assert.equal(r.code, 0, r.stderr);
  assert.deepEqual(r.log.trim().split('\n'), ['pgbackrest --stanza=eduworld --type=diff backup', 'pgbackrest --stanza=eduworld check']);
  const m = metric(metrics, 'eduworld_backup_postgres_diff.prom');
  for (const name of ['last_run_timestamp', 'last_status', 'duration_seconds', 'last_success_timestamp']) {
    assert.match(m, new RegExp(`^eduworld_backup_${name}\\{target="postgres",type="diff"\\} \\d+$`, 'm'), name);
  }
  assert.match(m, /^eduworld_backup_last_status\{target="postgres",type="diff"\} 1$/m);
  assert.match(metric(metrics, 'eduworld_backup_postgres_check.prom'), /last_status\{target="postgres",type="check"\} 1/);
  assert.ok(!r.stderr.includes('test-only-'), 'no secret in logs');
});

test('F130-AC-13: a failed backup writes status 0, keeps the previous last_success and exits 1', { skip }, () => {
  const stubs = tmpdir(); const metrics = tmpdir();
  const env = { METRIC_DIR: metrics, EW_SECRETS_DIR: secretsDir(repoSecrets) };
  stub(stubs, 'pgbackrest', 'exit 0');
  assert.equal(run('scripts/backup/pg_backup.sh', ['full'], { env, stubs }).code, 0);
  const before = metric(metrics, 'eduworld_backup_postgres_full.prom').match(/last_success_timestamp\{[^}]+\} (\d+)/)[1];
  stub(stubs, 'pgbackrest', '[ "$*" = "--stanza=eduworld check" ] && exit 0; exit 37');
  const r = run('scripts/backup/pg_backup.sh', ['full'], { env, stubs });
  assert.equal(r.code, 1);
  const m = metric(metrics, 'eduworld_backup_postgres_full.prom');
  assert.match(m, /last_status\{target="postgres",type="full"\} 0/);
  assert.match(m, new RegExp(`last_success_timestamp\\{target="postgres",type="full"\\} ${before}`));
});

test('F130-AC-13: WAL archiving stuck → pgbackrest check fails → check status 0, job exits 1', { skip }, () => {
  const stubs = tmpdir(); const metrics = tmpdir();
  stub(stubs, 'pgbackrest', 'case "$*" in *check*) echo "archive_command failed" >&2; exit 82 ;; esac; exit 0');
  const r = run('scripts/backup/pg_check.sh', [], { env: { METRIC_DIR: metrics, EW_SECRETS_DIR: secretsDir(repoSecrets) }, stubs });
  assert.equal(r.code, 1);
  assert.match(metric(metrics, 'eduworld_backup_postgres_check.prom'), /last_status\{target="postgres",type="check"\} 0/);
});

test('backup scripts refuse to start without their secret files (name only in the error)', { skip }, () => {
  const stubs = tmpdir();
  stub(stubs, 'pgbackrest', 'exit 0');
  const r = run('scripts/backup/pg_backup.sh', ['full'], { env: { METRIC_DIR: tmpdir(), EW_SECRETS_DIR: tmpdir() }, stubs });
  assert.equal(r.code, 1);
  assert.match(r.stderr, /PGBACKREST_REPO1_S3_KEY: secret file missing or empty/);
  assert.equal(r.log.trim(), '', 'pgbackrest never called');
});

function rcloneRig({ copied = [], cryptcheckRc = 0, catOverride = {} } = {}) {
  const stubs = tmpdir(); const work = tmpdir();
  const objects = path.join(work, 'objects.json');
  fs.writeFileSync(objects, JSON.stringify(catOverride));
  const copiedJson = copied.map((k) => JSON.stringify({ level: 'info', msg: 'Copied (new)', object: k })).join('\\n');
  stub(stubs, 'rclone', `
case "$1" in
  obscure) cat >/dev/null; echo obscured ;;
  sync) for a in "$@"; do case "$prev" in --log-file) printf '%b\\n' '${copiedJson}' > "$a" ;; esac; prev="$a"; done ;;
  cryptcheck) exit ${cryptcheckRc} ;;
  cat) k="\${2#*current/}"; k="\${k#*/}"; v=$(jq -r --arg k "$k" '.[$k] // empty' "${objects.replace(/\\/g, '/')}"); if [ -n "$v" ]; then printf '%s' "$v"; else printf 'content'; fi ;;
  rcat) cat > /dev/null ;;
  *) : ;;
esac`);
  return { stubs, work };
}
const offsiteSecrets = ['backup_source_access_key', 'backup_source_secret_key', 'backup_offsite_access_key', 'backup_offsite_secret_key', 'backup_offsite_crypt_password'];
const osEnv = (metrics) => ({ METRIC_DIR: metrics, EW_SECRETS_DIR: secretsDir(offsiteSecrets), EW_BACKUP_SOURCE_URL: 'http://src:8333',
  EW_BACKUP_OFFSITE_URL: 'http://off:9000', EW_BACKUP_OFFSITE_BUCKET: 'offsite' });

test('F130-AC-14: object_storage_backup.sh never copies ew-media-quarantine and verifies copied originals by their key', { skip }, () => {
  const sha = 'ed7002b439e9ac845f22357d822bac1444730fbdb6016d3ec9432297b9ec9f73'; // sha256("content")
  const { stubs } = rcloneRig({ copied: [`media/${sha}/original.png`, `media/${sha}/thumb.webp`] });
  const metrics = tmpdir();
  const r = run('scripts/backup/object_storage_backup.sh', [], { env: { ...osEnv(metrics), EW_BACKUP_BUCKETS: 'ew-media-public ew-media-quarantine' }, stubs });
  assert.equal(r.code, 0, r.stderr);
  assert.ok(!r.log.includes('ew-media-quarantine'), 'quarantine never synced');
  assert.match(r.stderr, /quarantine is never backed up/);
  assert.match(r.log, /rclone sync src:ew-media-public offcrypt:current\/ew-media-public --backup-dir offcrypt:history\/\d{4}-\d\d-\d\d\/ew-media-public/);
  assert.match(r.log, /rclone cryptcheck src:ew-media-public offcrypt:current\/ew-media-public --one-way/);
  assert.match(r.log, /rclone rcat off:offsite\/object-storage\/manifests\/\d{4}-\d\d-\d\d\/ew-media-public\.sha256/);
  assert.ok(!/rclone (delete|purge|deletefile|rmdirs)/.test(r.log), 'nothing deleted');
  const m = metric(metrics, 'eduworld_backup_object_storage_offsite.prom');
  assert.match(m, /last_status\{target="object_storage",type="offsite"\} 1/);
  assert.match(m, /eduworld_backup_objects_copied\{target="object_storage",type="offsite",bucket="ew-media-public"\} 2/);
});

test('F130-AC-14: content that does not match its media/<sha256> key fails the run (status 0)', { skip }, () => {
  const sha = 'a'.repeat(64);
  const { stubs } = rcloneRig({ copied: [`media/${sha}/original.png`] });
  const metrics = tmpdir();
  const r = run('scripts/backup/object_storage_backup.sh', [], { env: { ...osEnv(metrics), EW_BACKUP_BUCKETS: 'ew-media-private' }, stubs });
  assert.equal(r.code, 1);
  assert.match(r.stderr, /content hash differs from its key/);
  assert.match(metric(metrics, 'eduworld_backup_object_storage_offsite.prom'), /last_status\{target="object_storage",type="offsite"\} 0/);
});

test('F130-AC-14: a checksum mismatch between source and off-site copy fails the run', { skip }, () => {
  const { stubs } = rcloneRig({ cryptcheckRc: 1 });
  const metrics = tmpdir();
  const r = run('scripts/backup/object_storage_backup.sh', [], { env: { ...osEnv(metrics), EW_BACKUP_BUCKETS: 'ew-game-bundles' }, stubs });
  assert.equal(r.code, 1);
  assert.match(r.stderr, /checksum mismatch/);
});

test('F130-AC-14: replication lag probe exports the lag and fails beyond the RPO', { skip }, () => {
  const stubs = tmpdir(); const metrics = tmpdir(); const st = tmpdir();
  const fwd = st.replace(/\\/g, '/');
  // replica shows the canary on the 2nd read
  stub(stubs, 'rclone', `case "$1" in
  obscure) echo x ;;
  rcat) cat > "${fwd}/canary" ;;
  cat) n=$(cat "${fwd}/n" 2>/dev/null || echo 0); n=$((n+1)); echo $n > "${fwd}/n"; [ $n -ge 2 ] && cat "${fwd}/canary" ;;
esac`);
  const env = { METRIC_DIR: metrics, EW_SECRETS_DIR: secretsDir(['backup_source_access_key', 'backup_source_secret_key']), EW_BACKUP_SOURCE_URL: 'http://src',
    EW_REPLICA_URL: 'http://replica', EW_REPLICATION_POLL_SECONDS: '1', EW_RPO_SECONDS: '30' };
  let r = run('scripts/backup/object_storage_replication_check.sh', [], { env, stubs });
  assert.equal(r.code, 0, r.stderr);
  assert.match(metric(metrics, 'eduworld_backup_object_storage_replication.prom'), /eduworld_object_storage_replication_lag_seconds\{bucket="ew-media-private"\} [0-9]+/);
  stub(stubs, 'rclone', `case "$1" in obscure) echo x ;; rcat) cat >/dev/null ;; cat) exit 3 ;; esac`);
  r = run('scripts/backup/object_storage_replication_check.sh', [], { env: { ...env, EW_RPO_SECONDS: '2' }, stubs });
  assert.equal(r.code, 1);
  assert.match(r.stderr, /replication lag exceeds the RPO/);
  assert.match(metric(metrics, 'eduworld_backup_object_storage_replication.prom'), /last_status\{target="object_storage",type="replication"\} 0/);
});

test('F130-AC-14: Keycloak realm export with an unmasked secret is refused and not stored', { skip }, () => {
  const stubs = tmpdir(); const metrics = tmpdir();
  const sec = secretsDir(['backup_keycloak_export_secret', ...repoSecrets]);
  const env = { METRIC_DIR: metrics, EW_SECRETS_DIR: sec, EW_BACKUP_REPO_URL: 'http://repo', PGBACKREST_REPO1_S3_REGION: 'r', PGBACKREST_REPO1_S3_BUCKET: 'b' };
  stub(stubs, 'rclone', 'case "$1" in obscure) echo x ;; *) : ;; esac');
  const realm = (secret) => `{"realm":"eduworld","clients":[{"clientId":"eduworld-portal","secret":"${secret}"}]}`;
  stub(stubs, 'curl', `case "$*" in *token*) echo '{"access_token":"t"}' ;; *partial-export*) for a in "$@"; do [ "$p" = -o ] && echo '${realm('leaked-value')}' > "$a"; p="$a"; done ;; esac`);
  let r = run('scripts/backup/keycloak_export.sh', [], { env, stubs });
  assert.equal(r.code, 1);
  assert.match(r.stderr, /unmasked secret-like fields: not stored/);
  assert.ok(!r.log.includes('rclone copyto'));
  stub(stubs, 'curl', `case "$*" in *token*) echo '{"access_token":"t"}' ;; *partial-export*) for a in "$@"; do [ "$p" = -o ] && echo '${realm('**********')}' > "$a"; p="$a"; done ;; esac`);
  r = run('scripts/backup/keycloak_export.sh', [], { env, stubs });
  assert.equal(r.code, 0, r.stderr);
  assert.match(r.log, /rclone copyto .*repocrypt:\d{4}-\d\d-\d\d\.json/);
  assert.match(metric(metrics, 'eduworld_backup_keycloak_export_realm.prom'), /last_status\{target="keycloak_export",type="realm"\} 1/);
  assert.ok(!r.log.includes('test-only-backup_keycloak_export_secret'), 'client secret never on argv');
});

test('F130-AC-14: etcd snapshot saved, checked and stored; metric target=etcd', { skip }, () => {
  const stubs = tmpdir(); const metrics = tmpdir();
  stub(stubs, 'etcdctl', 'case "$*" in *"snapshot save"*) for a in "$@"; do f="$a"; done; echo snap > "$f" ;; *"snapshot status"*) echo \'{"revision":7,"totalKey":3}\' ;; esac');
  stub(stubs, 'rclone', 'case "$1" in obscure) echo x ;; *) : ;; esac');
  const env = { METRIC_DIR: metrics, EW_SECRETS_DIR: secretsDir(['backup_etcd_password', ...repoSecrets]), EW_BACKUP_REPO_URL: 'http://repo', PGBACKREST_REPO1_S3_REGION: 'r', PGBACKREST_REPO1_S3_BUCKET: 'b' };
  const r = run('scripts/backup/etcd_snapshot.sh', [], { env, stubs });
  assert.equal(r.code, 0, r.stderr);
  assert.match(metric(metrics, 'eduworld_backup_etcd_snapshot.prom'), /last_status\{target="etcd",type="snapshot"\} 1/);
  assert.ok(!r.log.includes('test-only-backup_etcd_password'), 'etcd password never on argv');
});

test('F130-AC-16: restore_test.sh --dry-run writes a report with every mandatory field', { skip }, () => {
  const reports = tmpdir();
  const r = run('scripts/backup/restore_test.sh', ['--mode=module', '--db=eduworld_learning', '--actor=devops.an', '--purpose=quarterly module drill',
    '--pipeline=https://ci.example/run/1', `--report-dir=${reports}`, '--dry-run'], { env: { EW_BACKUP_ENV: 'staging', METRIC_DIR: tmpdir() } });
  assert.equal(r.code, 0, r.stderr);
  const files = fs.readdirSync(reports);
  assert.equal(files.length, 1);
  assert.match(files[0], /^\d{4}-\d{2}\.md$/);
  const md = fs.readFileSync(path.join(reports, files[0]), 'utf8');
  for (const field of ['Loại kiểm thử', 'Môi trường', 'Người thực hiện', 'Mục đích', 'Thời điểm đích T', 'Bắt đầu', 'Kết thúc', 'Kết quả đối soát', 'Vấn đề', 'Pipeline']) {
    assert.ok(md.includes(field), `missing field ${field}`);
  }
  assert.match(md, /\| Người thực hiện \| devops\.an \|/);
  assert.match(md, /\| Môi trường \| staging \|/);
  assert.match(md, /eduworld_learning/);
  // random T within the last 7 days, at least 10 minutes old
  const t = md.match(/\| Thời điểm đích T \| ([^|]+) \|/)[1].trim();
  const age = (Date.now() - Date.parse(t.replace(' ', 'T').replace('+00', 'Z'))) / 1000;
  assert.ok(age >= 600 && age <= 7 * 86400, `random target age ${age}s`);
  assert.match(r.stdout, /"audit":true,"action":"restore_finished","actor":"devops\.an".*"purpose":"quarterly module drill"/);
});

test('F130-AC-15: restore_test.sh requires actor and purpose and the restore credential (never the write one)', { skip }, () => {
  let r = run('scripts/backup/restore_test.sh', ['--mode=full', '--purpose=x'], { env: { METRIC_DIR: tmpdir() } });
  assert.equal(r.code, 1); assert.match(r.stderr, /--actor is required/);
  r = run('scripts/backup/restore_test.sh', ['--mode=full', '--actor=a', '--purpose=p', `--report-dir=${tmpdir()}`],
    { env: { METRIC_DIR: tmpdir(), EW_RESTORE_CREDENTIALS_DIR: tmpdir(), EW_SECRETS_DIR: secretsDir(repoSecrets), EW_RESTORE_WORK_DIR: tmpdir() } });
  assert.equal(r.code, 1);
  assert.match(r.stderr, /PGBACKREST_REPO1_S3_KEY: secret file missing or empty/);
});

test('F130-AC-17: reset-consumers recreates only <module>_* durables at T; T beyond stream retention → exit 3', { skip }, () => {
  const stubs = tmpdir(); const st = tmpdir(); const fwd = st.replace(/\\/g, '/');
  const first = new Date(Date.now() - 6 * 86400e3).toISOString();
  stub(stubs, 'nats', `case "$1 $2" in
  "stream info") echo '{"state":{"first_ts":"${first}"}}' ;;
  "consumer ls") [ "$3" = EVT_IDENTITY ] && printf 'learning_principal\\ngame_principal\\n' ;;
  "consumer info") echo '{"config":{"name":"'$4'","durable_name":"'$4'","deliver_policy":"all","ack_policy":"explicit","filter_subjects":["ew.evt.identity.>"]}}' ;;
  "consumer add") for a in "$@"; do [ "$p" = --config ] && cp "$a" "${fwd}/added.json"; p="$a"; done ;;
esac`);
  const since = new Date(Date.now() - 86400e3).toISOString().replace(/\.\d+Z$/, 'Z');
  let r = run('scripts/ops/reset-consumers.sh', ['--module=learning', `--since=${since}`, '--yes'], { stubs });
  assert.equal(r.code, 0, r.stderr);
  assert.match(r.log, /nats consumer rm EVT_IDENTITY learning_principal -f/);
  assert.ok(!r.log.includes('game_principal -f'), 'other modules untouched');
  const added = JSON.parse(fs.readFileSync(path.join(st, 'added.json'), 'utf8'));
  assert.equal(added.deliver_policy, 'by_start_time');
  assert.equal(added.opt_start_time, since);
  assert.equal(added.durable_name, 'learning_principal');
  const old = new Date(Date.now() - 9 * 86400e3).toISOString().replace(/\.\d+Z$/, 'Z');
  r = run('scripts/ops/reset-consumers.sh', ['--module=learning', `--since=${old}`, '--yes'], { stubs });
  assert.equal(r.code, 3);
  assert.match(r.stderr, /replay from the producer outbox/);
  r = run('scripts/ops/reset-consumers.sh', ['--module=learning', `--since=${since}`], { stubs });
  assert.equal(r.code, 1); assert.match(r.stderr, /without --yes/);
});

test('F130-AC-19: RB-04 rotate creates new key providers, disables the old ones, sets not-before', { skip }, () => {
  const stubs = tmpdir();
  stub(stubs, 'curl', `case "$*" in
  *protocol/openid-connect/token/introspect*) echo '{"active":false}' ;;
  *protocol/openid-connect/token*) echo '{"access_token":"t"}' ;;
  *"components?type="*) echo '[{"id":"k1","name":"rsa-generated","providerId":"rsa-generated","config":{"priority":["100"]}},{"id":"k2","name":"hmac-generated","providerId":"hmac-generated","config":{"priority":["100"]}}]' ;;
  *"/admin/realms/eduworld"*"-X"*|*"-X"*) echo '{}' ;;
  *"/admin/realms/eduworld") echo '{"id":"realm-id"}' ;;
  *) echo '{}' ;;
esac`);
  const tok = path.join(tmpdir(), 'old.jwt'); fs.writeFileSync(tok, 'old-token');
  const r = run('scripts/ops/rb04-keycloak-restore.sh', ['--phase=rotate', '--actor=devops.an', '--purpose=RB-04 drill', `--old-token-file=${tok}`, '--yes'],
    { env: { EW_SECRETS_DIR: secretsDir(['ops_kc_admin_secret']) }, stubs });
  assert.equal(r.code, 0, r.stderr);
  const posts = r.log.split('\n').filter((l) => /-X POST .*\/components /.test(l));
  assert.equal(posts.length, 3);
  assert.match(posts[0], /"priority":\["110"\]/);
  assert.equal(r.log.split('\n').filter((l) => /-X PUT .*\/components\/k[12] /.test(l)).length, 2);
  assert.match(r.log, /-X PUT http:\/\/keycloak:8080\/admin\/realms\/eduworld -d \{"notBefore": \d+\}/);
  assert.match(r.log, /push-revocation/);
  assert.ok(!r.log.includes('old-token') && !r.log.includes('test-only-ops_kc_admin_secret'), 'no token/secret on argv');
});

test('RB-01 plan lists the consistency steps of BACKUP_RECOVERY §3.5 (reset, projection rebuild, retention-reapply, reconcile)', () => {
  const r = run('scripts/ops/rb01-pitr-restore.sh', ['--db=eduworld_learning', '--target=2026-11-20 08:14:00+07']);
  assert.equal(r.code, 0, r.stderr);
  for (const s of ['reset-consumers.sh --module=learning', 'projection rebuild --module=learning', 'retention-reapply', 'worker reconcile --module=learning', '--phase=swap']) {
    assert.ok(r.stdout.includes(s), s);
  }
});
