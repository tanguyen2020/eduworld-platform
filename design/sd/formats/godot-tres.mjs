// Formats: Godot 4 text resources (format=3) written directly from resolved tokens (F005).
// Clarification 2026-11-19: the pipeline writes the .tres text itself instead of a Godot headless
// script, so `make design-tokens` / `-check` run identically without Godot. The first line is a `;`
// comment (VariantParser comment syntax) carrying the GENERATED marker.
import { GENERATED_MARK, GODOT_FONTS, OUTPUTS } from '../../style-dictionary.config.mjs';
import { godotColor } from '../../lib/color.mjs';
import { gdFloat, px } from '../../lib/names.mjs';
import { resPath } from './gdscript-tokens.mjs';

const mark = (what, version) => `; ${GENERATED_MARK}. ${what} (tokens@${version}). Run make design-tokens.`;

function resource(type, subs, ext, props, comment) {
  const loadSteps = subs.length + ext.length + 1;
  const steps = loadSteps > 1 ? ` load_steps=${loadSteps}` : '';
  const lines = [comment, `[gd_resource type="${type}"${steps} format=3]`, ''];
  for (const e of ext) lines.push(`[ext_resource type="${e.type}" path="${e.path}" id="${e.id}"]`);
  if (ext.length) lines.push('');
  for (const s of subs) {
    lines.push(`[sub_resource type="${s.type}" id="${s.id}"]`);
    for (const [k, v] of s.props) lines.push(`${k} = ${v}`);
    lines.push('');
  }
  lines.push('[resource]');
  for (const [k, v] of props) lines.push(`${k} = ${v}`);
  return lines.join('\n') + '\n';
}

function styleBox(id, { bg, border, borderWidth = 0, radius = 0, padH = 0, padV = 0 }) {
  const props = [
    ['content_margin_left', gdFloat(padH)],
    ['content_margin_top', gdFloat(padV)],
    ['content_margin_right', gdFloat(padH)],
    ['content_margin_bottom', gdFloat(padV)],
    ['bg_color', godotColor(bg)],
  ];
  if (borderWidth > 0) {
    for (const side of ['left', 'top', 'right', 'bottom']) props.push([`border_width_${side}`, String(borderWidth)]);
    props.push(['border_color', godotColor(border)]);
  }
  for (const c of ['top_left', 'top_right', 'bottom_right', 'bottom_left']) props.push([`corner_radius_${c}`, String(radius)]);
  return { type: 'StyleBoxFlat', id: `StyleBoxFlat_${id}`, props };
}

const sub = (id) => `SubResource("StyleBoxFlat_${id}")`;

/** Font files copied into the Godot project: {key → {res, file}}. */
export function godotFontFiles() {
  return Object.fromEntries(Object.entries(GODOT_FONTS).map(([k, f]) => {
    const name = f.file.split('/').pop();
    return [k, { src: f.file, dest: `${OUTPUTS.godotFontsDir}/${name}`, res: resPath(`${OUTPUTS.godotFontsDir}/${name}`) }];
  }));
}

/**
 * Theme for one density band. `map` is a resolved game context (normal or high contrast).
 * Button vertical padding approximates the minimum height: (min_height − 1.3 × font size) / 2.
 */
