#!/usr/bin/env node
// TEST-ONLY (AGENTS R17): writes the release fixtures of the F133 GdUnit tests (ManifestVerifier, Config, ContentDownloader,
// PrefetchScheduler) with the real CI tools (scripts/godot/gen-config.mjs, sign-manifest.mjs), so the client is tested
// against bytes signed by Node exactly like a release. The RSA-3072 key pair is generated in memory: only the PUBLIC key
// and the signatures are written — the private key is discarded (nothing secret is committed, R4). The public key goes to
// `release_public_key.txt`, never `.pem`: the root .gitignore ignores `*.pem`, so a `.pem` fixture is not committed and
// the suites fail on every fresh checkout (fixed 2026-11-28).
//   node src/game/tests/fixtures/release/make_fixtures.mjs
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { buildConfig } from '../../../../../scripts/godot/gen-config.mjs';
import { signBytes, makeReleaseManifest, validateDoc } from '../../../../../scripts/godot/sign-manifest.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const { privateKey, publicKey } = crypto.generateKeyPairSync('rsa', { modulusLength: 3072 });
const priv = privateKey.export({ type: 'pkcs8', format: 'pem' });
const pub = publicKey.export({ type: 'spki', format: 'pem' });
const write = (name, data) => fs.writeFileSync(path.join(here, name), data);
const sign = (name, bytes) => { write(name, bytes); write(`${name}.sig`, signBytes(bytes, priv)); };

write('release_public_key.txt', pub);
const cfg = buildConfig('production', {
  EW_GAME_API_BASE_URL: 'https://api.eduworld.example/api/v1', EW_GAME_REALTIME_URL: 'wss://api.eduworld.example/realtime/v1',
  EW_GAME_CDN_BASE_URL: 'https://cdn.eduworld.example', EW_GAME_BUNDLES_BASE_URL: 'https://bundles.eduworld.example',
  EW_GAME_RELEASES_BASE_URL: 'https://play.eduworld.example/releases',
  EW_GAME_STORE_URL_ANDROID: 'https://play.google.com/store/apps/details?id=vn.eduworld.game',
  EW_GAME_STORE_URL_IOS: 'https://apps.apple.com/app/id0000000000',
  EW_GAME_MANIFEST_PUBKEY: pub, GITHUB_REF: 'refs/tags/client/v1.2.3', GITHUB_RUN_NUMBER: '77' });
write('config_production.json', JSON.stringify(cfg, null, 2) + '\n');

// Two tiny public "zone packs" (not real PCKs: the downloader only moves and hashes bytes).
const packs = { 'zone.math_g3': Buffer.from('EDUWORLD-TEST-PACK math g1_3\n'), 'zone.science_g4': Buffer.from('EDUWORLD-TEST-PACK science g4_5\n') };
const bands = { 'zone.math_g3': ['g1_3'], 'zone.science_g4': ['g4_5'] };
const bundles = Object.entries(packs).map(([id, data]) => {
  const sha256 = crypto.createHash('sha256').update(data).digest('hex');
  write(`${sha256}.pck`, data);
  return { bundle_id: id, version: 1, url: `https://bundles.eduworld.example/bundles/${sha256}.pck`, sha256, size_bytes: data.length, grade_bands: bands[id] };
});
const index = { schema_version: 1, client_version: '1.2.3', generated_at: '2026-11-28T02:00:00Z', bundles };
if (validateDoc('bundle-index', index).length) throw new Error('bundle index fixture invalid');
sign('bundle_index.json', Buffer.from(JSON.stringify(index)));
const manifest = makeReleaseManifest({ platform: 'android', channel: 'production', clientVersion: '1.3.0', build: 90,
  url: 'https://play.eduworld.example/releases/android/1.3.0/eduworld.apk', artifact: Buffer.from('apk'), minOs: 'Android 9',
  releasedAt: '2026-11-28T02:00:00Z' });
sign('release_manifest.json', Buffer.from(JSON.stringify(manifest)));
console.log('make_fixtures: written to', here);
