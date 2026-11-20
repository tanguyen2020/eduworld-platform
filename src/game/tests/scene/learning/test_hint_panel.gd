extends GdUnitTestSuite
## F063-AC-15, F063-AC-16 (TC-LRN-011; T023): "Gợi ý" calls `revealNextHint` with a fresh `Idempotency-Key` and shows
## the hint text with "Gợi ý {n}/{total}"; `409 HINTS_EXHAUSTED` greys the button with the exhausted message;
## `403 HINTS_NOT_ALLOWED` or a mode without hints hides it; the client never counts hints (levels from the server).

const PANEL_SCENE: String = "res://scenes/learning/HintPanel.tscn"
const HOST_SCENE: String = "res://scenes/learning/ActivityHost.tscn"

var h: AuthHarness = null


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()


func after_test() -> void:
	QualityManager.suspend_reasons.clear()
	h.dispose()


func _panel() -> VBoxContainer:
	var panel: VBoxContainer = auto_free((load(PANEL_SCENE) as PackedScene).instantiate()) as VBoxContainer
	add_child(panel)
	return panel


func _host(attempt: Dictionary) -> Control:
	var host: Control = auto_free((load(HOST_SCENE) as PackedScene).instantiate()) as Control
	host.set("services", ServiceLocator.new(h.api, AppState, h.clock))
	host.set("client_version", "1.2.3")
	add_child(host)
	h.http.reply(LearningFixtures.ROUTE_START, 201, attempt)
	await host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID})
	return host


func _settle(service: LearningService, route: String, count: int) -> void:
	for i: int in 120:
		await await_idle_frame()
		if h.http.count(route) >= count and service.last_error_code != "pending":
			await await_idle_frame()
			return


func test_service_sends_a_new_key_per_level_and_returns_the_hint() -> void:
	var service: LearningService = LearningService.new(h.api, null, h.clock)
	h.http.reply(LearningFixtures.ROUTE_HINT, 200, LearningFixtures.hint(1, 2))
	h.http.reply(LearningFixtures.ROUTE_HINT, 200, LearningFixtures.hint(2, 2))
	var first: HintResultModel = await service.request_next_hint(LearningFixtures.ATTEMPT_ID, 1)
	var second: HintResultModel = await service.request_next_hint(LearningFixtures.ATTEMPT_ID, 1)
	assert_int(first.level).is_equal(1)
	assert_int(second.level).is_equal(2)
	assert_bool(second.is_last()).is_true()
	var recs: Array[FakeHttp.Recorded] = h.http.find(LearningFixtures.ROUTE_HINT)
	assert_str(recs[0].path).is_equal("/learning/activity-attempts/%s/items/1/hints:next" % LearningFixtures.ATTEMPT_ID)
	assert_bool(Uuid.is_v7(String(recs[0].headers["idempotency-key"]))).is_true()
	assert_str(String(recs[0].headers["idempotency-key"])).is_not_equal(String(recs[1].headers["idempotency-key"]))
	assert_str(recs[0].raw_body).is_empty()


func test_service_reports_exhausted_and_not_allowed() -> void:
	var service: LearningService = LearningService.new(h.api, null, h.clock)
	h.http.reply(LearningFixtures.ROUTE_HINT, 409, LearningFixtures.problem(409, "HINTS_EXHAUSTED"))
	assert_object(await service.request_next_hint(LearningFixtures.ATTEMPT_ID, 1)).is_null()
	assert_str(service.last_error_code).is_equal("HINTS_EXHAUSTED")
	h.http.reply(LearningFixtures.ROUTE_HINT, 403, LearningFixtures.problem(403, "HINTS_NOT_ALLOWED"))
	assert_object(await service.request_next_hint(LearningFixtures.ATTEMPT_ID, 1)).is_null()
	assert_str(service.last_error_code).is_equal("HINTS_NOT_ALLOWED")


