class_name ModeCard
extends Button
## One competition mode the server opened (F109-AC-01; STUDENT_GAME_UX_SOCIAL §8 `ModeCard`): a big picture tile
## (`PictureTile`: a large mode picture in the top margin, the mode name and "Tính hạng" / "Luyện tập" below; the short
## hint in the tooltip). The chosen card is yellow **and** carries the tag "Đã chọn" (never colour alone). UI only: the
## owner reads `mode`.

## Picture size, in touch targets of the band.
const PICTURE_SCALE: float = 1.0

var mode: String = ""
var _picture: TextureRect = null
var _tag: Label = null


func _init() -> void:
	name = "ModeCard"
	theme_type_variation = &"PictureTile"
	toggle_mode = true
	autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
	vertical_icon_alignment = VERTICAL_ALIGNMENT_TOP
	_picture = TextureRect.new()
	_picture.name = "Picture"
	_picture.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
	_picture.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
	_picture.mouse_filter = Control.MOUSE_FILTER_IGNORE
	_picture.set_anchors_preset(Control.PRESET_CENTER_TOP)
	add_child(_picture)
	toggled.connect(func(_on: bool) -> void: _render_tag())


func bind(m: ArenaModeModel) -> void:
	mode = m.mode
	name = "ModeCard_%s" % m.mode
	_picture.texture = UiBuild.icon(String(CompetitionText.MODE_ICONS.get(m.mode, "star")))
	var t: float = UiBuild.target()
	var side: float = t * PICTURE_SCALE
	_picture.offset_left = -side / 2.0
	_picture.offset_right = side / 2.0
	# Under the "Đã chọn" tag, inside the picture margin of `PictureTile` (1.5 touch targets).
	_picture.offset_top = t * 0.5
	_picture.offset_bottom = t * 0.5 + side
	text = "%s\n%s" % [tr(CompetitionText.mode_key(m.mode)), tr("ui.arena.mode.ranked" if m.ranked else "ui.arena.mode.unranked")]
	tooltip_text = tr(String(CompetitionText.MODE_HINT_KEYS.get(m.mode, "ui.arena.mode.other")))
	custom_minimum_size = Vector2(t * 4.2, t * 3.0)
	_render_tag()


## Shows / hides the "Đã chọn" tag with the pressed state (also called after `set_pressed_no_signal`).
func refresh_tag() -> void:
	_render_tag()


func _render_tag() -> void:
	if _tag == null:
		_tag = Label.new()
		_tag.name = "ChosenTag"
		_tag.theme_type_variation = &"TileTagCorrect"
		# The card's own text is data (no lookup); the tag is UI words.
		_tag.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_ALWAYS
		_tag.text = "ui.arena.mode.chosen"
		_tag.mouse_filter = Control.MOUSE_FILTER_IGNORE
		_tag.set_anchors_preset(Control.PRESET_TOP_RIGHT)
		_tag.grow_horizontal = Control.GROW_DIRECTION_BEGIN
		var inset: float = float(UiTokens.COMPONENT_ANSWER_TILE_RADIUS)
		_tag.offset_left = -inset
		_tag.offset_right = -inset
		add_child(_tag)
	_tag.visible = button_pressed


func is_chosen() -> bool:
	return button_pressed
