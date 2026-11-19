// F005-AC-01: design/ layout of DESIGN_SYSTEM §2 and W3C token properties ($value, $type, $description).
import fs from 'node:fs';
import path from 'node:path';
import { DESIGN_DIR, loadFiles } from '../lib/tokens.mjs';
import { CONTEXTS } from '../style-dictionary.config.mjs';

export const REQUIRED = [
  'shared-tokens/brand.json',
  'shared-tokens/domain.json',
  'game-tokens/primitive.json',
  'game-tokens/semantic.json',
  'game-tokens/component.json',
  'game-tokens/art3d.json',
  'game-tokens/density/g1_3.json',
  'game-tokens/density/g4_5.json',
  'game-tokens/density/g6_7.json',
  'game-tokens/density/g8_9.json',
  'admin-tokens/seed.json',
  'admin-tokens/semantic.json',
  'admin-tokens/modes/light.json',
  'admin-tokens/modes/dark.json',
  'admin-tokens/modes/compact.json',
  'icons/',
  'icons/game/',
  'icons/admin/',
  'icons/shared/',
  'fonts/',
  'style-dictionary.config.mjs',
  'README.md',
  'CHANGELOG.md',
  'package.json',
];

export function checkStructure(designDir = DESIGN_DIR) {
  const errors = [];
  for (const rel of REQUIRED) {
    const abs = path.join(designDir, rel);
    const isDir = rel.endsWith('/');
    const ok = fs.existsSync(abs) && (isDir ? fs.statSync(abs).isDirectory() : fs.statSync(abs).isFile());
    if (!ok) errors.push({ rule: 'structure', file: rel, path: '', message: `missing ${isDir ? 'directory' : 'file'} (DESIGN_SYSTEM §2)` });
  }
  const files = [...new Set(Object.values(CONTEXTS).flatMap((c) => c.files))].filter((f) => fs.existsSync(path.join(designDir, f)));
  errors.push(...loadFiles(files, designDir).errors);
  return errors;
}
