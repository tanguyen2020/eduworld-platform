// Realm settings keycloak-config-cli 6.4 cannot express (F004): runs after it in every environment,
// idempotent. Removes offline_access and uma_authorization from default-roles-eduworld (no offline
// tokens for children, KEYCLOAK_SSO §4.3, §5).
import { admin, adminToken } from './kc.mjs';

const token = await adminToken();
const composites = (await admin('/roles/default-roles-eduworld/composites', { token })).json ?? [];
const drop = composites.filter((r) => r.name === 'offline_access' || r.name === 'uma_authorization');
if (drop.length > 0) {
  const r = await admin('/roles/default-roles-eduworld/composites', { method: 'DELETE', token, body: drop });
  if (r.status !== 204) throw new Error(`remove default role composites: ${r.status}`);
}
console.log(`post-apply: removed ${drop.length} default role composites`);
