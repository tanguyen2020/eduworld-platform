class_name QuestionRendererBase
extends Control
## Contract of the 11 question scenes (GODOT_ARCHITECTURE §6.2, STUDENT_GAME_UX_LEARNING §4.3, F063 T005). A scene
## draws one `QuestionRenderModel` and feeds the student's input to its AnswerModel; `commit()` emits
## `response_prepared(position, response)` (spec `response_ready`, past tense per GODOT_ARCHITECTURE §7) with exactly
## the `QuestionResponse` shape of the type — no score, no `is_correct`, no answer key (F063-AC-03, ADR-0016).

signal answer_changed(complete: bool)
signal response_prepared(position: int, response: Dictionary)
## F097: the student pressed "Đọc to" on the question card (the host plays the recorded stem).
signal read_aloud_requested()

const STEM_SCENE: String = "res://scenes/learning/RichContentView.tscn"
const READ_ALOUD_KEY: String = "ui.activity_player.read_aloud"
const ICON_SOUND: Texture2D = preload("res://resources/ui/icons/sound_on.svg")

var item: QuestionRenderModel = null
var answer: AnswerModel = null
var context: RendererContext = null
var locked: bool = false
## Set by the renderer that relays `read_aloud_requested` to the host (F097); without it no button is shown.
var read_aloud_enabled: bool = false
## The question card (stem + read-aloud) built by `_add_stem`.
var card: PanelContainer = null
var read_aloud: Button = null


func setup(question: QuestionRenderModel, ctx: RendererContext) -> void:
	item = question
	context = ctx
	answer = _make_answer()
	answer.setup(question.render_payload)
	answer.changed.connect(_on_answer_changed)
	_build()
	_refresh()


## Restores a saved answer (resume).
func restore(response: Dictionary) -> void:
	if answer != null and not response.is_empty():
		answer.restore(response)


func is_complete() -> bool:
	return answer != null and answer.is_complete()


func current_response() -> Dictionary:
	return answer.to_response() if answer != null else {}


## Emits `response_prepared` when the answer is complete and input is not locked.
func commit() -> bool:
	if locked or not is_complete():
		return false
	response_prepared.emit(item.position, answer.to_response())
	return true


func set_locked(value: bool) -> void:
	locked = value
	_refresh()


func select_index(_index: int) -> void:
	pass


## The AnswerModel of this type (overridden by each scene).
func _make_answer() -> AnswerModel:
	return AnswerModel.new()


## Builds the controls from the render payload (overridden).
func _build() -> void:
	pass


## Syncs the controls with the answer state (overridden).
func _refresh() -> void:
	pass


func _on_answer_changed() -> void:
	_refresh()
	answer_changed.emit(is_complete())


## Adds the question card to `parent` (F097, STUDENT_GAME_UX_LEARNING §4.1): the stem (rich content in the attempt
## language, `StemLabel` at the band's stem size) and, when the item has a recorded stem, a "Đọc to" button. The
## stem is drawn once: the host's voice caption does not repeat it (F096-3). Returns the stem view.
func _add_stem(parent: Control) -> Control:
	card = PanelContainer.new()
	card.name = "QuestionCard"
	card.theme_type_variation = &"QuestionCard"
	parent.add_child(card)
	var row: HBoxContainer = HBoxContainer.new()
	card.add_child(row)
	var view: Control = (load(STEM_SCENE) as PackedScene).instantiate() as Control
	view.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	view.set("label_variation", &"StemLabel")
	row.add_child(view)
	view.call("show_content", item.stem(), context, item)
	if read_aloud_enabled and not item.voice_url().is_empty():
		read_aloud = Button.new()
		read_aloud.name = "ReadAloud"
		read_aloud.theme_type_variation = &"SecondaryButton"
		read_aloud.icon = ICON_SOUND
		read_aloud.text = READ_ALOUD_KEY
		read_aloud.tooltip_text = READ_ALOUD_KEY
		read_aloud.size_flags_vertical = Control.SIZE_SHRINK_BEGIN
		read_aloud.pressed.connect(func() -> void: read_aloud_requested.emit())
		row.add_child(read_aloud)
	return view


## Styles `b` as answer tile `index` (F097): a big rounded tile in one of six colour sets (Theme `AnswerTile1…6`,
## sized for the grade band) with its number badge (keyboard key) on the left.
func _style_tile(b: Button, index: int, badge: bool = true) -> void:
	b.theme_type_variation = AnswerBadge.tile_type(index)
	b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
	b.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	b.alignment = HORIZONTAL_ALIGNMENT_LEFT
	if badge:
		b.add_child(AnswerBadge.new(index + 1, index))


## Minimum answer tile height of the grade band (F097, `density.<band>.answer.min_height`).
func _tile_height() -> float:
	var band: String = context.grade_band if context != null and not context.grade_band.is_empty() \
			else String(AppState.grade_band)
	if not UiTokens.DENSITY_ANSWER_MIN_HEIGHT.has(band):
		band = ThemeService.default_band()
	return float(UiTokens.DENSITY_ANSWER_MIN_HEIGHT[band])


## Locks a tile without greying it out (F097): the chosen answer keeps its selected look after grading, but the
## tile no longer takes clicks, taps or focus.
static func _lock_tile(b: BaseButton, value: bool) -> void:
	b.disabled = false
	b.mouse_filter = Control.MOUSE_FILTER_IGNORE if value else Control.MOUSE_FILTER_STOP
	b.focus_mode = Control.FOCUS_NONE if value else Control.FOCUS_ALL


## Plain text of a rich content (`{blocks}`) — formulas read as their `speech_text`.
func _plain(rich: Variant) -> String:
	return RichText.plain_text(rich, context)


## A choice card for an option content (text in the attempt language; its first formula, if any, as the icon —
## the speech text stands in until the SVG is ready). The tile's badge carries its keyboard number (`1–6`, F097).
func _make_choice(content: Variant, toggle: bool, index: int = -1) -> Button:
	var b: Button = Button.new()
	b.toggle_mode = toggle
	_style_tile(b, maxi(index, 0), index >= 0)
	var text: String = RichText.text_without_math(content, context)
	var formulas: Array[MathAssetRefModel] = RichText.math_of(content)
	if text.is_empty() and not formulas.is_empty():
		text = formulas[0].fallback_text()
	b.text = text
	if not formulas.is_empty() and context != null and context.math != null:
		_load_icon(b, formulas[0])
	return b


func _load_icon(button: Button, formula: MathAssetRefModel) -> void:
	var tex: Texture2D = await context.math.texture(formula,
			MathSvgCache.scale_for(MathInline.font_px(button), MathInline.screen_scale()), MathInline.text_color(button))
	if is_instance_valid(button) and tex != null:
		button.icon = tex
