// Least-privilege check of the datastore access templates against the Go code (F130 T053, OQ F090-2; DEPLOYMENT §8,
// §8b; SECURITY_ARCHITECTURE §8.1; VALKEY_DESIGN §2):
//   - deploy/valkey/users.acl.template: every Valkey key prefix / Pub/Sub channel literal ("ew:…") of the Go code is
//     covered for EVERY binary that loads the code (core-api and worker load the five domain modules, realtime loads
//     module realtime); every key/channel pattern of a binary user is used by its code (no dead grant); no
//     allkeys/allchannels/~*/&*, no +@all/+@dangerous/+@admin, no KEYS/FLUSHALL/DEBUG/CONFIG… for an application user;
//     -@dangerous -@admin after the last granted category; passwords only as {{sha256:<sops key>}}; user names = the
//     EW_VALKEY_USERNAME of deploy/swarm/<env>/<binary>.env and the gateway user of the routes env.
//   - deploy/nats/permissions.yaml: every NATS subject literal ("ew.…") of the Go code is granted (publish, subscribe
//     or through a stream the user manages/consumes) to a binary that runs the code, with the direction fixed for the
//     security-relevant ones (only realtime publishes ew.cmd.>/ew.in.>/ew.rt.presence.*, only core-api subscribes
//     them, only worker publishes the domain events); every grant is used; every stream of the code is managed;
//     no whole-space wildcard; $JS.API.> only for ops; the largest user JWT fits max_control_line of nats.conf.
// Usage: node scripts/deploy/check-acl-templates.mjs   → exit 1 on violations.
import fs from 'node:fs';
import path from 'node:path';
import { REPO_ROOT, yaml } from './lib-policy.mjs';
import { aclLines, loadNatsPermissions, expandAll } from './render-acl.mjs';

export const DOMAIN = ['identity', 'learning', 'game', 'competition', 'chat'];
export const BINARIES = ['core-api', 'worker', 'realtime'];
const DOMAIN_BIN = ['core-api', 'worker'];

// ---------------------------------------------------------------------------------------------- Go code scan

