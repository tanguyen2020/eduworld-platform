extends GdUnitTestSuite
## F073-AC-25, F073-AC-26 client part (T037): the 5 extended scenes (calculation, puzzle, word_game,
## science_simulation, timed_challenge) follow the same MiniGameBase contract as the core ones — exact response shapes,
## effects only after the server's grading, no orphan after teardown (touch targets: minigame_host_test covers all 11
## types). `timed_challenge` gets every item up front and shows the next one without waiting for the network, while
## each verdict still comes from the server; `science_simulation` sends its final state as the response of its
## `interactive` question. FakeHttp/FakeClock with the F073 fixtures (TEST-ONLY).

const HOST_SCENE: String = "res://scenes/minigames/MiniGameHost.tscn"
const SHAPES: Dictionary = {
	"single_choice": ["option_id"], "true_false": ["value"], "fill_in_blank": ["blanks"], "short_answer": ["text"],
	"numeric": ["value"], "ordering": ["order"], "interactive": ["state", "action_log"],
}

var h: AuthHarness = null
var services: ServiceLocator = null
var host: MiniGameHost = null
var _prepared: Array = []


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	services = ServiceLocator.new(h.api, AppState, h.clock)


func after_test() -> void:
	QualityManager.suspend_reasons.clear()
	h.dispose()


func _ctx() -> RendererContext:
	var ctx: RendererContext = RendererContext.new()
	ctx.content_language = "vi"
	ctx.grade_band = "g4_5"
	return ctx


func _scene(type: String) -> MiniGameQuestionScene:
	var s: MiniGameSessionModel = MiniGameFixtures.model(type)
	var scene: MiniGameQuestionScene = auto_free(MiniGameRegistry.instantiate(type)) as MiniGameQuestionScene
	add_child(scene)
	_prepared.clear()
	scene.response_prepared.connect(func(item_id: String, response: Dictionary) -> void: _prepared.append([item_id, response]))
	scene.setup(s.config, s, _ctx())
	return scene


func _answer(scene: MiniGameQuestionScene, i: int) -> Dictionary:
	_prepared.clear()
	var item: QuestionRenderModel = scene.session.items()[i]
	scene.present_item(item)
	MiniGameFixtures.answer(scene)
	if _prepared.is_empty():
		(scene.find_child("Done", true, false) as Button).pressed.emit()
	assert_array(_prepared).override_failure_message(item.type).has_size(1)
	assert_str(String(_prepared[0][0])).is_equal(str(item.position))
	var response: Dictionary = _prepared[0][1] as Dictionary
	for key: Variant in response.keys():
		assert_bool(String(key) in (SHAPES[item.type] as Array)).override_failure_message("%s: %s" % [item.type, key]).is_true()
	assert_array(Array(MiniGameFixtures.forbidden_paths(response))).is_empty()
	return response


func _open(type: String) -> MiniGameHost:
	host = auto_free((load(HOST_SCENE) as PackedScene).instantiate()) as MiniGameHost
	host.services = services
	host.grade_band = "g4_5"
	add_child(host)
	h.http.reply(MiniGameFixtures.ROUTE_START, 201, MiniGameFixtures.session(type))
	await host.open({"minigame_config_id": MiniGameFixtures.CONFIG_ID})
	return host


func _settle(max_frames: int = 240) -> void:
	for i: int in max_frames:
		await await_idle_frame()
		if not host.state in [MiniGameHost.State.AWAITING_RESULT, MiniGameHost.State.SUBMITTING] and host.results.size() > 0:
			return


func _result(position: int, correct: bool) -> ResponseResultModel:
	return ResponseResultModel.from_dict(MiniGameFixtures.result(position, correct))


# ---------- F073-AC-25: same contract ----------

func test_calculation_sends_the_numeric_value() -> void:
	var scene: MiniGameQuestionScene = _scene("calculation")
	assert_dict(_answer(scene, 0)).is_equal({"value": "6"})
	assert_dict(_answer(scene, 1)).is_equal({"value": "6"})


