extends GdUnitTestSuite
## F098-AC-09 (owner "ma trận test khối 1–9"; STUDENT_GAME_UX §4.3, DESIGN_SYSTEM §5.2, LEARNING_EXPERIENCE §8,
## NFR-A11Y-001): every student screen of the matrix is built for EVERY grade band (g1_3, g4_5, g6_7, g8_9) at the three
## reference windows (1280×720, 1024×600, 720×1280 → logical 1280×720, 1280×750, 1280×2276) inside a SubViewport of
## that size with the band's Theme, and BandLayoutProbe checks touch targets, the smallest font, answer tiles and that
## nothing is cut off or outside the view. Screens: S01 login, Hub HUD, activity (single choice, true/false, matching,
## numeric), feedback (grading, right, not yet right), result, S02 character creation, wardrobe and shop (F069), level
## up, world map. FakeHttp + contract fixtures (TEST-ONLY).

const HOST: String = "res://scenes/learning/ActivityHost.tscn"
const SETTLE_FRAMES: int = 6

var s: F069SceneHarness = null
## What each build showed to the probe (printed, for the evidence table).
var _seen: PackedStringArray = []
var _autostart: bool = true


func before() -> void:
	_autostart = AuthService.autostart
	AuthService.autostart = false


func after() -> void:
	AuthService.autostart = _autostart


func before_test() -> void:
	s = F069SceneHarness.new()
	await s.start("g1_3")


func after_test() -> void:
	for line: String in _seen:
		print("band-matrix ", line)
	_seen.clear()
	s.h.http.delay_frames = 1
	s.dispose()


## Builds `build` (a Callable returning the screen root, coroutine allowed) for every band × window, probes it and
## returns every problem found.
func _matrix(label: String, build: Callable) -> PackedStringArray:
	var failures: PackedStringArray = []
	for band: String in UiTokens.DENSITY_BANDS:
		for window: String in BandLayoutProbe.SIZES.keys():
			AppState.set_grade_band(band)
			var vp: SubViewport = SubViewport.new()
			vp.size = BandLayoutProbe.SIZES[window] as Vector2i
			vp.render_target_update_mode = SubViewport.UPDATE_DISABLED
			add_child(vp)
			var root: Node = await build.call(vp, band)
			if root == null:
				failures.append("%s %s %s: not built" % [label, band, window])
			else:
				for i: int in SETTLE_FRAMES:
					await await_idle_frame()
				failures.append_array(BandLayoutProbe.check(root, vp, band, "%s %s" % [label, window]))
				if BandLayoutProbe.last_texts == 0:
					failures.append("%s %s %s: no visible text checked" % [label, band, window])
				_seen.append("%s %s %s: %d buttons, %d texts, %d tiles" % [label, band, window,
						BandLayoutProbe.last_buttons, BandLayoutProbe.last_texts, BandLayoutProbe.last_tiles])
			vp.queue_free()
			await await_idle_frame()
	return failures


## Adds `node` to the viewport with the band Theme (a SubViewport does not inherit the window Theme).
func _mount(vp: SubViewport, node: Node) -> Node:
	var theme: Theme = AppState.theme_service.current_theme
	if node is Control:
		(node as Control).theme = theme
	else:
		for child: Node in node.get_children():
			if child is Control:
				(child as Control).theme = theme
	vp.add_child(node)
	return node


func _scene(path: String) -> Node:
	return (load(path) as PackedScene).instantiate()


func _assert_none(failures: PackedStringArray) -> void:
	assert_array(failures).override_failure_message("\n".join(failures)).is_empty()


# ---------- builders ----------

