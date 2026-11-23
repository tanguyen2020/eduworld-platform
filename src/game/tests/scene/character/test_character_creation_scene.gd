extends GdUnitTestSuite
## F070-AC-05, F070-AC-06, F070-AC-09 (client part) on the S02 scene (T013, T018, T025; STUDENT_GAME_UX_CORE S02):
## `g1_3` finishes with buttons only — 3 steps, touch targets ≥ the `g1_3` density (64 dp), a voice hint per step that
## does not crash without a recorded clip, no text input anywhere (R16); every choice redraws the 3D preview without
## a request; the name step shows the server's suggestions as chips with "Gợi ý khác"; confirm saves the avatar then
## the nickname option. Strings come from `ui.avatar.*` keys, catalog names from the catalog.

const SCENE: String = "res://scenes/character/CharacterCreation.tscn"
const BenchmarkScript := preload("res://tests/perf/main_hub_benchmark.gd")
## F070-AC-06: one choice redraws the preview within 100 ms.
const PREVIEW_BUDGET_USEC: int = 100_000
## Best of PREVIEW_PASSES full passes over every choice: a shared CI/dev machine under load inflates one pass, the best
## pass is the cost of the redraw itself (same approach as tests/integration/minigame_benchmark_test.gd).
const PREVIEW_PASSES: int = 3

var h: AuthHarness = null
var _services: ServiceLocator = null
var _band: String = ""
var _locale: String = ""


func before_test() -> void:
	_band = AppState.grade_band
	_locale = String(Settings.locale)
	Settings.set_locale("vi-VN", false)
	_services = AppState.services
	h = AuthHarness.create()
	await h.sign_in()
	ProfileFixtures.script_first_time(h.http)
	AppState.services = ServiceLocator.new(h.api, AppState, h.clock)


func after_test() -> void:
	h.dispose()
	AppState.services = _services
	AppState.set_grade_band(_band)
	Settings.set_locale(_locale, false)


func _open(band: String) -> Array:
	AppState.set_grade_band(band)
	var runner: GdUnitSceneRunner = scene_runner(SCENE)
	var screen: Control = runner.scene() as Control
	for i: int in 40:
		await runner.simulate_frames(1)
		var model: CharacterCreationModel = screen.get("model") as CharacterCreationModel
		if model != null and model.state != CharacterCreationModel.State.LOADING:
			break
	return [runner, screen]


func _model(screen: Control) -> CharacterCreationModel:
	return screen.get("model") as CharacterCreationModel


func _rig(screen: Control) -> AvatarBuilder:
	return screen.find_child("AvatarRig", true, false) as AvatarBuilder


func _visible_buttons(root: Node) -> Array[Button]:
	var out: Array[Button] = []
	for node: Node in root.find_children("*", "Button", true, false):
		var b: Button = node as Button
		if b.is_visible_in_tree():
			out.append(b)
	return out


func _press(screen: Control, unique: String) -> void:
	(screen.find_child(unique, true, false) as Button).pressed.emit()


