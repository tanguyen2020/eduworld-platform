// Renders the access-control templates of the datastores (F130 T053, OQ F090-2; DEPLOYMENT §8, §8b, §13):
//   - deploy/valkey/users.acl.template → the Docker secret `valkey_acl` (passwords as SHA-256 "#<hex>", no plaintext);
//   - deploy/nats/permissions.yaml → the concrete publish/subscribe lists signed into each user JWT by nsc;
//   - deploy/nats/accounts.template.conf → the Docker secret `nats_accounts` (operator mode, memory resolver).
// Values are read from a decrypted SOPS JSON in a private temp dir (secrets-sync.sh) and never printed except the
// rendered secret itself on stdout, which the caller redirects into a mode-600 file.
// Usage:
//   node scripts/deploy/render-acl.mjs valkey --secrets FILE.json          → rendered ACL file on stdout
//   node scripts/deploy/render-acl.mjs valkey-inputs                         → SOPS keys the ACL needs, one per line
//   node scripts/deploy/render-acl.mjs nats-expand [--tsv]                   → JSON {user: {secret, pub, sub, resp}}
//                                                                               (--tsv: user, secret, pub, sub, resp)
//   node scripts/deploy/render-acl.mjs nats-current --user-json FILE         → current allow/deny list of a user JWT
//   node scripts/deploy/render-acl.mjs nats-accounts --values FILE.json      → rendered accounts.conf on stdout
//   node scripts/deploy/render-acl.mjs nats-verify --secrets FILE.json | --creds-dir DIR
//                                                                             → exit 1 when a creds JWT differs from
//                                                                               deploy/nats/permissions.yaml
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';
import { REPO_ROOT, yaml } from './lib-policy.mjs';

export const VALKEY_TEMPLATE = path.join(REPO_ROOT, 'deploy', 'valkey', 'users.acl.template');
export const NATS_PERMISSIONS = path.join(REPO_ROOT, 'deploy', 'nats', 'permissions.yaml');
export const NATS_ACCOUNTS_TEMPLATE = path.join(REPO_ROOT, 'deploy', 'nats', 'accounts.template.conf');

const PLACEHOLDER = /\{\{sha256:([a-z][a-z0-9_]{1,62})(?:@([a-z_]{1,32}))?\}\}/g;

/** Template lines that are ACL rules (comments and blank lines removed). */
export function aclLines(text) {
  return text.split(/\r?\n/).map((l) => l.trim()).filter((l) => l && !l.startsWith('#'));
}

/** SOPS keys referenced by the ACL template (password placeholders). */
export function valkeyInputs(text) {
  return [...new Set([...text.matchAll(PLACEHOLDER)].map((m) => m[1]))].sort();
}

/** secretValue returns the password of key: the whole value, or the argument of `<directive> <password>`. */
function secretValue(secrets, key, directive) {
  const v = secrets[key];
  if (typeof v !== 'string' || v.trim() === '') throw new Error(`valkey ACL: secret ${key} is missing or empty`);
  if (!directive) return v;
  const m = new RegExp(`^\\s*${directive}\\s+(\\S+)\\s*$`, 'm').exec(v);
  if (!m) throw new Error(`valkey ACL: secret ${key} has no "${directive} <password>" line`);
  return m[1];
}

/** Renders the ACL file: every {{sha256:key}} → sha256(value) hex; comments removed. */
export function renderValkeyAcl(text, secrets) {
  const out = aclLines(text).map((l) => l.replace(PLACEHOLDER, (_, key, directive) =>
    crypto.createHash('sha256').update(secretValue(secrets, key, directive)).digest('hex')));
  for (const l of out) {
    if (/\{\{|\}\}/.test(l)) throw new Error(`valkey ACL: unresolved placeholder in "${l.slice(0, 40)}…"`);
    if (/(^|\s)>\S/.test(l)) throw new Error('valkey ACL: plaintext password (">…") is not allowed');
  }
  return out.join('\n') + '\n';
}

export function loadNatsPermissions(file = NATS_PERMISSIONS) {
  return yaml.load(fs.readFileSync(file, 'utf8'));
}

const JS_MODES = {
  manage: (s) => [`$JS.API.STREAM.CREATE.${s}`, `$JS.API.STREAM.UPDATE.${s}`, `$JS.API.STREAM.INFO.${s}`],
  consume: (s) => [`$JS.API.STREAM.INFO.${s}`, `$JS.API.CONSUMER.CREATE.${s}`, `$JS.API.CONSUMER.CREATE.${s}.>`,
    `$JS.API.CONSUMER.DURABLE.CREATE.${s}.>`, `$JS.API.CONSUMER.INFO.${s}.>`, `$JS.API.CONSUMER.MSG.NEXT.${s}.>`,
    `$JS.API.CONSUMER.DELETE.${s}.>`, `$JS.ACK.${s}.>`],
};

/** Expands one user of permissions.yaml into the exact allow lists of its JWT (sorted, unique). */
export function expandUser(u) {
  const pub = new Set(u.publish ?? []);
  for (const [stream, modes] of Object.entries(u.jetstream ?? {})) {
    if (!/^[A-Z][A-Z0-9_]{1,62}$/.test(stream)) throw new Error(`nats: invalid stream name ${stream}`);
    for (const m of modes) {
      if (!JS_MODES[m]) throw new Error(`nats: unknown jetstream mode ${m} on ${stream}`);
      for (const s of JS_MODES[m](stream)) pub.add(s);
    }
  }
  return { secret: u.secret, pub: [...pub].sort(), sub: [...new Set(u.subscribe ?? [])].sort(), resp: Boolean(u.allow_responses) };
}

