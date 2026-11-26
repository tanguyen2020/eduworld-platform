class_name ItemThumb
extends Control
## 2D picture of an item on a grid cell, a reward or a collection page (F069, STUDENT_GAME_UX_SOCIAL §3–§4): a
## rounded tile with a chibi-style glyph of the item kind (shirt, hair, hat, smile, paw, ribbon, medal, frame, card,
## sparkle) in the item's own art colour (`ItemVisual.display_tint`, design tokens) or its rarity colour, with the ink
## outline of ADR-0103. Vector shapes, no texture per item — cheap for a grid of 500 cells (NFR-COMP-001).
## `greyed` draws the SAME picture through res://resources/art/shaders/ui_greyscale.gdshader (missing collection
## item, F069-AC-23); the screen always adds the words "Chưa có". Display only.

const GREY_SHADER: Shader = preload("res://resources/art/shaders/ui_greyscale.gdshader")
const KINDS: PackedStringArray = [
	"outfit", "hair", "accessory", "emote", "pet", "title", "badge", "frame", "decoration", "knowledge_card",
	"consumable_cosmetic",
]

var kind: String = ""
var item_id: String = ""
var rarity: String = "common"
var greyed: bool = false
static var _grey_material: ShaderMaterial = null


func _init() -> void:
	mouse_filter = Control.MOUSE_FILTER_IGNORE


func set_item(item_kind: String, code: String, item_rarity: String, grey: bool = false) -> void:
	kind = item_kind
	item_id = code
	rarity = item_rarity
	greyed = grey
	material = grey_material() if grey else null
	queue_redraw()


static func grey_material() -> ShaderMaterial:
	if _grey_material == null:
		_grey_material = ShaderMaterial.new()
		_grey_material.shader = GREY_SHADER
	return _grey_material


## Art colour of the item: its visual's palette tint when this client has one, else one of the six answer-tile accents
## picked by the code (stable, colourful — a common item never looks like a greyed missing one); the rarity stays on the
## tile border.
func fill_color() -> Color:
	var visual: ItemVisual = ItemVisual.load_item(item_id) if ItemVisual.is_valid_code(item_id) else null
	if visual != null and visual.display_tint() != Color.WHITE:
		return visual.display_tint()
	if item_id.is_empty():
		return RarityMark.color_of(rarity)
	return AnswerBadge.ACCENTS[absi(item_id.hash()) % AnswerBadge.ACCENTS.size()]


