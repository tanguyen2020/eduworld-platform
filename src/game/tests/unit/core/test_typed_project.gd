extends GdUnitTestSuite
## F060-AC-01 (ADR-0014): the project pins the Godot minor in .godot-version and turns untyped declarations into
## errors, so an untyped script does not compile (CI fails through tools/check_typed.gd).
## The compile check runs in a throw-away project with the same setting, in a child Godot process, so the
## expected parse error does not pollute this test run.

const PROBE_DIR: String = "user://typed_probe"


func after_test() -> void:
	for file_name: String in ["project.godot", "probe.gd"]:
		DirAccess.remove_absolute(PROBE_DIR.path_join(file_name))
	DirAccess.remove_absolute(PROBE_DIR)


func test_godot_version_matches_pin() -> void:
	var pinned: String = FileAccess.get_file_as_string("res://.godot-version").strip_edges()
	var info: Dictionary = Engine.get_version_info()
	assert_str("%d.%d" % [int(info["major"]), int(info["minor"])]).is_equal(pinned)
	var features: PackedStringArray = ProjectSettings.get_setting("application/config/features") as PackedStringArray
	assert_bool(pinned in features).is_true()


func test_untyped_declaration_is_an_error() -> void:
	assert_int(int(ProjectSettings.get_setting("debug/gdscript/warnings/untyped_declaration"))).is_equal(2)


func test_untyped_script_fails_and_typed_script_runs() -> void:
	assert_int(_run_probe("\tvar count = 1\n\tquit(count - 1)\n")).is_not_equal(0)
	assert_int(_run_probe("\tvar count: int = 1\n\tquit(count - 1)\n")).is_equal(0)


func _run_probe(body: String) -> int:
	DirAccess.make_dir_recursive_absolute(PROBE_DIR)
	var project: FileAccess = FileAccess.open(PROBE_DIR.path_join("project.godot"), FileAccess.WRITE)
	project.store_string("config_version=5\n[application]\nconfig/name=\"typed_probe\"\n[debug]\ngdscript/warnings/untyped_declaration=2\n")
	project.close()
	var script: FileAccess = FileAccess.open(PROBE_DIR.path_join("probe.gd"), FileAccess.WRITE)
	script.store_string("extends SceneTree\nfunc _initialize() -> void:\n" + body)
	script.close()
	var output: Array = []
	var dir: String = ProjectSettings.globalize_path(PROBE_DIR)
	return OS.execute(OS.get_executable_path(), ["--headless", "--path", dir, "-s", "res://probe.gd"], output, true)
