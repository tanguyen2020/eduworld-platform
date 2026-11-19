// F005-AC-04: aliases point to existing tokens, no cycles, semantic/component/mode/density/art tokens alias
// a primitive (no literal), and the light/dark admin modes declare the same tokens.
import { checkLiterals } from '../lib/tokens.mjs';
import { SETS } from '../style-dictionary.config.mjs';

export function checkAliases(loaded) {
  const errors = loaded.errors.filter((e) => e.rule === 'alias-missing' || e.rule === 'alias-cycle');
  errors.push(...checkLiterals(loaded.byFile));
  const pathsOf = (files) => new Set(files.flatMap((f) => (loaded.byFile.get(f) ?? []).map((t) => t.path)));
  const light = pathsOf(SETS.adminModes.light);
  const dark = pathsOf(SETS.adminModes.dark);
  for (const p of light) if (!dark.has(p)) errors.push({ rule: 'mode-parity', file: SETS.adminModes.dark[0], path: p, message: 'declared in modes/light.json but missing in modes/dark.json' });
  for (const p of dark) if (!light.has(p)) errors.push({ rule: 'mode-parity', file: SETS.adminModes.light[0], path: p, message: 'declared in modes/dark.json but missing in modes/light.json' });
  return errors;
}
