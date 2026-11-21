// F004 T007/T040 — static checks of the realm as code (F004-AC-01, AC-02, AC-04, AC-05, AC-06, AC-08,
// AC-24, AC-25, AC-27; ADR-0094: no OTP step, no AMR/LoA mapper). Usage: node deploy/keycloak/scripts/check-realm.mjs  (exit 1 on any problem)
import { readFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';

export const CLIENTS = ['eduworld-game-backend', 'eduworld-portal', 'eduworld-api', 'eduworld-sis-sync', 'svc-identity'];
const DIRECT_GRANT = ['eduworld-game-backend', 'eduworld-portal'];
const SECRET_RE = /^\$\(env:KC_[A-Z0-9_]+\)$/;

// check returns the problems of the common realm file (no users except service accounts, no secrets).
export function check(realm) {
  const p = [];
  const clients = realm.clients ?? [];
  const ids = clients.map((c) => c.clientId);
  for (const id of CLIENTS) if (!ids.includes(id)) p.push(`client ${id} missing`);
  for (const id of ids) if (!CLIENTS.includes(id)) p.push(`unexpected client ${id} (KEYCLOAK_SSO §3; no svc-<module> clients, ADR-0074)`);
  for (const s of realm.clientScopes ?? []) if (s.name.startsWith('internal-')) p.push(`client scope ${s.name} must not exist (ADR-0074)`);
  for (const c of clients) {
    if ('secret' in c && !SECRET_RE.test(c.secret)) p.push(`client ${c.clientId}: secret must be a $(env:KC_…) placeholder`);
    if (c.directAccessGrantsEnabled && !DIRECT_GRANT.includes(c.clientId)) p.push(`client ${c.clientId}: direct access grants must be off`);
    const scopes = [...(c.defaultClientScopes ?? []), ...(c.optionalClientScopes ?? [])];
    if (scopes.includes('organization')) p.push(`client ${c.clientId}: scope organization must not be assigned (ADR-0091, ADR-0068)`);
    if (scopes.includes('offline_access')) p.push(`client ${c.clientId}: offline_access must not be assigned`);
    const overrides = c.authenticationFlowBindingOverrides ?? {};
    if ('browser' in overrides) p.push(`client ${c.clientId}: no browser flow binding (ADR-0091)`);
  }
  for (const id of DIRECT_GRANT) {
    const c = clients.find((x) => x.clientId === id);
    if (!c) continue;
    if (c.publicClient !== false || c.clientAuthenticatorType !== 'client-secret' || !c.secret) p.push(`client ${id}: must be confidential with a secret`);
    if (c.standardFlowEnabled !== false || c.implicitFlowEnabled !== false || c.serviceAccountsEnabled !== false) p.push(`client ${id}: only direct access grants`);
    if (c.directAccessGrantsEnabled !== true) p.push(`client ${id}: direct access grants must be on`);
    if ((c.redirectUris ?? []).length || (c.webOrigins ?? []).length) p.push(`client ${id}: no redirect URI or web origin`);
    if (c.attributes?.['pkce.code.challenge.method']) p.push(`client ${id}: no PKCE`);
    const want = id === 'eduworld-portal' ? 'eduworld-portal-direct-grant' : 'eduworld-game-direct-grant';
    if (c.authenticationFlowBindingOverrides?.direct_grant !== want) p.push(`client ${id}: direct grant flow must be ${want}`);
  }
  const svc = clients.find((c) => c.clientId === 'svc-identity');
  if (svc && (svc.serviceAccountsEnabled !== true || svc.standardFlowEnabled || svc.implicitFlowEnabled || svc.directAccessGrantsEnabled)) {
    p.push('client svc-identity: client credentials only');
  }
  for (const u of realm.users ?? []) if (!u.serviceAccountClientId) p.push(`user ${u.username}: the common realm file must not contain users`);
  const svcAccounts = clients.filter((c) => c.serviceAccountsEnabled).map((c) => c.clientId).sort();
  if (svcAccounts.join() !== ['eduworld-sis-sync', 'svc-identity'].join()) p.push(`service account clients must be eduworld-sis-sync and svc-identity, got ${svcAccounts}`);

  // Realm settings (KEYCLOAK_SSO §10, §5).
  const want = { registrationAllowed: false, duplicateEmailsAllowed: false, loginWithEmailAllowed: true, editUsernameAllowed: false,
    bruteForceProtected: true, permanentLockout: false, revokeRefreshToken: true, refreshTokenMaxReuse: 0, rememberMe: false };
  for (const [k, v] of Object.entries(want)) if (realm[k] !== v) p.push(`realm ${k} must be ${v}`);

  // Flows (AC-03, AC-25, AC-27).
  const flows = Object.fromEntries((realm.authenticationFlows ?? []).map((f) => [f.alias, f]));
  const seq = (alias) => (flows[alias]?.authenticationExecutions ?? []).slice().sort((a, b) => a.priority - b.priority)
    .map((e) => e.authenticator ?? `flow:${e.flowAlias}`);
  const same = (a, b) => a.length === b.length && a.every((x, i) => x === b[i]);
  if (!same(seq('eduworld-game-direct-grant'), ['direct-grant-validate-username', 'direct-grant-validate-password', 'flow:eduworld-game-deny-non-student'])) {
    p.push('flow eduworld-game-direct-grant: username → password → deny non-student');
  }
  if (!same(seq('eduworld-portal-direct-grant'), ['direct-grant-validate-username', 'direct-grant-validate-password',
    'flow:eduworld-portal-deny-non-staff'])) {
    p.push('flow eduworld-portal-direct-grant: username → password → deny non-staff, nothing else (no OTP, ADR-0094)');
  }
  const configs = Object.fromEntries((realm.authenticatorConfig ?? []).map((c) => [c.alias, c.config]));
  const used = new Map();
  for (const f of realm.authenticationFlows ?? []) {
    for (const e of f.authenticationExecutions ?? []) {
      if ((e.authenticator ?? '').includes('webauthn')) p.push(`flow ${f.alias}: WebAuthn is out of scope (ADR-0091)`);
      if (e.authenticatorConfig) {
        if (used.has(e.authenticatorConfig)) p.push(`authenticator config ${e.authenticatorConfig} shared by two executions (Keycloak keeps one)`);
        used.set(e.authenticatorConfig, true);
        if (!configs[e.authenticatorConfig]) p.push(`authenticator config ${e.authenticatorConfig} not declared`);
      }
      if (e.authenticator === 'direct-grant-validate-password' && configs[e.authenticatorConfig]?.['default.reference.value'] !== 'pwd') {
        p.push(`flow ${f.alias}: password needs reference value pwd (amr)`);
      }
      if (/otp/i.test(`${e.authenticator ?? ''} ${e.flowAlias ?? ''}`)) p.push(`flow ${f.alias}: no OTP step (ADR-0094)`);
    }
  }
  const denies = (alias, role, msg) => {
    const ex = flows[alias]?.authenticationExecutions ?? [];
    const conds = ex.filter((e) => e.authenticator === 'conditional-user-role').map((e) => configs[e.authenticatorConfig]);
    const deny = ex.find((e) => e.authenticator === 'deny-access-authenticator');
    return conds.every((c) => c?.negate === 'true') && conds.some((c) => c?.condUserRole === role) && configs[deny?.authenticatorConfig]?.denyErrorMessage === msg;
  };
  if (!denies('eduworld-game-deny-non-student', 'student', 'gameStaffDenied')) p.push('game flow must deny users without role student');
  if (!denies('eduworld-portal-deny-non-staff', 'tenant_staff', 'portalStudentDenied') || !denies('eduworld-portal-deny-non-staff', 'platform_staff', 'portalStudentDenied')) {
    p.push('portal flow must deny users without tenant_staff or platform_staff');
  }
  // ADR-0094: no second factor, so no AMR/LoA claims anywhere (nothing may rely on amr/acr).
  for (const s of realm.clientScopes ?? []) {
    for (const m of s.protocolMappers ?? []) {
      if (m.protocolMapper === 'oidc-amr-mapper' || m.protocolMapper === 'oidc-acr-mapper') p.push(`client scope ${s.name}: no AMR/LoA mapper (ADR-0094)`);
    }
  }
  for (const c of clients) {
    const scopes = [...(c.defaultClientScopes ?? []), ...(c.optionalClientScopes ?? [])];
    for (const s of ['eduworld-amr-loa', 'acr']) if (scopes.includes(s)) p.push(`client ${c.clientId}: scope ${s} must not be assigned (ADR-0094)`);
  }
  for (const id of DIRECT_GRANT) {
    const c = clients.find((x) => x.clientId === id);
    for (const s of ['eduworld-api-audience', 'eduworld-roles']) if (c && !(c.defaultClientScopes ?? []).includes(s)) p.push(`${id} needs scope ${s}`);
  }
  if (realm.loginTheme !== 'eduworld-ops') p.push('loginTheme must be eduworld-ops (action-token pages only)');
  const up = JSON.parse(realm.components?.['org.keycloak.userprofile.UserProfileProvider']?.[0]?.config?.['kc.user.profile.config']?.[0] ?? '{}');
  for (const a of up.attributes ?? []) if (a.required && a.name !== 'username') p.push(`user profile: ${a.name} must not be required (spike R5)`);
  return p;
}

// checkDev returns the problems of the local dev data file (only users/organizations, fake data).
export function checkDev(dev) {
  const p = [];
  for (const k of Object.keys(dev)) if (!['realm', 'users', 'organizations'].includes(k)) p.push(`dev file: unexpected key ${k}`);
  for (const u of dev.users ?? []) {
    if (u.email && !u.email.endsWith('@example.invalid')) p.push(`dev user ${u.username}: email must be @example.invalid`);
  }
  return p;
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  const realm = JSON.parse(readFileSync(new URL('../realm-eduworld.json', import.meta.url), 'utf8'));
  const dev = JSON.parse(readFileSync(new URL('../realm-eduworld.dev.json', import.meta.url), 'utf8'));
  const problems = [...check(realm), ...checkDev(dev)];
  for (const x of problems) console.error(x);
  console.log(`check-realm: ${problems.length} problem(s)`);
  process.exit(problems.length ? 1 : 0);
}
