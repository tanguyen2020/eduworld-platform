// Format: CSS custom properties `--ew-*` for the Web shell of the game (src/game/web/shell.html, F133 T028).
// The shell is the page shown before the Godot engine runs (loading, "tap to play", notices for browsers without
// WebGL 2), so it styles itself from game tokens only (AGENTS R32, ADR-0047): semantic colors, spacing, radius,
// border, font families, and the base density (BASE_DENSITY) as `--ew-density-*`. Never admin tokens.
import { GENERATED_MARK } from '../../style-dictionary.config.mjs';
import { byPath, cssVarName } from '../../lib/names.mjs';
import { cssValue } from './css-vars.mjs';

/** Token path prefixes emitted for the shell. */
export const SHELL_PREFIXES = ['color.surface.', 'color.text.', 'color.accent.', 'color.feedback.', 'color.focus',
  'component.logo.', 'component.backdrop.', 'component.mascot.', 'component.progress.',
  'size.space.', 'size.radius.', 'size.border.', 'size.layout.', 'font.family.'];

/**
 * @param {Map<string, {value: unknown, type: string}>} game resolved game context
 * @param {string} density base density band (e.g. g6_7)
 * @param {string} version tokens version
 */
export function formatGameShellCss(game, density, version) {
  const entries = [];
  for (const p of [...game.keys()].sort(byPath)) {
    const t = game.get(p);
    if (SHELL_PREFIXES.some((pre) => p === pre || p.startsWith(pre))) {
      entries.push([cssVarName(p), cssValue(t.value, t.type)]);
    } else if (p.startsWith(`density.${density}.`)) {
      entries.push([cssVarName(`density.${p.slice(`density.${density}.`.length)}`), cssValue(t.value, t.type)]);
    }
  }
  if (entries.length === 0) throw new Error('game shell css: no token matched');
  return [
    `/* ${GENERATED_MARK}. Source: design/shared-tokens, design/game-tokens (tokens@${version}, density ${density}). Run make design-tokens. */`,
    '/* Web shell of the game (src/game/web/shell.html); copied next to index.html by make godot-export-web. */',
    '',
    `:root {\n${entries.map(([n, v]) => `  ${n}: ${v};`).join('\n')}\n}`,
  ].join('\n') + '\n';
}
