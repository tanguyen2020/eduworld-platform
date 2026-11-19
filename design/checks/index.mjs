// Source checks shared by `build` and `check` (F005). Each returns findings {rule, file, path, message}.
import { checkStructure } from './structure.mjs';
import { checkAliases } from './aliases.mjs';
import { checkDomainSources } from './domain-variants.mjs';
import { checkSeparationSources } from './separation.mjs';
import { checkContrast } from './contrast.mjs';
import { checkMinSizes, loadThresholds } from './min-sizes.mjs';
import { checkFonts } from './fonts.mjs';
import { DESIGN_DIR, REPO_ROOT, loadContexts } from '../lib/tokens.mjs';

export function runSourceChecks({ designDir = DESIGN_DIR, repoRoot = REPO_ROOT } = {}) {
  const structure = checkStructure(designDir);
  const loaded = loadContexts(designDir);
  const contrast = checkContrast(loaded);
  const errors = [
    ...structure,
    ...checkAliases(loaded),
    ...checkDomainSources(loaded.byFile),
    ...checkSeparationSources(loaded.byFile),
    ...contrast.errors,
    ...checkMinSizes(loaded, loadThresholds(repoRoot)),
    ...checkFonts(designDir),
  ];
  return { errors, loaded, contrastReport: contrast.report };
}