func _activity(vp: SubViewport, band: String, types: PackedStringArray, result: Dictionary = {},
		tap: int = -1, grading: bool = false) -> Node:
	var host: Control = _mount(vp, _scene(HOST)) as Control
	host.set("services", s.services)
	host.set("client_version", "1.2.3")
	host.set("grade_band", band)
	s.h.http.delay_frames = 1
	s.h.http.clear(LearningFixtures.ROUTE_START)
	s.h.http.reply(LearningFixtures.ROUTE_START, 201, LearningFixtures.attempt_with_types(types))
	await host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID, "mode": "practice", "context_type": "free_play"})
	for i: int in 3:
		await await_idle_frame()
	if tap >= 0:
		s.h.http.clear(LearningFixtures.ROUTE_RESPOND)
		s.h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, result)
		if grading:
			s.h.http.delay_frames = 100000
		(host.get("renderer") as ActivityRendererBase).select_index(tap)
		for i: int in 10:
			await await_idle_frame()
	return host


func _correct_with_answer() -> Dictionary:
	var r: Dictionary = LearningFixtures.response_result()
	r["correct_response"] = {"option_id": "o_b"}
	return r


# ---------- tests ----------

func test_login_fits_every_band() -> void:
	AuthService.form.load_memory(true, "", false)
	AuthService.form.end_submit("")
	_assert_none(await _matrix("login", func(vp: SubViewport, _band: String) -> Node:
		return _mount(vp, _scene("res://scenes/auth/Login.tscn"))))


func test_hub_hud_fits_every_band() -> void:
	await s.services.progression.refresh()
	await s.services.inventory.load_wallet()
	_assert_none(await _matrix("hud", func(vp: SubViewport, _band: String) -> Node:
		return _mount(vp, _scene("res://scenes/ui/hud/MainHud.tscn"))))


func test_activity_single_choice_fits_every_band() -> void:
	_assert_none(await _matrix("activity", func(vp: SubViewport, band: String) -> Node:
		return await _activity(vp, band, PackedStringArray(["single_choice"]))))


func test_activity_true_false_fits_every_band() -> void:
	_assert_none(await _matrix("true_false", func(vp: SubViewport, band: String) -> Node:
		return await _activity(vp, band, PackedStringArray(["true_false"]))))


func test_activity_matching_fits_every_band() -> void:
	_assert_none(await _matrix("matching", func(vp: SubViewport, band: String) -> Node:
		return await _activity(vp, band, PackedStringArray(["matching"]))))


func test_activity_numeric_fits_every_band() -> void:
	_assert_none(await _matrix("numeric", func(vp: SubViewport, band: String) -> Node:
		return await _activity(vp, band, PackedStringArray(["numeric"]))))


func test_feedback_grading_fits_every_band() -> void:
	_assert_none(await _matrix("tap_grading", func(vp: SubViewport, band: String) -> Node:
		return await _activity(vp, band, PackedStringArray(["single_choice"]),
				LearningFixtures.response_result_correct(), 1, true)))


func test_feedback_right_fits_every_band() -> void:
	_assert_none(await _matrix("tap_correct", func(vp: SubViewport, band: String) -> Node:
		return await _activity(vp, band, PackedStringArray(["single_choice"]),
				LearningFixtures.response_result_correct(), 1)))


func test_feedback_not_yet_right_fits_every_band() -> void:
	_assert_none(await _matrix("tap_incorrect", func(vp: SubViewport, band: String) -> Node:
		return await _activity(vp, band, PackedStringArray(["single_choice"]), _correct_with_answer(), 0)))


func test_result_fits_every_band() -> void:
	_assert_none(await _matrix("result", func(vp: SubViewport, band: String) -> Node:
		var view: Control = _mount(vp, _scene("res://scenes/learning/ActivityResult.tscn")) as Control
		view.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
		view.call("show_review", AttemptReviewModel.from_dict(LearningFixtures.review()), band,
				[{"kind": "xp", "amount": 40}, {"kind": "badge"}])
		return view))


