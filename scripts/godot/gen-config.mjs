#!/usr/bin/env node
// Writes the client environment file src/game/config/env/<env>.json from the variables/secrets of the GitHub
// Environment (F133 T003, F133-AC-01, F133-AC-02, GODOT_ARCHITECTURE §4, §11, AGENTS R3/R4). Never hand-edit a
// generated file for a release; the committed files only hold placeholder URLs for local and PR builds.
//
//   node scripts/godot/gen-config.mjs --env=<local|dev|staging|production> [--out=<file>] [--keep-urls]
//
// Inputs (environment variables; URLs are Environment *variables*, the public key may be a variable or a secret):
//   EW_GAME_API_BASE_URL, EW_GAME_REALTIME_URL, EW_GAME_CDN_BASE_URL      required (unless --keep-urls)
//   EW_GAME_BUNDLES_BASE_URL, EW_GAME_RELEASES_BASE_URL                   optional (ew-game-bundles CDN, play.<domain>/releases)
//   EW_GAME_STORE_URL_ANDROID, EW_GAME_STORE_URL_IOS                      optional (store page of the "Cần cập nhật" screen)
//   EW_GAME_MANIFEST_PUBKEY                                               RSA ≥ 3072 PUBLIC key (PEM) checking the signed
//                                                                         release manifest / bundle index; required for
//                                                                         staging/production
//   EW_GAME_CLIENT_VERSION or GITHUB_REF=refs/tags/client/vX.Y.Z          client_version (else 0.0.0-ci.<run>)
//   GITHUB_RUN_NUMBER (or EW_GAME_BUILD_NUMBER)                           build_number
// --keep-urls keeps the URLs of the existing output file (PR/main builds have no Environment) and only stamps the
// version and the build number. Staging/production only accept https/wss URLs (no localhost, no Prism mock port, no
// query string, ≤ 2048 characters). A private key, a Keycloak token endpoint or any secret-like key is refused.
// Nothing secret is printed: the log names keys, never values.
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

export const ENVS = ['local', 'dev', 'staging', 'production'];
const RELEASE_ENVS = ['staging', 'production'];
const SEMVER = /^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$/;
const LOG_LEVEL = { local: 'debug', dev: 'debug', staging: 'info', production: 'warn' };
const MAX_URL = 2048;
const MAX_PEM = 4096;

const fail = (msg) => { throw new Error(`gen-config: ${msg}`); };

/** client_version from EW_GAME_CLIENT_VERSION, a `client/vX.Y.Z` tag, or `0.0.0-ci.<run>`. */
export function versionFromRef(vars) {
  let v = vars.EW_GAME_CLIENT_VERSION ?? '';
  const ref = vars.GITHUB_REF ?? '';
  if (!v && ref.startsWith('refs/tags/')) {
    const tag = ref.slice('refs/tags/'.length);
    if (!tag.startsWith('client/v')) fail(`tag ${tag} is not a client/v* tag`);
    v = tag.slice('client/v'.length);
  }
  if (!v) v = `0.0.0-ci.${Number(vars.GITHUB_RUN_NUMBER ?? vars.EW_GAME_BUILD_NUMBER ?? 0) || 0}`;
  if (!SEMVER.test(v) || v.length > 32) fail(`client_version "${v}" is not SemVer X.Y.Z[-pre] (≤ 32)`);
  return v;
}

