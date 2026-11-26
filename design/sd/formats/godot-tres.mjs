// Formats: Godot 4 text resources (format=3) written directly from resolved tokens (F005).
// Clarification 2026-11-19: the pipeline writes the .tres text itself instead of a Godot headless
// script, so `make design-tokens` / `-check` run identically without Godot. The first line is a `;`
// comment (VariantParser comment syntax) carrying the GENERATED marker.
import path from 'node:path';
import * as fontkit from 'fontkit';
import { GENERATED_MARK, GODOT_FONTS, OUTPUTS } from '../../style-dictionary.config.mjs';
import { DESIGN_DIR } from '../../lib/tokens.mjs';
import { godotColor } from '../../lib/color.mjs';
import { gdFloat, px } from '../../lib/names.mjs';
import { resPath } from './gdscript-tokens.mjs';

const iconRes = (name) => resPath(`${OUTPUTS.godotIconsDir}/${name}.svg`);

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

/**
 * StyleBoxFlat. `bg` omitted → `draw_center = false` (outline only: focus ring, flat buttons). `shadow` adds a
 * soft drop shadow (cards); `expand` draws the box that far outside the control (focus ring with a gap).
 * `borderBottom` thickens the bottom edge (F097 answer tiles: candy depth); `padL` widens the left content margin
 * (room for the number badge of an answer tile); `padT` a tall top margin (picture tiles: picture above the label).
 * F114 cartoon outline: `padB` a bottom margin of its own (room for the hard bottom edge), `sides` border widths per
 * side (tab column: right border only), `expandTop` a negative value draws the box lower than the control — the face
 * of a pressed control sinks while the control keeps its height.
 */
function styleBox(id, { bg, border, borderWidth = 0, borderBottom, sides, radius = 0, padH = 0, padV = 0, padL, padT, padB, shadow, shadowSize = 0, expand = 0, expandTop }) {
  const props = [
    ['content_margin_left', gdFloat(padL ?? padH)],
    ['content_margin_top', gdFloat(padT ?? padV)],
    ['content_margin_right', gdFloat(padH)],
    ['content_margin_bottom', gdFloat(padB ?? padV)],
  ];
  if (bg === undefined) props.push(['draw_center', 'false']);
  else props.push(['bg_color', godotColor(bg)]);
  if (sides) {
    for (const side of ['left', 'top', 'right', 'bottom']) if (sides[side] > 0) props.push([`border_width_${side}`, String(sides[side])]);
    props.push(['border_color', godotColor(border)]);
  } else if (borderWidth > 0) {
    for (const side of ['left', 'top', 'right', 'bottom']) props.push([`border_width_${side}`, String(side === 'bottom' && borderBottom !== undefined ? borderBottom : borderWidth)]);
    props.push(['border_color', godotColor(border)]);
  }
  for (const c of ['top_left', 'top_right', 'bottom_right', 'bottom_left']) props.push([`corner_radius_${c}`, String(radius)]);
  if (expand > 0) for (const side of ['left', 'top', 'right', 'bottom']) props.push([`expand_margin_${side}`, gdFloat(expand)]);
  if (expandTop !== undefined && expandTop !== 0) props.push(['expand_margin_top', gdFloat(expandTop)]);
  if (shadow !== undefined && shadowSize > 0) {
    props.push(['shadow_color', godotColor(shadow)], ['shadow_size', String(shadowSize)], ['shadow_offset', `Vector2(0, ${gdFloat(Math.round(shadowSize / 2))})`]);
  }
  return { type: 'StyleBoxFlat', id: `StyleBoxFlat_${id}`, props };
}

/** Game icons written by the pipeline (formatIcon) and referenced by the Theme (CheckBox boxes). */
export const THEME_ICONS = { checked: 'checkbox_on', unchecked: 'checkbox_off', sliderKnob: 'slider_knob', toggleOn: 'toggle_on', toggleOff: 'toggle_off' };

const sub = (id) => `SubResource("StyleBoxFlat_${id}")`;

/**
 * Line height Godot gives a single line of the body font at `size` px: ascent + descent of the font file, each
 * rounded up (FreeType metrics). Used so controls reach the band's minimum height exactly.
 */
export function bodyLineHeight(size, key = 'body') {
  const font = fontkit.openSync(path.join(DESIGN_DIR, GODOT_FONTS[key].file));
  return Math.ceil((font.ascent * size) / font.unitsPerEm) + Math.ceil((-font.descent * size) / font.unitsPerEm);
}

/** Font files copied into the Godot project: {key → {res, file}}. */
export function godotFontFiles() {
  return Object.fromEntries(Object.entries(GODOT_FONTS).map(([k, f]) => {
    const name = f.file.split('/').pop();
    return [k, { src: f.file, dest: `${OUTPUTS.godotFontsDir}/${name}`, res: resPath(`${OUTPUTS.godotFontsDir}/${name}`) }];
  }));
}

/**
 * Theme for one density band. `map` is a resolved game context (normal or high contrast).
 * Button and field vertical padding reaches the minimum height: (min_height − body line height) / 2, rounded up.
 */
