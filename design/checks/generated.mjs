// F005-AC-02, F005-AC-03 (TC-UI-001): generated files carry the GENERATED marker and are byte-identical to a
// fresh pipeline run; files edited by hand, missing or left over in pipeline-owned directories are listed.
import fs from 'node:fs';
import path from 'node:path';
import { GENERATED_MARK, OWNED_DIRS } from '../style-dictionary.config.mjs';

/** Godot writes `*.import` and `*.uid` sidecars next to assets/scripts; they are not pipeline output. */
const SIDECAR = /\.(import|uid)$/;
const BINARY_OR_LICENSE = /\.(ttf|otf|woff2?|txt)$/;

export function checkMarkers(files) {
  const errors = [];
  for (const [rel, content] of files) {
    if (BINARY_OR_LICENSE.test(rel)) continue;
    const first = String(content).split('\n', 1)[0];
    if (!first.includes(GENERATED_MARK)) errors.push({ rule: 'generated-marker', file: rel, path: '', message: 'first line lacks the GENERATED marker' });
  }
  return errors;
}

function listFiles(dir) {
  if (!fs.existsSync(dir)) return [];
  return fs.readdirSync(dir, { withFileTypes: true }).filter((e) => e.isFile()).map((e) => e.name);
}

/** Compares expected files (Map rel → content) with the tree under `root`. */
export function compareWithDisk(files, root, ownedDirs = OWNED_DIRS) {
  const errors = [];
  for (const [rel, content] of files) {
    const abs = path.join(root, rel);
    if (!fs.existsSync(abs)) {
      errors.push({ rule: 'generated-missing', file: rel, path: '', message: 'generated file is missing — run make design-tokens' });
      continue;
    }
    const expected = Buffer.isBuffer(content) ? content : Buffer.from(content, 'utf8');
    if (!fs.readFileSync(abs).equals(expected)) {
      errors.push({ rule: 'generated-drift', file: rel, path: '', message: 'differs from the pipeline output (edited by hand or tokens changed without make design-tokens)' });
    }
  }
  const expectedSet = new Set(files.keys());
  for (const dir of ownedDirs) {
    for (const name of listFiles(path.join(root, dir))) {
      const rel = `${dir}/${name}`;
      if (!expectedSet.has(rel) && !SIDECAR.test(name)) errors.push({ rule: 'generated-stale', file: rel, path: '', message: 'file in a pipeline-owned directory that the pipeline does not produce' });
    }
  }
  return errors;
}
