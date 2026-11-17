// Local dev data of realm eduworld (F004 T013/T043, LOCAL_DEVELOPMENT §5), idempotent:
//   1. users of realm-eduworld.dev.json through Admin API partialImport with ifResourceExists=SKIP
//      (creates missing users with password and OTP credentials, never rewrites existing ones, so the
//      realm password history is never violated);
//   2. Organizations and members (keycloak-config-cli 6.4 does not import organizations).
// Local only: refuses to run unless EW_ENV=local.
import { readFileSync } from 'node:fs';
import { admin, adminToken } from './kc.mjs';

if ((process.env.EW_ENV ?? 'local') !== 'local') {
  console.error('seed-dev: EW_ENV must be local');
  process.exit(1);
}
const dev = JSON.parse(readFileSync(new URL('../realm-eduworld.dev.json', import.meta.url), 'utf8'));
const token = await adminToken();
const imp = await admin('/partialImport', { method: 'POST', token, body: { ifResourceExists: 'SKIP', users: dev.users } });
if (imp.status !== 200) throw new Error(`partialImport: ${imp.status} ${JSON.stringify(imp.json)}`);
console.log(`users: ${imp.json.added} added, ${imp.json.skipped} skipped`);
await import('./seed-dev-orgs.mjs');