## F070-AC-05: 3 steps, buttons only, ≥ 64 dp touch targets, voice hint without a clip.
func test_g1_3_finishes_with_three_steps_large_buttons_and_no_typing() -> void:
	var r: Array = await _open("g1_3")
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var screen: Control = r[1] as Control
	var model: CharacterCreationModel = _model(screen)
	assert_int(model.state).is_equal(CharacterCreationModel.State.READY)
	assert_int((screen.find_child("StepIndicator", true, false) as Control).get_child_count()).is_equal(3)
	assert_int(screen.find_children("*", "LineEdit", true, false).size()).is_equal(0)
	assert_int(screen.find_children("*", "TextEdit", true, false).size()).is_equal(0)
	var minimum: float = float(UiTokens.DENSITY_BUTTON_MIN_HEIGHT["g1_3"])
	assert_float(minimum).is_greater_equal(64.0)
	var voice: Control = screen.find_child("VoiceOver", true, false) as Control
	assert_bool(voice.visible).is_true()
	assert_str((voice.find_child("Caption", true, false) as Label).text).is_equal(tr("ui.avatar.voice.preset"))
	assert_bool((screen.find_child("SkipCustomize", true, false) as Button).visible).is_false()
	# Step 1: preset.
	for b: Button in _visible_buttons(screen):
		assert_float(b.get_combined_minimum_size().y).override_failure_message(b.name).is_greater_equal(minimum)
	(screen.find_child("PresetGrid", true, false).get_child(1) as Button).pressed.emit()
	_press(screen, "Next")
	await runner.simulate_frames(1)
	# Step 2: hair colour.
	assert_str(String(model.current_step())).is_equal(String(CharacterCreationModel.STEP_HAIR_COLOR))
	assert_str((voice.find_child("Caption", true, false) as Label).text).is_equal(tr("ui.avatar.voice.hair_color"))
	var colours: Node = screen.find_child("HairColors", true, false)
	assert_bool((colours as Control).is_visible_in_tree()).is_true()
	assert_bool((screen.find_child("BodyTypes", true, false) as Control).is_visible_in_tree()).is_false()
	for b: Button in _visible_buttons(screen):
		assert_float(b.get_combined_minimum_size().y).override_failure_message(b.name).is_greater_equal(minimum)
	(colours.get_child(5) as Button).pressed.emit()
	_press(screen, "Next")
	await runner.simulate_frames(1)
	# Step 3: name → confirm.
	assert_str(String(model.current_step())).is_equal(String(CharacterCreationModel.STEP_NAME))
	var chips: Node = screen.find_child("NicknameChoiceChips", true, false)
	assert_int(chips.get_child_count()).is_equal(6)
	assert_bool((screen.find_child("Confirm", true, false) as Button).disabled).is_true()
	(chips.get_child(2) as Button).pressed.emit()
	await runner.simulate_frames(1)
	assert_bool((screen.find_child("Confirm", true, false) as Button).visible).is_true()
	assert_bool((screen.find_child("Confirm", true, false) as Button).disabled).is_false()
	for b: Button in _visible_buttons(screen):
		assert_float(b.get_combined_minimum_size().y).override_failure_message(b.name).is_greater_equal(minimum)
	h.http.reply(ProfileFixtures.ROUTE_AVATAR, 200, model.draft.to_dict())
	h.http.reply(ProfileFixtures.ROUTE_PATCH_PROFILE, 200, ProfileFixtures.profile_after_setup("Mèo Mây 15"))
	_press(screen, "Confirm")
	for i: int in 20:
		await runner.simulate_frames(1)
		if model.state == CharacterCreationModel.State.DONE:
			break
	assert_int(model.state).is_equal(CharacterCreationModel.State.DONE)
	assert_str(String((h.http.last(ProfileFixtures.ROUTE_AVATAR).body["hair"] as Dictionary)["color"])) \
			.is_equal("hair_col_06")
	assert_str(String(h.http.last(ProfileFixtures.ROUTE_PATCH_PROFILE).body["nickname_option_id"])) \
			.is_equal("07000000-0000-4000-8000-0000000000a3")


## F070-AC-06: every choice redraws the preview at once and sends nothing.
func test_every_choice_redraws_the_preview_without_a_request() -> void:
	var r: Array = await _open("g6_7")
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var screen: Control = r[1] as Control
	var rig: AvatarBuilder = _rig(screen)
	var before: int = h.http.requests.size()
	var slowest_usec: int = 0
	var presses: Array[Button] = []
	for b: Node in screen.find_child("PresetGrid", true, false).get_children():
		presses.append(b as Button)
	_press(screen, "Next")
	await runner.simulate_frames(1)
	for list: String in ["BodyTypes", "SkinTones", "HairColors"]:
		for b: Node in screen.find_child(list, true, false).get_children():
			presses.append(b as Button)
	for arrow: String in ["HairNext", "HairPrev", "OutfitNext", "OutfitPrev"]:
		presses.append(screen.find_child(arrow, true, false) as Button)
	var seen: Dictionary = {}
	slowest_usec = 1 << 62
	var passes: PackedInt64Array = []
	for pass_no: int in PREVIEW_PASSES:
		var pass_slowest: int = 0
		for b: Button in presses:
			var started: int = Time.get_ticks_usec()
			b.pressed.emit()
			pass_slowest = maxi(pass_slowest, Time.get_ticks_usec() - started)
			seen[JSON.stringify(rig.config.to_dict())] = true
			assert_object(rig.config).is_same(_model(screen).draft)
		passes.append(pass_slowest)
		slowest_usec = mini(slowest_usec, pass_slowest)
		await runner.simulate_frames(1)
	prints("avatar preview slowest choice per pass (us):", passes)
	assert_int(h.http.requests.size()).is_equal(before)
	assert_int(seen.size()).is_greater(10)
	assert_int(slowest_usec).override_failure_message("preview update %d us (passes %s)" % [slowest_usec, passes]) \
			.is_less(int(PREVIEW_BUDGET_USEC * BenchmarkScript.TOLERANCE))
	var tone: String = String(CharacterCatalogModel.codes_of(_model(screen).inventory.catalog.skin_tones)[5])
	(screen.find_child("SkinTones", true, false).get_child(5) as Button).pressed.emit()
	assert_object(rig.skin_color()).is_equal(ItemVisual.appearance_color(tone))


