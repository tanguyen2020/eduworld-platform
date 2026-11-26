// Formats: GDScript constant classes (F005 contracts §5).
//   src/game/scripts/core/ui_tokens.gd            class_name UiTokens  (UI: spacing, durations, colors, density, business colors)
//   src/game/resources/art/generated/art3d_tokens.gd class_name Art3dTokens (zone palette, toon, outline, light, rarity)
import { GENERATED_MARK, BUSINESS_COLOR_PREFIXES, DENSITY_BANDS, BASE_DENSITY, OUTPUTS } from '../../style-dictionary.config.mjs';
import { godotColor } from '../../lib/color.mjs';
import { byPath, gdConstName, gdFloat, px, seconds } from '../../lib/names.mjs';

/** Top-level groups that belong to 3D art (Art3dTokens) rather than UI (UiTokens). */
export const ART_GROUPS = ['art', 'avatar', 'zone', 'toon', 'outline', 'light', 'env', 'vfx'];

export const resPath = (repoRel) => `res://${repoRel.replace(/^src\/game\//, '')}`;

export const themeFile = (band) => (band ? `eduworld_theme_${band}.tres` : 'eduworld_theme.tres');
export const HIGH_CONTRAST_THEME = 'eduworld_theme_high_contrast.tres';

/** GDScript `const` declaration for one token, or null when the type has no constant form. */
export function gdConst(tokenPath, { value, type }) {
  const name = gdConstName(tokenPath);
  switch (type) {
    case 'color': return `const ${name}: Color = ${godotColor(value)}`;
    case 'dimension': return `const ${name}: int = ${Math.round(px(value))}`;
    case 'number': return `const ${name}: float = ${gdFloat(value)}`;
    case 'duration': return `const ${name}: float = ${gdFloat(seconds(value))}`;
    case 'fontWeight': return `const ${name}: int = ${value}`;
    default: return null; // fontFamily, gradient, shadow: carried by Theme / Gradient resources
  }
}

function header(cls, doc, version) {
  return `# ${GENERATED_MARK}. Source: design/game-tokens, design/shared-tokens (tokens@${version}). Run make design-tokens.
class_name ${cls}
extends RefCounted
## ${doc}

const TOKENS_VERSION: String = "${version}"
`;
}

function sections(map, paths) {
  const groups = new Map();
  for (const p of paths) {
    const line = gdConst(p, map.get(p));
    if (!line) continue;
    const g = p.split('.')[0];
    if (!groups.has(g)) groups.set(g, []);
    groups.get(g).push(line);
  }
  return [...groups].map(([g, lines]) => `\n# ${g}\n${lines.join('\n')}`).join('\n');
}

function dict(name, entries) {
  return `const ${name}: Dictionary = {\n${entries.map(([k, v]) => `\t"${k}": ${v},`).join('\n')}\n}`;
}

/** @param {Map} map resolved game context; @param {Map} variants path → variants for business colors */
export function formatUiTokens(map, variants, version) {
  const paths = [...map.keys()].sort(byPath).filter((p) => !ART_GROUPS.includes(p.split('.')[0]));
  const business = [...variants.keys()].sort(byPath).filter((p) => BUSINESS_COLOR_PREFIXES.some((pre) => p.startsWith(pre)));
  const themeDir = resPath(OUTPUTS.godotThemeDir);
  return `${header('UiTokens', 'Game UI design tokens. Style Controls through the Theme; read these constants only when code needs a value (spacing, tween duration, layout allowance, business color).', version)}${sections(map, paths)}

# business colors (shared-tokens, game variant) keyed by the token name used by the web (domainColors)
${dict('DOMAIN_COLORS', business.map((p) => [p, godotColor(variants.get(p).game)]))}

# density
const DENSITY_BANDS: Array[String] = [${DENSITY_BANDS.map((b) => `"${b}"`).join(', ')}]
const BASE_DENSITY: String = "${BASE_DENSITY}"
${dict('DENSITY_FONT_BASE', DENSITY_BANDS.map((b) => [b, String(Math.round(px(map.get(`density.${b}.font.base`).value)))]))}
${dict('DENSITY_BUTTON_MIN_HEIGHT', DENSITY_BANDS.map((b) => [b, String(Math.round(px(map.get(`density.${b}.button.min_height`).value)))]))}
${dict('DENSITY_FONT_STEM', DENSITY_BANDS.map((b) => [b, String(Math.round(px(map.get(`density.${b}.font.stem`).value)))]))}
${dict('DENSITY_ANSWER_MIN_HEIGHT', DENSITY_BANDS.map((b) => [b, String(Math.round(px(map.get(`density.${b}.answer.min_height`).value)))]))}
${dict('DENSITY_FONT_SMALL', DENSITY_BANDS.map((b) => [b, String(Math.round(px(map.get(`density.${b}.font.small`).value)))]))}

# generated Theme resources
const BASE_THEME_PATH: String = "${themeDir}/${themeFile()}"
const HIGH_CONTRAST_THEME_PATH: String = "${themeDir}/${HIGH_CONTRAST_THEME}"
${dict('THEME_PATHS', DENSITY_BANDS.map((b) => [b, `"${themeDir}/${themeFile(b)}"`]))}
`;
}

export function formatArt3dTokens(map, envZones, vfxKinds, version, slots = [], materials = []) {
  const paths = [...map.keys()].sort(byPath).filter((p) => ['zone', 'toon', 'outline', 'light'].includes(p.split('.')[0]));
  const rarity = [...map.keys()].sort(byPath).filter((p) => p.startsWith('color.rarity.'));
  const avatar = (group) => [...map.keys()].sort(byPath).filter((p) => p.startsWith(`avatar.${group}.`))
    .map((p) => [p.slice(`avatar.${group}.`.length), godotColor(map.get(p).value)]);
  const artDir = resPath(OUTPUTS.artDir);
  return `${header('Art3dTokens', '3D art tokens: zone material palette, toon/outline shader uniforms, light, rarity glow, avatar skin/hair palette (DESIGN_SYSTEM §5.3).', version)}${sections(map, paths)}

# rarity glow / frame (shared-tokens, game variant)
${dict('RARITY_COLORS', rarity.map((p) => [p.slice('color.rarity.'.length), godotColor(map.get(p).value)]))}

# avatar appearance palette keyed by the catalog codes (skin_tones, hair_colors; F070 T041)
${dict('AVATAR_SKIN_TONES', avatar('skin_tone'))}
${dict('AVATAR_HAIR_COLORS', avatar('hair_color'))}

# art palette (F067, ADR-0102): slot order of the toon shader palette; a baked mesh addresses slot i with UV = (i + 0.5, 0.5)
const ART_PALETTE: Array[String] = [${slots.map((s) => `"${s.name}"`).join(', ')}]
${dict('ART_PALETTE_COLORS', slots.map((s) => [s.name, godotColor(s.color)]))}

# generated resources
${dict('MATERIAL_PATHS', materials.map((m) => [m, `"${artDir}/mat_${m}.tres"`]))}
${dict('ENVIRONMENT_PATHS', envZones.map((z) => [z, `"${artDir}/env_${z}.tres"`]))}
${dict('VFX_GRADIENT_PATHS', vfxKinds.map((k) => [k, `"${artDir}/vfx_${k}_gradient.tres"`]))}
`;
}
