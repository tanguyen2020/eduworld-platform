// F130 T053 (OQ F090-2): Valkey ACL / NATS permission templates vs the Go code (scripts/deploy/check-acl-templates.mjs)
// and their rendering (scripts/deploy/render-acl.mjs). Each negative case mutates a copy of the real inputs, so a
// check that silently stops working fails here. Real servers: scripts/deploy/validate-acl.sh (valkey:8 ACL DRYRUN,
// nats-server -t + permission probes with nats-box).
import test from 'node:test';
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import { checkAll, checkValkey, checkNats, repoInputs, NATS_EXPECT, nCovers, nPrefix, vCovers, jwtEstimate } from '../../scripts/deploy/check-acl-templates.mjs';
import { renderValkeyAcl, valkeyInputs, expandUser, expandAll, credsDrift, credsClaims, verifyNatsCreds, renderNatsAccounts, currentPermissions } from '../../scripts/deploy/render-acl.mjs';

const base = repoInputs();
const clone = (o) => JSON.parse(JSON.stringify(o));
const withTemplate = (f) => ({ ...base, template: f(base.template) });
const withPerms = (f) => { const perms = clone(base.perms); f(perms); return { ...base, perms }; };
const has = (problems, re) => assert.ok(problems.some((p) => re.test(p)), `expected a problem matching ${re}, got:\n${problems.join('\n')}`);

test('the committed templates cover the Go code with least privilege (0 problems)', () => {
  assert.deepEqual(checkAll(base), []);
  assert.ok(base.code.filter((c) => c.kind === 'valkey').length > 50, 'Valkey literals found in the code');
  assert.ok(base.code.filter((c) => c.kind === 'nats').length > 30, 'NATS literals found in the code');
  for (const s of ['EVT_IDENTITY', 'DLQ_CHAT', 'CHAT_MESSAGES', 'IDENTITY_SIS_JOBS', 'GAME_RENDER_JOBS', 'EVT_REALTIME']) assert.ok(base.streamsInCode.has(s), s);
});

test('matching helpers: NATS subsumption/prefix, Valkey glob', () => {
  assert.ok(nCovers('ew.cmd.>', 'ew.cmd.chat.message.send'));
  assert.ok(!nCovers('ew.cmd.>', 'ew.cmd'));
  assert.ok(nCovers('ew.chat.*.msg.*', 'ew.chat.*.msg.*'));
  assert.ok(!nCovers('ew.chat.*.msg.*', 'ew.chat.>'));
  assert.ok(nPrefix('ew.chat.*.msg.*', 'ew.chat.'));
  assert.ok(!nPrefix('ew.evt.identity.>', 'ew.evt.game.'));
  assert.ok(vCovers('ew:rt:room:*', 'ew:rt:room:'));
  assert.ok(vCovers('ew:rt:rooms:game', 'ew:rt:rooms:game'));
  assert.ok(!vCovers('ew:rt:rooms:game', 'ew:rt:rooms:'));
});

