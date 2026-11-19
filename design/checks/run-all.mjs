// make design-tokens-check (F005-AC-03): source checks + regenerate in memory and compare with the committed
// files + output separation/parity + semver against a base ref.
//   node checks/run-all.mjs [--base <git-ref>] [--only web|godot]
// Base ref: --base, else $DESIGN_BASE_REF, else origin/main (skipped when the ref has no design/).
// --only web|godot: generated files of one client only, no semver (used by web.yml / godot.yml).
import { runSourceChecks } from './index.mjs';
import { checkMarkers, compareWithDisk } from './generated.mjs';
import { checkSeparationOutputs, isGodotOutput, isWebOutput } from './separation.mjs';
import { checkDomainOutputs, sharedBusinessColors } from './domain-variants.mjs';
import { checkSemver } from './semver.mjs';
import { generate } from '../lib/pipeline.mjs';
import { REPO_ROOT } from '../lib/tokens.mjs';
import { printFindings, writeContrastReport } from '../lib/report.mjs';

const args = process.argv.slice(2);
const opt = (name) => (args.includes(name) ? args[args.indexOf(name) + 1] : undefined);
const baseRef = opt('--base') ?? process.env.DESIGN_BASE_REF ?? 'origin/main';
const only = opt('--only');

const { errors, loaded, contrastReport } = runSourceChecks();
const reportPath = writeContrastReport(contrastReport);
console.log(`contrast: ${contrastReport.summary.passed}/${contrastReport.summary.pairs} pairs pass WCAG 2.1 AA → ${reportPath}`);
const all = [...errors];
if (errors.length === 0) {
  let { files } = await generate({ loaded });
  if (only === 'web') files = new Map([...files].filter(([rel]) => isWebOutput(rel)));
  if (only === 'godot') files = new Map([...files].filter(([rel]) => isGodotOutput(rel)));
  all.push(...checkMarkers(files), ...compareWithDisk(files, REPO_ROOT, only === 'web' ? [] : undefined), ...checkSeparationOutputs(files, loaded.byFile));
  if (!only) all.push(...checkDomainOutputs(files, sharedBusinessColors(loaded.byFile)));
  console.log(`generated: compared ${files.size} file(s)${only ? ` (${only})` : ''}`);
}
if (!only) {
  const semver = checkSemver(baseRef);
  all.push(...semver.errors);
  console.log(semver.skipped ? `semver: base ${baseRef} has no design/ — rename/removal rules skipped` : `semver: compared with ${baseRef}`);
}
if (all.length) {
  printFindings('design-tokens-check', all);
  process.exit(1);
}
console.log('design-tokens-check: OK');
