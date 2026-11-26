extends GdUnitTestSuite
## F097-AC-09 (F096-1, STUDENT_GAME_UX_CORE S02): the 3D preview is a large, central stage that turns like a turntable
## (paused after the student turns it, still with reduced motion); presets are picture tiles (AvatarThumb in the
## preset colours + name), skin/hair colours are round swatches with a tick when chosen, every option is at least the
## band touch target. F070 behaviour is covered by test_character_creation_scene.gd (unchanged).

const SCENE: String = "res://scenes/character/CharacterCreation.tscn"

var h: AuthHarness = null
var _services: ServiceLocator = null
var _band: String = ""
var _reduce: bool = false


func before_test() -> void:
	AppState.start_ui(get_tree().root)
	_band = AppState.grade_band
	_reduce = bool(Settings.reduce_motion)
	_services = AppState.services
	h = AuthHarness.create()
	await h.sign_in()
	ProfileFixtures.script_first_time(h.http)
	AppState.services = ServiceLocator.new(h.api, AppState, h.clock)


func after_test() -> void:
	h.dispose()
	AppState.services = _services
	AppState.set_grade_band(_band)
	Settings.reduce_motion = _reduce


func _open(band: String) -> Array:
	AppState.set_grade_band(band)
	var runner: GdUnitSceneRunner = scene_runner(SCENE)
	var screen: Control = runner.scene() as Control
	screen.theme = AppState.theme_service.current_theme if AppState.theme_service != null else null
	for i: int in 40:
		await runner.simulate_frames(1)
		var model: CharacterCreationModel = screen.get("model") as CharacterCreationModel
		if model != null and model.state != CharacterCreationModel.State.LOADING:
			break
	return [runner, screen]


func test_preview_is_the_largest_part_of_the_screen() -> void:
	var opened: Array = await _open("g1_3")
	var runner: GdUnitSceneRunner = opened[0]
	var screen: Control = opened[1]
	await runner.simulate_frames(3)
	var stage: Control = screen.find_child("Stage", true, false) as Control
	var options: Control = screen.find_child("Options", true, false) as Control
	assert_float(stage.size.x).is_greater(options.size.x)
	assert_float(stage.size.y).is_greater(screen.size.y * 0.5)


func test_preview_turns_like_a_turntable_and_pauses_after_a_manual_turn() -> void:
	Settings.reduce_motion = false
	var opened: Array = await _open("g1_3")
	var runner: GdUnitSceneRunner = opened[0]
	var screen: Control = opened[1]
	var rig: Node3D = screen.find_child("AvatarRig", true, false) as Node3D
	var before: float = rig.rotation.y
	await runner.simulate_frames(10, 50)
	assert_bool(absf(rig.rotation.y - before) > 0.001).is_true()
	(screen.find_child("RotateLeft", true, false) as Button).pressed.emit()
	var turned: float = rig.rotation.y
	await runner.simulate_frames(5, 50)
	assert_float(rig.rotation.y).is_equal_approx(turned, 0.0001)


func test_reduced_motion_keeps_the_preview_still() -> void:
	Settings.reduce_motion = true
	var opened: Array = await _open("g1_3")
	var runner: GdUnitSceneRunner = opened[0]
	var screen: Control = opened[1]
	var rig: Node3D = screen.find_child("AvatarRig", true, false) as Node3D
	var before: float = rig.rotation.y
	await runner.simulate_frames(10, 50)
	assert_float(rig.rotation.y).is_equal_approx(before, 0.0001)


func test_presets_are_picture_tiles_and_swatches_are_round_with_a_tick() -> void:
	var opened: Array = await _open("g1_3")
	var screen: Control = opened[1]
	var presets: Node = screen.find_child("PresetGrid", true, false)
	assert_int(presets.get_child_count()).is_greater(0)
	for b: Node in presets.get_children():
		assert_str(String((b as Button).theme_type_variation)).is_equal("PictureTile")
		assert_object(b.get_node_or_null("Thumb") as AvatarThumb).is_not_null()
		assert_str((b as Button).text).is_not_empty()
	var first: Button = presets.get_child(0) as Button
	first.pressed.emit()
	assert_bool((first.get_node("Thumb") as AvatarThumb).is_selected()).is_true()
	var tones: Node = screen.find_child("SkinTones", true, false)
	for b: Node in tones.get_children():
		assert_object(b.get_node_or_null("Swatch") as ColorDot).is_not_null()


func test_every_option_meets_the_band_touch_target() -> void:
	for band: String in UiTokens.DENSITY_BANDS:
		var opened: Array = await _open(band)
		var runner: GdUnitSceneRunner = opened[0]
		var screen: Control = opened[1]
		await runner.simulate_frames(2)
		var minimum: float = float(UiTokens.DENSITY_BUTTON_MIN_HEIGHT[band])
		for node: Node in screen.find_children("*", "BaseButton", true, false):
			var c: Control = node as Control
			if c.is_visible_in_tree():
				assert_float(maxf(c.get_combined_minimum_size().y, c.size.y)) \
						.override_failure_message("%s %s" % [band, c.name]).is_greater_equal(minimum - 0.5)
		var tile: Control = screen.find_child("PresetGrid", true, false).get_child(0) as Control
		assert_float(tile.get_combined_minimum_size().y).is_greater_equal(minimum * 1.5)
