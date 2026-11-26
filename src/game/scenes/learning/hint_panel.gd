extends VBoxContainer
## Multi-level hints (STUDENT_GAME_UX_LEARNING §4.7, ACTIVITY_ENGINE §8.3, F063-AC-15/16). "Gợi ý" asks the host
## for the next level (`hint_requested`); the panel shows "Mây nhắc em một chút nhé.", "Gợi ý {n}/{total}" and the
## hint text with formulas. `409 HINTS_EXHAUSTED` greys the button with `ui.activity_player.hints_exhausted`;
## `403 HINTS_NOT_ALLOWED` or a mode without hints hides it. The panel never counts hints: levels come from the
## server (`HintResult.level/total_levels`, `hint_count`, `hints_revealed`).

signal hint_requested()
signal math_failed()

var allowed: bool = false
var exhausted: bool = false
var level: int = 0
var total: int = 0

@onready var _button: Button = %HintButton
@onready var _level: Label = %Level
@onready var _status: Label = %Status
@onready var _intro: Label = %Intro
@onready var _text: RichTextLabel = %HintText


func _ready() -> void:
	_button.pressed.connect(request)
	EventBus.locale_changed.connect(_on_locale_changed)


func _exit_tree() -> void:
	if EventBus.locale_changed.is_connected(_on_locale_changed):
		EventBus.locale_changed.disconnect(_on_locale_changed)


## New item: hints allowed by the mode, `hint_count` (-1 unknown) and levels already revealed (resume).
func configure(mode_allows: bool, hint_count: int, revealed: int) -> void:
	allowed = mode_allows and hint_count != 0
	total = maxi(hint_count, 0)
	level = revealed
	exhausted = hint_count > 0 and revealed >= hint_count
	_status.text = "ui.activity_player.hints_exhausted" if exhausted else ""
	_intro.visible = false
	_text.clear()
	_text.visible = false
	_render()


## Keyboard `H` / button: asks the next level unless greyed out or hidden.
func request() -> void:
	if allowed and not exhausted and not _button.disabled:
		_status.text = ""
		hint_requested.emit()


func set_busy(busy: bool) -> void:
	_button.disabled = busy or exhausted


## Coroutine: shows a revealed level.
func show_hint(hint: HintResultModel, math: MathSvgCache) -> void:
	level = hint.level
	total = hint.total_levels
	exhausted = hint.is_last()
	_status.text = ""
	_intro.visible = true
	_text.visible = true
	_text.clear()
	_text.add_text(hint.text)
	_render()
	for formula: MathAssetRefModel in hint.math_assets:
		if formula.is_block():
			_text.newline()
		if not await MathInline.append_to(_text, formula, math):
			math_failed.emit()


## `409 HINTS_EXHAUSTED`.
func mark_exhausted() -> void:
	exhausted = true
	_status.text = "ui.activity_player.hints_exhausted"
	_render()


## `403 HINTS_NOT_ALLOWED` (or exam-like mode): the button disappears for this item.
func hide_hints() -> void:
	allowed = false
	_render()


## Network/server error: the answer stays, the student may try again.
func show_error() -> void:
	_status.text = "ui.activity_player.hint_error"
	_render()


func is_button_visible() -> bool:
	return _button.visible


func is_button_disabled() -> bool:
	return _button.disabled


## A revealed hint is on screen (F097: the coach stops cheering while Mây gives the hint).
func is_showing_hint() -> bool:
	return visible and _text.visible


func level_text() -> String:
	return _level.text


func _render() -> void:
	visible = allowed
	_button.visible = allowed
	_button.disabled = exhausted
	_level.visible = total > 0 and level > 0
	_level.text = tr("ui.activity_player.hint_level").format({"n": level, "total": total})
	_status.visible = not _status.text.is_empty()


func _on_locale_changed(_locale: String) -> void:
	_render()
