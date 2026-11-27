// Tests of the ZAP gate (F132-AC-18): High blocks, Medium is listed, false positives and evidence are ignored.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { evaluate, main } from './check-report.mjs';

const report = (alerts) => ({ site: [{ '@name': 'https://api.staging.example', alerts }] });

test('a High alert fails the gate, with its path but never the evidence', () => {
  const r = report([
    { pluginid: '40018', alert: 'SQL Injection', riskcode: '3', confidence: '2', count: '1',
      instances: [{ uri: 'https://api.staging.example/api/v1/cms/things?q=secret-token', evidence: 'Bearer abc' }] },
    { pluginid: '10038', alert: 'CSP Header Not Set', riskcode: '2', confidence: '3', count: '4', instances: [] },
    { pluginid: '10096', alert: 'Timestamp Disclosure', riskcode: '0', confidence: '1', count: '9', instances: [] },
  ]);
  const e = evaluate(r);
  assert.equal(e.high.length, 1);
  assert.equal(e.medium.length, 1);
  assert.deepEqual(e.high[0].paths, ['/api/v1/cms/things']);
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'zap-'));
  const f = path.join(dir, 'r.json');
  fs.writeFileSync(f, JSON.stringify(r));
  const lines = [];
  assert.equal(main([f], (l) => lines.push(l)), 1);
  const out = lines.join('\n');
  assert.match(out, /HIGH {3}40018 SQL Injection/);
  assert.doesNotMatch(out, /secret-token|Bearer abc/);
});

test('a High alert marked false positive (confidence 0) does not fail', () => {
  const e = evaluate(report([{ pluginid: '40012', alert: 'XSS', riskcode: '3', confidence: '0', count: '1', instances: [] }]));
  assert.equal(e.high.length, 0);
});

test('Medium only passes the gate', () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'zap-'));
  const f = path.join(dir, 'r.json');
  fs.writeFileSync(f, JSON.stringify(report([{ pluginid: '10038', alert: 'CSP', riskcode: '2', confidence: '2', count: '1' }])));
  assert.equal(main([f], () => {}), 0);
  assert.equal(main([], () => {}), 2);
});
