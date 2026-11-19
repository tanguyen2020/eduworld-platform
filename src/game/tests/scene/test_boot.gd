extends GdUnitTestSuite
## F060-AC-02 (GODOT_ARCHITECTURE §4, NFR-MAINT-007): autoload order and headless Boot → Splash.

const BootScript := preload("res://scenes/bootstrap/boot.gd")
const EXPECTED_ORDER: PackedStringArray = [
	"Config", "Log", "EventBus", "AppState", "SceneRouter", "AudioDirector", "Settings",
]


func test_autoload_order_follows_architecture() -> void:
	assert_array(Array(BootScript.autoload_order())).is_equal(Array(EXPECTED_ORDER))


func test_autoloads_are_in_the_tree() -> void:
	for autoload_name: String in EXPECTED_ORDER:
		assert_object(get_tree().root.get_node_or_null(autoload_name)).is_not_null()


func test_main_scene_is_boot() -> void:
	assert_str(String(ProjectSettings.get_setting("application/run/main_scene"))).is_equal("res://scenes/bootstrap/Boot.tscn")


func test_boot_reaches_splash_headless() -> void:
	var changed: Array[String] = []
	var on_changed: Callable = func(path: String) -> void: changed.append(path)
	SceneRouter.scene_changed.connect(on_changed)
	var runner: GdUnitSceneRunner = scene_runner("res://scenes/bootstrap/Boot.tscn")
	await runner.simulate_frames(30)
	await await_millis(200)
	SceneRouter.scene_changed.disconnect(on_changed)
	assert_bool(Config.is_valid()).is_true()
	assert_array(changed).contains(["res://scenes/bootstrap/Splash.tscn"])
	assert_object(get_tree().root.theme).is_not_null()
