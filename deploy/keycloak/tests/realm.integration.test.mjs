// F004 T008/T009/T010/T040 — realm behaviour on a running Keycloak (compose) with the dev data.
// KC_INTEGRATION=1 KC_ADMIN_PASSWORD=… node --test deploy/keycloak/tests/realm.integration.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { admin, adminToken, passwordGrant, decodeJWT, KC_URL, REALM } from '../scripts/kc.mjs';
import { DEMO_PASSWORD as P } from '../scripts/dev-constants.mjs';

const S = process.env.KC_LOCAL_SECRET ?? 'change-me-local';
const run = process.env.KC_INTEGRATION === '1' ? test : test.skip;

run('F004-AC-02: direct grant only through the two confidential clients with their secret', async () => {
  assert.equal((await passwordGrant('eduworld-game-backend', S, 'hs.an.3a', P)).status, 200);
  assert.equal((await passwordGrant('eduworld-portal', S, 'ad.truong.a', P)).status, 200);
  const sis = await passwordGrant('eduworld-sis-sync', S, 'hs.an.3a', P);
  assert.equal(sis.json.error, 'unauthorized_client');
  const wrong = await passwordGrant('eduworld-game-backend', 'wrong', 'hs.an.3a', P);
  assert.equal(wrong.json.error, 'unauthorized_client');
  const none = await passwordGrant('eduworld-portal', undefined, 'ad.truong.a', P);
  assert.notEqual(none.status, 200);
});

run('F004-AC-03/25: only students in the game, only staff in the portal; denial differs from a wrong password', async () => {
  const staffInGame = await fetch(`${KC_URL}/realms/${REALM}/protocol/openid-connect/token`, { method: 'POST',
    body: new URLSearchParams({ grant_type: 'password', client_id: 'eduworld-game-backend', client_secret: S, username: 'ad.donvi', password: P }) });
  assert.equal(staffInGame.status, 401);
  assert.match(staffInGame.headers.get('content-type'), /text\/html/);
  assert.match(await staffInGame.text(), /không dùng để vào game/);
  const studentInPortal = await fetch(`${KC_URL}/realms/${REALM}/protocol/openid-connect/token`, { method: 'POST',
    body: new URLSearchParams({ grant_type: 'password', client_id: 'eduworld-portal', client_secret: S, username: 'hs.binh.3a', password: P }) });
  assert.equal(studentInPortal.status, 401);
  assert.match(await studentInPortal.text(), /không dùng để đăng nhập trang quản trị/);
  // Module identity sends Accept: application/json; Keycloak 26.4 then answers the deny step with JSON whose
  // "error" is the localized message, not an OAuth code (F012 Clarifications 2026-11-21) → PORTAL_ACCESS_STAFF_ONLY.
  const studentJSON = await fetch(`${KC_URL}/realms/${REALM}/protocol/openid-connect/token`, { method: 'POST',
    headers: { Accept: 'application/json' },
    body: new URLSearchParams({ grant_type: 'password', client_id: 'eduworld-portal', client_secret: S, username: 'hs.binh.3a', password: P }) });
  assert.equal(studentJSON.status, 401);
  const denied = await studentJSON.json();
  assert.match(denied.error, /không dùng để đăng nhập trang quản trị/);
  assert.doesNotMatch(denied.error, /^[a-z_]+$/);
  const wrong = await passwordGrant('eduworld-portal', S, 'ad.truong.b', 'wrong-password');
  assert.equal(wrong.status, 401);
  assert.equal(wrong.json.error, 'invalid_grant');
  const setup = await passwordGrant('eduworld-portal', S, 'sa.moi', P);
  assert.equal(setup.status, 400);
  assert.match(setup.json.error_description, /not fully set up/);
});

