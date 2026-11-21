// node --test deploy/keycloak/scripts/check-realm.test.mjs
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { check, checkDev } from './check-realm.mjs';

const realm = () => JSON.parse(readFileSync(new URL('../realm-eduworld.json', import.meta.url), 'utf8'));
const has = (problems, text) => assert.ok(problems.some((p) => p.includes(text)), `expected "${text}" in ${JSON.stringify(problems)}`);
const client = (r, id) => r.clients.find((c) => c.clientId === id);

test('the committed realm passes', () => {
  assert.deepEqual(check(realm()), []);
  assert.deepEqual(checkDev(JSON.parse(readFileSync(new URL('../realm-eduworld.dev.json', import.meta.url), 'utf8'))), []);
});

test('F004-AC-01/08: real secret, extra client, users in the common file', () => {
  const r = realm();
  client(r, 'eduworld-game-backend').secret = 'p4ss-real-secret';
  r.clients.push({ clientId: 'svc-learning', serviceAccountsEnabled: true });
  r.users.push({ username: 'hs.an.3a', credentials: [{ type: 'password', value: 'x' }] });
  r.clientScopes.push({ name: 'internal-learning' });
  const p = check(r);
  has(p, 'secret must be a $(env:KC_');
  has(p, 'unexpected client svc-learning');
  has(p, 'must not contain users');
  has(p, 'internal-learning');
});

test('F004-AC-02/04: direct grants only for the two confidential clients; svc-identity client credentials only', () => {
  const r = realm();
  client(r, 'eduworld-sis-sync').directAccessGrantsEnabled = true;
  client(r, 'svc-identity').standardFlowEnabled = true;
  const p = check(r);
  has(p, 'eduworld-sis-sync: direct access grants must be off');
  has(p, 'svc-identity: client credentials only');
});

test('F004-AC-24: portal confidential, no redirect, no PKCE, no browser flow', () => {
  const r = realm();
  const c = client(r, 'eduworld-portal');
  c.publicClient = true;
  c.standardFlowEnabled = true;
  c.redirectUris = ['https://portal.example/auth/callback'];
  c.attributes['pkce.code.challenge.method'] = 'S256';
  c.authenticationFlowBindingOverrides.browser = 'eduworld-portal-browser';
  const p = check(r);
  for (const t of ['must be confidential', 'only direct access grants', 'no redirect URI', 'no PKCE', 'no browser flow']) has(p, t);
});

test('F004-AC-05: no organization scope, audience and roles scopes present', () => {
  const r = realm();
  client(r, 'eduworld-portal').defaultClientScopes.push('organization');
  client(r, 'eduworld-game-backend').defaultClientScopes = client(r, 'eduworld-game-backend').defaultClientScopes.filter((s) => s !== 'eduworld-roles');
  const p = check(r);
  has(p, 'scope organization must not be assigned');
  has(p, 'eduworld-game-backend needs scope eduworld-roles');
});

test('F004-AC-03/25/26/27: flow order, deny rules, reference values, no WebAuthn', () => {
  const r = realm();
  const portal = r.authenticationFlows.find((f) => f.alias === 'eduworld-portal-direct-grant');
  portal.authenticationExecutions.reverse().forEach((e, i) => { e.priority = (i + 1) * 10; });
  const game = r.authenticationFlows.find((f) => f.alias === 'eduworld-game-deny-non-student');
  r.authenticatorConfig.find((c) => c.alias === 'ew-game-not-student').config.negate = 'false';
  game.authenticationExecutions.push({ authenticator: 'webauthn-authenticator', requirement: 'REQUIRED', priority: 99 });
  const p = check(r);
  has(p, 'username → password, nothing else');
  has(p, 'deny users without role student');
  has(p, 'WebAuthn is out of scope');
});

test('ADR-0096: the portal flow has no deny step (a denial would count as a brute-force failure)', () => {
  const r = realm();
  const portal = r.authenticationFlows.find((f) => f.alias === 'eduworld-portal-direct-grant');
  portal.authenticationExecutions.push({ flowAlias: 'eduworld-portal-deny-non-staff', requirement: 'CONDITIONAL', priority: 30, authenticatorFlow: true });
  r.authenticationFlows.push({ alias: 'eduworld-portal-deny-non-staff', topLevel: false, authenticationExecutions: [
    { authenticator: 'deny-access-authenticator', requirement: 'REQUIRED', priority: 10 }] });
  const p = check(r);
  has(p, 'username → password, nothing else');
  has(p, 'flow eduworld-portal-deny-non-staff must not exist');
});

test('ADR-0094: no OTP step in any flow, no AMR/LoA mapper or scope', () => {
  const r = realm();
  const portal = r.authenticationFlows.find((f) => f.alias === 'eduworld-portal-direct-grant');
  portal.authenticationExecutions.push({ flowAlias: 'eduworld-portal-otp-configured', requirement: 'CONDITIONAL', priority: 50, authenticatorFlow: true });
  r.authenticationFlows.push({ alias: 'eduworld-portal-otp-configured', authenticationExecutions: [
    { authenticator: 'direct-grant-validate-otp', requirement: 'REQUIRED', priority: 20 }] });
  r.clientScopes.push({ name: 'eduworld-amr-loa', protocolMappers: [{ name: 'amr', protocolMapper: 'oidc-amr-mapper' }] });
  client(r, 'eduworld-portal').defaultClientScopes.push('eduworld-amr-loa');
  const p = check(r);
  has(p, 'nothing else (no OTP, ADR-0094');
  has(p, 'flow eduworld-portal-otp-configured: no OTP step');
  has(p, 'client scope eduworld-amr-loa: no AMR/LoA mapper');
  has(p, 'client eduworld-portal: scope eduworld-amr-loa must not be assigned');
});

test('F004-AC-06: realm settings', () => {
  const r = realm();
  r.registrationAllowed = true;
  r.permanentLockout = true;
  has(check(r), 'registrationAllowed must be false');
  has(check(r), 'permanentLockout must be false');
});

test('dev file: only users and organizations with fake emails', () => {
  const p = checkDev({ realm: 'eduworld', clients: [], users: [{ username: 'x', email: 'real@school.vn' }] });
  has(p, 'unexpected key clients');
  has(p, 'must be @example.invalid');
});
