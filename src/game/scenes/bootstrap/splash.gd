extends Control
## Splash scene (F060): end of the F060 boot path. F061 continues from here to VersionCheck / Resume / Login.

signal splash_shown()


func _ready() -> void:
	Log.info("splash_shown", {"scene": scene_file_path})
	splash_shown.emit()
