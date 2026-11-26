// Lists the Docker secret names an environment needs (F130 T024): every versioned secret referenced by its stacks, plus
// the one-off secrets of the policy (migrate jobs, restore credential, ops jobs). Used to build/verify
// deploy/secrets/<env>/*.sops.yaml (secrets.template.yaml shows the layout).
// Usage: node scripts/deploy/list-secrets.mjs <staging|production> [--check FILE.json]
import fs from 'node:fs';
import path from 'node:path';
import { REPO_ROOT, loadPolicy, loadStacks } from './lib-policy.mjs';
import { VALKEY_TEMPLATE, valkeyInputs } from './render-acl.mjs';

// Rendered by secrets-sync.sh (never in SOPS); their inputs are required instead (F130 T053).
export const DERIVED = ['valkey_acl'];

export const ONE_OFF = ['identity_migrator_dsn', 'learning_migrator_dsn', 'game_migrator_dsn', 'competition_migrator_dsn', 'chat_migrator_dsn',
  'backup_restore_access_key', 'backup_restore_secret_key', 'ops_pg_admin_pgpass', 'ops_drill_dsn', 'ops_nats_creds', 'ops_kc_admin_secret'];

export function requiredSecrets(env, policy = loadPolicy()) {
  const names = new Set(ONE_OFF);
  for (const st of loadStacks(path.join(REPO_ROOT, 'deploy', 'swarm', env), policy)) {
    if (!st.missing) for (const k of Object.keys(st.doc.secrets ?? {})) names.add(k);
  }
  for (const k of DERIVED) names.delete(k);
  for (const k of valkeyInputs(fs.readFileSync(VALKEY_TEMPLATE, 'utf8'))) names.add(k);
  return [...names].sort();
}

if (process.argv[1]?.endsWith('list-secrets.mjs')) {
  const [env, flag, file] = process.argv.slice(2);
  if (!/^(staging|production)$/.test(env ?? '')) { console.error('usage: list-secrets.mjs <staging|production> [--check FILE.json]'); process.exit(2); }
  const req = requiredSecrets(env);
  if (flag === '--check') {
    const have = new Set(Object.keys(JSON.parse(fs.readFileSync(file, 'utf8'))));
    const missing = req.filter((n) => !have.has(n));
    if (missing.length) { console.error(`missing secrets for ${env}: ${missing.join(', ')}`); process.exit(1); }
    const derived = DERIVED.filter((n) => have.has(n));
    if (derived.length) { console.error(`derived secrets must not be in SOPS (rendered by secrets-sync.sh): ${derived.join(', ')}`); process.exit(1); }
    console.log(`all ${req.length} secrets present for ${env}`);
  } else console.log(req.join('\n'));
}