func test_panel_shows_level_and_greys_out_after_the_last() -> void:
	var panel: VBoxContainer = _panel()
	panel.call("configure", true, 2, 0)
	assert_bool(bool(panel.call("is_button_visible"))).is_true()
	assert_bool(bool(panel.call("is_button_disabled"))).is_false()
	await panel.call("show_hint", HintResultModel.from_dict(LearningFixtures.hint(1, 2)), null)
	assert_str(String(panel.call("level_text"))).is_equal(tr("ui.activity_player.hint_level").format({"n": 1, "total": 2}))
	assert_str((panel.get_node("%HintText") as RichTextLabel).get_parsed_text()).contains("Mỗi túi có 6 quả")
	assert_bool((panel.get_node("%Intro") as Label).visible).is_true()
	await panel.call("show_hint", HintResultModel.from_dict(LearningFixtures.hint(2, 2)), null)
	assert_bool(bool(panel.call("is_button_disabled"))).is_true()


func test_panel_exhausted_and_hidden_states() -> void:
	var panel: VBoxContainer = _panel()
	panel.call("configure", true, -1, 0)
	assert_bool(bool(panel.call("is_button_visible"))).is_true()
	panel.call("mark_exhausted")
	assert_bool(bool(panel.call("is_button_disabled"))).is_true()
	assert_str((panel.get_node("%Status") as Label).text).is_equal("ui.activity_player.hints_exhausted")
	panel.call("hide_hints")
	assert_bool(bool(panel.call("is_button_visible"))).is_false()
	panel.call("configure", true, 0, 0)
	assert_bool(panel.visible).is_false()
	panel.call("configure", false, 3, 0)
	assert_bool(panel.visible).is_false()
	panel.call("configure", true, 2, 2)
	assert_bool(bool(panel.call("is_button_disabled"))).is_true()


func test_exam_like_modes_have_no_hints() -> void:
	for mode: String in ["exam", "diagnostic", "competition"]:
		var data: Dictionary = LearningFixtures.fresh_attempt()
		data["mode"] = mode
		assert_bool(AttemptModel.from_dict(data).hints_allowed()).override_failure_message(mode).is_false()
	assert_bool(AttemptModel.from_dict(LearningFixtures.fresh_attempt()).hints_allowed()).is_true()


func test_host_hint_flow_and_exhausted() -> void:
	var host: Control = await _host(LearningFixtures.fresh_attempt())
	var panel: VBoxContainer = host.get_node("%HintPanel") as VBoxContainer
	assert_bool(panel.visible).is_true()
	var service: LearningService = (host.get("services") as ServiceLocator).learning
	h.http.reply(LearningFixtures.ROUTE_HINT, 200, LearningFixtures.hint(1, 2))
	var key: InputEventKey = InputEventKey.new()
	key.keycode = KEY_H
	key.pressed = true
	host.call("_unhandled_input", key)
	await _settle(service, LearningFixtures.ROUTE_HINT, 1)
	assert_str(String(panel.call("level_text"))).is_equal(tr("ui.activity_player.hint_level").format({"n": 1, "total": 2}))
	h.http.reply(LearningFixtures.ROUTE_HINT, 409, LearningFixtures.problem(409, "HINTS_EXHAUSTED"))
	panel.call("request")
	await _settle(service, LearningFixtures.ROUTE_HINT, 2)
	assert_bool(bool(panel.call("is_button_disabled"))).is_true()
	assert_str((panel.get_node("%Status") as Label).text).is_equal("ui.activity_player.hints_exhausted")
	host.call("close")


func test_host_hides_hints_when_not_allowed() -> void:
	var host: Control = await _host(LearningFixtures.fresh_attempt())
	var panel: VBoxContainer = host.get_node("%HintPanel") as VBoxContainer
	var service: LearningService = (host.get("services") as ServiceLocator).learning
	h.http.reply(LearningFixtures.ROUTE_HINT, 403, LearningFixtures.problem(403, "HINTS_NOT_ALLOWED"))
	panel.call("request")
	await _settle(service, LearningFixtures.ROUTE_HINT, 1)
	assert_bool(bool(panel.call("is_button_visible"))).is_false()
	host.call("close")


func test_host_hides_hints_for_an_item_without_hints() -> void:
	var data: Dictionary = LearningFixtures.fresh_attempt()
	((data["items"] as Array)[0] as Dictionary)["hint_count"] = 0
	var host: Control = await _host(data)
	assert_bool((host.get_node("%HintPanel") as Control).visible).is_false()
	host.call("close")
