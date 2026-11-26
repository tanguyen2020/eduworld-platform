extends GdUnitTestSuite
## F063-AC-01, F063-AC-05, F063-AC-08, F063-AC-09, F063-AC-10 (TC-LRN-002, TC-PERF-020; T013): the activity player
## opened by the `activity_host` intent suspends the world, starts the attempt, picks the `quiz` renderer, runs
## Kiểm tra → server feedback → Tiếp → Hoàn thành → submit → result, pauses on exit, resumes the world on close and
## keeps the student's answer when the server refuses it. FakeHttp/FakeClock with contract fixtures (TEST-ONLY).

const HOST_SCENE: String = "res://scenes/learning/ActivityHost.tscn"

var h: AuthHarness = null
var services: ServiceLocator = null
var host: Control = null
var closed: int = 0


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	services = ServiceLocator.new(h.api, AppState, h.clock)
	host = auto_free((load(HOST_SCENE) as PackedScene).instantiate()) as Control
	host.set("services", services)
	host.set("client_version", "1.2.3")
	host.set("grade_band", "g4_5")
	add_child(host)
	closed = 0
	host.connect("closed", func() -> void: closed += 1)


func after_test() -> void:
	QualityManager.suspend_reasons.clear()
	h.dispose()


func _open(attempt: Dictionary) -> void:
	h.http.reply(LearningFixtures.ROUTE_START, 201, attempt)
	await host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID, "mode": "practice", "context_type": "free_play"})


func _renderer() -> QuizActivityRenderer:
	return host.get("renderer") as QuizActivityRenderer


func _primary() -> Button:
	return host.find_child("Primary", true, false) as Button


func _state() -> int:
	return int(host.get("state"))


## Waits until the player is no longer waiting for the server (FakeHttp answers after a few frames, retries included).
func _settle(max_frames: int = 120) -> void:
	for i: int in max_frames:
		await await_idle_frame()
		var waiting: bool = _state() in [1, 3, 5]
		var services_busy: bool = services.learning.offline and _state() == 3
		if not waiting or services_busy:
			return


func test_open_suspends_the_world_and_picks_the_quiz_renderer() -> void:
	await _open(LearningFixtures.fresh_attempt())
	assert_bool(QualityManager.suspend_reasons.has(&"activity")).is_true()
	assert_object(_renderer()).is_not_null()
	assert_int(_state()).is_equal(2)
	assert_str((host.find_child("Title", true, false) as Label).text).is_equal("Phép nhân trong bảng 6")
	assert_str((host.find_child("Progress", true, false) as Label).text).is_equal("1/11")
	var rec: FakeHttp.Recorded = h.http.last(LearningFixtures.ROUTE_START)
	assert_bool(Uuid.is_v7(String(rec.headers["idempotency-key"]))).is_true()
	assert_bool(_primary().disabled).is_true()


func test_close_resumes_the_world_and_returns_to_the_game() -> void:
	await _open(LearningFixtures.fresh_attempt())
	host.call("close")
	assert_bool(QualityManager.suspend_reasons.has(&"activity")).is_false()
	assert_bool(host.visible).is_false()
	assert_int(closed).is_equal(1)


func test_check_next_finish_submit_and_result() -> void:
	await _open(LearningFixtures.attempt_with_types(["single_choice", "true_false"]))
	_renderer().select_index(0)
	assert_bool(_primary().disabled).is_false()
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result())
	await host.call("press_primary")
	await _settle()
	var body: Dictionary = h.http.last(LearningFixtures.ROUTE_RESPOND).body
	assert_dict(body["response"] as Dictionary).is_equal({"option_id": "o_a"})
	assert_str(String(body["kind"])).is_equal("question")
	var strip: Control = host.find_child("FeedbackStrip", true, false) as Control
	assert_bool(strip.visible).is_true()
	assert_str(String(strip.call("verdict_icon"))).is_equal("incorrect")
	assert_str(_primary().text).is_equal("ui.activity_player.next")
	await host.call("press_primary")
	assert_int(_renderer().current_position()).is_equal(2)
	_renderer().select_index(0)
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result_correct())
	await host.call("press_primary")
	await _settle()
	assert_str(String(strip.call("verdict_icon"))).is_equal("correct")
	assert_str(_primary().text).is_equal("ui.activity_player.finish")
	h.http.reply(LearningFixtures.ROUTE_SUBMIT, 200, LearningFixtures.submitted())
	h.http.reply(LearningFixtures.ROUTE_REVIEW, 200, LearningFixtures.review())
	await host.call("press_primary")
	await _settle()
	assert_int(h.http.count(LearningFixtures.ROUTE_SUBMIT)).is_equal(1)
	assert_bool(Uuid.is_v7(String(h.http.last(LearningFixtures.ROUTE_SUBMIT).headers["idempotency-key"]))).is_true()
	var result_view: Control = host.get("result_view") as Control
	assert_object(result_view).is_not_null()
	assert_str(String(result_view.call("score_text"))).contains("1/2")
	assert_object(_renderer()).is_null()


