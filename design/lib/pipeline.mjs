// Token pipeline (F005): resolve every context with Style Dictionary and produce all generated files in
// memory. build.mjs writes them; checks/generated.mjs compares them with the committed files.
import fs from 'node:fs';
import path from 'node:path';
import { BUILD_DIR, CONTEXTS, DENSITY_BANDS, BASE_DENSITY, OUTPUTS, BUSINESS_COLOR_PREFIXES } from '../style-dictionary.config.mjs';
import { DESIGN_DIR, loadContexts } from './tokens.mjs';
import { resolveWithStyleDictionary } from '../sd/transforms.mjs';
import { formatAntdTheme } from '../sd/formats/antd-theme.mjs';
import { formatCssVars } from '../sd/formats/css-vars.mjs';
import { formatGodotJson } from '../sd/formats/godot-json.mjs';
import { formatUiTokens, formatArt3dTokens, themeFile, HIGH_CONTRAST_THEME } from '../sd/formats/gdscript-tokens.mjs';
import { formatTheme, formatEnvironment, formatGradient, godotFontFiles, artPaletteSlots, formatToonMaterial } from '../sd/formats/godot-tres.mjs';

/** Toon material presets (F067, DESIGN_SYSTEM §5.3): with the outline pass, and without it for small dense decor. */
export const TOON_MATERIALS = { toon: { outline: true }, toon_no_outline: { outline: false } };

export function tokensVersion(designDir = DESIGN_DIR) {
  return JSON.parse(fs.readFileSync(path.join(designDir, 'package.json'), 'utf8')).version;
}

/** Business color variants from the shared set: path → {light, dark, game}. */
export function businessVariants(tokens) {
  const out = new Map();
  for (const [p, t] of tokens) {
    if (BUSINESS_COLOR_PREFIXES.some((pre) => p.startsWith(pre)) && t.extensions?.variants) out.set(p, t.extensions.variants);
  }
  return out;
}

/** Names of groups under `prefix` that hold a given leaf, e.g. env zones. */
function groupsOf(map, prefix, leaf) {
  return [...new Set([...map.keys()].filter((p) => p.startsWith(prefix) && p.endsWith(`.${leaf}`)).map((p) => p.split('.')[1]))].sort();
}

/**
 * Generates every output. Returns {files: Map(repoRelPath → string|Buffer), loaded} where `loaded` is the
 * result of loadContexts (used by checks). Throws if Style Dictionary cannot resolve a context.
 */
export async function generate({ designDir = DESIGN_DIR, loaded = loadContexts(designDir) } = {}) {
  const version = tokensVersion(designDir);
  const resolved = {};
  for (const name of Object.keys(CONTEXTS)) resolved[name] = await resolveWithStyleDictionary(loaded.contexts[name].tokens);
  const variants = businessVariants(loaded.contexts.game.tokens);
  const files = new Map();
  const web = { light: resolved['web-light'], dark: resolved['web-dark'], compact: resolved['web-compact'] };

  files.set(OUTPUTS.antdTheme, formatAntdTheme(web, variants, version));
  files.set(OUTPUTS.tokensCss, formatCssVars(web, version));

  const game = resolved.game;
  const hc = resolved['game-high-contrast'];
  files.set(`${OUTPUTS.godotThemeDir}/${themeFile()}`, formatTheme(game, BASE_DENSITY, version, `base density ${BASE_DENSITY}`));
  for (const band of DENSITY_BANDS) files.set(`${OUTPUTS.godotThemeDir}/${themeFile(band)}`, formatTheme(game, band, version, `density ${band}`));
  files.set(`${OUTPUTS.godotThemeDir}/${HIGH_CONTRAST_THEME}`, formatTheme(hc, BASE_DENSITY, version, `high contrast, density ${BASE_DENSITY}`));
  files.set(OUTPUTS.uiTokens, formatUiTokens(game, variants, version));

  const zones = groupsOf(game, 'env.', 'fog_color');
  const kinds = groupsOf(game, 'vfx.', 'gradient');
  for (const z of zones) files.set(`${OUTPUTS.artDir}/env_${z}.tres`, formatEnvironment(game, z, version));
  for (const k of kinds) files.set(`${OUTPUTS.artDir}/vfx_${k}_gradient.tres`, formatGradient(game, k, version));
  const slots = artPaletteSlots(game, loaded.contexts.game.tokens);
  for (const [name, opts] of Object.entries(TOON_MATERIALS)) files.set(`${OUTPUTS.artDir}/mat_${name}.tres`, formatToonMaterial(game, version, { label: name, slots, ...opts }));
  files.set(`${OUTPUTS.artDir}/art3d_tokens.gd`, formatArt3dTokens(game, zones, kinds, version, slots, Object.keys(TOON_MATERIALS)));

  for (const f of Object.values(godotFontFiles())) {
    files.set(f.dest, fs.readFileSync(path.join(designDir, f.src)));
    const license = path.join(path.dirname(f.src), 'LICENSE');
    const family = path.basename(path.dirname(f.src));
    files.set(`${OUTPUTS.godotFontsDir}/${family}-OFL.txt`, fs.readFileSync(path.join(designDir, license)));
  }

  const intermediate = new Map([[`${BUILD_DIR}/godot/tokens.json`, formatGodotJson({ game, 'game-high-contrast': hc }, version)]]);
  return { files, intermediate, loaded, resolved, version };
}

export function writeFiles(files, root) {
  for (const [rel, content] of files) {
    const abs = path.join(root, rel);
    fs.mkdirSync(path.dirname(abs), { recursive: true });
    fs.writeFileSync(abs, content);
  }
}
