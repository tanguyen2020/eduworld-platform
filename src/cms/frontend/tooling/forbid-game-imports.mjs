// F050-AC-07 (TC-UI-010, AGENTS R32, ADR-0047): the web never imports the game client, the game tokens or a
// game Theme. Shared by the ESLint rule `ew/no-game-imports` and the Vite plugin below (build fails).
import path from 'node:path';

const GAME_PATH = /(^|\/)src\/game(\/|$)|(^|\/)design\/game-tokens(\/|$)/;
const GAME_FILE = /\.(tres|tscn|gd|godot)$|(^|\/)(eduworld_theme[^/]*|ui_tokens|art3d_tokens)(\.[a-z]+)?$/;

const posix = (p) => p.split(path.sep).join('/');

/**
 * True when `source` (as written in an import) imported from `importer` points into the game client, the game
 * tokens or a Godot resource. Relative sources are resolved against the importer's directory.
 */
export function isGameImport(source, importer) {
  if (typeof source !== 'string') return false;
  const cleaned = source.split('?')[0];
  let target = posix(cleaned);
  if ((cleaned.startsWith('.') || path.isAbsolute(cleaned)) && importer) {
    target = posix(path.resolve(path.dirname(importer), cleaned));
  }
  return GAME_PATH.test(target) || GAME_FILE.test(target);
}

/** Vite/Rollup plugin: any game import fails dev and build. */
export function forbidGameImports() {
  return {
    name: 'eduworld:forbid-game-imports',
    enforce: 'pre',
    resolveId(source, importer) {
      if (isGameImport(source, importer)) {
        this.error(`Import of game code/theme "${source}" from ${importer ?? '?'} is forbidden in the Admin Portal (AGENTS R32, ADR-0047).`);
      }
      return null;
    },
  };
}
