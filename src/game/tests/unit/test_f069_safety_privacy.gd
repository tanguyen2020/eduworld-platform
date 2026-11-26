extends GdUnitTestSuite
## F069 cross-cutting rules (T021, T039, T040; F069-AC-13, AC-28, AC-29, AC-30; AGENTS R1, R14, R19, R23, ADR-0064):
## - no real-money purchase SDK, loot box, trade/gift or promotion countdown anywhere in the client (scan of addons,
##   export presets, F069 scripts and translations);
## - progression/wallet/inventory/challenge live in RAM only (no `user://` in the F069 code) and are dropped when the
##   session ends; F069 scenes never call ApiClient/RealtimeClient;
## - every request carries `X-Tenant-Id`; `GAME_ACCESS_STUDENT_ONLY` goes to the F061 flow, `TENANT_MISMATCH` to the
##   ApiClient auth handler; `422 VALIDATION_FAILED` is a `client_bug` without PII.

const LayerRules := preload("res://tools/lib/layer_rules.gd")
const ToolFiles := preload("res://tools/lib/tool_files.gd")
const F069_DIRS: PackedStringArray = [
	"res://scripts/progression", "res://scripts/inventory", "res://scripts/achievement", "res://scripts/challenge",
	"res://scenes/progression", "res://scenes/inventory", "res://scenes/achievement", "res://scenes/challenge",
]
const F069_FILES: PackedStringArray = [
	"res://scenes/ui/hud/xp_bar.gd", "res://scenes/ui/hud/currency_chip.gd", "res://scenes/ui/hud/game_menu.gd",
	"res://scenes/ui/components/virtual_item_grid.gd", "res://scenes/ui/components/item_thumb.gd",
	"res://scenes/ui/components/rarity_mark.gd", "res://scenes/ui/components/reward_row.gd",
	"res://scenes/ui/components/filter_chips.gd", "res://scenes/ui/components/ui_build.gd",
	"res://scenes/character/avatar_preview_3d.gd", "res://scripts/services/game_api_service.gd",
]
## Words of a real-money / random-paid / pressure feature (code identifiers and translation keys).
const FORBIDDEN_CODE: PackedStringArray = ["iap", "in_app_purchase", "billing", "storekit", "lootbox", "loot_box", "gacha",
	"random_reward", "trade_item", "gift_item", "send_gift", "countdown_offer", "limited_time", "flash_sale", "real_money"]
const FORBIDDEN_TEXT: PackedStringArray = ["vnd", "₫", "usd", "$", "loot", "gacha", "chỉ hôm nay", "only today",
	"mua ngay kẻo lỡ", "hurry", "tặng bạn", "trao đổi"]

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()


func after_test() -> void:
	h.dispose()


static func _files() -> PackedStringArray:
	var out: PackedStringArray = F069_FILES.duplicate()
	for dir: String in F069_DIRS:
		out.append_array(_scan(dir))
	return out


static func _scan(dir: String) -> PackedStringArray:
	var out: PackedStringArray = []
	var d: DirAccess = DirAccess.open(dir)
	if d == null:
		return out
	for f: String in d.get_files():
		if f.ends_with(".gd") or f.ends_with(".tscn"):
			out.append(dir.path_join(f))
	for sub: String in d.get_directories():
		out.append_array(_scan(dir.path_join(sub)))
	return out


func test_no_real_money_no_loot_box_no_trade_no_countdown() -> void:
	var addons: DirAccess = DirAccess.open("res://addons")
	for name: String in addons.get_directories():
		for word: String in ["iap", "billing", "purchase", "storekit", "admob", "ads"]:
			assert_str(name.to_lower()).override_failure_message(name).not_contains(word)
	var presets: String = FileAccess.get_file_as_string("res://export_presets.cfg").to_lower()
	for word: String in ["billing", "iap", "storekit", "com.android.vending.billing"]:
		assert_str(presets).override_failure_message(word).not_contains(word)
	for path: String in _files():
		var text: String = FileAccess.get_file_as_string(path).to_lower()
		for word: String in FORBIDDEN_CODE:
			assert_bool(RegEx.create_from_string("(?<![a-z])%s(?![a-z])" % word).search(text) == null) \
					.override_failure_message("%s: %s" % [path, word]).is_true()
	for locale: String in ["vi-VN", "en-US"]:
		var tr_obj: Translation = TranslationServer.get_translation_object(locale)
		for key: StringName in tr_obj.get_message_list():
			var k: String = String(key)
			if not (k.begins_with("ui.shop.") or k.begins_with("ui.inventory.") or k.begins_with("ui.reward.")
					or k.begins_with("ui.collection.") or k.begins_with("ui.levelup.") or k.begins_with("ui.challenge.")):
				continue
			var message: String = String(tr_obj.get_message(key)).to_lower()
			for word: String in FORBIDDEN_TEXT:
				assert_str(message).override_failure_message("%s %s: %s" % [locale, k, word]).not_contains(word)


