extends GdUnitTestSuite
## F064-AC-12, F064-AC-13 (TC-EXM-002, TC-EXM-003; T016): when the server clock reaches `deadline_at` input is locked
## once, the queued answers are sent first and `submitExamAttempt` is called once (best effort), and the runner shows
## `ui.exam.auto_submitted` then the waiting screen — the client never decides a score; `422 EXAM_TIME_OVER` on an
## answer or the submit closes the attempt without any retry. FakeHttp/FakeClock with contract fixtures (TEST-ONLY).

const LOBBY_SCENE: String = "res://scenes/exam/ExamLobby.tscn"

var h: AuthHarness = null
var service: ExamService = null
var expired: int = 0
var closed: int = 0


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	service = ExamService.new(h.api, h.clock)
	expired = 0
	closed = 0
	service.time_expired.connect(func() -> void: expired += 1)
	service.attempt_closed.connect(func() -> void: closed += 1)


func after_test() -> void:
	QualityManager.suspend_reasons.clear()
	h.dispose()


func _start() -> void:
	h.http.reply(ExamFixtures.ROUTE_START, 201, ExamFixtures.attempt())
	await service.start(ExamFixtures.SESSION_ID)


func test_deadline_on_the_server_clock_locks_once() -> void:
	await _start()
	h.clock.advance(899.0)
	service.tick()
	assert_int(expired).is_equal(0)
	assert_bool(service.locked).is_false()
	h.clock.advance(1.0)
	service.tick()
	service.tick()
	assert_int(expired).is_equal(1)
	assert_bool(service.locked).is_true()
	h.http.reply(ExamFixtures.ROUTE_SAVE, 204)
	assert_bool(await service.save_answer(1, {"option_id": "o_a"})).is_false()
	assert_int(h.http.count(ExamFixtures.ROUTE_SAVE)).is_equal(0)


func test_auto_submit_sends_the_queue_first_then_one_submit() -> void:
	await _start()
	h.http.fail(ExamFixtures.ROUTE_SAVE)
	await service.save_answer(1, {"option_id": "o_a"})
	await service.save_answer(2, {"option_ids": ["o_a"]})
	assert_int(service.pending.size()).is_equal(2)
	h.clock.advance(float(ExamFixtures.DURATION_SEC))
	service.tick()
	h.http.reply(ExamFixtures.ROUTE_SAVE, 204)
	h.http.reply(ExamFixtures.ROUTE_SUBMIT, 200, ExamFixtures.submit_result())
	await service.auto_submit()
	var order: PackedStringArray = []
	for rec: FakeHttp.Recorded in h.http.requests:
		if rec.path.begins_with("/exams/attempts/"):
			order.append(rec.method)
	# One failed PUT, the two queued PUTs replayed, then a single submit.
	assert_array(Array(order)).is_equal(["PUT", "PUT", "PUT", "POST"])
	assert_int(h.http.count(ExamFixtures.ROUTE_SUBMIT)).is_equal(1)
	assert_str(h.http.last(ExamFixtures.ROUTE_SUBMIT).raw_body).is_empty()
	assert_bool(service.submitted).is_true()
	assert_int(service.pending.size()).is_equal(0)


func test_auto_submit_is_best_effort_when_the_network_is_down() -> void:
	await _start()
	h.clock.advance(float(ExamFixtures.DURATION_SEC))
	service.tick()
	h.http.fail(ExamFixtures.ROUTE_SUBMIT)
	await service.auto_submit()
	assert_bool(service.locked).is_true()
	# ApiClient may retry the keyed submit inside the one call; the service never calls submit again by itself.
	var calls: int = h.http.count(ExamFixtures.ROUTE_SUBMIT)
	h.clock.advance(30.0)
	service.tick()
	await await_idle_frame()
	assert_int(h.http.count(ExamFixtures.ROUTE_SUBMIT)).is_equal(calls)


func test_time_over_on_an_answer_closes_without_retry() -> void:
	await _start()
	h.http.reply(ExamFixtures.ROUTE_SAVE, 422, ExamFixtures.problem(422, "EXAM_TIME_OVER"))
	assert_bool(await service.save_answer(1, {"option_id": "o_a"})).is_false()
	assert_int(closed).is_equal(1)
	assert_bool(service.locked).is_true()
	assert_int(service.pending.size()).is_equal(0)
	h.clock.advance(10.0)
	service.tick()
	await service.flush_pending()
	assert_bool(await service.save_answer(2, {"option_ids": ["o_a"]})).is_false()
	assert_int(h.http.count(ExamFixtures.ROUTE_SAVE)).is_equal(1)


func test_time_over_on_a_queued_answer_drops_the_rest() -> void:
	await _start()
	h.http.fail(ExamFixtures.ROUTE_SAVE)
	await service.save_answer(1, {"option_id": "o_a"})
	await service.save_answer(2, {"option_ids": ["o_a"]})
	h.http.reply(ExamFixtures.ROUTE_SAVE, 422, ExamFixtures.problem(422, "EXAM_TIME_OVER"))
	await service.flush_pending()
	assert_int(closed).is_equal(1)
	assert_int(service.pending.size()).is_equal(0)
	# 1 failed + 1 refused: the second queued answer is never sent.
	assert_int(h.http.count(ExamFixtures.ROUTE_SAVE)).is_equal(2)


func test_time_over_on_submit_is_closed_without_retry() -> void:
	await _start()
	h.http.reply(ExamFixtures.ROUTE_SUBMIT, 422, ExamFixtures.problem(422, "EXAM_TIME_OVER"))
	assert_bool(await service.submit()).is_true()
	assert_bool(service.submitted).is_true()
	assert_int(closed).is_equal(1)
	assert_bool(await service.submit()).is_true()
	assert_int(h.http.count(ExamFixtures.ROUTE_SUBMIT)).is_equal(1)


func test_runner_locks_shows_auto_submitted_and_goes_to_the_waiting_screen() -> void:
	var services: ServiceLocator = ServiceLocator.new(h.api, AppState, h.clock)
	var lobby: Control = auto_free((load(LOBBY_SCENE) as PackedScene).instantiate()) as Control
	lobby.set("services", services)
	lobby.set("grade_band", "g4_5")
	add_child(lobby)
	h.http.reply(ExamFixtures.ROUTE_SESSION, 200, ExamFixtures.session())
	await lobby.call("open", {"exam_session_id": ExamFixtures.SESSION_ID})
	h.http.reply(ExamFixtures.ROUTE_START, 201, ExamFixtures.attempt())
	await lobby.call("start_exam")
	var runner: Control = lobby.get("runner") as Control
	var question: QuestionRendererBase = runner.get("question") as QuestionRendererBase
	h.http.reply(ExamFixtures.ROUTE_SUBMIT, 200, ExamFixtures.submit_result())
	h.clock.advance(float(ExamFixtures.DURATION_SEC))
	runner.call("_process", 1.0)
	assert_bool(question.locked).is_true()
	assert_str((runner.find_child("Notice", true, false) as Label).text).is_equal(tr("ui.exam.auto_submitted"))
	for i: int in 30:
		await await_idle_frame()
	assert_int(h.http.count(ExamFixtures.ROUTE_SUBMIT)).is_equal(1)
	assert_bool(QualityManager.suspend_reasons.has(&"exam")).is_false()
	assert_bool((lobby.find_child("After", true, false) as Control).visible).is_true()
	assert_object(lobby.get("runner")).is_null()
