// T022 (F133-AC-09, DEPLOYMENT §9/§11.2, PERFORMANCE_BUDGET §14): deploy/cdn/play/headers.yaml gives immutable,
// Brotli-compressed version folders and a short-cached entry page, never COOP/COEP (single-threaded) and never a
// wildcard CORS origin; publish-web.mjs uploads assets first and the root index.html LAST with a <base> to the version
// folder, refuses to overwrite a published version, and a rollback only re-points index.html. The live header check
// with curl on play-staging is T018 (needs the CDN of F130). Run: node --test scripts/godot/
import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { loadRules, ruleFor, entryPage, publish } from '../publish-web.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const posix = (p) => p.replaceAll('\\', '/');
const rules = loadRules();

test('headers.yaml: version folder immutable + Brotli, entry short cache, no COOP/COEP, CORS allowlist only', () => {
  for (const f of ['index.wasm', 'index.pck', 'index.js', 'index.html', 'shell_tokens.css']) {
    const r = ruleFor(rules.web.rules, `v/1.0.0/${f}`);
    assert.match(r.cache_control, /max-age=31536000, immutable/, f);
    assert.equal(r.brotli, true, f);
  }
  assert.equal(ruleFor(rules.web.rules, 'v/1.0.0/index.wasm').content_type, 'application/wasm');
  const entry = ruleFor(rules.web.rules, 'index.html');
  assert.equal(entry.cache_control, 'public, max-age=60');
  assert.equal(entry.brotli, false);
  assert.deepEqual(rules.web.forbidden_headers.sort(), ['Cross-Origin-Embedder-Policy', 'Cross-Origin-Opener-Policy']);
  for (const h of Object.keys(rules.web.security_headers)) assert.ok(!rules.web.forbidden_headers.includes(h));
  assert.equal(rules.cors.allowed_origins_env, 'EW_CORS_GAME_ORIGINS');
  assert.equal(rules.cors.allow_credentials, false);
  assert.doesNotMatch(fs.readFileSync(path.join(here, '..', '..', '..', 'deploy', 'cdn', 'play', 'headers.yaml'), 'utf8'), /origin[s]?:\s*["']?\*/i);
  assert.match(ruleFor(rules.bundles.rules, 'bundles/' + 'a'.repeat(64) + '.pck').cache_control, /immutable/);
  assert.equal(ruleFor(rules.web.rules, 'v/1.0.0/sub/x.js'), null, '* does not cross folders');
});

test('entry page gets a <base> to the version folder (relative assets load from it)', () => {
  assert.equal(entryPage('<html><head><title>x</title></head></html>', '1.2.3'),
    '<html><head>\n<base href="/v/1.2.3/"><title>x</title></head></html>');
  assert.match(entryPage('<html><head><base href="/v/1.0.0/"></head></html>', '1.2.3'), /<base href="\/v\/1\.2\.3\/">/);
  assert.throws(() => entryPage('<html></html>', '1.2.3'), /head/);
});

// Fake aws CLI keeping objects in a folder (no network); --content-encoding recorded per key.
const w = fs.mkdtempSync(path.join(os.tmpdir(), 'pubweb-'));
const bin = path.join(w, 'bin');
fs.mkdirSync(bin);
fs.writeFileSync(path.join(bin, 'aws'), `#!/usr/bin/env bash
echo "$*" >> "$FAKE_S3/calls.log"
op="$2"; shift 2; key=""; body=""; out=""
while [ $# -gt 0 ]; do case "$1" in --bucket|--cache-control|--content-type|--content-encoding|--endpoint-url) shift;; --key) key="$2"; shift;; --body) body="$2"; shift;; *) out="$1";; esac; shift; done
obj="$FAKE_S3/objects/$key"
case "$op" in head-object) [ -f "$obj" ] || exit 254 ;; get-object) cp "$obj" "$out" ;; put-object) mkdir -p "$(dirname "$obj")"; cp "$body" "$obj" ;; esac
`, { mode: 0o755 });
const s3 = path.join(w, 's3');
fs.mkdirSync(path.join(s3, 'objects'), { recursive: true });
const web = path.join(w, 'web');
fs.mkdirSync(web);
fs.writeFileSync(path.join(web, 'index.html'), '<html><head><title>EduWorld</title></head><body>EduWorldShell</body></html>');
fs.writeFileSync(path.join(web, 'index.wasm'), Buffer.alloc(5000, 1));
fs.writeFileSync(path.join(web, 'index.js'), 'console.log(1)');
const cli = (...a) => spawnSync(process.execPath, [path.join(here, '..', 'publish-web.mjs'), ...a], { encoding: 'utf8',
  env: { ...process.env, PATH: `${posix(bin)}${path.delimiter}${process.env.PATH}`, FAKE_S3: posix(s3) } });
const calls = () => fs.readFileSync(path.join(s3, 'calls.log'), 'utf8').trim().split('\n').filter((l) => l.startsWith('s3api put-object'));

test('publish: assets first, root index.html last with <base>, Brotli bodies, immutable version folder', () => {
  const r = cli(`--dir=${posix(web)}`, '--version=1.0.0', '--bucket=s3://ew-play-staging');
  assert.equal(r.status, 0, r.stderr);
  const puts = calls();
  assert.match(puts.at(-1), /--key index\.html .*max-age=60/);
  assert.match(puts.at(-2), /--key v\/1\.0\.0\/index\.html .*immutable.*--content-encoding br/);
  assert.ok(puts.findIndex((l) => l.includes('v/1.0.0/index.wasm')) < puts.length - 2);
  const wasm = fs.readFileSync(path.join(s3, 'objects', 'v', '1.0.0', 'index.wasm'));
  assert.equal(zlib.brotliDecompressSync(wasm).length, 5000);
  assert.match(fs.readFileSync(path.join(s3, 'objects', 'index.html'), 'utf8'), /<base href="\/v\/1\.0\.0\/">/);
  const again = cli(`--dir=${posix(web)}`, '--version=1.0.0', '--bucket=s3://ew-play-staging');
  assert.equal(again.status, 1);
  assert.match(again.stderr, /already published/);
});

test('rollback re-points index.html to an earlier version without uploading assets; unknown version refused', () => {
  assert.equal(cli(`--dir=${posix(web)}`, '--version=1.1.0', '--bucket=s3://ew-play-staging').status, 0);
  assert.match(fs.readFileSync(path.join(s3, 'objects', 'index.html'), 'utf8'), /\/v\/1\.1\.0\//);
  fs.writeFileSync(path.join(s3, 'calls.log'), '');
  const r = cli('--rollback', '--version=1.0.0', '--bucket=s3://ew-play-staging');
  assert.equal(r.status, 0, r.stderr);
  assert.deepEqual(calls().map((l) => l.match(/--key (\S+)/)[1]), ['index.html']);
  assert.match(fs.readFileSync(path.join(s3, 'objects', 'index.html'), 'utf8'), /\/v\/1\.0\.0\//);
  assert.equal(cli('--rollback', '--version=0.1.0', '--bucket=s3://ew-play-staging').status, 1);
  assert.equal(cli('--rollback', '--version=0.1.0', '--bucket=https://x').status, 1);
});

test('dry run lists the uploads in order and touches nothing', () => {
  const log = publish({ dir: web, version: '2.0.0', bucket: 'b', dryRun: true, rules });
  assert.match(log.at(-1), /--key index\.html/);
  assert.ok(!fs.existsSync(path.join(s3, 'objects', 'v', '2.0.0')));
});

test.after(() => fs.rmSync(w, { recursive: true, force: true }));