export function expandAll(perms = loadNatsPermissions()) {
  return Object.fromEntries(Object.entries(perms.users ?? {}).map(([name, u]) => [name, expandUser(u)]));
}

/** Renders accounts.conf from the template and {operator_jwt, system_account, system_account_jwt, account, account_jwt}. */
export function renderNatsAccounts(text, values) {
  const KEYS = { operator_jwt: /^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/, system_account_jwt: /^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/,
    account_jwt: /^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$/, system_account: /^A[A-Z2-7]{55}$/, account: /^A[A-Z2-7]{55}$/ };
  for (const [k, re] of Object.entries(KEYS)) {
    if (!re.test(String(values[k] ?? ''))) throw new Error(`nats accounts: ${k} missing or malformed`);
  }
  const out = text.replace(/\{\{([a-z_]+)\}\}/g, (_, k) => {
    if (!(k in KEYS)) throw new Error(`nats accounts: unknown placeholder ${k}`);
    return values[k];
  });
  return out;
}

/** JWT claims of a .creds file (payload only; the server verifies the signature). */
export function credsClaims(creds) {
  const m = /-----BEGIN NATS USER JWT-----\s*([A-Za-z0-9_.-]+)\s*------END NATS USER JWT------/.exec(creds);
  if (!m) throw new Error('creds: no user JWT block');
  const payload = m[1].split('.')[1];
  return JSON.parse(Buffer.from(payload, 'base64url').toString('utf8'));
}

/** Drift between the permissions of a creds JWT and the expanded permissions of its user. */
export function credsDrift(user, expanded, claims) {
  const n = claims.nats ?? {};
  const out = [];
  const eq = (a, b) => JSON.stringify([...(a ?? [])].sort()) === JSON.stringify([...b].sort());
  if (!eq(n.pub?.allow, expanded.pub)) out.push(`${user}: publish permissions differ from deploy/nats/permissions.yaml`);
  if (!eq(n.sub?.allow, expanded.sub)) out.push(`${user}: subscribe permissions differ from deploy/nats/permissions.yaml`);
  if ((n.pub?.deny ?? []).length || (n.sub?.deny ?? []).length) out.push(`${user}: unexpected deny list in the JWT`);
  if (Boolean(n.resp) !== expanded.resp) out.push(`${user}: allow_responses differs from deploy/nats/permissions.yaml`);
  if (claims.name && claims.name !== user) out.push(`${user}: JWT is for user ${claims.name}`);
  return out;
}

/** Checks every <binary>_nats_creds present in secrets against permissions.yaml. */
export function verifyNatsCreds(secrets, perms = loadNatsPermissions()) {
  const problems = [];
  for (const [user, exp] of Object.entries(expandAll(perms))) {
    const creds = secrets[exp.secret];
    if (creds === undefined) continue;
    try {
      problems.push(...credsDrift(user, exp, credsClaims(creds)));
    } catch (e) {
      problems.push(`${user}: ${exp.secret}: ${e.message}`);
    }
  }
  return problems;
}

/** Every allow/deny subject of a user JWT (nsc describe user --json), to remove before re-applying permissions.yaml. */
export function currentPermissions(claims) {
  const n = claims.nats ?? {};
  return [...new Set(['pub', 'sub'].flatMap((k) => [...(n[k]?.allow ?? []), ...(n[k]?.deny ?? [])]))];
}

/** {<secret>: creds text} of the <secret>.creds files written by nats-accounts.sh. */
export function credsFromDir(dir) {
  return Object.fromEntries(fs.readdirSync(dir).filter((f) => f.endsWith('.creds'))
    .map((f) => [f.slice(0, -'.creds'.length), fs.readFileSync(path.join(dir, f), 'utf8')]));
}

function arg(name) {
  const i = process.argv.indexOf(`--${name}`);
  return i > 0 ? process.argv[i + 1] : undefined;
}

if (process.argv[1]?.endsWith('render-acl.mjs')) {
  const cmd = process.argv[2];
  const readJson = (k) => {
    const f = arg(k);
    if (!f) { console.error(`--${k} FILE.json required`); process.exit(2); }
    return JSON.parse(fs.readFileSync(f, 'utf8'));
  };
  try {
    if (cmd === 'valkey') process.stdout.write(renderValkeyAcl(fs.readFileSync(VALKEY_TEMPLATE, 'utf8'), readJson('secrets')));
    else if (cmd === 'valkey-inputs') console.log(valkeyInputs(fs.readFileSync(VALKEY_TEMPLATE, 'utf8')).join('\n'));
    else if (cmd === 'nats-expand' && process.argv.includes('--tsv')) {
      for (const [u, e] of Object.entries(expandAll())) console.log([u, e.secret, e.pub.join(','), e.sub.join(','), e.resp].join('\t'));
    } else if (cmd === 'nats-expand') console.log(JSON.stringify(expandAll(), null, 2));
    else if (cmd === 'nats-current') console.log(currentPermissions(readJson('user-json')).join(','));
    else if (cmd === 'nats-accounts') process.stdout.write(renderNatsAccounts(fs.readFileSync(NATS_ACCOUNTS_TEMPLATE, 'utf8'), readJson('values')));
    else if (cmd === 'nats-verify') {
      const problems = verifyNatsCreds(arg('creds-dir') ? credsFromDir(arg('creds-dir')) : readJson('secrets'));
      for (const p of problems) console.error(`nats-verify: ${p}`);
      if (problems.length) process.exit(1);
    } else { console.error('usage: render-acl.mjs valkey|valkey-inputs|nats-expand|nats-accounts|nats-verify'); process.exit(2); }
  } catch (e) {
    console.error(`render-acl: ${e.message}`);
    process.exit(1);
  }
}
