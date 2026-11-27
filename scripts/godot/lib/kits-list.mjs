#!/usr/bin/env node
// kits.json of F134's publish-index.sh (F133 T039, F134-AC-12): one entry per signed school APK kit.
//   node kits-list.mjs add <kits.json> <kit file> <bands csv> <client_version> <url base>   (build-kit.sh)
//   node kits-list.mjs scan <dir of eduworld-school-<bands>.apk> <client_version> <url base> (production promotion)
// kit_id = android-<bands>-<version> (lowercase, [a-z0-9-], ≤ 64), url = <url base>/<file name>.
import fs from 'node:fs';
import path from 'node:path';

export function kitId(bands, version) {
  return `android-${bands.join('-')}-${version}`.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/-+$/, '').slice(0, 64);
}

export function entry(file, bands, version, urlBase) {
  return { kit_id: kitId(bands, version), grade_bands: bands, client_version: version, min_os: 'Android 9', file,
    url: `${urlBase.replace(/\/+$/, '')}/${path.basename(file)}` };
}

const [cmd, ...a] = process.argv.slice(2);
if (cmd === 'add') {
  const [list, file, bandsCsv, version, urlBase] = a;
  const kits = fs.existsSync(list) ? JSON.parse(fs.readFileSync(list, 'utf8')) : [];
  const e = entry(file, bandsCsv.split(','), version, urlBase);
  fs.writeFileSync(list, JSON.stringify([...kits.filter((k) => k.kit_id !== e.kit_id), e], null, 2) + '\n');
} else if (cmd === 'scan') {
  const [dir, version, urlBase] = a;
  const kits = fs.readdirSync(dir).filter((f) => /^eduworld-school-g[0-9_-]+\.apk$/.test(f)).sort()
    .map((f) => entry(path.join(dir, f), f.replace(/^eduworld-school-|\.apk$/g, '').split('-'), version, urlBase));
  if (kits.length === 0) { console.error('kits-list: no school kit APK found'); process.exit(1); }
  fs.writeFileSync(path.join(dir, 'kits.json'), JSON.stringify(kits, null, 2) + '\n');
} else if (cmd) {
  console.error('usage: kits-list.mjs add|scan …');
  process.exit(2);
}
