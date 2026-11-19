extends Control
## Splash scene (F060 boot path; F061 VersionCheck host): logs `splash_shown`, then starts the entry flow
## (AuthService.start: client-config → Resume / Login). Tests turn `AuthService.autostart` off.

signal splash_shown()


func _ready() -> void:
	Log.info("splash_shown", {"scene": scene_file_path})
	splash_shown.emit()
	if AuthService.autostart and AuthService.state == GameStateMachine.State.BOOT:
		AuthService.start.call_deferred()
