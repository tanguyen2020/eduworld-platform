extends GdUnitTestSuite
## F069 S08 InventoryShopScreen (T018, T019, T020; F069-AC-07…AC-12, AC-26, AC-28): tabs by entry point, `listInventory`
## by kind, ONE AvatarPreview3D that renders once per change, equipping through ProfileService with the preview taking
## the server's answer (422 → previous look + friendly message), the shop's "Cần thêm {n} Xu", ALWAYS a confirmation
## with the price and the balance after, the key kept on a network retry, the history tab hidden for g1_3 with
## friendly reasons for older grades, rarity by colour + shape (+ word for grades 4–9), writes locked while offline.

const SCREEN: String = "res://scenes/inventory/InventoryShopScreen.tscn"

var s: F069SceneHarness = null


func before_test() -> void:
	s = F069SceneHarness.new()
	await s.start("g1_3")


func after_test() -> void:
	s.dispose()


func _open(tab: String, band: String = "g1_3") -> Array:
	AppState.set_grade_band(band)
	var runner: GdUnitSceneRunner = scene_runner(SCREEN)
	await runner.simulate_frames(1)
	var screen: Control = runner.scene() as Control
	screen.call("open", {"tab": tab})
	await runner.simulate_frames(8)
	return [runner, screen]


func test_wardrobe_tab_loads_inventory_by_kind_with_one_preview() -> void:
	var r: Array = await _open("wardrobe")
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var screen: Control = r[1] as Control
	assert_int(s.h.http.count(F069Fixtures.ROUTE_INVENTORY)).is_equal(1)
	assert_int(s.h.http.count(F069Fixtures.ROUTE_WALLET)).is_greater_equal(1)
	assert_int(screen.find_children("*", "SubViewportContainer", true, false).size()).is_equal(1)
	var preview: AvatarPreview3D = screen.call("preview") as AvatarPreview3D
	assert_bool(preview.is_continuous()).is_false()
	(screen.find_child("Chip_hair", true, false) as Button).pressed.emit()
	await runner.simulate_frames(6)
	assert_str(s.h.http.last(F069Fixtures.ROUTE_INVENTORY).query).contains("kind=hair")


func test_equip_takes_the_server_answer_and_reverts_on_refusal() -> void:
	var r: Array = await _open("wardrobe")
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var screen: Control = r[1] as Control
	var before: AvatarConfigModel = s.services.profile.avatar_config()
	screen.call("select_entry", 5) # acc_cap_star
	await runner.simulate_frames(2)
	var saved: AvatarConfigModel = before.copy()
	saved.accessories["head"] = "acc_cap_star"
	s.h.http.reply(ProfileFixtures.ROUTE_AVATAR, 200, saved.to_dict())
	await screen.call("equip_selected")
	var put: FakeHttp.Recorded = s.h.http.last(ProfileFixtures.ROUTE_AVATAR)
	assert_str(JSON.stringify(put.body)).contains("acc_cap_star")
	var preview: AvatarPreview3D = screen.call("preview") as AvatarPreview3D
	assert_str(String(preview.config.accessories.get("head", ""))).is_equal("acc_cap_star")
	var renders: int = preview.render_requests
	screen.call("select_entry", 6)
	await runner.simulate_frames(2)
	s.h.http.clear(ProfileFixtures.ROUTE_AVATAR)
	s.h.http.reply(ProfileFixtures.ROUTE_AVATAR, 422, F069Fixtures.problem("ITEM_NOT_OWNED", 422))
	await screen.call("equip_selected")
	assert_str(String(preview.config.accessories.get("head", ""))).is_equal("acc_cap_star")
	assert_str(String(screen.call("notice_text"))).is_equal(L10n.error_text("ITEM_NOT_OWNED"))
	assert_int(preview.render_requests).is_greater(renders)
	s.h.http.clear(ProfileFixtures.ROUTE_AVATAR)
	s.h.http.reply(ProfileFixtures.ROUTE_AVATAR, 422, F069Fixtures.problem("EQUIP_SLOT_INVALID", 422))
	await screen.call("equip_selected")
	assert_str(String(screen.call("notice_text"))).is_equal(L10n.error_text("EQUIP_SLOT_INVALID"))