func _draw() -> void:
	var s: float = minf(size.x, size.y)
	if s <= 0.0:
		return
	var o: Vector2 = (size - Vector2(s, s)) * 0.5
	var ink: Color = UiTokens.COMPONENT_MASCOT_OUTLINE
	var w: float = maxf(1.5, s / 28.0)
	var tile: Rect2 = Rect2(o + Vector2(s, s) * 0.04, Vector2(s, s) * 0.92)
	draw_rect(tile, UiTokens.COMPONENT_QUESTION_CARD_BG, true)
	draw_rect(tile, RarityMark.color_of(rarity), false, w)
	var c: Vector2 = o + Vector2(s, s) * 0.5
	var f: Color = fill_color()
	var light: Color = UiTokens.COMPONENT_ICON_ON_ACCENT
	match kind:
		"outfit":
			_poly([c + Vector2(-0.3, -0.25) * s, c + Vector2(-0.1, -0.3) * s, c + Vector2(0, -0.22) * s,
				c + Vector2(0.1, -0.3) * s, c + Vector2(0.3, -0.25) * s, c + Vector2(0.38, -0.05) * s,
				c + Vector2(0.24, 0.0) * s, c + Vector2(0.24, 0.3) * s, c + Vector2(-0.24, 0.3) * s,
				c + Vector2(-0.24, 0.0) * s, c + Vector2(-0.38, -0.05) * s], f, ink, w)
		"hair":
			_poly(_arc_points(c + Vector2(0, 0.05) * s, s * 0.3, PI, TAU) + PackedVector2Array([
				c + Vector2(0.3, 0.2) * s, c + Vector2(0.15, 0.05) * s, c + Vector2(0, 0.12) * s,
				c + Vector2(-0.15, 0.05) * s, c + Vector2(-0.3, 0.2) * s]), f, ink, w)
		"emote":
			draw_circle(c, s * 0.3, f)
			draw_arc(c, s * 0.3, 0.0, TAU, 32, ink, w, true)
			draw_circle(c + Vector2(-0.1, -0.06) * s, s * 0.035, ink)
			draw_circle(c + Vector2(0.1, -0.06) * s, s * 0.035, ink)
			draw_arc(c + Vector2(0, 0.02) * s, s * 0.14, 0.2, PI - 0.2, 12, ink, w, true)
		"pet":
			draw_circle(c + Vector2(0, 0.06) * s, s * 0.24, f)
			draw_arc(c + Vector2(0, 0.06) * s, s * 0.24, 0.0, TAU, 32, ink, w, true)
			for side: float in [-1.0, 1.0]:
				_poly([c + Vector2(0.22 * side, -0.06) * s, c + Vector2(0.24 * side, -0.32) * s,
					c + Vector2(0.06 * side, -0.16) * s], f, ink, w)
				draw_circle(c + Vector2(0.09 * side, 0.02) * s, s * 0.03, ink)
		"title":
			_poly([c + Vector2(-0.38, -0.12) * s, c + Vector2(0.38, -0.12) * s, c + Vector2(0.3, 0.0) * s,
				c + Vector2(0.38, 0.12) * s, c + Vector2(-0.38, 0.12) * s, c + Vector2(-0.3, 0.0) * s], f, ink, w)
		"badge":
			_poly([c + Vector2(-0.16, -0.36) * s, c + Vector2(-0.04, -0.36) * s, c + Vector2(0.04, -0.1) * s,
				c + Vector2(-0.08, -0.1) * s], f, ink, w)
			_poly([c + Vector2(0.16, -0.36) * s, c + Vector2(0.04, -0.36) * s, c + Vector2(-0.04, -0.1) * s,
				c + Vector2(0.08, -0.1) * s], f, ink, w)
			draw_circle(c + Vector2(0, 0.1) * s, s * 0.22, f)
			draw_arc(c + Vector2(0, 0.1) * s, s * 0.22, 0.0, TAU, 32, ink, w, true)
			_poly(_star_points(c + Vector2(0, 0.1) * s, s * 0.12, s * 0.05), light, ink, maxf(1.0, w * 0.5))
		"frame":
			draw_rect(Rect2(c - Vector2(0.3, 0.3) * s, Vector2(0.6, 0.6) * s), f, true)
			draw_rect(Rect2(c - Vector2(0.3, 0.3) * s, Vector2(0.6, 0.6) * s), ink, false, w)
			draw_rect(Rect2(c - Vector2(0.17, 0.17) * s, Vector2(0.34, 0.34) * s), UiTokens.COMPONENT_QUESTION_CARD_BG, true)
			draw_rect(Rect2(c - Vector2(0.17, 0.17) * s, Vector2(0.34, 0.34) * s), ink, false, w)
		"knowledge_card":
			draw_rect(Rect2(c - Vector2(0.22, 0.32) * s, Vector2(0.44, 0.64) * s), f, true)
			draw_rect(Rect2(c - Vector2(0.22, 0.32) * s, Vector2(0.44, 0.64) * s), ink, false, w)
			_poly(_star_points(c, s * 0.14, s * 0.06), light, ink, maxf(1.0, w * 0.5))
		"decoration", "consumable_cosmetic":
			_poly(_star_points(c, s * 0.34, s * 0.12), f, ink, w)
		_:
			# Accessory (hat) and any newer kind: a round cap with a brim.
			_poly(_arc_points(c + Vector2(0, 0.08) * s, s * 0.24, PI, TAU), f, ink, w)
			draw_rect(Rect2(c + Vector2(-0.34, 0.06) * s, Vector2(0.68, 0.1) * s), f, true)
			draw_rect(Rect2(c + Vector2(-0.34, 0.06) * s, Vector2(0.68, 0.1) * s), ink, false, w)
			_poly(_star_points(c + Vector2(0, -0.04) * s, s * 0.08, s * 0.035), light, ink, maxf(1.0, w * 0.5))


func _arc_points(c: Vector2, r: float, from: float, to: float, steps: int = 16) -> PackedVector2Array:
	var out: PackedVector2Array = []
	for i: int in steps + 1:
		var a: float = lerpf(from, to, float(i) / float(steps))
		out.append(c + Vector2(cos(a), sin(a)) * r)
	return out


func _star_points(c: Vector2, outer: float, inner: float) -> PackedVector2Array:
	var out: PackedVector2Array = []
	for i: int in 10:
		var r: float = outer if i % 2 == 0 else inner
		var a: float = -PI / 2.0 + float(i) * PI / 5.0
		out.append(c + Vector2(cos(a), sin(a)) * r)
	return out


func _poly(points: PackedVector2Array, fill: Color, ink: Color, width: float) -> void:
	if points.size() < 3:
		return
	draw_colored_polygon(points, fill)
	var closed: PackedVector2Array = points.duplicate()
	closed.append(points[0])
	draw_polyline(closed, ink, width, true)
