extends GdUnitTestSuite
## F069 InventoryService (T017; F069-AC-07, AC-10, AC-11, AC-12; INVENTORY_ECONOMY §4–§5, AGENTS R10, R19): one
## Idempotency-Key per purchase intent kept on network/5xx retries and on a second press after a failure (never
## charged twice, SC-002), the balance from `balance_after`, the new item from `item`, refusals as codes with the wallet
## and shop read again, `428` / `IDEMPOTENCY_KEY_REUSED` reported as `client_bug` telemetry, the ledgers by cursor and
## the inventory by `kind`. FakeHttp (AuthHarness).

var h: AuthHarness = null
var service: InventoryService = null
var telemetry: TelemetryService = null


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	telemetry = TelemetryService.new(h.api, h.clock)
	service = InventoryService.new(h.api, null, telemetry)
	h.http.reply(F069Fixtures.ROUTE_WALLET, 200, F069Fixtures.wallet(150))
	h.http.reply(F069Fixtures.ROUTE_SHOP, 200, F069Fixtures.page(F069Fixtures.shop_items()))
	await service.load_wallet()
	await service.load_shop()


func after_test() -> void:
	h.dispose()


func _cheap() -> ShopItemModel:
	return service.shop_item(F069Fixtures.SHOP_CHEAP)


func test_purchase_carries_one_key_and_takes_balance_after() -> void:
	h.http.reply(F069Fixtures.ROUTE_PURCHASE, 200, F069Fixtures.purchase_result(F069Fixtures.SHOP_CHEAP, "acc_flag_small", 30))
	var done: Array[Dictionary] = []
	service.purchase_completed.connect(func(r: Dictionary) -> void: done.append(r))
	var result: ApiResult = await service.purchase(_cheap())
	assert_bool(result.ok).is_true()
	var rec: FakeHttp.Recorded = h.http.last(F069Fixtures.ROUTE_PURCHASE)
	assert_str(rec.path).is_equal("/game/shop/items/%s:purchase" % F069Fixtures.SHOP_CHEAP)
	assert_bool(Uuid.is_valid(String(rec.headers.get("idempotency-key", "")))).is_true()
	assert_int(int(rec.body["expected_price"])).is_equal(120)
	assert_int(service.wallet_store.balance()).is_equal(30)
	assert_object(service.owned_item("acc_flag_small")).is_not_null()
	assert_bool("acc_flag_small" in service.owned_item_ids()).is_true()
	assert_int(done.size()).is_equal(1)
	assert_str(service.pending_purchase_key(F069Fixtures.SHOP_CHEAP)).is_empty()


func test_network_failure_keeps_the_key_for_the_retry() -> void:
	h.http.fail(F069Fixtures.ROUTE_PURCHASE)
	var first: ApiResult = await service.purchase(_cheap())
	assert_bool(first.ok).is_false()
	assert_bool(service.offline).is_true()
	var keys: PackedStringArray = []
	for rec: FakeHttp.Recorded in h.http.find(F069Fixtures.ROUTE_PURCHASE):
		keys.append(String(rec.headers["idempotency-key"]))
	# ApiClient retried the keyed write itself with the same key.
	assert_int(keys.size()).is_greater(1)
	var key: String = service.pending_purchase_key(F069Fixtures.SHOP_CHEAP)
	assert_str(key).is_not_empty()
	for k: String in keys:
		assert_str(k).is_equal(key)
	assert_int(service.wallet_store.balance()).is_equal(150)
	h.http.clear(F069Fixtures.ROUTE_PURCHASE)
	h.http.reply(F069Fixtures.ROUTE_PURCHASE, 200, F069Fixtures.purchase_result(F069Fixtures.SHOP_CHEAP, "acc_flag_small", 30),
			{"Idempotent-Replayed": "true"})
	var second: ApiResult = await service.purchase(_cheap())
	assert_bool(second.ok).is_true()
	assert_str(String(h.http.last(F069Fixtures.ROUTE_PURCHASE).headers["idempotency-key"])).is_equal(key)
	assert_int(service.wallet_store.balance()).is_equal(30)
	assert_bool(service.offline).is_false()


func test_server_error_keeps_the_key() -> void:
	h.http.reply(F069Fixtures.ROUTE_PURCHASE, 500, F069Fixtures.problem("INTERNAL", 500))
	await service.purchase(_cheap())
	var key: String = service.pending_purchase_key(F069Fixtures.SHOP_CHEAP)
	assert_str(key).is_not_empty()
	h.http.clear(F069Fixtures.ROUTE_PURCHASE)
	h.http.reply(F069Fixtures.ROUTE_PURCHASE, 200, F069Fixtures.purchase_result(F069Fixtures.SHOP_CHEAP, "acc_flag_small", 30))
	await service.purchase(_cheap())
	assert_str(String(h.http.last(F069Fixtures.ROUTE_PURCHASE).headers["idempotency-key"])).is_equal(key)