func test_shop_need_more_and_always_confirms_with_balance_after() -> void:
	var r: Array = await _open("shop")
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var screen: Control = r[1] as Control
	screen.call("select_entry", 1) # 190 Xu with 150 Xu
	await runner.simulate_frames(2)
	assert_bool((screen.call("buy_button") as Button).disabled).is_true()
	assert_str(String(screen.call("missing_text"))).is_equal(tr("ui.shop.need_more").format({"n": "40"}))
	screen.call("select_entry", 0) # 120 Xu
	await runner.simulate_frames(2)
	var buy: Button = screen.call("buy_button") as Button
	assert_bool(buy.disabled).is_false()
	buy.pressed.emit()
	await runner.simulate_frames(1)
	assert_bool(bool(screen.call("dialog_open"))).is_true()
	assert_int(s.h.http.count(F069Fixtures.ROUTE_PURCHASE)).is_equal(0)
	var texts: PackedStringArray = screen.call("dialog_texts") as PackedStringArray
	assert_str(texts[0]).is_equal(tr("ui.shop.confirm_purchase").format({"item": "Cờ nhỏ", "price": "120"}))
	assert_str(texts[1]).is_equal(tr("ui.shop.balance_after").format({"balance": "30"}))
	s.h.http.reply(F069Fixtures.ROUTE_PURCHASE, 200, F069Fixtures.purchase_result(F069Fixtures.SHOP_CHEAP, "acc_flag_small", 30))
	await screen.call("confirm_purchase")
	assert_bool(bool(screen.call("dialog_open"))).is_false()
	assert_str(String(screen.call("notice_text"))).is_equal(tr("ui.shop.bought").format({"item": "Cờ nhỏ"}))
	assert_int(s.services.inventory.wallet_store.balance()).is_equal(30)


func test_network_failure_keeps_the_dialog_and_the_key() -> void:
	var r: Array = await _open("shop")
	var screen: Control = r[1] as Control
	screen.call("select_entry", 0)
	screen.call("ask_purchase")
	s.h.http.fail(F069Fixtures.ROUTE_PURCHASE)
	await screen.call("confirm_purchase")
	assert_bool(bool(screen.call("dialog_open"))).is_true()
	assert_str((screen.call("dialog_texts") as PackedStringArray)[2]).is_equal(tr("ui.shop.retry_safe"))
	var key: String = s.services.inventory.pending_purchase_key(F069Fixtures.SHOP_CHEAP)
	assert_str(key).is_not_empty()
	s.h.http.clear(F069Fixtures.ROUTE_PURCHASE)
	s.h.http.reply(F069Fixtures.ROUTE_PURCHASE, 200, F069Fixtures.purchase_result(F069Fixtures.SHOP_CHEAP, "acc_flag_small", 30))
	await screen.call("confirm_purchase")
	assert_str(String(s.h.http.last(F069Fixtures.ROUTE_PURCHASE).headers["idempotency-key"])).is_equal(key)


func test_refusal_shows_a_friendly_message_never_the_code() -> void:
	var r: Array = await _open("shop")
	var screen: Control = r[1] as Control
	screen.call("select_entry", 0)
	screen.call("ask_purchase")
	s.h.http.reply(F069Fixtures.ROUTE_PURCHASE, 409, F069Fixtures.problem("SHOP_ITEM_UNAVAILABLE", 409))
	await screen.call("confirm_purchase")
	var text: String = String(screen.call("notice_text"))
	assert_str(text).is_equal(L10n.error_text("SHOP_ITEM_UNAVAILABLE"))
	assert_str(text).not_contains("SHOP_ITEM_UNAVAILABLE")
	screen.call("select_entry", 0)
	screen.call("ask_purchase")
	s.h.http.clear(F069Fixtures.ROUTE_PURCHASE)
	s.h.http.reply(F069Fixtures.ROUTE_PURCHASE, 422, F069Fixtures.problem("IDEMPOTENCY_KEY_REUSED", 422))
	await screen.call("confirm_purchase")
	assert_str(String(screen.call("notice_text"))).is_equal(tr("ui.error.generic"))


