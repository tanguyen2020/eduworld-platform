// make design-tokens (F005): run the source checks, then write every generated file.
//   node build.mjs [--out <dir>]   (default: repository root)
// Exit 1 when a check fails (nothing is written then, except the contrast report).
import path from 'node:path';
import { runSourceChecks } from './checks/index.mjs';
import { checkMarkers } from './checks/generated.mjs';
import { checkSeparationOutputs } from './checks/separation.mjs';
import { checkDomainOutputs, sharedBusinessColors } from './checks/domain-variants.mjs';
import { generate, writeFiles } from './lib/pipeline.mjs';
import { DESIGN_DIR, REPO_ROOT } from './lib/tokens.mjs';
import { printFindings, writeContrastReport } from './lib/report.mjs';

const args = process.argv.slice(2);
const outIdx = args.indexOf('--out');
const outRoot = outIdx >= 0 ? path.resolve(args[outIdx + 1]) : REPO_ROOT;

const { errors, loaded, contrastReport } = runSourceChecks();
const reportPath = writeContrastReport(contrastReport);
console.log(`contrast: ${contrastReport.summary.passed}/${contrastReport.summary.pairs} pairs pass WCAG 2.1 AA → ${reportPath}`);
if (errors.length) {
  printFindings('design tokens', errors);
  process.exit(1);
}

const { files, intermediate, version } = await generate({ loaded });
const outErrors = [
  ...checkMarkers(files),
  ...checkSeparationOutputs(files, loaded.byFile),
  ...checkDomainOutputs(files, sharedBusinessColors(loaded.byFile)),
];
if (outErrors.length) {
  printFindings('generated outputs', outErrors);
  process.exit(1);
}
writeFiles(files, outRoot);
writeFiles(intermediate, DESIGN_DIR);
console.log(`tokens@${version}: wrote ${files.size} file(s) under ${path.relative(process.cwd(), outRoot) || '.'}`);
for (const rel of [...files.keys()].sort()) console.log(`  ${rel}`);
console.log('Godot: .tres/.gd are written as text by this pipeline; importing them needs no Godot run here (validated in godot.yml / F060).');
