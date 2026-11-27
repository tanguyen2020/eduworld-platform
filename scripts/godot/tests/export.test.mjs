// T011 (F133-AC-01, F133-AC-02): export.sh exports Android (APK + AAB), iOS, Web with the version/build stamped into
// the packed config and restores the committed config afterwards; no Windows target; an Android release needs the
// keystore env of the signing job; iOS outside macOS is reported as pending (exit 3). A fake Godot on a temporary
// project records the calls (the real export runs in CI and in the local proof of tasks.md). Run: node --test scripts/godot/
import { test } from 'node:test';
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const script = path.join(here, '..', 'export.sh');
const posix = (p) => p.replaceAll('\\', '/');
const w = fs.mkdtempSync(path.join(os.tmpdir(), 'export-'));
const proj = path.join(w, 'game');
const committed = { env: 'production', api_base_url: 'https://api.eduworld.example/api/v1', realtime_url: 'wss://api.eduworld.example/realtime/v1',
  cdn_base_url: 'https://cdn.eduworld.example', login_path: '/auth/game/login', client_version: '0.0.0-dev', log_level: 'warn', feature_flags_default: {} };
fs.mkdirSync(path.join(proj, 'config', 'env'), { recursive: true });
fs.mkdirSync(path.join(proj, 'web'), { recursive: true });
const cfgFile = path.join(proj, 'config', 'env', 'production.json');
fs.writeFileSync(cfgFile, JSON.stringify(committed, null, 2) + '\n');
fs.writeFileSync(path.join(proj, 'web', 'shell_tokens.css'), ':root{}');
fs.writeFileSync(path.join(proj, 'export_presets.cfg'), '[preset.0]\nname="Android"\n[preset.0.options]\ngradle_build/export_format=0\n');
// Fake Godot: logs args + the stamped config, writes plausible artifacts at the export path (relative to --path).
const fake = path.join(w, 'godot');
fs.writeFileSync(fake, `#!/usr/bin/env bash
proj=""; out=""; prev=""
for a in "$@"; do [ "$prev" = "--path" ] && proj="$a"; case "$prev" in --export-release|--export-debug) preset="$a";; esac
  case "$prev" in "Android"|"iOS"|"Web") out="$a";; esac; prev="$a"; done
echo "godot $* | format=$(grep export_format "$proj/export_presets.cfg") | cfg=$(tr -d '\\n ' < "$proj/config/env/production.json")" >> "$FAKE_LOG"
[ -n "$out" ] || exit 0
mkdir -p "$proj/$(dirname "$out")"
case "$out" in
  *.html) d="$proj/$(dirname "$out")"; printf '<script>const EduWorldShell=1; const GODOT_THREADS_ENABLED = false; const GODOT_CONFIG = {"gdextensionLibs":[]};</script>' > "$d/index.html"
    printf 'GDPC res://scripts/platform/adapters/memory_only_secure_storage.gd' > "$d/index.pck"; printf 'wasm' > "$d/index.wasm"; printf 'js' > "$d/index.js" ;;
  *) printf 'artifact' > "$proj/$out" ;;
esac
`, { mode: 0o755 });
const log = path.join(w, 'godot.log');
// A tag build takes its URLs and the manifest public key from the GitHub Environment (gen-config.mjs).
const release = { EW_GAME_API_BASE_URL: 'https://api.staging.eduworld.example/api/v1', EW_GAME_REALTIME_URL: 'wss://api.staging.eduworld.example/realtime/v1',
  EW_GAME_CDN_BASE_URL: 'https://cdn.staging.eduworld.example',
  EW_GAME_MANIFEST_PUBKEY: crypto.generateKeyPairSync('rsa', { modulusLength: 3072 }).publicKey.export({ type: 'spki', format: 'pem' }) };
const run = (args, env = {}) => {
  fs.writeFileSync(log, '');
  const r = spawnSync('bash', [script, ...args, `--project=${posix(proj)}`, `--godot=${posix(fake)}`, '--skip-scan'], {
    encoding: 'utf8', env: { ...process.env, FAKE_LOG: posix(log), ...release, GITHUB_REF: 'refs/tags/client/v1.2.3', GITHUB_RUN_NUMBER: '77',
      GODOT_ANDROID_KEYSTORE_RELEASE_PATH: '', ...env } });
  return { ...r, log: fs.readFileSync(log, 'utf8') };
};

test('Web: release export with client_version/build stamped, shell stylesheet copied, config restored after', () => {
  const r = run(['web']);
  assert.equal(r.status, 0, r.stderr + r.stdout);
  assert.match(r.log, /--export-release Web build\/web\/index\.html/);
  assert.match(r.log, /"client_version":"1\.2\.3"/);
  assert.match(r.log, /"build_number":77/);
  assert.match(r.log, /api.staging.eduworld.example/);
  assert.ok(fs.existsSync(path.join(proj, 'build', 'web', 'shell_tokens.css')));
  assert.deepEqual(JSON.parse(fs.readFileSync(cfgFile, 'utf8')), committed, 'committed config restored');
  const info = JSON.parse(fs.readFileSync(path.join(proj, 'build', 'web', 'build-info.json'), 'utf8'));
  assert.deepEqual(info, { platform: 'web', client_version: '1.2.3', build_number: 77 });
});

test('Android --debug exports APK then AAB (export_format 0 then 1) and restores the preset', () => {
  const r = run(['android', '--debug']);
  assert.equal(r.status, 0, r.stderr + r.stdout);
  const lines = r.log.trim().split('\n').filter((l) => l.includes('--export-debug'));
  assert.match(lines[0], /Android build\/android\/EduWorld\.apk .*export_format=0/);
  assert.match(lines[1], /Android build\/android\/EduWorld\.aab .*export_format=1/);
  assert.match(fs.readFileSync(path.join(proj, 'export_presets.cfg'), 'utf8'), /export_format=0/);
});

test('Android release without the keystore env of the signing job is refused (R4: no password in the preset)', () => {
  const r = run(['android']);
  assert.equal(r.status, 1);
  assert.match(r.stderr, /GODOT_ANDROID_KEYSTORE_RELEASE_PATH/);
});

test('iOS outside macOS is pending (exit 3, needs the macOS runner); Windows/unknown target is refused', () => {
  const r = run(['ios'], { EW_FORCE_UNAME: 'Linux' });
  assert.equal(r.status, 3);
  assert.match(r.stderr, /macOS/);
  assert.equal(run(['windows']).status, 2);
  assert.equal(run([]).status, 2);
});

test('a branch build keeps the committed URLs; a tag build without Environment URLs is refused', () => {
  let r = run(['web'], { GITHUB_REF: 'refs/heads/main', EW_GAME_API_BASE_URL: '' });
  assert.equal(r.status, 0, r.stderr);
  assert.match(r.log, /"client_version":"0.0.0-ci.77"/);
  assert.match(r.log, /api.eduworld.example/);
  r = run(['web'], { EW_GAME_API_BASE_URL: '' });
  assert.equal(r.status, 1);
  assert.doesNotMatch(r.log, /--export/);
});

test('a bad tag stops before export and leaves the config untouched', () => {
  const r = run(['web'], { GITHUB_REF: 'refs/tags/client/v1.2' });
  assert.equal(r.status, 1);
  assert.doesNotMatch(r.log, /--export/);
  assert.deepEqual(JSON.parse(fs.readFileSync(cfgFile, 'utf8')), committed);
});

test.after(() => fs.rmSync(w, { recursive: true, force: true }));