export function formatTheme(map, band, version, label) {
  const v = (p) => {
    if (!map.has(p)) throw new Error(`theme needs token ${p}`);
    return map.get(p).value;
  };
  const n = (p) => Math.round(px(v(p)));
  const font = n(`density.${band}.font.base`);
  const heading = n(`density.${band}.font.heading`);
  const minH = n(`density.${band}.button.min_height`);
  const space = n(`density.${band}.spacing.base`);
  const padV = Math.max(Math.round((minH - font * 1.3) / 2), 4);
  const btnRadius = n('component.button.primary.radius');
  const border = n('component.button.border_width');
  const focusW = n('component.focus_ring.width');
  const fonts = godotFontFiles();
  const ext = [
    { type: 'FontFile', path: fonts.body.res, id: '1_body' },
    { type: 'FontFile', path: fonts.bodyBold.res, id: '2_body_bold' },
    { type: 'FontFile', path: fonts.heading.res, id: '3_heading' },
  ];
  const btn = { radius: btnRadius, padH: space, padV };
  const subs = [
    styleBox('button_normal', { ...btn, bg: v('component.button.primary.bg') }),
    styleBox('button_hover', { ...btn, bg: v('component.button.primary.bg_pressed') }),
    styleBox('button_pressed', { ...btn, bg: v('component.button.primary.bg_pressed') }),
    styleBox('button_disabled', { ...btn, bg: v('component.progress.track') }),
    styleBox('button_focus', { ...btn, bg: v('component.button.primary.bg'), border: v('component.focus_ring.color'), borderWidth: focusW }),
    styleBox('secondary_normal', { ...btn, bg: v('component.button.secondary.bg'), border: v('component.button.secondary.border'), borderWidth: border }),
    styleBox('hint_normal', { ...btn, bg: v('component.hint_button.bg') }),
    styleBox('panel', { bg: v('component.panel.bg'), border: v('component.panel.border'), borderWidth: border, radius: n('component.panel.radius'), padH: n('component.panel.padding'), padV: n('component.panel.padding') }),
    styleBox('quest_card', { bg: v('component.quest_card.bg'), radius: n('component.quest_card.radius'), padH: space, padV: space }),
    styleBox('hud', { bg: v('component.hud.bg'), radius: btnRadius, padH: space, padV: Math.round(space / 2) }),
    styleBox('voice_caption', { bg: v('component.voice_caption.bg'), radius: btnRadius, padH: space, padV: Math.round(space / 2) }),
    styleBox('feedback_correct', { bg: v('color.feedback.correct'), radius: btnRadius, padH: space, padV: Math.round(space / 2) }),
    styleBox('feedback_incorrect', { bg: v('color.feedback.incorrect'), radius: btnRadius, padH: space, padV: Math.round(space / 2) }),
    styleBox('progress_bg', { bg: v('component.progress.track'), radius: btnRadius }),
    styleBox('progress_fill', { bg: v('component.progress.fill'), radius: btnRadius }),
    styleBox('line_edit', { bg: v('color.surface.panel_alt'), border: v('component.button.secondary.border'), borderWidth: border, radius: btnRadius, padH: space, padV: Math.round(space / 2) }),
    styleBox('line_edit_focus', { bg: v('color.surface.panel_alt'), border: v('component.focus_ring.color'), borderWidth: focusW, radius: btnRadius, padH: space, padV: Math.round(space / 2) }),
  ];
  const c = (p) => godotColor(v(p));
  const props = [
    ['default_font', 'ExtResource("1_body")'],
    ['default_font_size', String(font)],
    ['Button/colors/font_color', c('component.button.primary.text')],
    ['Button/colors/font_disabled_color', c('color.text.secondary')],
    ['Button/colors/font_focus_color', c('component.button.primary.text')],
    ['Button/colors/font_hover_color', c('component.button.primary.text')],
    ['Button/colors/font_pressed_color', c('component.button.primary.text')],
    ['Button/font_sizes/font_size', String(font)],
    ['Button/fonts/font', 'ExtResource("2_body_bold")'],
    ['Button/styles/disabled', sub('button_disabled')],
    ['Button/styles/focus', sub('button_focus')],
    ['Button/styles/hover', sub('button_hover')],
    ['Button/styles/normal', sub('button_normal')],
    ['Button/styles/pressed', sub('button_pressed')],
    ['ExamClockWarningLabel/base_type', '&"Label"'],
    ['ExamClockWarningLabel/colors/font_color', c('component.exam_clock.warning_text')],
    ['FeedbackCorrectPanel/base_type', '&"PanelContainer"'],
    ['FeedbackCorrectPanel/styles/panel', sub('feedback_correct')],
    ['FeedbackIncorrectPanel/base_type', '&"PanelContainer"'],
    ['FeedbackIncorrectPanel/styles/panel', sub('feedback_incorrect')],
    ['FeedbackLabel/base_type', '&"Label"'],
    ['FeedbackLabel/colors/font_color', c('color.text.on_accent')],
    ['HeadingLabel/base_type', '&"Label"'],
    ['HeadingLabel/colors/font_color', c('color.text.primary')],
    ['HeadingLabel/font_sizes/font_size', String(heading)],
    ['HeadingLabel/fonts/font', 'ExtResource("3_heading")'],
    ['HintButton/base_type', '&"Button"'],
    ['HintButton/colors/font_color', c('component.hint_button.text')],
    ['HintButton/styles/normal', sub('hint_normal')],
    ['HudLabel/base_type', '&"Label"'],
    ['HudLabel/colors/font_color', c('component.hud.text')],
    ['HudPanel/base_type', '&"PanelContainer"'],
    ['HudPanel/styles/panel', sub('hud')],
    ['Label/colors/font_color', c('color.text.primary')],
    ['Label/font_sizes/font_size', String(font)],
    ['LineEdit/colors/font_color', c('color.text.primary')],
    ['LineEdit/colors/font_placeholder_color', c('color.text.muted')],
    ['LineEdit/font_sizes/font_size', String(font)],
    ['LineEdit/styles/focus', sub('line_edit_focus')],
    ['LineEdit/styles/normal', sub('line_edit')],
    ['MutedLabel/base_type', '&"Label"'],
    ['MutedLabel/colors/font_color', c('color.text.muted')],
    ['Panel/styles/panel', sub('panel')],
    ['PanelContainer/styles/panel', sub('panel')],
    ['ProgressBar/styles/background', sub('progress_bg')],
    ['ProgressBar/styles/fill', sub('progress_fill')],
    ['QuestCard/base_type', '&"PanelContainer"'],
    ['QuestCard/styles/panel', sub('quest_card')],
    ['RichTextLabel/colors/default_color', c('color.text.primary')],
    ['RichTextLabel/font_sizes/normal_font_size', String(font)],
    ['SecondaryButton/base_type', '&"Button"'],
    ['SecondaryButton/colors/font_color', c('component.button.secondary.text')],
    ['SecondaryButton/colors/font_hover_color', c('component.button.secondary.text')],
    ['SecondaryButton/colors/font_pressed_color', c('component.button.secondary.text')],
    ['SecondaryButton/styles/normal', sub('secondary_normal')],
    ['VoiceCaption/base_type', '&"PanelContainer"'],
    ['VoiceCaption/styles/panel', sub('voice_caption')],
    ['VoiceCaptionLabel/base_type', '&"Label"'],
    ['VoiceCaptionLabel/colors/font_color', c('component.voice_caption.text')],
  ];
  return resource('Theme', subs, ext, props, mark(`EduWorld game Theme, ${label}`, version));
}