export function formatTheme(map, band, version, label) {
  const v = (p) => {
    if (!map.has(p)) throw new Error(`theme needs token ${p}`);
    return map.get(p).value;
  };
  const n = (p) => Math.round(px(v(p)));
  const font = n(`density.${band}.font.base`);
  const heading = n(`density.${band}.font.heading`);
  const small = n(`density.${band}.font.small`);
  const minH = n(`density.${band}.button.min_height`);
  const space = n(`density.${band}.spacing.base`);
  const padV = Math.max(Math.ceil((minH - bodyLineHeight(font)) / 2), 4);
  const icon = n('component.icon.size');
  const box = n('component.icon.checkbox_size');
  // Square icon-only buttons and checkbox rows reach the band's minimum touch target (DESIGN_SYSTEM §5.2).
  const padIcon = Math.max(Math.round((minH - icon) / 2), 4);
  const padBox = Math.max(Math.round((minH - box) / 2), 4);
  const space4 = n('size.space.4');
  const halfSpace = Math.round(space / 2);
  // Gap between the groups of a form (label + field, button, checkbox…): ¾ of the band spacing keeps a full S01
  // card with an error message inside a 720 px tall window.
  const formGap = Math.round(space * 0.75);
  const btnRadius = n('component.button.primary.radius');
  const border = n('component.button.border_width');
  const focusW = n('component.focus_ring.width');
  const fonts = godotFontFiles();
  const ext = [
    { type: 'FontFile', path: fonts.body.res, id: '1_body' },
    { type: 'FontFile', path: fonts.bodyBold.res, id: '2_body_bold' },
    { type: 'FontFile', path: fonts.heading.res, id: '3_heading' },
    { type: 'Texture2D', path: iconRes(THEME_ICONS.checked), id: '4_checkbox_on' },
    { type: 'Texture2D', path: iconRes(THEME_ICONS.unchecked), id: '5_checkbox_off' },
    { type: 'Texture2D', path: iconRes(THEME_ICONS.sliderKnob), id: '6_slider_knob' },
    { type: 'Texture2D', path: iconRes(THEME_ICONS.toggleOn), id: '7_toggle_on' },
    { type: 'Texture2D', path: iconRes(THEME_ICONS.toggleOff), id: '8_toggle_off' },
  ];
  const btn = { radius: btnRadius, padH: space, padV };
  // F114 tiered cartoon outline (DESIGN_SYSTEM §5.1, §5.2): one ink colour; tier 1 (interactive controls, window
  // frames) = outline + hard bottom edge, tier 2 (information surfaces) = thin outline only, tier 3 (stem, question
  // card, answer tiles, TextButton) unchanged.
  const ink = v('color.outline');
  const o = n(`density.${band}.outline.interactive`);
  const oi = n(`density.${band}.outline.info`);
  const d = n(`density.${band}.depth.rest`);
  const dp = n(`density.${band}.depth.pressed`);
  const sink = d - dp;
  const lineH = bodyLineHeight(font);
  /**
   * Rest and pressed geometry of a tier 1 control whose content line is `inner` px tall: the content stays centred
   * on the face above the bottom edge and the control keeps the band's minimum height; pressed, the edge shrinks to
   * `depth.pressed`, the face is drawn `sink` px lower and the content moves down by the same amount.
   */
  const raised = (base, inner, minTop = 4) => {
    const room = Math.max(minH - inner, 2 * minTop + d);
    const top = Math.max(Math.floor((room - d) / 2), minTop);
    const bottom = Math.max(room - top, top + d);
    return {
      rest: { ...base, border: ink, borderWidth: o, borderBottom: o + d, padT: top, padB: bottom },
      pressed: { ...base, border: ink, borderWidth: o, borderBottom: o + dp, padT: top + sink, padB: bottom - sink, expandTop: -sink },
    };
  };
  const tb = raised(btn, lineH);
  const iconLine = Math.max(icon, lineH);
  const ib = raised({ radius: btnRadius, padH: padIcon }, iconLine);
  const cb = raised({ radius: btnRadius, padH: space }, iconLine);
  const message = { radius: btnRadius, borderWidth: oi, padH: space, padV: halfSpace };
  // F097 activity player: answer tiles (6 colour sets), question card, drop zones, soft verdict panels.
  const stem = n(`density.${band}.font.stem`);
  const tileH = n(`density.${band}.answer.min_height`);
  const tileR = n('component.answer_tile.radius');
  const edge = n('component.answer_tile.edge');
  const badge = n('component.answer_tile.badge_size');
  const tilePadV = Math.max(Math.ceil((tileH - Math.min(bodyLineHeight(stem), bodyLineHeight(stem, 'bodyBold'))) / 2), 4);
  const resultW = n('component.answer_tile.result_border');
  // Tag pinned on a tile (F098): a pill of the band's small font, half the band spacing around the words.
  const tag = { radius: tileR, padH: halfSpace, padV: Math.round(space4 / 2) };
  const tile = { radius: tileR, borderWidth: border, borderBottom: edge, padH: space, padV: tilePadV, padL: space + badge + halfSpace };
  const zone = { radius: tileR, borderWidth: border, borderBottom: edge, padH: space, padV: tilePadV };
  const shadow = { shadow: v('component.form_card.shadow'), shadowSize: n('component.form_card.shadow_size') };
  // Picture tile (S02 presets): the picture fills the top margin (1.5 × the band's touch target), the label below.
  const picTop = Math.round(minH * 1.5) + halfSpace;
  const picture = { radius: btnRadius, padH: halfSpace, border: ink, borderWidth: o };
  const pictureRest = { ...picture, borderBottom: o + d, padT: picTop - d, padB: halfSpace + d };
  const picturePressed = { ...picture, borderBottom: o + dp, padT: picTop - d + sink, padB: halfSpace + d - sink, expandTop: -sink };
  // Window frames (panel, dialog, title bar): outline and bottom edge, never pressed.
  const frame = { border: ink, borderWidth: o, borderBottom: o + d };
  const titleBar = { ...frame, radius: btnRadius, padH: space, padT: halfSpace, padB: halfSpace + d };
  // Slider groove and filled part: outline + edge; the groove height comes from the content margins.
  const groove = { ...frame, radius: btnRadius, padH: 0, padT: halfSpace, padB: halfSpace + d };
  const subs = [
    styleBox('button_normal', { ...tb.rest, bg: v('component.button.primary.bg') }),
    styleBox('button_hover', { ...tb.rest, bg: v('component.button.primary.bg_pressed') }),
    styleBox('button_pressed', { ...tb.pressed, bg: v('component.button.primary.bg_pressed') }),
    styleBox('button_disabled', { ...tb.rest, bg: v('component.progress.track') }),
    styleBox('secondary_normal', { ...tb.rest, bg: v('component.button.secondary.bg') }),
    styleBox('secondary_hover', { ...tb.rest, bg: v('component.button.secondary.bg_pressed') }),
    styleBox('secondary_pressed', { ...tb.pressed, bg: v('component.button.secondary.bg_pressed') }),
    styleBox('danger_normal', { ...tb.rest, bg: v('component.button.danger.bg') }),
    styleBox('danger_hover', { ...tb.rest, bg: v('component.button.danger.bg_pressed') }),
    styleBox('danger_pressed', { ...tb.pressed, bg: v('component.button.danger.bg_pressed') }),
    styleBox('hint_normal', { ...tb.rest, bg: v('component.hint_button.bg') }),
    styleBox('hint_pressed', { ...tb.pressed, bg: v('component.hint_button.bg') }),
    styleBox('panel', { ...frame, bg: v('component.panel.bg'), border: v('component.panel.border'), radius: n('component.panel.radius'), padH: n('component.panel.padding'), padV: n('component.panel.padding') }),
    styleBox('quest_card', { bg: v('component.quest_card.bg'), radius: n('component.quest_card.radius'), padH: space, padV: space }),
    styleBox('hud', { bg: v('component.hud.bg'), border: ink, borderWidth: oi, radius: btnRadius, padH: space, padV: Math.round(space / 2) }),
    styleBox('voice_caption', { bg: v('component.voice_caption.bg'), radius: btnRadius, padH: space, padV: Math.round(space / 2) }),
    styleBox('feedback_correct', { bg: v('component.feedback.correct_bg'), border: v('component.feedback.correct_border'), borderWidth: border, radius: btnRadius, padH: space, padV: Math.round(space / 2) }),
    styleBox('feedback_incorrect', { bg: v('component.feedback.try_bg'), border: v('component.feedback.try_border'), borderWidth: border, radius: btnRadius, padH: space, padV: Math.round(space / 2) }),
    ...[1, 2, 3, 4, 5, 6].flatMap((i) => [
      styleBox(`tile${i}_normal`, { ...tile, bg: v(`component.answer_tile.bg_${i}`), border: v(`component.answer_tile.accent_${i}`) }),
      styleBox(`tile${i}_hover`, { ...tile, borderWidth: border + 1, bg: v(`component.answer_tile.bg_${i}`), border: v(`component.answer_tile.accent_${i}`) }),
      styleBox(`tile${i}_pressed`, { ...tile, bg: v(`component.answer_tile.accent_${i}`), border: v('component.progress_trail.outline') }),
    ]),
    // F098 tap-to-answer: graded tiles (right / not yet right / right answer) and the tags pinned on them.
    styleBox('tile_correct', { ...tile, borderWidth: resultW, borderBottom: edge, bg: v('component.answer_tile.correct_bg'), border: v('component.answer_tile.correct_border') }),
    styleBox('tile_try', { ...tile, borderWidth: resultW, borderBottom: edge, bg: v('component.answer_tile.try_bg'), border: v('component.answer_tile.try_border') }),
    // F099: resting tile after grading (neither chosen nor the right answer), caption card, video screen.
    styleBox('tile_rest', { ...tile, bg: v('component.answer_tile.rest_bg'), border: v('component.answer_tile.rest_border') }),
    styleBox('caption_card', { bg: v('component.caption.bg'), border: v('component.caption.border'), borderWidth: border, radius: tileR, padH: space, padV: halfSpace }),
    styleBox('media_stage', { bg: v('component.media_stage.bg'), radius: tileR }),
    styleBox('tag_correct', { ...tag, bg: v('component.answer_tile.tag_correct_bg') }),
    styleBox('tag_try', { ...tag, bg: v('component.answer_tile.tag_try_bg') }),
    styleBox('tag_pending', { ...tag, borderWidth: border, bg: v('component.answer_tile.tag_pending_bg'), border: v('component.answer_tile.tag_pending_border') }),
    styleBox('picture_normal', { ...pictureRest, bg: v('component.choice_chip.bg') }),
    styleBox('picture_hover', { ...pictureRest, bg: v('component.button.secondary.bg_hover') }),
    styleBox('picture_selected', { ...picturePressed, bg: v('component.choice_chip.selected_bg') }),
    styleBox('question_card', { bg: v('component.question_card.bg'), border: v('component.question_card.border'), borderWidth: border, radius: tileR, padH: space, padV: space, ...shadow }),
    styleBox('drop_zone', { ...zone, bg: v('component.drop_zone.bg'), border: v('component.drop_zone.border') }),
    styleBox('drop_zone_hover', { ...zone, bg: v('component.drop_zone.bg_hover'), border: v('component.drop_zone.border') }),
    styleBox('drop_zone_pressed', { ...zone, bg: v('component.drop_zone.bg_hover'), border: v('color.accent.primary') }),
    styleBox('activity_board', { bg: v('component.activity_board.bg'), border: v('component.activity_board.border'), borderWidth: border, radius: n('component.panel.radius'), padH: space, padV: space, ...shadow }),
    styleBox('progress_bg', { bg: v('component.progress.track'), border: ink, borderWidth: oi, radius: btnRadius }),
    styleBox('progress_fill', { bg: v('component.progress.fill'), border: ink, borderWidth: oi, radius: btnRadius }),
    // Single-line fields are as tall as the band's minimum control height (S01: ≥ 48 px, g1_3 ≥ 64 px).
    styleBox('line_edit', { bg: v('color.surface.panel_alt'), border: v('component.button.secondary.border'), borderWidth: border, radius: btnRadius, padH: space, padV }),
    styleBox('line_edit_focus', { bg: v('color.surface.panel_alt'), border: v('component.focus_ring.color'), borderWidth: focusW, radius: btnRadius, padH: space, padV }),
    styleBox('line_edit_read_only', { bg: v('color.surface.panel_alt'), border: v('component.progress.track'), borderWidth: border, radius: btnRadius, padH: space, padV }),
    // Keyboard/gamepad focus: an outline drawn just outside the control (visible on filled and outlined buttons).
    styleBox('focus_ring', { border: v('component.focus_ring.color'), borderWidth: focusW, radius: btnRadius + focusW, expand: focusW + Math.round(space4 / 2) }),
    styleBox('icon_normal', { ...ib.rest, bg: v('component.button.secondary.bg') }),
    styleBox('icon_hover', { ...ib.rest, bg: v('component.button.secondary.bg_pressed') }),
    styleBox('icon_pressed', { ...ib.pressed, bg: v('component.button.secondary.bg_pressed') }),
    // F114: a chosen chip is pressed in (yellow, sunk face, short edge) with a tick; not chosen = white, raised.
    styleBox('chip_normal', { ...cb.rest, bg: v('component.choice_chip.bg') }),
    styleBox('chip_hover', { ...cb.rest, bg: v('component.button.secondary.bg_hover') }),
    styleBox('chip_selected', { ...cb.pressed, bg: v('component.choice_chip.selected_bg') }),
    styleBox('text_normal', { ...btn }),
    styleBox('text_hover', { ...btn, bg: v('component.button.secondary.bg_hover') }),
    styleBox('text_pressed', { ...btn, bg: v('component.button.secondary.bg_pressed') }),
    styleBox('checkbox', { radius: btnRadius, padH: space4, padV: padBox }),
    styleBox('checkbox_hover', { radius: btnRadius, padH: space4, padV: padBox, bg: v('component.button.secondary.bg_hover') }),
    styleBox('inline_error', { ...message, bg: v('component.inline_error.bg'), border: v('component.inline_error.border') }),
    styleBox('info_banner', { ...message, bg: v('component.info_banner.bg'), border: v('component.info_banner.border') }),
    styleBox('form_card', { ...frame, bg: v('component.form_card.bg'), border: v('component.form_card.border'), radius: n('component.panel.radius'), padH: n('component.form_card.padding'), padV: n('component.form_card.padding') }),
    styleBox('speech_bubble', { bg: v('component.speech_bubble.bg'), border: v('component.speech_bubble.border'), borderWidth: oi, radius: n('component.panel.radius'), padH: space, padV: space }),
    // F114 new variants: HUD chip (tier 2 button), section tag, value chip, scrims, title bars, tab column, tabs, slider.
    styleBox('hud_chip', { ...btn, bg: v('component.button.secondary.bg'), border: ink, borderWidth: oi }),
    styleBox('hud_chip_hover', { ...btn, bg: v('component.button.secondary.bg_pressed'), border: ink, borderWidth: oi }),
    styleBox('section_tag', { bg: v('component.section_tag.bg'), border: v('component.section_tag.border'), borderWidth: oi, radius: tileR, padH: space, padV: space4 }),
    styleBox('value_chip', { bg: v('component.settings.value_bg'), border: ink, borderWidth: oi, radius: btnRadius, padH: halfSpace, padV: Math.round(space4 / 2) }),
    styleBox('scrim', { bg: v('color.overlay.scrim') }),
    styleBox('scrim_nested', { bg: v('color.overlay.scrim_nested') }),
    styleBox('title_bar', { ...titleBar, bg: v('component.settings.title_bg') }),
    styleBox('confirm_title_bar', { ...titleBar, bg: v('component.settings.confirm_title_bg') }),
    styleBox('tab_column', { bg: v('component.settings.tab_column_bg'), border: ink, sides: { right: o }, radius: btnRadius, padH: halfSpace, padV: halfSpace }),
    styleBox('tab_idle', { ...tb.rest, border: undefined, borderWidth: 0, borderBottom: undefined }),
    styleBox('tab_hover', { ...tb.rest, border: undefined, borderWidth: 0, borderBottom: undefined, bg: v('component.settings.tab_hover_bg') }),
    styleBox('tab_idle_pressed', { ...tb.pressed, border: undefined, borderWidth: 0, borderBottom: undefined, expandTop: undefined, bg: v('component.settings.tab_hover_bg') }),
    styleBox('tab_selected', { ...tb.rest, bg: v('component.settings.tab_selected_bg') }),
    styleBox('tab_selected_pressed', { ...tb.pressed, bg: v('component.settings.tab_selected_bg') }),
    styleBox('slider', { ...groove, bg: v('component.slider.track') }),
    styleBox('slider_fill', { ...groove, bg: v('component.slider.fill') }),
  ];
  const c = (p) => godotColor(v(p));
  // Icons keep their own colours (white modulate) unless a variant tints a white icon (chosen chip tick, tabs).
  const white = godotColor(v('palette.white'));
  const iconColors = (type, idle, chosen = idle) => [
    [`${type}/colors/icon_disabled_color`, idle],
    [`${type}/colors/icon_focus_color`, idle],
    [`${type}/colors/icon_hover_color`, idle],
    [`${type}/colors/icon_hover_pressed_color`, chosen],
    [`${type}/colors/icon_normal_color`, idle],
    [`${type}/colors/icon_pressed_color`, chosen],
  ];
  const fontColors = (type, color, pressed = color) => [
    [`${type}/colors/font_color`, color],
    [`${type}/colors/font_disabled_color`, color],
    [`${type}/colors/font_focus_color`, color],
    [`${type}/colors/font_hover_color`, color],
    [`${type}/colors/font_hover_pressed_color`, pressed],
    [`${type}/colors/font_pressed_color`, pressed],
  ];
  const props = [
    ['default_font', 'ExtResource("1_body")'],
    ['default_font_size', String(font)],
    ['Button/colors/font_color', c('component.button.primary.text')],
    ['Button/colors/font_disabled_color', c('color.text.secondary')],
    ['Button/colors/font_focus_color', c('component.button.primary.text')],
    ['Button/colors/font_hover_color', c('component.button.primary.text')],
    ['Button/colors/font_pressed_color', c('component.button.primary.text')],
    ['Button/constants/h_separation', String(halfSpace)],
    ['Button/constants/icon_max_width', String(icon)],
    ['Button/font_sizes/font_size', String(font)],
    ['Button/fonts/font', 'ExtResource("2_body_bold")'],
    ['Button/styles/disabled', sub('button_disabled')],
    ['Button/styles/focus', sub('focus_ring')],
    ['Button/styles/hover', sub('button_hover')],
    ['Button/styles/normal', sub('button_normal')],
    ['Button/styles/pressed', sub('button_pressed')],
    ['CheckBox/colors/font_color', c('color.text.primary')],
    ['CheckBox/colors/font_focus_color', c('color.text.primary')],
    ['CheckBox/colors/font_hover_color', c('color.text.primary')],
    ['CheckBox/colors/font_hover_pressed_color', c('color.text.primary')],
    ['CheckBox/colors/font_pressed_color', c('color.text.primary')],
    ['CheckBox/constants/h_separation', String(halfSpace)],
    ['CheckBox/icons/checked', 'ExtResource("4_checkbox_on")'],
    ['CheckBox/icons/unchecked', 'ExtResource("5_checkbox_off")'],
    ['CheckBox/styles/focus', sub('focus_ring')],
    ['CheckBox/styles/hover', sub('checkbox_hover')],
    ['CheckBox/styles/hover_pressed', sub('checkbox_hover')],
    ['CheckBox/styles/normal', sub('checkbox')],
    ['CheckBox/styles/pressed', sub('checkbox')],
    ['ChoiceChip/base_type', '&"Button"'],
    ['ChoiceChip/colors/font_color', c('component.choice_chip.text')],
    ['ChoiceChip/colors/font_focus_color', c('component.choice_chip.text')],
    ['ChoiceChip/colors/font_hover_color', c('component.choice_chip.text')],
    ['ChoiceChip/colors/font_hover_pressed_color', c('component.choice_chip.selected_text')],
    ['ChoiceChip/colors/font_pressed_color', c('component.choice_chip.selected_text')],
    ...iconColors('ChoiceChip', white, c('component.choice_chip.selected_text')),
    ['ChoiceChip/constants/h_separation', String(halfSpace)],
    ['ChoiceChip/constants/icon_max_width', String(icon)],
    ['ChoiceChip/styles/disabled', sub('chip_normal')],
    ['ChoiceChip/styles/focus', sub('focus_ring')],
    ['ChoiceChip/styles/hover', sub('chip_hover')],
    ['ChoiceChip/styles/hover_pressed', sub('chip_selected')],
    ['ChoiceChip/styles/normal', sub('chip_normal')],
    ['ChoiceChip/styles/pressed', sub('chip_selected')],
    ['ExamClockWarningLabel/base_type', '&"Label"'],
    ['ExamClockWarningLabel/colors/font_color', c('component.exam_clock.warning_text')],
    ['FeedbackCorrectPanel/base_type', '&"PanelContainer"'],
    ['FeedbackCorrectPanel/styles/panel', sub('feedback_correct')],
    ['FeedbackIncorrectPanel/base_type', '&"PanelContainer"'],
    ['FeedbackIncorrectPanel/styles/panel', sub('feedback_incorrect')],
    ['FeedbackLabel/base_type', '&"Label"'],
    ['FeedbackLabel/colors/font_color', c('component.feedback.text')],
    ['FeedbackLabel/fonts/font', 'ExtResource("3_heading")'],
    ...[1, 2, 3, 4, 5, 6].flatMap((i) => [
      [`AnswerTile${i}/base_type`, '&"Button"'],
      [`AnswerTile${i}/colors/font_color`, c('component.answer_tile.text')],
      [`AnswerTile${i}/colors/font_disabled_color`, c('component.answer_tile.text')],
      [`AnswerTile${i}/colors/font_focus_color`, c('component.answer_tile.text')],
      [`AnswerTile${i}/colors/font_hover_color`, c('component.answer_tile.text')],
      [`AnswerTile${i}/colors/font_hover_pressed_color`, c('component.answer_tile.text_selected')],
      [`AnswerTile${i}/colors/font_pressed_color`, c('component.answer_tile.text_selected')],
      [`AnswerTile${i}/constants/h_separation`, String(halfSpace)],
      [`AnswerTile${i}/constants/icon_max_width`, String(icon)],
      [`AnswerTile${i}/font_sizes/font_size`, String(stem)],
      [`AnswerTile${i}/fonts/font`, 'ExtResource("2_body_bold")'],
      [`AnswerTile${i}/styles/disabled`, sub(`tile${i}_normal`)],
      [`AnswerTile${i}/styles/focus`, sub('focus_ring')],
      [`AnswerTile${i}/styles/hover`, sub(`tile${i}_hover`)],
      [`AnswerTile${i}/styles/hover_pressed`, sub(`tile${i}_pressed`)],
      [`AnswerTile${i}/styles/normal`, sub(`tile${i}_normal`)],
      [`AnswerTile${i}/styles/pressed`, sub(`tile${i}_pressed`)],
    ]),
    // F098: graded answer tiles and their tags (icon badge + words, never colour alone).
    ...[
      [`AnswerTileCorrect/base_type`, '&"Button"'],
      [`AnswerTileCorrect/colors/font_color`, c('component.answer_tile.text')],
      [`AnswerTileCorrect/colors/font_disabled_color`, c('component.answer_tile.text')],
      [`AnswerTileCorrect/colors/font_focus_color`, c('component.answer_tile.text')],
      [`AnswerTileCorrect/colors/font_hover_color`, c('component.answer_tile.text')],
      [`AnswerTileCorrect/colors/font_hover_pressed_color`, c('component.answer_tile.text')],
      [`AnswerTileCorrect/colors/font_pressed_color`, c('component.answer_tile.text')],
      [`AnswerTileCorrect/constants/h_separation`, String(halfSpace)],
      [`AnswerTileCorrect/constants/icon_max_width`, String(icon)],
      [`AnswerTileCorrect/font_sizes/font_size`, String(stem)],
      [`AnswerTileCorrect/fonts/font`, 'ExtResource("2_body_bold")'],
      [`AnswerTileCorrect/styles/disabled`, sub('tile_correct')],
      [`AnswerTileCorrect/styles/hover`, sub('tile_correct')],
      [`AnswerTileCorrect/styles/hover_pressed`, sub('tile_correct')],
      [`AnswerTileCorrect/styles/normal`, sub('tile_correct')],
      [`AnswerTileCorrect/styles/pressed`, sub('tile_correct')],
      [`AnswerTileCorrect/styles/focus`, sub('focus_ring')],
      [`AnswerTileTry/base_type`, '&"Button"'],
      [`AnswerTileTry/colors/font_color`, c('component.answer_tile.text')],
      [`AnswerTileTry/colors/font_disabled_color`, c('component.answer_tile.text')],
      [`AnswerTileTry/colors/font_focus_color`, c('component.answer_tile.text')],
      [`AnswerTileTry/colors/font_hover_color`, c('component.answer_tile.text')],
      [`AnswerTileTry/colors/font_hover_pressed_color`, c('component.answer_tile.text')],
      [`AnswerTileTry/colors/font_pressed_color`, c('component.answer_tile.text')],
      [`AnswerTileTry/constants/h_separation`, String(halfSpace)],
      [`AnswerTileTry/constants/icon_max_width`, String(icon)],
      [`AnswerTileTry/font_sizes/font_size`, String(stem)],
      [`AnswerTileTry/fonts/font`, 'ExtResource("2_body_bold")'],
      [`AnswerTileTry/styles/disabled`, sub('tile_try')],
      [`AnswerTileTry/styles/hover`, sub('tile_try')],
      [`AnswerTileTry/styles/hover_pressed`, sub('tile_try')],
      [`AnswerTileTry/styles/normal`, sub('tile_try')],
      [`AnswerTileTry/styles/pressed`, sub('tile_try')],
      [`AnswerTileTry/styles/focus`, sub('focus_ring')],
      [`AnswerTileRest/base_type`, '&"Button"'],
      [`AnswerTileRest/colors/font_color`, c('component.answer_tile.rest_text')],
      [`AnswerTileRest/colors/font_disabled_color`, c('component.answer_tile.rest_text')],
      [`AnswerTileRest/colors/font_focus_color`, c('component.answer_tile.rest_text')],
      [`AnswerTileRest/colors/font_hover_color`, c('component.answer_tile.rest_text')],
      [`AnswerTileRest/colors/font_hover_pressed_color`, c('component.answer_tile.rest_text')],
      [`AnswerTileRest/colors/font_pressed_color`, c('component.answer_tile.rest_text')],
      [`AnswerTileRest/constants/h_separation`, String(halfSpace)],
      [`AnswerTileRest/constants/icon_max_width`, String(icon)],
      [`AnswerTileRest/font_sizes/font_size`, String(stem)],
      [`AnswerTileRest/fonts/font`, 'ExtResource("2_body_bold")'],
      [`AnswerTileRest/styles/disabled`, sub('tile_rest')],
      [`AnswerTileRest/styles/hover`, sub('tile_rest')],
      [`AnswerTileRest/styles/hover_pressed`, sub('tile_rest')],
      [`AnswerTileRest/styles/normal`, sub('tile_rest')],
      [`AnswerTileRest/styles/pressed`, sub('tile_rest')],
      [`AnswerTileRest/styles/focus`, sub('focus_ring')],
      [`TileTagCorrect/base_type`, '&"Label"'],
      [`TileTagCorrect/colors/font_color`, c('component.answer_tile.tag_text')],
      [`TileTagCorrect/font_sizes/font_size`, String(small)],
      [`TileTagCorrect/fonts/font`, 'ExtResource("2_body_bold")'],
      [`TileTagCorrect/styles/normal`, sub('tag_correct')],
      [`TileTagTry/base_type`, '&"Label"'],
      [`TileTagTry/colors/font_color`, c('component.answer_tile.tag_text')],
      [`TileTagTry/font_sizes/font_size`, String(small)],
      [`TileTagTry/fonts/font`, 'ExtResource("2_body_bold")'],
      [`TileTagTry/styles/normal`, sub('tag_try')],
      [`TileTagPending/base_type`, '&"Label"'],
      [`TileTagPending/colors/font_color`, c('component.answer_tile.tag_pending_text')],
      [`TileTagPending/font_sizes/font_size`, String(small)],
      [`TileTagPending/fonts/font`, 'ExtResource("2_body_bold")'],
      [`TileTagPending/styles/normal`, sub('tag_pending')],
    ],
    ['ActivityBoard/base_type', '&"PanelContainer"'],
    ['AnswerBox/base_type', '&"VBoxContainer"'],
    ['AnswerBox/constants/separation', String(space)],
    ['AnswerFlow/base_type', '&"HFlowContainer"'],
    ['AnswerFlow/constants/h_separation', String(space)],
    ['AnswerFlow/constants/v_separation', String(space)],
    ['AnswerGrid/base_type', '&"GridContainer"'],
    ['AnswerGrid/constants/h_separation', String(space)],
    ['AnswerGrid/constants/v_separation', String(space)],
    ['ActivityBoard/styles/panel', sub('activity_board')],
    ['DropZone/base_type', '&"Button"'],
    ['DropZone/colors/font_color', c('component.question_card.text')],
    ['DropZone/colors/font_disabled_color', c('component.question_card.text')],
    ['DropZone/colors/font_focus_color', c('component.question_card.text')],
    ['DropZone/colors/font_hover_color', c('component.question_card.text')],
    ['DropZone/colors/font_hover_pressed_color', c('component.question_card.text')],
    ['DropZone/colors/font_pressed_color', c('component.question_card.text')],
    ['DropZone/constants/h_separation', String(halfSpace)],
    ['DropZone/constants/icon_max_width', String(icon)],
    ['DropZone/font_sizes/font_size', String(stem)],
    ['DropZone/fonts/font', 'ExtResource("2_body_bold")'],
    ['DropZone/styles/disabled', sub('drop_zone')],
    ['DropZone/styles/focus', sub('focus_ring')],
    ['DropZone/styles/hover', sub('drop_zone_hover')],
    ['DropZone/styles/hover_pressed', sub('drop_zone_pressed')],
    ['DropZone/styles/normal', sub('drop_zone')],
    ['DropZone/styles/pressed', sub('drop_zone_pressed')],
    ['InstructionLabel/base_type', '&"Label"'],
    ['InstructionLabel/colors/font_color', c('component.text_button.text')],
    ['InstructionLabel/fonts/font', 'ExtResource("2_body_bold")'],
    ['PictureTile/base_type', '&"Button"'],
    ['PictureTile/colors/font_color', c('component.button.secondary.text')],
    ['PictureTile/colors/font_focus_color', c('component.button.secondary.text')],
    ['PictureTile/colors/font_hover_color', c('component.button.secondary.text')],
    ['PictureTile/colors/font_hover_pressed_color', c('component.choice_chip.selected_text')],
    ['PictureTile/colors/font_pressed_color', c('component.choice_chip.selected_text')],
    ['PictureTile/styles/disabled', sub('picture_normal')],
    ['PictureTile/styles/focus', sub('focus_ring')],
    ['PictureTile/styles/hover', sub('picture_hover')],
    ['PictureTile/styles/hover_pressed', sub('picture_selected')],
    ['PictureTile/styles/normal', sub('picture_normal')],
    ['PictureTile/styles/pressed', sub('picture_selected')],
    ['QuestionCard/base_type', '&"PanelContainer"'],
    ['QuestionCard/styles/panel', sub('question_card')],
    ['StemLabel/base_type', '&"Label"'],
    ['StemLabel/colors/font_color', c('component.question_card.text')],
    ['StemLabel/font_sizes/font_size', String(stem)],
    ['StemLabel/fonts/font', 'ExtResource("3_heading")'],
    ['FieldBox/base_type', '&"VBoxContainer"'],
    ['FieldBox/constants/separation', String(space4)],
    ['FormBox/base_type', '&"VBoxContainer"'],
    ['FormBox/constants/separation', String(formGap)],
    ['FormCard/base_type', '&"PanelContainer"'],
    ['FormCard/styles/panel', sub('form_card')],
    ['HeadingLabel/base_type', '&"Label"'],
    ['HeadingLabel/colors/font_color', c('color.text.primary')],
    ['HeadingLabel/font_sizes/font_size', String(heading)],
    ['HeadingLabel/fonts/font', 'ExtResource("3_heading")'],
    ['HintButton/base_type', '&"Button"'],
    ['HintButton/colors/font_color', c('component.hint_button.text')],
    ['HintButton/colors/font_focus_color', c('component.hint_button.text')],
    ['HintButton/colors/font_hover_color', c('component.hint_button.text')],
    ['HintButton/colors/font_pressed_color', c('component.hint_button.text')],
    ['HintButton/styles/hover', sub('hint_normal')],
    ['HintButton/styles/normal', sub('hint_normal')],
    ['HintButton/styles/pressed', sub('hint_pressed')],
    ['HudLabel/base_type', '&"Label"'],
    ['HudLabel/colors/font_color', c('component.hud.text')],
    ['HudPanel/base_type', '&"PanelContainer"'],
    ['HudPanel/styles/panel', sub('hud')],
    ['IconButton/base_type', '&"Button"'],
    ['IconButton/colors/font_color', c('component.button.secondary.text')],
    ['IconButton/colors/font_hover_color', c('component.button.secondary.text')],
    ['IconButton/colors/font_pressed_color', c('component.button.secondary.text')],
    ['IconButton/constants/icon_max_width', String(icon)],
    ['IconButton/styles/disabled', sub('icon_normal')],
    ['IconButton/styles/focus', sub('focus_ring')],
    ['IconButton/styles/hover', sub('icon_hover')],
    ['IconButton/styles/hover_pressed', sub('icon_pressed')],
    ['IconButton/styles/normal', sub('icon_normal')],
    ['IconButton/styles/pressed', sub('icon_pressed')],
    ['InfoBannerLabel/base_type', '&"Label"'],
    ['InfoBannerLabel/colors/font_color', c('component.info_banner.text')],
    ['InfoBannerPanel/base_type', '&"PanelContainer"'],
    ['InfoBannerPanel/styles/panel', sub('info_banner')],
    ['InlineErrorLabel/base_type', '&"Label"'],
    ['InlineErrorLabel/colors/font_color', c('component.inline_error.text')],
    ['InlineErrorPanel/base_type', '&"PanelContainer"'],
    ['InlineErrorPanel/styles/panel', sub('inline_error')],
    ['Label/colors/font_color', c('color.text.primary')],
    ['Label/font_sizes/font_size', String(font)],
    ['LineEdit/colors/caret_color', c('color.text.primary')],
    ['LineEdit/colors/clear_button_color', c('color.text.secondary')],
    ['LineEdit/colors/clear_button_color_pressed', c('component.icon.primary')],
    ['LineEdit/colors/font_color', c('color.text.primary')],
    ['LineEdit/colors/font_placeholder_color', c('color.text.muted')],
    ['LineEdit/colors/font_uneditable_color', c('color.text.secondary')],
    ['LineEdit/font_sizes/font_size', String(font)],
    ['LineEdit/styles/focus', sub('line_edit_focus')],
    ['LineEdit/styles/normal', sub('line_edit')],
    ['LineEdit/styles/read_only', sub('line_edit_read_only')],
    ['LogoLabel/base_type', '&"Label"'],
    ['LogoLabel/colors/font_color', c('component.logo.text')],
    ['LogoLabel/colors/font_outline_color', c('component.logo.outline')],
    ['LogoLabel/constants/outline_size', String(n('component.logo.outline_width'))],
    ['LogoLabel/font_sizes/font_size', String(n('component.logo.font_size'))],
    ['LogoLabel/fonts/font', 'ExtResource("3_heading")'],
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
    ['ScreenMargin/base_type', '&"MarginContainer"'],
    ['ScreenMargin/constants/margin_bottom', String(space)],
    ['ScreenMargin/constants/margin_left', String(space)],
    ['ScreenMargin/constants/margin_right', String(space)],
    ['ScreenMargin/constants/margin_top', String(space)],
    ['SecondaryButton/base_type', '&"Button"'],
    ['SecondaryButton/colors/font_color', c('component.button.secondary.text')],
    ['SecondaryButton/colors/font_focus_color', c('component.button.secondary.text')],
    ['SecondaryButton/colors/font_hover_color', c('component.button.secondary.text')],
    ['SecondaryButton/colors/font_pressed_color', c('component.button.secondary.text')],
    ['SecondaryButton/styles/disabled', sub('secondary_normal')],
    ['SecondaryButton/styles/hover', sub('secondary_hover')],
    ['SecondaryButton/styles/normal', sub('secondary_normal')],
    ['SecondaryButton/styles/pressed', sub('secondary_pressed')],
    ['SpeechBubble/base_type', '&"PanelContainer"'],
    ['SpeechBubble/styles/panel', sub('speech_bubble')],
    ['SpeechBubbleLabel/base_type', '&"Label"'],
    ['SpeechBubbleLabel/colors/font_color', c('component.speech_bubble.text')],
    ['TextButton/base_type', '&"Button"'],
    ['TextButton/colors/font_color', c('component.text_button.text')],
    ['TextButton/colors/font_focus_color', c('component.text_button.text')],
    ['TextButton/colors/font_hover_color', c('component.text_button.text')],
    ['TextButton/colors/font_hover_pressed_color', c('component.text_button.text')],
    ['TextButton/colors/font_pressed_color', c('component.text_button.text')],
    ['TextButton/constants/h_separation', String(halfSpace)],
    ['TextButton/constants/icon_max_width', String(icon)],
    ['TextButton/styles/focus', sub('focus_ring')],
    ['TextButton/styles/hover', sub('text_hover')],
    ['TextButton/styles/hover_pressed', sub('text_pressed')],
    ['TextButton/styles/normal', sub('text_normal')],
    ['TextButton/styles/pressed', sub('text_pressed')],
    ['ToolbarBox/base_type', '&"HBoxContainer"'],
    ['ToolbarBox/constants/separation', String(halfSpace)],
    // F099: readable caption card (stem size of the band) and the dark screen behind the video picture.
    ['CaptionCard/base_type', '&"PanelContainer"'],
    ['CaptionCard/styles/panel', sub('caption_card')],
    ['CaptionLabel/base_type', '&"Label"'],
    ['CaptionLabel/colors/font_color', c('component.caption.text')],
    ['CaptionLabel/font_sizes/font_size', String(stem)],
    ['CaptionLabel/fonts/font', 'ExtResource("2_body_bold")'],
    ['MediaStage/base_type', '&"PanelContainer"'],
    ['MediaStage/styles/panel', sub('media_stage')],
    // F114 (DESIGN_SYSTEM §5.1): danger button (leave / end only), HUD chip (tier 2 button), section tag, value chip,
    // scrims, window title bars, Settings tab column and tabs, toggle switch, slider.
    ['DangerButton/base_type', '&"Button"'],
    ...fontColors('DangerButton', c('component.button.danger.text')),
    ['DangerButton/styles/disabled', sub('danger_normal')],
    ['DangerButton/styles/focus', sub('focus_ring')],
    ['DangerButton/styles/hover', sub('danger_hover')],
    ['DangerButton/styles/hover_pressed', sub('danger_pressed')],
    ['DangerButton/styles/normal', sub('danger_normal')],
    ['DangerButton/styles/pressed', sub('danger_pressed')],
    ['HudChip/base_type', '&"Button"'],
    ...fontColors('HudChip', c('component.button.secondary.text')),
    ['HudChip/styles/disabled', sub('hud_chip')],
    ['HudChip/styles/focus', sub('focus_ring')],
    ['HudChip/styles/hover', sub('hud_chip_hover')],
    ['HudChip/styles/hover_pressed', sub('hud_chip_hover')],
    ['HudChip/styles/normal', sub('hud_chip')],
    ['HudChip/styles/pressed', sub('hud_chip_hover')],
    ['SectionTag/base_type', '&"Label"'],
    ['SectionTag/colors/font_color', c('component.section_tag.text')],
    ['SectionTag/font_sizes/font_size', String(small)],
    ['SectionTag/fonts/font', 'ExtResource("2_body_bold")'],
    ['SectionTag/styles/normal', sub('section_tag')],
    ['ValueChip/base_type', '&"Label"'],
    ['ValueChip/colors/font_color', c('component.settings.value_text')],
    ['ValueChip/fonts/font', 'ExtResource("2_body_bold")'],
    ['ValueChip/styles/normal', sub('value_chip')],
    ['Scrim/base_type', '&"Panel"'],
    ['Scrim/styles/panel', sub('scrim')],
    ['ScrimNested/base_type', '&"Panel"'],
    ['ScrimNested/styles/panel', sub('scrim_nested')],
    ['WindowTitleBar/base_type', '&"PanelContainer"'],
    ['WindowTitleBar/styles/panel', sub('title_bar')],
    ['WindowTitleLabel/base_type', '&"Label"'],
    ['WindowTitleLabel/colors/font_color', c('component.settings.title_text')],
    ['WindowTitleLabel/font_sizes/font_size', String(heading)],
    ['WindowTitleLabel/fonts/font', 'ExtResource("3_heading")'],
    ['ConfirmTitleBar/base_type', '&"PanelContainer"'],
    ['ConfirmTitleBar/styles/panel', sub('confirm_title_bar')],
    ['ConfirmTitleLabel/base_type', '&"Label"'],
    ['ConfirmTitleLabel/colors/font_color', c('component.settings.confirm_title_text')],
    ['ConfirmTitleLabel/font_sizes/font_size', String(heading)],
    ['ConfirmTitleLabel/fonts/font', 'ExtResource("3_heading")'],
    ['SettingsTabColumn/base_type', '&"PanelContainer"'],
    ['SettingsTabColumn/styles/panel', sub('tab_column')],
    ['TabButton/base_type', '&"Button"'],
    ...fontColors('TabButton', c('component.settings.tab_text')),
    ...iconColors('TabButton', c('component.settings.tab_icon')),
    ['TabButton/constants/h_separation', String(halfSpace)],
    ['TabButton/constants/icon_max_width', String(icon)],
    ['TabButton/styles/disabled', sub('tab_idle')],
    ['TabButton/styles/focus', sub('focus_ring')],
    ['TabButton/styles/hover', sub('tab_hover')],
    ['TabButton/styles/hover_pressed', sub('tab_idle_pressed')],
    ['TabButton/styles/normal', sub('tab_idle')],
    ['TabButton/styles/pressed', sub('tab_idle_pressed')],
    ['TabButtonSelected/base_type', '&"Button"'],
    ...fontColors('TabButtonSelected', c('component.settings.tab_selected_text')),
    ...iconColors('TabButtonSelected', c('component.settings.tab_selected_text')),
    ['TabButtonSelected/constants/h_separation', String(halfSpace)],
    ['TabButtonSelected/constants/icon_max_width', String(icon)],
    ['TabButtonSelected/styles/disabled', sub('tab_selected')],
    ['TabButtonSelected/styles/focus', sub('focus_ring')],
    ['TabButtonSelected/styles/hover', sub('tab_selected')],
    ['TabButtonSelected/styles/hover_pressed', sub('tab_selected_pressed')],
    ['TabButtonSelected/styles/normal', sub('tab_selected')],
    ['TabButtonSelected/styles/pressed', sub('tab_selected_pressed')],
    ['ToggleSwitch/base_type', '&"CheckButton"'],
    ...fontColors('ToggleSwitch', c('component.toggle.text')),
    ['ToggleSwitch/constants/h_separation', String(halfSpace)],
    // The switch icon keeps its own size (Button's icon_max_width would shrink it to the button icon size).
    ['ToggleSwitch/constants/icon_max_width', '0'],
    ['ToggleSwitch/icons/checked', 'ExtResource("7_toggle_on")'],
    ['ToggleSwitch/icons/checked_disabled', 'ExtResource("7_toggle_on")'],
    ['ToggleSwitch/icons/checked_mirrored', 'ExtResource("7_toggle_on")'],
    ['ToggleSwitch/icons/unchecked', 'ExtResource("8_toggle_off")'],
    ['ToggleSwitch/icons/unchecked_disabled', 'ExtResource("8_toggle_off")'],
    ['ToggleSwitch/icons/unchecked_mirrored', 'ExtResource("8_toggle_off")'],
    ['ToggleSwitch/styles/focus', sub('focus_ring')],
    ['ToggleSwitch/styles/hover', sub('checkbox_hover')],
    ['ToggleSwitch/styles/hover_pressed', sub('checkbox_hover')],
    ['ToggleSwitch/styles/normal', sub('checkbox')],
    ['ToggleSwitch/styles/pressed', sub('checkbox')],
    ['HSlider/icons/grabber', 'ExtResource("6_slider_knob")'],
    ['HSlider/icons/grabber_disabled', 'ExtResource("6_slider_knob")'],
    ['HSlider/icons/grabber_highlight', 'ExtResource("6_slider_knob")'],
    ['HSlider/styles/grabber_area', sub('slider_fill')],
    ['HSlider/styles/grabber_area_highlight', sub('slider_fill')],
    ['HSlider/styles/slider', sub('slider')],
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
    ['shader_parameter/band_softness', gdFloat(Number(v('toon.band_softness')))],
  );
  return resource('ShaderMaterial', subs, ext, props, mark(`Toon material preset "${label}"`, version));
}

/** '#RRGGBB' → ['r', 'g', 'b', 'a'] float strings of a Godot Color (flat form of PackedColorArray). */
function colorFloats(hex) {
  return godotColor(hex).slice('Color('.length, -1).split(', ');
}

/**
 * Game icon (DESIGN_SYSTEM §2: SVG sources in design/icons/game). `{token.path}` placeholders in the source are
 * replaced by the resolved colour of that token (game context), so icons follow the palette without literals.
 */
export function formatIcon(map, name, source, version) {
  const svg = source.replace(/\{([A-Za-z0-9_.-]+)\}/g, (_, p) => {
    if (!map.has(p)) throw new Error(`icon ${name}: unknown token {${p}}`);
    const value = String(map.get(p).value);
    if (!/^#[0-9A-Fa-f]{6}$/.test(value)) throw new Error(`icon ${name}: {${p}} is not an opaque #RRGGBB colour`);
    return value.toUpperCase();
  });
  return `<!-- ${GENERATED_MARK}. Game icon "${name}" (tokens@${version}), source design/icons/game/${name}.svg. Run make design-tokens. -->\n${svg.trimEnd()}\n`;
}