func test_already_submitted_on_finish_still_shows_the_result() -> void:
	await _open(LearningFixtures.attempt_with_types(["true_false"]))
	_renderer().select_index(1)
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result_hidden())
	await host.call("press_primary")
	await _settle()
	h.http.reply(LearningFixtures.ROUTE_SUBMIT, 409, LearningFixtures.problem(409, "ATTEMPT_ALREADY_SUBMITTED"))
	h.http.reply(LearningFixtures.ROUTE_GET, 200, LearningFixtures.submitted())
	h.http.reply(LearningFixtures.ROUTE_REVIEW, 200, LearningFixtures.review())
	await host.call("press_primary")
	await _settle()
	assert_object(host.get("result_view")).is_not_null()


func test_exit_asks_then_pauses_and_closes() -> void:
	await _open(LearningFixtures.fresh_attempt())
	host.call("ask_exit")
	var confirm: Control = host.find_child("ExitConfirm", true, false) as Control
	assert_bool(confirm.visible).is_true()
	h.http.reply(LearningFixtures.ROUTE_PAUSE, 200, LearningFixtures.fresh_attempt())
	(host.find_child("Pause", true, false) as Button).pressed.emit()
	await await_millis(100)
	assert_int(h.http.count(LearningFixtures.ROUTE_PAUSE)).is_equal(1)
	assert_int(closed).is_equal(1)
	assert_bool(QualityManager.suspend_reasons.has(&"activity")).is_false()


func test_refused_answer_shows_the_code_message_and_keeps_the_answer() -> void:
	await _open(LearningFixtures.fresh_attempt())
	_renderer().select_index(1)
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 422, LearningFixtures.problem(422, "INVALID_RESPONSE"))
	await host.call("press_primary")
	await _settle()
	var status: Label = host.get_node("%Status") as Label
	assert_bool(status.visible).is_true()
	assert_str(status.text).is_equal(tr("ui.error.INVALID_RESPONSE"))
	assert_str(status.text).not_contains("INVALID_RESPONSE")
	assert_dict(_renderer().question.current_response()).is_equal({"option_id": "o_b"})
	assert_bool(_primary().disabled).is_false()
	(host.find_child("ActionDetails", true, false) as Button).pressed.emit()
	assert_str(status.text).contains("INVALID_RESPONSE")


func test_offline_answer_shows_the_banner_and_is_sent_later() -> void:
	await _open(LearningFixtures.fresh_attempt())
	_renderer().select_index(1)
	h.http.fail(LearningFixtures.ROUTE_RESPOND)
	await host.call("press_primary")
	await _settle()
	assert_bool((host.find_child("Offline", true, false) as Control).visible).is_true()
	assert_bool(_primary().disabled).is_true()
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result())
	await host.call("_flush")
	await _settle()
	assert_bool((host.find_child("Offline", true, false) as Control).visible).is_false()
	assert_bool((host.find_child("FeedbackStrip", true, false) as Control).visible).is_true()


func test_keyboard_numbers_and_enter() -> void:
	await _open(LearningFixtures.fresh_attempt())
	var key: InputEventKey = InputEventKey.new()
	key.keycode = KEY_3
	key.pressed = true
	host.call("_unhandled_input", key)
	assert_dict(_renderer().question.current_response()).is_equal({"option_id": "o_c"})
	h.http.reply(LearningFixtures.ROUTE_RESPOND, 200, LearningFixtures.response_result())
	var enter: InputEventKey = InputEventKey.new()
	enter.keycode = KEY_ENTER
	enter.pressed = true
	host.call("_unhandled_input", enter)
	await _settle()
	assert_int(h.http.count(LearningFixtures.ROUTE_RESPOND)).is_equal(1)


func test_question_set_assignment_intent_uses_an_ad_hoc_attempt() -> void:
	h.http.reply(LearningFixtures.ROUTE_ADHOC, 201, LearningFixtures.fresh_attempt())
	await host.call("open", {
		"question_set_id": LearningFixtures.QUESTION_SET_ID, "mode": "assignment", "context_type": "assignment",
		"context_id": LearningFixtures.ASSIGNMENT_ID,
	})
	assert_int(h.http.count(LearningFixtures.ROUTE_ADHOC)).is_equal(1)
	assert_object(_renderer()).is_not_null()


func test_not_found_shows_no_attempt_data() -> void:
	h.http.reply(LearningFixtures.ROUTE_START, 404, LearningFixtures.problem(404, "NOT_FOUND"))
	await host.call("open", {"activity_id": LearningFixtures.ACTIVITY_ID})
	assert_object(_renderer()).is_null()
	assert_str((host.find_child("Title", true, false) as Label).text).is_empty()
	assert_bool((host.find_child("StatePanel", true, false) as Control).visible).is_true()
	assert_str((host.find_child("StateLabel", true, false) as Label).text).is_equal(tr("ui.error.NOT_FOUND"))


func test_world_root_registers_the_activity_host_screen() -> void:
	assert_str(UiScreenRegistry.path_for(&"activity_host")).is_equal(HOST_SCENE)
	assert_bool(host.get("blocks_break_reminder") == true).is_true()
