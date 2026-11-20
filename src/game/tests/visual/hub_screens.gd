extends SceneTree
## TEST-ONLY visual capture of the Main Hub HUD and the "Nhà trường giao" tab (T031, TC-I18N-006, DESIGN_SYSTEM §9):
## grade bands g1_3 / g6_7 × vi-VN / en-US / xx-PS, with the assignments of the contract example (no network: the
## list is put into AppState directly). Needs a renderer (not --headless):
##   godot --path src/game -s res://tests/visual/hub_screens.gd
## Images go to res://tests/scene/screenshots/<platform>/hub_*.png (review artefacts, ignored by git).

const WORLD: String = "res://scenes/world/WorldRoot.tscn"
const OUTPUT_DIR: String = "res://tests/scene/screenshots"
const BANDS: PackedStringArray = ["g1_3", "g6_7"]
const LOCALES: PackedStringArray = ["vi-VN", "en-US", "xx-PS"]
const SETTLE_FRAMES: int = 6


func _initialize() -> void:
	_run.call_deferred()


func _run() -> void:
	if DisplayServer.get_name() == "headless":
		printerr("hub_screens: needs a renderer (run without --headless)")
		quit(2)
		return
	var output_dir: String = OUTPUT_DIR.path_join(OS.get_name().to_lower())
	DirAccess.make_dir_recursive_absolute(ProjectSettings.globalize_path(output_dir))
	var marker: FileAccess = FileAccess.open(OUTPUT_DIR.path_join(".gdignore"), FileAccess.WRITE)
	if marker != null:
		marker.close()
	var app_state: Node = root.get_node("AppState")
	var settings: Node = root.get_node("Settings")
	var quality: Node = root.get_node("QualityManager")
	app_state.call("start_ui", root)
	quality.call("apply_preset", "medium", false)
	quality.get("settings").set("quality_preset", "medium")
	var body: Dictionary = JSON.parse_string(FileAccess.get_file_as_string(
			"res://tests/fixtures/contract/openapi/listMyAssignments.200.json")) as Dictionary
	app_state.call("set_assignments", MyAssignmentModel.list_from(body["data"] as Array))
	var world: Node = (load(WORLD) as PackedScene).instantiate()
	root.add_child(world)
	var count: int = 0
	for band: String in BANDS:
		app_state.call("set_grade_band", band)
		for locale: String in LOCALES:
			settings.call("set_locale", locale, false)
			for screen: String in ["hud", "journal"]:
				var journal: Control = (world.get("panels") as Dictionary).get(&"quest_journal") as Control
				if screen == "journal" and (journal == null or not journal.visible):
					world.call("_on_journal_pressed")
				elif screen == "hud" and journal != null and journal.visible:
					journal.call("close")
				for i: int in SETTLE_FRAMES:
					await process_frame
				await RenderingServer.frame_post_draw
				var image: Image = root.get_texture().get_image()
				image.save_png(ProjectSettings.globalize_path(output_dir.path_join(
						"hub_%s_%s_%s.png" % [screen, band, locale.replace("-", "_")])))
				count += 1
	print("hub_screens: %d images in %s" % [count, output_dir])
	quit(0)
