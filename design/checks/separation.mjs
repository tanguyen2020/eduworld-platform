// F005-AC-07 (TC-UI-010): two design systems stay apart. Sources: game-tokens and admin-tokens never declare
// the same token. Outputs: the web theme carries no game token, the Godot outputs carry no admin token and
// nothing of the Admin UI (Ant Design, ConfigProvider, ops-ui).
import { SETS, OUTPUTS, OWNED_DIRS } from '../style-dictionary.config.mjs';
import { cssVarName, gdConstName } from '../lib/names.mjs';

const gameFiles = [...SETS.game, ...SETS.gameHighContrast];
const adminFiles = [...SETS.admin, ...new Set(Object.values(SETS.adminModes).flat())];

function pathsIn(byFile, files) {
  const out = new Map();
  for (const f of files) for (const t of byFile.get(f) ?? []) if (!out.has(t.path)) out.set(t.path, f);
  return out;
}

export function checkSeparationSources(byFile) {
  const game = pathsIn(byFile, gameFiles);
  const admin = pathsIn(byFile, adminFiles);
  const errors = [];
  for (const [p, f] of game) {
    if (admin.has(p)) errors.push({ rule: 'separation', file: f, path: p, message: `declared in both game-tokens and admin-tokens (${admin.get(p)}); the two design systems must not share token names` });
  }
  return errors;
}

export const isWebOutput = (rel) => rel === OUTPUTS.antdTheme || rel === OUTPUTS.tokensCss;
// The Web shell of the game (F133) is a game output: game tokens only, written as CSS variables.
export const isGodotOutput = (rel) => rel === OUTPUTS.uiTokens || rel === OUTPUTS.gameShellCss || OWNED_DIRS.some((d) => rel.startsWith(`${d}/`));

const ADMIN_UI_MARKERS = [/\bantd\b/i, /ConfigProvider/, /ops-ui/, /admin-tokens/];
const GAME_MARKERS = [/game-tokens/];

/** files: Map(rel → string|Buffer) of generated outputs. */
export function checkSeparationOutputs(files, byFile) {
  const errors = [];
  const gameOnly = [...pathsIn(byFile, gameFiles).keys()];
  const adminOnly = [...pathsIn(byFile, adminFiles).keys()];
  const gameCss = gameOnly.map((p) => [p, `${cssVarName(p)}:`]);
  const gameTs = gameOnly.map((p) => [p, `"${p}"`]);
  const adminGd = adminOnly.map((p) => [p, new RegExp(`\\b${gdConstName(p)}\\b`)]);
  const adminCss = adminOnly.map((p) => [p, `${cssVarName(p)}:`]);
  for (const [rel, content] of files) {
    if (Buffer.isBuffer(content)) continue;
    if (isWebOutput(rel)) {
      for (const re of GAME_MARKERS) if (re.test(content)) errors.push({ rule: 'separation', file: rel, path: '', message: `web output mentions ${re}` });
      for (const [p, needle] of [...gameCss, ...gameTs]) if (content.includes(needle)) errors.push({ rule: 'separation', file: rel, path: p, message: 'web output contains a game-tokens token' });
    } else if (isGodotOutput(rel)) {
      for (const re of ADMIN_UI_MARKERS) if (re.test(content)) errors.push({ rule: 'separation', file: rel, path: '', message: `Godot output contains Admin UI marker ${re}` });
      for (const [p, re] of adminGd) if (re.test(content)) errors.push({ rule: 'separation', file: rel, path: p, message: 'Godot output contains an admin-tokens token' });
      if (rel === OUTPUTS.gameShellCss) for (const [p, needle] of adminCss) if (content.includes(needle)) errors.push({ rule: 'separation', file: rel, path: p, message: 'game Web shell contains an admin-tokens token' });
    }
  }
  return errors;
}