test('Valkey: a key prefix used by realtime but not granted is reported', () => {
  has(checkValkey(withTemplate((t) => t.replace(' ~ew:gateway:*', ''))), /"ew:gateway:.*not covered for realtime/);
});

test('Valkey: a grant no code uses is reported (least privilege)', () => {
  has(checkValkey(withTemplate((t) => t.replace('user core-api on #{{sha256:core_valkey_password}} resetkeys resetchannels',
    'user core-api on #{{sha256:core_valkey_password}} resetkeys resetchannels ~ew:unused:*'))), /core-api: pattern ew:unused:\* is used by no code/);
});

test('Valkey: allkeys/~*/+@dangerous/KEYS/plaintext password/default on are refused', () => {
  const core = 'user core-api on #{{sha256:core_valkey_password}} resetkeys resetchannels';
  has(checkValkey(withTemplate((t) => t.replace(core, `${core} ~*`))), /forbidden rule ~\*/);
  has(checkValkey(withTemplate((t) => t.replace(core, `${core} allkeys`))), /forbidden rule allkeys/);
  has(checkValkey(withTemplate((t) => t.replace('-@dangerous -@admin\n\n# worker', '+@dangerous\n\n# worker'))), /forbidden rule \+@dangerous|-@dangerous must follow/);
  has(checkValkey(withTemplate((t) => t.replace('+incr +expire +ping', '+incr +expire +ping +keys'))), /may run KEYS/);
  has(checkValkey(withTemplate((t) => t.replace('+incr +expire +ping', '+incr +expire +ping +config|get'))), /\+config\|get is reserved to exporter/);
  has(checkValkey(withTemplate((t) => t.replace('#{{sha256:apisix_valkey_password}}', '>secret'))), /password must be #\{\{sha256/);
  has(checkValkey(withTemplate((t) => t.replace('user default off', 'user default on nopass'))), /default user must be off/);
  has(checkValkey(withTemplate((t) => t.replace('{{sha256:core_valkey_password}}', '{{sha256:not_a_key}}'))), /not_a_key is not a key/);
});

test('Valkey: application users must not reach the gateway counters; the gateway user must', () => {
  const core = 'user core-api on #{{sha256:core_valkey_password}} resetkeys resetchannels';
  has(checkValkey(withTemplate((t) => t.replace(core, `${core} ~ew:rl:*`))), /core-api must not reach the gateway counters|pattern ew:rl:\* is used by no code/);
  has(checkValkey({ ...base, gatewayUser: 'nobody' }), /no ACL user nobody/);
});

test('NATS: only realtime publishes the gateway commands, only core-api subscribes them', () => {
  has(checkNats(withPerms((p) => p.users.worker.publish.push('ew.cmd.>'))), /only realtime may publish ew\.cmd/);
  has(checkNats(withPerms((p) => p.users['core-api'].publish.push('ew.rt.presence.*'))), /only realtime may publish ew\.rt\.presence/);
  has(checkNats(withPerms((p) => p.users.realtime.subscribe.push('ew.cmd.>'))), /only core-api may subscribe|realtime subscribes nothing/);
  has(checkNats(withPerms((p) => p.users['core-api'].publish.push('ew.evt.identity.>'))), /only worker may publish ew\.evt\.identity/);
});

test('NATS: a subject used by the code but not granted is reported', () => {
  has(checkNats(withPerms((p) => { p.users['core-api'].subscribe = p.users['core-api'].subscribe.filter((s) => s !== 'ew.in.room.>'); })),
    /core-api must receive "ew\.in\.room\."/);
  // A literal of a domain module must be granted to core-api or worker (the binary is not known statically).
  has(checkNats(withPerms((p) => {
    for (const u of ['core-api', 'worker']) p.users[u].publish = p.users[u].publish.filter((s) => s !== 'ew.job.identity.sis.>');
    delete p.users.worker.jetstream.IDENTITY_SIS_JOBS;
  })), /"ew\.job\.identity\.sis\.[a-z_>]*".*granted to none of core-api, worker/);
});

test('NATS: a pkg/ literal must be classified; parseOnly takes no direction and grants nothing', () => {
  const extra = (file, lit) => ({ ...base, code: [...base.code, { file, lit, kind: 'nats' }] });
  has(checkNats(extra('pkg/unknown/thing.go', 'ew.evt.')), /pkg\/unknown\/thing\.go: "ew\.evt\." is not classified/);
  const saved = NATS_EXPECT['pkg/module/consumers.go'];
  try {
    NATS_EXPECT['pkg/module/consumers.go'] = { 'ew.evt.': { parseOnly: true, sub: ['worker'] } };
    has(checkNats(base), /"ew\.evt\." is parseOnly and must not list pub\/sub/);
    NATS_EXPECT['pkg/module/consumers.go'] = { 'ew.evt.': {} };
    has(checkNats(base), /"ew\.evt\." has neither a direction \(pub\/sub\) nor parseOnly/);
  } finally {
    NATS_EXPECT['pkg/module/consumers.go'] = saved;
  }
  assert.deepEqual(checkNats(base), []);
});

test('NATS: whole-space wildcards, raw $JS grants, unknown streams and unused grants are refused', () => {
  has(checkNats(withPerms((p) => p.users.worker.subscribe.push('>'))), /whole space >/);
  has(checkNats(withPerms((p) => p.users.worker.publish.push('$JS.API.>'))), /whole space \$JS\.API\.>|JetStream API only/);
  has(checkNats(withPerms((p) => { p.users.worker.jetstream.EVT_NOPE = ['manage']; })), /EVT_NOPE is not created by the code/);
  has(checkNats(withPerms((p) => p.users['core-api'].publish.push('ew.foo.bar'))), /grant ew\.foo\.bar is used by no code/);
  has(checkNats(withPerms((p) => { delete p.users.worker.jetstream.EVT_GAME; })), /stream EVT_GAME is managed by no user/);
});

test('NATS: the largest user JWT must fit max_control_line', () => {
  has(checkNats({ ...base, maxControlLine: 4096 }), /worker: JWT ~\d+ B needs max_control_line/);
  assert.ok(jwtEstimate(expandAll(base.perms).worker) > 6247, 'estimate above the measured worker JWT (6247 B)');
});

test('render: Valkey ACL holds only SHA-256 password hashes, refuses a missing secret', () => {
  const secrets = Object.fromEntries(valkeyInputs(base.template).map((k) => [k, k === 'valkey_masterauth' ? 'masterauth rep-pw' : `${k}-value`]));
  const out = renderValkeyAcl(base.template, secrets);
  assert.doesNotMatch(out, /\{\{|-value|rep-pw|^#/m);
  const sha = (v) => crypto.createHash('sha256').update(v).digest('hex');
  assert.match(out, new RegExp(`^user core-api on #${sha('core_valkey_password-value')} `, 'm'));
  assert.match(out, new RegExp(`^user replicator on #${sha('rep-pw')} `, 'm'), 'directive secret: password after "masterauth"');
  assert.throws(() => renderValkeyAcl(base.template, { ...secrets, worker_valkey_password: '' }), /worker_valkey_password is missing or empty/);
  assert.throws(() => renderValkeyAcl(base.template, { ...secrets, valkey_masterauth: 'nope' }), /no "masterauth <password>" line/);
});

const b64 = (o) => Buffer.from(JSON.stringify(o)).toString('base64url');
const creds = (claims) => `-----BEGIN NATS USER JWT-----\n${b64({ typ: 'JWT', alg: 'ed25519-nkey' })}.${b64(claims)}.sig\n------END NATS USER JWT------\n`;

test('render: NATS expansion, creds drift detection, accounts template', () => {
  const e = expandUser(base.perms.users.worker);
  assert.ok(e.pub.includes('$JS.API.CONSUMER.MSG.NEXT.EVT_GAME.>') && e.pub.includes('$JS.API.STREAM.CREATE.IDENTITY_SIS_JOBS'));
  assert.ok(!e.pub.includes('$JS.API.>'));
  const exp = expandAll(base.perms)['core-api'];
  const good = { name: 'core-api', nats: { pub: { allow: exp.pub }, sub: { allow: exp.sub }, resp: { max: 1 } } };
  assert.deepEqual(credsDrift('core-api', exp, credsClaims(creds(good))), []);
  assert.deepEqual(currentPermissions(good).length, new Set([...exp.pub, ...exp.sub]).size);
  const wider = clone(good); wider.nats.pub.allow.push('ew.evt.>');
  has(credsDrift('core-api', exp, wider), /publish permissions differ/);
  const noResp = clone(good); delete noResp.nats.resp;
  has(credsDrift('core-api', exp, noResp), /allow_responses differs/);
  has(verifyNatsCreds({ core_nats_creds: creds(wider) }, base.perms), /core-api: publish permissions differ/);
  has(verifyNatsCreds({ worker_nats_creds: 'not creds' }, base.perms), /no user JWT block/);
  assert.deepEqual(verifyNatsCreds({ learning_db_dsn: 'x' }, base.perms), [], 'no creds in the input → nothing to verify');
  const jwt = `${b64({ a: 1 })}.${b64({ b: 2 })}.c2ln`;
  const acc = `A${'B'.repeat(55)}`;
  const out = renderNatsAccounts('operator: {{operator_jwt}}\nresolver_preload: { {{account}}: {{account_jwt}}, {{system_account}}: {{system_account_jwt}} }\n',
    { operator_jwt: jwt, system_account: acc, system_account_jwt: jwt, account: acc, account_jwt: jwt });
  assert.doesNotMatch(out, /\{\{/);
  assert.throws(() => renderNatsAccounts('x', { operator_jwt: 'bad' }), /operator_jwt missing or malformed/);
});
