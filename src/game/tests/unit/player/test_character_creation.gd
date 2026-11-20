extends GdUnitTestSuite
## F070-AC-05, F070-AC-06, F070-AC-09 (client part) — T013, T025 (STUDENT_GAME_UX_CORE S02, CHARACTER_SYSTEM §5.0):
## the S02 flow model. `g1_3` completes in ≤ 3 steps (preset → hair colour → name, then confirm) with choices only (no
## typing); other bands: preset → customise (body type, skin tone, hair, hair colour, outfit) → name → preview →
## confirm, with "Bỏ qua tùy chỉnh". Every choice updates the draft locally without calling ApiClient. The name step
## lists `suggestNicknames?count=6`; "Gợi ý khác" asks again; confirm = `updateAvatar` then `updateGameProfile`
## (`nickname_option_id`), and a `409 NICKNAME_UNAVAILABLE` brings new suggestions and returns to the name step.

var h: AuthHarness = null
var profiles: ProfileService = null
var inventory: InventoryService = null


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	ProfileFixtures.script_first_time(h.http)
	profiles = ProfileService.new(h.api)
	inventory = InventoryService.new(h.api)


func after_test() -> void:
	h.dispose()


func _model(band: String) -> CharacterCreationModel:
	var model: CharacterCreationModel = CharacterCreationModel.new(profiles, inventory, band)
	assert_bool(await model.start()).is_true()
	return model


## F070-AC-05: `g1_3` — preset → hair colour → name (3 steps) then confirm; nothing to type.
func test_g1_3_completes_in_three_steps_without_typing() -> void:
	var model: CharacterCreationModel = await _model("g1_3")
	assert_array(model.steps).contains_exactly([CharacterCreationModel.STEP_PRESET,
			CharacterCreationModel.STEP_HAIR_COLOR, CharacterCreationModel.STEP_NAME])
	assert_int(model.steps.size()).is_less_equal(3)
	assert_bool(model.can_skip_customize()).is_false()
	model.select_preset("preset_artist")
	assert_bool(model.next()).is_true()
	model.set_hair_color("hair_col_06")
	assert_bool(model.next()).is_true()
	assert_str(String(model.current_step())).is_equal(String(CharacterCreationModel.STEP_NAME))
	assert_bool(model.is_last_step()).is_true()
	assert_bool(model.can_confirm()).is_false()
	model.choose_nickname(profiles.suggestions[2].option_id)
	assert_bool(model.can_confirm()).is_true()
	h.http.reply(ProfileFixtures.ROUTE_AVATAR, 200, model.draft.to_dict())
	h.http.reply(ProfileFixtures.ROUTE_PATCH_PROFILE, 200, ProfileFixtures.profile_after_setup("Mèo Mây 15"))
	var done: Array[bool] = []
	model.creation_completed.connect(func() -> void: done.append(true))
	assert_bool(await model.confirm()).is_true()
	assert_int(done.size()).is_equal(1)
	var avatar: FakeHttp.Recorded = h.http.last(ProfileFixtures.ROUTE_AVATAR)
	assert_str(String(avatar.body["body_type"])).is_equal("kid_b")
	assert_str(String((avatar.body["hair"] as Dictionary)["color"])).is_equal("hair_col_06")
	assert_array(avatar.body["emote_slots"] as Array).is_not_empty()
	var patch: FakeHttp.Recorded = h.http.last(ProfileFixtures.ROUTE_PATCH_PROFILE)
	assert_array(patch.body.keys()).contains_exactly(["nickname_option_id"])
	assert_str(String(patch.body["nickname_option_id"])).is_equal("07000000-0000-4000-8000-0000000000a3")
	# Avatar first, then the nickname.
	var order: PackedStringArray = []
	for rec: FakeHttp.Recorded in h.http.requests:
		if rec.method in ["PUT", "PATCH"]:
			order.append(rec.method)
	assert_array(Array(order)).contains_exactly(["PUT", "PATCH"])


