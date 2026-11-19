extends Control
## Boot scene (GODOT_ARCHITECTURE §5, F060-AC-02, F060-AC-03): checks Config, starts the UI services and moves on
## to Splash; an invalid configuration shows ConfigError and stops there. Calls service methods only (R1).

const SPLASH_SCENE: String = "res://scenes/bootstrap/Splash.tscn"
const CONFIG_ERROR_SCENE: String = "res://scenes/bootstrap/ConfigError.tscn"


func _ready() -> void:
	if not Config.is_valid():
		Log.error("config_invalid", {"code": Config.error_code, "key": Config.error_key})
		get_tree().change_scene_to_file.call_deferred(CONFIG_ERROR_SCENE)
		return
	AppState.start_ui(get_tree().root)
	Log.info("boot_completed", {
		"env": Config.env_name,
		"client_version": Config.client_version,
		"autoloads": ",".join(autoload_order()),
		"locale": TranslationServer.get_locale(),
		"density": AppState.grade_band,
		"shared_device": Config.shared_device,
	})
	SceneRouter.goto(SPLASH_SCENE)


## Autoload names in declaration order (project.godot), logged at boot for support and checked by tests.
static func autoload_order() -> PackedStringArray:
	var names: PackedStringArray = []
	for property: Dictionary in ProjectSettings.get_property_list():
		var setting_name: String = String(property.get("name", ""))
		if setting_name.begins_with("autoload/"):
			names.append(setting_name.trim_prefix("autoload/"))
	return names
