// Tests of gen-config.mjs (F133 T003, F133-AC-01, F133-AC-02, AGENTS R3/R4). Run: node --test scripts/godot/
import { test } from 'node:test';
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { buildConfig, versionFromRef } from '../gen-config.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const pub3072 = crypto.generateKeyPairSync('rsa', { modulusLength: 3072 }).publicKey.export({ type: 'spki', format: 'pem' });
const vars = {
  EW_GAME_API_BASE_URL: 'https://api.staging.eduworld.example/api/v1',
  EW_GAME_REALTIME_URL: 'wss://api.staging.eduworld.example/realtime/v1',
  EW_GAME_CDN_BASE_URL: 'https://cdn.staging.eduworld.example',
  EW_GAME_BUNDLES_BASE_URL: 'https://bundles.staging.eduworld.example',
  EW_GAME_RELEASES_BASE_URL: 'https://play.staging.eduworld.example/releases',
  EW_GAME_STORE_URL_ANDROID: 'https://play.google.com/store/apps/details?id=vn.eduworld.game',
  EW_GAME_MANIFEST_PUBKEY: pub3072,
  GITHUB_REF: 'refs/tags/client/v0.9.0-rc1',
  GITHUB_RUN_NUMBER: '412',
};

test('a tag build embeds client_version X.Y.Z and the run number as build_number (F133-AC-01)', () => {
  const cfg = buildConfig('staging', vars);
  assert.equal(cfg.client_version, '0.9.0-rc1');
  assert.equal(cfg.build_number, 412);
  assert.equal(cfg.env, 'staging');
  assert.equal(cfg.login_path, '/auth/game/login');
  assert.equal(cfg.api_base_url, vars.EW_GAME_API_BASE_URL);
  assert.equal(cfg.store_urls.android, vars.EW_GAME_STORE_URL_ANDROID);
  assert.ok(!('ios' in cfg.store_urls));
  assert.match(cfg.manifest_public_key, /^-----BEGIN PUBLIC KEY-----/);
});

test('version comes from the tag, else EW_GAME_CLIENT_VERSION, else 0.0.0-ci.<run>', () => {
  assert.equal(versionFromRef({ GITHUB_REF: 'refs/tags/client/v1.2.3' }), '1.2.3');
  assert.equal(versionFromRef({ GITHUB_REF: 'refs/heads/main', GITHUB_RUN_NUMBER: '7' }), '0.0.0-ci.7');
  assert.equal(versionFromRef({ EW_GAME_CLIENT_VERSION: '2.0.0', GITHUB_REF: 'refs/heads/x' }), '2.0.0');
  assert.throws(() => versionFromRef({ GITHUB_REF: 'refs/tags/client/v1.2' }), /client_version/);
  assert.throws(() => versionFromRef({ GITHUB_REF: 'refs/tags/backend/v1.2.3' }), /client\/v/);
});

test('staging/production refuse http, localhost, the mock port and a missing public key (R3)', () => {
  assert.throws(() => buildConfig('production', { ...vars, EW_GAME_API_BASE_URL: 'http://api.eduworld.example/api/v1' }), /https/);
  assert.throws(() => buildConfig('production', { ...vars, EW_GAME_REALTIME_URL: 'ws://api.eduworld.example/rt' }), /wss/);
  assert.throws(() => buildConfig('staging', { ...vars, EW_GAME_CDN_BASE_URL: 'https://localhost:8443' }), /localhost/);
  assert.throws(() => buildConfig('staging', { ...vars, EW_GAME_API_BASE_URL: 'https://x.example:4011/api/v1' }), /mock/);
  const { EW_GAME_MANIFEST_PUBKEY, ...noKey } = vars;
  assert.throws(() => buildConfig('production', noKey), /EW_GAME_MANIFEST_PUBKEY/);
  assert.throws(() => buildConfig('production', { ...vars, EW_GAME_API_BASE_URL: '' }), /EW_GAME_API_BASE_URL/);
});