func test_other_bands_have_the_full_flow_with_skip() -> void:
	for band: String in ["g4_5", "g6_7", "g8_9"]:
		var model: CharacterCreationModel = await _model(band)
		assert_array(model.steps).override_failure_message(band).contains_exactly([CharacterCreationModel.STEP_PRESET,
				CharacterCreationModel.STEP_CUSTOMIZE, CharacterCreationModel.STEP_NAME,
				CharacterCreationModel.STEP_PREVIEW])
		assert_bool(model.can_skip_customize()).is_true()
		model.skip_customize()
		assert_str(String(model.current_step())).is_equal(String(CharacterCreationModel.STEP_NAME))
		assert_bool(model.next()).override_failure_message("name needs a choice").is_false()
		model.choose_nickname(profiles.suggestions[0].option_id)
		assert_bool(model.next()).is_true()
		assert_str(String(model.current_step())).is_equal(String(CharacterCreationModel.STEP_PREVIEW))
		assert_bool(model.is_last_step()).is_true()
		model.back()
		assert_str(String(model.current_step())).is_equal(String(CharacterCreationModel.STEP_NAME))


## F070-AC-06: every choice changes the draft locally (signal) and sends no request.
func test_every_choice_updates_the_preview_without_any_request() -> void:
	var model: CharacterCreationModel = await _model("g6_7")
	var drafts: Array[AvatarConfigModel] = []
	model.draft_changed.connect(func(c: AvatarConfigModel) -> void: drafts.append(c))
	var before: int = h.http.requests.size()
	for code: String in inventory.catalog.preset_codes():
		model.select_preset(code)
	assert_str(model.draft.outfit_item_id).is_equal("outfit_rain_01")
	model.set_body_type("kid_c")
	model.set_skin_tone("tone_06")
	model.cycle_hair(1)
	model.cycle_hair(-1)
	model.cycle_hair(-1)
	model.set_hair_color("hair_col_08")
	model.cycle_outfit(1)
	model.choose_nickname(profiles.suggestions[1].option_id)
	assert_int(h.http.requests.size()).is_equal(before)
	assert_int(drafts.size()).is_equal(inventory.catalog.presets.size() + 7)
	assert_str(model.draft.body_type).is_equal("kid_c")
	assert_str(model.draft.skin_tone).is_equal("tone_06")
	assert_str(model.draft.hair_color).is_equal("hair_col_08")
	assert_str(model.draft.hair_item_id).is_equal(String(inventory.items_for_slot("hair")[0]))
	# Last preset wears outfit_rain_01; one step forward in the catalog order is outfit_star_01.
	assert_str(model.draft.outfit_item_id).is_equal("outfit_star_01")


func test_codes_outside_the_catalog_are_ignored() -> void:
	var model: CharacterCreationModel = await _model("g4_5")
	var body: String = model.draft.body_type
	model.set_body_type("kid_zz")
	model.set_skin_tone("tone_99")
	model.select_preset("preset_unknown")
	assert_str(model.draft.body_type).is_equal(body)
	assert_str(model.draft.skin_tone).is_not_equal("tone_99")


## F070-AC-09 (client part): "Gợi ý khác" asks for 6 new names; 409 NICKNAME_UNAVAILABLE on confirm brings new
## suggestions and returns to the name step with a light notice; the avatar is not sent again.
func test_nickname_taken_on_confirm_refreshes_suggestions_and_returns_to_the_name_step() -> void:
	var model: CharacterCreationModel = await _model("g1_3")
	h.http.reply(ProfileFixtures.ROUTE_SUGGESTIONS, 200, ProfileFixtures.suggestions_more())
	await model.refresh_suggestions()
	assert_int(h.http.count(ProfileFixtures.ROUTE_SUGGESTIONS)).is_equal(2)
	assert_str(h.http.last(ProfileFixtures.ROUTE_SUGGESTIONS).query).is_equal("count=6")
	assert_str(profiles.suggestions[0].option_id).is_equal("07000000-0000-4000-8000-0000000000b1")
	model.next()
	model.next()
	model.choose_nickname(profiles.suggestions[0].option_id)
	h.http.reply(ProfileFixtures.ROUTE_AVATAR, 200, model.draft.to_dict())
	h.http.reply(ProfileFixtures.ROUTE_PATCH_PROFILE, 409, AuthHarness.problem(409, "NICKNAME_UNAVAILABLE"))
	h.http.reply(ProfileFixtures.ROUTE_SUGGESTIONS, 200, ProfileFixtures.suggestions())
	var notices: Array[String] = []
	model.notice_shown.connect(func(key: String) -> void: notices.append(key))
	assert_bool(await model.confirm()).is_false()
	assert_int(h.http.count(ProfileFixtures.ROUTE_SUGGESTIONS)).is_equal(3)
	assert_str(String(model.current_step())).is_equal(String(CharacterCreationModel.STEP_NAME))
	assert_str(model.selected_option_id).is_empty()
	assert_array(notices).contains(["ui.avatar.nickname_taken"])
	assert_int(model.state).is_equal(CharacterCreationModel.State.READY)
	model.choose_nickname(profiles.suggestions[1].option_id)
	h.http.reply(ProfileFixtures.ROUTE_PATCH_PROFILE, 200, ProfileFixtures.profile_after_setup("Rồng Xanh Vui Vẻ 08"))
	assert_bool(await model.confirm()).is_true()
	assert_int(h.http.count(ProfileFixtures.ROUTE_AVATAR)).is_equal(1)