func test_character_creation_fits_every_band() -> void:
	_assert_none(await _matrix("character", func(vp: SubViewport, _band: String) -> Node:
		# A first-time student (no avatar, no nickname yet): S02 opens on its first step.
		ProfileFixtures.script_first_time(s.h.http)
		s.services.profile.profile = null
		var screen: Control = _mount(vp, _scene("res://scenes/character/CharacterCreation.tscn")) as Control
		for i: int in 40:
			await await_idle_frame()
			var model: CharacterCreationModel = screen.get("model") as CharacterCreationModel
			if model != null and model.state != CharacterCreationModel.State.LOADING:
				break
		var loaded: CharacterCreationModel = screen.get("model") as CharacterCreationModel
		assert_int(loaded.state).override_failure_message("S02 not ready").is_equal(CharacterCreationModel.State.READY)
		return screen))


func test_wardrobe_fits_every_band() -> void:
	_assert_none(await _matrix("wardrobe", func(vp: SubViewport, _band: String) -> Node:
		var screen: Control = _mount(vp, _scene("res://scenes/inventory/InventoryShopScreen.tscn")) as Control
		screen.call("open", {"tab": "wardrobe"})
		for i: int in 8:
			await await_idle_frame()
		screen.call("select_entry", 1)
		return screen))


func test_shop_fits_every_band() -> void:
	_assert_none(await _matrix("shop", func(vp: SubViewport, _band: String) -> Node:
		var screen: Control = _mount(vp, _scene("res://scenes/inventory/InventoryShopScreen.tscn")) as Control
		screen.call("open", {"tab": "shop"})
		for i: int in 8:
			await await_idle_frame()
		screen.call("select_entry", 0)
		return screen))


func test_level_up_fits_every_band() -> void:
	_assert_none(await _matrix("level_up", func(vp: SubViewport, band: String) -> Node:
		var modal: Control = _mount(vp, _scene("res://scenes/progression/LevelUpModal.tscn")) as Control
		var list: Array[LevelUpModel] = [LevelUpModel.from_dict(F069Fixtures.level_up(3, 4, true))]
		modal.call("present", LevelUpModel.merge(list), ProgressionModel.from_dict(F069Fixtures.progression(4, 20, 140)),
				band)
		await await_idle_frame()
		modal.call("skip_effect")
		return modal))


func test_world_map_fits_every_band() -> void:
	_assert_none(await _matrix("world_map", func(vp: SubViewport, band: String) -> Node:
		var map: Control = _mount(vp, _scene("res://scenes/world/WorldMap.tscn")) as Control
		map.show()
		map.call("render", WorldMapModel.build(WorldFixtures.area_models(), WorldFixtures.exploration(), band, "main_hub"))
		return map))


## The probe itself catches what it must (a guard against a silent matrix).
func test_probe_flags_small_buttons_small_fonts_and_overflow() -> void:
	AppState.set_grade_band("g1_3")
	var vp: SubViewport = auto_free(SubViewport.new()) as SubViewport
	vp.size = Vector2i(1280, 720)
	add_child(vp)
	var root: Control = Control.new()
	root.theme = AppState.theme_service.current_theme
	root.set_anchors_and_offsets_preset(Control.PRESET_FULL_RECT)
	vp.add_child(root)
	var small: Button = Button.new()
	small.text = "x"
	small.custom_minimum_size = Vector2(10, 10)
	root.add_child(small)
	var off: Label = Label.new()
	off.text = "off screen"
	off.position = Vector2(1400, 10)
	root.add_child(off)
	var tiny: Label = Label.new()
	tiny.text = "tiny"
	tiny.add_theme_font_size_override(&"font_size", 10)
	tiny.position = Vector2(10, 300)
	root.add_child(tiny)
	var cut: Label = Label.new()
	cut.text = "a long line that cannot fit"
	cut.clip_text = true
	cut.size = Vector2(30, 40)
	cut.position = Vector2(10, 400)
	root.add_child(cut)
	await await_idle_frame()
	await await_idle_frame()
	# The small button is held at its theme minimum height; force the probe to see a too-small one.
	small.size = Vector2(10, 10)
	var text: String = "\n".join(BandLayoutProbe.check(root, vp, "g1_3", "probe"))
	assert_str(text).contains("outside the view")
	assert_str(text).contains("font 10 px")
	assert_str(text).contains("trimmed text")
