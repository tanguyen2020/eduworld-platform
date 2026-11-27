#!/usr/bin/env node
// Builds the bundle index of build-bundles.sh (F133 T013, F133-AC-05, specs/schemas/bundle-index.schema.json).
//   node lib/bundle-index.mjs <bundles.json> <client_version> <base_url> <released_at> <index_out> <copies_out.tsv>
// bundles.json: [{bundle_id, version, grade_bands[], file}] (written by the export step; [] while the zones still ship
// in the core build — OQ-16). Writes the compact index JSON and "<file>\t<sha256>" lines of the packs to store under
// their hash key. Exit 1 with a message on any invalid entry.
import crypto from 'node:crypto';
import fs from 'node:fs';
import { validateDoc } from '../sign-manifest.mjs';

const [listFile, clientVersion, baseUrl, releasedAt, indexOut, copiesOut] = process.argv.slice(2);
const fail = (m) => { console.error(`build-bundles: ${m}`); process.exit(1); };
const BANDS = ['g1_3', 'g4_5', 'g6_7', 'g8_9'];

let list;
try { list = JSON.parse(fs.readFileSync(listFile, 'utf8')); } catch (e) { fail(`cannot read ${listFile}: ${e.message}`); }
if (!Array.isArray(list) || list.length > 500) fail('the list must be an array of ≤ 500 bundles');
if (!/^https:\/\/[^?#\s]+$/.test(baseUrl) || baseUrl.length > 1900) fail('--base-url must be an https URL without query string');
const seen = new Set();
const copies = [];
const bundles = list.map((b, i) => {
  if (!/^[a-z0-9][a-z0-9._-]{0,127}$/.test(b.bundle_id ?? '')) fail(`entry ${i}: bundle_id must match [a-z0-9][a-z0-9._-]{0,127}`);
  if (seen.has(b.bundle_id)) fail(`entry ${i}: duplicate bundle_id ${b.bundle_id}`);
  seen.add(b.bundle_id);
  if (!Number.isInteger(b.version) || b.version < 1) fail(`entry ${i}: version must be an integer ≥ 1`);
  if (!Array.isArray(b.grade_bands) || b.grade_bands.length < 1 || b.grade_bands.some((g) => !BANDS.includes(g))) {
    fail(`entry ${i}: grade_bands must be a non-empty subset of ${BANDS.join(', ')}`);
  }
  if (!b.file || !fs.existsSync(b.file)) fail(`entry ${i}: file ${b.file} not found`);
  if (!b.file.endsWith('.pck')) fail(`entry ${i}: only .pck packs are bundles`);
  const data = fs.readFileSync(b.file);
  const sha256 = crypto.createHash('sha256').update(data).digest('hex');
  copies.push(`${b.file}\t${sha256}`);
  return { bundle_id: b.bundle_id, version: b.version, url: `${baseUrl.replace(/\/+$/, '')}/bundles/${sha256}.pck`,
    sha256, size_bytes: data.length, grade_bands: [...new Set(b.grade_bands)] };
});
bundles.sort((a, b) => a.bundle_id.localeCompare(b.bundle_id));
const index = { schema_version: 1, client_version: clientVersion, generated_at: releasedAt, bundles };
const errors = validateDoc('bundle-index', index);
if (errors.length) fail(`index invalid: ${errors.join('; ')}`);
fs.writeFileSync(indexOut, JSON.stringify(index));
fs.writeFileSync(copiesOut, copies.join('\n') + (copies.length ? '\n' : ''));