func test_refusals_are_codes_and_refresh_wallet_and_shop() -> void:
	for code: String in ["INSUFFICIENT_BALANCE", "PURCHASE_LIMIT_REACHED", "ITEM_ALREADY_OWNED", "SHOP_ITEM_UNAVAILABLE"]:
		h.http.clear(F069Fixtures.ROUTE_PURCHASE)
		h.http.reply(F069Fixtures.ROUTE_PURCHASE, 409, F069Fixtures.problem(code, 409))
		var failed: Array[String] = []
		var cb: Callable = func(c: String) -> void: failed.append(c)
		service.purchase_failed.connect(cb)
		var wallets: int = h.http.count(F069Fixtures.ROUTE_WALLET)
		var shops: int = h.http.count(F069Fixtures.ROUTE_SHOP)
		var result: ApiResult = await service.purchase(_cheap())
		for i: int in 4:
			await get_tree().process_frame
		service.purchase_failed.disconnect(cb)
		assert_str(result.code).is_equal(code)
		assert_array(failed).contains_exactly([code])
		assert_int(h.http.count(F069Fixtures.ROUTE_WALLET)).override_failure_message(code).is_greater(wallets)
		assert_int(h.http.count(F069Fixtures.ROUTE_SHOP)).override_failure_message(code).is_greater(shops)
		assert_str(service.pending_purchase_key(F069Fixtures.SHOP_CHEAP)).is_empty()
		assert_bool(L10n.has_key("ui.error." + code)).override_failure_message(code).is_true()
		assert_str(L10n.error_text(code)).not_contains(code)


func test_precondition_and_key_reuse_are_client_bugs() -> void:
	for spec: Array in [[428, "PRECONDITION_REQUIRED"], [422, "IDEMPOTENCY_KEY_REUSED"], [422, "VALIDATION_FAILED"]]:
		telemetry.clear()
		h.http.clear(F069Fixtures.ROUTE_PURCHASE)
		h.http.reply(F069Fixtures.ROUTE_PURCHASE, int(spec[0]), F069Fixtures.problem(String(spec[1]), int(spec[0])))
		await service.purchase(_cheap())
		assert_int(telemetry.queue.size()).override_failure_message(String(spec[1])).is_equal(1)
		assert_str(String(telemetry.queue[0]["error_code"])).is_equal("client_bug")
		assert_str(String(telemetry.queue[0]["scene_code"])).is_equal("purchaseShopItem")
		assert_str(JSON.stringify(telemetry.queue[0])).not_contains("MeoXanh42")


func test_inventory_by_kind_and_ledgers_by_cursor() -> void:
	h.http.reply(F069Fixtures.ROUTE_INVENTORY, 200, F069Fixtures.page(F069Fixtures.inventory_items()))
	await service.load_inventory("hair")
	assert_str(h.http.last(F069Fixtures.ROUTE_INVENTORY).query).contains("kind=hair")
	assert_str(service.items_kind).is_equal("hair")
	h.http.reply(F069Fixtures.ROUTE_LEDGER, 200, F069Fixtures.ledger())
	var page: Dictionary = await service.load_wallet_ledger()
	var entries: Array = page["entries"] as Array
	assert_int(entries.size()).is_equal(3)
	assert_str((entries[2] as LedgerEntryModel).label()).is_equal(
			tr("ui.collection.duplicate_converted").format({"amount": "25"}))
	h.http.reply(F069Fixtures.ROUTE_ITEM_HISTORY, 200, F069Fixtures.item_history())
	var items: Dictionary = await service.load_item_history("c2")
	assert_str(h.http.last(F069Fixtures.ROUTE_ITEM_HISTORY).query).contains("cursor=c2")
	assert_str(((items["entries"] as Array)[0] as LedgerEntryModel).label()).contains(tr("ui.inventory.reason.shop_purchase"))


func test_reward_notification_refreshes_the_wallet() -> void:
	var before: int = h.http.count(F069Fixtures.ROUTE_WALLET)
	service.handle_message("notification.item.created", {"notification_id": Uuid.v7(), "category": "reward", "title": {}})
	service.handle_message("notification.item.created", {"notification_id": Uuid.v7(), "category": "friend", "title": {}})
	for i: int in 3:
		await get_tree().process_frame
	assert_int(h.http.count(F069Fixtures.ROUTE_WALLET)).is_equal(before + 1)


func test_affordability_is_a_display_hint_only() -> void:
	var pricey: ShopItemModel = service.shop_item(F069Fixtures.SHOP_PRICEY)
	assert_int(pricey.missing_for(service.wallet_store.balance())).is_equal(40)
	assert_bool(service.shop_item(F069Fixtures.SHOP_OWNED).is_sold_out_for_me()).is_true()
	assert_bool(service.shop_item(F069Fixtures.SHOP_LIMITED).is_sold_out_for_me()).is_false()
