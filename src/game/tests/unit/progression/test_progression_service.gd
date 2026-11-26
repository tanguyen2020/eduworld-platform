extends GdUnitTestSuite
## F069 ProgressionService (T009, T013; F069-AC-01, AC-02, AC-05, AC-06; FR-GAM-007/008, ADR-0016): `getProgression` fills
## the store, `progression.xp.updated` REPLACES the values (never adds the delta) without any REST call, `capped`
## turns the moon on, `xp_to_next = null` is the maximum level, a level span change reloads `getProgression`, the XP
## history pages by cursor with friendly labels, and no request ever carries a score/XP/result field. FakeHttp
## (AuthHarness), contract fixtures (gen_fixtures).

var h: AuthHarness = null
var service: ProgressionService = null
var store_node: Node = null


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	store_node = auto_free(BlockingStoreFake.new())
	service = ProgressionService.new(h.api, store_node)


func after_test() -> void:
	h.dispose()


func test_entering_reads_get_progression_into_the_store() -> void:
	h.http.reply(F069Fixtures.ROUTE_PROGRESSION, 200, F069Fixtures.progression(3, 40, 80))
	var result: ApiResult = await service.refresh()
	assert_bool(result.ok).is_true()
	var p: ProgressionModel = service.progression()
	assert_int(p.level).is_equal(3)
	assert_int(p.xp_in_level).is_equal(40)
	assert_int(p.span()).is_equal(120)
	assert_str(p.title_text()).is_not_empty()
	assert_float(p.fraction()).is_equal_approx(40.0 / 120.0, 0.001)
	assert_str(h.http.last(F069Fixtures.ROUTE_PROGRESSION).headers.get("x-tenant-id", "")).is_equal(AuthHarness.TENANT_ID)


func test_xp_message_replaces_values_without_polling() -> void:
	h.http.reply(F069Fixtures.ROUTE_PROGRESSION, 200, F069Fixtures.progression(3, 40, 80))
	await service.refresh()
	var before: int = h.http.count(F069Fixtures.ROUTE_PROGRESSION)
	var gained: Array[int] = []
	service.store.xp_gained.connect(func(d: int) -> void: gained.append(d))
	# Same span (in + to_next = 120): no reload; the values are the message's, not old + delta.
	service.handle_message(ProgressionService.MSG_XP_UPDATED, F069Fixtures.xp_update(25, 65, 55))
	await get_tree().process_frame
	assert_int(service.progression().xp_in_level).is_equal(65)
	assert_int(service.progression().xp_total).is_equal(465)
	assert_array(gained).contains_exactly([25])
	assert_int(h.http.count(F069Fixtures.ROUTE_PROGRESSION)).is_equal(before)


func test_capped_message_shows_the_moon_and_null_is_max_level() -> void:
	h.http.reply(F069Fixtures.ROUTE_PROGRESSION, 200, F069Fixtures.progression(3, 40, 80))
	await service.refresh()
	service.handle_message(ProgressionService.MSG_XP_UPDATED, F069Fixtures.xp_update(10, 50, 70, true))
	assert_bool(service.progression().capped).is_true()
	h.http.reply(F069Fixtures.ROUTE_PROGRESSION, 200, F069Fixtures.progression(50, 0, null))
	await service.refresh()
	assert_bool(service.progression().is_max_level()).is_true()
	assert_float(service.progression().fraction()).is_equal(1.0)


func test_span_change_reloads_progression() -> void:
	h.http.reply(F069Fixtures.ROUTE_PROGRESSION, 200, F069Fixtures.progression(3, 40, 80))
	await service.refresh()
	h.http.reply(F069Fixtures.ROUTE_PROGRESSION, 200, F069Fixtures.progression(3, 40, 100))
	service.handle_message(ProgressionService.MSG_XP_UPDATED, F069Fixtures.xp_update(0, 40, 100))
	for i: int in 4:
		await get_tree().process_frame
	assert_int(h.http.count(F069Fixtures.ROUTE_PROGRESSION)).is_equal(2)
	assert_int(service.progression().span()).is_equal(140)


func test_level_reached_is_queued_and_reloads() -> void:
	h.http.reply(F069Fixtures.ROUTE_PROGRESSION, 200, F069Fixtures.progression(3, 40, 80))
	await service.refresh()
	var released: Array[LevelUpModel] = []
	service.queue.batch_released.connect(func(b: LevelUpModel) -> void: released.append(b))
	var reached: Array[int] = []
	service.level_reached.connect(func(l: int) -> void: reached.append(l))
	h.http.reply(F069Fixtures.ROUTE_PROGRESSION, 200, F069Fixtures.progression(4, 5, 140))
	service.handle_message(ProgressionService.MSG_LEVEL_REACHED, F069Fixtures.level_up(3, 4))
	for i: int in 4:
		await get_tree().process_frame
	assert_int(released.size()).is_equal(1)
	assert_int(released[0].to_level).is_equal(4)
	assert_array(reached).contains_exactly([4])
	assert_int(service.progression().level).is_equal(4)


func test_history_pages_by_cursor_with_friendly_labels() -> void:
	h.http.reply(F069Fixtures.ROUTE_HISTORY, 200, F069Fixtures.history(4, "cursor-2"))
	var page: Dictionary = await service.load_history()
	assert_bool(bool(page["ok"])).is_true()
	assert_str(String(page["next_cursor"])).is_equal("cursor-2")
	var rec: FakeHttp.Recorded = h.http.last(F069Fixtures.ROUTE_HISTORY)
	assert_str(rec.query).contains("limit=20")
	var entries: Array = page["entries"] as Array
	assert_int(entries.size()).is_equal(4)
	assert_str((entries[0] as XpEntryModel).label()).is_equal(tr("ui.journey.source.activity.completed"))
	# An unknown source code falls back to the server's friendly label.
	assert_str((entries[3] as XpEntryModel).label()).is_equal("Bài học vui")
	await service.load_history("cursor-2")
	assert_str(h.http.last(F069Fixtures.ROUTE_HISTORY).query).contains("cursor=cursor-2")


func test_no_request_carries_progress_values() -> void:
	h.http.reply(F069Fixtures.ROUTE_PROGRESSION, 200, F069Fixtures.progression())
	h.http.reply(F069Fixtures.ROUTE_HISTORY, 200, F069Fixtures.history())
	h.http.reply(F069Fixtures.ROUTE_LEVELS, 200, F069Fixtures.levels())
	await service.refresh()
	await service.load_history()
	await service.load_levels()
	service.handle_message(ProgressionService.MSG_XP_UPDATED, F069Fixtures.xp_update(10, 50))
	for rec: FakeHttp.Recorded in h.http.requests:
		for field: String in ["xp", "level", "score", "amount", "lives_left", "result"]:
			assert_bool(rec.body.has(field)).override_failure_message("%s %s" % [rec.path, field]).is_false()
			assert_bool(rec.query.contains(field + "=")).override_failure_message("%s %s" % [rec.path, field]).is_false()


func test_clear_drops_everything() -> void:
	h.http.reply(F069Fixtures.ROUTE_PROGRESSION, 200, F069Fixtures.progression())
	await service.refresh()
	service.queue.push(LevelUpModel.from_dict(F069Fixtures.level_up(3, 4)))
	service.clear()
	assert_object(service.progression()).is_null()
	assert_array(service.queue.pending).is_empty()
