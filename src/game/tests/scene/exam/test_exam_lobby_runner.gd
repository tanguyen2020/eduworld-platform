extends GdUnitTestSuite
## F064-AC-01, F064-AC-02, F064-AC-06, F064-AC-07, F064-AC-09 (TC-EXM-002, TC-EXM-003; T008): opening an exam calls
## `getExamSession` and shows the rules (questions, duration, navigation, result release, "time keeps running");
## "Bắt đầu" starts the attempt and opens the runner with the world suspended for `exam`, question scenes only (no
## feedback, hint or NPC); `sequential_no_return` hides "Câu trước" and the grid only goes forward, `free` moves freely;
## "Nộp bài" lists the unanswered and flagged questions before submitting; a refused start shows `ui.error.<CODE>` on
## the rules screen without opening the runner. FakeHttp/FakeClock with contract fixtures (TEST-ONLY).

const LOBBY_SCENE: String = "res://scenes/exam/ExamLobby.tscn"

var h: AuthHarness = null
var services: ServiceLocator = null
var lobby: Control = null
var closed: int = 0


func before_test() -> void:
	h = AuthHarness.create()
	await h.sign_in()
	services = ServiceLocator.new(h.api, AppState, h.clock)
	lobby = auto_free((load(LOBBY_SCENE) as PackedScene).instantiate()) as Control
	lobby.set("services", services)
	lobby.set("grade_band", "g4_5")
	add_child(lobby)
	closed = 0
	lobby.connect("closed", func() -> void: closed += 1)


func after_test() -> void:
	QualityManager.suspend_reasons.clear()
	h.dispose()


func _node(node_name: String) -> Control:
	return lobby.find_child(node_name, true, false) as Control


func _text(node_name: String) -> String:
	var n: Control = _node(node_name)
	if n is Label:
		return (n as Label).text
	return (n as Button).text


func _open_rules(navigation: String = "free", release: String = "after_close") -> void:
	h.http.reply(ExamFixtures.ROUTE_SESSION, 200, ExamFixtures.session(navigation, release))
	await lobby.call("open", {"exam_session_id": ExamFixtures.SESSION_ID, "assignment_id": ExamFixtures.ASSIGNMENT_ID})


func _start(navigation: String = "free", count: int = 5, saved: Array = []) -> Control:
	await _open_rules(navigation)
	h.http.reply(ExamFixtures.ROUTE_START, 201, ExamFixtures.attempt(count, saved))
	h.http.reply(ExamFixtures.ROUTE_SAVE, 204)
	await lobby.call("start_exam")
	return lobby.get("runner") as Control


func test_opening_an_exam_shows_the_rules() -> void:
	await _open_rules("sequential_no_return", "manual")
	assert_int(h.http.count(ExamFixtures.ROUTE_SESSION)).is_equal(1)
	assert_bool(_node("Rules").visible).is_true()
	assert_str(_text("RulesTitle")).is_equal("Kiểm tra: Phép nhân")
	assert_str(_text("RulesQuestions")).is_equal(tr("ui.exam.rules_questions").format({"count": 15}))
	assert_str(_text("RulesDuration")).is_equal(tr("ui.exam.rules_duration").format({"minutes": 15}))
	assert_str(_text("RulesNavigation")).is_equal(tr("ui.exam.rules_navigation_sequential"))
	assert_str(_text("RulesRelease")).is_equal(tr("ui.exam.rules_release_manual"))
	assert_str(_text("RulesOffline")).is_equal(tr("ui.exam.rules_offline"))
	assert_str(_text("Start")).is_equal("ui.exam.rules_start")
	assert_object(lobby.get("runner")).is_null()
	assert_int(h.http.count(ExamFixtures.ROUTE_START)).is_equal(0)


func test_free_navigation_and_release_texts() -> void:
	await _open_rules("free", "immediate")
	assert_str(_text("RulesNavigation")).is_equal(tr("ui.exam.rules_navigation_free"))
	assert_str(_text("RulesRelease")).is_equal(tr("ui.exam.rules_release_immediate"))


func test_start_opens_the_runner_with_the_world_suspended_and_no_feedback() -> void:
	var runner: Control = await _start()
	assert_object(runner).is_not_null()
	assert_bool(runner.visible).is_true()
	assert_bool(QualityManager.suspend_reasons.has(&"exam")).is_true()
	assert_bool(Uuid.is_v7(String(h.http.last(ExamFixtures.ROUTE_START).headers["idempotency-key"]))).is_true()
	# Exam mode: question scenes only — no feedback strip, hint panel, voice-over NPC.
	for absent: String in ["FeedbackStrip", "HintPanel", "VoiceOver"]:
		assert_object(runner.find_child(absent, true, false)).override_failure_message(absent).is_null()
	assert_object(runner.get("question")).is_not_null()
	assert_str((runner.find_child("Position", true, false) as Label).text).is_equal(
			tr("ui.exam.question_of").format({"n": 1, "total": 5}))


