extends GdUnitTestSuite
## F070 US3 Wardrobe (T030; CHARACTER_SYSTEM §5.1, §6): CustomizationStudio opens as the `wardrobe` screen, lists the
## items the student may wear by tab (`g1_3`: one "Trang phục" tab for outfit + hair + accessories), redraws the
## preview locally, saves with `updateAvatar`, shows the server's 422 `ITEM_NOT_OWNED` / `EQUIP_SLOT_INVALID` as a
## friendly localized message (draft kept), and saves/wears outfits in slots 1…3 (`listAvatarPresets`,
## `saveAvatarPreset`). Equipped emotes are capped at `emote_wheel_slots`.

const SCENE: String = "res://scenes/character/CustomizationStudio.tscn"

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
	ProfileFixtures.script_ready(h.http)
	var one: Array[int] = [1]
	h.http.reply(ProfileFixtures.ROUTE_PRESETS, 200, ProfileFixtures.preset_list(one))
	AppState.services = ServiceLocator.new(h.api, AppState, h.clock)


func after_test() -> void:
	h.dispose()
	AppState.services = _services
	AppState.set_grade_band(_band)
	Settings.set_locale(_locale, false)


func _open(band: String) -> Array:
	AppState.set_grade_band(band)
	var runner: GdUnitSceneRunner = scene_runner(SCENE)
	var studio: Control = runner.scene() as Control
	studio.call("open", {})
	for i: int in 40:
		await runner.simulate_frames(1)
		var model: WardrobeModel = studio.get("model") as WardrobeModel
		if model != null and model.state != WardrobeModel.State.LOADING:
			break
	return [runner, studio]


func _model(studio: Control) -> WardrobeModel:
	return studio.get("model") as WardrobeModel


func _tab(studio: Control, tab: StringName) -> Button:
	return studio.find_child("Tab_%s" % tab, true, false) as Button


func _item(studio: Control, item_id: String) -> Button:
	return studio.find_child("Item_%s" % item_id, true, false) as Button


func _wait_state(runner: GdUnitSceneRunner, model: WardrobeModel, wanted: int) -> void:
	for i: int in 20:
		await runner.simulate_frames(1)
		if model.state == wanted:
			return


func test_wardrobe_is_a_registered_screen() -> void:
	assert_bool(UiScreenRegistry.has(&"wardrobe")).is_true()
	assert_str(UiScreenRegistry.path_for(&"wardrobe")).is_equal(SCENE)


func test_g1_3_has_one_clothes_tab_with_outfit_hair_and_accessories() -> void:
	var r: Array = await _open("g1_3")
	var studio: Control = r[1] as Control
	var model: WardrobeModel = _model(studio)
	assert_int(model.state).is_equal(WardrobeModel.State.READY)
	assert_array(model.tabs()).contains_exactly([WardrobeModel.TAB_LOOK, WardrobeModel.TAB_CLOTHES,
			WardrobeModel.TAB_EMOTE])
	assert_object(_tab(studio, WardrobeModel.TAB_HAIR)).is_null()
	var clothes: Button = _tab(studio, WardrobeModel.TAB_CLOTHES)
	assert_str(clothes.text).is_equal(tr("ui.wardrobe.tab.clothes"))
	clothes.pressed.emit()
	for item_id: String in ["outfit_sport_01", "hair_spiky_01", "acc_cap_star"]:
		assert_object(_item(studio, item_id)).override_failure_message(item_id).is_not_null()
	var minimum: float = float(UiTokens.DENSITY_BUTTON_MIN_HEIGHT["g1_3"])
	assert_float(_item(studio, "outfit_sport_01").get_combined_minimum_size().y).is_greater_equal(minimum)


func test_other_bands_have_separate_tabs() -> void:
	var r: Array = await _open("g6_7")
	var model: WardrobeModel = _model(r[1] as Control)
	assert_array(model.tabs()).contains_exactly([WardrobeModel.TAB_LOOK, WardrobeModel.TAB_HAIR,
			WardrobeModel.TAB_OUTFIT, WardrobeModel.TAB_ACCESSORY, WardrobeModel.TAB_EMOTE])


