extends GdUnitTestSuite
## F064-AC-05, F064-AC-11 (TC-EXM-002, TC-TEN-024; T009): in the runner a question scene (F063) works in exam mode —
## choosing or changing an answer saves it at once with `saveExamAnswer` (latest answer per position), nothing about
## right/wrong, hints or explanations is ever shown; typed answers keep the render payload length limit, are saved
## when the student leaves the question, and no score or client-computed time is sent. Saved answers come back when
## the student returns to a question. FakeHttp/FakeClock with contract fixtures (TEST-ONLY).

const LOBBY_SCENE: String = "res://scenes/exam/ExamLobby.tscn"

var h: AuthHarness = null
var services: ServiceLocator = null
var lobby: Control = null
var runner: Control = null


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	services = ServiceLocator.new(h.api, AppState, h.clock)
	lobby = auto_free((load(LOBBY_SCENE) as PackedScene).instantiate()) as Control
	lobby.set("services", services)
	lobby.set("grade_band", "g4_5")
	add_child(lobby)
	h.http.reply(ExamFixtures.ROUTE_SESSION, 200, ExamFixtures.session())
	await lobby.call("open", {"exam_session_id": ExamFixtures.SESSION_ID})
	h.http.reply(ExamFixtures.ROUTE_START, 201, ExamFixtures.attempt(6))
	h.http.reply(ExamFixtures.ROUTE_SAVE, 204)
	await lobby.call("start_exam")
	runner = lobby.get("runner") as Control


func after_test() -> void:
	QualityManager.suspend_reasons.clear()
	h.dispose()


func _question() -> QuestionRendererBase:
	return runner.get("question") as QuestionRendererBase


func _settle() -> void:
	for i: int in 10:
		await await_idle_frame()


func test_choosing_and_changing_an_answer_saves_each_time() -> void:
	_question().select_index(0)
	await _settle()
	assert_int(h.http.count(ExamFixtures.ROUTE_SAVE)).is_equal(1)
	var first: Dictionary = h.http.last(ExamFixtures.ROUTE_SAVE).body
	_question().select_index(1)
	await _settle()
	assert_int(h.http.count(ExamFixtures.ROUTE_SAVE)).is_equal(2)
	var second: FakeHttp.Recorded = h.http.last(ExamFixtures.ROUTE_SAVE)
	assert_str(second.path).is_equal("/exams/attempts/%s/answers/1" % ExamFixtures.ATTEMPT_ID)
	assert_dict(second.body["response"] as Dictionary).is_not_equal(first["response"] as Dictionary)
	for key: String in ["score", "is_correct", "client_elapsed_ms"]:
		assert_bool(second.body.has(key)).override_failure_message(key).is_false()


func test_no_right_or_wrong_is_shown_while_answering() -> void:
	_question().select_index(0)
	await _settle()
	var text: String = ""
	for node: Node in runner.find_children("*", "Label", true, false):
		if node.name != "Title":
			text += tr((node as Label).text) + "\n"
	for node: Node in runner.find_children("*", "Button", true, false):
		assert_str((node as Button).text).is_not_equal("ui.activity_player.check")
	for key: String in ["ui.feedback.correct", "ui.feedback.incorrect", "ui.feedback.saved"]:
		assert_str(text).not_contains(tr(key))
	assert_bool(_question().locked).is_false()
	assert_object(runner.find_child("HintPanel", true, false)).is_null()


func test_answered_question_is_marked_in_the_grid() -> void:
	_question().select_index(0)
	await _settle()
	var states: Array = (runner.find_child("Grid", true, false) as Control).call("states") as Array
	assert_str(String(states[0])).is_equal("answered")
	assert_str(String(states[1])).is_equal("unanswered")


func test_typed_answer_keeps_the_limit_and_is_saved_when_leaving_the_question() -> void:
	runner.call("go_to", 4)
	assert_str(_question().item.type).is_equal("short_answer")
	var edit: LineEdit = _question().get("edit") as LineEdit
	assert_int(edit.max_length).is_equal(40)
	edit.text_changed.emit("a".repeat(60))
	await _settle()
	assert_int(h.http.count(ExamFixtures.ROUTE_SAVE)).is_equal(0)
	runner.call("go_next")
	await _settle()
	assert_int(h.http.count(ExamFixtures.ROUTE_SAVE)).is_equal(1)
	var rec: FakeHttp.Recorded = h.http.last(ExamFixtures.ROUTE_SAVE)
	assert_int(int(rec.body["position"])).is_equal(5)
	assert_int(String((rec.body["response"] as Dictionary)["text"]).length()).is_equal(40)


func test_returning_to_a_question_restores_the_answer() -> void:
	_question().select_index(1)
	await _settle()
	var saved: Dictionary = _question().current_response()
	runner.call("go_next")
	runner.call("go_to", 0)
	assert_dict(_question().current_response()).is_equal(saved)