/** Environment per zone: procedural sky, sky ambient, filmic tonemap and fog (DESIGN_SYSTEM §5.3, F067 light tokens). */
export function formatEnvironment(map, zone, version) {
  const c = (k) => godotColor(map.get(`env.${zone}.${k}`).value);
  const n = (p) => gdFloat(Number(map.get(p).value));
  const subs = [
    { type: 'ProceduralSkyMaterial', id: 'ProceduralSkyMaterial_sky', props: [['sky_top_color', c('sky_top')], ['sky_horizon_color', c('sky_horizon')], ['ground_horizon_color', c('ground_horizon')]] },
    { type: 'Sky', id: 'Sky_main', props: [['sky_material', 'SubResource("ProceduralSkyMaterial_sky")']] },
  ];
  const props = [
    ['background_mode', '2'],
    ['sky', 'SubResource("Sky_main")'],
    ['ambient_light_source', '2'],
    ['ambient_light_color', godotColor(map.get('light.ambient_color').value)],
    ['ambient_light_sky_contribution', '0.0'],
    ['ambient_light_energy', n('light.ambient_energy')],
    ['tonemap_mode', '0'],
    ['tonemap_exposure', n('light.exposure')],
    ['fog_enabled', 'true'],
    ['fog_light_color', c('fog_color')],
    ['fog_density', n('light.fog_density')],
  ];
  return resource('Environment', subs, [], props, mark(`Environment for zone "${zone}"`, version));
}