func test_equipping_redraws_locally_then_save_puts_the_avatar() -> void:
	var r: Array = await _open("g4_5")
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var studio: Control = r[1] as Control
	var model: WardrobeModel = _model(studio)
	var rig: AvatarBuilder = studio.find_child("AvatarRig", true, false) as AvatarBuilder
	var before: int = h.http.requests.size()
	_tab(studio, WardrobeModel.TAB_OUTFIT).pressed.emit()
	_item(studio, "outfit_star_01").pressed.emit()
	_tab(studio, WardrobeModel.TAB_ACCESSORY).pressed.emit()
	_item(studio, "acc_cap_star").pressed.emit()
	_item(studio, "acc_glasses_round").pressed.emit()
	assert_int(h.http.requests.size()).is_equal(before)
	assert_str(String(rig.shown["outfit"])).is_equal("outfit_star_01")
	assert_str(String(rig.shown["head"])).is_equal("acc_cap_star")
	assert_bool(rig.shown.has("face")).is_false()
	assert_bool(_item(studio, "acc_cap_star").button_pressed).is_true()
	assert_bool(model.has_changes()).is_true()
	h.http.reply(ProfileFixtures.ROUTE_AVATAR, 200, model.draft.to_dict())
	(studio.find_child("Save", true, false) as Button).pressed.emit()
	await _wait_state(runner, model, WardrobeModel.State.READY)
	var rec: FakeHttp.Recorded = h.http.last(ProfileFixtures.ROUTE_AVATAR)
	assert_str(String((rec.body["outfit"] as Dictionary)["item_id"])).is_equal("outfit_star_01")
	assert_array(rec.body["accessories"] as Array).contains_exactly([{"slot": "head", "item_id": "acc_cap_star"}])
	assert_bool(model.has_changes()).is_false()
	await runner.simulate_frames(1)
	assert_str((studio.find_child("StatusLabel", true, false) as Label).text).is_equal(tr("ui.wardrobe.saved"))


func test_server_refusals_are_shown_as_friendly_messages() -> void:
	var r: Array = await _open("g4_5")
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var studio: Control = r[1] as Control
	var model: WardrobeModel = _model(studio)
	_tab(studio, WardrobeModel.TAB_OUTFIT).pressed.emit()
	_item(studio, "outfit_rain_01").pressed.emit()
	for code: String in ["ITEM_NOT_OWNED", "EQUIP_SLOT_INVALID"]:
		h.http.reply(ProfileFixtures.ROUTE_AVATAR, 422, AuthHarness.problem(422, code))
		(studio.find_child("Save", true, false) as Button).pressed.emit()
		await _wait_state(runner, model, WardrobeModel.State.ERROR)
		await runner.simulate_frames(1)
		var text: String = (studio.find_child("StatusLabel", true, false) as Label).text
		assert_str(text).override_failure_message(code).is_equal(L10n.error_text(code))
		assert_str(text).is_not_equal(tr(L10n.ERROR_GENERIC))
		assert_bool(text.contains(code)).is_false()
		assert_str(model.draft.outfit_item_id).is_equal("outfit_rain_01")
		(studio.find_child("Retry", true, false) as Button).pressed.emit()
		assert_int(model.state).is_equal(WardrobeModel.State.READY)


func test_outfits_are_saved_and_worn_by_slot() -> void:
	var r: Array = await _open("g4_5")
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var studio: Control = r[1] as Control
	var model: WardrobeModel = _model(studio)
	assert_int(h.http.count(ProfileFixtures.ROUTE_PRESETS)).is_equal(1)
	assert_bool((studio.find_child("WearPreset1", true, false) as Button).disabled).is_false()
	assert_bool((studio.find_child("WearPreset2", true, false) as Button).disabled).is_true()
	_tab(studio, WardrobeModel.TAB_HAIR).pressed.emit()
	_item(studio, "hair_braids_01").pressed.emit()
	var two: Array[int] = [2]
	var saved: Dictionary = (ProfileFixtures.preset_list(two)["data"] as Array)[0] as Dictionary
	(saved["avatar_config"] as Dictionary)["hair"] = {"item_id": "hair_braids_01", "color": "hair_col_03"}
	h.http.reply(ProfileFixtures.ROUTE_SAVE_PRESET, 200, saved)
	(studio.find_child("SavePreset2", true, false) as Button).pressed.emit()
	await _wait_state(runner, model, WardrobeModel.State.READY)
	await runner.simulate_frames(1)
	var rec: FakeHttp.Recorded = h.http.last(ProfileFixtures.ROUTE_SAVE_PRESET)
	assert_str(rec.path).is_equal("/game/avatar-presets/2")
	assert_str(String(((rec.body["avatar_config"] as Dictionary)["hair"] as Dictionary)["item_id"])) \
			.is_equal("hair_braids_01")
	assert_bool((studio.find_child("WearPreset2", true, false) as Button).disabled).is_false()
	(studio.find_child("WearPreset1", true, false) as Button).pressed.emit()
	assert_str(model.draft.hair_item_id).is_equal("hair_bob_01")