run('ADR-0094: platform staff sign in with the password alone; the portal flow and tokens carry no OTP/LoA', async () => {
  const ok = await passwordGrant('eduworld-portal', S, 'sa.admin', P);
  assert.equal(ok.status, 200, JSON.stringify(ok.json));
  const t = decodeJWT(ok.json.access_token);
  assert.ok(t.roles.includes('platform_staff'));
  assert.ok(!(t.amr ?? []).includes('otp')); // no OTP step, no AMR/LoA mapper on eduworld-portal any more
  assert.ok(t.iat > 0); // direct grant emits no auth_time (spike R5): module identity bounds the platform session
  const token = await adminToken();
  const ex = (await admin('/authentication/flows/eduworld-portal-direct-grant/executions', { token })).json;
  assert.ok(!ex.some((e) => /otp/i.test(`${e.providerId ?? ''} ${e.displayName ?? ''}`)), JSON.stringify(ex));
  const totp = (await admin('/authentication/required-actions/CONFIGURE_TOTP', { token })).json;
  assert.equal(totp.enabled, false);
});

run('F004-AC-05: audience, flat realm roles, no organization claim, no personal data, no offline_access', async () => {
  for (const [client, user] of [['eduworld-game-backend', 'hs.chi.3b'], ['eduworld-portal', 'qlcm.truong.a']]) {
    const t = decodeJWT((await passwordGrant(client, S, user, P)).json.access_token);
    assert.equal(t.azp, client);
    assert.ok([].concat(t.aud).includes('eduworld-api'));
    assert.equal(t.typ, 'Bearer');
    assert.ok(Array.isArray(t.roles) && !t.roles.includes('offline_access') && !t.roles.includes('uma_authorization'));
    for (const k of ['organization', 'email', 'given_name', 'family_name', 'name', 'birthdate']) assert.equal(t[k], undefined, `${client}: claim ${k}`);
    assert.equal(t.preferred_username, user);
  }
});

run('F004-AC-04: svc-identity is the only backend service account with minimal realm-management roles', async () => {
  const token = await adminToken();
  const clients = (await admin('/clients?max=100', { token })).json;
  const svc = clients.filter((c) => c.serviceAccountsEnabled).map((c) => c.clientId).sort();
  assert.deepEqual(svc, ['eduworld-sis-sync', 'svc-identity']);
  assert.ok(!clients.some((c) => /^svc-(learning|game|competition|chat|realtime)$/.test(c.clientId)));
  const id = clients.find((c) => c.clientId === 'svc-identity').id;
  const sa = (await admin(`/clients/${id}/service-account-user`, { token })).json;
  const rm = clients.find((c) => c.clientId === 'realm-management').id;
  const roles = (await admin(`/users/${sa.id}/role-mappings/clients/${rm}`, { token })).json.map((r) => r.name).sort();
  assert.deepEqual(roles, ['manage-realm', 'manage-users', 'query-users', 'view-users']);
  const cc = await fetch(`${KC_URL}/realms/${REALM}/protocol/openid-connect/token`, { method: 'POST',
    body: new URLSearchParams({ grant_type: 'client_credentials', client_id: 'svc-identity', client_secret: S }) });
  assert.equal(cc.status, 200);
});

run('F004-AC-06/07: realm settings; applying twice changes nothing; dev organizations exist', async () => {
  const token = await adminToken();
  const r = (await admin('', { token })).json;
  assert.equal(r.registrationAllowed, false);
  assert.equal(r.duplicateEmailsAllowed, false);
  assert.equal(r.loginWithEmailAllowed, true);
  assert.equal(r.editUsernameAllowed, false);
  assert.equal(r.bruteForceProtected, true);
  assert.equal(r.permanentLockout, false);
  assert.equal(r.accessTokenLifespan, 300);
  const snapshot = async () => {
    const t = await adminToken();
    const realm = (await admin('', { token: t })).json;
    const clients = (await admin('/clients?max=100', { token: t })).json.map(({ id, ...c }) => c);
    const flows = (await admin('/authentication/flows', { token: t })).json.map(({ id, authenticationExecutions, ...f }) => f);
    delete realm.id;
    return JSON.stringify({ realm, clients, flows });
  };
  const before = await snapshot();
  execFileSync('bash', ['deploy/keycloak/scripts/apply.sh'], { stdio: 'pipe', env: process.env });
  assert.equal(await snapshot(), before, 'second apply changed the realm');
  const orgs = (await admin('/organizations?max=100', { token: await adminToken() })).json.map((o) => o.alias).sort();
  for (const a of ['common', 'pgd-demo', 'th-demo-a']) assert.ok(orgs.includes(a), `organization ${a}`);
});
