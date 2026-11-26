class_name RarityMark
extends Control
## Rarity of an item (F069-AC-26, STUDENT_GAME_UX_SOCIAL §3, DESIGN_SYSTEM §5.1): the `rarity.*` colour of
## design/shared-tokens AND a shape that differs per rarity — common = round dot, uncommon = diamond, rare = star,
## epic = double star, legendary = crown — so rarity never relies on colour alone (NFR-A11Y-001). Grades 4–9 also get
## the word (`show_word`); every grade gets it as the tooltip for screen readers. Display only.

const RARITIES: PackedStringArray = ["common", "uncommon", "rare", "epic", "legendary"]
## Translation keys of the contract rarities, listed literally for tools/extract_pot.gd.
const KEYS: PackedStringArray = [
	"ui.inventory.rarity.common", "ui.inventory.rarity.uncommon", "ui.inventory.rarity.rare",
	"ui.inventory.rarity.epic", "ui.inventory.rarity.legendary",
]
const STAR_POINTS: int = 5

var rarity: String = "common"
var show_word: bool = false
var _word: Label = null


func _init(value: String = "common", word: bool = false) -> void:
	rarity = value if value in RARITIES else "common"
	show_word = word
	mouse_filter = Control.MOUSE_FILTER_PASS


func _ready() -> void:
	_word = Label.new()
	_word.theme_type_variation = &"MutedLabel"
	_word.position = Vector2(_side() + float(UiTokens.SIZE_SPACE_4), 0.0)
	add_child(_word)
	set_rarity(rarity, show_word)


static func color_of(value: String) -> Color:
	match value:
		"uncommon":
			return UiTokens.COLOR_RARITY_UNCOMMON
		"rare":
			return UiTokens.COLOR_RARITY_RARE
		"epic":
			return UiTokens.COLOR_RARITY_EPIC
		"legendary":
			return UiTokens.COLOR_RARITY_LEGENDARY
	return UiTokens.COLOR_RARITY_COMMON


static func key_of(value: String) -> String:
	return "ui.inventory.rarity." + (value if value in RARITIES else "common")


## The shape name drawn for `value` (tests check that no two rarities share a shape).
static func shape_of(value: String) -> String:
	match value:
		"uncommon":
			return "diamond"
		"rare":
			return "star"
		"epic":
			return "double_star"
		"legendary":
			return "crown"
	return "dot"


func set_rarity(value: String, word: bool) -> void:
	rarity = value if value in RARITIES else "common"
	show_word = word
	tooltip_text = key_of(rarity)
	if _word != null:
		_word.text = key_of(rarity)
		_word.visible = show_word
		custom_minimum_size = Vector2(_side() + (float(UiTokens.SIZE_SPACE_4) + _word.get_minimum_size().x
				if show_word else 0.0), _side())
	queue_redraw()


func word_text() -> String:
	return tr(key_of(rarity)) if show_word else ""


func _side() -> float:
	return float(UiTokens.COMPONENT_ICON_SIZE)


func _draw() -> void:
	var s: float = _side()
	var c: Vector2 = Vector2(s, s) * 0.5
	var fill: Color = color_of(rarity)
	var ink: Color = UiTokens.COMPONENT_MASCOT_OUTLINE
	var outline: float = maxf(1.0, float(UiTokens.COMPONENT_MASCOT_OUTLINE_WIDTH) * 0.5)
	match shape_of(rarity):
		"dot":
			draw_circle(c, s * 0.3, fill)
			draw_arc(c, s * 0.3, 0.0, TAU, 24, ink, outline, true)
		"diamond":
			_poly([c + Vector2(0, -s * 0.42), c + Vector2(s * 0.34, 0), c + Vector2(0, s * 0.42),
				c + Vector2(-s * 0.34, 0)], fill, ink, outline)
		"star":
			_poly(_star(c, s * 0.46, s * 0.2), fill, ink, outline)
		"double_star":
			_poly(_star(c + Vector2(-s * 0.14, s * 0.06), s * 0.34, s * 0.15), fill, ink, outline)
			_poly(_star(c + Vector2(s * 0.22, -s * 0.18), s * 0.22, s * 0.1), fill, ink, outline)
		"crown":
			_poly([c + Vector2(-s * 0.42, s * 0.3), c + Vector2(-s * 0.42, -s * 0.22), c + Vector2(-s * 0.18, s * 0.02),
				c + Vector2(0, -s * 0.38), c + Vector2(s * 0.18, s * 0.02), c + Vector2(s * 0.42, -s * 0.22),
				c + Vector2(s * 0.42, s * 0.3)], fill, ink, outline)


func _star(c: Vector2, outer: float, inner: float) -> PackedVector2Array:
	var points: PackedVector2Array = []
	for i: int in STAR_POINTS * 2:
		var r: float = outer if i % 2 == 0 else inner
		var a: float = -PI / 2.0 + float(i) * PI / float(STAR_POINTS)
		points.append(c + Vector2(cos(a), sin(a)) * r)
	return points


func _poly(points: PackedVector2Array, fill: Color, ink: Color, width: float) -> void:
	draw_colored_polygon(points, fill)
	var closed: PackedVector2Array = points.duplicate()
	closed.append(points[0])
	draw_polyline(closed, ink, width, true)
