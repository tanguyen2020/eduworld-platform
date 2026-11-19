// Writes the contrast report (design/build/reports/contrast.json) and prints findings.
import fs from 'node:fs';
import path from 'node:path';
import { BUILD_DIR } from '../style-dictionary.config.mjs';
import { DESIGN_DIR, formatErrors } from './tokens.mjs';

export function writeContrastReport(report, designDir = DESIGN_DIR) {
  const file = path.join(designDir, BUILD_DIR, 'reports', 'contrast.json');
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, JSON.stringify(report, null, 2) + '\n');
  return path.relative(path.resolve(designDir, '..'), file).split(path.sep).join('/');
}

export function printFindings(title, errors) {
  if (errors.length === 0) return;
  console.error(`${title}: ${errors.length} problem(s)`);
  console.error(formatErrors(errors));
}
