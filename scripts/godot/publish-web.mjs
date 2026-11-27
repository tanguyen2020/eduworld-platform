#!/usr/bin/env node
// Publishes the Web client to the CDN origin bucket of play.<domain> (F133 T022, F133-AC-09, DEPLOYMENT §9, §11.2,
// research R4). Rules (Cache-Control, Content-Type, Brotli) come from deploy/cdn/play/headers.yaml (R33).
//   node scripts/godot/publish-web.mjs --dir=<build/web> --version=X.Y.Z --bucket=s3://<bucket> [--dry-run]
//   node scripts/godot/publish-web.mjs --rollback --version=<earlier X.Y.Z> --bucket=s3://<bucket> [--dry-run]
// Publish: every file → v/<version>/<file> (Brotli pre-compressed with Content-Encoding: br where the rule says so,
// immutable cache); a version folder that already exists is refused (immutable). Then, LAST, the entry index.html at the
// bucket root: the version's page with <base href="/v/<version>/"> so its relative assets load from the version folder,
// short cache. Rollback: re-point the root index.html to an earlier published version (no upload of assets).
// Uses the AWS CLI (`aws s3api`, endpoint EW_S3_ENDPOINT, credentials from the job env); prints keys, never secrets.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { execFileSync } from 'node:child_process';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const yaml = require('js-yaml');
const here = path.dirname(fileURLToPath(import.meta.url));
const HEADERS = path.join(here, '..', '..', 'deploy', 'cdn', 'play', 'headers.yaml');
const SEMVER = /^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$/;

export function loadRules(file = HEADERS) {
  return yaml.load(fs.readFileSync(file, 'utf8'));
}

const globRe = (g) => new RegExp(`^${g.split('*').map((s) => s.replace(/[.+?^${}()|[\]\\]/g, '\\$&')).join('[^/]*')}$`);

/** First rule of `rules` matching `key` (glob, `*` never crosses `/`). */
export function ruleFor(rules, key) {
  return rules.find((r) => globRe(r.match).test(key)) ?? null;
}

/** Root entry page of `version`: the version page with a <base> pointing to its folder. */
export function entryPage(html, version) {
  if (!SEMVER.test(version)) throw new Error('publish-web: bad version');
  const base = `<base href="/v/${version}/">`;
  if (/<base\s/i.test(html)) return html.replace(/<base\s[^>]*>/i, base);
  if (!/<head[^>]*>/i.test(html)) throw new Error('publish-web: the page has no <head>');
  return html.replace(/<head[^>]*>/i, (m) => `${m}\n${base}`);
}

