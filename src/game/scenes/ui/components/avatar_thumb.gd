class_name AvatarThumb
extends Control
## Picture of a character preset on its S02 tile (F097, STUDENT_GAME_UX_CORE S02, CHARACTER_SYSTEM §5.0): a chibi bust
## (big head ≈ 1/2, anime eyes, cheeks, hair cap, outfit) drawn with vector shapes in the preset's own colours — skin
## tone and hair colour from `ItemVisual.appearance_color` (design tokens `avatar.*`), outfit from the item tint — and
## the Mây outline/eye/cheek tokens. Young students choose by picture, not by reading (F096-1). Static, no asset.
## It fills the tall top margin of its parent `PictureTile` (the label stays below) and shows a tick when the tile is
## chosen (shape, not only colour — NFR-A11Y-001).

var skin: Color = Color.WHITE
var hair: Color = Color.WHITE
var outfit: Color = Color.WHITE
## F102: a portrait of its own (party member card, HUD party strip, invite picker) sized by `custom_minimum_size` instead
## of filling the top margin of a parent `PictureTile`.
var standalone: bool = false
## F102: the tick badge of a chosen tile on a standalone portrait (a party member who is Ready, on the HUD strip).
var ready_mark: bool = false:
	set(value):
		ready_mark = value
		queue_redraw()


func _init() -> void:
	name = "Thumb"
	mouse_filter = Control.MOUSE_FILTER_IGNORE


func _ready() -> void:
	if standalone:
		return
	set_anchors_preset(Control.PRESET_FULL_RECT)
	var parent: Control = get_parent() as Control
	if parent != null:
		parent.draw.connect(queue_redraw)
		parent.resized.connect(_layout)
		parent.theme_changed.connect(_layout)
	_layout()


## The preset look (`AvatarConfigModel` of the catalog): skin tone, hair colour, outfit tint.
## `fallback` colours an outfit that has no tint of its own (white).
func set_look(config: AvatarConfigModel, fallback: Color = UiTokens.COMPONENT_ANSWER_TILE_ACCENT_1) -> void:
	if config == null:
		return
	skin = ItemVisual.appearance_color(config.skin_tone)
	hair = ItemVisual.appearance_color(config.hair_color)
	var visual: ItemVisual = ItemVisual.load_item(config.outfit_item_id)
	outfit = visual.tint if visual != null and not visual.tint.is_equal_approx(Color.WHITE) else fallback
	queue_redraw()


## Covers the top margin of the tile (where `PictureTile` leaves room for the picture).
func _layout() -> void:
	var parent: Control = get_parent() as Control
	if parent == null or standalone:
		return
	var box: StyleBox = parent.get_theme_stylebox(&"normal")
	var pad: float = box.content_margin_left if box != null else 0.0
	var top: float = box.content_margin_top if box != null else parent.size.y / 2.0
	offset_left = pad
	offset_top = pad
	offset_right = -pad
	anchor_bottom = 0.0
	offset_bottom = top
	queue_redraw()


func is_selected() -> bool:
	if standalone:
		return false
	var parent: BaseButton = get_parent() as BaseButton
	return parent != null and parent.toggle_mode and parent.button_pressed


func _draw() -> void:
	var side: float = minf(size.x, size.y)
	if side <= 0.0:
		return
	var o: Vector2 = Vector2((size.x - side) / 2.0, size.y - side)
	var line: float = maxf(side * 0.025, 1.5)
	var ink: Color = UiTokens.COMPONENT_MASCOT_OUTLINE
	# Shoulders / outfit.
	var body: PackedVector2Array = PackedVector2Array([
		o + Vector2(0.18, 1.0) * side, o + Vector2(0.24, 0.8) * side, o + Vector2(0.36, 0.72) * side,
		o + Vector2(0.64, 0.72) * side, o + Vector2(0.76, 0.8) * side, o + Vector2(0.82, 1.0) * side,
	])
	draw_colored_polygon(body, outfit)
	draw_polyline(body, ink, line, true)
	# Head (big, chibi) with the hair cap behind and the bangs on top.
	var head_c: Vector2 = o + Vector2(0.5, 0.44) * side
	var head_r: float = side * 0.3
	draw_circle(head_c, head_r + side * 0.04 + line, ink)
	draw_circle(head_c, head_r + side * 0.04, hair)
	draw_circle(head_c + Vector2(0.0, side * 0.03), head_r + line, ink)
	draw_circle(head_c + Vector2(0.0, side * 0.03), head_r, skin)
	draw_colored_polygon(_arc_points(head_c, head_r + side * 0.04, PI * 1.02, PI * 1.98, 18), hair)
	# Anime eyes with a highlight, cheeks, small smile.
	for x: float in [-0.11, 0.11]:
		var eye: Vector2 = head_c + Vector2(x, 0.07) * side
		draw_circle(eye, side * 0.045, UiTokens.COMPONENT_MASCOT_EYE)
		draw_circle(eye + Vector2(side * 0.015, -side * 0.018), side * 0.016, UiTokens.COMPONENT_MASCOT_BODY)
	for x: float in [-0.19, 0.19]:
		draw_circle(head_c + Vector2(x, 0.15) * side, side * 0.035, UiTokens.COMPONENT_MASCOT_CHEEK)
	draw_arc(head_c + Vector2(0.0, 0.15) * side, side * 0.045, PI * 0.2, PI * 0.8, 10, ink, line, true)
	if is_selected() or ready_mark:
		# A Ready badge on a small HUD portrait is drawn larger so the tick reads at phone size.
		var big: bool = ready_mark and not is_selected()
		var c: Vector2 = o + (Vector2(0.8, 0.2) if big else Vector2(0.88, 0.14)) * side
		var r: float = side * (0.2 if big else 0.13)
		# The HUD Ready badge is the chosen-chip pair (yellow disc, ink tick, F114); the tile badge keeps its F097 look.
		draw_circle(c, r + line, ink)
		draw_circle(c, r, UiTokens.COMPONENT_CHOICE_CHIP_SELECTED_BG if big else UiTokens.COMPONENT_ANSWER_TILE_TEXT_SELECTED)
		draw_polyline(PackedVector2Array([c + Vector2(-0.45, 0.0) * r, c + Vector2(-0.1, 0.35) * r,
				c + Vector2(0.5, -0.35) * r]), ink if big else UiTokens.COMPONENT_CHIP_SELECTED_BG, line * 1.6, true)


static func _arc_points(c: Vector2, r: float, from: float, to: float, steps: int) -> PackedVector2Array:
	var out: PackedVector2Array = PackedVector2Array()
	for i: int in steps + 1:
		var a: float = lerpf(from, to, float(i) / float(steps))
		out.append(c + Vector2(cos(a), sin(a)) * r)
	return out
