extends GdUnitTestSuite
## F070 ProfileService (T018, T025; F070-AC-01 client part, F070-AC-09 client part, F070-AC-10, R14, R16): reads
## `getGameProfile`, asks `suggestNicknames?count=6`, chooses a nickname only by `nickname_option_id` (JSON Merge
## Patch, no free-text field), refetches suggestions by itself on `409 NICKNAME_UNAVAILABLE`, saves the avatar with
## `updateAvatar` and the outfits with `listAvatarPresets` / `saveAvatarPreset`. Runs on FakeHttp (AuthHarness).

var h: AuthHarness = null
var service: ProfileService = null


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	service = ProfileService.new(h.api)


func after_test() -> void:
	h.dispose()


func _config() -> AvatarConfigModel:
	return AvatarConfigModel.from_dict(ProfileFixtures.profile_ready()["avatar_config"])


func test_profile_of_a_new_student_requires_setup() -> void:
	h.http.clear(ProfileFixtures.ROUTE_PROFILE)
	h.http.reply(ProfileFixtures.ROUTE_PROFILE, 200, ProfileFixtures.profile_new())
	var loaded: Array[GameProfileModel] = []
	service.profile_loaded.connect(func(p: GameProfileModel) -> void: loaded.append(p))
	var result: ApiResult = await service.fetch_profile()
	assert_bool(result.ok).is_true()
	assert_bool(service.needs_setup()).is_true()
	assert_object(service.profile.avatar_config).is_null()
	assert_str(service.profile.nickname).is_empty()
	assert_int(loaded.size()).is_equal(1)
	var rec: FakeHttp.Recorded = h.http.last(ProfileFixtures.ROUTE_PROFILE)
	assert_str(String(rec.headers.get("x-tenant-id", ""))).is_equal(AuthHarness.TENANT_ID)


func test_profile_with_avatar_and_nickname_needs_no_setup() -> void:
	var result: ApiResult = await service.fetch_profile()
	assert_bool(result.ok).is_true()
	assert_bool(service.needs_setup()).is_false()
	assert_str(service.profile.avatar_config.outfit_item_id).is_equal("outfit_casual_01")
	assert_str(service.profile.nickname).is_equal("Cú Mèo Chăm Chỉ 27")


func test_failed_profile_read_keeps_setup_required() -> void:
	h.http.clear(ProfileFixtures.ROUTE_PROFILE)
	h.http.reply(ProfileFixtures.ROUTE_PROFILE, 500, AuthHarness.problem(500, "INTERNAL"))
	var result: ApiResult = await service.fetch_profile()
	assert_bool(result.ok).is_false()
	assert_bool(service.needs_setup()).is_true()


func test_suggestions_ask_for_six_names() -> void:
	h.http.reply(ProfileFixtures.ROUTE_SUGGESTIONS, 200, ProfileFixtures.suggestions())
	var result: ApiResult = await service.suggest_nicknames()
	assert_bool(result.ok).is_true()
	assert_int(service.suggestions.size()).is_equal(6)
	assert_str(service.suggestions[0].option_id).is_equal("07000000-0000-4000-8000-0000000000a1")
	assert_str(h.http.last(ProfileFixtures.ROUTE_SUGGESTIONS).query).is_equal("count=6")


## F070-AC-10 / R16: the only nickname field ever sent is `nickname_option_id` (merge patch).
func test_choosing_a_nickname_sends_only_the_option_id_as_merge_patch() -> void:
	h.http.reply(ProfileFixtures.ROUTE_PATCH_PROFILE, 200, ProfileFixtures.profile_after_setup("Mèo Mây 15"))
	var chosen: Array[String] = []
	service.nickname_chosen.connect(func(n: String) -> void: chosen.append(n))
	var result: ApiResult = await service.choose_nickname("07000000-0000-4000-8000-0000000000a3")
	assert_bool(result.ok).is_true()
	var rec: FakeHttp.Recorded = h.http.last(ProfileFixtures.ROUTE_PATCH_PROFILE)
	assert_str(String(rec.headers["content-type"])).is_equal("application/merge-patch+json")
	assert_array(rec.body.keys()).contains_exactly(["nickname_option_id"])
	assert_str(String(rec.body["nickname_option_id"])).is_equal("07000000-0000-4000-8000-0000000000a3")
	assert_str(service.profile.nickname).is_equal("Mèo Mây 15")
	assert_array(chosen).contains_exactly(["Mèo Mây 15"])


## F070-AC-09 (client part): `409 NICKNAME_UNAVAILABLE` → new suggestions are fetched automatically.
func test_nickname_unavailable_fetches_new_suggestions_automatically() -> void:
	h.http.reply(ProfileFixtures.ROUTE_SUGGESTIONS, 200, ProfileFixtures.suggestions())
	await service.suggest_nicknames()
	h.http.reply(ProfileFixtures.ROUTE_SUGGESTIONS, 200, ProfileFixtures.suggestions_more())
	h.http.reply(ProfileFixtures.ROUTE_PATCH_PROFILE, 409, AuthHarness.problem(409, "NICKNAME_UNAVAILABLE"))
	var result: ApiResult = await service.choose_nickname("07000000-0000-4000-8000-0000000000a1")
	assert_bool(result.ok).is_false()
	assert_str(result.code).is_equal("NICKNAME_UNAVAILABLE")
	assert_int(h.http.count(ProfileFixtures.ROUTE_SUGGESTIONS)).is_equal(2)
	assert_str(service.suggestions[0].option_id).is_equal("07000000-0000-4000-8000-0000000000b1")