function checkUrl(name, value, env, schemes) {
  if (typeof value !== 'string' || value.length === 0) fail(`${name} is required`);
  if (value.length > MAX_URL) fail(`${name} is longer than ${MAX_URL} characters`);
  let u;
  try { u = new URL(value); } catch { fail(`${name} is not a URL`); }
  if (/\/protocol\/openid-connect\//.test(u.pathname)) fail(`${name} must not be a Keycloak openid-connect endpoint (ADR-0068)`);
  if (u.search || u.hash || u.username || u.password) fail(`${name} must not carry a query string, fragment or credentials`);
  const release = RELEASE_ENVS.includes(env);
  const allowed = release ? schemes.filter((s) => s === 'https:' || s === 'wss:') : schemes;
  if (!allowed.includes(u.protocol)) fail(`${name} must use ${allowed.map((s) => s.replace(':', '')).join('/')} in ${env}`);
  if (release && /^(localhost|127\.|0\.0\.0\.0|\[::1\])/.test(u.hostname)) fail(`${name} must not point to localhost in ${env}`);
  if (u.port === '4011') fail(`${name} points to the Prism mock port 4011`);
  return value;
}

function checkPublicKey(pem) {
  if (pem.length > MAX_PEM) fail('EW_GAME_MANIFEST_PUBKEY is too long');
  if (!/^-----BEGIN PUBLIC KEY-----\r?\n[\s\S]+\r?\n-----END PUBLIC KEY-----\s*$/.test(pem) || /PRIVATE/.test(pem)) {
    fail('EW_GAME_MANIFEST_PUBKEY must be a PEM "BEGIN PUBLIC KEY" (never a private key, AGENTS R4)');
  }
  let key;
  try { key = crypto.createPublicKey(pem); } catch { fail('EW_GAME_MANIFEST_PUBKEY is not a valid public key'); }
  const bits = key.asymmetricKeyDetails?.modulusLength ?? 0;
  if (key.asymmetricKeyType !== 'rsa' || bits < 3072) fail('EW_GAME_MANIFEST_PUBKEY must be RSA with ≥ 3072 bits (research R2)');
  return pem.trim() + '\n';
}

/** Builds the config object. `existing` = current file (for --keep-urls). Throws on any problem. */
export function buildConfig(env, vars, existing = {}, { keepUrls = false, requireKey = RELEASE_ENVS.includes(env) } = {}) {
  if (!ENVS.includes(env)) fail(`--env must be one of ${ENVS.join(', ')} (got "${env}")`);
  const pick = (varName, key) => {
    const v = vars[varName];
    if ((v === undefined || v === '') && keepUrls && existing[key]) return existing[key];
    if (v === undefined || v === '') fail(`${varName} is required for ${key}`);
    return v;
  };
  const cfg = {
    env,
    api_base_url: checkUrl('EW_GAME_API_BASE_URL', pick('EW_GAME_API_BASE_URL', 'api_base_url'), env, ['https:', 'http:']),
    realtime_url: checkUrl('EW_GAME_REALTIME_URL', pick('EW_GAME_REALTIME_URL', 'realtime_url'), env, ['wss:', 'ws:']),
    cdn_base_url: checkUrl('EW_GAME_CDN_BASE_URL', pick('EW_GAME_CDN_BASE_URL', 'cdn_base_url'), env, ['https:', 'http:']),
  };
  for (const [varName, key] of [['EW_GAME_BUNDLES_BASE_URL', 'bundles_base_url'], ['EW_GAME_RELEASES_BASE_URL', 'releases_base_url']]) {
    const v = vars[varName] || (keepUrls ? existing[key] : '');
    if (v) cfg[key] = checkUrl(varName, v, env, ['https:', 'http:']);
  }
  const stores = {};
  for (const [varName, platform] of [['EW_GAME_STORE_URL_ANDROID', 'android'], ['EW_GAME_STORE_URL_IOS', 'ios']]) {
    if (vars[varName]) {
      // Store pages carry their app id in the query string (play.google.com/…?id=), so only the scheme/length rules apply.
      if (vars[varName].length > MAX_URL) fail(`${varName} is longer than ${MAX_URL} characters`);
      if (!vars[varName].startsWith('https://')) fail(`${varName} must use https`);
      stores[platform] = vars[varName];
    }
  }
  if (Object.keys(stores).length === 0 && keepUrls && existing.store_urls) Object.assign(stores, existing.store_urls);
  cfg.login_path = '/auth/game/login';
  cfg.client_version = versionFromRef(vars);
  const build = Number(vars.GITHUB_RUN_NUMBER ?? vars.EW_GAME_BUILD_NUMBER ?? 0);
  if (!Number.isInteger(build) || build < 0 || build > 2147483647) fail('build number must be an integer');
  if (build > 0) cfg.build_number = build;
  if (Object.keys(stores).length > 0) cfg.store_urls = stores;
  const pem = vars.EW_GAME_MANIFEST_PUBKEY || (keepUrls ? existing.manifest_public_key ?? '' : '');
  if (pem) cfg.manifest_public_key = checkPublicKey(pem);
  else if (requireKey) fail(`EW_GAME_MANIFEST_PUBKEY is required in ${env}`);
  cfg.log_level = LOG_LEVEL[env];
  cfg.feature_flags_default = {};
  return cfg;
}

function main(argv) {
  const args = Object.fromEntries(argv.map((a) => { const m = a.match(/^--([a-z-]+)(?:=(.*))?$/); return m ? [m[1], m[2] ?? true] : [a, true]; }));
  const env = String(args.env ?? '');
  const here = path.dirname(fileURLToPath(import.meta.url));
  const out = String(args.out || path.join(here, '..', '..', 'src', 'game', 'config', 'env', `${env}.json`));
  const keepUrls = args['keep-urls'] === true;
  const existing = keepUrls && fs.existsSync(out) ? JSON.parse(fs.readFileSync(out, 'utf8')) : {};
  const cfg = buildConfig(env, process.env, existing, { keepUrls, requireKey: RELEASE_ENVS.includes(env) && !keepUrls });
  fs.mkdirSync(path.dirname(out), { recursive: true });
  fs.writeFileSync(out, JSON.stringify(cfg, null, 2) + '\n');
  console.log(`gen-config: ${out} env=${env} client_version=${cfg.client_version} build=${cfg.build_number ?? '-'} keys=${Object.keys(cfg).join(',')}`);
}

if (process.argv[1] && fileURLToPath(import.meta.url) === path.resolve(process.argv[1])) {
  try { main(process.argv.slice(2)); } catch (e) { console.error(e.message); process.exit(1); }
}