func test_word_game_sends_blanks_and_letter_order() -> void:
	var scene: MiniGameQuestionScene = _scene("word_game")
	assert_dict(_answer(scene, 0)).is_equal({"blanks": {"b_1": "4"}})
	assert_dict(_answer(scene, 1)).is_equal({"order": ["i_2", "i_1", "i_3"]})


func test_puzzle_accepts_every_auto_graded_type_and_sends_their_shapes() -> void:
	var scene: MiniGameQuestionScene = _scene("puzzle")
	assert_dict(_answer(scene, 0)).is_equal({"option_id": "o_b"})
	assert_dict(_answer(scene, 1)).is_equal({"value": false})
	assert_dict(_answer(scene, 2)).is_equal({"value": "6"})


func test_science_simulation_sends_the_final_state_of_its_interactive_question() -> void:
	var scene: MiniGameQuestionScene = _scene("science_simulation")
	assert_str(scene.session.items()[0].type).is_equal("interactive")
	assert_str((scene.find_child("Done", true, false) as Button).text).is_equal("ui.minigame.submit_state")
	var response: Dictionary = _answer(scene, 0)
	assert_float(float((response["state"] as Dictionary)["value"])).is_equal(18.0)
	assert_bool(response.has("action_log")).is_true()


func test_timed_challenge_sends_its_three_types() -> void:
	var scene: MiniGameQuestionScene = _scene("timed_challenge")
	assert_dict(_answer(scene, 0)).is_equal({"option_id": "o_b"})
	assert_dict(_answer(scene, 1)).is_equal({"value": false})
	assert_dict(_answer(scene, 2)).is_equal({"value": "6"})


func test_effects_play_only_after_the_server_grading() -> void:
	for type: String in ["calculation", "puzzle", "science_simulation", "timed_challenge"]:
		var scene: MiniGameQuestionScene = _scene(type)
		_answer(scene, 0)
		assert_int(scene.effects_played).override_failure_message(type).is_equal(0)
		assert_bool((scene.find_child("Effect", true, false) as Control).visible).override_failure_message(type).is_false()
		scene.show_feedback(_result(1, false))
		assert_int(scene.effects_played).override_failure_message(type).is_equal(1)
		assert_str(scene.last_icon).is_equal(MiniGameBase.ICON_INCORRECT)
		assert_int(scene.sounds_played).is_equal(1)


func test_word_game_end_of_round_defers_effects() -> void:
	var scene: MiniGameQuestionScene = _scene("word_game")
	assert_bool(scene.config.defers_effects()).is_true()
	_answer(scene, 0)
	scene.show_feedback(_result(1, true))
	_answer(scene, 1)
	scene.show_feedback(_result(2, true))
	assert_int(scene.effects_played).is_equal(0)
	scene.finish(MiniGameSummaryModel.from_attempt(AttemptModel.from_dict(MiniGameFixtures.submitted("word_game", 20, true))))
	assert_int(scene.effects_played).is_equal(2)


func test_puzzle_pieces_open_only_on_the_server_verdict() -> void:
	var scene: MiniGameQuestionScene = _scene("puzzle")
	assert_int((scene.get("pieces") as Array).size()).is_equal(3)
	_answer(scene, 0)
	assert_int(int(scene.get("revealed"))).is_equal(0)
	scene.show_feedback(_result(1, false))
	assert_int(int(scene.get("revealed"))).is_equal(0)
	_answer(scene, 1)
	scene.show_feedback(_result(2, true))
	assert_int(int(scene.get("revealed"))).is_equal(1)
	assert_str(((scene.get("pieces") as Array)[0] as Label).text).is_equal("★")


