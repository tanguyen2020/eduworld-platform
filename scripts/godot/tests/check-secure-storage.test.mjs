// T044 (F133-AC-24, F133-AC-25, TC-AUTH-084, ADR-0086): check-secure-storage.mjs fails on an Android artifact
// without the Keystore plugin or without the backup exclusion, an iOS build with Keychain Sharing / iCloud or
// without the Keychain adapter, and a Web shell that writes browser storage; passes on clean fixtures.
// Fixtures are built in memory (lib/zip.mjs). Run: node --test scripts/godot/
import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { writeZip } from '../lib/zip.mjs';
import { checkAndroid, checkIos, checkWeb } from '../check-secure-storage.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const u16 = (s) => Buffer.from(s, 'utf16le');
const manifest = (attrs) => Buffer.concat([Buffer.from([3, 0, 8, 0]), ...attrs.map(u16), u16('application')]);
const PCK = Buffer.from('GDPC res://scripts/platform/secure_storage_factory.gd res://scripts/platform/adapters/memory_only_secure_storage.gd');

function apk({ attrs = ['allowBackup', 'dataExtractionRules', 'fullBackupContent'], rules = true, plugin = true } = {}) {
  const entries = [
    { name: 'AndroidManifest.xml', data: manifest(attrs) },
    { name: 'classes.dex', data: Buffer.from(plugin ? 'dex\nLvn/eduworld/platform/KeystoreSecureStorage;' : 'dex\nLorg/godotengine/godot/Godot;'), deflate: true },
    { name: 'assets/scripts/platform/secure_storage_factory.gd.remap', data: PCK },
  ];
  if (rules) {
    entries.push({ name: 'res/xml/data_extraction_rules.xml', data: Buffer.from('xml') });
    entries.push({ name: 'res/xml/backup_rules.xml', data: Buffer.from('xml') });
  }
  return writeZip(entries);
}

test('clean Android APK passes; AAB layout (base/…) is understood', () => {
  assert.deepEqual(checkAndroid(apk()), []);
  const aab = writeZip([
    { name: 'base/manifest/AndroidManifest.xml', data: Buffer.from('dataExtractionRules fullBackupContent') },
    { name: 'base/dex/classes.dex', data: Buffer.from('Lvn/eduworld/platform/KeystoreSecureStorage;') },
    { name: 'base/res/xml/data_extraction_rules.xml', data: Buffer.from('x') },
    { name: 'base/res/xml/backup_rules.xml', data: Buffer.from('x') },
    { name: 'base/assets/x', data: PCK },
  ]);
  assert.deepEqual(checkAndroid(aab), []);
});

test('Android without the Keystore plugin fails (native library missing)', () => {
  assert.match(checkAndroid(apk({ plugin: false })).join('\n'), /KeystoreSecureStorage/);
});

test('Android manifest without dataExtractionRules / fullBackupContent fails (F133-AC-24)', () => {
  assert.match(checkAndroid(apk({ attrs: ['allowBackup'] })).join('\n'), /dataExtractionRules/);
  assert.match(checkAndroid(apk({ attrs: ['allowBackup', 'dataExtractionRules'] })).join('\n'), /fullBackupContent/);
  assert.match(checkAndroid(apk({ rules: false })).join('\n'), /data_extraction_rules\.xml/);
});

function iosDir({ entitlements = '<plist><dict/></plist>', plugin = true } = {}) {
  const d = fs.mkdtempSync(path.join(os.tmpdir(), 'ios-'));
  fs.mkdirSync(path.join(d, 'EduWorld'), { recursive: true });
  fs.writeFileSync(path.join(d, 'EduWorld', 'EduWorld.entitlements'), entitlements);
  fs.writeFileSync(path.join(d, 'EduWorld.pck'), PCK);
  if (plugin) fs.writeFileSync(path.join(d, 'EduWorld', 'libIosKeychainSecureStorage.a'), 'IosKeychainSecureStorage');
  return d;
}

test('iOS: clean Xcode project passes; Keychain Sharing / iCloud entitlements or a missing adapter fail (F133-AC-23)', () => {
  const ok = iosDir();
  assert.deepEqual(checkIos(ok), []);
  const sharing = iosDir({ entitlements: '<plist><dict><key>keychain-access-groups</key><array/></dict></plist>' });
  assert.match(checkIos(sharing).join('\n'), /keychain-access-groups/);
  const icloud = iosDir({ entitlements: '<plist><dict><key>com.apple.developer.icloud-services</key></dict></plist>' });
  assert.match(checkIos(icloud).join('\n'), /iCloud/);
  const noPlugin = iosDir({ plugin: false });
  assert.match(checkIos(noPlugin).join('\n'), /IosKeychainSecureStorage/);
  for (const d of [ok, sharing, icloud, noPlugin]) fs.rmSync(d, { recursive: true, force: true });
});

test('Web: a shell that writes localStorage / IndexedDB / cookies fails; MemoryOnly must ship (F133-AC-25)', () => {
  const d = fs.mkdtempSync(path.join(os.tmpdir(), 'web-'));
  fs.writeFileSync(path.join(d, 'index.html'), '<script>const EduWorldShell = 1;</script>');
  fs.writeFileSync(path.join(d, 'index.pck'), PCK);
  assert.deepEqual(checkWeb(d), []);
  fs.writeFileSync(path.join(d, 'index.html'), '<script>localStorage.setItem("eduworld.game.session", t)</script>');
  assert.match(checkWeb(d).join('\n'), /localStorage/);
  fs.writeFileSync(path.join(d, 'index.html'), '<script>indexedDB.open("x")</script>');
  assert.match(checkWeb(d).join('\n'), /indexedDB/);
  fs.writeFileSync(path.join(d, 'index.html'), '<script>const EduWorldShell = 1;</script>');
  fs.writeFileSync(path.join(d, 'index.pck'), Buffer.from('GDPC res://scripts/platform/adapters/file_cipher_secure_storage.gd'));
  assert.match(checkWeb(d).join('\n'), /memory_only_secure_storage/);
  fs.rmSync(d, { recursive: true, force: true });
});

test('CLI: exit 1 on a problem, 0 when clean, 2 on bad usage', () => {
  const d = fs.mkdtempSync(path.join(os.tmpdir(), 'css-'));
  fs.writeFileSync(path.join(d, 'good.apk'), apk());
  fs.writeFileSync(path.join(d, 'bad.apk'), apk({ attrs: [] }));
  const run = (...a) => spawnSync(process.execPath, [path.join(here, '..', 'check-secure-storage.mjs'), ...a], { encoding: 'utf8' });
  assert.equal(run('--platform=android', path.join(d, 'good.apk')).status, 0);
  assert.equal(run('--platform=android', path.join(d, 'bad.apk')).status, 1);
  assert.equal(run('--platform=windows', path.join(d, 'good.apk')).status, 2);
  assert.equal(run('--platform=android').status, 2);
  fs.rmSync(d, { recursive: true, force: true });
});