func test_leaving_the_runner_resumes_the_world() -> void:
	var runner: Control = await _start()
	runner.call("leave")
	assert_bool(QualityManager.suspend_reasons.has(&"exam")).is_false()


func test_sequential_hides_previous_and_the_grid_only_goes_forward() -> void:
	var runner: Control = await _start("sequential_no_return")
	var prev: Button = runner.find_child("Prev", true, false) as Button
	assert_bool(prev.visible).is_false()
	runner.call("go_next")
	runner.call("go_next")
	assert_int(int(runner.call("current_index"))).is_equal(2)
	assert_bool(prev.visible).is_false()
	var grid: Control = runner.find_child("Grid", true, false) as Control
	var buttons: Array[Button] = grid.call("buttons") as Array[Button]
	assert_bool(buttons[0].disabled).is_true()
	assert_bool(buttons[1].disabled).is_true()
	assert_bool(buttons[2].disabled).is_false()
	assert_bool(buttons[4].disabled).is_false()
	buttons[0].pressed.emit()
	assert_int(int(runner.call("current_index"))).is_equal(2)
	buttons[4].pressed.emit()
	assert_int(int(runner.call("current_index"))).is_equal(4)


func test_free_navigation_moves_back_and_through_the_grid() -> void:
	var runner: Control = await _start("free")
	var prev: Button = runner.find_child("Prev", true, false) as Button
	runner.call("go_next")
	assert_bool(prev.visible).is_true()
	assert_bool(prev.disabled).is_false()
	prev.pressed.emit()
	assert_int(int(runner.call("current_index"))).is_equal(0)
	var buttons: Array[Button] = (runner.find_child("Grid", true, false) as Control).call("buttons") as Array[Button]
	for b: Button in buttons:
		assert_bool(b.disabled).is_false()
	buttons[3].pressed.emit()
	assert_int(int(runner.call("current_index"))).is_equal(3)


func test_submit_lists_unanswered_and_flagged_then_submits_with_a_key() -> void:
	var runner: Control = await _start("free", 5, [{"position": 1, "response": {"option_id": "o_b"}}])
	runner.call("go_to", 3)
	(runner.find_child("Flag", true, false) as Button).pressed.emit()
	(runner.find_child("Submit", true, false) as Button).pressed.emit()
	var confirm: Control = runner.find_child("SubmitConfirm", true, false) as Control
	assert_bool(confirm.visible).is_true()
	assert_str((confirm.find_child("Unanswered", true, false) as Label).text).is_equal(
			tr("ui.exam.confirm_unanswered").format({"list": "2, 3, 4, 5"}))
	assert_str((confirm.find_child("Flagged", true, false) as Label).text).is_equal(
			tr("ui.exam.confirm_flagged").format({"list": "4"}))
	assert_int(h.http.count(ExamFixtures.ROUTE_SUBMIT)).is_equal(0)
	h.http.reply(ExamFixtures.ROUTE_SUBMIT, 200, ExamFixtures.submit_result())
	h.http.reply(ExamFixtures.ROUTE_RESULT, 403, ExamFixtures.problem(403, "EXAM_RESULTS_NOT_RELEASED"))
	confirm.call("confirm")
	for i: int in 30:
		await await_idle_frame()
	assert_int(h.http.count(ExamFixtures.ROUTE_SUBMIT)).is_equal(1)
	assert_bool(Uuid.is_v7(String(h.http.last(ExamFixtures.ROUTE_SUBMIT).headers["idempotency-key"]))).is_true()


func test_cancel_keeps_the_exam_open() -> void:
	var runner: Control = await _start()
	(runner.find_child("Submit", true, false) as Button).pressed.emit()
	var confirm: Control = runner.find_child("SubmitConfirm", true, false) as Control
	(confirm.find_child("Cancel", true, false) as Button).pressed.emit()
	assert_bool(confirm.visible).is_false()
	assert_int(h.http.count(ExamFixtures.ROUTE_SUBMIT)).is_equal(0)


func test_refused_start_shows_the_error_on_the_rules_screen() -> void:
	for pair: Array in [[403, "EXAM_NOT_ASSIGNED"], [403, "EXAM_NOT_OPEN"], [403, "EXAM_WINDOW_CLOSED"], [409, "EXAM_MAX_ATTEMPTS"]]:
		await _open_rules()
		h.http.reply(ExamFixtures.ROUTE_START, int(pair[0]), ExamFixtures.problem(int(pair[0]), String(pair[1])))
		await lobby.call("start_exam")
		assert_bool(_node("Rules").visible).is_true()
		assert_bool(_node("RulesError").visible).is_true()
		assert_str(_text("RulesError")).is_equal(L10n.error_text(String(pair[1])))
		assert_object(lobby.get("runner")).is_null()
		assert_bool(QualityManager.suspend_reasons.has(&"exam")).is_false()


func test_close_returns_to_the_game() -> void:
	await _open_rules()
	lobby.call("close")
	assert_bool(lobby.visible).is_false()
	assert_int(closed).is_equal(1)