/** Gradient for a VFX kind (DESIGN_SYSTEM §5.3). */
export function formatGradient(map, kind, version) {
  const stops = map.get(`vfx.${kind}.gradient`).value;
  const offsets = stops.map((s) => gdFloat(s.position).replace(/\.0$/, ''));
  const colors = stops.flatMap((s) => godotColor(s.color).replace(/^Color\(|\)$/g, '').split(', '));
  const props = [['offsets', `PackedFloat32Array(${offsets.join(', ')})`], ['colors', `PackedColorArray(${colors.join(', ')})`]];
  return resource('Gradient', [], [], props, mark(`VFX gradient "${kind}"`, version));
}

/** Shared toon shaders (hand-written, MIT — ADR-0102); the generated presets only carry token uniforms. */
export const TOON_SHADER = 'res://resources/art/shaders/toon.gdshader';
export const OUTLINE_SHADER = 'res://resources/art/shaders/toon_outline.gdshader';
/** Size of the toon shader palette array (`uniform vec4 palette[64]`). */
export const PALETTE_SLOTS_MAX = 64;

/**
 * Ordered palette slots `[{name, color}]` (F067): `art.palette.<name>` tokens sorted by `eduworld.index`.
 * Throws when an index is missing, duplicated or leaves a gap (a slot is addressed by baked meshes).
 */
export function artPaletteSlots(map, tokens) {
  const slots = [...map.keys()].filter((p) => p.startsWith('art.palette.')).map((p) => {
    const index = tokens.get(p)?.extensions?.index;
    if (!Number.isInteger(index)) throw new Error(`${p} needs $extensions.eduworld.index`);
    return { name: p.slice('art.palette.'.length), color: map.get(p).value, index };
  }).sort((a, b) => a.index - b.index);
  slots.forEach((s, i) => { if (s.index !== i) throw new Error(`art.palette index ${i} missing or duplicated (found ${s.index} for ${s.name})`); });
  if (slots.length > PALETTE_SLOTS_MAX) throw new Error(`art.palette has ${slots.length} slots, max ${PALETTE_SLOTS_MAX}`);
  return slots;
}

/**
 * Toon material preset (DESIGN_SYSTEM §5.3): shared toon shader with the art palette and the toon tokens, plus
 * the inverted-hull outline as next pass. `smoothNormals`: baked meshes carry smoothed normals in COLOR for a
 * crack-free outline.
 */
export function formatToonMaterial(map, version, { label, slots, smoothNormals = true, outline = true }) {
  const v = (p) => {
    if (!map.has(p)) throw new Error(`toon material needs token ${p}`);
    return map.get(p).value;
  };
  const ext = [
    { type: 'Shader', path: TOON_SHADER, id: '1_toon' },
  ];
  const subs = [];
  if (outline) {
    ext.push({ type: 'Shader', path: OUTLINE_SHADER, id: '2_outline' });
    subs.push({ type: 'ShaderMaterial', id: 'ShaderMaterial_outline', props: [
      ['render_priority', '0'],
      ['shader', 'ExtResource("2_outline")'],
      ['shader_parameter/outline_color', godotColor(v('outline.color'))],
      ['shader_parameter/outline_width', gdFloat(Number(v('outline.width')))],
      ['shader_parameter/smooth_normals_in_color', String(smoothNormals)],
    ] });
  }
  const props = [['render_priority', '0']];
  if (outline) props.push(['next_pass', 'SubResource("ShaderMaterial_outline")']);
  props.push(
    ['shader', 'ExtResource("1_toon")'],
    ['shader_parameter/palette', `PackedColorArray(${slots.flatMap((s) => colorFloats(s.color)).join(', ')})`],
    ['shader_parameter/palette_size', String(slots.length)],
    ['shader_parameter/albedo', godotColor(v('palette.white'))],
    ['shader_parameter/toon_steps', gdFloat(Number(v('toon.steps')))],
    ['shader_parameter/shadow_tint', godotColor(v('toon.shadow_tint'))],
    ['shader_parameter/rim_strength', gdFloat(Number(v('toon.rim_strength')))],
  );
  return resource('ShaderMaterial', subs, ext, props, mark(`Toon material preset "${label}"`, version));
}

/** '#RRGGBB' → ['r', 'g', 'b', 'a'] float strings of a Godot Color (flat form of PackedColorArray). */
function colorFloats(hex) {
  return godotColor(hex).slice('Color('.length, -1).split(', ');
}