func test_emotes_toggle_and_are_capped() -> void:
	var r: Array = await _open("g4_5")
	var studio: Control = r[1] as Control
	var model: WardrobeModel = _model(studio)
	_tab(studio, WardrobeModel.TAB_EMOTE).pressed.emit()
	assert_int(model.draft.emote_slots.size()).is_equal(3)
	_item(studio, "emote_wave").pressed.emit()
	assert_bool("emote_wave" in model.draft.emote_slots).is_false()
	for emote: String in ["emote_wave", "emote_celebrate", "emote_thank", "emote_apologize"]:
		_item(studio, emote).pressed.emit()
	assert_int(model.draft.emote_slots.size()).is_equal(AvatarConfigModel.MAX_EMOTE_SLOTS)
	var notices: Array[String] = []
	model.notice_shown.connect(func(key: String) -> void: notices.append(key))
	model.draft.emote_slots.remove_at(0)
	model.draft.emote_slots.append("emote_extra_07")
	_item(studio, "emote_clap").pressed.emit()
	assert_array(notices).contains([WardrobeModel.NOTICE_EMOTES_FULL])


func test_look_tab_changes_body_and_skin_for_free() -> void:
	var r: Array = await _open("g1_3")
	var studio: Control = r[1] as Control
	var model: WardrobeModel = _model(studio)
	_tab(studio, WardrobeModel.TAB_LOOK).pressed.emit()
	(studio.find_child("Look_kid_d", true, false) as Button).pressed.emit()
	(studio.find_child("Look_tone_01", true, false) as Button).pressed.emit()
	(studio.find_child("Look_hair_col_07", true, false) as Button).pressed.emit()
	assert_str(model.draft.body_type).is_equal("kid_d")
	assert_str(model.draft.skin_tone).is_equal("tone_01")
	assert_str(model.draft.hair_color).is_equal("hair_col_07")


func test_reopening_binds_the_buttons_to_the_new_model() -> void:
	var r: Array = await _open("g4_5")
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var studio: Control = r[1] as Control
	(studio.find_child("Close", true, false) as Button).pressed.emit()
	studio.call("open", {})
	for i: int in 40:
		await runner.simulate_frames(1)
		if _model(studio).state != WardrobeModel.State.LOADING:
			break
	await runner.simulate_frames(1)
	var model: WardrobeModel = _model(studio)
	_tab(studio, WardrobeModel.TAB_HAIR).pressed.emit()
	_item(studio, "hair_spiky_01").pressed.emit()
	assert_str(model.draft.hair_item_id).is_equal("hair_spiky_01")
	(studio.find_child("WearPreset1", true, false) as Button).pressed.emit()
	assert_str(model.draft.hair_item_id).is_equal("hair_bob_01")


func test_close_emits_closed_and_drops_unsaved_changes() -> void:
	var r: Array = await _open("g4_5")
	var studio: Control = r[1] as Control
	var model: WardrobeModel = _model(studio)
	var closed: Array[bool] = []
	studio.connect("closed", func() -> void: closed.append(true))
	_tab(studio, WardrobeModel.TAB_OUTFIT).pressed.emit()
	_item(studio, "outfit_star_01").pressed.emit()
	(studio.find_child("Close", true, false) as Button).pressed.emit()
	assert_int(closed.size()).is_equal(1)
	assert_bool(studio.visible).is_false()
	assert_bool(model.has_changes()).is_false()