func test_extended_scenes_leave_no_orphan_after_teardown() -> void:
	for type: String in MiniGameFixtures.EXTENDED_TYPES:
		await await_idle_frame()
		var before: int = int(Performance.get_monitor(Performance.OBJECT_ORPHAN_NODE_COUNT))
		var s: MiniGameSessionModel = MiniGameFixtures.model(type)
		var scene: MiniGameQuestionScene = MiniGameRegistry.instantiate(type) as MiniGameQuestionScene
		add_child(scene)
		scene.setup(s.config, s, _ctx())
		scene.present_item(s.items()[0])
		MiniGameFixtures.answer(scene)
		scene.show_feedback(_result(1, true))
		scene.finish(MiniGameSummaryModel.new())
		scene.teardown()
		remove_child(scene)
		scene.free()
		await await_idle_frame()
		await await_idle_frame()
		var after: int = int(Performance.get_monitor(Performance.OBJECT_ORPHAN_NODE_COUNT))
		assert_int(after).override_failure_message("%s leaks %d node(s)" % [type, after - before]).is_less_equal(before)


# ---------- F073-AC-26: timed_challenge prefetch, server grading; science_simulation state ----------

func test_timed_challenge_shows_the_next_item_without_waiting_for_the_network() -> void:
	await _open("timed_challenge")
	assert_bool(host.session.config.prefetch_items).is_true()
	assert_int(host.session.items().size()).is_equal(3)
	h.http.delay_frames = 30
	h.http.reply(MiniGameFixtures.ROUTE_RESPOND, 200, MiniGameFixtures.result(1, true, 10, 1))
	MiniGameFixtures.answer(host.scene as MiniGameQuestionScene)
	# Synchronously after the tap: item 2 is on screen, the answer of item 1 is still on its way, no verdict yet.
	assert_int(host.index).is_equal(1)
	assert_int(host.state).is_equal(MiniGameHost.State.PLAYING)
	assert_int(host.scene.effects_played).is_equal(0)
	assert_bool(host.results.has(1)).is_false()
	await _settle()
	assert_bool(host.results.has(1)).is_true()
	assert_int(host.scene.effects_played).is_equal(1)
	assert_int(host.game_points).is_equal(10)
	var routes: Array[String] = []
	for rec: FakeHttp.Recorded in h.http.requests:
		routes.append("%s %s" % [rec.method, rec.path])
	for route: String in routes:
		assert_bool(route.begins_with("GET /learning")).override_failure_message(route).is_false()


func test_timed_challenge_is_graded_by_the_server_only() -> void:
	await _open("timed_challenge")
	h.http.reply(MiniGameFixtures.ROUTE_RESPOND, 200, MiniGameFixtures.result(1, false))
	MiniGameFixtures.answer(host.scene as MiniGameQuestionScene)
	await _settle()
	assert_str(host.scene.last_icon).is_equal(MiniGameBase.ICON_INCORRECT)
	var body: Dictionary = h.http.last(MiniGameFixtures.ROUTE_RESPOND).body
	assert_dict(body["response"] as Dictionary).is_equal({"option_id": "o_b"})
	assert_bool(body.has("is_correct")).is_false()


func test_science_simulation_state_goes_to_the_responses_endpoint() -> void:
	await _open("science_simulation")
	h.http.reply(MiniGameFixtures.ROUTE_RESPOND, 200, MiniGameFixtures.result(1, true))
	MiniGameFixtures.answer(host.scene as MiniGameQuestionScene)
	(host.scene.find_child("Done", true, false) as Button).pressed.emit()
	await _settle()
	var rec: FakeHttp.Recorded = h.http.last(MiniGameFixtures.ROUTE_RESPOND)
	assert_str(rec.path).is_equal("/learning/activity-attempts/%s/responses" % MiniGameFixtures.attempt_id("science_simulation"))
	assert_str(String(rec.body["kind"])).is_equal("question")
	assert_int(int(rec.body["position"])).is_equal(1)
	assert_float(float(((rec.body["response"] as Dictionary)["state"] as Dictionary)["value"])).is_equal(18.0)
