extends Control
## PLACEHOLDER (AGENTS R17) — in-world destination of the F061 entry flow until the Main Hub (F062) and the first
## avatar screen (F070) exist; replaced by changing `GameStateMachine.IN_WORLD_SCENE`. Not production UI. It shows
## the account actions F061 owns: "Đăng xuất" with confirmation (Cài đặt → Tài khoản comes with the settings
## screen) and the curfew warning (F061-AC-36).

@onready var _curfew_warning: Label = %CurfewWarning
@onready var _logout: Button = %Logout
@onready var _logout_confirm: PanelContainer = %LogoutConfirm


func _ready() -> void:
	Log.info("world_placeholder_shown", {"placeholder": true})
	_curfew_warning.visible = false
	_logout.pressed.connect(_on_logout_pressed)
	AuthService.curfew_warning_raised.connect(_on_curfew_warning_raised)


func _exit_tree() -> void:
	if AuthService.curfew_warning_raised.is_connected(_on_curfew_warning_raised):
		AuthService.curfew_warning_raised.disconnect(_on_curfew_warning_raised)


func _on_logout_pressed() -> void:
	_logout_confirm.call("open")


func _on_curfew_warning_raised(minutes_left: int) -> void:
	_curfew_warning.visible = true
	_curfew_warning.text = tr("ui.login.curfew_warning").format({"minutes": minutes_left})