func test_catalog_names_label_the_choices() -> void:
	var r: Array = await _open("g4_5")
	var screen: Control = r[1] as Control
	var presets: Node = screen.find_child("PresetGrid", true, false)
	assert_int(presets.get_child_count()).is_equal(6)
	assert_str((presets.get_child(0) as Button).text).is_equal("Nhà thám hiểm")
	Settings.set_locale("en-US", false)
	EventBus.locale_changed.emit("en-US")
	assert_str((presets.get_child(0) as Button).text).is_equal("Explorer")
	var swatch: Button = screen.find_child("SkinTones", true, false).get_child(0) as Button
	assert_str(swatch.tooltip_text).is_equal("Light")


## F070-AC-09 (client part): "Gợi ý khác" and 409 NICKNAME_UNAVAILABLE refresh the chips.
func test_more_names_and_taken_name_refresh_the_chips() -> void:
	var r: Array = await _open("g1_3")
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var screen: Control = r[1] as Control
	var model: CharacterCreationModel = _model(screen)
	_press(screen, "Next")
	_press(screen, "Next")
	await runner.simulate_frames(1)
	var chips: Node = screen.find_child("NicknameChoiceChips", true, false)
	assert_str((chips.get_child(0) as Button).text).is_equal("Cú Mèo Chăm Chỉ 27")
	h.http.reply(ProfileFixtures.ROUTE_SUGGESTIONS, 200, ProfileFixtures.suggestions_more())
	_press(screen, "MoreNames")
	await runner.simulate_frames(4)
	assert_str((chips.get_child(0) as Button).text).is_equal("Thỏ Ngọc Siêng Năng 11")
	assert_str(h.http.last(ProfileFixtures.ROUTE_SUGGESTIONS).query).is_equal("count=6")
	(chips.get_child(0) as Button).pressed.emit()
	h.http.reply(ProfileFixtures.ROUTE_AVATAR, 200, model.draft.to_dict())
	h.http.reply(ProfileFixtures.ROUTE_PATCH_PROFILE, 409, AuthHarness.problem(409, "NICKNAME_UNAVAILABLE"))
	h.http.reply(ProfileFixtures.ROUTE_SUGGESTIONS, 200, ProfileFixtures.suggestions())
	_press(screen, "Confirm")
	for i: int in 20:
		await runner.simulate_frames(1)
		if model.state == CharacterCreationModel.State.READY:
			break
	await runner.simulate_frames(1)
	assert_str((chips.get_child(0) as Button).text).is_equal("Cú Mèo Chăm Chỉ 27")
	assert_str((screen.find_child("StatusLabel", true, false) as Label).text).is_equal(tr("ui.avatar.nickname_taken"))
	assert_bool((screen.find_child("Confirm", true, false) as Button).disabled).is_true()


func test_save_error_keeps_choices_and_offers_retry() -> void:
	var r: Array = await _open("g1_3")
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var screen: Control = r[1] as Control
	var model: CharacterCreationModel = _model(screen)
	_press(screen, "Next")
	_press(screen, "Next")
	(screen.find_child("NicknameChoiceChips", true, false).get_child(0) as Button).pressed.emit()
	h.http.reply(ProfileFixtures.ROUTE_AVATAR, 422, AuthHarness.problem(422, "ITEM_NOT_OWNED"))
	_press(screen, "Confirm")
	for i: int in 20:
		await runner.simulate_frames(1)
		if model.state == CharacterCreationModel.State.ERROR:
			break
	await runner.simulate_frames(1)
	assert_str((screen.find_child("StatusLabel", true, false) as Label).text).is_equal(L10n.error_text("ITEM_NOT_OWNED"))
	var retry: Button = screen.find_child("Retry", true, false) as Button
	assert_bool(retry.visible).is_true()
	retry.pressed.emit()
	await runner.simulate_frames(1)
	assert_int(model.state).is_equal(CharacterCreationModel.State.READY)
	assert_str(model.selected_option_id).is_not_empty()
