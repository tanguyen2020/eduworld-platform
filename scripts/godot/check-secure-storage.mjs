#!/usr/bin/env node
// Secure storage of the session in each release artifact (F133 T048, F133-AC-23/24/25, ADR-0086, ADR-0101,
// GODOT_ARCHITECTURE §5.2, §11). Run by `godot / scan-artifact` (scripts/godot/scan-artifact.sh) for every target:
//   android (APK/AAB): the EduWorld Keystore plugin (vn.eduworld.platform.KeystoreSecureStorage) is in the dex; the
//     merged manifest sets android:dataExtractionRules + android:fullBackupContent and the two rule files exist
//     (session folder excluded from cloud backup and device transfer); SecureStorageFactory ships;
//   ios (Xcode project folder or IPA): the Keychain adapter (IosKeychainSecureStorage) is linked; no entitlement
//     for Keychain Sharing (keychain-access-groups) or iCloud;
//   web (export folder): MemoryOnly ships and no file of the page writes localStorage/sessionStorage/IndexedDB/cookies.
//
//   node scripts/godot/check-secure-storage.mjs --platform=<android|ios|web> <artifact|dir>
// Exit 0 clean, 1 problems (listed), 2 usage. No Windows target (ADR-0101).
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { readZip, containsText } from './lib/zip.mjs';

const FACTORY = 'secure_storage_factory';
const KEYSTORE_CLASS = 'vn/eduworld/platform/KeystoreSecureStorage';
const KEYCHAIN_ADAPTER = 'IosKeychainSecureStorage';
const MEMORY_ONLY = 'memory_only_secure_storage';
const BROWSER_STORAGE = /\b(localStorage|sessionStorage|indexedDB)\s*\.|document\.cookie\s*=/;

/** Problems of an Android APK/AAB (Buffer). */
export function checkAndroid(buf) {
  const entries = readZip(buf);
  const byName = (re) => entries.filter((e) => re.test(e.name));
  const problems = [];
  const manifest = byName(/^(base\/manifest\/)?AndroidManifest\.xml$/)[0];
  if (!manifest) problems.push('AndroidManifest.xml not found');
  else {
    const xml = manifest.data();
    for (const attr of ['dataExtractionRules', 'fullBackupContent']) {
      if (!containsText(xml, attr)) problems.push(`manifest has no android:${attr} (session must be excluded from Auto Backup / device transfer, F133-AC-24)`);
    }
  }
  for (const file of ['data_extraction_rules.xml', 'backup_rules.xml']) {
    if (byName(new RegExp(`^(base/)?res/xml/${file.replace('.', '\\.')}$`)).length === 0) problems.push(`res/xml/${file} missing`);
  }
  const dex = byName(/^(base\/dex\/)?classes\d*\.dex$/);
  if (!dex.some((e) => containsText(e.data(), KEYSTORE_CLASS))) {
    problems.push(`Android Keystore plugin missing (${KEYSTORE_CLASS.replaceAll('/', '.')} not in the dex, F133-AC-25)`);
  }
  if (!entries.some((e) => e.name.includes(FACTORY) || (/\.pck$|^(base\/)?assets\//.test(e.name) && containsText(e.data(), FACTORY)))) {
    problems.push('SecureStorageFactory is not in the game data');
  }
  return problems;
}

function filesOf(dir) {
  return fs.readdirSync(dir, { withFileTypes: true }).flatMap((e) =>
    e.isDirectory() ? filesOf(path.join(dir, e.name)) : [path.join(dir, e.name)]);
}

/** Problems of an iOS Xcode project folder or IPA (path). */
export function checkIos(target) {
  const files = fs.statSync(target).isDirectory()
    ? filesOf(target).map((f) => ({ name: path.relative(target, f).replaceAll('\\', '/'), data: () => fs.readFileSync(f) }))
    : readZip(fs.readFileSync(target)).map((e) => ({ name: e.name, data: () => e.data() }));
  const problems = [];
  for (const f of files.filter((x) => /\.entitlements$|embedded\.mobileprovision$|\.xcent$/.test(x.name))) {
    const text = f.data().toString('latin1');
    if (/keychain-access-groups/.test(text) && /\.entitlements$|\.xcent$/.test(f.name)) problems.push(`${f.name}: keychain-access-groups (Keychain Sharing) is not allowed (F133-AC-23)`);
    if (/com\.apple\.developer\.(icloud|ubiquity)/.test(text) && /\.entitlements$|\.xcent$/.test(f.name)) problems.push(`${f.name}: iCloud entitlement is not allowed (ADR-0086)`);
  }
  if (!files.some((f) => f.name.includes(KEYCHAIN_ADAPTER) || (/\.(a|dylib|framework|xcframework|plist|pck)$|\/[^.]+$/.test(f.name) && containsText(f.data(), KEYCHAIN_ADAPTER)))) {
    problems.push(`iOS Keychain adapter ${KEYCHAIN_ADAPTER} not linked (F133-AC-23/25; plugin of F133 T046)`);
  }
  return problems;
}

/** Problems of a Web export folder. */
export function checkWeb(dir) {
  const problems = [];
  const files = filesOf(dir);
  for (const f of files.filter((x) => /\.html?$/.test(x))) {
    const m = fs.readFileSync(f, 'utf8').match(BROWSER_STORAGE);
    if (m) problems.push(`${path.basename(f)} uses ${m[1] ?? 'document.cookie'} (the Web build keeps the session in RAM only, ADR-0086)`);
  }
  const pcks = files.filter((x) => x.endsWith('.pck'));
  if (pcks.length === 0) problems.push('no .pck in the Web export');
  else if (!pcks.some((f) => containsText(fs.readFileSync(f), MEMORY_ONLY))) problems.push(`${MEMORY_ONLY} is not in the Web pack (F133-AC-25)`);
  return problems;
}

function main(argv) {
  const platform = (argv.find((a) => a.startsWith('--platform=')) ?? '').slice('--platform='.length);
  const target = argv.find((a) => !a.startsWith('--'));
  if (!['android', 'ios', 'web'].includes(platform) || !target || !fs.existsSync(target)) {
    console.error('usage: check-secure-storage.mjs --platform=<android|ios|web> <artifact|dir>');
    return 2;
  }
  let problems;
  if (platform === 'android') {
    const files = fs.statSync(target).isDirectory() ? filesOf(target).filter((f) => /\.(apk|aab)$/.test(f)) : [target];
    if (files.length === 0) { console.error('check-secure-storage: no APK/AAB found'); return 2; }
    problems = files.flatMap((f) => checkAndroid(fs.readFileSync(f)).map((p) => `${path.basename(f)}: ${p}`));
  } else if (platform === 'ios') problems = checkIos(target);
  else problems = checkWeb(target);
  for (const p of problems) console.error(`check-secure-storage: ${p}`);
  console.log(`check-secure-storage: ${platform} ${problems.length ? `${problems.length} problem(s)` : 'OK'}`);
  return problems.length ? 1 : 0;
}

if (process.argv[1] && fileURLToPath(import.meta.url) === path.resolve(process.argv[1])) {
  try { process.exit(main(process.argv.slice(2))); } catch (e) { console.error(`check-secure-storage: ${e.message}`); process.exit(1); }
}
