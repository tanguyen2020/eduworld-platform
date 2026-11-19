extends Control
## ForceUpdate (F061-AC-01, F061-AC-27, FR-GAME-008): "Có phiên bản mới" with the minimum version and, when the
## server sent one, a button opening `store_url`. No login is possible from here; the stored session is kept.

@onready var _body: Label = %Body
@onready var _store: Button = %Store


func _ready() -> void:
	_store.visible = not AuthService.store_url.is_empty()
	_store.pressed.connect(_on_store_pressed)
	EventBus.locale_changed.connect(_on_locale_changed)
	_render()


func _render() -> void:
	var version: String = AuthService.min_client_version
	_body.visible = not version.is_empty()
	_body.text = tr("ui.login.update_body").format({"version": version})


func _on_store_pressed() -> void:
	AuthService.open_store()


func _on_locale_changed(_locale: String) -> void:
	_render()
