// Tests of check-build-size.mjs (F133 T009/T012, F133-AC-04, NFR-PERF-006, PERFORMANCE_BUDGET §11). The limits come
// from the owner document (R33); the fixtures use a small budget text so no 300 MB file is written.
// Run: node --test scripts/godot/
import { test } from 'node:test';
import assert from 'node:assert/strict';
import crypto from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { spawnSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import { parseBudget, measure, renderReport } from '../check-build-size.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const MB = 1_000_000;
const tiny = `## 11. Thời gian tải và dung lượng

| Chỉ số | Mục tiêu (Tầm trung) |
|---|---|
| Bản cài Android (APK/AAB base) | ≤ 0,01 MB |
| Bản cài iOS | ≤ 0,01 MB |
| Web tải đầu (wasm + pck lõi, đã nén) | ≤ 0,02 MB; phần còn lại tải theo vùng |
| Gói nội dung mỗi vùng (PCK) | ≤ 0,01 MB nén |
| Bộ nhớ cache nội dung trên thiết bị | ≤ 1,5 GB, xóa LRU theo vùng ít dùng |

## 12. Pin
`;

function tmp() { return fs.mkdtempSync(path.join(os.tmpdir(), 'bsize-')); }
const rand = (n) => crypto.randomBytes(n);

test('limits are read from PERFORMANCE_BUDGET §11 (build_size: 300/300/40/40 MB, cache 1.5 GB)', () => {
  const doc = fs.readFileSync(path.join(here, '..', '..', '..', 'docs', '08-game-client', 'PERFORMANCE_BUDGET.md'), 'utf8');
  const b = parseBudget(doc);
  assert.equal(b.android, 300 * MB);
  assert.equal(b.ios, 300 * MB);
  assert.equal(b.web, 40 * MB);
  assert.equal(b.bundle, 40 * MB);
  assert.equal(b.cache, 1.5 * 1000 * MB);
  assert.throws(() => parseBudget('## 11. x\n| a | b |\n'), /Android/);
});

test('every platform and bundle over its limit fails; within the limit passes with a report', () => {
  const budget = parseBudget(tiny);
  const d = tmp();
  fs.writeFileSync(path.join(d, 'big.apk'), rand(20_000));
  fs.writeFileSync(path.join(d, 'ok.apk'), rand(5_000));
  fs.writeFileSync(path.join(d, 'big.ipa'), rand(20_000));
  fs.mkdirSync(path.join(d, 'web'));
  fs.writeFileSync(path.join(d, 'web', 'index.wasm'), rand(30_000));
  fs.writeFileSync(path.join(d, 'web', 'index.html'), '<html></html>');
  fs.mkdirSync(path.join(d, 'bundles'));
  fs.writeFileSync(path.join(d, 'bundles', 'a.pck'), rand(20_000));
  fs.writeFileSync(path.join(d, 'bundles', 'b.pck'), Buffer.alloc(50_000)); // compresses well → within "nén" limit
  const rows = measure({ android: [path.join(d, 'big.apk'), path.join(d, 'ok.apk')], ios: [path.join(d, 'big.ipa')],
    web: path.join(d, 'web'), bundles: path.join(d, 'bundles') }, budget);
  const by = Object.fromEntries(rows.map((r) => [r.name, r]));
  assert.equal(by['android big.apk'].ok, false);
  assert.equal(by['android ok.apk'].ok, true);
  assert.equal(by['ios big.ipa'].ok, false);
  assert.equal(by['web first download'].ok, false);
  assert.equal(by['bundle a.pck'].ok, false);
  assert.equal(by['bundle b.pck'].ok, true);
  const text = renderReport(rows, budget);
  assert.match(text, /\| android big\.apk \| .* \| ❌/);
  assert.match(text, /PERFORMANCE_BUDGET.md` §11/);
  fs.rmSync(d, { recursive: true, force: true });
});

test('CLI exits 1 over budget, 0 within it, and writes the report file', () => {
  const d = tmp();
  fs.writeFileSync(path.join(d, 'budget.md'), tiny);
  fs.writeFileSync(path.join(d, 'a.apk'), rand(20_000));
  fs.writeFileSync(path.join(d, 'b.apk'), rand(1_000));
  const run = (...a) => spawnSync(process.execPath, [path.join(here, '..', 'check-build-size.mjs'), `--budget=${path.join(d, 'budget.md')}`, ...a], { encoding: 'utf8' });
  assert.equal(run(`--android=${path.join(d, 'a.apk')}`).status, 1);
  const r = run(`--android=${path.join(d, 'b.apk')}`, `--report=${path.join(d, 'report.md')}`);
  assert.equal(r.status, 0, r.stderr);
  assert.match(fs.readFileSync(path.join(d, 'report.md'), 'utf8'), /\| android b\.apk \|/);
  assert.equal(run().status, 2, 'nothing to measure is a usage error');
  assert.equal(run(`--android=${path.join(d, 'missing.apk')}`).status, 2);
  fs.rmSync(d, { recursive: true, force: true });
});