func test_failed_avatar_save_keeps_the_choices_and_reports_the_code() -> void:
	var model: CharacterCreationModel = await _model("g1_3")
	model.select_preset("preset_sporty")
	model.next()
	model.next()
	model.choose_nickname(profiles.suggestions[0].option_id)
	h.http.reply(ProfileFixtures.ROUTE_AVATAR, 422, AuthHarness.problem(422, "ITEM_NOT_OWNED"))
	assert_bool(await model.confirm()).is_false()
	assert_int(model.state).is_equal(CharacterCreationModel.State.ERROR)
	assert_str(model.error_code).is_equal("ITEM_NOT_OWNED")
	assert_str(model.draft.outfit_item_id).is_equal("outfit_sport_01")
	assert_str(model.selected_option_id).is_equal(profiles.suggestions[0].option_id)
	assert_int(h.http.count(ProfileFixtures.ROUTE_PATCH_PROFILE)).is_equal(0)


func test_catalog_change_on_save_reloads_the_catalog() -> void:
	var model: CharacterCreationModel = await _model("g1_3")
	model.next()
	model.next()
	model.choose_nickname(profiles.suggestions[0].option_id)
	h.http.reply(ProfileFixtures.ROUTE_AVATAR, 422, AuthHarness.problem(422, "VALIDATION_FAILED"))
	assert_bool(await model.confirm()).is_false()
	assert_int(h.http.count(ProfileFixtures.ROUTE_CATALOG)).is_equal(2)
	assert_str(String(h.http.last(ProfileFixtures.ROUTE_CATALOG).headers.get("if-none-match", ""))) \
			.is_equal(ProfileFixtures.CATALOG_ETAG)


func test_start_failure_is_an_error_state() -> void:
	h.http.clear(ProfileFixtures.ROUTE_CATALOG)
	h.http.reply(ProfileFixtures.ROUTE_CATALOG, 500, AuthHarness.problem(500, "INTERNAL"))
	var model: CharacterCreationModel = CharacterCreationModel.new(profiles, inventory, "g1_3")
	assert_bool(await model.start()).is_false()
	assert_int(model.state).is_equal(CharacterCreationModel.State.ERROR)


func test_profile_already_set_up_completes_at_once() -> void:
	ProfileFixtures.script_ready(h.http)
	var model: CharacterCreationModel = CharacterCreationModel.new(profiles, inventory, "g1_3")
	var done: Array[bool] = []
	model.creation_completed.connect(func() -> void: done.append(true))
	await model.start()
	assert_int(model.state).is_equal(CharacterCreationModel.State.DONE)
	assert_int(done.size()).is_equal(1)


func test_voice_hint_key_per_step() -> void:
	var model: CharacterCreationModel = await _model("g1_3")
	assert_str(model.voice_key()).is_equal("ui.avatar.voice.preset")
	model.next()
	assert_str(model.voice_key()).is_equal("ui.avatar.voice.hair_color")
	model.next()
	assert_str(model.voice_key()).is_equal("ui.avatar.voice.name")
