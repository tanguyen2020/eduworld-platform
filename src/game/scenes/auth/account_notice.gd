extends Control
## Account notices after a login refusal (GODOT_ARCHITECTURE §9): NotStudent (`GAME_ACCESS_STUDENT_ONLY`,
## F061-AC-07), AccountNotProvisioned (F061-AC-08), AccountBlocked (`ACCOUNT_LOCKED`, `NO_ACTIVE_ENROLLMENT`…,
## F061-AC-09). No token is kept; "Đăng nhập tài khoản khác" returns to the form. The text comes from
## `message_key` or, when empty, from `ui.error.<AuthService.notice_code>` — never the server `detail`.

@export var message_key: String = ""

@onready var _body: Label = %Body
@onready var _other: Button = %Other


func _ready() -> void:
	_other.pressed.connect(_on_other_pressed)
	EventBus.locale_changed.connect(_on_locale_changed)
	_render()


func _render() -> void:
	_body.text = tr(message_key) if not message_key.is_empty() else L10n.error_text(AuthService.notice_code)


func _on_other_pressed() -> void:
	AuthService.back_to_login()


func _on_locale_changed(_locale: String) -> void:
	_render()