func test_history_hidden_for_g1_3_and_friendly_for_older_grades() -> void:
	var r: Array = await _open("history", "g1_3")
	var screen: Control = r[1] as Control
	assert_bool(bool(screen.call("tab_visible", "history"))).is_false()
	assert_str(String(screen.get("tab"))).is_equal("wardrobe")
	var r2: Array = await _open("history", "g6_7")
	var older: Control = r2[1] as Control
	await (r2[0] as GdUnitSceneRunner).simulate_frames(4)
	assert_bool(bool(older.call("tab_visible", "history"))).is_true()
	var texts: PackedStringArray = older.call("history_texts") as PackedStringArray
	assert_array(Array(texts)).contains([tr("ui.collection.duplicate_converted").format({"amount": "25"}),
		tr("ui.inventory.reason.shop_purchase")])


func test_rarity_is_colour_and_shape_with_the_word_for_older_grades() -> void:
	var shapes: Array[String] = []
	for rarity: String in RarityMark.RARITIES:
		assert_bool(RarityMark.shape_of(rarity) in shapes).override_failure_message(rarity).is_false()
		shapes.append(RarityMark.shape_of(rarity))
		assert_bool(L10n.has_key(RarityMark.key_of(rarity))).is_true()
	var r: Array = await _open("wardrobe", "g1_3")
	var screen: Control = r[1] as Control
	screen.call("select_entry", 1)
	assert_str(String(screen.call("rarity_word"))).is_empty()
	var r2: Array = await _open("wardrobe", "g6_7")
	var older: Control = r2[1] as Control
	older.call("select_entry", 1)
	assert_str(String(older.call("rarity_word"))).is_equal("ui.inventory.rarity.rare")


func test_offline_locks_buy_and_equip_but_keeps_the_items() -> void:
	var r: Array = await _open("wardrobe")
	var runner: GdUnitSceneRunner = r[0] as GdUnitSceneRunner
	var screen: Control = r[1] as Control
	s.h.http.clear(F069Fixtures.ROUTE_WALLET)
	s.h.http.fail(F069Fixtures.ROUTE_WALLET)
	await s.services.inventory.load_wallet()
	assert_bool(s.services.inventory.offline).is_true()
	screen.call("select_entry", 1)
	await runner.simulate_frames(2)
	assert_bool((screen.call("equip_button") as Button).disabled).is_true()
	assert_int((screen.call("grid") as VirtualItemGrid).entries.size()).is_equal(F069Fixtures.inventory_items().size())
	screen.call("select_tab", "shop")
	await runner.simulate_frames(4)
	# The shop answered (online again); the next failure locks "Mua" while the loaded shop stays visible.
	await s.services.inventory.load_wallet()
	assert_bool(s.services.inventory.offline).is_true()
	screen.call("select_entry", 0)
	await runner.simulate_frames(2)
	assert_bool((screen.call("buy_button") as Button).disabled).is_true()
	assert_str(String(screen.call("missing_text"))).is_equal(tr("ui.inventory.offline_locked"))


func test_touch_targets_follow_the_band() -> void:
	for band: String in ["g1_3", "g6_7"]:
		var r: Array = await _open("shop", band)
		var screen: Control = r[1] as Control
		screen.call("select_entry", 0)
		await (r[0] as GdUnitSceneRunner).simulate_frames(2)
		var minimum: float = float(UiTokens.DENSITY_BUTTON_MIN_HEIGHT[band])
		for b: Node in screen.find_children("*", "Button", true, false):
			var button: Button = b as Button
			if button.is_visible_in_tree():
				assert_float(button.get_combined_minimum_size().y).override_failure_message("%s %s" % [band, button.name]) \
						.is_greater_equal(minimum)
