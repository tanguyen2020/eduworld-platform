// Test helpers (F005): a throw-away copy of design/ to mutate tokens without touching the repository.
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { DESIGN_DIR } from '../lib/tokens.mjs';

const COPY = ['shared-tokens', 'game-tokens', 'admin-tokens', 'icons', 'fonts', 'package.json', 'README.md', 'CHANGELOG.md', 'style-dictionary.config.mjs'];

export function copyDesign() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'ew-design-'));
  for (const name of COPY) fs.cpSync(path.join(DESIGN_DIR, name), path.join(dir, name), { recursive: true });
  return dir;
}

export function editJson(dir, rel, fn) {
  const file = path.join(dir, rel);
  const json = JSON.parse(fs.readFileSync(file, 'utf8'));
  fn(json);
  fs.writeFileSync(file, JSON.stringify(json, null, 2));
}

export function rm(dir) {
  fs.rmSync(dir, { recursive: true, force: true });
}

export const rules = (errors) => errors.map((e) => e.rule);
export const find = (errors, rule, tokenPath) => errors.find((e) => e.rule === rule && (tokenPath === undefined || e.path === tokenPath));