test('a private key, a short RSA key, a token endpoint or a query secret is refused (F133-AC-02, R4)', () => {
  const priv = crypto.generateKeyPairSync('rsa', { modulusLength: 2048 });
  const privPem = priv.privateKey.export({ type: 'pkcs8', format: 'pem' });
  assert.throws(() => buildConfig('staging', { ...vars, EW_GAME_MANIFEST_PUBKEY: privPem }), /PUBLIC KEY/);
  const short = priv.publicKey.export({ type: 'spki', format: 'pem' });
  assert.throws(() => buildConfig('staging', { ...vars, EW_GAME_MANIFEST_PUBKEY: short }), /3072/);
  assert.throws(() => buildConfig('staging', {
    ...vars, EW_GAME_API_BASE_URL: 'https://auth.example/realms/x/protocol/openid-connect/token' }), /openid-connect/);
  assert.throws(() => buildConfig('staging', { ...vars, EW_GAME_CDN_BASE_URL: 'https://cdn.example/?X-Amz-Signature=abc' }), /query/);
  assert.throws(() => buildConfig('staging', { ...vars, EW_GAME_STORE_URL_IOS: 'http://apps.apple.com/app/1' }), /https/);
  assert.throws(() => buildConfig('staging', { ...vars, EW_GAME_API_BASE_URL: `https://a.example/${'x'.repeat(2050)}` }), /2048/);
});

test('local/dev allow http(s) URLs of the developer stack and no key', () => {
  const cfg = buildConfig('local', {
    EW_GAME_API_BASE_URL: 'http://localhost:9080/api/v1', EW_GAME_REALTIME_URL: 'ws://localhost:9080/realtime/v1',
    EW_GAME_CDN_BASE_URL: 'http://localhost:9000', EW_GAME_CLIENT_VERSION: '0.1.0' });
  assert.equal(cfg.log_level, 'debug');
  assert.ok(!('manifest_public_key' in cfg));
  assert.throws(() => buildConfig('mock', vars), /env/);
});

test('--keep-urls reuses the committed URLs and only stamps version/build (PR builds without Environment)', () => {
  const existing = { env: 'production', api_base_url: 'https://api.eduworld.example/api/v1',
    realtime_url: 'wss://api.eduworld.example/realtime/v1', cdn_base_url: 'https://cdn.eduworld.example',
    login_path: '/auth/game/login', client_version: '0.0.0-dev', log_level: 'warn', feature_flags_default: {} };
  const cfg = buildConfig('production', { GITHUB_REF: 'refs/pull/9/merge', GITHUB_RUN_NUMBER: '55' }, existing, { keepUrls: true, requireKey: false });
  assert.equal(cfg.api_base_url, existing.api_base_url);
  assert.equal(cfg.client_version, '0.0.0-ci.55');
  assert.equal(cfg.build_number, 55);
});

test('CLI writes src/game-style JSON with only allowed keys and never prints values of the key', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'gencfg-'));
  const out = path.join(dir, 'staging.json');
  const r = spawnSync(process.execPath, [path.join(here, '..', 'gen-config.mjs'), '--env=staging', `--out=${out}`],
    { env: { ...process.env, ...vars }, encoding: 'utf8' });
  assert.equal(r.status, 0, r.stderr);
  const cfg = JSON.parse(fs.readFileSync(out, 'utf8'));
  assert.deepEqual(Object.keys(cfg).sort(), ['api_base_url', 'build_number', 'bundles_base_url', 'cdn_base_url',
    'client_version', 'env', 'feature_flags_default', 'log_level', 'login_path', 'manifest_public_key',
    'realtime_url', 'releases_base_url', 'store_urls'].sort());
  assert.doesNotMatch(r.stdout + r.stderr, /BEGIN PUBLIC KEY/);
  const bad = spawnSync(process.execPath, [path.join(here, '..', 'gen-config.mjs'), '--env=windows', `--out=${out}`],
    { env: { ...process.env, ...vars }, encoding: 'utf8' });
  assert.notEqual(bad.status, 0);
  fs.rmSync(dir, { recursive: true, force: true });
});
