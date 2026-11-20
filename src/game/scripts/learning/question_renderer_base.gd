class_name QuestionRendererBase
extends Control
## Contract of the 11 question scenes (GODOT_ARCHITECTURE §6.2, STUDENT_GAME_UX_LEARNING §4.3, F063 T005). A scene
## draws one `QuestionRenderModel` and feeds the student's input to its AnswerModel; `commit()` emits
## `response_prepared(position, response)` (spec `response_ready`, past tense per GODOT_ARCHITECTURE §7) with exactly
## the `QuestionResponse` shape of the type — no score, no `is_correct`, no answer key (F063-AC-03, ADR-0016).

signal answer_changed(complete: bool)
signal response_prepared(position: int, response: Dictionary)

const STEM_SCENE: String = "res://scenes/learning/RichContentView.tscn"

var item: QuestionRenderModel = null
var answer: AnswerModel = null
var context: RendererContext = null
var locked: bool = false


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


## Adds the stem (rich content in the attempt language) to `parent`.
func _add_stem(parent: Control) -> Control:
	var view: Control = (load(STEM_SCENE) as PackedScene).instantiate() as Control
	parent.add_child(view)
	view.call("show_content", item.stem(), context, item)
	return view


## Plain text of a rich content (`{blocks}`) — formulas read as their `speech_text`.
func _plain(rich: Variant) -> String:
	return RichText.plain_text(rich, context)


## A choice card for an option content (text in the attempt language; its first formula, if any, as the icon —
## the speech text stands in until the SVG is ready). `index` < 6 gets its keyboard number (`1–6`).
func _make_choice(content: Variant, toggle: bool, index: int = -1) -> Button:
	var b: Button = Button.new()
	b.toggle_mode = toggle
	b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
	b.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	b.alignment = HORIZONTAL_ALIGNMENT_LEFT
	var text: String = RichText.text_without_math(content, context)
	var formulas: Array[MathAssetRefModel] = RichText.math_of(content)
	if text.is_empty() and not formulas.is_empty():
		text = formulas[0].fallback_text()
	b.text = ("%d. %s" % [index + 1, text]) if index >= 0 and index < 6 else text
	if not formulas.is_empty() and context != null and context.math != null:
		_load_icon(b, formulas[0])
	return b


func _load_icon(button: Button, formula: MathAssetRefModel) -> void:
	var tex: Texture2D = await context.math.texture(formula,
			MathSvgCache.scale_for(MathInline.font_px(button), MathInline.screen_scale()), MathInline.text_color(button))
	if is_instance_valid(button) and tex != null:
		button.icon = tex