/** Lists the non-test Go files of the three binaries (src/backend/{cmd,pkg,modules}, without modules/<m>/cmd tools). */
export function goFiles(root = path.join(REPO_ROOT, 'src', 'backend')) {
  const out = [];
  const walk = (d) => {
    for (const e of fs.readdirSync(d, { withFileTypes: true })) {
      const p = path.join(d, e.name);
      if (e.isDirectory()) {
        if (['ent', 'testdata', 'node_modules', 'migrations'].includes(e.name)) continue;
        walk(p);
      } else if (e.name.endsWith('.go') && !e.name.endsWith('_test.go')) out.push(p);
    }
  };
  for (const top of ['cmd', 'pkg', 'modules']) if (fs.existsSync(path.join(root, top))) walk(path.join(root, top));
  return out.map((f) => path.relative(root, f).split(path.sep).join('/'))
    .filter((f) => !/^modules\/[^/]+\/cmd\//.test(f) && !f.startsWith('cmd/internal/wiring/wiringtest'));
}

/** Binaries that load a Go file (null for pkg/ and cmd/: classified by the EXPECT tables). */
export function candidatesOf(file) {
  const m = /^modules\/([^/]+)\//.exec(file);
  if (!m) return null;
  if (m[1] === 'realtime') return ['realtime'];
  if (DOMAIN.includes(m[1])) return DOMAIN_BIN;
  return [];
}

// Valkey literals outside the modules: who uses them (pkg/realtime channels are subscribed by realtime and published
// by core-api/worker through pkg/realtime.Publisher; the realtime ticket is written by module game and consumed by
// realtime; pkg/roomhost is the room registry of module game).
export const VALKEY_EXPECT = {
  'pkg/realtime/realtime.go': {
    'ew:rt:broadcast': ['realtime'], 'ew:rt:party:': ['realtime'], 'ew:rt:node:': ['realtime'],
    'ew:rt:user:': ['realtime', 'core-api', 'worker'], 'ew:rt:room:': ['realtime', 'core-api', 'worker'],
    'ew:chat:fanout:': ['realtime', 'core-api', 'worker'], 'ew:game:rt:ticket:': ['realtime', 'core-api', 'worker'],
  },
  'pkg/roomhost/valkey.go': { 'ew:rt:room:': DOMAIN_BIN },
};

// NATS literals outside the modules, with their direction.
export const NATS_EXPECT = {
  'pkg/realtime/command.go': { 'ew.cmd.': { pub: ['realtime'], sub: ['core-api'] }, 'ew.in.room.': { pub: ['realtime'], sub: ['core-api'] } },
  'pkg/realtime/presence.go': { 'ew.rt.presence.': { pub: ['realtime'], sub: ['core-api'] } },
  'pkg/natsx/streams.go': { 'ew.evt.': { pub: ['worker'] }, 'ew.dlq.': { pub: ['worker'] } },
  'pkg/outbox/writer.go': { 'ew.evt.': { pub: ['worker'] } },
  'pkg/natsx/principal_invalidator.go': { '*': { sub: ['core-api'] } },
};

/** Extracts the "ew:…" (Valkey) and "ew.…" (NATS) string literals of the Go files: [{file, lit, kind}]. */
export function scanCode(root = path.join(REPO_ROOT, 'src', 'backend'), files = goFiles(root)) {
  const out = [];
  for (const f of files) {
    const text = fs.readFileSync(path.join(root, f), 'utf8');
    for (const m of text.matchAll(/"(ew:[^"]*)"/g)) out.push({ file: f, lit: m[1].split('%')[0], kind: 'valkey' });
    for (const m of text.matchAll(/"(ew\.[A-Za-z0-9_.*>-]*)"/g)) out.push({ file: f, lit: m[1], kind: 'nats' });
    for (const m of text.matchAll(/CacheOptions\{Module:\s*"([a-z]+)"/g)) out.push({ file: f, lit: `ew:${m[1]}:principal:`, kind: 'valkey', authz: true });
    for (const m of text.matchAll(/\b\w*Stream\w*\s*=\s*"([A-Z][A-Z0-9_]+)"/g)) out.push({ file: f, lit: m[1], kind: 'stream' });
  }
  return out;
}

// ---------------------------------------------------------------------------------------------- matching

const globRe = (p) => new RegExp('^' + p.split('*').map((s) => s.replace(/[.+?^${}()|[\]\\]/g, '\\$&')).join('.*') + '$');
/** A Valkey literal ("prefix:" or full key) is covered by a glob pattern. */
export const vCovers = (pattern, lit) => globRe(pattern).test(lit.endsWith(':') ? `${lit}x` : lit);

/** NATS: grant g covers subject or pattern s (every subject matching s matches g). */
export function nCovers(g, s) {
  const G = g.split('.'); const S = s.split('.');
  for (let i = 0; i < G.length; i++) {
    if (G[i] === '>') return i < S.length;
    if (i >= S.length || S[i] === '>') return false;
    if (G[i] !== '*' && G[i] !== S[i]) return false;
    if (G[i] === '*' && S[i] === '>') return false;
  }
  return G.length === S.length;
}

/** NATS: a literal prefix "a.b." can be produced under grant g (dynamic subject built from the prefix). */
export function nPrefix(g, lit) {
  const L = lit.replace(/\.$/, '').split('.'); const G = g.split('.');
  for (let i = 0; i < L.length; i++) {
    if (G[i] === undefined) return false;
    if (G[i] === '>') return true;
    if (G[i] !== '*' && G[i] !== L[i] && L[i] !== '*') return false;
  }
  return G.length > L.length;
}
const nMatch = (g, lit) => (lit.endsWith('.') ? nPrefix(g, lit) : nCovers(g, lit));

// ---------------------------------------------------------------------------------------------- Valkey

/** Parses one ACL rule line: {name, tokens, keys, channels, cmds, selectors, passwords, on}. */
export function parseAclLine(line) {
  const sel = [...line.matchAll(/\(([^)]*)\)/g)].map((m) => m[1].trim().split(/\s+/));
  const tokens = line.replace(/\([^)]*\)/g, ' ').trim().split(/\s+/);
  if (tokens[0] !== 'user' || !tokens[1]) throw new Error(`not a user rule: ${line.slice(0, 40)}`);
  const rest = tokens.slice(2);
  const pick = (ts) => ({
    keys: ts.filter((t) => /^(~|%R~|%W~|%RW~)/.test(t)).map((t) => t.replace(/^%?R?W?~/, '')),
    channels: ts.filter((t) => t.startsWith('&')).map((t) => t.slice(1)),
    cmds: ts.filter((t) => /^[+-]/.test(t)),
  });
  return { name: tokens[1], tokens: rest, on: rest.includes('on'), passwords: rest.filter((t) => /^[#>]/.test(t)), ...pick(rest),
    selectors: sel.map(pick) };
}

const FORBIDDEN_TOKENS = ['allkeys', 'allchannels', 'allcommands', '+@all', '+@dangerous', '+@admin', '~*', '&*', '%R~*', '%W~*', '%RW~*'];
const FORBIDDEN_CMDS = ['+keys', '+flushall', '+flushdb', '+debug', '+config', '+config|set', '+config|resetstat', '+shutdown', '+monitor',
  '+acl', '+module', '+save', '+bgsave', '+bgrewriteaof', '+migrate', '+restore', '+swapdb', '+failover', '+cluster'];
// Commands only some infrastructure users may hold (Valkey Sentinel documentation, redis_exporter, replication).
const RESERVED = { '+info': ['sentinel', 'exporter'], '+config|get': ['exporter'], '+config|rewrite': ['sentinel'],
  '+slaveof': ['sentinel'], '+replicaof': ['sentinel'], '+client|kill': ['sentinel'], '+script|kill': ['sentinel'],
  '+psync': ['replicator'], '+replconf': ['replicator'] };

export function checkValkey({ template, code, secretKeys, binaryUsers, gatewayUser, gatewayKeyLit }) {
  const problems = [];
  const users = {};
  for (const l of aclLines(template)) {
    let u;
    try { u = parseAclLine(l); } catch (e) { problems.push(`valkey: ${e.message}`); continue; }
    if (users[u.name]) problems.push(`valkey: user ${u.name} defined twice`);
    users[u.name] = u;
    const all = [...u.tokens, ...u.selectors.flatMap((s) => [...s.keys.map((k) => `~${k}`), ...s.cmds])];
    for (const t of all) {
      if (FORBIDDEN_TOKENS.includes(t)) problems.push(`valkey: user ${u.name} has forbidden rule ${t}`);
      if (FORBIDDEN_CMDS.includes(t.toLowerCase())) problems.push(`valkey: user ${u.name} may run ${t.slice(1).toUpperCase()}`);
      const r = RESERVED[t.toLowerCase()];
      if (r && !r.includes(u.name)) problems.push(`valkey: ${t} is reserved to ${r.join('/')} (user ${u.name})`);
    }
    if (u.name === 'default') {
      if (u.on || !u.tokens.includes('off')) problems.push('valkey: the default user must be off');
      continue;
    }
    if (!u.on) continue;
    if (u.tokens.includes('nopass')) problems.push(`valkey: user ${u.name} has nopass`);
    if (!u.tokens.includes('resetkeys') || !u.tokens.includes('resetchannels')) problems.push(`valkey: user ${u.name} must start with resetkeys resetchannels`);
    if (u.cmds[0] !== '-@all') problems.push(`valkey: user ${u.name}: first command rule must be -@all`);
    const lastPlusCat = u.cmds.map((c) => c.startsWith('+@')).lastIndexOf(true);
    if (lastPlusCat >= 0) {
      const after = u.cmds.slice(lastPlusCat + 1);
      for (const need of ['-@dangerous', '-@admin']) if (!after.includes(need)) problems.push(`valkey: user ${u.name}: ${need} must follow the last granted category`);
    }
    if (u.passwords.length !== 1) problems.push(`valkey: user ${u.name} needs exactly one password`);
    for (const p of u.passwords) {
      const m = /^#\{\{sha256:([a-z][a-z0-9_]{1,62})(?:@[a-z_]+)?\}\}$/.exec(p);
      if (!m) problems.push(`valkey: user ${u.name}: password must be #{{sha256:<sops key>}} (no plaintext, no fixed hash)`);
      else if (!secretKeys.has(m[1])) problems.push(`valkey: user ${u.name}: ${m[1]} is not a key of deploy/secrets/secrets.template.yaml`);
    }
  }
  // Binary users exist, named as their binaries' EW_VALKEY_USERNAME, password = <prefix>_valkey_password.
  for (const [bin, { username, secret }] of Object.entries(binaryUsers)) {
    const u = users[username];
    if (!u) { problems.push(`valkey: no ACL user ${username} (EW_VALKEY_USERNAME of ${bin})`); continue; }
    if (!u.passwords.some((p) => p.includes(`{{sha256:${secret}}}`))) problems.push(`valkey: user ${username} must use ${secret}`);
  }
  // Coverage: every literal of the code, for every binary loading it.
  const userOf = (bin) => users[binaryUsers[bin]?.username];
  const covered = (u, lit) => u && [...u.keys, ...u.channels, ...u.selectors.flatMap((s) => [...s.keys, ...s.channels])].some((p) => vCovers(p, lit));
  const used = new Map(); // `${user}\u0000${pattern}` → true
  const attributed = [];
  for (const c of code.filter((x) => x.kind === 'valkey')) {
    if (c.lit === 'ew:') continue; // dynamic prefix of pkg/authz (attributed through CacheOptions{Module: …})
    let bins = candidatesOf(c.file);
    if (c.authz) bins = c.file.startsWith('cmd/') ? ['core-api'] : bins;
    if (!bins) {
      bins = VALKEY_EXPECT[c.file]?.[c.lit];
      if (!bins) { problems.push(`valkey: ${c.file}: "${c.lit}" is not classified (add it to VALKEY_EXPECT of check-acl-templates.mjs)`); continue; }
    }
    for (const b of bins) {
      attributed.push({ bin: b, lit: c.lit });
      if (!covered(userOf(b), c.lit)) problems.push(`valkey: "${c.lit}" (${c.file}) is not covered for ${b} (user ${binaryUsers[b]?.username})`);
    }
  }
  for (const b of BINARIES) {
    const u = userOf(b);
    if (!u) continue;
    for (const p of [...u.keys, ...u.channels, ...u.selectors.flatMap((s) => [...s.keys, ...s.channels])]) {
      if (!attributed.some((a) => a.bin === b && vCovers(p, a.lit))) problems.push(`valkey: user ${u.name}: pattern ${p} is used by no code of ${b} (least privilege)`);
    }
  }
  void used;
  // Gateway user: the rate-limit key of ew-api-rate, nobody else.
  const g = users[gatewayUser];
  if (!g) problems.push(`valkey: no ACL user ${gatewayUser} (EW_RATELIMIT_VALKEY_USERNAME)`);
  else if (!g.keys.some((p) => vCovers(p, gatewayKeyLit))) problems.push(`valkey: user ${gatewayUser} does not cover ${gatewayKeyLit}`);
  for (const b of BINARIES) if (covered(userOf(b), gatewayKeyLit)) problems.push(`valkey: ${b} must not reach the gateway counters ${gatewayKeyLit}`);
  return problems;
}

// ---------------------------------------------------------------------------------------------- NATS

const STREAM_SUBJECTS_STATIC = {
  CHAT_MESSAGES: ['ew.chat.*.msg.*', 'ew.chat.*.evt.*', 'ew.chat.*.cursor.*'],
  IDENTITY_SIS_JOBS: ['ew.job.identity.sis.>'],
  GAME_RENDER_JOBS: ['ew.job.game.render.>'],
};
export function streamSubjects(name) {
  const m = /^(EVT|DLQ)_([A-Z]+)$/.exec(name);
  if (m) return [`ew.${m[1].toLowerCase()}.${m[2].toLowerCase()}.>`];
  return STREAM_SUBJECTS_STATIC[name] ?? [];
}

/** Estimated size (bytes) of the JWT of a user, calibrated on nsc 2.10 output (base64 of the claims + signature). */
export function jwtEstimate(e) {
  const json = 420 + [...e.pub, ...e.sub].reduce((n, s) => n + s.length + 3, 0) + (e.resp ? 40 : 0);
  return Math.ceil(((json * 4) / 3 + 120) * 1.1); // +10 %: the worker JWT measured 6247 B for an estimate of 5954 B
}

export function checkNats({ perms, code, secretKeys, streamsInCode, maxControlLine, stackCreds }) {
  const problems = [];
  const users = perms.users ?? {};
  for (const name of [...BINARIES, 'ops']) if (!users[name]) problems.push(`nats: user ${name} missing`);
  let expanded;
  try { expanded = expandAll(perms); } catch (e) { return [`nats: ${e.message}`]; }
  for (const [name, u] of Object.entries(users)) {
    if (!secretKeys.has(u.secret)) problems.push(`nats: user ${name}: ${u.secret} is not a key of deploy/secrets/secrets.template.yaml`);
    for (const s of [...(u.publish ?? []), ...(u.subscribe ?? [])]) {
      if (/^(>|\*\.>|ew\.>|\*|\$JS\.>|\$JS\.API\.>)$/.test(s) && !(name === 'ops' && s === '$JS.API.>')) problems.push(`nats: user ${name} grants the whole space ${s}`);
      if (s.startsWith('$JS.') && name !== 'ops') problems.push(`nats: user ${name}: JetStream API only through jetstream.<STREAM> (${s})`);
      if (s.length > 255) problems.push(`nats: user ${name}: subject too long`);
    }
    for (const s of u.subscribe ?? []) if (s.startsWith('_INBOX') && s !== '_INBOX.>') problems.push(`nats: user ${name}: unexpected inbox subscription ${s}`);
    for (const st of Object.keys(u.jetstream ?? {})) if (!streamsInCode.has(st)) problems.push(`nats: user ${name}: stream ${st} is not created by the code`);
  }
  for (const [bin, secret] of Object.entries(stackCreds)) {
    if (users[bin] && users[bin].secret !== secret) problems.push(`nats: user ${bin} uses ${users[bin].secret}, the stack mounts ${secret}`);
  }
  const pubOf = (u) => users[u]?.publish ?? [];
  const subOf = (u) => users[u]?.subscribe ?? [];
  const streamSubs = (u) => Object.keys(users[u]?.jetstream ?? {}).flatMap(streamSubjects);
  // Security invariants.
  const sample = { 'ew.cmd.x.y': 'realtime', 'ew.in.room.r': 'realtime', 'ew.rt.presence.t': 'realtime', 'ew.evt.realtime.x': 'realtime' };
  for (const d of DOMAIN) sample[`ew.evt.${d}.x`] = 'worker';
  for (const [s, owner] of Object.entries(sample)) {
    for (const u of Object.keys(users)) {
      if (u !== owner && u !== 'ops' && pubOf(u).some((g) => nCovers(g, s))) problems.push(`nats: only ${owner} may publish ${s.replace(/\.[a-z]$/, '.…')} (user ${u})`);
    }
  }
  for (const u of Object.keys(users)) {
    if (u !== 'core-api' && subOf(u).some((g) => nCovers(g, 'ew.cmd.x.y') || nCovers(g, 'ew.in.room.r'))) problems.push(`nats: only core-api may subscribe the realtime commands (user ${u})`);
  }
  if (subOf('realtime').some((s) => s !== '_INBOX.>')) problems.push('nats: realtime subscribes nothing but its inbox');
  // Coverage + attribution.
  const attributed = [];
  const grantsAny = (u, lit) => [...pubOf(u), ...subOf(u), ...streamSubs(u)].some((g) => nMatch(g, lit));
  for (const c of code.filter((x) => x.kind === 'nats')) {
    const bins = candidatesOf(c.file);
    if (bins) {
      bins.forEach((b) => attributed.push({ bin: b, lit: c.lit }));
      if (!bins.some((b) => grantsAny(b, c.lit))) problems.push(`nats: "${c.lit}" (${c.file}) is granted to none of ${bins.join(', ')}`);
      continue;
    }
    const exp = NATS_EXPECT[c.file]?.[c.lit] ?? NATS_EXPECT[c.file]?.['*'];
    if (!exp) { problems.push(`nats: ${c.file}: "${c.lit}" is not classified (add it to NATS_EXPECT of check-acl-templates.mjs)`); continue; }
    for (const b of exp.pub ?? []) {
      attributed.push({ bin: b, lit: c.lit });
      if (!pubOf(b).some((g) => nMatch(g, c.lit))) problems.push(`nats: ${b} must publish "${c.lit}" (${c.file})`);
    }
    for (const b of exp.sub ?? []) {
      attributed.push({ bin: b, lit: c.lit });
      if (![...subOf(b), ...streamSubs(b)].some((g) => nMatch(g, c.lit))) problems.push(`nats: ${b} must receive "${c.lit}" (${c.file})`);
    }
  }
  // Audit events of module realtime go straight to JetStream (natsx.EventSubject("realtime", …)).
  if (code.some((c) => c.file.startsWith('modules/realtime/') && c.lit === 'realtime' )) void 0;
  attributed.push({ bin: 'realtime', lit: 'ew.evt.realtime.' });
  if (!pubOf('realtime').some((g) => nMatch(g, 'ew.evt.realtime.'))) problems.push('nats: realtime must publish ew.evt.realtime.> (audit.recorded, EVT_REALTIME)');
  for (const b of BINARIES) {
    for (const g of [...pubOf(b), ...subOf(b)]) {
      if (g === '_INBOX.>') continue;
      if (!attributed.some((a) => a.bin === b && (nMatch(g, a.lit) || (a.lit.endsWith('.') ? false : nCovers(a.lit, g))))) {
        problems.push(`nats: user ${b}: grant ${g} is used by no code of ${b} (least privilege)`);
      }
    }
  }
  // Streams: each one created by the code is managed by a binary.
  for (const st of streamsInCode) {
    if (!Object.values(users).some((u) => (u.jetstream?.[st] ?? []).includes('manage'))) problems.push(`nats: stream ${st} is managed by no user`);
  }
  for (const [name, e] of Object.entries(expanded)) {
    const est = jwtEstimate(e);
    if (est * 2 > maxControlLine) problems.push(`nats: user ${name}: JWT ~${est} B needs max_control_line >= ${est * 2} B (nats.conf has ${maxControlLine})`);
  }
  return problems;
}

// ---------------------------------------------------------------------------------------------- repo inputs

const sizeOf = (s) => { const m = /^(\d+)\s*(KB|K|MB|M)?$/i.exec(String(s ?? '').trim()); if (!m) return 4096; const n = Number(m[1]); return /^m/i.test(m[2] ?? '') ? n * 1048576 : m[2] ? n * 1024 : n; };
const envFile = (f) => Object.fromEntries(fs.readFileSync(f, 'utf8').split('\n').map((l) => /^([A-Z0-9_]+)=(.*)$/.exec(l.trim())).filter(Boolean).map((m) => [m[1], m[2]]));

export function repoInputs(root = REPO_ROOT) {
  const secretKeys = new Set(Object.keys(yaml.load(fs.readFileSync(path.join(root, 'deploy', 'secrets', 'secrets.template.yaml'), 'utf8')) ?? {}));
  const code = scanCode(path.join(root, 'src', 'backend'));
  const binaryUsers = {};
  const problems = [];
  for (const [bin, prefix] of [['core-api', 'core'], ['worker', 'worker'], ['realtime', 'realtime']]) {
    const names = new Set(['staging', 'production'].map((e) => envFile(path.join(root, 'deploy', 'swarm', e, `${bin}.env`)).EW_VALKEY_USERNAME));
    if (names.size !== 1) problems.push(`valkey: EW_VALKEY_USERNAME of ${bin} differs between staging and production`);
    binaryUsers[bin] = { username: [...names][0], secret: `${prefix}_valkey_password` };
  }
  const gatewayUser = envFile(path.join(root, 'deploy', 'apisix', 'routes', 'env', 'staging.env.example')).EW_RATELIMIT_VALKEY_USERNAME;
  const lua = fs.readFileSync(path.join(root, 'deploy', 'apisix', 'plugins', 'ew-api-rate.lua'), 'utf8');
  const gatewayKeyLit = /"(ew:rl:[a-z_]+:)"/.exec(lua)?.[1] ?? 'ew:rl:api:';
  // Streams created by the code: EVT_/DLQ_ of every module with an outbox relay, of module realtime (audit), and
  // the named streams (…Stream… = "NAME").
  const streamsInCode = new Set(code.filter((c) => c.kind === 'stream').map((c) => c.lit));
  for (const m of DOMAIN) {
    const dir = path.join(root, 'src', 'backend', 'modules', m);
    const reg = fs.readdirSync(dir).filter((f) => f.endsWith('.go') && !f.endsWith('_test.go')).map((f) => fs.readFileSync(path.join(dir, f), 'utf8')).join('\n');
    if (/r\.Relay\(/.test(reg)) { streamsInCode.add(`EVT_${m.toUpperCase()}`); streamsInCode.add(`DLQ_${m.toUpperCase()}`); }
  }
  const audit = path.join(root, 'src', 'backend', 'modules', 'realtime', 'internal', 'audit', 'publisher.go');
  if (fs.existsSync(audit) && /EnsureStreams?\(/.test(fs.readFileSync(audit, 'utf8'))) { streamsInCode.add('EVT_REALTIME'); streamsInCode.add('DLQ_REALTIME'); }
  const natsConf = fs.readFileSync(path.join(root, 'deploy', 'swarm', 'config', 'nats.conf'), 'utf8');
  const maxControlLine = sizeOf(/^max_control_line:\s*(\S+)/m.exec(natsConf)?.[1]);
  // NATS creds mounted by the stacks (EW_NATS_CREDS_FILE of each backend service).
  const stackCreds = {};
  for (const e of ['staging', 'production']) {
    const stack = yaml.load(fs.readFileSync(path.join(root, 'deploy', 'swarm', e, 'app.stack.yml'), 'utf8'));
    for (const [svc, def] of Object.entries(stack.services ?? {})) {
      const f = def.environment?.EW_NATS_CREDS_FILE;
      if (!f) continue;
      const bin = svc.startsWith('worker') ? 'worker' : svc;
      const secret = path.posix.basename(f);
      if (stackCreds[bin] && stackCreds[bin] !== secret) problems.push(`nats: ${bin} services mount different creds (${stackCreds[bin]}, ${secret})`);
      stackCreds[bin] = secret;
    }
  }
  return { secretKeys, code, binaryUsers, gatewayUser, gatewayKeyLit, streamsInCode, maxControlLine, stackCreds, problems,
    template: fs.readFileSync(path.join(root, 'deploy', 'valkey', 'users.acl.template'), 'utf8'),
    perms: loadNatsPermissions(path.join(root, 'deploy', 'nats', 'permissions.yaml')) };
}

export function checkAll(inputs = repoInputs()) {
  return [...inputs.problems, ...checkValkey(inputs), ...checkNats(inputs)];
}

if (process.argv[1]?.endsWith('check-acl-templates.mjs')) {
  const inputs = repoInputs();
  const problems = checkAll(inputs);
  for (const p of problems) console.error(p);
  const n = (k) => inputs.code.filter((c) => c.kind === k).length;
  console.log(`check-acl-templates: ${n('valkey')} Valkey literals, ${n('nats')} NATS literals, ${inputs.streamsInCode.size} streams, ${problems.length} problem(s)`);
  process.exit(problems.length ? 1 : 0);
}