function aws(args, dryRun, log) {
  const endpoint = process.env.EW_S3_ENDPOINT ? ['--endpoint-url', process.env.EW_S3_ENDPOINT] : [];
  log.push(`aws s3api ${args.filter((a) => !a.startsWith('/') && !/^[A-Za-z]:/.test(a)).join(' ')}`);
  if (dryRun) return '';
  // Through bash so the same call works on the Linux runner and under Git Bash (aws resolved from PATH).
  return execFileSync('bash', ['-c', 'exec aws "$@"', 'aws', 's3api', ...args, ...endpoint], { encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe'] });
}

function exists(bucket, key, dryRun, log) {
  if (dryRun) return false;
  try { aws(['head-object', '--bucket', bucket, '--key', key], false, log); return true; } catch { return false; }
}

function put(bucket, key, file, rule, dryRun, log) {
  const args = ['put-object', '--bucket', bucket, '--key', key, '--body', file, '--cache-control', rule.cache_control,
    '--content-type', rule.content_type];
  if (rule.brotli) args.push('--content-encoding', 'br');
  aws(args, dryRun, log);
}

export function publish({ dir, version, bucket, dryRun = false, rules = loadRules() }) {
  const log = [];
  if (!SEMVER.test(version)) throw new Error('publish-web: --version must be SemVer');
  const prefix = `${rules.web.versioned_prefix}${version}/`;
  if (exists(bucket, `${prefix}${rules.web.entry}`, dryRun, log)) throw new Error(`publish-web: ${prefix} already published (immutable)`);
  const files = fs.readdirSync(dir).filter((f) => fs.statSync(path.join(dir, f)).isFile()).sort();
  if (!files.includes(rules.web.entry)) throw new Error(`publish-web: ${dir} has no ${rules.web.entry}`);
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'pubweb-'));
  try {
    // Assets first, the version's own page after them, the root entry last (F133-AC-09).
    const ordered = [...files.filter((f) => f !== rules.web.entry), rules.web.entry];
    for (const f of ordered) {
      const key = `${prefix}${f}`;
      const rule = ruleFor(rules.web.rules, key);
      if (!rule) throw new Error(`publish-web: no rule for ${key}`);
      let body = path.join(dir, f);
      if (rule.brotli) {
        const br = path.join(tmp, `${f}.br`);
        fs.writeFileSync(br, zlib.brotliCompressSync(fs.readFileSync(body), { params: { [zlib.constants.BROTLI_PARAM_QUALITY]: 11 } }));
        body = br;
      }
      put(bucket, key, body, rule, dryRun, log);
    }
    const entry = path.join(tmp, rules.web.entry);
    fs.writeFileSync(entry, entryPage(fs.readFileSync(path.join(dir, rules.web.entry), 'utf8'), version));
    put(bucket, rules.web.entry, entry, ruleFor(rules.web.rules, rules.web.entry), dryRun, log);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
  return log;
}

export function rollback({ version, bucket, dryRun = false, rules = loadRules() }) {
  const log = [];
  if (!SEMVER.test(version)) throw new Error('publish-web: --version must be SemVer');
  const key = `${rules.web.versioned_prefix}${version}/${rules.web.entry}`;
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'pubweb-'));
  try {
    const page = path.join(tmp, 'page.html');
    if (dryRun) fs.writeFileSync(page, '<html><head></head></html>');
    else {
      if (!exists(bucket, key, false, log)) throw new Error(`publish-web: ${key} is not published, cannot roll back to it`);
      aws(['get-object', '--bucket', bucket, '--key', key, page], false, log);
    }
    const raw = fs.readFileSync(page);
    const html = raw[0] === 0x3c ? raw.toString('utf8') : zlib.brotliDecompressSync(raw).toString('utf8');
    const entry = path.join(tmp, rules.web.entry);
    fs.writeFileSync(entry, entryPage(html, version));
    put(bucket, rules.web.entry, entry, ruleFor(rules.web.rules, rules.web.entry), dryRun, log);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
  return log;
}

function main(argv) {
  const a = Object.fromEntries(argv.map((x) => { const m = x.match(/^--([a-z-]+)(?:=(.*))?$/); if (!m) throw new Error(`publish-web: bad argument ${x}`); return [m[1], m[2] ?? true]; }));
  const bucketArg = String(a.bucket ?? '');
  if (!/^s3:\/\/[a-z0-9.-]{3,63}$/.test(bucketArg)) throw new Error('publish-web: --bucket=s3://<bucket> required');
  const bucket = bucketArg.slice('s3://'.length);
  const dryRun = a['dry-run'] === true;
  const log = a.rollback ? rollback({ version: String(a.version ?? ''), bucket, dryRun })
    : publish({ dir: String(a.dir ?? ''), version: String(a.version ?? ''), bucket, dryRun });
  for (const l of log) console.log(l);
  console.log(`publish-web: ${a.rollback ? 'rolled back to' : 'published'} ${a.version}${dryRun ? ' (dry run)' : ''}`);
}

if (process.argv[1] && fileURLToPath(import.meta.url) === path.resolve(process.argv[1])) {
  try { main(process.argv.slice(2)); } catch (e) { console.error(e.message); process.exit(1); }
}
