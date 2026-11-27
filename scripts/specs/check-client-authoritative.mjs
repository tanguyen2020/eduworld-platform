#!/usr/bin/env node
// F132-AC-21 (ANTI_CHEAT §4, ADR-0016, AGENTS R5): no student request schema declares score, xp, is_correct or rating.
// Runs the Spectral function specs/spectral-functions/noClientAuthoritativeFields.js (rule
// `no-client-authoritative-fields` of specs/.spectral.yaml) over the public OpenAPI files without the Spectral CLI,
// resolving local $ref itself. Exit 1 on a violation.
//
// Usage: node scripts/specs/check-client-authoritative.mjs [file...]
//        (default: specs/openapi/eduworld-api.v1.yaml and the public specs/openapi/modules/<module>.v1.yaml)
import { createRequire } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const require = createRequire(import.meta.url);
const yaml = require('js-yaml');
const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '..', '..');
const { isStudentOperation, forbiddenRequestFields } = await import(
  pathToFileURL(path.join(root, 'specs', 'spectral-functions', 'noClientAuthoritativeFields.js')).href
);

const METHODS = ['post', 'put', 'patch'];

/** Returns [{file, operation, method, path, field, pointer}] for one parsed OpenAPI document. */
export function checkDocument(doc, file = '<doc>') {
  const resolve = (node) => {
    let n = node;
    for (let i = 0; i < 32 && n && typeof n === 'object' && typeof n.$ref === 'string'; i++) {
      if (!n.$ref.startsWith('#/')) return undefined; // external refs are checked through the file that bundles them
      n = n.$ref.slice(2).split('/').reduce((o, k) => (o == null ? o : o[k.replace(/~1/g, '/').replace(/~0/g, '~')]), doc);
    }
    return n;
  };
  const out = [];
  for (const [p, item] of Object.entries(doc?.paths ?? {})) {
    for (const m of METHODS) {
      const op = item?.[m];
      if (!op || !isStudentOperation(p, op)) continue;
      for (const f of forbiddenRequestFields(op, resolve)) out.push({ file, operation: op.operationId ?? `${m} ${p}`, method: m, path: p, ...f });
    }
  }
  return out;
}

function defaultFiles() {
  const dir = path.join(root, 'specs', 'openapi');
  const mods = fs.readdirSync(path.join(dir, 'modules')).filter((f) => /^[a-z]+\.v1\.yaml$/.test(f)).map((f) => path.join(dir, 'modules', f));
  return [path.join(dir, 'eduworld-api.v1.yaml'), ...mods];
}

if (import.meta.url === pathToFileURL(process.argv[1]).href) {
  const files = process.argv.slice(2).length ? process.argv.slice(2) : defaultFiles();
  let ops = 0;
  const all = [];
  for (const f of files) {
    const doc = yaml.load(fs.readFileSync(f, 'utf8'));
    for (const [p, item] of Object.entries(doc?.paths ?? {})) for (const m of METHODS) if (item?.[m] && isStudentOperation(p, item[m])) ops++;
    all.push(...checkDocument(doc, path.relative(root, f).split(path.sep).join('/')));
  }
  for (const v of all) console.error(`${v.file}: ${v.operation} (${v.method.toUpperCase()} ${v.path}) declares "${v.field}" at ${v.pointer} — server-authoritative field (ANTI_CHEAT §4)`);
  console.log(`check-client-authoritative: ${files.length} file(s), ${ops} student request operation(s), ${all.length} violation(s)`);
  process.exit(all.length ? 1 : 0);
}
