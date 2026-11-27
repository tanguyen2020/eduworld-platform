// T047 (F133-AC-24, ADR-0086): the Android plugin of F060 carries the backup exclusion of the release manifest —
// android:dataExtractionRules (Android 12+) and android:fullBackupContent (Android 9–11) point to rule files that keep
// the session folders out of cloud backup and device-to-device transfer; no runtime permission is added; the release
// preset keeps allowBackup=false. The built APK is checked by check-secure-storage.mjs in `godot / scan-artifact`.
// Run: node --test scripts/godot/
import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const main = path.join(here, '..', '..', '..', 'src', 'game', 'addons', 'eduworld_platform', 'android', 'src', 'main');
const read = (...p) => fs.readFileSync(path.join(main, ...p), 'utf8');

test('plugin manifest declares both rule attributes and no permission', () => {
  const m = read('AndroidManifest.xml');
  assert.match(m, /android:dataExtractionRules="@xml\/data_extraction_rules"/);
  assert.match(m, /android:fullBackupContent="@xml\/backup_rules"/);
  assert.doesNotMatch(m, /<uses-permission/);
});

test('rules exclude the session folder from cloud backup and device transfer', () => {
  const rules = read('res', 'xml', 'data_extraction_rules.xml');
  for (const block of ['cloud-backup', 'device-transfer']) {
    const body = rules.match(new RegExp(`<${block}>([\\s\\S]*?)</${block}>`))?.[1] ?? '';
    assert.match(body, /<exclude domain="file" path="secure\/" \/>/, block);
  }
  assert.match(read('res', 'xml', 'backup_rules.xml'), /<exclude domain="file" path="secure\/" \/>/);
});

test('the release preset keeps allowBackup=false (Godot user_data_backup/allow)', () => {
  const presets = fs.readFileSync(path.join(here, '..', '..', '..', 'src', 'game', 'export_presets.cfg'), 'utf8');
  assert.match(presets, /^user_data_backup\/allow=false$/m);
});
