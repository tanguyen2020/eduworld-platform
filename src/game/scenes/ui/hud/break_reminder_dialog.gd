extends PanelContainer
## BreakReminderDialog (FR-GAME-012, CHILD_SAFETY, STUDENT_GAME_UX_CORE S03 "Nhắc nghỉ", F062-AC-21): Mây's gentle
## reminder after `break_reminder_minutes` of continuous play. Never blocks the game (no modal overlay, the world keeps
## running) and closes with one tap. Shown by WorldRoot when PlayTimeService emits `break_reminder_shown`.

signal dismissed()

var minutes_played: int = 0

@onready var _body: Label = %Body
@onready var _ok: Button = %Ok


func _ready() -> void:
	visible = false
	_ok.pressed.connect(close)
	EventBus.locale_changed.connect(_on_locale_changed)


func open(minutes: int) -> void:
	minutes_played = minutes
	_render()
	show()


func close() -> void:
	if visible:
		hide()
		dismissed.emit()


func _render() -> void:
	_body.text = tr("ui.common.break_reminder").format({"minutes": minutes_played})


func _on_locale_changed(_locale: String) -> void:
	_render()
