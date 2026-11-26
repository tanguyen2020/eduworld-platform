// Secret distribution rules (F130-AC-08, AC-09; DEPLOYMENT §13; AGENTS R3, R4; policy deploy/swarm/policy.yaml).
//  - every secret mounted by a service matches exactly one rule of the policy, and the service may mount it
//    (module credential only where the module is loaded, kc_game_backend only core-api, chat_purger_dsn only
//    worker-chat, <module>_migrator_dsn never in a stack, realtime never a database credential…);
//  - every top-level secret is external and versioned: name "<key>_v${SECRET_VER_<KEY>}";
//  - no secret value in a stack `environment` or a <service>.env: a secret-like key carries only <KEY>_FILE pointing
//    to a mounted /run/secrets/<name>; no value looks like a credential (user:password@ URL, private key, AWS key…).
// Usage: node scripts/deploy/check-secret-refs.mjs [envDir…] → exit 1 on violations (job `validate` of deploy-config.yml).
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { REPO_ROOT, loadPolicy, loadStacks, anyGlob, serviceLine, lineOf, readEnvFile, serviceEnv, serviceSecrets, asList } from './lib-policy.mjs';

const BINARY_OF = { core: ['core-api'], realtime: ['realtime'], worker: ['worker-events', 'worker-chat', 'worker-jobs'] };

// Value patterns that are credentials whatever the key (gitleaks covers Git history in security.yml).
export const VALUE_PATTERNS = [
  [/[a-z][a-z0-9+.-]*:\/\/[^/\s:@]+:[^@\s/]+@/i, 'credentials embedded in a URL'],
  [/-----BEGIN [A-Z ]*PRIVATE KEY-----/, 'private key'],
  [/\bAKIA[0-9A-Z]{16}\b/, 'AWS access key id'],
  [/\bSU[A-Z2-7]{56}\b/, 'NATS nkey seed'],
  [/\bgh[pousr]_[A-Za-z0-9]{36,}\b/, 'GitHub token'],
  [/\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}/, 'JWT'],
];

function ruleFor(policy, name) {
  return (policy.secrets ?? []).map((r) => ({ r, m: name.match(new RegExp(r.match)) })).filter((x) => x.m);
}

/** Checks one secret mount of one service; returns an error message or null. */
export function checkMount(policy, service, secret) {
  const hits = ruleFor(policy, secret);
  if (hits.length === 0) return `${secret} matches no rule of deploy/swarm/policy.yaml (unknown secret)`;
  if (hits.length > 1) return `${secret} matches ${hits.length} rules (ambiguous policy)`;
  const { r, m } = hits[0];
  if (!anyGlob(r.services, service)) {
    return r.services.length === 0 ? `${secret} must never be mounted by a stack service (one-off job only)`
      : `${secret} may only be mounted by ${r.services.join(', ')} (DEPLOYMENT §13)`;
  }
  if (r.module_group) {
    const mod = m[r.module_group];
    const loaded = policy.backend_services?.[service]?.modules ?? [];
    if (!loaded.includes(mod)) return `${secret} is a credential of module ${mod}, not loaded by ${service}`;
  }
  if (r.binary_group) {
    const bin = m[r.binary_group];
    if (!(BINARY_OF[bin] ?? []).includes(service)) return `${secret} belongs to binary ${bin}, not to ${service}`;
  }
  return null;
}