func test_data_stays_in_ram_and_scenes_never_call_the_network() -> void:
	for path: String in _files():
		var text: String = FileAccess.get_file_as_string(path)
		var code: PackedStringArray = []
		for line: String in text.split("
"):
			code.append(ToolFiles.strip_comment(line))
		var body: String = "
".join(code)
		assert_str(body).override_failure_message(path).not_contains("user://")
		assert_str(body).override_failure_message(path).not_contains("FileAccess.open")
		if path.begins_with("res://scenes/"):
			assert_array(Array(LayerRules.check(path, text))).override_failure_message(path).is_empty()


func test_session_end_drops_every_f069_store() -> void:
	var services: ServiceLocator = ServiceLocator.new(h.api, AppState, h.clock)
	h.http.reply(F069Fixtures.ROUTE_PROGRESSION, 200, F069Fixtures.progression())
	h.http.reply(F069Fixtures.ROUTE_WALLET, 200, F069Fixtures.wallet())
	h.http.reply(F069Fixtures.ROUTE_INVENTORY, 200, F069Fixtures.page(F069Fixtures.inventory_items()))
	h.http.reply(F069Fixtures.ROUTE_ACHIEVEMENTS, 200, F069Fixtures.achievements())
	h.http.reply(F069Fixtures.ROUTE_CHALLENGES, 200, F069Fixtures.challenges())
	h.http.reply(F069Fixtures.ROUTE_ATTEMPT, 200, F069Fixtures.attempt())
	await services.progression.refresh()
	await services.inventory.load_wallet()
	await services.inventory.load_inventory()
	await services.achievements.load_achievements()
	await services.challenge.list()
	await services.challenge.resume(F069Fixtures.ATTEMPT_ID)
	assert_str(String(AppState.blocking_activity)).is_equal("challenge")
	services.end_session()
	assert_object(services.progression.progression()).is_null()
	assert_object(services.inventory.wallet_store.wallet).is_null()
	assert_array(services.inventory.items).is_empty()
	assert_array(services.achievements.achievements).is_empty()
	assert_array(services.challenge.challenges).is_empty()
	assert_object(services.challenge.attempt()).is_null()
	assert_str(String(AppState.blocking_activity)).is_empty()


func test_tenant_header_and_session_ending_refusals() -> void:
	var denied: Array[String] = []
	var services: ServiceLocator = ServiceLocator.new(h.api, AppState, h.clock)
	services.access_denied_handler = func(code: String) -> void: denied.append(code)
	h.http.reply(F069Fixtures.ROUTE_PROGRESSION, 200, F069Fixtures.progression())
	await services.progression.refresh()
	assert_str(String(h.http.last(F069Fixtures.ROUTE_PROGRESSION).headers.get("x-tenant-id", ""))).is_equal(AuthHarness.TENANT_ID)
	h.http.reply(F069Fixtures.ROUTE_WALLET, 403, F069Fixtures.problem("GAME_ACCESS_STUDENT_ONLY", 403))
	await services.inventory.load_wallet()
	h.http.reply(F069Fixtures.ROUTE_CHALLENGES, 403, F069Fixtures.problem("GAME_ACCESS_STUDENT_ONLY", 403))
	await services.challenge.list()
	h.http.reply(F069Fixtures.ROUTE_ACHIEVEMENTS, 403, F069Fixtures.problem("GAME_ACCESS_STUDENT_ONLY", 403))
	await services.achievements.load_achievements()
	assert_array(denied).contains_exactly(["GAME_ACCESS_STUDENT_ONLY", "GAME_ACCESS_STUDENT_ONLY", "GAME_ACCESS_STUDENT_ONLY"])
	var tenant: Array[String] = []
	h.api.auth_error_handler = func(code: String) -> void: tenant.append(code)
	h.http.clear(F069Fixtures.ROUTE_PROGRESSION)
	h.http.reply(F069Fixtures.ROUTE_PROGRESSION, 403, F069Fixtures.problem("TENANT_MISMATCH", 403))
	await services.progression.refresh()
	assert_array(tenant).contains(["TENANT_MISMATCH"])


func test_validation_failed_is_a_client_bug_without_pii() -> void:
	var services: ServiceLocator = ServiceLocator.new(h.api, AppState, h.clock)
	services.telemetry.clear()
	h.http.reply(F069Fixtures.ROUTE_CHALLENGES, 422, F069Fixtures.problem("VALIDATION_FAILED", 422))
	var notices: Array[String] = []
	await services.challenge.list({"type": "time"})
	assert_int(services.telemetry.queue.size()).is_equal(1)
	assert_str(String(services.telemetry.queue[0]["error_code"])).is_equal("client_bug")
	assert_str(String(services.telemetry.queue[0]["scene_code"])).is_equal("listGameChallenges")
	var dump: String = JSON.stringify(services.telemetry.queue)
	for pii: String in ["MeoXanh42", "hs.an.3a", AuthHarness.ACCESS_1]:
		assert_str(dump).not_contains(pii)
	h.http.reply(F069Fixtures.ROUTE_START, 422, F069Fixtures.problem("VALIDATION_FAILED", 422))
	services.challenge.notice_requested.connect(func(key: String, _a: Dictionary) -> void: notices.append(key))
	await services.challenge.start(F069Fixtures.CHALLENGE_TIME, "easy")
	assert_array(notices).contains(["ui.error.generic"])


func test_client_sends_only_server_ids_and_enums() -> void:
	var client: ChallengeClient = ChallengeClient.new(h.api, null, h.clock)
	var bad: ApiResult = await client.start("not-a-uuid", "easy")
	assert_bool(bad.ok).is_false()
	bad = await client.start(F069Fixtures.CHALLENGE_TIME, "super_hard")
	assert_bool(bad.ok).is_false()
	assert_int(h.http.count(F069Fixtures.ROUTE_START)).is_equal(0)
	var achievements: AchievementService = AchievementService.new(h.api)
	assert_object(await achievements.load_collection("../../etc")).is_null()
	assert_int(h.http.count(F069Fixtures.ROUTE_COLLECTION)).is_equal(0)
