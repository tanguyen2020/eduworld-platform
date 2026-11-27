#!/usr/bin/env node
// Packs one school APK kit (F133 T039, F133-AC-17, PERFORMANCE_BUDGET §10.1, ADR-0101) — called by
// deploy/installer/school-kit/build-kit.sh:
//   node school-kit.mjs <base.apk> <index.json> <index.sig> <bundles_dir> <bands csv> <out.apk> <packs_out.json> [previous packs.json]
// Copies every entry of the base APK byte for byte except the v1 signature files (META-INF/*.SF|*.RSA|*.EC|MANIFEST.MF —
// the kit is signed again by apksigner), and adds assets/preinstalled/<sha256>.pck (stored, 4-byte aligned) for each
// public zone pack of the signed bundle index whose grade bands meet the kit bands, plus the index and its signature
// (assets/preinstalled/index.json[.sig]) so the device can check the packs offline. On the device these are
// res://preinstalled/… (DeviceConfig.DEFAULT_PREINSTALLED_DIR), used in place by ContentDownloader.
// Each pack must match its SHA-256 and must not hold tenant data, a secret or question/answer-key content.
// Writes <packs_out.json> {bands, packs:[{bundle_id, sha256, size_bytes}]} and prints the delta against [previous]
// (packs are content-addressed: a device keeps an unchanged pack, only new hashes are new bytes).
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import { readZip, writeZip } from './zip.mjs';

const fail = (m) => { console.error(`build-kit: ${m}`); process.exit(1); };
const [apkFile, indexFile, sigFile, bundlesDir, bandsCsv, outApk, packsOut, previous] = process.argv.slice(2);
const bands = bandsCsv.split(',').filter(Boolean);
// Content that must never be in a public pack (F133-AC-17, AGENTS R6/R31): tenant data, secrets, question/answer keys.
const FORBIDDEN = [
  /"tenant_id"\s*:/, /X-Tenant-Id/i, /-----BEGIN ([A-Z0-9]+ )?PRIVATE KEY-----/, /"client_secret"\s*:/,
  /"(answer_key|scoring_rule|accepted_answers|correct_option_ids?)"\s*:/, /eduworld-game-backend/,
];

const index = JSON.parse(fs.readFileSync(indexFile, 'utf8'));
const wanted = index.bundles.filter((b) => b.grade_bands.some((g) => bands.includes(g)));
const added = [];
const packs = [];
for (const b of wanted) {
  const file = path.join(bundlesDir, `${b.sha256}.pck`);
  if (!fs.existsSync(file)) fail(`pack ${b.bundle_id} (${b.sha256}.pck) not found in ${bundlesDir}`);
  const data = fs.readFileSync(file);
  if (crypto.createHash('sha256').update(data).digest('hex') !== b.sha256) fail(`pack ${b.bundle_id}: SHA-256 does not match the index`);
  const text = data.toString('latin1');
  for (const re of FORBIDDEN) if (re.test(text)) fail(`pack ${b.bundle_id} holds forbidden content (${re.source})`);
  added.push({ name: `assets/preinstalled/${b.sha256}.pck`, data });
  packs.push({ bundle_id: b.bundle_id, sha256: b.sha256, size_bytes: data.length });
}
added.push({ name: 'assets/preinstalled/index.json', data: fs.readFileSync(indexFile) });
added.push({ name: 'assets/preinstalled/index.json.sig', data: fs.readFileSync(sigFile) });

const base = readZip(fs.readFileSync(apkFile));
if (!base.some((e) => e.name === 'AndroidManifest.xml')) fail(`${apkFile} is not an APK (no AndroidManifest.xml)`);
const kept = base.filter((e) => !/^META-INF\/(.*\.(SF|RSA|EC|DSA)|MANIFEST\.MF)$/.test(e.name) && !e.name.startsWith('assets/preinstalled/'))
  .map((e) => ({ name: e.name, raw: e.raw }));
fs.writeFileSync(outApk, writeZip([...kept, ...added]));
const result = { bands, packs };
fs.writeFileSync(packsOut, JSON.stringify(result, null, 2) + '\n');

let delta = `${packs.length} pack(s)`;
if (previous && fs.existsSync(previous)) {
  const old = new Set(JSON.parse(fs.readFileSync(previous, 'utf8')).packs.map((p) => p.sha256));
  const fresh = packs.filter((p) => !old.has(p.sha256));
  delta += `, ${packs.length - fresh.length} unchanged, ${fresh.length} new (${fresh.reduce((s, p) => s + p.size_bytes, 0)} bytes)`;
}
console.log(`build-kit: ${path.basename(outApk)} bands ${bands.join(',')}: ${delta}`);
