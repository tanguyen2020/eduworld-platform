extends VBoxContainer
## "Cần cập nhật game để làm hoạt động này" (STUDENT_GAME_UX_LEARNING §3.5, FR-EXR-015, F063-AC-25): shown instead of
## a renderer when this client has no renderer for the `activity_type`, is older than the content needs, or the
## server answered `422 ACTIVITY_TYPE_NOT_SUPPORTED`. "Cập nhật" opens the store when the server gave its URL; "Quay
## lại" closes the player. The host sends the telemetry.

signal back_pressed()

@onready var _update: Button = %Update
@onready var _back: Button = %Back


func _ready() -> void:
	_update.pressed.connect(_on_update_pressed)
	_back.pressed.connect(func() -> void: back_pressed.emit())


func _on_update_pressed() -> void:
	AuthService.open_store()