func test_nickname_change_limit_does_not_refetch_suggestions() -> void:
	h.http.reply(ProfileFixtures.ROUTE_PATCH_PROFILE, 409, AuthHarness.problem(409, "NICKNAME_CHANGE_LIMIT"))
	var result: ApiResult = await service.choose_nickname("07000000-0000-4000-8000-0000000000a1")
	assert_str(result.code).is_equal("NICKNAME_CHANGE_LIMIT")
	assert_int(h.http.count(ProfileFixtures.ROUTE_SUGGESTIONS)).is_equal(0)


func test_save_avatar_puts_the_avatar_config() -> void:
	var config: AvatarConfigModel = _config()
	h.http.reply(ProfileFixtures.ROUTE_AVATAR, 200, config.to_dict())
	var saved: Array[AvatarConfigModel] = []
	service.avatar_saved.connect(func(c: AvatarConfigModel) -> void: saved.append(c))
	var result: ApiResult = await service.save_avatar(config)
	assert_bool(result.ok).is_true()
	var rec: FakeHttp.Recorded = h.http.last(ProfileFixtures.ROUTE_AVATAR)
	assert_str(String(rec.headers["content-type"])).is_equal("application/json")
	assert_int(int(rec.body["schema_version"])).is_equal(1)
	assert_str(String((rec.body["hair"] as Dictionary)["color"])).is_equal("hair_col_03")
	assert_array(rec.body["accessories"] as Array).contains_exactly([{"slot": "face", "item_id": "acc_glasses_round"}])
	assert_int(saved.size()).is_equal(1)


func test_item_not_owned_is_reported_with_its_code() -> void:
	h.http.reply(ProfileFixtures.ROUTE_AVATAR, 422, AuthHarness.problem(422, "ITEM_NOT_OWNED"))
	var result: ApiResult = await service.save_avatar(_config())
	assert_bool(result.ok).is_false()
	assert_str(result.code).is_equal("ITEM_NOT_OWNED")
	assert_str(L10n.error_text(result.code)).is_not_equal(tr(L10n.ERROR_GENERIC))


func test_presets_are_listed_and_saved_by_slot() -> void:
	var slots: Array[int] = [1, 3]
	h.http.reply(ProfileFixtures.ROUTE_PRESETS, 200, ProfileFixtures.preset_list(slots))
	assert_bool((await service.list_presets()).ok).is_true()
	assert_int(service.presets.size()).is_equal(2)
	assert_int(service.presets[1].slot).is_equal(3)
	var two: Array[int] = [2]
	var preset_body: Dictionary = (ProfileFixtures.preset_list(two)["data"] as Array)[0] as Dictionary
	h.http.reply(ProfileFixtures.ROUTE_SAVE_PRESET, 200, preset_body)
	assert_bool((await service.save_preset(2, _config(), "Bộ 2")).ok).is_true()
	var rec: FakeHttp.Recorded = h.http.last(ProfileFixtures.ROUTE_SAVE_PRESET)
	assert_str(rec.path).is_equal("/game/avatar-presets/2")
	assert_str(String(rec.body["name"])).is_equal("Bộ 2")
	assert_bool(rec.body["avatar_config"] is Dictionary).is_true()
	assert_int(service.presets.size()).is_equal(3)


func test_preset_slot_outside_one_to_three_is_not_sent() -> void:
	var result: ApiResult = await service.save_preset(4, _config())
	assert_bool(result.ok).is_false()
	assert_int(h.http.count(ProfileFixtures.ROUTE_SAVE_PRESET)).is_equal(0)


## R14: the nickname never reaches the log or the telemetry queue.
func test_nickname_is_never_logged() -> void:
	Log.clear()
	h.http.reply(ProfileFixtures.ROUTE_PATCH_PROFILE, 200, ProfileFixtures.profile_after_setup("Rồng Xanh Vui Vẻ 08"))
	await service.choose_nickname("07000000-0000-4000-8000-0000000000a2")
	await service.fetch_profile()
	var text: String = JSON.stringify(Log.entries()) + JSON.stringify(AppState.pending_telemetry)
	assert_bool(text.contains("Rồng Xanh")).is_false()
	assert_bool(text.contains("Cú Mèo")).is_false()


func test_clear_drops_the_session_data() -> void:
	await service.fetch_profile()
	service.clear()
	assert_object(service.profile).is_null()
	assert_bool(service.needs_setup()).is_true()
	assert_int(service.suggestions.size()).is_equal(0)