/** Violations of one environment directory. */
export function checkEnv(envDir, policy = loadPolicy()) {
  const out = [];
  const keyRe = new RegExp(policy.secret_env_key);
  const allow = (policy.secret_env_key_allow ?? []).map((p) => new RegExp(p));
  const checkEnvValue = (add, line, where, key, value) => {
    if (/_FILE$/.test(key)) return; // checked against mounts by the caller
    if (keyRe.test(key) && !allow.some((a) => a.test(key)) && value !== '') {
      add(line, 'secret-value', `${where}: ${key} carries a value; use ${key}_FILE=/run/secrets/<name> (Docker secret)`);
    }
    for (const [re, what] of VALUE_PATTERNS) if (re.test(value)) add(line, 'secret-pattern', `${where}: ${key} looks like a ${what}`);
  };
  for (const st of loadStacks(envDir, policy)) {
    if (st.missing) continue; // reported by check-stacks
    const { file, text, doc } = st;
    const add = (line, rule, message, f = file) => out.push({ file: f, line, rule, message });
    for (const [name, raw] of Object.entries(doc.services ?? {})) {
      const svc = raw ?? {};
      const at = serviceLine(text, name);
      const mounts = serviceSecrets(svc);
      const targets = new Set(mounts.map((s) => (s.target.startsWith('/') ? s.target : `/run/secrets/${s.target}`)));
      for (const s of mounts) {
        const msg = checkMount(policy, name, s.source);
        if (msg) add(lineOf(text, new RegExp(`\\b${s.source}\\b`), at, at), 'secret-owner', `${name}: ${msg}`);
        if (!doc.secrets?.[s.source]) add(at, 'secret-undeclared', `${name}: secret ${s.source} is not declared at the top level`);
      }
      if (name === 'realtime' && mounts.some((s) => /_dsn$/.test(s.source))) add(at, 'realtime-db', 'realtime: never a database credential (DEPLOYMENT §13)');
      for (const { key, value } of serviceEnv(svc)) {
        const line = lineOf(text, new RegExp(`\\b${key}\\b`), at, at);
        checkEnvValue(add, line, name, key, value);
        if (/_FILE$/.test(key) && value.startsWith('/run/secrets/') && !targets.has(value)) {
          add(line, 'secret-file-unmounted', `${name}: ${key} points to ${value}, which no mounted secret provides`);
        }
      }
      for (const f of asList(svc.env_file)) {
        const envPath = path.join(path.dirname(file), f);
        const entries = readEnvFile(envPath);
        if (!entries) { add(at, 'env-file', `${name}: env_file ${f} is missing`); continue; }
        for (const e of entries) {
          checkEnvValue((l, r, m) => add(l, r, m, envPath), e.line, `${f}`, e.key, e.value);
          if (/_FILE$/.test(e.key)) add(e.line, 'secret-file-in-env', `${f}: ${e.key} belongs in the stack next to its secret mount`, envPath);
        }
      }
      for (const field of ['command', 'entrypoint']) {
        const v = JSON.stringify(svc[field] ?? '');
        for (const [re, what] of VALUE_PATTERNS) if (re.test(v)) add(at, 'secret-pattern', `${name}: ${field} contains a ${what}`);
      }
    }
    for (const [key, def] of Object.entries(doc.secrets ?? {})) {
      const line = lineOf(text, new RegExp(`^  ${key}:`));
      const want = `${key}_v\${SECRET_VER_${key.toUpperCase()}}`;
      if (def?.external !== true) add(line, 'secret-external', `secret ${key} must be external (created by secrets-sync.sh), never a file in the stack`);
      if (def?.name !== want) add(line, 'secret-version', `secret ${key} must be named "${want}" (versioned, DEPLOYMENT §4.5)`);
      if (def?.file) add(line, 'secret-file', `secret ${key} must not come from a file`);
    }
  }
  return out;
}

export function main(argv = process.argv.slice(2)) {
  const policy = loadPolicy();
  const dirs = argv.length ? argv : policy.envs.map((e) => path.join(REPO_ROOT, 'deploy', 'swarm', e));
  const out = dirs.flatMap((d) => checkEnv(d, policy));
  for (const v of out) console.error(`${path.relative(process.cwd(), v.file)}:${v.line}: [${v.rule}] ${v.message}`);
  if (out.length) { console.error(`check-secret-refs: ${out.length} violation(s)`); return 1; }
  console.log(`check-secret-refs: ${dirs.length} environment(s) ok`);
  return 0;
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) process.exit(main());
