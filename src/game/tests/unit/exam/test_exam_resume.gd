extends GdUnitTestSuite
## F064-AC-15, F064-AC-16 (TC-EXM-002; T018, T031): reopening an exam with an attempt in progress calls
## `startExamAttempt` again and rebuilds the runner from the returned attempt — items, `saved_responses` restored and
## the unchanged `deadline_at` (time left = deadline − the new server time); `409 EXAM_ACTIVE_ON_OTHER_DEVICE` shows
## that the exam is open on another device without opening the runner and offers "Chuyển sang thiết bị này"
## (`take_over`, new key); the same code while answering locks the runner. FakeHttp/FakeClock (TEST-ONLY).

const LOBBY_SCENE: String = "res://scenes/exam/ExamLobby.tscn"
const SAVED: Array = [
	{"position": 1, "response": {"option_id": "o_b"}},
	{"position": 2, "response": {"option_ids": ["o_a", "o_c"]}},
]

var h: AuthHarness = null
var services: ServiceLocator = null
var lobby: Control = null


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	services = ServiceLocator.new(h.api, AppState, h.clock)
	lobby = auto_free((load(LOBBY_SCENE) as PackedScene).instantiate()) as Control
	lobby.set("services", services)
	lobby.set("grade_band", "g4_5")
	add_child(lobby)


func after_test() -> void:
	QualityManager.suspend_reasons.clear()
	h.dispose()


func _open(navigation: String = "free") -> void:
	h.http.reply(ExamFixtures.ROUTE_SESSION, 200, ExamFixtures.session(navigation))
	await lobby.call("open", {"exam_session_id": ExamFixtures.SESSION_ID})


## The attempt as the server returns it again five minutes later: same deadline, later server time.
static func _resumed(minutes_later: int = 5) -> Dictionary:
	var a: Dictionary = ExamFixtures.attempt(5, SAVED)
	a["server_time"] = "2026-11-20T01:%02d:00Z" % (30 + minutes_later)
	return a


func test_reopening_rebuilds_the_runner_from_the_server_attempt() -> void:
	await _open()
	h.http.reply(ExamFixtures.ROUTE_START, 201, _resumed())
	await lobby.call("start_exam")
	var runner: Control = lobby.get("runner") as Control
	assert_object(runner).is_not_null()
	assert_str(h.http.last(ExamFixtures.ROUTE_START).path).is_equal("/exams/sessions/%s/attempts" % ExamFixtures.SESSION_ID)
	# Time left from the unchanged deadline and the new server time: 15 − 5 = 10 minutes.
	assert_int(services.exam.remaining_ms()).is_equal(600_000)
	# Free navigation resumes at the first question without an answer; the saved ones are marked.
	assert_int(int(runner.call("current_index"))).is_equal(2)
	var states: Array = (runner.find_child("Grid", true, false) as Control).call("states") as Array
	assert_array(states.slice(0, 3)).is_equal(["answered", "answered", "unanswered"])
	runner.call("go_to", 0)
	assert_dict((runner.get("question") as QuestionRendererBase).current_response()).is_equal({"option_id": "o_b"})


func test_sequential_exam_resumes_after_the_last_answered_question() -> void:
	await _open("sequential_no_return")
	h.http.reply(ExamFixtures.ROUTE_START, 201, _resumed())
	await lobby.call("start_exam")
	var runner: Control = lobby.get("runner") as Control
	assert_int(int(runner.call("current_index"))).is_equal(2)
	runner.call("go_to", 0)
	assert_int(int(runner.call("current_index"))).is_equal(2)


func test_leaving_and_reopening_keeps_the_deadline() -> void:
	await _open()
	h.http.reply(ExamFixtures.ROUTE_START, 201, ExamFixtures.attempt(5, SAVED))
	await lobby.call("start_exam")
	await (lobby.get("runner") as Control).call("leave")
	h.clock.advance(300.0)
	h.http.reply(ExamFixtures.ROUTE_START, 201, _resumed(5))
	await lobby.call("start_exam")
	assert_int(services.exam.remaining_ms()).is_equal(600_000)
	assert_int(h.http.count(ExamFixtures.ROUTE_START)).is_equal(2)


func test_other_device_shows_a_notice_and_offers_take_over() -> void:
	await _open()
	h.http.reply(ExamFixtures.ROUTE_START, 409, ExamFixtures.problem(409, "EXAM_ACTIVE_ON_OTHER_DEVICE"))
	await lobby.call("start_exam")
	assert_object(lobby.get("runner")).is_null()
	assert_bool(QualityManager.suspend_reasons.has(&"exam")).is_false()
	var error: Label = lobby.find_child("RulesError", true, false) as Label
	assert_bool(error.visible).is_true()
	assert_str(error.text).is_equal(L10n.error_text("EXAM_ACTIVE_ON_OTHER_DEVICE"))
	var take_over: Button = lobby.find_child("TakeOver", true, false) as Button
	assert_bool(take_over.visible).is_true()
	var first_key: String = String(h.http.last(ExamFixtures.ROUTE_START).headers["idempotency-key"])
	h.http.reply(ExamFixtures.ROUTE_START, 201, ExamFixtures.attempt(5, SAVED))
	take_over.pressed.emit()
	for i: int in 20:
		await await_idle_frame()
	var rec: FakeHttp.Recorded = h.http.last(ExamFixtures.ROUTE_START)
	assert_dict(rec.body).is_equal({"take_over": true})
	assert_str(String(rec.headers["idempotency-key"])).is_not_equal(first_key)
	assert_object(lobby.get("runner")).is_not_null()


func test_other_device_while_answering_locks_the_runner() -> void:
	await _open()
	h.http.reply(ExamFixtures.ROUTE_START, 201, ExamFixtures.attempt())
	await lobby.call("start_exam")
	var runner: Control = lobby.get("runner") as Control
	h.http.reply(ExamFixtures.ROUTE_SAVE, 409, ExamFixtures.problem(409, "EXAM_ACTIVE_ON_OTHER_DEVICE"))
	(runner.get("question") as QuestionRendererBase).select_index(0)
	for i: int in 10:
		await await_idle_frame()
	assert_bool(services.exam.locked).is_true()
	assert_bool((runner.get("question") as QuestionRendererBase).locked).is_true()
	assert_str((runner.find_child("Notice", true, false) as Label).text).is_equal(
			L10n.error_text("EXAM_ACTIVE_ON_OTHER_DEVICE"))
	assert_bool((runner.find_child("Submit", true, false) as Button).disabled).is_true()
