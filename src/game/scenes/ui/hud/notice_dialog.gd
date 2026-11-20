extends PanelContainer
## NoticeDialog: a short message with "Đã hiểu" — e.g. `ui.activity_player.update_required` when an assignment opens a
## screen this client does not have yet (F062-AC-10) or `ui.error.<CODE>` of a failed action. Text comes only from
## translation keys (never the server `detail`).

signal dismissed()

var message_key: String = ""
var message_args: Dictionary = {}

@onready var _body: Label = %Body
@onready var _ok: Button = %Ok


func _ready() -> void:
	visible = false
	_ok.pressed.connect(close)
	EventBus.locale_changed.connect(_on_locale_changed)


func open(key: String, args: Dictionary = {}) -> void:
	message_key = key
	message_args = args
	_render()
	show()
	_ok.grab_focus.call_deferred()


func close() -> void:
	if visible:
		hide()
		dismissed.emit()


func _render() -> void:
	_body.text = tr(message_key).format(message_args) if not message_args.is_empty() else tr(message_key)


func _on_locale_changed(_locale: String) -> void:
	_render()
