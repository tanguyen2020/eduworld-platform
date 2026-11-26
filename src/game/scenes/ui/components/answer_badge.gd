class_name AnswerBadge
extends Control
## Number badge on the left of an answer tile (F097, STUDENT_GAME_UX_LEARNING §4.1, DESIGN_SYSTEM §5.1): a disc in the
## tile's accent colour with its keyboard number (1–6). When the tile is selected the badge turns into a white disc
## with a tick in the accent colour — the choice is shown by shape and words, never by colour alone (NFR-A11Y-001).
## Drawn with `UiTokens.COMPONENT_ANSWER_TILE_*` colours; it follows the state of its parent Button. Display only.
## F098 (tap-to-answer): `verdict` overrides the look — three dots while the server grades, a tick on a green disc for
## a right answer or the right-answer tile, a soft cross on a warm orange disc for a not-yet-right answer (shape and the
## tile's tag words, never colour alone).

enum Verdict { NONE, PENDING, CORRECT, TRY, ANSWER }

const ACCENTS: Array[Color] = [
	UiTokens.COMPONENT_ANSWER_TILE_ACCENT_1, UiTokens.COMPONENT_ANSWER_TILE_ACCENT_2,
	UiTokens.COMPONENT_ANSWER_TILE_ACCENT_3, UiTokens.COMPONENT_ANSWER_TILE_ACCENT_4,
	UiTokens.COMPONENT_ANSWER_TILE_ACCENT_5, UiTokens.COMPONENT_ANSWER_TILE_ACCENT_6,
]
## Theme type of tile `i` (0-based): six colour sets, then repeat.
const TILE_TYPES: Array[StringName] = [&"AnswerTile1", &"AnswerTile2", &"AnswerTile3", &"AnswerTile4", &"AnswerTile5",
	&"AnswerTile6"]

## Number drawn in the badge (keyboard key `1–6` of the tile).
var number: int = 1
## Colour set of the tile (0-based, wraps at 6).
var palette_index: int = 0
## F098: grading state of the tile (NONE = number / tick of the selection).
var verdict: Verdict = Verdict.NONE:
	set(value):
		verdict = value
		queue_redraw()


static func tile_type(index: int) -> StringName:
	return TILE_TYPES[posmod(index, TILE_TYPES.size())]


func _init(badge_number: int = 1, index: int = 0) -> void:
	number = badge_number
	palette_index = posmod(index, ACCENTS.size())
	name = "Badge"
	mouse_filter = Control.MOUSE_FILTER_IGNORE


func _ready() -> void:
	var side: float = float(UiTokens.COMPONENT_ANSWER_TILE_BADGE_SIZE)
	custom_minimum_size = Vector2(side, side)
	anchor_top = 0.5
	anchor_bottom = 0.5
	var parent: Control = get_parent() as Control
	if parent != null:
		parent.draw.connect(queue_redraw)
		parent.resized.connect(_layout)
		parent.theme_changed.connect(_layout)
	_layout()


## Places the badge in the left content margin of the tile (margin = spacing + badge + half spacing).
func _layout() -> void:
	var side: float = float(UiTokens.COMPONENT_ANSWER_TILE_BADGE_SIZE)
	var left: float = side
	var parent: Control = get_parent() as Control
	if parent != null:
		var box: StyleBox = parent.get_theme_stylebox(&"normal")
		if box != null:
			left = box.content_margin_left
	var gap: float = maxf((left - side) / 1.5, 0.0)
	offset_left = gap
	offset_right = gap + side
	offset_top = -side / 2.0
	offset_bottom = side / 2.0


func is_selected() -> bool:
	var parent: BaseButton = get_parent() as BaseButton
	return parent != null and parent.toggle_mode and parent.button_pressed


func _draw() -> void:
	var r: float = minf(size.x, size.y) / 2.0
	if r <= 0.0:
		return
	var c: Vector2 = size / 2.0
	var accent: Color = ACCENTS[palette_index]
	if verdict != Verdict.NONE:
		_draw_verdict(c, r)
		return
	if is_selected():
		draw_circle(c, r, UiTokens.COMPONENT_ANSWER_TILE_TEXT_SELECTED)
		var w: float = maxf(r * 0.28, 2.0)
		draw_polyline(PackedVector2Array([c + Vector2(-0.45, 0.02) * r, c + Vector2(-0.12, 0.35) * r,
				c + Vector2(0.5, -0.32) * r]), accent, w, true)
		return
	draw_circle(c, r, accent)
	var font: Font = get_theme_font(&"font", &"Button")
	var font_size: int = int(r * 1.2)
	var text: String = str(number)
	var text_size: Vector2 = font.get_string_size(text, HORIZONTAL_ALIGNMENT_CENTER, -1, font_size)
	var baseline: float = c.y + (font.get_ascent(font_size) - font.get_descent(font_size)) / 2.0
	draw_string(font, Vector2(c.x - text_size.x / 2.0, baseline), text, HORIZONTAL_ALIGNMENT_LEFT, -1, font_size,
			UiTokens.COMPONENT_ANSWER_TILE_TEXT_SELECTED)


## F098: the grading look of the badge (pending dots, tick, soft cross).
func _draw_verdict(c: Vector2, r: float) -> void:
	var w: float = maxf(r * 0.28, 2.0)
	match verdict:
		Verdict.PENDING:
			draw_circle(c, r, UiTokens.COMPONENT_ANSWER_TILE_TAG_PENDING_BG)
			draw_arc(c, r - w / 2.0, 0.0, TAU, 32, UiTokens.COMPONENT_ANSWER_TILE_TAG_PENDING_BORDER, w * 0.6, true)
			for x: float in [-0.42, 0.0, 0.42]:
				draw_circle(c + Vector2(x, 0.0) * r, r * 0.14, UiTokens.COMPONENT_ANSWER_TILE_TAG_PENDING_BORDER)
		Verdict.CORRECT, Verdict.ANSWER:
			draw_circle(c, r, UiTokens.COMPONENT_ANSWER_TILE_CORRECT_BORDER)
			draw_polyline(PackedVector2Array([c + Vector2(-0.45, 0.02) * r, c + Vector2(-0.12, 0.35) * r,
					c + Vector2(0.5, -0.32) * r]), UiTokens.COMPONENT_ANSWER_TILE_TAG_TEXT, w, true)
		Verdict.TRY:
			draw_circle(c, r, UiTokens.COMPONENT_ANSWER_TILE_TRY_BORDER)
			var d: float = r * 0.36
			draw_line(c + Vector2(-d, -d), c + Vector2(d, d), UiTokens.COMPONENT_ANSWER_TILE_TAG_TEXT, w, true)
			draw_line(c + Vector2(d, -d), c + Vector2(-d, d), UiTokens.COMPONENT_ANSWER_TILE_TAG_TEXT, w, true)
