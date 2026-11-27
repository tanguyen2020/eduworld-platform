#!/usr/bin/env node
// Release manifest / bundle index signing (F133 T013, F133-AC-05, research R2, DEPLOYMENT §9):
// RSA ≥ 3072, PKCS#1 v1.5 SHA-256 over the exact JSON bytes, detached base64 signature in `<file>.sig` — the same
// format publish-index.sh (F134) writes and the client ManifestVerifier checks with Godot Crypto.
//
//   node scripts/godot/sign-manifest.mjs manifest --platform=<android|ios|web> --channel=<staging|production> \
//        --artifact=<file> --url=<https://…> --min-os=<text> --out=<file> [--client-version=X.Y.Z]
//        (version from --client-version, EW_GAME_CLIENT_VERSION or the client/vX.Y.Z tag; build = GITHUB_RUN_NUMBER)
//   node scripts/godot/sign-manifest.mjs sign --in=<file.json> --key=<private.pem> [--pub=<public.pem>]
//   node scripts/godot/sign-manifest.mjs verify --in=<file.json> --pub=<public.pem> [--sig=<file.sig>]
// The private key comes from a file written by the CI secret step (never an argument value, never logged — R4).
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { versionFromRef } from './gen-config.mjs';

const require = createRequire(import.meta.url);
const here = path.dirname(fileURLToPath(import.meta.url));
const repo = path.resolve(here, '..', '..');
const MIN_BITS = 3072;
const PLATFORMS = ['android', 'ios', 'web'];
const CHANNELS = ['staging', 'production'];

function rsaKey(pem, isPrivate) {
  let key;
  try { key = isPrivate ? crypto.createPrivateKey(pem) : crypto.createPublicKey(pem); } catch { throw new Error('sign-manifest: unreadable key'); }
  if (key.asymmetricKeyType !== 'rsa') throw new Error('sign-manifest: the key must be RSA (research R2)');
  if ((key.asymmetricKeyDetails?.modulusLength ?? 0) < MIN_BITS) throw new Error(`sign-manifest: the key must have ≥ ${MIN_BITS} bits`);
  return key;
}

/** Base64 RSA-SHA256 (PKCS#1 v1.5) signature of `bytes`. */
export function signBytes(bytes, privatePem) {
  const key = rsaKey(privatePem, true);
  return crypto.sign('sha256', bytes, { key, padding: crypto.constants.RSA_PKCS1_PADDING }).toString('base64');
}

export function verifyBytes(bytes, sigB64, publicPem) {
  const key = rsaKey(publicPem, false);
  const sig = Buffer.from(String(sigB64).trim(), 'base64');
  return sig.length > 0 && crypto.verify('sha256', bytes, { key, padding: crypto.constants.RSA_PKCS1_PADDING }, sig);
}

let ajv = null;
/** Errors of `doc` against specs/schemas/<name>.schema.json ([] = valid). */
export function validateDoc(name, doc) {
  if (!ajv) {
    const Ajv = require('ajv/dist/2020').default;
    const addFormats = require('ajv-formats').default;
    ajv = new Ajv({ strict: true, allErrors: true });
    addFormats(ajv);
  }
  const file = path.join(repo, 'specs', 'schemas', `${name}.schema.json`);
  const schema = JSON.parse(fs.readFileSync(file, 'utf8'));
  const validate = ajv.getSchema(schema.$id) ?? ajv.compile(schema);
  return validate(doc) ? [] : validate.errors.map((e) => `${e.instancePath || '/'} ${e.message}`);
}

/** Release manifest of one artifact (data-model §1, client-release-manifest.schema.json). */
export function makeReleaseManifest({ platform, channel, clientVersion, build, url, artifact, minOs, releasedAt }) {
  if (!PLATFORMS.includes(platform)) throw new Error(`sign-manifest: platform must be one of ${PLATFORMS.join(', ')} (ADR-0101)`);
  if (!CHANNELS.includes(channel)) throw new Error('sign-manifest: channel must be staging or production');
  const m = {
    schema_version: 1, platform, channel, client_version: clientVersion, build, url,
    sha256: crypto.createHash('sha256').update(artifact).digest('hex'), size_bytes: artifact.length,
    min_os: minOs, released_at: releasedAt,
  };
  const errors = validateDoc('client-release-manifest', m);
  if (errors.length) throw new Error(`sign-manifest: manifest invalid: ${errors.join('; ')}`);
  return m;
}

function main(argv) {
  const [cmd, ...rest] = argv;
  const a = Object.fromEntries(rest.map((x) => { const m = x.match(/^--([a-z-]+)=(.*)$/); if (!m) throw new Error(`sign-manifest: bad argument ${x}`); return [m[1], m[2]]; }));
  const need = (k) => { if (!a[k]) throw new Error(`sign-manifest: --${k} is required`); return a[k]; };
  if (cmd === 'manifest') {
    const artifact = fs.readFileSync(need('artifact'));
    const vars = { ...process.env, ...(a['client-version'] ? { EW_GAME_CLIENT_VERSION: a['client-version'] } : {}) };
    const m = makeReleaseManifest({ platform: need('platform'), channel: need('channel'), clientVersion: versionFromRef(vars),
      build: Number(process.env.GITHUB_RUN_NUMBER || 1), url: need('url'), artifact, minOs: need('min-os'),
      releasedAt: process.env.EW_RELEASED_AT || new Date().toISOString().replace(/\.\d{3}Z$/, 'Z') });
    fs.mkdirSync(path.dirname(need('out')), { recursive: true });
    fs.writeFileSync(a.out, JSON.stringify(m));
    console.log(`sign-manifest: ${a.out} ${m.platform}/${m.channel} ${m.client_version} build ${m.build} sha256 ${m.sha256}`);
  } else if (cmd === 'sign') {
    const bytes = fs.readFileSync(need('in'));
    const sig = signBytes(bytes, fs.readFileSync(need('key'), 'utf8'));
    if (a.pub && !verifyBytes(bytes, sig, fs.readFileSync(a.pub, 'utf8'))) throw new Error('sign-manifest: signature does not verify with --pub');
    fs.writeFileSync(`${a.in}.sig`, sig);
    console.log(`sign-manifest: ${a.in}.sig written (${bytes.length} bytes signed)`);
  } else if (cmd === 'verify') {
    const bytes = fs.readFileSync(need('in'));
    const sig = fs.readFileSync(a.sig || `${a.in}.sig`, 'utf8');
    if (!verifyBytes(bytes, sig, fs.readFileSync(need('pub'), 'utf8'))) throw new Error(`sign-manifest: ${a.in}: signature does NOT verify`);
    console.log(`sign-manifest: ${a.in}: signature OK`);
  } else {
    throw new Error('usage: sign-manifest.mjs <manifest|sign|verify> --…');
  }
}

if (process.argv[1] && fileURLToPath(import.meta.url) === path.resolve(process.argv[1])) {
  try { main(process.argv.slice(2)); } catch (e) { console.error(e.message); process.exit(1); }
}
