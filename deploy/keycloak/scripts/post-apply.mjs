// Realm settings keycloak-config-cli 6.4 cannot express (F004): runs after it in every environment,
// idempotent.
//   1. Removes offline_access and uma_authorization from default-roles-eduworld (no offline tokens for
//      children, KEYCLOAK_SSO §4.3, §5).
//   2. ADR-0094 (no MFA on the Admin Portal): fails if the portal direct grant flow still validates an OTP,
//      disables the CONFIGURE_TOTP required action and removes it from every staff account that still
//      carries it, so nobody is blocked by "Account is not fully set up". OTP credentials enrolled before are
//      left in place (no flow reads them).
import { admin, adminToken } from './kc.mjs';

const token = await adminToken();
const composites = (await admin('/roles/default-roles-eduworld/composites', { token })).json ?? [];
const drop = composites.filter((r) => r.name === 'offline_access' || r.name === 'uma_authorization');
if (drop.length > 0) {
  const r = await admin('/roles/default-roles-eduworld/composites', { method: 'DELETE', token, body: drop });
  if (r.status !== 204) throw new Error(`remove default role composites: ${r.status}`);
}
console.log(`post-apply: removed ${drop.length} default role composites`);

// 2a. The portal flow has no OTP step left (keycloak-config-cli applied realm-eduworld.json).
const executions = (await admin('/authentication/flows/eduworld-portal-direct-grant/executions', { token })).json ?? [];
const otpSteps = executions.filter((e) => /otp/i.test(`${e.providerId ?? ''} ${e.displayName ?? ''} ${e.alias ?? ''}`));
if (otpSteps.length > 0) throw new Error(`eduworld-portal-direct-grant still has OTP steps: ${otpSteps.map((e) => e.displayName).join(', ')}`);

// 2b. CONFIGURE_TOTP is never required.
const totp = await admin('/authentication/required-actions/CONFIGURE_TOTP', { token });
if (totp.status === 200 && (totp.json.enabled || totp.json.defaultAction)) {
  const r = await admin('/authentication/required-actions/CONFIGURE_TOTP', { method: 'PUT', token,
    body: { ...totp.json, enabled: false, defaultAction: false } });
  if (r.status !== 204) throw new Error(`disable CONFIGURE_TOTP: ${r.status}`);
}

// 2c. Staff accounts still carrying CONFIGURE_TOTP (enrolment pending from before ADR-0094).
const page = 200;
let cleared = 0;
for (const role of ['platform_staff', 'tenant_staff']) {
  for (let first = 0; ; first += page) {
    const members = (await admin(`/roles/${role}/users?first=${first}&max=${page}`, { token })).json ?? [];
    for (const m of members) {
      const u = (await admin(`/users/${m.id}`, { token })).json;
      const actions = u?.requiredActions ?? [];
      if (!actions.includes('CONFIGURE_TOTP')) continue;
      const r = await admin(`/users/${m.id}`, { method: 'PUT', token,
        body: { ...u, requiredActions: actions.filter((a) => a !== 'CONFIGURE_TOTP') } });
      if (r.status !== 204) throw new Error(`remove CONFIGURE_TOTP from a user: ${r.status}`);
      cleared++;
    }
    if (members.length < page) break;
  }
}
console.log(`post-apply: OTP disabled for the portal (ADR-0094), CONFIGURE_TOTP removed from ${cleared} account(s)`);
